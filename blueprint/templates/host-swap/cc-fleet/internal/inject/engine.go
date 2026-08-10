package inject

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/resolve"
)

// Engine owns target resolution and the guarded tmux delivery sequence.
type Engine struct {
	resolver Resolver
	tmux     Tmux
	options  Options
}

// New constructs a jailed-path-aware injection engine.
func New(dependencies Dependencies) (*Engine, error) {
	if dependencies.Resolver == nil {
		resolver, err := resolve.New(nil)
		if err != nil {
			return nil, err
		}
		dependencies.Resolver = resolver
	}
	if dependencies.Tmux == nil {
		dependencies.Tmux = CommandTmux{}
	}
	resolved, err := paths.Resolve()
	if err != nil {
		return nil, err
	}
	options := withDefaults(dependencies.Options)
	applyEnvironment(&options)
	if options.LockRoot == "" {
		options.LockRoot = filepath.Join(
			resolved.Home,
			".local",
			"state",
			"cc-fleet",
			"inject-locks",
		)
	}
	return &Engine{
		resolver: dependencies.Resolver,
		tmux:     dependencies.Tmux,
		options:  options,
	}, nil
}

func applyEnvironment(options *Options) {
	seconds := []struct {
		name   string
		target *time.Duration
	}{
		{"CHAT_INJECT_POLL", &options.Poll},
		{"CHAT_INJECT_ENTER_GAP", &options.EnterGap},
		{"CHAT_INJECT_ENTER_SETTLE", &options.EnterSettle},
		{"CHAT_INJECT_PROOF_SETTLE", &options.ProofSettle},
		{"CHAT_INJECT_LOCK_TIMEOUT", &options.LockTimeout},
		{"CHAT_INJECT_LOCK_POLL", &options.LockPoll},
		{"CHAT_INJECT_LOCK_MAXHOLD", &options.LockMaxHold},
	}
	for _, setting := range seconds {
		value := os.Getenv(setting.name)
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err == nil && parsed >= 0 {
			*setting.target = time.Duration(parsed * float64(time.Second))
		}
	}
	integers := []struct {
		name   string
		target *int
	}{
		{"CHAT_INJECT_BUSY_TRIES", &options.BusyTries},
		{"CHAT_INJECT_INTR_TRIES", &options.InterruptTries},
		{"CHAT_INJECT_STASH_TRIES", &options.StashTries},
		{"CHAT_INJECT_SETTLE_TRIES", &options.SettleTries},
		{"CHAT_INJECT_ENTER_TRIES", &options.EnterTries},
		{"CHAT_INJECT_SCROLLBACK", &options.Scrollback},
		{"CHAT_INJECT_PROOF_LINES", &options.ProofLines},
		{"CHAT_INJECT_CX_INLINE_MAX", &options.CodexInlineMax},
		{"CC_FLEET_MCP_MAX_MESSAGE_BYTES", &options.AbsoluteByteMax},
	}
	for _, setting := range integers {
		value := os.Getenv(setting.name)
		if value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err == nil && parsed > 0 {
			*setting.target = parsed
		}
	}
}

func withDefaults(options Options) Options {
	if options.Poll == 0 {
		options.Poll = 200 * time.Millisecond
	}
	if options.EnterGap == 0 {
		options.EnterGap = 150 * time.Millisecond
	}
	if options.EnterSettle == 0 {
		options.EnterSettle = 400 * time.Millisecond
	}
	if options.ProofSettle == 0 {
		options.ProofSettle = 500 * time.Millisecond
	}
	if options.BusyTries == 0 {
		options.BusyTries = 5
	}
	if options.InterruptTries == 0 {
		options.InterruptTries = 8
	}
	if options.StashTries == 0 {
		options.StashTries = 4
	}
	if options.SettleTries == 0 {
		options.SettleTries = 40
	}
	if options.EnterTries == 0 {
		options.EnterTries = 12
	}
	if options.Scrollback == 0 {
		options.Scrollback = 300
	}
	if options.ProofLines == 0 {
		options.ProofLines = 20
	}
	if options.LockTimeout == 0 {
		options.LockTimeout = 30 * time.Second
	}
	if options.LockPoll == 0 {
		options.LockPoll = 100 * time.Millisecond
	}
	if options.LockMaxHold == 0 {
		options.LockMaxHold = 60 * time.Second
	}
	if options.CodexInlineMax == 0 {
		options.CodexInlineMax = CodexInlineMax
	}
	if options.AbsoluteByteMax == 0 {
		options.AbsoluteByteMax = AbsoluteMessageMax
	}
	return options
}

