// Package reload reboots one Claude seat in its existing tmux pane.
package reload

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"hostops/pfm/internal/action"
	"hostops/pfm/internal/gather"
)

type Pane struct {
	ID          string
	Dead        bool
	CurrentPath string
	TTY         string
	PID         int
}

type Tmux interface {
	ListPanes(context.Context, string) ([]Pane, error)
	SetRemain(context.Context, string, string, bool) error
	PaneInMode(context.Context, string, string) (bool, error)
	CancelMode(context.Context, string, string) error
	Capture(context.Context, string, string) (string, error)
	SendKey(context.Context, string, string, string) error
	SendLiteral(context.Context, string, string, string) error
	Respawn(context.Context, string, string, string, string) error
	Display(context.Context, string, string, string) error
}

type Process interface {
	PIDs() ([]int, error)
	Cmdline(int) ([]string, error)
	Environ(int) (map[string]string, error)
	Stat(int) (gather.ProcStat, error)
}

type Request struct {
	SocketPath        string
	Pane              string
	PanePID           int
	SessionID         string
	Transcript        string
	CWD               string
	Account           int
	AccountIDs        []int
	AccountConfigDir  string
	AccountImplicit   bool
	ClaudeBinary      string
	PromptPermissions bool
	Cache1H           bool
	Then              string
}

type Options struct {
	Home        string
	SIDDir      string
	ClaudeRoots []string
	Delay       time.Duration
	Poll        time.Duration
	ExitTries   int
	ThenTries   int
}

type Result struct {
	Account int
	Cache1H bool
	Fresh   bool
}

func (o *Options) defaults() {
	if o.Delay < 0 {
		o.Delay = 0
	} else if o.Delay == 0 {
		o.Delay = 1500 * time.Millisecond
	}
	if o.Poll < 0 {
		o.Poll = 0
	} else if o.Poll == 0 {
		o.Poll = time.Second
	}
	if o.ExitTries == 0 {
		o.ExitTries = 20
	}
	if o.ThenTries == 0 {
		o.ThenTries = 900
	}
}

