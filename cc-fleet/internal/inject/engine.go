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
	spawner  ThenSpawner
	options  Options
	whoami   SelfIdentifier
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
	if dependencies.Spawner == nil {
		dependencies.Spawner = CommandThenSpawner{}
	}
	if _, err := paths.Resolve(); err != nil {
		return nil, err
	}
	options := withDefaults(dependencies.Options)
	applyEnvironment(&options)
	// chat.sh:147 locks under ${TMPDIR:-/tmp}/chat-inject-locks. The Go engine
	// shares that namespace so a Go inject and a chat.sh inject into the same
	// pane mutually exclude instead of interleaving keystrokes.
	if options.LockRoot == "" {
		options.LockRoot = filepath.Join(tempRoot(), "chat-inject-locks")
	}
	if options.ThenLogRoot == "" {
		options.ThenLogRoot = tempRoot()
	}
	if dependencies.Identifier == nil {
		identifier, err := resolve.NewWhoami(resolve.WhoamiDependencies{})
		if err != nil {
			return nil, err
		}
		dependencies.Identifier = identifier
	}
	return &Engine{
		resolver: dependencies.Resolver,
		tmux:     dependencies.Tmux,
		spawner:  dependencies.Spawner,
		options:  options,
		whoami:   dependencies.Identifier,
	}, nil
}

