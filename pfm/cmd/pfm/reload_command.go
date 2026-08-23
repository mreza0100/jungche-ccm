package main

import (
	"context"
	"errors"
	"fmt"
	pfmengine "hostops/pfm/internal/engine"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/deps"
	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/reload"
	"hostops/pfm/internal/resolve"
	"hostops/pfm/internal/tmuxfmt"
)

type reloadCommandTmux struct{}

const reloadUsage = "usage: pfm chat reload [account] [--then prompt] [--sock socket] [--1h on|off]"

var startReloadWorker = func(command *exec.Cmd) error {
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func (reloadCommandTmux) command(ctx context.Context, socket string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, deps.Executable("tmux"), append([]string{"-S", socket}, args...)...)
	cmd.Env = append(os.Environ(), "TMUX=")
	return cmd
}

func (tmux reloadCommandTmux) ListPanes(ctx context.Context, socket string) ([]reload.Pane, error) {
	format := strings.Join([]string{"#{pane_id}", "#{pane_dead}", "#{pane_current_path}", "#{pane_tty}", "#{pane_pid}"}, "\x1f")
	output, err := tmux.command(ctx, socket, "list-panes", "-a", "-F", format).Output()
	if err != nil {
		return nil, fmt.Errorf("list panes: %w", err)
	}
	rows := make([]reload.Pane, 0)
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		if line == "" {
			continue
		}
		// Either spelling of the control separator: see internal/tmuxfmt.
		fields := tmuxfmt.SplitN(line, 5)
		if len(fields) != 5 {
			return nil, fmt.Errorf("tmux returned %d pane fields", len(fields))
		}
		pid, err := strconv.Atoi(fields[4])
		if err != nil {
			return nil, fmt.Errorf("parse pane pid %q: %w", fields[4], err)
		}
		rows = append(rows, reload.Pane{ID: fields[0], Dead: fields[1] == "1", CurrentPath: fields[2], TTY: strings.TrimPrefix(fields[3], "/dev/"), PID: pid})
	}
	return rows, nil
}

func (tmux reloadCommandTmux) SetRemain(ctx context.Context, socket, pane string, on bool) error {
	if on {
		return tmux.command(ctx, socket, "set-option", "-p", "-t", pane, "remain-on-exit", "on").Run()
	}
	return tmux.command(ctx, socket, "set-option", "-p", "-t", pane, "-u", "remain-on-exit").Run()
}
func (tmux reloadCommandTmux) PaneInMode(ctx context.Context, socket, pane string) (bool, error) {
	out, err := tmux.command(ctx, socket, "display-message", "-p", "-t", pane, "#{pane_in_mode}").Output()
	return strings.TrimSpace(string(out)) == "1", err
}
func (tmux reloadCommandTmux) CancelMode(ctx context.Context, socket, pane string) error {
	return tmux.command(ctx, socket, "send-keys", "-t", pane, "-X", "cancel").Run()
}
func (tmux reloadCommandTmux) Capture(ctx context.Context, socket, pane string) (string, error) {
	out, err := tmux.command(ctx, socket, "capture-pane", "-t", pane, "-p", "-J", "-S", "-").Output()
	return string(out), err
}
func (tmux reloadCommandTmux) SendKey(ctx context.Context, socket, pane, key string) error {
	return tmux.command(ctx, socket, "send-keys", "-t", pane, key).Run()
}
func (tmux reloadCommandTmux) SendLiteral(ctx context.Context, socket, pane, text string) error {
	return tmux.command(ctx, socket, "send-keys", "-t", pane, "-l", "--", text).Run()
}
func (tmux reloadCommandTmux) Respawn(ctx context.Context, socket, pane, cwd, command string) error {
	return tmux.command(ctx, socket, "respawn-pane", "-k", "-t", pane, "-c", cwd, command).Run()
}
func (tmux reloadCommandTmux) Display(ctx context.Context, socket, pane, message string) error {
	return tmux.command(ctx, socket, "display-message", "-t", pane, message).Run()
}

func runChatReload(args []string, stdout, stderr io.Writer) int {
	runtime, err := loadCommandRuntime("")
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: load config: %v\n", err)
		return 1
	}
	return runChatReloadWithRuntime(args, stdout, stderr, runtime)
}

