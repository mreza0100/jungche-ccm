package spawn

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"hostops/cc-fleet/internal/store"
)

// The Codex rename markers below are read from the codex binary's own strings
// (codex-cli 0.147): the slash command is "rename", described as "rename the
// current thread", and its modal asks you to "Type a name and press Enter".
// Codex has no launch flag for a thread name, so this UI is the only way to
// set one — and if a Codex release changes the wording, every wait below times
// out, the chat is reported UNNAMED with a warning, and nothing is typed
// blindly into a composer that would have sent it to the model as a prompt.
const (
	codexRenameCommand = "/rename"
	codexRenameOffered = "rename the current thread"
	codexRenamePrompt  = "Type a name and press Enter"
	codexRenameEmpty   = "Thread name cannot be empty."
	codexRenameDone    = "Session renamed to"

	// modalClearKeys is how many BSpace presses clear the rename field. Codex
	// PRE-FILLS it with the thread's current name, so typing straight into it
	// APPENDS — a retry after a half-finished attempt produced
	// "_HIDE probeTIMING TEST" on a live box. Extra backspaces on an empty
	// field are ignored, so over-clearing is free and under-clearing is not.
	modalClearKeys = 80

	// confirmPresses is how many times the modal's Enter is re-sent before the
	// rename is called unconfirmed.
	confirmPresses = 3

	// codexComposer and codexStatus are the two halves of "this chat will
	// receive what I type". BOTH are required, and neither is enough alone:
	//
	//   - "›" starts the composer's input line — but it also marks the
	//     SELECTED ROW of Codex's modals, so a trust dialog matches it. That
	//     false positive is what sent a "/rename" and a first prompt into a
	//     hooks-review modal on a live box.
	//   - the status line's token meter renders only under an idle composer;
	//     every startup overlay observed (hooks review, trust selection)
	//     paints over it.
	//
	// A modal that somehow matched both would still be survivable: Escape is
	// the DECLINE path on each of them ("esc to close", "esc to go back"), and
	// this package never presses `t` or Enter on a screen it has not confirmed.
	codexComposer = "›"
	codexStatus   = "% used"

	// startupEscapes bounds how many overlays are dismissed before giving up,
	// so a screen that is simply slow is never escaped forever.
	startupEscapes = 4

	// composerHoldReads is how many consecutive reads must show the composer
	// before it counts as ready. Codex paints its composer FIRST and its
	// startup overlays a beat later, so a single sighting is a flash, not a
	// state — that flash is what sent a "/rename" into a hooks modal.
	composerHoldReads = 4

	// renameAttempts covers an overlay that appears between the composer and
	// the keystroke: dismiss it and try the whole rename again rather than
	// declaring a Codex that plainly has /rename incapable of it.
	renameAttempts = 3
)