// Run performs the graceful in-place reboot. The caller has already resolved
// the target identity and account/cache birth values; this package owns every
// tmux mutation and the pane lock.
func Run(ctx context.Context, request Request, options Options, tmux Tmux, proc Process, stderr io.Writer) (Result, error) {
	options.defaults()
	if request.SocketPath == "" || request.Pane == "" {
		return Result{}, errors.New("reload requires a socket and pane")
	}
	if !rosterContains(request.AccountIDs, request.Account) {
		return Result{}, fmt.Errorf("account %d is not in the configured roster", request.Account)
	}
	if stderr == nil {
		stderr = io.Discard
	}
	lockPath := filepath.Join(options.SIDDir, "."+filepath.Base(request.SocketPath)+"."+request.Pane+".reloadlock")
	if err := os.MkdirAll(options.SIDDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create reload lock directory: %w", err)
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Result{}, fmt.Errorf("open reload lock: %w", err)
	}
	defer func() {
		if err := lock.Close(); err != nil {
			fmt.Fprintf(stderr, "pfm chat reload: close pane mutex: %v\n", err)
		}
	}()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return Result{}, errors.New("another reload of this pane is already in flight")
		}
		return Result{}, fmt.Errorf("lock reload pane: %w", err)
	}
	defer func() {
		if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
			fmt.Fprintf(stderr, "pfm chat reload: unlock pane mutex: %v\n", err)
		}
	}()
	if tmux == nil {
		return Result{}, errors.New("reload requires a tmux client")
	}

	if options.Delay > 0 {
		timer := time.NewTimer(options.Delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Result{}, ctx.Err()
		case <-timer.C:
		}
	}
	if err := tmux.SetRemain(ctx, request.SocketPath, request.Pane, true); err != nil {
		return Result{}, fmt.Errorf("set pane remain-on-exit: %w", err)
	}
	clearRemain := true
	defer func() {
		if clearRemain {
			if err := tmux.SetRemain(context.Background(), request.SocketPath, request.Pane, false); err != nil {
				fmt.Fprintf(stderr, "pfm chat reload: clear remain-on-exit after failure: %v\n", err)
			}
		}
	}()
	mode, err := tmux.PaneInMode(ctx, request.SocketPath, request.Pane)
	if err != nil {
		return Result{}, fmt.Errorf("read pane mode: %w", err)
	}
	if mode {
		if err := tmux.CancelMode(ctx, request.SocketPath, request.Pane); err != nil {
			return Result{}, fmt.Errorf("cancel pane mode: %w", err)
		}
	}
	cap, err := tmux.Capture(ctx, request.SocketPath, request.Pane)
	if err != nil {
		return Result{}, fmt.Errorf("capture pane before /exit: %w", err)
	}
	if selectorOpen(cap) {
		cause := errors.New("open selector menu on the pane — refusing to /exit")
		if displayErr := tmux.Display(ctx, request.SocketPath, request.Pane, "reload ABORTED — answer the open menu first, then reload again"); displayErr != nil {
			return Result{}, errors.Join(cause, fmt.Errorf("display selector refusal: %w", displayErr))
		}
		return Result{}, cause
	}
	if err := tmux.SendKey(ctx, request.SocketPath, request.Pane, "C-s"); err != nil {
		return Result{}, fmt.Errorf("stash pane draft: %w", err)
	}
	if err := tmux.SendLiteral(ctx, request.SocketPath, request.Pane, "/exit"); err != nil {
		return Result{}, fmt.Errorf("send /exit: %w", err)
	}
	if err := tmux.SendKey(ctx, request.SocketPath, request.Pane, "Enter"); err != nil {
		return Result{}, fmt.Errorf("submit /exit: %w", err)
	}

	dead := false
	empties := 0
	for i := 0; i < options.ExitTries; i++ {
		panes, listErr := tmux.ListPanes(ctx, request.SocketPath)
		if listErr != nil {
			return Result{}, fmt.Errorf("check pane exit state: %w", listErr)
		} else if len(panes) == 0 {
			empties++
			if empties >= 3 {
				dead = true
				break
			}
		} else {
			empties = 0
			for _, pane := range panes {
				if pane.ID == request.Pane && pane.Dead {
					dead = true
				}
			}
			if dead {
				break
			}
		}
		timer := time.NewTimer(options.Poll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Result{}, ctx.Err()
		case <-timer.C:
		}
	}
	if !dead {
		return Result{}, errors.New("/exit did not complete; chat left running")
	}
	run := claudeRun(request)
	if err := tmux.Respawn(ctx, request.SocketPath, request.Pane, request.CWD, run); err != nil {
		return Result{}, fmt.Errorf("respawn pane: %w", err)
	}
	if err := tmux.SetRemain(ctx, request.SocketPath, request.Pane, false); err != nil {
		return Result{}, fmt.Errorf("clear pane remain-on-exit: %w", err)
	}
	clearRemain = false
	if request.Then != "" {
		if request.Transcript != "" {
			if info, statErr := os.Stat(request.Transcript); statErr == nil {
				scaled := 90 + 30*int(info.Size()/1048576)
				if scaled > 900 {
					scaled = 900
				}
				if options.ThenTries == 900 {
					options.ThenTries = scaled
				}
			}
		}
		if err := deliverThen(ctx, request, options, tmux, proc, stderr); err != nil {
			return Result{}, errors.Join(
				err,
				failThen(ctx, request, options.SIDDir, tmux, err.Error()),
			)
		}
	}
	return Result{Account: request.Account, Cache1H: request.Cache1H, Fresh: request.SessionID == ""}, nil
}

func rosterContains(accounts []int, wanted int) bool {
	if len(accounts) == 0 {
		accounts = []int{1, 2, 3}
	}
	for _, account := range accounts {
		if account == wanted {
			return true
		}
	}
	return false
}

func selectorOpen(capture string) bool {
	selector := regexp.MustCompile(`❯[[:space:]]*[0-9]+\.`)
	for _, line := range strings.Split(capture, "\n") {
		if selector.MatchString(line) {
			return true
		}
	}
	return false
}