// Resolve applies chat.sh's live target ladder.
func (engine *Engine) Resolve(ctx context.Context, name string) (Target, int, string, error) {
	if name == "" {
		return Target{}, 1, "empty target", nil
	}
	if name == "self" || name == "me" {
		socket := currentSocketPath()
		if socket == "" {
			return Target{}, 1, "self target requires TMUX", nil
		}
		session, err := engine.tmux.CurrentSession(ctx, socket)
		if err != nil {
			return Target{}, 1, "", err
		}
		return targetFromParts(socket, session), 0, "", nil
	}
	if rawPane(name) {
		socket := currentSocketPath()
		if socket == "" {
			return Target{}, 1, "raw pane target requires TMUX", nil
		}
		return targetFromParts(socket, name), 0, "", nil
	}

	for _, kind := range []resolve.Kind{
		resolve.Session,
		resolve.Label,
		resolve.CxWindow,
	} {
		outcome, err := engine.resolver.Resolve(ctx, kind, name)
		if err != nil {
			return Target{}, 2, "", err
		}
		switch outcome.Code {
		case 0:
			socket, pane, ok := parseTargetLine(outcome.Stdout)
			if !ok {
				return Target{}, 2, "", fmt.Errorf(
					"resolver %s returned malformed target",
					kind,
				)
			}
			return targetFromParts(socket, pane), 0, outcome.Stderr, nil
		case 2:
			return Target{}, 2, outcome.Stderr, nil
		}
	}
	return Target{}, 1, fmt.Sprintf("target %q matched no live chat", name), nil
}

// Capture resolves and captures a pane without mutating it.
func (engine *Engine) Capture(
	ctx context.Context,
	name string,
	tailLines int,
) (Target, string, int, string, error) {
	target, code, detail, err := engine.Resolve(ctx, name)
	if err != nil || code != 0 {
		return target, "", code, detail, err
	}
	capture, err := engine.tmux.Capture(
		ctx,
		target.SocketPath,
		target.Pane,
		false,
		0,
	)
	if err != nil {
		return target, "", 1, "target pane is dead or unreadable", nil
	}
	if tailLines > 0 {
		capture = lastNonEmptyLines(capture, tailLines)
	}
	return target, capture, 0, "", nil
}

