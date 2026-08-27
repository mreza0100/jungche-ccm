package mcpserv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"hostops/pfm/internal/headless"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (service *Service) chatLast(
	ctx context.Context,
	request *mcp.CallToolRequest,
	input LastInput,
) (*mcp.CallToolResult, LastOutput, error) {
	if strings.TrimSpace(input.Target) == "" {
		return nil, LastOutput{}, fmt.Errorf("target is required")
	}
	if service.backend.dispatch == nil {
		return nil, LastOutput{}, fmt.Errorf("chat_last command is not configured")
	}
	target, err := service.cliTargetForRequest(ctx, request, input.Target)
	if err != nil {
		return nil, LastOutput{}, err
	}
	var stdout, stderr bytes.Buffer
	code := service.backend.dispatch(
		ctx,
		[]string{"chat", "last", target},
		&stdout,
		&stderr,
	)
	if code != 0 {
		return nil, LastOutput{}, fmt.Errorf(
			"chat_last command rc=%d stderr=%q",
			code,
			strings.TrimSpace(stderr.String()),
		)
	}
	text := strings.TrimRight(stdout.String(), "\r\n")
	if text == "" {
		return nil, LastOutput{}, fmt.Errorf("chat_last command returned no answer")
	}
	return nil, LastOutput{Target: input.Target, Text: text}, nil
}

func (service *Service) chatStatus(
	ctx context.Context,
	request *mcp.CallToolRequest,
	input StatusInput,
) (*mcp.CallToolResult, StatusOutput, error) {
	if strings.TrimSpace(input.Target) == "" {
		return nil, StatusOutput{}, fmt.Errorf("target is required")
	}
	if !input.Summary && !input.Ask && (input.Engine != "" || input.Model != "") {
		return nil, StatusOutput{}, fmt.Errorf("engine and model require summary=true or ask=true")
	}
	if service.backend.dispatch == nil {
		return nil, StatusOutput{}, fmt.Errorf("chat_status command is not configured")
	}
	target, err := service.cliTargetForRequest(ctx, request, input.Target)
	if err != nil {
		return nil, StatusOutput{}, err
	}
	args := []string{"chat", "status", target, "--json"}
	if input.Summary {
		args = append(args, "--summary")
	}
	if input.Ask {
		args = append(args, "--ask")
	}
	if input.Engine != "" {
		args = append(args, "--engine", input.Engine)
	}
	if input.Model != "" {
		args = append(args, "--model", input.Model)
	}
	var stdout, stderr bytes.Buffer
	code := service.backend.dispatch(ctx, args, &stdout, &stderr)
	var output StatusOutput
	decodeErr := json.Unmarshal(stdout.Bytes(), &output)
	if code != 0 {
		if decodeErr == nil && output.State == headless.StateDead {
			return nil, output, nil
		}
		return nil, StatusOutput{}, fmt.Errorf(
			"chat_status command rc=%d stderr=%q",
			code,
			strings.TrimSpace(stderr.String()),
		)
	}
	if decodeErr != nil {
		return nil, StatusOutput{}, fmt.Errorf(
			"chat_status command returned invalid JSON stderr=%q: decode output: %w",
			strings.TrimSpace(stderr.String()),
			decodeErr,
		)
	}
	return nil, output, nil
}

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

func (service *Service) chatOpen(ctx context.Context, request *mcp.CallToolRequest, input TargetInput) (*mcp.CallToolResult, ActionOutput, error) {
	target, err := service.cliTargetForRequest(ctx, request, input.Target)
	if err != nil {
		return nil, ActionOutput{}, err
	}
	return service.cliTargetAction(ctx, "open", target)
}

func (service *Service) chatName(ctx context.Context, request *mcp.CallToolRequest, input NameInput) (*mcp.CallToolResult, ActionOutput, error) {
	if strings.TrimSpace(input.Name) == "" || strings.ContainsAny(input.Name, "\r\n\x00") {
		return nil, ActionOutput{}, fmt.Errorf("name must be one non-empty line")
	}
	target, err := service.cliTargetForRequest(ctx, request, input.Target)
	if err != nil {
		return nil, ActionOutput{}, err
	}
	return service.cliAction(ctx, "chat", "name", target, input.Name)
}

func (service *Service) chatKill(ctx context.Context, request *mcp.CallToolRequest, input KillInput) (*mcp.CallToolResult, ActionOutput, error) {
	target, err := service.cliTargetForRequest(ctx, request, input.Target)
	if err != nil {
		return nil, ActionOutput{}, err
	}
	args := []string{"chat", "kill", target}
	if input.Exit {
		args = append(args, "--exit")
	}
	return service.cliAction(ctx, args...)
}

func (service *Service) chatUnkill(ctx context.Context, request *mcp.CallToolRequest, input TargetInput) (*mcp.CallToolResult, ActionOutput, error) {
	target, err := service.cliTargetForRequest(ctx, request, input.Target)
	if err != nil {
		return nil, ActionOutput{}, err
	}
	return service.cliTargetAction(ctx, "unkill", target)
}

func (service *Service) chatReload(ctx context.Context, request *mcp.CallToolRequest, input TargetInput) (*mcp.CallToolResult, ActionOutput, error) {
	target, err := service.cliTargetForRequest(ctx, request, input.Target)
	if err != nil {
		return nil, ActionOutput{}, err
	}
	return service.cliTargetAction(ctx, "reload", target)
}

func (service *Service) chatSave(ctx context.Context, request *mcp.CallToolRequest, input SaveInput) (*mcp.CallToolResult, ActionOutput, error) {
	target, err := service.cliTargetForRequest(ctx, request, input.Target)
	if err != nil {
		return nil, ActionOutput{}, err
	}
	args := []string{"chat", "save", target}
	if input.Transcript != "" {
		args = append(args, input.Transcript)
	}
	return service.cliAction(ctx, args...)
}

// cliTargetForRequest translates a request-scoped Codex self into the stable
// thread id understood by the in-process CLI dispatcher. The HTTP daemon has
// no tmux ancestry of its own, so forwarding the literal word "self" asks the
// daemon who it is and necessarily resolves nothing.
func (service *Service) cliTargetForRequest(
	ctx context.Context,
	request *mcp.CallToolRequest,
	target string,
) (string, error) {
	if !selfTarget(target) {
		return target, nil
	}
	caller, err := service.backend.callerForRequest(ctx, requestMeta(request))
	if err != nil {
		return "", err
	}
	if !caller.present {
		if !service.backend.allowAmbientIdentity {
			return "", fmt.Errorf("resolve MCP self: %s", noAmbientCallerRemedy)
		}
		return target, nil
	}
	if !caller.valid {
		return "", fmt.Errorf("resolve MCP self: %s", caller.detail)
	}
	return caller.identity.ID, nil
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