func runChatReloadWithRuntime(
	args []string,
	stdout, stderr io.Writer,
	runtime commandRuntime,
) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, reloadUsage)
		return 0
	}
	if err := validateReloadArgs(args); err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: %v\n", err)
		return 2
	}
	resolved := runtime.Paths
	tmux := reloadCommandTmux{}
	socketPath, _, _, code := reloadTarget(
		context.Background(), reloadSocketArgument(args), resolved, tmux, stderr,
	)
	if code != 0 {
		return code
	}
	if err := os.MkdirAll(resolved.SIDDir, 0o700); err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: create worker log directory: %v\n", err)
		return 1
	}
	logPath := filepath.Join(
		resolved.SIDDir,
		"reload-"+filepath.Base(socketPath)+".log",
	)
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: open worker log: %v\n", err)
		return 1
	}
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Fprintf(stderr, "pfm chat reload: close worker log: %v\n", err)
		}
	}()
	null, err := os.Open(os.DevNull)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: open null input: %v\n", err)
		return 1
	}
	defer func() {
		if err := null.Close(); err != nil {
			fmt.Fprintf(stderr, "pfm chat reload: close null input: %v\n", err)
		}
	}()
	workerArgs := []string{"--config", runtime.Config.Path, "internal", "reload-run"}
	workerArgs = append(workerArgs, args...)
	command := exec.Command(os.Args[0], workerArgs...)
	command.Stdin = null
	command.Stdout = log
	command.Stderr = log
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := startReloadWorker(command); err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: schedule worker: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "pfm chat reload: reload scheduled in place (log %s)\n", logPath)
	return 0
}

func runChatReloadWorker(args []string, stdout, stderr io.Writer) int {
	runtime, err := loadCommandRuntime("")
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: load config: %v\n", err)
		return 1
	}
	return runChatReloadWorkerWithRuntime(args, stdout, stderr, runtime)
}

func runChatReloadWorkerWithRuntime(
	args []string,
	stdout, stderr io.Writer,
	runtime commandRuntime,
) int {
	if err := validateReloadArgs(args); err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: %v\n", err)
		return 2
	}
	var account, cacheOverride, sock, then string
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--then":
			if index+1 >= len(args) {
				fmt.Fprintln(stderr, "pfm chat reload: --then needs a prompt")
				return 2
			}
			index++
			then = strings.NewReplacer("\n", " ", "\r", " ").Replace(args[index])
		case "--sock":
			if index+1 >= len(args) {
				fmt.Fprintln(stderr, "pfm chat reload: --sock needs a socket")
				return 2
			}
			index++
			sock = args[index]
		case "--1h":
			if index+1 >= len(args) || (args[index+1] != "on" && args[index+1] != "off" && args[index+1] != "1" && args[index+1] != "0") {
				fmt.Fprintln(stderr, "pfm chat reload: --1h needs on|off")
				return 2
			}
			index++
			cacheOverride = args[index]
		default:
			if _, valid := positiveAccount(args[index]); !valid {
				fmt.Fprintln(stderr, reloadUsage)
				return 2
			}
			if account != "" {
				fmt.Fprintln(stderr, "pfm chat reload: account specified twice")
				return 2
			}
			account = args[index]
		}
	}
	if account == "" && cacheOverride == "" {
		fmt.Fprintln(stderr, reloadUsage)
		return 2
	}
	resolved := runtime.Paths
	tmux := reloadCommandTmux{}
	socketPath, pane, paneState, code := reloadTarget(context.Background(), sock, resolved, tmux, stderr)
	if code != 0 {
		return code
	}
	engine := reloadEngine(socketPath)
	id, transcript, err := resolveReloadSession(resolved, runtime.Config, socketPath, pane, sock == "")
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: %v\n", err)
		return 1
	}
	cwd, err := reload.TranscriptCWD(transcript)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: %v; using the live pane directory\n", err)
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
	birthAccount, birthCache, err := reloadBirth(resolved, runtime.Config, socketPath, paneState, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: %v\n", err)
		return 2
	}
	if account == "" {
		account = strconv.Itoa(birthAccount)
		fmt.Fprintf(stdout, "pfm chat reload: no account given — keeping the chat's current account %s\n", account)
	}
	acct, _ := strconv.Atoi(account)
	selected, err := validateReloadAccount(runtime.Config, engine, acct)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: %v\n", err)
		return 2
	}
	cache := birthCache
	if cacheOverride != "" {
		cache = cacheOverride == "on" || cacheOverride == "1"
	}
	fmt.Fprintf(stdout, "pfm chat reload: reloading this chat to account %d IN PLACE — it reboots right here under the new account\n", acct)
	if then != "" {
		fmt.Fprintln(stdout, "pfm chat reload: --then queued — the follow-up is typed into the reborn chat once it reaches its prompt")
	}
	options := reload.Options{Home: resolved.Home, SIDDir: resolved.SIDDir, ClaudeRoots: resolved.ClaudeRoots, Delay: reloadDurationEnv("PFM_RELOAD_DELAY_MS", 1500), Poll: reloadDurationEnv("PFM_RELOAD_POLL_MS", 1000), ExitTries: reload.ParseIntEnv("PFM_RELOAD_EXIT_TRIES", 20), ThenTries: reload.ParseIntEnv("PFM_RELOAD_THEN_TRIES", 900)}
	result, err := reload.Run(context.Background(), reload.Request{Engine: engine, SocketPath: socketPath, Pane: pane, PanePID: paneState.PID, SessionID: id, Transcript: transcript, CWD: cwd, Account: acct, AccountIDs: selected.IDs, AccountHome: selected.CodexHome, AccountConfigDir: selected.ClaudeConfigDir, AccountImplicit: selected.ClaudeImplicit, ClaudeBinary: selected.ClaudeBinary, CodexBinary: selected.CodexBinary, CodexYolo: selected.CodexYolo, PromptPermissions: selected.PromptPermissions, Cache1H: cache, Then: then}, options, tmux, reloadProc{procfs: gather.NewProcFS(resolved.ProcRoot)}, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: %v\n", err)
		return 1
	}
	if result.Fresh {
		fmt.Fprintln(stdout, "pfm chat reload: no transcript yet — rebooted FRESH")
	} else {
		fmt.Fprintf(stdout, "pfm chat reload: respawned in place: %s %s\n", filepath.Base(socketPath), pane)
	}
	return 0
}

