package mcpserv

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hostops/pfm/internal/headless"
	"hostops/pfm/internal/transcript"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (service *Service) chatLast(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input LastInput,
) (*mcp.CallToolResult, LastOutput, error) {
	if strings.TrimSpace(input.Target) == "" {
		return nil, LastOutput{}, fmt.Errorf("target is required")
	}
	source, err := service.backend.resolveReadSource(ctx, input.Target)
	if err != nil {
		return nil, LastOutput{}, err
	}
	entries, _, err := transcript.Tail(ctx, source.path, source.engine, 200, 0)
	if err != nil {
		return nil, LastOutput{}, err
	}
	entry, ok := transcript.Last(entries, transcript.RoleAssistant)
	if !ok {
		return nil, LastOutput{}, fmt.Errorf("%q has not answered yet", source.name)
	}
	return nil, LastOutput{Target: input.Target, Text: entry.Text}, nil
}

func (service *Service) chatStatus(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input StatusInput,
) (*mcp.CallToolResult, StatusOutput, error) {
	if strings.TrimSpace(input.Target) == "" {
		return nil, StatusOutput{}, fmt.Errorf("target is required")
	}
	source, err := service.backend.resolveReadSource(ctx, input.Target)
	if err != nil {
		return nil, StatusOutput{}, err
	}
	target, code, detail, err := service.backend.injector.Resolve(ctx, input.Target)
	if err != nil {
		return nil, StatusOutput{}, err
	}
	if code != 0 && detail != "" {
		return nil, StatusOutput{}, fmt.Errorf("resolve %q: %s", input.Target, detail)
	}
	status, err := headless.Inspect(ctx, headless.Chat{
		Name: source.name, ID: source.id, Engine: source.engine, Path: source.path,
		CWD: source.dir, Socket: target.SocketPath, Pane: target.Pane, Live: code == 0,
	}, now())
	if err != nil {
		return nil, StatusOutput{}, err
	}
	return nil, StatusOutput{
		Name: status.Name, State: status.State, IdleSeconds: status.IdleSeconds,
		Engine: status.Engine, Model: status.Model, CWD: status.CWD,
		SessionID: status.SessionID, Socket: status.Socket,
		ContextPct: status.ContextPct, Last: status.Last,
	}, nil
}

// now is a small seam for the status adapter; unlike the CLI's package-level
// test clock, production uses wall time and MCP callers receive the same
// headless.Inspect semantics.
var now = func() (value time.Time) { return time.Now() }

func (service *Service) chatNew(ctx context.Context, _ *mcp.CallToolRequest, input NewInput) (*mcp.CallToolResult, ActionOutput, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, ActionOutput{}, fmt.Errorf("name is required")
	}
	args := []string{"chat", "new", "--name", input.Name}
	if input.Engine != "" {
		args = append(args, "--engine", input.Engine)
	}
	if input.CWD != "" {
		args = append(args, "--cwd", input.CWD)
	}
	if input.Account != 0 {
		args = append(args, "--account", fmt.Sprint(input.Account))
	}
	if input.Cache1H {
		args = append(args, "--1h")
	}
	if input.Model != "" {
		args = append(args, "--model", input.Model)
	}
	if input.Effort != "" {
		args = append(args, "--effort", input.Effort)
	}
	if input.Await {
		args = append(args, "--await")
	}
	if input.Timeout != 0 {
		args = append(args, "--timeout", fmt.Sprint(input.Timeout))
	}
	if input.Settle != 0 {
		args = append(args, "--settle", fmt.Sprint(input.Settle))
	}
	if input.Progress {
		args = append(args, "--progress")
	}
	if input.Attach {
		args = append(args, "--attach")
	}
	if input.Prompt != "" {
		args = append(args, input.Prompt)
	}
	return service.cliAction(ctx, args...)
}

func (service *Service) chatOpen(ctx context.Context, _ *mcp.CallToolRequest, input TargetInput) (*mcp.CallToolResult, ActionOutput, error) {
	return service.cliTargetAction(ctx, "open", input.Target)
}

func (service *Service) chatName(ctx context.Context, _ *mcp.CallToolRequest, input NameInput) (*mcp.CallToolResult, ActionOutput, error) {
	if strings.TrimSpace(input.Name) == "" || strings.ContainsAny(input.Name, "\r\n\x00") {
		return nil, ActionOutput{}, fmt.Errorf("name must be one non-empty line")
	}
	return service.cliAction(ctx, "chat", "name", input.Target, input.Name)
}

func (service *Service) chatHide(ctx context.Context, _ *mcp.CallToolRequest, input HideInput) (*mcp.CallToolResult, ActionOutput, error) {
	args := []string{"chat", "hide", input.Target}
	if input.Exit {
		args = append(args, "--exit")
	}
	return service.cliAction(ctx, args...)
}

func (service *Service) chatUnhide(ctx context.Context, _ *mcp.CallToolRequest, input TargetInput) (*mcp.CallToolResult, ActionOutput, error) {
	return service.cliTargetAction(ctx, "unhide", input.Target)
}

func (service *Service) chatReload(ctx context.Context, _ *mcp.CallToolRequest, input TargetInput) (*mcp.CallToolResult, ActionOutput, error) {
	return service.cliTargetAction(ctx, "reload", input.Target)
}

func (service *Service) chatSave(ctx context.Context, _ *mcp.CallToolRequest, input SaveInput) (*mcp.CallToolResult, ActionOutput, error) {
	args := []string{"chat", "save", input.Target}
	if input.Transcript != "" {
		args = append(args, input.Transcript)
	}
	return service.cliAction(ctx, args...)
}

func (service *Service) cliTargetAction(ctx context.Context, verb, target string) (*mcp.CallToolResult, ActionOutput, error) {
	if strings.TrimSpace(target) == "" {
		return nil, ActionOutput{}, fmt.Errorf("target is required")
	}
	return service.cliAction(ctx, "chat", verb, target)
}

func (service *Service) cliAction(ctx context.Context, args ...string) (*mcp.CallToolResult, ActionOutput, error) {
	if service.backend.dispatch == nil {
		return nil, ActionOutput{Status: "error", Code: 1}, fmt.Errorf("chat action in-process CLI dispatcher is not configured")
	}
	var stdout, stderr strings.Builder
	code := service.backend.dispatch(ctx, args, &stdout, &stderr)
	if code != 0 {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		return nil, ActionOutput{Status: "error", Code: code, Message: message}, fmt.Errorf("pfm %s exited %d: %s", strings.Join(args, " "), code, message)
	}
	return nil, ActionOutput{Status: "ok", Code: 0, Message: strings.TrimSpace(stdout.String())}, nil
}
