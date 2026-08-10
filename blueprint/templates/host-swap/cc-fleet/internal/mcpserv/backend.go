package mcpserv

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"hostops/cc-fleet/internal/compose"
	"hostops/cc-fleet/internal/gather"
	fleetindex "hostops/cc-fleet/internal/index"
	"hostops/cc-fleet/internal/inject"
	"hostops/cc-fleet/internal/naming"
	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/resolve"
	"hostops/cc-fleet/internal/store"
)

type injectionService interface {
	Resolve(context.Context, string) (inject.Target, int, string, error)
	Capture(context.Context, string, int) (inject.Target, string, int, string, error)
	Inject(context.Context, inject.Request) (inject.Result, error)
}

type backend struct {
	database *store.Store
	injector injectionService
	resolver resolve.Resolver
	paths    paths.Values
	indexMu  sync.Mutex
}

func newBackend(warnings io.Writer) (*backend, error) {
	database, err := store.Open(store.WithWarningWriter(warnings))
	if err != nil {
		return nil, err
	}
	resolver, err := resolve.New(nil)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	injector, err := inject.New(inject.Dependencies{Resolver: resolver})
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	resolved, err := paths.Resolve()
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	return &backend{
		database: database,
		injector: injector,
		resolver: *resolver,
		paths:    resolved,
	}, nil
}

func (current *backend) close() error {
	return current.database.Close()
}

func (current *backend) index(ctx context.Context) error {
	current.indexMu.Lock()
	defer current.indexMu.Unlock()
	indexer, err := fleetindex.New(current.database)
	if err != nil {
		return err
	}
	_, err = indexer.Run(ctx, fleetindex.Options{})
	return err
}

func (current *backend) list(ctx context.Context, input LSInput) (LSOutput, error) {
	if input.All && input.Hidden {
		return LSOutput{}, fmt.Errorf("all and hidden are mutually exclusive")
	}
	if err := current.index(ctx); err != nil {
		return LSOutput{}, err
	}
	transcripts, err := current.database.Transcripts(ctx)
	if err != nil {
		return LSOutput{}, err
	}
	rollouts, err := current.database.Rollouts(ctx)
	if err != nil {
		return LSOutput{}, err
	}
	cxNames, err := current.database.CxNames(ctx)
	if err != nil {
		return LSOutput{}, err
	}
	hidden, err := current.database.HiddenChats(ctx)
	if err != nil {
		return LSOutput{}, err
	}

	codexNamesByPath := make(map[string]string, len(rollouts))
	for _, rollout := range rollouts {
		codexNamesByPath[filepath.Clean(rollout.Path)] = naming.CxName(
			rollout.ID,
			rollout.SessionID,
			rollout.ParentThread,
			cxNames,
			rollout.FirstPrompt,
		)
	}
	tmuxClient := gather.CommandTmux{TmuxTmpDir: filepath.Dir(current.paths.TmuxDir)}
	gatherer, err := gather.New(gather.Dependencies{
		Tmux:       tmuxClient,
		TmuxTmpDir: filepath.Dir(current.paths.TmuxDir),
		CodexName: func(path string) string {
			return codexNamesByPath[filepath.Clean(path)]
		},
	})
	if err != nil {
		return LSOutput{}, err
	}
	snapshot, err := gatherer.Gather(ctx)
	if err != nil {
		return LSOutput{}, err
	}
	for _, rename := range snapshot.Renames {
		if err := tmuxClient.RenameWindow(ctx, rename); err == nil {
			for index := range snapshot.Panes {
				if snapshot.Panes[index].Socket == rename.Socket &&
					snapshot.Panes[index].WindowID == rename.WindowID {
					snapshot.Panes[index].WindowName = rename.TargetName
				}
			}
		}
	}

	view := compose.DefaultView
	if input.All {
		view = compose.AllView
	} else if input.Hidden {
		view = compose.HiddenView
	}
	cwd, err := os.Getwd()
	if err != nil {
		return LSOutput{}, err
	}
	output := compose.Compose(compose.Input{
		Snapshot:     snapshot,
		Transcripts:  transcripts,
		Rollouts:     rollouts,
		CxNames:      cxNames,
		Hidden:       hidden,
		AccountRoots: accountRoots(current.paths.ClaudeRoots),
		Options: compose.Options{
			View:           view,
			CurrentDir:     cwd,
			CurrentSocket:  filepath.Base(currentSocket()),
			PrimaryAccount: primaryAccount(current.paths.Home),
			CodexAvailable: codexAvailable(current.paths.CodexRoot),
			NowNS:          time.Now().UnixNano(),
		},
	})
	if err := current.applyComposeIntents(ctx, hidden, output); err != nil {
		return LSOutput{}, err
	}

	rows := make([]ChatRow, 0, len(output.Rows))
	filter := strings.ToLower(strings.TrimSpace(input.Project))
	for _, row := range output.Rows {
		if row.Kind == compose.NewClaude || row.Kind == compose.NewCodex {
			continue
		}
		if filter != "" &&
			!strings.Contains(strings.ToLower(row.Project), filter) &&
			!strings.Contains(strings.ToLower(row.CWD), filter) {
			continue
		}
		state := "idle"
		session := row.SessionName
		pane := row.PaneID
		if row.Kind == compose.LiveSplit {
			state = "idle"
			for _, candidate := range snapshot.Panes {
				if candidate.Socket != row.Socket {
					continue
				}
				if session == "" {
					session = candidate.SessionName
				}
				socketPath := filepath.Join(current.paths.TmuxDir, row.Socket)
				capture, captureErr := inject.CommandTmux{}.Capture(
					ctx,
					socketPath,
					candidate.PaneID,
					false,
					0,
				)
				if captureErr != nil {
					if state != "busy" {
						state = "dead"
					}
				} else if inject.IsBusy(capture) {
					state = "busy"
				}
			}
		} else if isLive(row.Kind) {
			socketPath := filepath.Join(current.paths.TmuxDir, row.Socket)
			capture, captureErr := inject.CommandTmux{}.Capture(
				ctx,
				socketPath,
				liveTarget(row),
				false,
				0,
			)
			if captureErr != nil {
				state = "dead"
			} else if inject.IsBusy(capture) {
				state = "busy"
			}
		} else {
			state = "resumable"
		}
		if session == "" {
			session = row.ID
		}
		account := row.Account
		if account == 0 && len(row.Accounts) > 0 {
			account = row.Accounts[0]
		}
		rows = append(rows, ChatRow{
			Session: session,
			ID:      row.ID,
			Engine:  engineForKind(row.Kind),
			State:   state,
			Dir:     row.CWD,
			Project: row.Project,
			Name:    row.Name,
			Account: account,
			Kind:    row.Kind.String(),
			Hidden:  row.Hidden,
			Socket:  row.Socket,
			Pane:    pane,
		})
	}
	return LSOutput{
		Rows:        rows,
		Count:       len(rows),
		HiddenCount: output.HiddenCount,
	}, nil
}