// Inject performs the full guard/type/submit/proof transaction.
func (engine *Engine) Inject(ctx context.Context, request Request) (Result, error) {
	if request.Message == "" {
		return refused(1, "refusing to inject an empty message"), nil
	}
	if len(request.Message) > engine.options.AbsoluteByteMax {
		return refused(
			6,
			fmt.Sprintf(
				"ABORT: message is %d bytes (absolute MCP limit %d); nothing was typed",
				len(request.Message),
				engine.options.AbsoluteByteMax,
			),
		), nil
	}

	target, code, detail, err := engine.Resolve(ctx, request.Target)
	if err != nil {
		return Result{}, err
	}
	if code != 0 {
		return refused(code, detail), nil
	}
	base := Result{
		Status:     "refused",
		SocketPath: target.SocketPath,
		Pane:       target.Pane,
	}
	if target.Engine == "cx" &&
		utf8.RuneCountInString(request.Message) > engine.options.CodexInlineMax {
		base.Code = 6
		base.Message = fmt.Sprintf(
			"ABORT: message is %d chars — over the %d-char inline cap for codex panes; nothing was typed",
			utf8.RuneCountInString(request.Message),
			engine.options.CodexInlineMax,
		)
		return base, nil
	}

	lock, err := acquireTargetLock(
		engine.options.LockRoot,
		target.SocketPath+":"+target.Pane,
		engine.options.LockTimeout,
		engine.options.LockPoll,
		engine.options.LockMaxHold,
	)
	if err != nil {
		base.Code = 4
		base.Message = fmt.Sprintf(
			"could not acquire inject lock for %q: %v",
			target.Pane,
			err,
		)
		return base, nil
	}
	defer lock.release()

	capture, err := engine.capture(ctx, target, 0)
	if err != nil {
		base.Code = 1
		base.Message = "target pane is dead or unreadable"
		return base, nil
	}
	base.Busy = IsBusy(capture)
	if base.Busy && request.ForceNow {
		for attempt := 0; attempt < engine.options.InterruptTries; attempt++ {
			if !IsBusy(capture) {
				break
			}
			if err := engine.tmux.SendKey(
				ctx,
				target.SocketPath,
				target.Pane,
				"Escape",
			); err != nil {
				return Result{}, err
			}
			base.Interrupted = true
			sleepContext(ctx, engine.options.Poll)
			capture, err = engine.capture(ctx, target, 0)
			if err != nil {
				base.Code = 1
				base.Message = "target pane died while interrupting"
				return base, nil
			}
		}
	} else if base.Busy {
		for attempt := 0; attempt < engine.options.BusyTries; attempt++ {
			sleepContext(ctx, engine.options.Poll)
			capture, err = engine.capture(ctx, target, 0)
			if err != nil {
				base.Code = 1
				base.Message = "target pane died while waiting for idle"
				return base, nil
			}
			if !IsBusy(capture) {
				base.Busy = false
				break
			}
		}
		if base.Busy {
			base.Code = 7
			base.Message = "ABORT: target pane is busy; nothing was typed (retry when idle or use force_now)"
			return base, nil
		}
	}

	if inMode, modeErr := engine.tmux.PaneInMode(
		ctx,
		target.SocketPath,
		target.Pane,
	); modeErr == nil && inMode {
		if err := engine.tmux.CancelCopyMode(ctx, target.SocketPath, target.Pane); err != nil {
			return Result{}, err
		}
		sleepContext(ctx, engine.options.Poll)
		capture, err = engine.capture(ctx, target, 0)
		if err != nil {
			base.Code = 1
			base.Message = "target pane died while leaving copy mode"
			return base, nil
		}
	}
	if strings.Contains(strings.ToLower(capture), "restore the code") ||
		strings.Contains(strings.ToLower(lastLines(capture, 12)), "create a plan?") {
		if err := engine.tmux.SendKey(
			ctx,
			target.SocketPath,
			target.Pane,
			"Escape",
		); err != nil {
			return Result{}, err
		}
		sleepContext(ctx, engine.options.Poll)
		capture, err = engine.capture(ctx, target, 0)
		if err != nil {
			base.Code = 1
			base.Message = "target pane died while dismissing an overlay"
			return base, nil
		}
	}

	if menu := SelectorLine(capture); menu != "" {
		base.Code = 4
		base.Message = fmt.Sprintf(
			"ABORT: %q has an OPEN selector menu (marker on a numbered option: %s); nothing was typed",
			target.Pane,
			menu,
		)
		return base, nil
	}

	if err := engine.tmux.SendKey(
		ctx,
		target.SocketPath,
		target.Pane,
		"C-s",
	); err != nil {
		return Result{}, err
	}
	sleepContext(ctx, engine.options.Poll)
	capture, err = engine.capture(ctx, target, 0)
	if err != nil {
		base.Code = 1
		base.Message = "target pane died during draft guard"
		return base, nil
	}
	base.DraftStashed = strings.Contains(
		strings.ToLower(lastLines(capture, 8)),
		"stashed",
	)
	draftLine := lastLineContaining(capture, "❯")
	for attempt := 0; attempt < engine.options.StashTries && hasDraft(draftLine); attempt++ {
		styled, _ := engine.tmux.Capture(
			ctx,
			target.SocketPath,
			target.Pane,
			true,
			0,
		)
		if isDimPlaceholder(lastLineContaining(styled, "❯")) {
			break
		}
		if err := engine.tmux.SendKey(
			ctx,
			target.SocketPath,
			target.Pane,
			"C-s",
		); err != nil {
			return Result{}, err
		}
		sleepContext(ctx, engine.options.Poll)
		capture, err = engine.capture(ctx, target, 0)
		if err != nil {
			break
		}
		if strings.Contains(strings.ToLower(lastLines(capture, 8)), "stashed") {
			base.DraftStashed = true
		}
		draftLine = lastLineContaining(capture, "❯")
	}
	if hasDraft(draftLine) {
		styled, _ := engine.tmux.Capture(
			ctx,
			target.SocketPath,
			target.Pane,
			true,
			0,
		)
		if !isDimPlaceholder(lastLineContaining(styled, "❯")) {
			base.Code = 5
			base.Message = fmt.Sprintf(
				"ABORT: %q composer holds a draft that will not stash (mash guard): %.80s; nothing was typed",
				target.Pane,
				strings.TrimSpace(strings.TrimPrefix(draftLine, "❯")),
			)
			return base, nil
		}
	}

	message := engine.signedMessage(ctx, request.Message, base.Interrupted)
	if err := engine.tmux.SendLiteral(
		ctx,
		target.SocketPath,
		target.Pane,
		message,
	); err != nil {
		return Result{}, err
	}
	base.Typed = true
	normalized := normalizeSpace(message)
	needle := tailRunes(normalized, 40)
	for attempt := 0; attempt < engine.options.SettleTries; attempt++ {
		_ = lock.beat()
		capture, err = engine.capture(ctx, target, 0)
		if err == nil && strings.Contains(normalizeSpace(capture), needle) {
			break
		}
		sleepContext(ctx, engine.options.Poll)
	}

	prefix := headRunes(normalizeSpace(message), 24)
	submitted := false
	for attempt := 1; attempt <= engine.options.EnterTries; attempt++ {
		base.SubmitRetries = attempt
		_ = lock.beat()
		if err := engine.tmux.SendKey(
			ctx,
			target.SocketPath,
			target.Pane,
			"Enter",
		); err != nil {
			return Result{}, err
		}
		if !base.DraftStashed {
			sleepContext(ctx, engine.options.EnterGap)
			if err := engine.tmux.SendKey(
				ctx,
				target.SocketPath,
				target.Pane,
				"Enter",
			); err != nil {
				return Result{}, err
			}
		}
		sleepContext(ctx, engine.options.EnterSettle)
		capture, err = engine.capture(ctx, target, 0)
		if err != nil {
			continue
		}
		input := lastComposerLine(capture)
		if input == "" {
			scrollback, scrollErr := engine.capture(
				ctx,
				target,
				engine.options.Scrollback,
			)
			if scrollErr == nil {
				input = lastComposerLine(scrollback)
			}
		}
		if input == "" || strings.Contains(input, "[Pasted text") {
			continue
		}
		if !strings.Contains(normalizeSpace(input), prefix) {
			submitted = true
			break
		}
	}
	if !submitted {
		base.Status = "typed_unconfirmed"
		base.Code = 3
		base.Message = fmt.Sprintf(
			"typed text into %q but could not confirm submission after %d Enter attempts",
			target.Pane,
			engine.options.EnterTries,
		)
		return base, nil
	}

	sleepContext(ctx, engine.options.ProofSettle)
	proof, proofErr := engine.capture(ctx, target, 0)
	if proofErr != nil {
		return Result{}, proofErr
	}
	proofInput := lastComposerLine(proof)
	if proofInput == "" {
		scrollback, _ := engine.capture(ctx, target, engine.options.Scrollback)
		proofInput = lastComposerLine(scrollback)
	}
	if strings.Contains(proofInput, "[Pasted text") ||
		(proofInput != "" && strings.Contains(normalizeSpace(proofInput), prefix)) {
		base.Status = "typed_unconfirmed"
		base.Code = 4
		base.Message = fmt.Sprintf(
			"PROOF-CONTRADICTION: %q composer still holds the message",
			target.Pane,
		)
		base.Proof = lastNonEmptyLines(proof, engine.options.ProofLines)
		return base, nil
	}
	base.Status = "delivered"
	base.Code = 0
	base.Message = fmt.Sprintf(
		"injected LIVE into %q — typed inline and submitted (Enter confirmed, input cleared)",
		target.Pane,
	)
	base.Proof = lastNonEmptyLines(proof, engine.options.ProofLines)
	return base, nil
}

