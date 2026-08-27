package inject

import (
	"context"
	"errors"
	"fmt"
	pfmengine "hostops/pfm/internal/engine"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"hostops/pfm/internal/naming"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/resolve"
	"hostops/pfm/internal/shared"
)

// SenderSessionEnv, SenderLabelEnv, and SenderIDEnv are how a chat states its
// own identity to a process that cannot derive one — the --then waiter, and
// any dispatcher a chat detaches from itself. Both implementations read the
// same three names, so a chat.sh waiter and a pfm waiter sign alike.
const (
	SenderSessionEnv = "CHAT_SENDER_SESSION"
	SenderLabelEnv   = "CHAT_SENDER_LABEL"
	SenderIDEnv      = "CHAT_SENDER_SID"
)

// Engine owns target resolution and the guarded tmux delivery sequence.
type Engine struct {
	resolver      Resolver
	tmux          Tmux
	spawner       ThenSpawner
	options       Options
	whoami        SelfIdentifier
	codexSeat     SelfIdentifier
	binaries      map[pfmengine.ID]string
	accountEmojis []string
	recorder      func(context.Context, shared.CommsEvent) error
	warningWriter io.Writer
	// senderSelf is this process's own identity, resolved at most once: it
	// costs a tmux capture, and it cannot change while we run.
	senderOnce sync.Once
	senderSelf Sender
}

// New constructs a jailed-path-aware injection engine.
func New(dependencies Dependencies) (*Engine, error) {
	binaries := resolve.Binaries{
		Values: map[pfmengine.ID]string{
			pfmengine.Claude:   dependencies.ClaudeBinary,
			pfmengine.Codex:    dependencies.CodexBinary,
			pfmengine.Opencode: dependencies.OpencodeBinary,
		},
		AccountEmojis: dependencies.AccountEmojis,
	}
	if dependencies.Resolver == nil {
		resolver, err := resolve.New(nil, binaries)
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
	if dependencies.WarningWriter == nil {
		dependencies.WarningWriter = os.Stderr
	}
	resolved, err := paths.Resolve()
	if err != nil {
		return nil, err
	}
	options := withDefaults(dependencies.Options)
	applyEnvironment(&options)
	if options.BodyRoot == "" {
		options.BodyRoot = filepath.Join(
			resolved.Home,
			".local", "state", "pfm", "inject-bodies",
		)
	}
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
		resolver:      dependencies.Resolver,
		tmux:          dependencies.Tmux,
		spawner:       dependencies.Spawner,
		options:       options,
		whoami:        dependencies.Identifier,
		codexSeat:     dependencies.CodexSeat,
		binaries:      cloneEngineBinaries(binaries.Values),
		accountEmojis: append([]string(nil), dependencies.AccountEmojis...),
		recorder:      dependencies.Recorder,
		warningWriter: dependencies.WarningWriter,
	}, nil
}

// WithIdentity returns a request-scoped engine whose caller identity is the
// live seat supplied by the fleet registry. MCP's HTTP server is shared by
// every Codex thread, so the long-lived engine must never cache one caller's
// sender or use it to resolve another caller's "self" target.
//
// The engine is rebuilt field-by-field instead of copying Engine: senderOnce
// is a sync.Once and may already have been used by the shared engine. The
// scoped copy starts with a fresh sender cache and an explicit sender.
func (engine *Engine) WithIdentity(identity resolve.Identity, label string) *Engine {
	sender := &Sender{
		Session: identity.Session,
		Label:   strings.TrimSpace(label),
		UUID:    identity.ID,
	}
	options := engine.options
	options.Sender = sender
	return &Engine{
		resolver:      engine.resolver,
		tmux:          engine.tmux,
		spawner:       engine.spawner,
		options:       options,
		whoami:        fixedIdentifier{identity: identity},
		binaries:      cloneEngineBinaries(engine.binaries),
		accountEmojis: append([]string(nil), engine.accountEmojis...),
		recorder:      engine.recorder,
		warningWriter: engine.warningWriter,
	}
}

type fixedIdentifier struct {
	identity resolve.Identity
}

func (fixed fixedIdentifier) Identify(context.Context) (resolve.Identity, error) {
	return fixed.identity, nil
}