// Run creates the detached session and, for Codex, drives its rename UI. The
// session is left running in every case a chat exists: a rename that could not
// be completed is a warning on a live chat, never a reason to kill it.
func Run(
	ctx context.Context,
	tmux Tmux,
	request Request,
) (Result, error) {
	if tmux == nil {
		return Result{}, errors.New("spawn requires a tmux client")
	}
	if request.Socket == "" || request.Run == "" || request.CWD == "" {
		return Result{}, errors.New("spawn requires a socket, command and directory")
	}
	timings := request.Timings.orDefaults()
	trace := newTracer(request.Trace, time.Now())
	window := WindowName(request.Name)
	spec := SessionSpec{
		Socket:  request.Socket,
		Session: request.Socket,
		Window:  window,
		CWD:     request.CWD,
		Run:     request.Run,
		Width:   request.Width,
		Height:  request.Height,
	}
	if spec.Width <= 0 {
		spec.Width = 220
	}
	if spec.Height <= 0 {
		spec.Height = 50
	}
	if err := tmux.NewSession(ctx, spec); err != nil {
		return Result{}, err
	}

	result := Result{
		Socket:  request.Socket,
		Session: spec.Session,
		Window:  window,
		Name:    request.Name,
		Named:   request.Engine == store.ClaudeEngine,
	}
	target := spec.Session
	trace.step("session %s created, running: %s", spec.Session, request.Run)
	boot, err := waitForBoot(ctx, tmux, request.Socket, target, timings)
	if err != nil {
		return result, err
	}
	trace.step("booted | %s", screen(boot))
	if request.Engine != store.CodexEngine {
		result.Prompted = request.Prompt != ""
		return result, nil
	}

	// Nothing is typed until a composer is on screen and STAYS there. A
	// startup overlay swallows every keystroke sent to it — that is how a
	// chat ended up unnamed AND unprompted, with its "/rename" and its first
	// prompt both eaten by a hooks-review modal.
	named, warning, blocked := nameCodexThread(
		ctx,
		tmux,
		request.Socket,
		target,
		request.Name,
		timings,
		trace,
	)
	result.Named = named
	if warning != "" {
		result.Warnings = append(result.Warnings, warning)
	}
	if blocked || request.Prompt == "" {
		return result, nil
	}
	// An overlay can arrive at any moment during startup (MCP notices land
	// asynchronously), so the composer is re-confirmed before the prompt is
	// typed, exactly as it was before the rename.
	if !waitForCodexComposer(ctx, tmux, request.Socket, target, timings, trace) {
		result.Warnings = append(
			result.Warnings,
			"a startup screen is holding the chat — the first prompt was not "+
				"delivered; attach it and clear the screen by hand",
		)
		return result, nil
	}
	if err := typeLine(ctx, tmux, request.Socket, target, request.Prompt, timings); err != nil {
		result.Warnings = append(
			result.Warnings,
			fmt.Sprintf("the first prompt was not delivered: %v", err),
		)
		return result, nil
	}
	result.Prompted = true
	trace.step("prompt delivered")
	return result, nil
}

// waitForBoot returns once the pane has drawn something and stopped changing,
// which is the only readiness signal both engines share. A capture error means
// the session is gone — the chat died at birth, and saying so beats reporting
// a socket nothing is listening on.
func waitForBoot(
	ctx context.Context,
	tmux Tmux,
	socket, target string,
	timings Timings,
) (string, error) {
	deadline := time.Now().Add(timings.Boot)
	previous := ""
	settled := false
	for {
		capture, err := tmux.Capture(ctx, socket, target)
		if err != nil {
			return "", fmt.Errorf(
				"the chat died at birth on socket %s: %w",
				socket,
				err,
			)
		}
		trimmed := strings.TrimSpace(capture)
		if trimmed != "" && trimmed == previous {
			if settled {
				return capture, nil
			}
			settled = true
		} else {
			settled = false
		}
		previous = trimmed
		if time.Now().After(deadline) {
			if trimmed == "" {
				return "", fmt.Errorf(
					"the chat drew nothing within %s on socket %s",
					timings.Boot,
					socket,
				)
			}
			return capture, nil
		}
		if err := sleep(ctx, timings.Poll); err != nil {
			return "", err
		}
	}
}

// composerReady reports whether a capture shows an idle composer.
func composerReady(capture string) bool {
	return strings.Contains(capture, codexComposer) &&
		strings.Contains(capture, codexStatus)
}

// waitForCodexComposer returns once the composer is drawn, dismissing startup
// overlays along the way. Escape is the documented way out of every one of
// them ("esc to close" / "esc to go back"), and it is sent only once the
// screen has stopped changing, so a slow paint is never mistaken for a stuck
// modal. Escape on an empty composer does nothing, which is what makes it safe
// to send before knowing which screen is up.
func waitForCodexComposer(
	ctx context.Context,
	tmux Tmux,
	socket, target string,
	timings Timings,
	trace tracer,
) bool {
	deadline := time.Now().Add(timings.Boot)
	previous := ""
	escapes := 0
	held := 0
	for {
		capture, err := tmux.Capture(ctx, socket, target)
		switch {
		case err == nil && composerReady(capture):
			held++
			if held >= composerHoldReads {
				trace.step("composer held %d reads | %s", held, screen(capture))
				return true
			}
		case err == nil:
			// An overlay: dismiss it once the screen has stopped changing, so
			// a half-drawn frame is never mistaken for a stuck modal.
			held = 0
			trimmed := strings.TrimSpace(capture)
			if trimmed != "" && trimmed == previous && escapes < startupEscapes {
				trace.step("overlay %d dismissed with Escape | %s", escapes+1, screen(capture))
				_ = tmux.SendKey(ctx, socket, target, "Escape")
				escapes++
				previous = ""
			} else {
				previous = trimmed
			}
		default:
			held = 0
		}
		if time.Now().After(deadline) {
			return false
		}
		if err := sleep(ctx, timings.Poll); err != nil {
			return false
		}
	}
}