func claudeRun(request Request) string {
	parts := []string{"env", "-u", "CLAUDE_CODE_SESSION_ID", "-u", "CLAUDECODE", "-u", "ENABLE_PROMPT_CACHING_1H", "-u", "FORCE_PROMPT_CACHING_5M", "-u", "ANTHROPIC_BASE_URL", "-u", "ANTHROPIC_AUTH_TOKEN", "-u", "ANTHROPIC_MODEL", "-u", "ANTHROPIC_SMALL_FAST_MODEL", "-u", "CLAUDE_CODE_AUTO_COMPACT_WINDOW", "-u", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC", "-u", "CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK"}
	if !request.AccountImplicit && request.AccountConfigDir != "" {
		parts = append(parts, "CLAUDE_CONFIG_DIR="+action.Quote(request.AccountConfigDir))
	} else {
		parts = append(parts, "-u", "CLAUDE_CONFIG_DIR")
	}
	if request.Cache1H {
		parts = append(parts, "ENABLE_PROMPT_CACHING_1H=1")
	} else {
		parts = append(parts, "FORCE_PROMPT_CACHING_5M=1")
	}
	binary := request.ClaudeBinary
	if binary == "" {
		binary = "claude"
	}
	if binary != "claude" {
		binary = action.Quote(binary)
	}
	parts = append(parts, binary)
	if request.SessionID != "" {
		parts = append(parts, "--resume", action.Quote(request.SessionID))
	}
	if !request.PromptPermissions {
		parts = append(parts, "--allow-dangerously-skip-permissions", "--dangerously-skip-permissions")
	}
	return strings.Join(parts, " ")
}

func deliverThen(ctx context.Context, request Request, options Options, tmux Tmux, proc Process, stderr io.Writer) error {
	for i := 0; i < options.ThenTries; i++ {
		capture, err := tmux.Capture(ctx, request.SocketPath, request.Pane)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat reload --then: capture input box (try %d): %v\n", i+1, err)
		} else {
			trustPrompt := false
			for _, needle := range []string{"Trust this directory?", "trust this folder", "trust these settings"} {
				if strings.Contains(capture, needle) {
					if err := tmux.SendKey(ctx, request.SocketPath, request.Pane, "Enter"); err != nil {
						return fmt.Errorf("reload --then: accept trust prompt: %w", err)
					}
					trustPrompt = true
				}
			}
			if trustPrompt {
				continue
			}
			lines := strings.Split(capture, "\n")
			for j := len(lines) - 1; j >= 0 && j >= len(lines)-15; j-- {
				if strings.Contains(lines[j], "❯") {
					goto ready
				}
			}
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("reload --then: input box never appeared")
ready:
	// A respawn returns before the child necessarily reaches Claude. Prove the
	// process only after the input box appears; checking immediately after
	// respawn races normal startup and drops an otherwise deliverable baton.
	panePID, err := currentPanePID(ctx, request.SocketPath, request.Pane, tmux)
	if err != nil {
		return fmt.Errorf("reload --then: refresh reborn pane process: %w", err)
	}
	live, err := claudeLive(proc, panePID)
	if err != nil {
		return fmt.Errorf("reload --then: prove live Claude: %w", err)
	}
	if !live {
		return errors.New("reload --then: no live Claude on the pane")
	}
	if err := tmux.SendLiteral(ctx, request.SocketPath, request.Pane, request.Then); err != nil {
		return fmt.Errorf("reload --then: type prompt: %w", err)
	}
	flat := strings.Join(strings.Fields(request.Then), " ")
	needle := flat
	if len(needle) > 40 {
		needle = needle[len(needle)-40:]
	}
	typed := false
	for i := 0; i < 40; i++ {
		capture, err := tmux.Capture(ctx, request.SocketPath, request.Pane)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat reload --then: confirm typed text (try %d): %v\n", i+1, err)
			continue
		}
		if strings.Contains(strings.Join(strings.Fields(capture), " "), needle) {
			typed = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !typed {
		return errors.New("reload --then: typed text never rendered — refusing blind Enter")
	}
	prefix := flat
	if len(prefix) > 24 {
		prefix = prefix[:24]
	}
	for i := 0; i < 12; i++ {
		if err := tmux.SendKey(ctx, request.SocketPath, request.Pane, "Enter"); err != nil {
			return fmt.Errorf("reload --then: submit prompt: %w", err)
		}
		time.Sleep(150 * time.Millisecond)
		if err := tmux.SendKey(ctx, request.SocketPath, request.Pane, "Enter"); err != nil {
			return fmt.Errorf("reload --then: confirm prompt submit: %w", err)
		}
		time.Sleep(400 * time.Millisecond)
		capture, err := tmux.Capture(ctx, request.SocketPath, request.Pane)
		if err != nil {
			return fmt.Errorf("reload --then: verify prompt submit: %w", err)
		}
		// Submitted text remains in pane scrollback. Only the active composer
		// decides whether Enter cleared it; scanning the whole capture would
		// report every successful submission as still pending.
		composer := lastComposerLine(capture)
		if !strings.Contains(strings.Join(strings.Fields(composer), " "), prefix) {
			fmt.Fprintln(stderr, "then: follow-up delivered and submitted")
			return nil
		}
	}
	if err := tmux.Display(ctx, request.SocketPath, request.Pane, "reload --then typed but submit unconfirmed — press Enter"); err != nil {
		return fmt.Errorf("reload --then: display unconfirmed submit: %w", err)
	}
	return nil
}

func currentPanePID(ctx context.Context, socket, wanted string, tmux Tmux) (int, error) {
	panes, err := tmux.ListPanes(ctx, socket)
	if err != nil {
		return 0, err
	}
	for _, pane := range panes {
		if pane.ID != wanted {
			continue
		}
		if pane.Dead {
			return 0, errors.New("reborn pane is dead")
		}
		if pane.PID <= 0 {
			return 0, errors.New("reborn pane process id is unavailable")
		}
		return pane.PID, nil
	}
	return 0, errors.New("reborn pane disappeared")
}

func lastComposerLine(capture string) string {
	lines := strings.Split(capture, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if strings.Contains(lines[index], "❯") {
			return lines[index]
		}
	}
	return ""
}

func claudeLive(proc Process, panePID int) (bool, error) {
	if proc == nil {
		return false, errors.New("process reader is unavailable")
	}
	if panePID <= 0 {
		return false, errors.New("pane process id is unavailable")
	}
	pids, err := proc.PIDs()
	if err != nil {
		return false, err
	}
processes:
	for _, pid := range pids {
		argv, err := proc.Cmdline(pid)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return false, fmt.Errorf("read process %d command: %w", pid, err)
		}
		if !gather.IsClaudeCommand(argv) {
			continue
		}
		current := pid
		for depth := 0; depth <= 4; depth++ {
			if current == panePID {
				return true, nil
			}
			stat, statErr := proc.Stat(current)
			if statErr != nil {
				if errors.Is(statErr, fs.ErrNotExist) {
					continue processes
				}
				return false, fmt.Errorf("read process %d ancestry: %w", current, statErr)
			}
			if stat.ParentPID <= 1 || stat.ParentPID == current {
				break
			}
			current = stat.ParentPID
		}
	}
	return false, nil
}

func failThen(ctx context.Context, request Request, sidDir string, tmux Tmux, reason string) error {
	var failures []error
	if request.Then != "" && request.SocketPath != "" {
		path := filepath.Join(sidDir, filepath.Base(request.SocketPath)+".then-failed")
		if err := os.WriteFile(path, []byte(request.Then+"\n"), 0o600); err != nil {
			failures = append(failures, fmt.Errorf("write reload --then sentinel %q: %w", path, err))
		}
		if err := tmux.Display(ctx, request.SocketPath, request.Pane, "reload --then NOT delivered ("+reason+") — prompt saved"); err != nil {
			failures = append(failures, fmt.Errorf("display reload --then failure: %w", err))
		}
	}
	return errors.Join(failures...)
}

func TranscriptCWD(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open reload transcript %q: %w", path, err)
	}
	decoder := json.NewDecoder(file)
	cwd := ""
	for i := 0; i < 40; i++ {
		var row map[string]any
		if err := decoder.Decode(&row); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			_ = file.Close()
			return "", fmt.Errorf("decode reload transcript %q record %d: %w", path, i+1, err)
		}
		if value, ok := row["cwd"].(string); ok && value != "" {
			cwd = value
			break
		}
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close reload transcript %q: %w", path, err)
	}
	return cwd, nil
}

func SessionFromCrumb(sidDir, socket, pane string) (string, string, error) {
	for _, name := range []string{socket + "." + pane, socket} {
		path := filepath.Join(sidDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return "", "", fmt.Errorf("read reload breadcrumb %q: %w", path, err)
		}
		transcript := strings.TrimSpace(string(content))
		if transcript == "" {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(transcript), filepath.Ext(transcript))
		return id, transcript, nil
	}
	return "", "", nil
}

func ParseIntEnv(name string, fallback int) int {
	if value, err := strconv.Atoi(os.Getenv(name)); err == nil && value > 0 {
		return value
	}
	return fallback
}
