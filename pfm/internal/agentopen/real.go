package agentopen

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"hostops/pfm/internal/action"
	"hostops/pfm/internal/policy"
)

// ExecCommands is the production command boundary. It strips inherited chat
// identity before every Claude invocation, exactly as the shell helper did.
type ExecCommands struct {
	Binary            string
	Home              string
	PromptPermissions bool
	Stdout            io.Writer
	Stderr            io.Writer
}

func (commands ExecCommands) binary() string {
	if commands.Binary != "" {
		return commands.Binary
	}
	return "claude"
}
func (commands ExecCommands) environment(config string, cache1H bool) []string {
	dropped := map[string]struct{}{
		"CLAUDE_CODE_SESSION_ID": {}, "CLAUDECODE": {}, "CLAUDE_CONFIG_DIR": {},
		"ENABLE_PROMPT_CACHING_1H": {}, "FORCE_PROMPT_CACHING_5M": {},
		"ANTHROPIC_BASE_URL": {}, "ANTHROPIC_AUTH_TOKEN": {}, "ANTHROPIC_MODEL": {},
		"ANTHROPIC_SMALL_FAST_MODEL": {}, "CLAUDE_CODE_AUTO_COMPACT_WINDOW": {},
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": {}, "CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK": {},
		"CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY": {},
	}
	env := make([]string, 0, len(os.Environ())+2)
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if _, ok := dropped[key]; !ok {
			env = append(env, value)
		}
	}
	if config != "" {
		env = append(env, "CLAUDE_CONFIG_DIR="+config)
	}
	if cache1H {
		env = append(env, "ENABLE_PROMPT_CACHING_1H=1")
	} else {
		env = append(env, "FORCE_PROMPT_CACHING_5M=1")
	}
	return env
}
func (commands ExecCommands) command(ctx context.Context, config string, cache1H bool, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, commands.binary(), args...)
	command.Env = commands.environment(config, cache1H)
	command.Stdout, command.Stderr = commands.Stdout, commands.Stderr
	return command
}
func (commands ExecCommands) QueryAgents(ctx context.Context, config string) ([]byte, error) {
	command := commands.command(ctx, config, false, "agents", "--json")
	// Output captures stdout for the registry parser; diagnostics still flow to
	// the caller's stderr through command.Stderr.
	command.Stdout = nil
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	return output, nil
}
func (commands ExecCommands) Resume(ctx context.Context, config, cwd, id string, cache1H bool) error {
	arguments := []string{"--resume", id}
	if !commands.PromptPermissions && policy.Autonomy(commands.Home) {
		arguments = append([]string{"--allow-dangerously-skip-permissions", "--dangerously-skip-permissions"}, arguments...)
	}
	command := commands.command(ctx, config, cache1H, arguments...)
	command.Dir = cwd
	return command.Run()
}
func (commands ExecCommands) View(ctx context.Context, config, cwd string) error {
	command := commands.command(ctx, config, false, "agents", "--cwd", cwd)
	return command.Run()
}

type RealProcesses struct{ Root string }

func (processes RealProcesses) Processes(ctx context.Context) ([]Process, error) {
	rows, err := (action.RealProcesses{Root: processes.Root}).Processes(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Process, 0, len(rows))
	for _, row := range rows {
		result = append(result, Process{PID: row.PID, Argv: row.Argv})
	}
	return result, nil
}
func (RealProcesses) Alive(pid int) bool { return pid > 0 && syscall.Kill(pid, 0) == nil }
func (RealProcesses) Terminate(pid int) error {
	err := syscall.Kill(pid, syscall.SIGTERM)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
func (RealProcesses) Kill(pid int) error {
	err := syscall.Kill(pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

type RealTmux struct {
	Binary string
	Dir    string
	Stderr io.Writer
}

func (tmux RealTmux) binary() string {
	if tmux.Binary != "" {
		return tmux.Binary
	}
	return "tmux"
}
func (tmux RealTmux) command(ctx context.Context, socket string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, tmux.binary(), append([]string{"-S", filepath.Join(tmux.Dir, socket)}, args...)...)
	command.Env = append(os.Environ(), "TMUX=")
	return command
}
func (tmux RealTmux) SocketForPID(ctx context.Context, pid int) (string, error) {
	entries, err := os.ReadDir(tmux.Dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "cc-") {
			continue
		}
		output, err := tmux.command(ctx, entry.Name(), "list-panes", "-a", "-F", "#{pane_pid}").Output()
		if err != nil {
			if tmux.Stderr != nil {
				fmt.Fprintf(tmux.Stderr, "pfm internal agent-open: probe tmux socket %s for pid %d: %v\n", entry.Name(), pid, err)
			}
			continue
		}
		for _, line := range strings.Split(string(output), "\n") {
			if strings.TrimSpace(line) == fmt.Sprint(pid) {
				return entry.Name(), nil
			}
		}
	}
	return "", nil
}
func (tmux RealTmux) Attach(ctx context.Context, socket string) error {
	if err := tmux.command(ctx, socket, "set-option", "-g", "window-size", "latest").Run(); err != nil {
		return fmt.Errorf("set tmux window size: %w", err)
	}
	return tmux.command(ctx, socket, "attach").Run()
}