// nameCodexThread waits for a composer that holds, then renames — retrying the
// pair when an overlay lands between the two. Codex draws its composer before
// its startup modals, so "composer, then modal, then keystroke" is a real
// ordering, not a hypothetical one.
// The blocked return says the composer was never reachable, which is a
// different verdict from "renamed nothing": nothing was typed at all, so the
// caller must not try the prompt either.
func nameCodexThread(
	ctx context.Context,
	tmux Tmux,
	socket, target, name string,
	timings Timings,
	trace tracer,
) (named bool, warning string, blocked bool) {
	for attempt := 0; attempt < renameAttempts; attempt++ {
		trace.step("rename attempt %d/%d", attempt+1, renameAttempts)
		if !waitForCodexComposer(ctx, tmux, socket, target, timings, trace) {
			return false, "Codex is still holding a startup screen — nothing " +
				"was typed into it, so the chat is unnamed and unprompted; " +
				"attach it and clear the screen by hand", true
		}
		renamed, why := renameCodexThread(ctx, tmux, socket, target, name, timings, trace)
		if renamed {
			return true, "", false
		}
		warning = why
	}
	return false, warning, false
}

// renameCodexThread drives Codex's own rename UI and verifies each step before
// taking the next one. It reports the warning rather than an error: an unnamed
// chat is still a working chat.
func renameCodexThread(
	ctx context.Context,
	tmux Tmux,
	socket, target, name string,
	timings Timings,
	trace tracer,
) (bool, string) {
	if err := tmux.SendLiteral(ctx, socket, target, codexRenameCommand); err != nil {
		return false, fmt.Sprintf("could not type the rename command: %v", err)
	}
	trace.step("typed %s", codexRenameCommand)
	if !waitFor(ctx, tmux, socket, target, codexRenameOffered, timings) {
		capture, _ := tmux.Capture(ctx, socket, target)
		trace.step("no /rename offer | %s", screen(capture))
		leftover := clearComposer(
			ctx,
			tmux,
			socket,
			target,
			codexRenameCommand,
		)
		warning := "this Codex build did not offer /rename — the chat is running unnamed"
		if leftover {
			warning += "; its composer may still hold " + codexRenameCommand
		}
		return false, warning
	}
	if err := tmux.SendKey(ctx, socket, target, "Enter"); err != nil {
		return false, fmt.Sprintf("could not open the rename prompt: %v", err)
	}
	if !waitFor(ctx, tmux, socket, target, codexRenamePrompt, timings) {
		capture, _ := tmux.Capture(ctx, socket, target)
		trace.step("no name prompt | %s", screen(capture))
		_ = tmux.SendKey(ctx, socket, target, "Escape")
		return false, "Codex never asked for a thread name — the chat is running unnamed"
	}
	for index := 0; index < modalClearKeys; index++ {
		if err := tmux.SendKey(ctx, socket, target, "BSpace"); err != nil {
			return false, fmt.Sprintf("could not clear the name field: %v", err)
		}
	}
	if err := tmux.SendLiteral(ctx, socket, target, name); err != nil {
		return false, fmt.Sprintf("could not type the thread name: %v", err)
	}
	// Codex announces the rename in the transcript ("• Session renamed to X."),
	// so success is PROVEN, never inferred from a modal that merely closed.
	//
	// The Enter is re-sent while the modal stands: a TUI reading its input in
	// bursts can drop a confirmation that arrives glued to the text, and one
	// extra Enter on a modal that already closed lands on an empty composer,
	// where it does nothing.
	trace.step("typed the name")
	confirmed := false
	for press := 0; press < confirmPresses && !confirmed; press++ {
		if err := sleep(ctx, timings.Typed); err != nil {
			break
		}
		if err := tmux.SendKey(ctx, socket, target, "Enter"); err != nil {
			return false, fmt.Sprintf("could not confirm the thread name: %v", err)
		}
		confirmed = pollCapture(
			ctx,
			tmux,
			socket,
			target,
			Timings{Poll: timings.Poll, Step: confirmWait(timings)},
			renameLanded(name),
		)
		if !confirmed {
			trace.step("confirmation press %d did not take", press+1)
		}
	}
	if !confirmed {
		// Read the reason BEFORE dismissing the prompt: Escape takes the
		// modal — and the refusal printed inside it — off the screen.
		warning := "Codex never confirmed the rename — the chat may be unnamed"
		capture, _ := tmux.Capture(ctx, socket, target)
		if strings.Contains(capture, codexRenameEmpty) {
			warning = "Codex refused the name as empty — the chat is running unnamed"
		}
		trace.step("rename unconfirmed: %s | %s", warning, screen(capture))
		_ = tmux.SendKey(ctx, socket, target, "Escape")
		return false, warning
	}
	trace.step("rename confirmed")
	return true, ""
}