func reloadSocketArgument(args []string) string {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == "--sock" {
			return args[index+1]
		}
	}
	return ""
}

func validateReloadArgs(args []string) error {
	account, cache := false, false
	for index := 0; index < len(args); index++ {
		switch args[index] {
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
			if _, valid := positiveAccount(args[index]); !valid {
				return errors.New(reloadUsage)
			}
			if account {
				return errors.New("account specified twice")
			}
			account = true
		}
	}
	if !account && !cache {
		return errors.New(reloadUsage)
	}
	return nil
}

func positiveAccount(value string) (int, bool) {
	account, err := strconv.Atoi(value)
	return account, err == nil && account > 0
}

func reloadRequestedAccount(args []string) int {
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--then", "--sock", "--1h":
			index++
			continue
		}
		if account, valid := positiveAccount(args[index]); valid {
			return account
		}
	}
	return 0
}

func reloadDurationEnv(name string, fallbackMS int) time.Duration {
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

func reloadTarget(ctx context.Context, sock string, resolved paths.Values, tmux reload.Tmux, stderr io.Writer) (string, string, reload.Pane, int) {
	if sock != "" {
		path := sock
		if !filepath.IsAbs(path) {
			path = filepath.Join(resolved.TmuxDir, path)
		}
		panes, err := tmux.ListPanes(ctx, path)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat reload: no live server on %s: %v\n", sock, err)
			return "", "", reload.Pane{}, 1
		}
		if len(panes) == 0 {
			fmt.Fprintf(stderr, "pfm chat reload: no live panes on %s\n", sock)
			return "", "", reload.Pane{}, 1
		}
		if len(panes) != 1 {
			fmt.Fprintf(stderr, "pfm chat reload: %s has multiple panes — run reload inside the chat instead\n", sock)
			return "", "", reload.Pane{}, 1
		}
		return path, panes[0].ID, panes[0], 0
	}
	identifier, err := resolve.NewWhoami(resolve.WhoamiDependencies{})
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: %v\n", err)
		return "", "", reload.Pane{}, 1
	}
	identity, err := identifier.Identify(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: couldn't identify this chat: %v\n", err)
		return "", "", reload.Pane{}, 1
	}
	path := identity.SocketPath
	pane := identity.Pane
	if pane == "" {
		pane = os.Getenv("TMUX_PANE")
	}
	if pane == "" {
		fmt.Fprintln(stderr, "pfm chat reload: not in tmux")
		return "", "", reload.Pane{}, 1
	}
	panes, err := tmux.ListPanes(ctx, path)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: list panes: %v\n", err)
		return "", "", reload.Pane{}, 1
	}
	for _, item := range panes {
		if item.ID == pane {
			return path, pane, item, 0
		}
	}
	fmt.Fprintf(stderr, "pfm chat reload: pane %s is not live\n", pane)
	return "", "", reload.Pane{}, 1
}

