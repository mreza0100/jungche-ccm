package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/resolve"
	"hostops/pfm/internal/swap"
)

type swapCommandTmux struct{}

const swapUsage = "usage: pfm chat swap [1|2] [--then prompt] [--sock socket] [--1h on|off]"

var startSwapWorker = func(command *exec.Cmd) error {
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func (swapCommandTmux) command(ctx context.Context, socket string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "tmux", append([]string{"-S", socket}, args...)...)
	cmd.Env = append(os.Environ(), "TMUX=")
	return cmd
}

func (tmux swapCommandTmux) ListPanes(ctx context.Context, socket string) ([]swap.Pane, error) {
	format := strings.Join([]string{"#{pane_id}", "#{pane_dead}", "#{pane_current_path}", "#{pane_tty}", "#{pane_pid}"}, "\x1f")
	output, err := tmux.command(ctx, socket, "list-panes", "-a", "-F", format).Output()
	if err != nil {
		return nil, fmt.Errorf("list panes: %w", err)
	}
	rows := make([]swap.Pane, 0)
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		if line == "" {
			continue
		}
		// tmux renders the control separator in a format string as the
		// printable escape \037, keeping pane paths from introducing raw
		// line-protocol control bytes.
		fields := strings.SplitN(line, `\037`, 5)
		if len(fields) != 5 {
			return nil, fmt.Errorf("tmux returned %d pane fields", len(fields))
		}
		pid, err := strconv.Atoi(fields[4])
		if err != nil {
			return nil, fmt.Errorf("parse pane pid %q: %w", fields[4], err)
		}
		rows = append(rows, swap.Pane{ID: fields[0], Dead: fields[1] == "1", CurrentPath: fields[2], TTY: strings.TrimPrefix(fields[3], "/dev/"), PID: pid})
	}
	return rows, nil
}

func (tmux swapCommandTmux) SetRemain(ctx context.Context, socket, pane string, on bool) error {
	if on {
		return tmux.command(ctx, socket, "set-option", "-p", "-t", pane, "remain-on-exit", "on").Run()
	}
	return tmux.command(ctx, socket, "set-option", "-p", "-t", pane, "-u", "remain-on-exit").Run()
}
func (tmux swapCommandTmux) PaneInMode(ctx context.Context, socket, pane string) (bool, error) {
	out, err := tmux.command(ctx, socket, "display-message", "-p", "-t", pane, "#{pane_in_mode}").Output()
	return strings.TrimSpace(string(out)) == "1", err
}
func (tmux swapCommandTmux) CancelMode(ctx context.Context, socket, pane string) error {
	return tmux.command(ctx, socket, "send-keys", "-t", pane, "-X", "cancel").Run()
}
func (tmux swapCommandTmux) Capture(ctx context.Context, socket, pane string) (string, error) {
	out, err := tmux.command(ctx, socket, "capture-pane", "-t", pane, "-p", "-J", "-S", "-").Output()
	return string(out), err
}
func (tmux swapCommandTmux) SendKey(ctx context.Context, socket, pane, key string) error {
	return tmux.command(ctx, socket, "send-keys", "-t", pane, key).Run()
}
func (tmux swapCommandTmux) SendLiteral(ctx context.Context, socket, pane, text string) error {
	return tmux.command(ctx, socket, "send-keys", "-t", pane, "-l", "--", text).Run()
}
func (tmux swapCommandTmux) Respawn(ctx context.Context, socket, pane, cwd, command string) error {
	return tmux.command(ctx, socket, "respawn-pane", "-k", "-t", pane, "-c", cwd, command).Run()
}
func (tmux swapCommandTmux) Display(ctx context.Context, socket, pane, message string) error {
	return tmux.command(ctx, socket, "display-message", "-t", pane, message).Run()
}

