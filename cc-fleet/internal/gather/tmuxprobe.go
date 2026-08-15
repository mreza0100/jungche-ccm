package gather

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// TmuxClient probes one named tmux socket.
type TmuxClient interface {
	ListPanes(ctx context.Context, socket string) ([]Pane, error)
}

// PaneCapturer is the optional TmuxClient extension that reads a pane's
// visible screen. Only the claude half of window-name convergence needs it —
// the 🔖 label lives on the statusline and nowhere else — so a client that
// cannot capture stays usable for every other probe, and the convergence
// simply plans no claude rename rather than planning a wrong one.
type PaneCapturer interface {
	CapturePane(ctx context.Context, socket, paneID string) (string, error)
}

// ErrServerGone marks a probe that failed because the socket has no server
// behind it — a chat that ended. It is the most ordinary outcome there is: the
// socket file outlives the server, so every pass finds leftovers. Reporting it
// as a probe warning buried the anomalies that matter (a missing tmux binary, a
// permission failure) under a wall of "exit status 1" after every picker close.
var ErrServerGone = errors.New("no tmux server on socket")

// serverGone reads tmux's own words for a socket with nothing behind it. The
// exit status alone cannot say it — tmux exits 1 for every failure and writes
// the reason to stderr, which exec keeps on the ExitError rather than in the
// error message.
func serverGone(err error) bool {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return false
	}
	stderr := string(exit.Stderr)
	return strings.Contains(stderr, "no server running") ||
		strings.Contains(stderr, "error connecting to") ||
		strings.Contains(stderr, "No such file or directory")
}

// CommandTmux invokes a tmux binary inside a caller-supplied TMUX_TMPDIR.
type CommandTmux struct {
	Binary     string
	TmuxTmpDir string
}

// ListPanes performs one list-panes -a call for socket.
func (tmux CommandTmux) ListPanes(ctx context.Context, socket string) ([]Pane, error) {
	binary := tmux.Binary
	if binary == "" {
		binary = "tmux"
	}
	// pane_current_path is asked for WHOLE, never through tmux's b: basename
	// modifier. Pane.CurrentPath is a directory: it becomes a row's CWD, and it
	// is the directory the Codex thread resolver matches against the state
	// store's absolute threads.cwd. A basename matched nothing there, so every
	// live Codex session that holds no rollout descriptor — the normal shape
	// since Codex 0.146.1 — silently lost its live row. The project name is
	// derived in Go (compose.projectName), where the full path is still there
	// to derive it from.
	format := strings.Join([]string{
		"#{session_name}",
		"#{window_id}",
		"#{window_name}",
		"#{pane_title}",
		"#{pane_tty}",
		"#{pane_pid}",
		"#{pane_id}",
		"#{?session_attached,1,0}",
		"#{pane_current_path}",
		"#{pane_current_command}",
	}, "\x1f")
	command := exec.CommandContext(
		ctx,
		binary,
		"-L",
		socket,
		"list-panes",
		"-a",
		"-F",
		format,
	)
	command.Env = append(
		os.Environ(),
		"TMUX=",
		"TMUX_TMPDIR="+tmux.TmuxTmpDir,
	)
	output, err := command.Output()
	if err != nil {
		// Match the legacy probe's older field set as a compatibility fallback.
		// Some long-lived servers reject newer format fields while remaining
		// fully attachable.
		legacyFormat := strings.Join([]string{
			"#{session_name}",
			"#{s/^[^ ]* //:pane_title}",
			"#{pane_current_path}",
			"#{session_windows}",
			"#{?session_attached,1,0}",
			"#{session_created}",
			"#{pane_id}",
			"#{pane_tty}",
			"#{window_name}",
			"#{pane_pid}",
		}, "\t")
		legacyCommand := exec.CommandContext(
			ctx,
			binary,
			"-L",
			socket,
			"list-panes",
			"-a",
			"-F",
			legacyFormat,
		)
		legacyCommand.Env = command.Env
		legacyOutput, legacyErr := legacyCommand.Output()
		if legacyErr != nil {
			// Both probes agree there is nothing behind the socket: say so in a
			// form the caller can classify, since the bare ExitError reads only
			// "exit status 1".
			if serverGone(err) && serverGone(legacyErr) {
				return nil, fmt.Errorf("%w: %s", ErrServerGone, socket)
			}
			return nil, err
		}
		return parseLegacyPaneOutput(socket, legacyOutput)
	}

	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	panes := make([]Pane, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		// tmux renders the control separator in a format string as the
		// printable escape \037, keeping pane titles from introducing raw
		// line-protocol control bytes.
		fields := strings.SplitN(line, `\037`, 10)
		if len(fields) != 10 {
			return nil, fmt.Errorf(
				"tmux %s returned %d pane fields in %q",
				socket,
				len(fields),
				line,
			)
		}
		pid, err := strconv.Atoi(fields[5])
		if err != nil {
			return nil, fmt.Errorf("tmux %s pane pid %q: %w", socket, fields[5], err)
		}
		attached, err := strconv.ParseBool(fields[7])
		if err != nil {
			return nil, fmt.Errorf(
				"tmux %s attached flag %q: %w",
				socket,
				fields[7],
				err,
			)
		}
		panes = append(panes, Pane{
			Socket:         socket,
			SessionName:    fields[0],
			WindowID:       fields[1],
			WindowName:     fields[2],
			PaneTitle:      fields[3],
			CurrentPath:    fields[8],
			CurrentCommand: fields[9],
			TTY:            fields[4],
			PID:            pid,
			PaneID:         fields[6],
			Attached:       attached,
		})
	}
	return panes, nil
}