func (current *backend) applyComposeIntents(
	ctx context.Context,
	hidden []store.Hidden,
	output compose.Output,
) error {
	byID := make(map[string]store.Hidden, len(hidden))
	for _, row := range hidden {
		byID[row.ID] = row
	}
	for _, update := range output.BaselineUpdates {
		hiddenAt := time.Now().Unix()
		if existing, found := byID[update.ID]; found {
			hiddenAt = existing.HiddenAt
		}
		baseline := update.BaselinePrompts
		if err := current.database.Hide(ctx, store.Hidden{
			ID:              update.ID,
			Engine:          update.Engine,
			HiddenAt:        hiddenAt,
			BaselinePrompts: &baseline,
		}); err != nil {
			return err
		}
	}
	for _, id := range output.UnhideIDs {
		if err := current.database.Unhide(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func accountRoots(values []string) []compose.AccountRoot {
	roots := make([]compose.AccountRoot, 0, len(values))
	for index, value := range values {
		path := value
		if resolved, err := filepath.EvalSymlinks(value); err == nil {
			path = resolved
		} else if absolute, err := filepath.Abs(value); err == nil {
			path = absolute
		}
		roots = append(roots, compose.AccountRoot{
			Account: index%3 + 1,
			Path:    filepath.Clean(path),
		})
	}
	return roots
}

func primaryAccount(home string) int {
	content, err := os.ReadFile(filepath.Join(home, ".claude-primary"))
	if err != nil {
		return 1
	}
	account, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil || account < 1 || account > 3 {
		return 1
	}
	return account
}

func codexAvailable(root string) bool {
	switch os.Getenv("CC_FLEET_CODEX_AVAILABLE") {
	case "1":
		return true
	case "0":
		return false
	}
	if _, err := exec.LookPath("codex"); err == nil {
		return true
	}
	info, err := os.Stat(root)
	return err == nil && info.IsDir()
}

func currentSocket() string {
	value := os.Getenv("TMUX")
	if comma := strings.IndexByte(value, ','); comma >= 0 {
		value = value[:comma]
	}
	return value
}

func isLive(kind compose.Kind) bool {
	return kind == compose.LiveClaude ||
		kind == compose.LiveCodex ||
		kind == compose.LiveSplit ||
		kind == compose.Agent
}

func liveTarget(row compose.Row) string {
	if row.PaneID != "" {
		return row.PaneID
	}
	return row.SessionName
}

func engineForKind(kind compose.Kind) string {
	if kind == compose.LiveCodex || kind == compose.ResumeCodex {
		return "cx"
	}
	return "cc"
}