// reloadProc adapts the gathered process table to the tree walker. It holds ONE
// reader rather than building a fresh one per call: on macOS the reader carries
// a snapshot cache, and a per-call instance would resample the world each hop.
type reloadProc struct{ procfs gather.ProcFS }

func (proc reloadProc) PIDs() ([]int, error) { return proc.procfs.PIDs() }
func (proc reloadProc) Cmdline(pid int) ([]string, error) {
	return proc.procfs.Cmdline(pid)
}
func (proc reloadProc) Environ(pid int) (map[string]string, error) {
	return proc.procfs.Environ(pid)
}
func (proc reloadProc) Stat(pid int) (gather.ProcStat, error) {
	return proc.procfs.Stat(pid)
}

func reloadBirth(
	resolved paths.Values,
	machine pfmconfig.Config,
	socketPath string,
	pane reload.Pane,
	stderr io.Writer,
) (int, bool, error) {
	engine := reloadEngine(socketPath)
	ids := machine.AccountIDs()
	if engine == string(pfmengine.Codex) {
		ids = machine.CodexAccountIDs()
	}
	if len(ids) == 0 {
		return 0, false, fmt.Errorf("no %s accounts configured", reloadEngineLabel(engine))
	}
	account, cache := ids[0], true
	proc := gather.NewProcFS(resolved.ProcRoot)
	procTree := reloadProc{procfs: proc}
	pids, err := proc.PIDs()
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat reload: inspect birth processes for %s: %v; using safe defaults\n", filepath.Base(socketPath), err)
		return account, cache, nil
	}
	for _, pid := range pids {
		argv, err := proc.Cmdline(pid)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat reload: inspect process %d command: %v\n", pid, err)
			continue
		}
		matching := gather.IsClaudeCommand(argv, machine.Claude.Binary)
		if engine == string(pfmengine.Codex) {
			matching = gather.IsCodexCommand(argv, machine.Codex.Binary)
		}
		if !matching {
			continue
		}
		inPane, err := reloadProcessInPane(procTree, pid, pane.PID)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat reload: inspect process %d ancestry: %v\n", pid, err)
			continue
		}
		if !inPane {
			continue
		}
		env, err := proc.Environ(pid)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat reload: inspect process %d environment: %v\n", pid, err)
			continue
		}
		if engine == string(pfmengine.Codex) {
			account = accountForCodexHome(machine, env["CODEX_HOME"])
		} else {
			account = accountForConfig(machine, env["CLAUDE_CONFIG_DIR"])
			cache = env["FORCE_PROMPT_CACHING_5M"] != "1"
		}
		return account, cache, nil
	}
	// A tool shell can be detached from the seat's process tree. In that case
	// its own birth config is the only safe account rung for a cache-only reload.
	if engine == string(pfmengine.Codex) {
		account = accountForCodexHome(machine, os.Getenv("CODEX_HOME"))
	} else {
		account = accountForConfig(machine, os.Getenv("CLAUDE_CONFIG_DIR"))
	}
	return account, cache, nil
}

func reloadEngine(socketPath string) string {
	if strings.HasPrefix(filepath.Base(socketPath), "cx-") {
		return string(pfmengine.Codex)
	}
	return string(pfmengine.Claude)
}

func reloadEngineLabel(engine string) string {
	if engine == string(pfmengine.Codex) {
		return "Codex"
	}
	return "Claude"
}