func cloneEngineBinaries(values map[pfmengine.ID]string) map[pfmengine.ID]string {
	cloned := make(map[pfmengine.ID]string, len(values))
	for id, binary := range values {
		cloned[id] = binary
	}
	return cloned
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
		{"CHAT_INJECT_COMMAND_GAP", &options.CommandChunkGap},
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
		{"CHAT_INJECT_COMMAND_CHUNK", &options.CommandChunkRunes},
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
		options.ProofLines = 15
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
	if options.ClaudeAutoFileMax == 0 {
		options.ClaudeAutoFileMax = ClaudeAutoFileMax
	}
	if options.CodexAutoFileMax == 0 {
		options.CodexAutoFileMax = CodexAutoFileMax
	}
	if options.CommandChunkRunes == 0 {
		options.CommandChunkRunes = CommandChunkRunes
	}
	if options.CommandChunkGap == 0 {
		options.CommandChunkGap = CommandChunkGap
	}
	if options.BodyMaxAge == 0 {
		options.BodyMaxAge = defaultBodyMaxAge
	}
	if options.Now == nil {
		options.Now = time.Now
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
	name = unquoteTarget(name)
	if name == "" {
		return Target{}, CodeUnknown, "empty target", nil
	}
	if name == "self" || name == "me" {
		identity, err := engine.whoami.Identify(ctx)
		if (err != nil || identity.Session == "") &&
			engine.codexSeat != nil &&
			os.Getenv(resolve.CodexThreadEnv) != "" {
			identity, err = engine.codexSeat.Identify(ctx)
		}
		if err != nil || identity.Session == "" || identity.SocketPath == "" {
			return Target{}, CodeUnknown, "self target has no live tmux seat", nil
		}
		pane := identity.Session
		if identity.Pane != "" {
			pane = identity.Pane
		}
		target := targetFromParts(identity.SocketPath, pane)
		if identity.Engine == string(pfmengine.Codex) {
			target.Engine = string(pfmengine.Codex)
		}
		return target, 0, "", nil
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
			return Target{}, CodeUnknown, "raw pane target requires TMUX", nil
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
			return Target{}, CodeUndelivered, "", err
		}
		switch outcome.Code {
		case 0:
			socket, pane, ok := parseTargetLine(outcome.Stdout)
			if !ok {
				return Target{}, CodeUndelivered, "", fmt.Errorf(
					"resolver %s returned malformed target",
					kind,
				)
			}
			target := targetFromParts(socket, pane)
			if kind == resolve.CxWindow {
				target.Engine = string(pfmengine.Codex)
			}
			return target, 0, outcome.Stderr, nil
		case 2:
			return Target{}, CodeAmbiguous, outcome.Stderr, nil
		}
	}
	return Target{}, CodeUnknown, fmt.Sprintf("target %q matched no live chat", name), nil
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
		return target, "", CodeDead, "target pane is dead or unreadable", nil
	}
	if tailLines > 0 {
		capture = lastNonEmptyLines(capture, tailLines)
	}
	return target, capture, 0, "", nil
}