// tempRoot is chat.sh's ${TMPDIR:-/tmp}, the root both implementations share
// for inject locks and --then chain logs.
func tempRoot() string {
	if value := os.Getenv("TMPDIR"); value != "" {
		return strings.TrimRight(value, "/")
	}
	return "/tmp"
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
		{"CHAT_THEN_MIN", &options.ThenMin},
		{"CHAT_THEN_IDLE_POLL", &options.ThenIdlePoll},
		{"CHAT_THEN_SETTLE", &options.ThenSettle},
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
		{"COMPACT_FOCUS_MAX", &options.CompactFocusMax},
		{"CHAT_THEN_BUSY_TRIES", &options.ThenBusyTries},
		{"CHAT_THEN_IDLE_TRIES", &options.ThenIdleTries},
		{"CHAT_THEN_IDLE_STABLE", &options.ThenIdleStable},
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
	if options.CompactFocusMax == 0 {
		options.CompactFocusMax = CompactFocusMax
	}
	// chat.sh:1063-1077 __then cadence.
	if options.ThenMin == 0 {
		options.ThenMin = 1500 * time.Millisecond
	}
	if options.ThenBusyTries == 0 {
		options.ThenBusyTries = 25
	}
	if options.ThenIdlePoll == 0 {
		options.ThenIdlePoll = 400 * time.Millisecond
	}
	if options.ThenIdleTries == 0 {
		options.ThenIdleTries = 1500
	}
	if options.ThenIdleStable == 0 {
		options.ThenIdleStable = 3
	}
	if options.ThenSettle == 0 {
		options.ThenSettle = 400 * time.Millisecond
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
		// chat.sh:549 — pane ids are unique per tmux SERVER, not globally, so a
		// bare %id needs its socket from CHAT_INJECT_SOCKET (set by the __then
		// waiter re-delivering to the pane it watched) or from our own $TMUX.
		socket := os.Getenv("CHAT_INJECT_SOCKET")
		if socket == "" {
			socket = currentSocketPath()
		}
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
	// chat.sh:1215-1219 captures with -S - : the WHOLE retained scrollback, not
	// just the visible fold, bounded only by tmux's history-limit. The caller's
	// last_n / max_bytes bounds are applied to the result, after the capture.
	capture, err := engine.tmux.Capture(
		ctx,
		target.SocketPath,
		target.Pane,
		false,
		FullScrollback,
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
	if result, ok := engine.checkSteerChain(request); !ok {
		return result, nil
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
			base.Code = CodeBusy
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

	message, unsigned := engine.signedMessage(ctx, request.Message, base.Interrupted)
	base.Unsigned = unsigned
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

	// chat.sh:922-951 — the steer chain is armed ONLY here, after the primary
	// submit is confirmed. Our lock is released first so the detached waiter is
	// never blocked by us; it takes its own per-target lock when it delivers.
	if len(request.Then) > 0 {
		lock.release()
		base.SteerLog = engine.steerLogPath(target.Pane)
		if err := engine.spawner.Spawn(ctx, SteerSpawn{
			SocketPath: target.SocketPath,
			Target:     target.Pane,
			Steers:     request.Then,
			LogPath:    base.SteerLog,
			Append:     request.Chain,
		}); err != nil {
			base.Status = "delivered"
			base.Code = 8
			base.Message = fmt.Sprintf(
				"delivered the primary into %q but could NOT arm the %d then steer(s): %v — deliver them yourself once the pane is idle",
				target.Pane,
				len(request.Then),
				err,
			)
			base.Proof = lastNonEmptyLines(capture, engine.options.ProofLines)
			return base, nil
		}
		base.Steers = len(request.Then)
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
	if base.Steers > 0 {
		base.Message += fmt.Sprintf(
			" — %d then steer(s) queued; deliver in order, one settled turn apart (log: %s)",
			base.Steers,
			base.SteerLog,
		)
	}
	base.Proof = lastNonEmptyLines(proof, engine.options.ProofLines)
	return base, nil
}

// checkSteerChain applies chat.sh:596-625 before anything is resolved or
// typed: a bad chain must die at the caller, not later in a detached waiter's
// log where nobody is watching.
func (engine *Engine) checkSteerChain(request Request) (Result, bool) {
	for _, steer := range request.Then {
		if strings.TrimSpace(steer) == "" {
			return refused(
				1,
				"ERROR: a then steer must be non-empty",
			), false
		}
		if isCompactCommand(steer) {
			return refused(
				1,
				"ERROR: a then steer must not itself start with /compact — compact-steering-into-compact recurses and loses the thread",
			), false
		}
	}
	if !isCompactCommand(request.Message) {
		return Result{}, true
	}
	// EVERY /compact inject must carry a steer (founder law): compaction
	// returns to an idle prompt — no turn fires — so a steerless compact
	// strands the target command-less (chat.sh:591-611).
	if len(request.Then) == 0 {
		return refused(
			6,
			"ABORT: a /compact inject requires a then steer — compaction ends at an idle prompt with no turn fired, stranding the target. Re-run with the steer: chat_inject{target, message:'/compact <focus>', then:['<post-compact steer>']}. Nothing was typed.",
		), false
	}
	// LONG-FOCUS GUARD (chat.sh:612-624): a long /compact body is typed as a
	// bracketed PASTE, the TUI collapses it to "[Pasted text #N]", the Enter
	// lands on the collapsed block and the compaction never fires.
	length := utf8.RuneCountInString(request.Message)
	if length > engine.options.CompactFocusMax {
		return refused(
			6,
			fmt.Sprintf(
				"ABORT: /compact focus is %d chars (max %d) — a body this long is typed as a PASTE, the TUI collapses it, and the compaction never fires (queue-limbo, seen twice). Write the hold to a file and make the focus a POINTER: chat_inject{target, message:'/compact hold: read <abs path> — {2-3 facts that must survive verbatim}', then:['<steer>']}. Nothing was typed.",
				length,
				engine.options.CompactFocusMax,
			),
		), false
	}
	return Result{}, true
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

// signedMessage appends chat.sh's mandatory sender footer (chat.sh:628-668).
// The second return reports the UNSIGNED fallback, so the caller can say the
// message went out unsigned instead of leaving the absence silent.
func (engine *Engine) signedMessage(
	ctx context.Context,
	message string,
	interrupted bool,
) (string, bool) {
	trimmed := strings.TrimLeftFunc(message, unicode.IsSpace)
	if strings.HasPrefix(trimmed, "/") || engine.options.DisableSignature {
		return message, false
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
		// chat.sh:648-657 — every derivation source came back empty. Dropping
		// the footer silently makes an unsigned message indistinguishable from
		// a signed one at the recipient, so the absence is STATED instead.
		return message + marker + "  — " + unsignedFooter(), true
	}
	return message + marker + "  — " + strings.Join(parts, " · "), false
}

func unsignedFooter() string {
	footer := "UNSIGNED — sender identity underivable"
	if os.Getenv("TMUX") == "" {
		footer += "; no tmux context"
	}
	if os.Getenv("CLAUDE_CODE_SESSION_ID") == "" {
		footer += "; no session id"
	}
	return footer
}

// detectSender derives this chat's own identity the way chat.sh does — from
// its own process, never from a caller flag. The tmux handle comes from
// $TMUX, or from ANCESTRY RECOVERY when the engine spawned our shell without
// passing tmux context through (the codex path), which is what left every
// codex-origin message unsigned (chat.sh:65-96).
func (engine *Engine) detectSender(ctx context.Context) Sender {
	sender := Sender{UUID: os.Getenv("CLAUDE_CODE_SESSION_ID")}
	identity, err := engine.whoami.Identify(ctx)
	if err != nil || identity.Session == "" {
		return sender
	}
	if sender.UUID == "" && identity.Engine == resolve.CodexEngine {
		sender.UUID = identity.ID
	}
	sender.Session = identity.Session
	sender.Label = engine.senderLabel(ctx, identity)
	return sender
}

// senderLabel scrapes this chat's own 🔖 label off its statusline, with
// chat.sh's codex fallback (chat.sh:104-121): a codex chat has no 🔖
// statusline, so its label is the tmux window name — the human thread name set
// by cc-fleet — returned BARE, since the recipient must be able to reply by
// exactly the label the founder gave the chat.
func (engine *Engine) senderLabel(
	ctx context.Context,
	identity resolve.Identity,
) string {
	target := identity.Session
	if identity.Pane != "" {
		target = identity.Pane
	}
	capture, err := engine.tmux.Capture(
		ctx,
		identity.SocketPath,
		target,
		false,
		0,
	)
	if err == nil {
		if label := captureLabel(capture); label != "" {
			return label
		}
	}
	if identity.Engine != resolve.CodexEngine {
		return ""
	}
	window, err := engine.tmux.WindowName(ctx, identity.SocketPath, target)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(window)
}

// steerLogPath mirrors chat.sh:940 — ${TMPDIR:-/tmp}/chat-then-<target>.log
// with every non-alphanumeric character of the target replaced by '_'.
func (engine *Engine) steerLogPath(target string) string {
	sanitized := strings.Map(func(character rune) rune {
		switch {
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9':
			return character
		default:
			return '_'
		}
	}, target)
	return filepath.Join(engine.options.ThenLogRoot, "chat-then-"+sanitized+".log")
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
		// 🍀 is the retired account-4 medal — chats labelled while it was live
		// still render it, so it stays a valid label marker.
		if !strings.Contains(line, "🔖") ||
			(!strings.Contains(line, "🥇") &&
				!strings.Contains(line, "🥈") &&
				!strings.Contains(line, "🥉") &&
				!strings.Contains(line, "🍀")) {
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