func accountForConfig(machine pfmconfig.Config, config string) int {
	if len(machine.Accounts) == 0 {
		return 1
	}
	if config == "" {
		for _, account := range machine.Accounts {
			if account.Implicit {
				return account.ID
			}
		}
		return machine.Accounts[0].ID
	}
	if resolved, err := filepath.EvalSymlinks(config); err == nil {
		config = resolved
	}
	config = filepath.Clean(config)
	for _, account := range machine.Accounts {
		candidate := filepath.Clean(account.ConfigDir)
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
			candidate = resolved
		}
		if config == candidate {
			return account.ID
		}
	}
	return machine.Accounts[0].ID
}

func accountForCodexHome(machine pfmconfig.Config, home string) int {
	if len(machine.CodexAccounts) == 0 {
		return 0
	}
	cleaned := filepath.Clean(home)
	for _, account := range machine.CodexAccounts {
		if cleaned == filepath.Clean(account.Home) {
			return account.ID
		}
	}
	return machine.CodexAccounts[0].ID
}

type reloadAccountSelection struct {
	IDs               []int
	ClaudeConfigDir   string
	ClaudeImplicit    bool
	ClaudeBinary      string
	PromptPermissions bool
	CodexHome         string
	CodexBinary       string
	CodexYolo         bool
}

func validateReloadAccount(machine pfmconfig.Config, engine string, account int) (reloadAccountSelection, error) {
	if engine == string(pfmengine.Codex) || engine == "codex" {
		if len(machine.CodexAccounts) == 0 {
			return reloadAccountSelection{}, errors.New("no Codex accounts configured")
		}
		selected, found := machine.CodexAccountByID(account)
		if !found {
			return reloadAccountSelection{}, fmt.Errorf("Codex account %d is not in the configured roster", account)
		}
		policy := machine.EffectiveCodex(account)
		return reloadAccountSelection{
			IDs: machine.CodexAccountIDs(), CodexHome: selected.Home,
			CodexBinary: policy.Binary, CodexYolo: policy.Yolo,
		}, nil
	}
	if len(machine.Accounts) == 0 {
		return reloadAccountSelection{}, errors.New("no Claude accounts configured")
	}
	selected, found := machine.Account(account)
	if !found {
		return reloadAccountSelection{}, fmt.Errorf("Claude account %d is not in the configured roster", account)
	}
	policy := machine.EffectiveClaude(account)
	return reloadAccountSelection{
		IDs: machine.AccountIDs(), ClaudeConfigDir: selected.ConfigDir,
		ClaudeImplicit: selected.Implicit, ClaudeBinary: policy.Binary,
		PromptPermissions: policy.PermissionMode == pfmconfig.PermissionPrompt,
	}, nil
}
func reloadProcessInPane(proc reloadProc, pid, panePID int) (bool, error) {
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

func resolveReloadSession(
	resolved paths.Values,
	machine pfmconfig.Config,
	socketPath, pane string,
	allowAmbient bool,
) (string, string, error) {
	id, crumbPath, err := reload.SessionFromCrumb(
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
	engine := reloadEngine(socketPath)
	if id == "" && allowAmbient {
		ambient := os.Getenv("CLAUDE_CODE_SESSION_ID")
		if engine == string(pfmengine.Codex) {
			ambient = os.Getenv("CODEX_THREAD_ID")
		}
		if chatUUIDPattern.MatchString(ambient) {
			path, err := findEngineTranscript(resolved, machine, engine, ambient)
			if err != nil {
				return "", "", err
			}
			if path != "" {
				id, transcript = ambient, path
			}
		}
	}
	if id == "" {
		return "", "", errors.New("couldn't identify this chat — run the statusline before reloading")
	}
	if transcript == "" {
		path, err := findEngineTranscript(resolved, machine, engine, id)
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

func findEngineTranscript(resolved paths.Values, machine pfmconfig.Config, engine, id string) (string, error) {
	if engine != string(pfmengine.Codex) {
		return findClaudeTranscript(resolved.ClaudeRoots, id)
	}
	for _, account := range machine.CodexAccounts {
		found := ""
		err := filepath.WalkDir(filepath.Join(account.Home, "sessions"), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry != nil && !entry.IsDir() && strings.Contains(filepath.Base(path), id) && filepath.Ext(path) == ".jsonl" {
				found = path
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("search Codex rollouts under %q: %w", account.Home, err)
		}
		if found != "" {
			return found, nil
		}
	}
	return "", nil
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