func parseLegacyPaneOutput(socket string, output []byte) ([]Pane, error) {
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	panes := make([]Pane, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 10)
		if len(fields) != 10 {
			return nil, fmt.Errorf(
				"tmux %s returned %d legacy pane fields in %q",
				socket,
				len(fields),
				line,
			)
		}
		attached, err := strconv.ParseBool(fields[4])
		if err != nil {
			return nil, fmt.Errorf(
				"tmux %s legacy attached flag %q: %w",
				socket,
				fields[4],
				err,
			)
		}
		pid, err := strconv.Atoi(fields[9])
		if err != nil {
			return nil, fmt.Errorf(
				"tmux %s legacy pane pid %q: %w",
				socket,
				fields[9],
				err,
			)
		}
		panes = append(panes, Pane{
			Socket:      socket,
			SessionName: fields[0],
			PaneTitle:   fields[1],
			CurrentPath: fields[2],
			PaneID:      fields[6],
			TTY:         fields[7],
			WindowName:  fields[8],
			PID:         pid,
			Attached:    attached,
		})
	}
	return panes, nil
}

// CapturePane returns one pane's visible screen inside the same jailed tmux
// namespace used by ListPanes. Joined wrapped lines (-J) match how the label
// resolver reads a statusline, so a label wrapped by a narrow pane still
// reads whole.
func (tmux CommandTmux) CapturePane(
	ctx context.Context,
	socket, paneID string,
) (string, error) {
	binary := tmux.Binary
	if binary == "" {
		binary = "tmux"
	}
	command := exec.CommandContext(
		ctx,
		binary,
		"-L",
		socket,
		"capture-pane",
		"-t",
		paneID,
		"-p",
		"-J",
	)
	command.Env = append(
		os.Environ(),
		"TMUX=",
		"TMUX_TMPDIR="+tmux.TmuxTmpDir,
	)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf(
			"capture tmux pane %s %s: %w",
			socket,
			paneID,
			err,
		)
	}
	return string(output), nil
}

// RenameWindow applies one indexed Codex window-name convergence inside the
// same jailed tmux namespace used by ListPanes.
func (tmux CommandTmux) RenameWindow(
	ctx context.Context,
	rename WindowRename,
) error {
	binary := tmux.Binary
	if binary == "" {
		binary = "tmux"
	}
	command := exec.CommandContext(
		ctx,
		binary,
		"-L",
		rename.Socket,
		"rename-window",
		"-t",
		rename.WindowID,
		rename.TargetName,
	)
	command.Env = append(
		os.Environ(),
		"TMUX=",
		"TMUX_TMPDIR="+tmux.TmuxTmpDir,
	)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"rename tmux window %s %s to %q: %w: %s",
			rename.Socket,
			rename.WindowID,
			rename.TargetName,
			err,
			output,
		)
	}
	return nil
}