// renameLanded is the proof a rename took: Codex's own announcement, or the
// status line carrying the new name. The status line is the stronger of the
// two — it is still true a minute later, while an announcement scrolls away.
func renameLanded(name string) func(string) bool {
	return func(capture string) bool {
		return strings.Contains(capture, codexRenameDone) ||
			(composerReady(capture) && strings.Contains(capture, name+" · "))
	}
}

// confirmWait bounds one confirmation press, so a dropped Enter costs a
// fraction of the step budget instead of all of it.
func confirmWait(timings Timings) time.Duration {
	wait := timings.Step / confirmPresses
	if wait < timings.Poll*2 {
		wait = timings.Poll * 2
	}
	return wait
}

// typeLine sends text as literal keys and then Enter. The pause between them
// is what keeps a TUI from receiving the newline before it has processed the
// text — the same gap chat.sh leaves when it injects.
func typeLine(
	ctx context.Context,
	tmux Tmux,
	socket, target, text string,
	timings Timings,
) error {
	if err := tmux.SendLiteral(ctx, socket, target, text); err != nil {
		return err
	}
	if err := sleep(ctx, timings.Typed); err != nil {
		return err
	}
	return tmux.SendKey(ctx, socket, target, "Enter")
}

// clearComposer erases text this package typed but will not submit, so an
// abandoned "/rename" never rides along with the user's first real prompt.
//
// One BSpace per typed rune, not C-u: every composer implements backspace,
// while the kill-line binding belongs to the engine's own line editor and a
// build that ignores it would leave the text sitting there — the exact failure
// this function exists to prevent. It reports whether the text is STILL on
// screen afterwards, because a clear nobody verified is a guess.
func clearComposer(
	ctx context.Context,
	tmux Tmux,
	socket, target, typed string,
) bool {
	for range []rune(typed) {
		if err := tmux.SendKey(ctx, socket, target, "BSpace"); err != nil {
			return true
		}
	}
	capture, err := tmux.Capture(ctx, socket, target)
	if err != nil {
		return true
	}
	return strings.Contains(capture, typed)
}

func waitFor(
	ctx context.Context,
	tmux Tmux,
	socket, target, marker string,
	timings Timings,
) bool {
	return pollCapture(ctx, tmux, socket, target, timings, func(capture string) bool {
		return strings.Contains(capture, marker)
	})
}

func pollCapture(
	ctx context.Context,
	tmux Tmux,
	socket, target string,
	timings Timings,
	satisfied func(string) bool,
) bool {
	deadline := time.Now().Add(timings.Step)
	for {
		capture, err := tmux.Capture(ctx, socket, target)
		if err == nil && satisfied(capture) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		if err := sleep(ctx, timings.Poll); err != nil {
			return false
		}
	}
}

func sleep(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WindowName reduces a chat name to something tmux can carry as a window name:
// no colons or control characters (both break a target spec), one line, and
// short enough to stay readable in a status bar.
func WindowName(name string) string {
	var builder strings.Builder
	for _, character := range name {
		switch {
		case character == ':' || character == '.':
			builder.WriteByte('-')
		case character < ' ' || character == 0x7f:
			builder.WriteByte(' ')
		default:
			builder.WriteRune(character)
		}
	}
	cleaned := strings.Join(strings.Fields(builder.String()), " ")
	if cleaned == "" {
		return "chat"
	}
	return clipRunes(cleaned, 40)
}

func clipRunes(value string, limit int) string {
	count := 0
	for index := range value {
		if count == limit {
			return value[:index]
		}
		count++
	}
	return value
}