// ScheduleAfterCurrentTurn arms a detached steer chain without typing into
// the current composer. Slash commands queued while a Codex model turn is
// running become ordinary model input rather than TUI commands; the detached
// waiter must therefore wait for idle before it types the primary command.
func (engine *Engine) ScheduleAfterCurrentTurn(
	ctx context.Context,
	request Request,
) (Result, error) {
	if request.Message == "" {
		return refused(CodeUndelivered, "refusing to schedule an empty message"), nil
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
	if _, captureErr := engine.capture(ctx, target, 0); captureErr != nil {
		return refused(CodeDead, "target pane is dead or unreadable"), nil
	}
	steers := make([]string, 0, len(request.Then)+1)
	steers = append(steers, request.Message)
	steers = append(steers, request.Then...)
	logPath := engine.steerLogPath(target.Pane)
	if err := engine.spawner.Spawn(ctx, SteerSpawn{
		SocketPath: target.SocketPath,
		Target:     target.Pane,
		Steers:     steers,
		LogPath:    logPath,
		Append:     request.Chain,
		Sender:     engine.sender(ctx),
		// Only the ORIGINAL self-compaction needs the caller's turn ridden
		// out. A chained re-arm is typed into a pane this waiter already
		// watched settle, so its next busy is the steer's own turn.
		SelfTarget: !request.Chain && isSelfTarget(request.Target),
	}); err != nil {
		return refused(
			CodeUndelivered,
			fmt.Sprintf("could not schedule command after the current turn: %v", err),
		), nil
	}
	message := fmt.Sprintf("scheduled COMMAND into %q after the current turn settles — %d post-command steer(s) armed (log: %s)", target.Pane, len(request.Then), logPath)
	if isSelfCompactRequest(request) {
		message += SelfCompactStopNotice
	}
	return Result{
		Status:     "scheduled",
		Code:       0,
		Message:    message,
		SocketPath: target.SocketPath,
		Pane:       target.Pane,
		Steers:     len(request.Then),
		SteerLog:   logPath,
	}, nil
}

// isSelfCompactRequest is true for the one shape the stop rule applies to: a
// chat compacting ITSELF. For any other target the waiter is watching somebody
// else's pane, so what this caller does next cannot blur the turn boundary.
func isSelfCompactRequest(request Request) bool {
	return isCompactCommand(request.Message) && isSelfTarget(request.Target)
}

// isSelfTarget reports the one target spelling that names the calling chat's
// own pane.
func isSelfTarget(target string) bool {
	return strings.EqualFold(strings.TrimSpace(target), "self")
}

// SelfCompactStopNotice rides on the SUCCESS result of a self-compaction,
// where it is read in the same breath as the decision it governs. The --then
// waiter recognises the compaction turn by watching this pane yield and then go
// busy again (waitForSettledTurn); a caller that keeps working after queueing
// the compaction merges its own turn into the compaction's and leaves the
// waiter no boundary to find.
const SelfCompactStopNotice = " — STOP NOW: end this turn without running " +
	"another tool. Say only that compaction is queued with its steer to " +
	"follow. Any further work here competes with the compaction the waiter " +
	"is watching for, and the steer will land beside it instead of after it."

// Inject performs the delivery and records a successful direct send without
// making the recipient pay for a ledger failure.
func (engine *Engine) Inject(ctx context.Context, request Request) (Result, error) {
	result, err := engine.inject(ctx, request)
	if err != nil || result.Code != 0 || !result.Typed || request.Origin != "" || engine.recorder == nil {
		return result, err
	}
	sender := engine.sender(ctx)
	event := shared.CommsEvent{
		AtNS:           engine.options.Now().UnixNano(),
		Kind:           shared.KindInject,
		SenderSession:  sender.Session,
		SenderLabel:    sender.Label,
		SenderUUID:     sender.UUID,
		Target:         request.Target,
		ReceiverSocket: result.SocketPath,
		ReceiverPane:   result.Pane,
		Message:        request.Message,
	}
	if recordErr := engine.recorder(ctx, event); recordErr != nil {
		fmt.Fprintf(engine.warningWriter, "pfm: comms ledger: %v\n", recordErr)
	}
	return result, nil
}

// inject performs the full guard/type/submit/proof transaction.
func (engine *Engine) inject(ctx context.Context, request Request) (Result, error) {
	if request.Message == "" {
		return refused(CodeUndelivered, "refusing to inject an empty message"), nil
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
		Status:         "refused",
		SocketPath:     target.SocketPath,
		Pane:           target.Pane,
		ResolutionNote: detail,
	}
	lock, err := acquireTargetLock(
		engine.options.LockRoot,
		target.SocketPath+":"+target.Pane,
		engine.options.LockTimeout,
		engine.options.LockPoll,
		engine.options.LockMaxHold,
	)
	if err != nil {
		base.Code = CodeUndelivered
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
		base.Code = CodeDead
		base.Message = "target pane is dead or unreadable"
		return base, nil
	}
	base.Busy = IsBusy(capture)
	command, commandErr := engine.tmux.PaneCommand(
		ctx,
		target.SocketPath,
		target.Pane,
	)
	verifiedEngine := ""
	if commandErr == nil {
		verifiedEngine = paneCommandEngine(command, engine.binaries)
		if verifiedEngine != "" {
			target.Engine = verifiedEngine
		}
	}
	// Both TUIs own a safe composer queue while a turn is running. A normal
	// inject types there and submits without interrupting the active turn;
	// force-now alone is allowed to send Escape. pane_current_command is NOT a
	// queue gate: tmux reports the foreground descendant, so a legitimate chat
	// reads as node/python/sleep while one of its tools owns the foreground.
	queueing := base.Busy && !request.ForceNow
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
				base.Code = CodeDead
				base.Message = "target pane died while interrupting"
				return base, nil
			}
		}
	} else if base.Busy && !queueing {
		for attempt := 0; attempt < engine.options.BusyTries; attempt++ {
			sleepContext(ctx, engine.options.Poll)
			capture, err = engine.capture(ctx, target, 0)
			if err != nil {
				base.Code = CodeDead
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
			base.Code = CodeDead
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
			base.Code = CodeDead
			base.Message = "target pane died while dismissing an overlay"
			return base, nil
		}
	}
	// Overlay dismissal can put tmux back into copy mode. Re-check at the last
	// possible moment before any composer key, matching the first cancellation
	// rather than assuming the overlay Escape left a writable pane.
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
			base.Code = CodeDead
			base.Message = "target pane died while leaving copy mode after overlay dismissal"
			return base, nil
		}
	}

	if menu := SelectorLine(capture); menu != "" {
		base.Code = CodeUndelivered
		base.Message = fmt.Sprintf(
			"ABORT: %q has an OPEN selector menu (marker on a numbered option: %s); nothing was typed",
			target.Pane,
			menu,
		)
		return base, nil
	}

	// Idle panes take the ordinary C-s mash guard. A busy pane with an empty
	// composer is already a safe queue surface and receives no control key;
	// only an actual busy draft is stashed before the new turn is queued.
	needsStashGuard := !queueing
	if queueing && hasDraft(lastComposerLine(capture)) {
		styled, _ := engine.tmux.Capture(
			ctx,
			target.SocketPath,
			target.Pane,
			true,
			0,
		)
		needsStashGuard = !isDimPlaceholder(lastComposerLine(styled))
	}
	if needsStashGuard {
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
			base.Code = CodeDead
			base.Message = "target pane died during draft guard"
			return base, nil
		}
		base.DraftStashed = strings.Contains(
			strings.ToLower(lastLines(capture, 8)),
			"stashed",
		)
		draftLine := lastComposerLine(capture)
		for attempt := 0; attempt < engine.options.StashTries && hasDraft(draftLine); attempt++ {
			styled, _ := engine.tmux.Capture(
				ctx,
				target.SocketPath,
				target.Pane,
				true,
				0,
			)
			if isDimPlaceholder(lastComposerLine(styled)) {
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
			draftLine = lastComposerLine(capture)
		}
		if hasDraft(draftLine) {
			styled, _ := engine.tmux.Capture(
				ctx,
				target.SocketPath,
				target.Pane,
				true,
				0,
			)
			if !isDimPlaceholder(lastComposerLine(styled)) {
				base.Code = CodeUndelivered
				base.Message = fmt.Sprintf(
					"ABORT: %q composer holds a draft that will not stash (mash guard): %.80s; nothing was typed and the draft is preserved (dim placeholder text is exempt)",
					target.Pane,
					strings.TrimSpace(strings.TrimLeft(draftLine, "❯›")),
				)
				return base, nil
			}
		}
	}

	preSubmitCapture := capture
	prepared, prepareErr := engine.prepareLiveMessage(
		ctx,
		target,
		request.Message,
		base.Interrupted,
	)
	if prepareErr != nil {
		base.Code = CodeUndelivered
		if errors.Is(prepareErr, ErrUnsigned) {
			// Nothing was sent, so nothing is unsigned: the flag describes what
			// the RECIPIENT got, and the recipient got nothing.
			base.Message = prepareErr.Error()
			return base, nil
		}
		base.Message = fmt.Sprintf("could not persist long inject body: %v", prepareErr)
		return base, nil
	}
	message := prepared.Message
	base.Unsigned = prepared.Unsigned
	base.AutoFilePath = prepared.AutoFilePath
	commandTransport := isHarnessCommand(request.Message)
	pasteTransport := request.FileBacked && prepared.AutoFilePath == "" && !commandTransport
	pacedLiteral := commandTransport ||
		utf8.RuneCountInString(message) > engine.autoFileThreshold(target.Engine)
	var sendErr error
	if pasteTransport {
		sendErr = engine.tmux.SendPaste(
			ctx,
			target.SocketPath,
			target.Pane,
			message,
		)
	} else if pacedLiteral {
		base.LiteralChunks, sendErr = engine.sendPacedLiteral(
			ctx,
			target,
			message,
			lock,
		)
	} else {
		sendErr = engine.tmux.SendLiteral(
			ctx,
			target.SocketPath,
			target.Pane,
			message,
		)
	}
	if sendErr != nil {
		return Result{}, sendErr
	}
	base.Typed = true
	normalized := normalizeSpace(message)
	needle := tailRunes(normalized, 40)
	for attempt := 0; attempt < engine.options.SettleTries; attempt++ {
		_ = lock.beat()
		capture, err = engine.capture(ctx, target, 0)
		if err == nil && (strings.Contains(normalizeSpace(capture), needle) ||
			(pasteTransport && HasPastePlaceholder(capture))) {
			break
		}
		sleepContext(ctx, engine.options.Poll)
	}
	// Render-settle is advisory only. Echo detection flakes on wrapping,
	// bracketed-paste placeholders, and footer glyphs; once literal bytes have
	// been typed we must always reach the bounded Enter loop. Returning here
	// would strand a typed-but-unsent body and make every retry mash another
	// message into the same composer.

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
		if input == "" || HasPastePlaceholder(input) {
			continue
		}
		if !strings.Contains(normalizeSpace(input), prefix) {
			submitted = true
			break
		}
	}
	if !submitted {
		base.Status = "typed_unconfirmed"
		base.Code = CodeUndelivered
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
			// Resolved HERE, in the chat, because the waiter cannot: it is
			// detached from us and derives nothing.
			Sender: engine.sender(ctx),
		}); err != nil {
			base.Status = "delivered"
			base.Code = CodeUndelivered
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
		base.Status = "delivered_unproven"
		base.Code = 0
		base.Message = fmt.Sprintf(
			"delivered-unproven into %q — input cleared, but the post-submit pane capture failed: %v",
			target.Pane,
			proofErr,
		)
		base.Proof = fmt.Sprintf("[pane capture failed: %v]", proofErr)
		return base, nil
	}
	proofInput := lastComposerLine(proof)
	if proofInput == "" {
		scrollback, _ := engine.capture(ctx, target, engine.options.Scrollback)
		proofInput = lastComposerLine(scrollback)
	}
	blindProof := proofInput == ""
	if HasPastePlaceholder(proofInput) ||
		(proofInput != "" && strings.Contains(normalizeSpace(proofInput), prefix)) {
		base.Status = "typed_unconfirmed"
		base.Code = CodeUndelivered
		base.Message = fmt.Sprintf(
			"PROOF-CONTRADICTION: %q composer still holds the message",
			target.Pane,
		)
		base.Proof = lastNonEmptyLines(proof, engine.options.ProofLines)
		return base, nil
	}
	base.Status = "delivered"
	if queueing {
		base.Status = "queued"
	}
	base.Code = 0
	switch {
	case queueing && commandTransport:
		base.Message = fmt.Sprintf(
			"queued COMMAND into %q — busy %s accepted %d paced literal chunk(s) without interruption (Enter confirmed, input cleared)",
			target.Pane,
			engineName(target.Engine),
			base.LiteralChunks,
		)
	case queueing && prepared.AutoFilePath != "":
		base.Message = fmt.Sprintf(
			"queued AUTO-FILE pointer into %q — full body saved at %s; busy %s accepted the pointer without interruption (Enter confirmed, input cleared)",
			target.Pane,
			prepared.AutoFilePath,
			engineName(target.Engine),
		)
	case queueing && request.FileBacked:
		base.Message = fmt.Sprintf(
			"queued FILE-BACKED into %q — busy %s accepted the bracketed-paste block without interruption (Enter confirmed, input cleared)",
			target.Pane,
			engineName(target.Engine),
		)
	case queueing:
		base.Message = fmt.Sprintf(
			"queued into %q — busy %s accepted the turn without interruption (Enter confirmed, input cleared)",
			target.Pane,
			engineName(target.Engine),
		)
	case commandTransport:
		base.Message = fmt.Sprintf(
			"injected LIVE COMMAND into %q — %d paced literal chunk(s) submitted (Enter confirmed, input cleared)",
			target.Pane,
			base.LiteralChunks,
		)
	case prepared.AutoFilePath != "":
		base.Message = fmt.Sprintf(
			"injected LIVE AUTO-FILE pointer into %q — full body saved at %s (Enter confirmed, input cleared)",
			target.Pane,
			prepared.AutoFilePath,
		)
	case request.FileBacked:
		base.Message = fmt.Sprintf(
			"injected LIVE FILE-BACKED into %q — bracketed-paste body submitted (Enter confirmed, input cleared)",
			target.Pane,
		)
	default:
		base.Message = fmt.Sprintf(
			"injected LIVE into %q — typed inline and submitted (Enter confirmed, input cleared)",
			target.Pane,
		)
	}
	if base.Interrupted {
		base.Message += " — FORCE-NOW: interrupted the target's running flow"
	}
	if base.DraftStashed {
		base.Message += " — stashed and restored the target's unsent draft"
	}
	if base.Steers > 0 {
		base.Message += fmt.Sprintf(
			" — %d then steer(s) queued; deliver in order, one settled turn apart (log: %s)",
			base.Steers,
			base.SteerLog,
		)
	}
	for _, warning := range prepared.Warnings {
		base.Message += " — WARNING: " + warning
	}
	base.Proof = lastNonEmptyLines(proof, engine.options.ProofLines)
	if !deliveryProven(
		preSubmitCapture,
		proof,
		message,
		queueing,
		pasteTransport,
	) {
		base.Status = "delivered_unproven"
		base.Message = fmt.Sprintf(
			"delivered-unproven into %q — input cleared, but the pane capture below shows neither the message past the input bar nor the expected %s",
			target.Pane,
			proofExpectation(queueing),
		)
	}
	if blindProof {
		base.Message += " (proof caveat: no composer line was visible in capture or scrollback; submission was confirmed, but the screen proof is blind)"
	}
	return base, nil
}