// ProbeTmux enumerates chat sockets and probes each server concurrently.
func ProbeTmux(
	ctx context.Context,
	tmuxDir string,
	client TmuxClient,
	now time.Time,
) (TmuxProbe, error) {
	return probeTmux(ctx, tmuxDir, client, now, true)
}

// ProbeTmuxReadOnly performs the same probes without removing old corpse
// sockets. It is used by shadow comparison, where observation must not heal
// the namespace being compared.
func ProbeTmuxReadOnly(
	ctx context.Context,
	tmuxDir string,
	client TmuxClient,
	now time.Time,
) (TmuxProbe, error) {
	return probeTmux(ctx, tmuxDir, client, now, false)
}

func probeTmux(
	ctx context.Context,
	tmuxDir string,
	client TmuxClient,
	now time.Time,
	sweep bool,
) (TmuxProbe, error) {
	var result TmuxProbe
	entries, err := os.ReadDir(tmuxDir)
	if errors.Is(err, fs.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("read tmux socket directory: %w", err)
	}

	type socketFile struct {
		name string
		path string
	}
	sockets := make([]socketFile, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !isChatSocketName(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSocket == 0 {
			continue
		}
		sockets = append(sockets, socketFile{
			name: name,
			path: filepath.Join(tmuxDir, name),
		})
	}
	sort.Slice(sockets, func(left, right int) bool {
		return sockets[left].name < sockets[right].name
	})

	var mutex sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	for _, socket := range sockets {
		socket := socket
		group.Go(func() error {
			panes, err := client.ListPanes(groupCtx, socket.name)
			if err != nil && groupCtx.Err() == nil {
				// A server can rotate its active window between tmux accepting
				// the client and list-panes. One immediate read-only retry
				// avoids turning that transient into missing live rows.
				panes, err = client.ListPanes(groupCtx, socket.name)
			}
			if err == nil {
				mutex.Lock()
				result.Panes = append(result.Panes, panes...)
				mutex.Unlock()
				return nil
			}
			if groupCtx.Err() != nil {
				return groupCtx.Err()
			}

			// A socket with no server behind it is a chat that ended, not a
			// fault — it is swept below and stays silent. Everything else is a
			// real anomaly and still gets said out loud.
			if !errors.Is(err, ErrServerGone) {
				mutex.Lock()
				result.ProbeWarnings = append(
					result.ProbeWarnings,
					fmt.Sprintf("%s: %v", socket.name, err),
				)
				mutex.Unlock()
			}
			info, statErr := os.Stat(socket.path)
			if sweep && statErr == nil && now.Sub(info.ModTime()) > time.Hour {
				if removeErr := os.Remove(socket.path); removeErr == nil ||
					errors.Is(removeErr, fs.ErrNotExist) {
					mutex.Lock()
					result.CorpseSwept = append(result.CorpseSwept, socket.name)
					mutex.Unlock()
				}
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return TmuxProbe{}, err
	}

	sort.Slice(result.Panes, func(left, right int) bool {
		if result.Panes[left].Socket != result.Panes[right].Socket {
			return result.Panes[left].Socket < result.Panes[right].Socket
		}
		return result.Panes[left].PaneID < result.Panes[right].PaneID
	})
	sort.Strings(result.CorpseSwept)
	sort.Strings(result.ProbeWarnings)
	return result, nil
}

// IsChatSocketName reports whether a tmux socket belongs to the chat fleet.
// It is the single answer to that question (K3): the picker's probe and the
// reaper's sweep must agree on which sockets are chats, or one of them acts on
// a socket the other cannot see.
func IsChatSocketName(name string) bool {
	if strings.HasPrefix(name, "vsct") || strings.HasPrefix(name, "revive") {
		return false
	}
	return strings.HasPrefix(name, "cc-") || strings.HasPrefix(name, "cx-")
}

func isChatSocketName(name string) bool {
	return IsChatSocketName(name)
}
