package mcpserv

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode"

	"hostops/pfm/internal/compose"
	pfmconfig "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/inject"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/resolve"
	"hostops/pfm/internal/store"
)

type injectionService interface {
	Resolve(context.Context, string) (inject.Target, int, string, error)
	Capture(context.Context, string, int) (inject.Target, string, int, string, error)
	Inject(context.Context, inject.Request) (inject.Result, error)
	ScheduleAfterCurrentTurn(context.Context, inject.Request) (inject.Result, error)
}

type backend struct {
	database             *store.Store
	injector             injectionService
	resolver             resolve.Resolver
	operations           SharedOperations
	dispatch             Dispatch
	paths                paths.Values
	allowAmbientIdentity bool
}

func newBackendConfigured(warnings io.Writer, runtime Runtime) (*backend, error) {
	if len(runtime.Accounts) == 0 {
		machine := pfmconfig.Defaults(runtime.Paths.Home, runtime.Paths.Roots[pfmengine.Claude])
		runtime.Accounts = machine.Accounts
	}
	if runtime.CodexBinary == "" {
		runtime.CodexBinary = pfmengine.MustLookup(pfmengine.Codex).Binary
	}
	if runtime.ClaudeBinary == "" {
		runtime.ClaudeBinary = pfmengine.MustLookup(pfmengine.Claude).Binary
	}
	if runtime.OpencodeBinary == "" {
		runtime.OpencodeBinary = pfmengine.MustLookup(pfmengine.Opencode).Binary
	}
	database, err := store.Open(store.WithWarningWriter(warnings))
	if err != nil {
		return nil, err
	}
	resolver, err := resolve.New(nil, resolve.Binaries{
		Values: map[pfmengine.ID]string{
			pfmengine.Claude: runtime.ClaudeBinary,
			pfmengine.Codex:  runtime.CodexBinary,
		},
		AccountEmojis: accountEmojis(runtime.Accounts),
	})
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	injector, err := inject.New(inject.Dependencies{
		Resolver:       resolver,
		Spawner:        inject.CommandThenSpawner{ConfigPath: runtime.ConfigPath},
		ClaudeBinary:   runtime.ClaudeBinary,
		CodexBinary:    runtime.CodexBinary,
		OpencodeBinary: runtime.OpencodeBinary,
		AccountEmojis:  accountEmojis(runtime.Accounts),
	})
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	return &backend{
		database:             database,
		injector:             injector,
		resolver:             *resolver,
		operations:           runtime.Operations,
		dispatch:             runtime.Dispatch,
		paths:                runtime.Paths,
		allowAmbientIdentity: runtime.AllowAmbientIdentity,
	}, nil
}

func (current *backend) close() error {
	return current.database.Close()
}

func accountEmojis(accounts []pfmconfig.Account) []string {
	result := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if account.Emoji != "" && account.Emoji != "·" {
			result = append(result, account.Emoji)
		}
	}
	return result
}

func (current *backend) list(ctx context.Context, input LSInput) (LSOutput, error) {
	if current.operations.List == nil {
		return LSOutput{}, fmt.Errorf("chat_ls shared CLI operation is not configured")
	}
	return current.operations.List(ctx, input)
}

type callerIdentity struct {
	identity resolve.Identity
	row      ChatRow
	present  bool
	valid    bool
	detail   string
}

func (current *backend) callerForRequest(
	ctx context.Context,
	meta map[string]any,
) (callerIdentity, error) {
	raw, present := meta["threadId"]
	if !present {
		return callerIdentity{}, nil
	}
	caller := callerIdentity{present: true}
	threadID, ok := raw.(string)
	if !ok || strings.TrimSpace(threadID) == "" ||
		strings.TrimSpace(threadID) != threadID || containsControl(threadID) {
		caller.detail = "MCP _meta.threadId must be a non-empty thread id without whitespace or control characters"
		return caller, nil
	}
	listed, err := current.list(ctx, LSInput{All: true})
	if err != nil {
		return caller, fmt.Errorf("resolve MCP caller thread %q: list live chats: %w", threadID, err)
	}
	var match ChatRow
	count := 0
	for _, row := range listed.Rows {
		if row.ID != threadID || row.Engine != pfmengine.Codex || row.Killed ||
			row.Kind != compose.LiveCodex.String() ||
			row.Session == "" || row.Socket == "" || row.Pane == "" {
			continue
		}
		match = row
		count++
	}
	if count == 0 {
		caller.detail = fmt.Sprintf("MCP _meta.threadId %q has no live Codex tmux seat", threadID)
		return caller, nil
	}
	if count != 1 {
		caller.detail = fmt.Sprintf("MCP _meta.threadId %q matched %d live Codex tmux seats", threadID, count)
		return caller, nil
	}
	socketPath, err := socketPathUnder(current.paths.TmuxDir, match.Socket)
	if err != nil {
		caller.detail = fmt.Sprintf("MCP _meta.threadId %q has an invalid tmux socket: %v", threadID, err)
		return caller, nil
	}
	caller.row = match
	caller.identity = resolve.Identity{
		Session:    match.Session,
		SocketPath: socketPath,
		SocketName: filepath.Base(socketPath),
		Pane:       match.Pane,
		Engine:     string(match.Engine),
		ID:         match.ID,
		Source:     "mcp-thread-meta",
	}
	caller.valid = true
	return caller, nil
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func socketPathUnder(root, socket string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("tmux directory is empty")
	}
	if socket == "" || socket == "." || filepath.IsAbs(socket) || filepath.Base(socket) != socket {
		return "", fmt.Errorf("socket must be one relative tmux socket name")
	}
	path := filepath.Join(root, filepath.Clean(socket))
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("socket escapes tmux directory")
	}
	return path, nil
}

// excludedFromChatLS names the row kinds chat_ls never lists. NewClaude and
// NewCodex are synthetic launch actions with no chat behind them yet.
// Booting is a real live chat, but it carries no crumb or transcript yet —
// only a crumbless socket — so it has no stable identity for the MCP tool
// contract to hand a caller; whoami/find/resume all key on an id this row
// does not have one of. It stays excluded here until that identity exists,
// the same reason the picker's ⌃X kill guard (ui/model.go) and compose's
// applyKill refuse it too.
func excludedFromChatLS(kind compose.Kind) bool {
	return kind == compose.NewClaude ||
		kind == compose.NewCodex ||
		kind == compose.Booting
}