func (engine *Engine) capture(
	ctx context.Context,
	target Target,
	scrollback int,
) (string, error) {
	return engine.tmux.Capture(
		ctx,
		target.SocketPath,
		target.Pane,
		false,
		scrollback,
	)
}

func (engine *Engine) signedMessage(
	ctx context.Context,
	message string,
	interrupted bool,
) string {
	trimmed := strings.TrimLeftFunc(message, unicode.IsSpace)
	if strings.HasPrefix(trimmed, "/") || engine.options.DisableSignature {
		return message
	}
	marker := ""
	if interrupted {
		marker = " — ⚠ FORCE-DELIVERED via Esc (your running flow was interrupted; re-check any in-progress action)"
	}
	sender := Sender{}
	if engine.options.Sender != nil {
		sender = *engine.options.Sender
	} else {
		sender = engine.detectSender(ctx)
	}
	parts := make([]string, 0, 3)
	if sender.UUID != "" {
		parts = append(parts, "sid "+headRunes(sender.UUID, 8))
	}
	if sender.Session != "" {
		parts = append(
			parts,
			"to reply: /chat:inject "+sender.Session+" <message>",
		)
	}
	if sender.Label != "" {
		parts = append(parts, "🔖 "+sender.Label)
	}
	if len(parts) == 0 {
		return message + marker
	}
	return message + marker + "  — " + strings.Join(parts, " · ")
}

