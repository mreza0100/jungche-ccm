package reap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/tmuxfmt"
)

// Tmux is the reaper's whole tmux surface.
type Tmux interface {
	ListPanes(ctx context.Context, socket string) ([]gather.Pane, error)
	Sessions(ctx context.Context, socket string) ([]VSCTSession, error)
	KillSession(ctx context.Context, socket, session string) error
}

// CommandTmux talks to tmux inside one socket directory and nowhere else, so a
// jail's TMUX_TMPDIR is the whole world a test run can reach.
type CommandTmux struct {
	Binary  string
	TmuxDir string
	Now     func() time.Time
}

// ListPanes reads one socket's panes through the same probe the picker uses,
// so both halves of the fleet see one shape of a chat (K3).
func (tmux CommandTmux) ListPanes(
	ctx context.Context,
	socket string,
) ([]gather.Pane, error) {
	return gather.CommandTmux{
		Binary:     tmux.Binary,
		TmuxTmpDir: filepath.Dir(tmux.TmuxDir),
	}.ListPanes(ctx, socket)
}

// Sessions lists the sessions on a SHARED socket — the vsct bunker, where
// plain terminals live many-to-one rather than one server per chat.
func (tmux CommandTmux) Sessions(
	ctx context.Context,
	socket string,
) ([]VSCTSession, error) {
	format := strings.Join([]string{
		"#{session_name}",
		"#{?session_attached,1,0}",
		"#{session_activity}",
	}, "\x1f")
	output, err := tmux.command(
		ctx,
		socket,
		"list-sessions",
		"-F",
		format,
	).Output()
	if err != nil {
		// No bunker socket at all is the ordinary state on a machine that
		// never opened one; it is not a sweep failure.
		return nil, nil
	}
	now := tmux.Now
	if now == nil {
		now = time.Now
	}
	sessions := make([]VSCTSession, 0)
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := tmuxfmt.SplitN(line, 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf(
				"tmux %s returned %d session fields in %q",
				socket,
				len(fields),
				line,
			)
		}
		activity, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf(
				"tmux %s session activity %q: %w",
				socket,
				fields[2],
				err,
			)
		}
		idle := now().Sub(time.Unix(activity, 0))
		if idle < 0 {
			idle = 0
		}
		sessions = append(sessions, VSCTSession{
			Name:     fields[0],
			Attached: fields[1] == "1",
			Idle:     idle,
		})
	}
	return sessions, nil
}

// KillSession ends one session on a shared socket, leaving its neighbours up.
func (tmux CommandTmux) KillSession(
	ctx context.Context,
	socket, session string,
) error {
	output, err := tmux.command(
		ctx,
		socket,
		"kill-session",
		"-t",
		"="+session,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"kill tmux session %s on %s: %w: %s",
			session,
			socket,
			err,
			strings.TrimSpace(string(output)),
		)
	}
	return nil
}

func (tmux CommandTmux) command(
	ctx context.Context,
	socket string,
	arguments ...string,
) *exec.Cmd {
	binary := tmux.Binary
	if binary == "" {
		binary = "tmux"
	}
	commandArguments := []string{"-S", filepath.Join(tmux.TmuxDir, socket)}
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(ctx, binary, commandArguments...)
	command.Env = append(os.Environ(), "TMUX=")
	return command
}