func engineName(value string) string {
	id, err := pfmengine.Parse(value)
	if err != nil {
		return fmt.Sprintf("engine %q", value)
	}
	return pfmengine.MustLookup(id).Short
}

func (engine *Engine) sendPacedLiteral(
	ctx context.Context,
	target Target,
	message string,
	lock *targetLock,
) (int, error) {
	runes := []rune(message)
	chunks := 0
	for start := 0; start < len(runes); start += engine.options.CommandChunkRunes {
		end := min(start+engine.options.CommandChunkRunes, len(runes))
		if err := lock.beat(); err != nil {
			return chunks, fmt.Errorf("refresh inject lock while typing command chunk %d: %w", chunks+1, err)
		}
		if err := engine.tmux.SendLiteral(
			ctx,
			target.SocketPath,
			target.Pane,
			string(runes[start:end]),
		); err != nil {
			return chunks, err
		}
		chunks++
		if end < len(runes) {
			sleepContext(ctx, engine.options.CommandChunkGap)
		}
	}
	return chunks, nil
}

func paneCommandEngine(command string, binaries map[pfmengine.ID]string) string {
	name := filepath.Base(strings.TrimSpace(command))
	for _, id := range pfmengine.All() {
		descriptor := pfmengine.MustLookup(id)
		configured := binaries[id]
		if name == descriptor.Binary || configured != "" && name == filepath.Base(configured) {
			return string(id)
		}
	}
	// Claude's version-managed native executable is named like 2.1.47. This
	// is the same process-name seam the live resolver already recognizes.
	dot := strings.IndexByte(name, '.')
	if dot <= 0 {
		return ""
	}
	for _, character := range name[:dot] {
		if character < '0' || character > '9' {
			return ""
		}
	}
	return string(pfmengine.Claude)
}

