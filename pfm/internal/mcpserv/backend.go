package mcpserv

import (
	"context"
	"fmt"
	"io"

	"hostops/pfm/internal/compose"
	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/inject"
	"hostops/pfm/internal/resolve"
	"hostops/pfm/internal/store"
)

type injectionService interface {
	Resolve(context.Context, string) (inject.Target, int, string, error)
	Capture(context.Context, string, int) (inject.Target, string, int, string, error)
	Inject(context.Context, inject.Request) (inject.Result, error)
}

type backend struct {
	database   *store.Store
	injector   injectionService
	resolver   resolve.Resolver
	operations SharedOperations
	dispatch   Dispatch
}

func newBackendConfigured(warnings io.Writer, runtime Runtime) (*backend, error) {
	if len(runtime.Accounts) == 0 {
		machine := pfmconfig.Defaults(runtime.Paths.Home, runtime.Paths.ClaudeRoots)
		runtime.Accounts = machine.Accounts
	}
	if runtime.CodexBinary == "" {
		runtime.CodexBinary = "codex"
	}
	if runtime.ClaudeBinary == "" {
		runtime.ClaudeBinary = "claude"
	}
	if runtime.OpencodeBinary == "" {
		runtime.OpencodeBinary = "opencode"
	}
	database, err := store.Open(store.WithWarningWriter(warnings))
	if err != nil {
		return nil, err
	}
	resolver, err := resolve.New(nil, resolve.Binaries{
		Claude:        runtime.ClaudeBinary,
		Codex:         runtime.CodexBinary,
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
		database:   database,
		injector:   injector,
		resolver:   *resolver,
		operations: runtime.Operations,
		dispatch:   runtime.Dispatch,
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