func (engine *Engine) detectSender(ctx context.Context) Sender {
	sender := Sender{UUID: os.Getenv("CLAUDE_CODE_SESSION_ID")}
	socket := currentSocketPath()
	if socket == "" {
		return sender
	}
	session, err := engine.tmux.CurrentSession(ctx, socket)
	if err != nil {
		return sender
	}
	sender.Session = session
	capture, err := engine.tmux.Capture(ctx, socket, session, false, 0)
	if err == nil {
		sender.Label = captureLabel(capture)
	}
	return sender
}

func targetFromParts(socketPath, pane string) Target {
	engine := "cc"
	if strings.HasPrefix(filepath.Base(socketPath), "cx-") {
		engine = "cx"
	}
	return Target{SocketPath: socketPath, Pane: pane, Engine: engine}
}

func parseTargetLine(line string) (string, string, bool) {
	fields := strings.Split(strings.TrimSpace(line), "\t")
	return firstTwo(fields)
}

func firstTwo(fields []string) (string, string, bool) {
	if len(fields) != 2 || fields[0] == "" || fields[1] == "" {
		return "", "", false
	}
	return fields[0], fields[1], true
}

func currentSocketPath() string {
	value := os.Getenv("TMUX")
	if comma := strings.IndexByte(value, ','); comma >= 0 {
		value = value[:comma]
	}
	return value
}

func rawPane(value string) bool {
	if len(value) < 2 || value[0] != '%' {
		return false
	}
	for _, character := range value[1:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func refused(code int, message string) Result {
	return Result{Status: "refused", Code: code, Message: message}
}

func normalizeSpace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func headRunes(value string, count int) string {
	runes := []rune(value)
	if len(runes) > count {
		runes = runes[:count]
	}
	return string(runes)
}

func tailRunes(value string, count int) string {
	runes := []rune(value)
	if len(runes) > count {
		runes = runes[len(runes)-count:]
	}
	return string(runes)
}

func lastLines(value string, count int) string {
	lines := strings.Split(value, "\n")
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	return strings.Join(lines, "\n")
}

func sleepContext(ctx context.Context, duration time.Duration) {
	if duration <= 0 {
		return
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func captureLabel(capture string) string {
	label := ""
	for _, line := range strings.Split(capture, "\n") {
		if !strings.Contains(line, "🔖") ||
			(!strings.Contains(line, "🥇") &&
				!strings.Contains(line, "🥈") &&
				!strings.Contains(line, "🥉")) {
			continue
		}
		index := strings.LastIndex(line, "🔖")
		candidate := strings.TrimSpace(line[index+len("🔖"):])
		if separator := strings.Index(candidate, "│"); separator >= 0 {
			candidate = strings.TrimSpace(candidate[:separator])
		}
		label = candidate
	}
	return label
}