// checkSteerChain applies chat.sh:596-625 before anything is resolved or
// typed: a bad chain must die at the caller, not later in a detached waiter's
// log where nobody is watching.
func (engine *Engine) checkSteerChain(request Request) (Result, bool) {
	for _, steer := range request.Then {
		if strings.TrimSpace(steer) == "" {
			return refused(
				CodeUndelivered,
				"ERROR: a then steer must be non-empty",
			), false
		}
		if isCompactCommand(steer) {
			return refused(
				CodeUndelivered,
				"ERROR: a then steer must not itself start with /compact — compact-steering-into-compact recurses and loses the thread",
			), false
		}
	}
	if !isCompactCommand(request.Message) {
		return Result{}, true
	}
	// EVERY /compact inject must carry a steer (operator rule): compaction
	// returns to an idle prompt — no turn fires — so a steerless compact
	// strands the target command-less (chat.sh:591-611).
	if len(request.Then) == 0 {
		return refused(
			6,
			"ABORT: a /compact inject requires a then steer — compaction ends at an idle prompt with no turn fired, stranding the target. Re-run with the steer: chat_inject{target, message:'/compact <focus>', then:['<post-compact steer>']}. Nothing was typed.",
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
	if isHarnessCommand(message) || engine.options.DisableSignature {
		return message, false
	}
	marker := ""
	if interrupted {
		marker = " — ⚠ FORCE-DELIVERED via Esc (your running flow was interrupted; re-check any in-progress action)"
	}
	sender := engine.sender(ctx)
	parts := signatureParts(sender)
	if len(parts) == 0 {
		// chat.sh:648-657 — every derivation source came back empty. Dropping
		// the footer silently makes an unsigned message indistinguishable from
		// a signed one at the recipient, so the absence is STATED instead.
		return message + marker + "  — " + unsignedFooter(sender), true
	}
	return message + marker + "  — " + strings.Join(parts, " · "), false
}

// SignForResume applies the same identity policy as a live send, but uses a
// block footer because transcript injection is JSON text rather than terminal
// keystrokes. Slash commands remain byte-exact.
func (engine *Engine) SignForResume(ctx context.Context, message string) (string, bool) {
	if isHarnessCommand(message) || engine.options.DisableSignature {
		return message, false
	}
	sender := engine.sender(ctx)
	parts := signatureParts(sender)
	if len(parts) == 0 {
		return message + "\n\n— " + unsignedFooter(sender), true
	}
	return message + "\n\n— " + strings.Join(parts, " · "), false
}

// signatureParts builds the footer a recipient reads to know who spoke and
// how to answer. Two rules it got wrong for a long time are load-bearing here.
//
// The reply hint advertises the LABEL whenever there is one. It used to
// advertise sender.Session — a raw tmux session id — two lines above stating
// the sender's label, while senderLabel's own contract is that the label is
// carried bare precisely "since the recipient must be able to reply by
// exactly the label the operator gave the chat". A caller obeying the old
// footer replied with a raw session id, which landed in the comms ledger as
// the target and minted a cosmos node named after a socket. The session is
// still the fallback, and it is legitimate there: it is the literal argument
// a caller must pass, not a name being presented as one.
//
// There is no 🔖 part any more, and its absence is deliberate rather than an
// omission. 🔖 is the STATUSLINE's identity marker, and naming.BookmarkLabelFor
// scrapes a pane's own label by finding the last 🔖 line on screen. Stamping
// it into a delivered message wrote the SENDER's identity into the
// RECIPIENT's pane, where the scraper could read it back as the recipient's
// own label — a chat's identity becoming whatever the last message it
// received claimed its sender was called. The reply hint above already names
// the sender, so nothing is lost by not re-stating it under a marker that
// means something else.
func signatureParts(sender Sender) []string {
	parts := make([]string, 0, 2)
	if sender.UUID != "" {
		parts = append(parts, "sid "+headRunes(sender.UUID, 8))
	}
	if address := replyAddress(sender); address != "" {
		parts = append(
			parts,
			naming.DeliveredFooterMarker+address+" <message>",
		)
	}
	return parts
}

// unquoteTarget strips one layer of surrounding double quotes from a target.
//
// It is the read side of the reply hint's write side: a label containing
// spaces is advertised as chat_inject "Delivery Trust" <message>, because the
// CLI form needs the shell quoting to see one argument. A recipient reaching
// for the MCP tool instead passes the target as a JSON string, where those
// quotes are just two extra characters that would make the label match
// nothing. Accepting both spellings costs one trim; refusing one of them
// would make the hint wrong for whichever caller read it the other way.
func unquoteTarget(name string) string {
	trimmed := strings.TrimSpace(name)
	if len(trimmed) >= 2 &&
		strings.HasPrefix(trimmed, `"`) &&
		strings.HasSuffix(trimmed, `"`) {
		return strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	}
	return trimmed
}

// replyAddress is the one string a recipient can pass straight back to
// chat_inject. A label with whitespace is quoted, because a codex chat's
// label is its tmux WINDOW name and those legitimately contain spaces —
// unquoted, "chat_inject Delivery Trust <message>" reads as a target plus a
// stray word. Quoting is safe for the label's other hazard too: ':' is tmux's
// session:window separator, but this is a chat_inject argument and never a
// tmux target, and inject.Resolve hands a label to the label resolver, which
// answers with a %pane id.
func replyAddress(sender Sender) string {
	if sender.Label != "" {
		if strings.ContainsFunc(sender.Label, unicode.IsSpace) {
			return `"` + sender.Label + `"`
		}
		return sender.Label
	}
	return sender.Session
}

// unsignedFooter states what was TRIED, never what happens to be unset. Reading
// $TMUX told a codex chat sitting in a live pane it had "no tmux context" —
// true of the variable, false of the chat, since a scrubbed tool shell never
// carries one and ANCESTRY is what resolves the handle there. The resolved
// sender is the honest witness.
func unsignedFooter(sender Sender) string {
	footer := "UNSIGNED — sender identity underivable"
	if sender.Session == "" {
		footer += "; no tmux session (ambient or via ancestry)"
	}
	if sender.UUID == "" {
		footer += "; no session id"
	}
	return footer
}

// sender answers who WE are, resolved at most once per process: the caller's
// override (tests, the MCP surface), then the STATED sender, then live
// derivation from this process.
//
// The stated rung exists because derivation is only possible where the chat
// is. A --then waiter is spawned with setsid, which severs the process chain
// ancestry recovery walks; a codex tool shell carries no $TMUX and no session
// id of its own (codex scrubs its command environment), so a codex-origin
// waiter has NOTHING left to derive from and every steer it delivered went out
// UNSIGNED. The origin resolves identity while it still can and hands it down,
// which is why this is also resolved for a `/`-prefixed primary: that message
// is exempt from signing, but its steers are not.
func (engine *Engine) sender(ctx context.Context) Sender {
	engine.senderOnce.Do(func() {
		if engine.options.Sender != nil {
			engine.senderSelf = *engine.options.Sender
			return
		}
		if stated, ok := statedSender(); ok {
			engine.senderSelf = stated
			return
		}
		engine.senderSelf = engine.detectSender(ctx)
	})
	return engine.senderSelf
}

// statedSender reads the identity a spawning chat handed this process. It is
// read from our OWN environment only — never from a message or a caller flag,
// so a chat can state who IT is and never who somebody else is.
func statedSender() (Sender, bool) {
	stated := Sender{
		Session: os.Getenv(SenderSessionEnv),
		Label:   os.Getenv(SenderLabelEnv),
		UUID:    os.Getenv(SenderIDEnv),
	}
	if stated.Session == "" && stated.Label == "" && stated.UUID == "" {
		return Sender{}, false
	}
	return stated, true
}

// detectSender derives this chat's own identity the way chat.sh does — from
// its own process, never from a caller flag. The tmux handle comes from
// $TMUX, or from ANCESTRY RECOVERY when the engine spawned our shell without
// passing tmux context through (the codex path), which is what left every
// codex-origin message unsigned (chat.sh:65-96).
func (engine *Engine) detectSender(ctx context.Context) Sender {
	sender := Sender{UUID: os.Getenv("CLAUDE_CODE_SESSION_ID")}
	identity, err := engine.whoami.Identify(ctx)
	if (err != nil || identity.Session == "") &&
		engine.codexSeat != nil &&
		os.Getenv(resolve.CodexThreadEnv) != "" &&
		os.Getenv(resolve.ClaudeSessionEnv) == "" {
		identity, err = engine.codexSeat.Identify(ctx)
	}
	if err != nil || identity.Session == "" {
		return sender
	}
	if sender.UUID == "" && identity.Engine == string(pfmengine.Codex) {
		sender.UUID = identity.ID
	}
	sender.Session = identity.Session
	sender.Label = engine.senderLabel(ctx, identity)
	return sender
}

// senderLabel scrapes this chat's own 🔖 label off its statusline, with
// chat.sh's codex fallback (chat.sh:104-121): a codex chat has no 🔖
// statusline, so its label is the tmux window name — the human thread name set
// by pfm — returned BARE, since the recipient must be able to reply by
// exactly the label the operator gave the chat.
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
		if label := naming.BookmarkLabelFor(capture, engine.accountEmojis); label != "" {
			return label
		}
	}
	if identity.Engine != string(pfmengine.Codex) {
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
	base := filepath.Base(socketPath)
	id, ok := pfmengine.FromSocket(base)
	if !ok && os.Getenv("PFM_TEST_PROBE_SOCKETS") == "1" {
		id, ok = pfmengine.FromSocket(strings.TrimPrefix(base, "probe-"))
	}
	if !ok {
		return Target{SocketPath: socketPath, Pane: pane, Engine: "unknown"}
	}
	return Target{SocketPath: socketPath, Pane: pane, Engine: string(id)}
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

// captureLabel reads this chat's own 🔖 label through naming, the one package
// that owns that scrape (K3).
func captureLabel(capture string) string {
	return naming.BookmarkLabel(capture)
}