func runChatSwap(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, swapUsage)
		return 0
	}
	if err := validateSwapArgs(args); err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: %v\n", err)
		return 2
	}
	resolved, err := paths.Resolve()
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: %v\n", err)
		return 1
	}
	tmux := swapCommandTmux{}
	socketPath, _, _, code := swapTarget(
		context.Background(), swapSocketArgument(args), resolved, tmux, stderr,
	)
	if code != 0 {
		return code
	}
	if err := os.MkdirAll(resolved.SIDDir, 0o700); err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: create worker log directory: %v\n", err)
		return 1
	}
	logPath := filepath.Join(
		resolved.SIDDir,
		"swap-"+filepath.Base(socketPath)+".log",
	)
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: open worker log: %v\n", err)
		return 1
	}
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Fprintf(stderr, "pfm chat swap: close worker log: %v\n", err)
		}
	}()
	null, err := os.Open(os.DevNull)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: open null input: %v\n", err)
		return 1
	}
	defer func() {
		if err := null.Close(); err != nil {
			fmt.Fprintf(stderr, "pfm chat swap: close null input: %v\n", err)
		}
	}()
	command := exec.Command(os.Args[0], append([]string{"internal", "swap-run"}, args...)...)
	command.Stdin = null
	command.Stdout = log
	command.Stderr = log
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := startSwapWorker(command); err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: schedule worker: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "pfm chat swap: swap scheduled in place (log %s)\n", logPath)
	return 0
}

func runChatSwapWorker(args []string, stdout, stderr io.Writer) int {
	if err := validateSwapArgs(args); err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: %v\n", err)
		return 2
	}
	var account, cacheOverride, sock, then string
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "1", "2":
			if account != "" {
				fmt.Fprintln(stderr, "pfm chat swap: account specified twice")
				return 2
			}
			account = args[index]
		case "--then":
			if index+1 >= len(args) {
				fmt.Fprintln(stderr, "pfm chat swap: --then needs a prompt")
				return 2
			}
			index++
			then = strings.NewReplacer("\n", " ", "\r", " ").Replace(args[index])
		case "--sock":
			if index+1 >= len(args) {
				fmt.Fprintln(stderr, "pfm chat swap: --sock needs a socket")
				return 2
			}
			index++
			sock = args[index]
		case "--1h":
			if index+1 >= len(args) || (args[index+1] != "on" && args[index+1] != "off" && args[index+1] != "1" && args[index+1] != "0") {
				fmt.Fprintln(stderr, "pfm chat swap: --1h needs on|off")
				return 2
			}
			index++
			cacheOverride = args[index]
		default:
			fmt.Fprintln(stderr, swapUsage)
			return 2
		}
	}
	if account == "" && cacheOverride == "" {
		fmt.Fprintln(stderr, swapUsage)
		return 2
	}
	resolved, err := paths.Resolve()
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: %v\n", err)
		return 1
	}
	tmux := swapCommandTmux{}
	socketPath, pane, paneState, code := swapTarget(context.Background(), sock, resolved, tmux, stderr)
	if code != 0 {
		return code
	}
	id, transcript, err := resolveSwapSession(resolved, socketPath, pane, sock == "")
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: %v\n", err)
		return 1
	}
	cwd, err := swap.TranscriptCWD(transcript)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: %v; using the live pane directory\n", err)
	}
	if cwd == "" {
		cwd = paneState.CurrentPath
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	if info, statErr := os.Stat(cwd); statErr != nil || !info.IsDir() {
		cwd, _ = os.Getwd()
	}
	birthAccount, birthCache := swapBirth(resolved, socketPath, paneState, stderr)
	if account == "" {
		account = strconv.Itoa(birthAccount)
		fmt.Fprintf(stdout, "pfm chat swap: no account given — keeping the chat's current account %s\n", account)
	}
	acct, _ := strconv.Atoi(account)
	cache := birthCache
	if cacheOverride != "" {
		cache = cacheOverride == "on" || cacheOverride == "1"
	}
	fmt.Fprintf(stdout, "pfm chat swap: swapping this chat to account %d IN PLACE — it reboots right here under the new account\n", acct)
	if then != "" {
		fmt.Fprintln(stdout, "pfm chat swap: --then queued — the follow-up is typed into the reborn chat once it reaches its prompt")
	}
	options := swap.Options{Home: resolved.Home, SIDDir: resolved.SIDDir, ClaudeRoots: resolved.ClaudeRoots, Delay: swapDurationEnv("PFM_SWAP_DELAY_MS", 1500), Poll: swapDurationEnv("PFM_SWAP_POLL_MS", 1000), ExitTries: swap.ParseIntEnv("PFM_SWAP_EXIT_TRIES", 20), ThenTries: swap.ParseIntEnv("PFM_SWAP_THEN_TRIES", 900)}
	result, err := swap.Run(context.Background(), swap.Request{SocketPath: socketPath, Pane: pane, PanePID: paneState.PID, SessionID: id, Transcript: transcript, CWD: cwd, Account: acct, Cache1H: cache, Then: then}, options, tmux, swapProc{root: resolved.ProcRoot}, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: %v\n", err)
		return 1
	}
	if result.Fresh {
		fmt.Fprintln(stdout, "pfm chat swap: no transcript yet — rebooted FRESH")
	} else {
		fmt.Fprintf(stdout, "pfm chat swap: respawned in place: %s %s\n", filepath.Base(socketPath), pane)
	}
	return 0
}

