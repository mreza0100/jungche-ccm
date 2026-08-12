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
	if _, err := waitForBoot(ctx, tmux, request.Socket, target, timings); err != nil {
		return result, err
	}
	if request.Engine != store.CodexEngine {
		result.Prompted = request.Prompt != ""
		return result, nil
	}

	named, warning := renameCodexThread(
		ctx,
		tmux,
		request.Socket,
		target,
		request.Name,
		timings,
	)
	result.Named = named
	if warning != "" {
		result.Warnings = append(result.Warnings, warning)
	}
	if request.Prompt == "" {
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

// renameCodexThread drives Codex's own rename UI and verifies each step before
// taking the next one. It reports the warning rather than an error: an unnamed
// chat is still a working chat.
func renameCodexThread(
	ctx context.Context,
	tmux Tmux,
	socket, target, name string,
	timings Timings,
) (bool, string) {
	if err := tmux.SendLiteral(ctx, socket, target, codexRenameCommand); err != nil {
		return false, fmt.Sprintf("could not type the rename command: %v", err)
	}
	if !waitFor(ctx, tmux, socket, target, codexRenameOffered, timings) {
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
		_ = tmux.SendKey(ctx, socket, target, "Escape")
		return false, "Codex never asked for a thread name — the chat is running unnamed"
	}
	if err := typeLine(ctx, tmux, socket, target, name, timings); err != nil {
		return false, fmt.Sprintf("could not type the thread name: %v", err)
	}
	if !waitUntilGone(ctx, tmux, socket, target, codexRenamePrompt, timings) {
		// Read the reason BEFORE dismissing the prompt: Escape takes the
		// modal — and the refusal printed inside it — off the screen.
		warning := "Codex's rename prompt never closed — the chat may be unnamed"
		if capture, err := tmux.Capture(ctx, socket, target); err == nil &&
			strings.Contains(capture, codexRenameEmpty) {
			warning = "Codex refused the name as empty — the chat is running unnamed"
		}
		_ = tmux.SendKey(ctx, socket, target, "Escape")
		return false, warning
	}
	return true, ""
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

func waitUntilGone(
	ctx context.Context,
	tmux Tmux,
	socket, target, marker string,
	timings Timings,
) bool {
	return pollCapture(ctx, tmux, socket, target, timings, func(capture string) bool {
		return !strings.Contains(capture, marker)
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
