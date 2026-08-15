package seat

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"hostops/pfm/internal/action"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/spawn"
)

// tmuxLiteralChunkBytes stays well below tmux 3.5a's measured command-size
// ceiling. It remains the safe fallback for ordinary literal sends. The one
// large Dream brief uses PasteLiteral instead: rapid independent send-keys
// chunks can outrun the TUI and leave the tail out of a 20KB+ composer.
const tmuxLiteralChunkBytes = 4 << 10

// Host is the complete tmux effect boundary for a Dream seat. KillServer
// tears down the pane and the process group hosted by that private server.
type Host interface {
	spawn.Tmux
	PaneRootPID(context.Context, string, string) (int, error)
	SocketAlive(context.Context, string) bool
	KillServer(context.Context, string) error
}

// CommandHost composes the fleet's jailed spawn and lifecycle clients without
// adding another tmux implementation.
type CommandHost struct {
	spawn         spawn.CommandTmux
	action        action.CommandTmux
	binary        string
	tmuxDirectory string
}

func NewCommandHost(binary, tmuxDirectory string) CommandHost {
	return CommandHost{
		spawn:         spawn.CommandTmux{Binary: binary, TmuxDir: tmuxDirectory},
		action:        action.CommandTmux{Binary: binary, TmuxDir: tmuxDirectory},
		binary:        binary,
		tmuxDirectory: tmuxDirectory,
	}
}

// NewDefaultRunner wires the production, database-free boundaries. The path
// resolver computes names only; rollout discovery reads Codex files directly.
func NewDefaultRunner(events EventSink) (*Runner, error) {
	values, err := paths.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve seat host paths: %w", err)
	}
	locator := FilesystemRolloutLocator{CodexRoot: values.CodexRoot}
	runner := NewRunner(Dependencies{
		Commands:  ExecCommandRunner{},
		Host:      NewCommandHost("", values.TmuxDir),
		Processes: ProcTree{Root: values.ProcRoot},
		Jailer:    ProcJailer{Root: values.ProcRoot},
		Rollouts:  locator,
		Events:    events,
	})
	if os.Getenv("DREAM_SEAT_TRACE") == "1" {
		runner.spawnTrace = os.Stderr
	}
	return runner, nil
}

func (host CommandHost) NewSession(
	ctx context.Context,
	spec spawn.SessionSpec,
) error {
	return host.spawn.NewSession(ctx, spec)
}

func (host CommandHost) Capture(
	ctx context.Context,
	socket, target string,
) (string, error) {
	return host.spawn.Capture(ctx, socket, target)
}

func (host CommandHost) SendLiteral(
	ctx context.Context,
	socket, target, value string,
) error {
	for index, chunk := range literalChunks(value, tmuxLiteralChunkBytes) {
		if err := host.spawn.SendLiteral(ctx, socket, target, chunk); err != nil {
			return fmt.Errorf("send literal chunk %d: %w", index+1, err)
		}
	}
	return nil
}

func (host CommandHost) SendKey(
	ctx context.Context,
	socket, target, key string,
) error {
	return host.spawn.SendKey(ctx, socket, target, key)
}

// PasteLiteral transports one large brief through tmux's stdin-backed paste
// buffer. The text never appears in argv, -r preserves its linefeeds, and -p
// lets Codex's requested bracketed-paste mode make the entire brief one editor
// operation. The buffer is deleted by the successful paste.
func (host CommandHost) PasteLiteral(
	ctx context.Context,
	socket, target, value string,
) error {
	load := host.command(ctx, socket, "load-buffer", "-")
	load.Stdin = strings.NewReader(value)
	if output, err := load.CombinedOutput(); err != nil {
		return fmt.Errorf("load Dream brief into tmux buffer: %w: %s", err, output)
	}
	if output, err := host.command(
		ctx,
		socket,
		"paste-buffer", "-d", "-p", "-r", "-t", target,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("paste Dream brief into composer: %w: %s", err, output)
	}
	return nil
}

// PaneRootPID resolves the root of the exact pane addressed by this private
// socket and session. A multi-row response is not visibility: the gate would
// not know which process it had inspected, so it fails closed.
func (host CommandHost) PaneRootPID(
	ctx context.Context,
	socket, target string,
) (int, error) {
	command := host.command(
		ctx,
		socket,
		"list-panes", "-t", target, "-F", "#{pane_pid}",
	)
	output, err := command.Output()
	if err != nil {
		return 0, fmt.Errorf("resolve exact pane root process: %w", err)
	}
	rows := strings.Fields(string(output))
	if len(rows) != 1 {
		return 0, fmt.Errorf("resolve exact pane root process: tmux returned %d rows", len(rows))
	}
	pid, err := strconv.Atoi(rows[0])
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("resolve exact pane root process: invalid pid %q", rows[0])
	}
	return pid, nil
}

func (host CommandHost) command(
	ctx context.Context,
	socket string,
	arguments ...string,
) *exec.Cmd {
	binary := host.binary
	if binary == "" {
		binary = "tmux"
	}
	commandArguments := []string{"-S", filepath.Join(host.tmuxDirectory, socket)}
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(ctx, binary, commandArguments...)
	command.Env = append(os.Environ(), "TMUX=")
	return command
}

func (host CommandHost) SocketAlive(ctx context.Context, socket string) bool {
	return host.action.SocketAlive(ctx, socket)
}

func (host CommandHost) KillServer(ctx context.Context, socket string) error {
	return host.action.KillServer(ctx, socket)
}

func literalChunks(value string, limit int) []string {
	if value == "" {
		return []string{""}
	}
	if limit <= 0 {
		return []string{value}
	}
	chunks := make([]string, 0, len(value)/limit+1)
	for len(value) > limit {
		end := limit
		for end > 0 && !utf8.RuneStart(value[end]) {
			end--
		}
		if end == 0 {
			_, width := utf8.DecodeRuneInString(value)
			end = width
		}
		chunks = append(chunks, value[:end])
		value = value[end:]
	}
	if value != "" {
		chunks = append(chunks, value)
	}
	return chunks
}