func swapSocketArgument(args []string) string {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == "--sock" {
			return args[index+1]
		}
	}
	return ""
}

func validateSwapArgs(args []string) error {
	account, cache := false, false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "1", "2":
			if account {
				return errors.New("account specified twice")
			}
			account = true
		case "--then", "--sock":
			if index+1 >= len(args) {
				return fmt.Errorf("%s needs a value", args[index])
			}
			index++
		case "--1h":
			if index+1 >= len(args) || (args[index+1] != "on" && args[index+1] != "off" && args[index+1] != "1" && args[index+1] != "0") {
				return errors.New("--1h needs on|off")
			}
			cache = true
			index++
		default:
			return errors.New(swapUsage)
		}
	}
	if !account && !cache {
		return errors.New(swapUsage)
	}
	return nil
}

func swapDurationEnv(name string, fallbackMS int) time.Duration {
	if raw, present := os.LookupEnv(name); present {
		if value, err := strconv.Atoi(raw); err == nil && value >= 0 {
			if value == 0 {
				return -1
			}
			return time.Duration(value) * time.Millisecond
		}
	}
	return time.Duration(fallbackMS) * time.Millisecond
}

func swapTarget(ctx context.Context, sock string, resolved paths.Values, tmux swap.Tmux, stderr io.Writer) (string, string, swap.Pane, int) {
	if sock != "" {
		path := sock
		if !filepath.IsAbs(path) {
			path = filepath.Join(resolved.TmuxDir, path)
		}
		panes, err := tmux.ListPanes(ctx, path)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat swap: no live server on %s: %v\n", sock, err)
			return "", "", swap.Pane{}, 1
		}
		if len(panes) == 0 {
			fmt.Fprintf(stderr, "pfm chat swap: no live panes on %s\n", sock)
			return "", "", swap.Pane{}, 1
		}
		if len(panes) != 1 {
			fmt.Fprintf(stderr, "pfm chat swap: %s has multiple panes — run swap inside the chat instead\n", sock)
			return "", "", swap.Pane{}, 1
		}
		return path, panes[0].ID, panes[0], 0
	}
	identifier, err := resolve.NewWhoami(resolve.WhoamiDependencies{})
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: %v\n", err)
		return "", "", swap.Pane{}, 1
	}
	identity, err := identifier.Identify(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: couldn't identify this chat: %v\n", err)
		return "", "", swap.Pane{}, 1
	}
	path := identity.SocketPath
	pane := identity.Pane
	if pane == "" {
		pane = os.Getenv("TMUX_PANE")
	}
	if pane == "" {
		fmt.Fprintln(stderr, "pfm chat swap: not in tmux")
		return "", "", swap.Pane{}, 1
	}
	panes, err := tmux.ListPanes(ctx, path)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: list panes: %v\n", err)
		return "", "", swap.Pane{}, 1
	}
	for _, item := range panes {
		if item.ID == pane {
			return path, pane, item, 0
		}
	}
	fmt.Fprintf(stderr, "pfm chat swap: pane %s is not live\n", pane)
	return "", "", swap.Pane{}, 1
}

type swapProc struct{ root string }

func (proc swapProc) PIDs() ([]int, error) { return (gather.RealProcFS{Root: proc.root}).PIDs() }
func (proc swapProc) Cmdline(pid int) ([]string, error) {
	return (gather.RealProcFS{Root: proc.root}).Cmdline(pid)
}
func (proc swapProc) Environ(pid int) (map[string]string, error) {
	return (gather.RealProcFS{Root: proc.root}).Environ(pid)
}
func (proc swapProc) Stat(pid int) (gather.ProcStat, error) {
	return (gather.RealProcFS{Root: proc.root}).Stat(pid)
}

func swapBirth(resolved paths.Values, socketPath string, pane swap.Pane, stderr io.Writer) (int, bool) {
	account, cache := 1, true
	proc := gather.RealProcFS{Root: resolved.ProcRoot}
	procTree := swapProc{root: resolved.ProcRoot}
	pids, err := proc.PIDs()
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat swap: inspect birth processes for %s: %v; using safe defaults\n", filepath.Base(socketPath), err)
		return account, cache
	}
	for _, pid := range pids {
		argv, err := proc.Cmdline(pid)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat swap: inspect process %d command: %v\n", pid, err)
			continue
		}
		if !gather.IsClaudeCommand(argv) {
			continue
		}
		inPane, err := swapProcessInPane(procTree, pid, pane.PID)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat swap: inspect process %d ancestry: %v\n", pid, err)
			continue
		}
		if !inPane {
			continue
		}
		env, err := proc.Environ(pid)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat swap: inspect process %d environment: %v\n", pid, err)
			continue
		}
		account = accountForConfig(resolved.Home, env["CLAUDE_CONFIG_DIR"])
		cache = env["FORCE_PROMPT_CACHING_5M"] != "1"
		return account, cache
	}
	// A tool shell can be detached from the seat's process tree. In that case
	// its own birth config is the only safe account rung for a cache-only swap.
	account = accountForConfig(resolved.Home, os.Getenv("CLAUDE_CONFIG_DIR"))
	return account, cache
}

func accountForConfig(home, config string) int {
	if config == "" {
		return 1
	}
	if resolved, err := filepath.EvalSymlinks(config); err == nil {
		config = resolved
	}
	if config == filepath.Join(home, ".cc", "2") || config == filepath.Join(home, ".claude3") {
		return 2
	}
	return 1
}
func swapProcessInPane(proc swapProc, pid, panePID int) (bool, error) {
	current := pid
	for depth := 0; depth <= 4; depth++ {
		if current == panePID {
			return true, nil
		}
		stat, err := proc.Stat(current)
		if err != nil {
			return false, err
		}
		if stat.ParentPID <= 1 || stat.ParentPID == current {
			break
		}
		current = stat.ParentPID
	}
	return false, nil
}

func resolveSwapSession(
	resolved paths.Values,
	socketPath, pane string,
	allowAmbient bool,
) (string, string, error) {
	id, crumbPath, err := swap.SessionFromCrumb(
		resolved.SIDDir,
		filepath.Base(socketPath),
		pane,
	)
	if err != nil {
		return "", "", err
	}
	transcript := ""
	if id != "" && !chatUUIDPattern.MatchString(id) {
		return "", "", fmt.Errorf("couldn't identify this chat from breadcrumb %q", crumbPath)
	}
	if crumbPath != "" {
		info, err := os.Stat(crumbPath)
		switch {
		case err == nil && info.Mode().IsRegular():
			transcript = crumbPath
		case err == nil:
			return "", "", fmt.Errorf("chat breadcrumb transcript is not a regular file: %s", crumbPath)
		case !errors.Is(err, fs.ErrNotExist):
			return "", "", fmt.Errorf("stat chat breadcrumb transcript %q: %w", crumbPath, err)
		}
	}
	if id == "" && allowAmbient {
		ambient := os.Getenv("CLAUDE_CODE_SESSION_ID")
		if chatUUIDPattern.MatchString(ambient) {
			path, err := findClaudeTranscript(resolved.ClaudeRoots, ambient)
			if err != nil {
				return "", "", err
			}
			if path != "" {
				id, transcript = ambient, path
			}
		}
	}
	if id == "" {
		return "", "", errors.New("couldn't identify this chat — run the statusline before swapping")
	}
	if transcript == "" {
		path, err := findClaudeTranscript(resolved.ClaudeRoots, id)
		if err != nil {
			return "", "", err
		}
		transcript = path
	}
	if transcript == "" {
		// A breadcrumb proves which chat this is even before its first
		// transcript record exists. Nothing can be resumed yet, so reboot it
		// fresh without discarding or borrowing another seat's identity.
		return "", "", nil
	}
	return id, transcript, nil
}

func findClaudeTranscript(roots []string, id string) (string, error) {
	for _, root := range roots {
		found := ""
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry != nil && !entry.IsDir() && filepath.Base(path) == id+".jsonl" {
				found = path
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("search Claude transcripts under %q: %w", root, err)
		}
		if found != "" {
			return found, nil
		}
	}
	return "", nil
}
