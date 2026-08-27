package mcpserv

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"hostops/pfm/internal/chatgroup"
	"hostops/pfm/internal/chatload"
	"hostops/pfm/internal/inject"
	"hostops/pfm/internal/resolve"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (service *Service) chatBranch(
	ctx context.Context,
	request *mcp.CallToolRequest,
	input BranchInput,
) (*mcp.CallToolResult, ActionOutput, error) {
	if input.Name != "" && (strings.TrimSpace(input.Name) == "" || strings.ContainsAny(input.Name, "\r\n\x00")) {
		return nil, ActionOutput{}, fmt.Errorf("name must be one non-empty line when provided")
	}
	caller, err := service.backend.callerForRequest(ctx, requestMeta(request))
	if err != nil {
		return nil, ActionOutput{}, err
	}
	if refused, detail := service.selfCallerRefusal(caller); refused {
		return nil, ActionOutput{Status: "not_found", Code: inject.CodeUnknown, Message: detail}, nil
	}
	if !caller.valid {
		args := []string{"chat", "branch"}
		if input.Name != "" {
			args = append(args, "--name", input.Name)
		}
		return service.cliAction(ctx, args...)
	}
	if strings.TrimSpace(caller.row.Dir) == "" {
		return nil, ActionOutput{
			Status: "not_found", Code: inject.CodeUnknown,
			Message: fmt.Sprintf("MCP caller %q has no project directory to fork", caller.row.ID),
		}, nil
	}
	args := []string{
		"chat", "branch", "--engine", string(caller.row.Engine),
		"--session-id", caller.row.ID, "--cwd", caller.row.Dir,
	}
	if caller.row.Account != 0 {
		args = append(args, "--account", fmt.Sprint(caller.row.Account))
	}
	if input.Name != "" {
		args = append(args, "--name", input.Name)
	}
	return service.cliAction(ctx, args...)
}

func (service *Service) chatGoal(
	ctx context.Context,
	request *mcp.CallToolRequest,
	input GoalInput,
) (*mcp.CallToolResult, InjectOutput, error) {
	goal := strings.TrimSpace(input.Goal)
	if goal == "" || strings.ContainsAny(goal, "\r\n\x00") {
		return nil, InjectOutput{}, fmt.Errorf("goal must be one non-empty line")
	}
	if utf8.RuneCountInString(goal) > 4000 {
		return nil, InjectOutput{}, fmt.Errorf("goal is %d characters; maximum is 4000", utf8.RuneCountInString(goal))
	}
	target := strings.TrimSpace(input.Target)
	if target == "" {
		target = "self"
	}
	injector, caller, err := service.injectorForRequest(ctx, request)
	if err != nil {
		return nil, InjectOutput{}, err
	}
	if refused, detail := service.selfCallerRefusal(caller); selfTarget(target) && refused {
		return nil, InjectOutput{Status: "not_found", Code: inject.CodeUnknown, Message: detail}, nil
	}
	result, err := injector.Inject(ctx, inject.Request{Target: target, Message: "/goal " + goal})
	return nil, outputFromInject(result), err
}

func (service *Service) chatLoad(
	_ context.Context,
	_ *mcp.CallToolRequest,
	input LoadInput,
) (*mcp.CallToolResult, LoadOutput, error) {
	limit := input.MaxBytes
	if limit == 0 {
		limit = 1 << 20
	}
	if limit < 1 || limit > maxCaptureBytes {
		return nil, LoadOutput{}, fmt.Errorf("max_bytes must be between 1 and %d", maxCaptureBytes)
	}
	loaded, err := chatload.Load(input.Paths, limit)
	if err != nil {
		return nil, LoadOutput{}, err
	}
	output := LoadOutput{
		Warnings: loaded.Warnings, Count: len(loaded.Files),
		TotalLines: loaded.TotalLines, TotalBytes: loaded.TotalBytes,
		Files: make([]LoadFile, 0, len(loaded.Files)),
	}
	for _, file := range loaded.Files {
		output.Files = append(output.Files, LoadFile{
			Path: file.Path, Lines: file.Lines, Bytes: file.Bytes, Text: file.Text,
		})
	}
	return nil, output, nil
}

func (service *Service) chatGroupCreate(ctx context.Context, request *mcp.CallToolRequest, input GroupInput) (*mcp.CallToolResult, GroupReceiptOutput, error) {
	_, caller, member, refusal, err := service.groupCaller(ctx, request)
	if err != nil || refusal != "" {
		return groupIdentityFailure(caller, refusal, err)
	}
	bus, err := service.groupBus()
	if err != nil {
		return nil, GroupReceiptOutput{}, err
	}
	receipt, err := bus.Create(ctx, input.Group, member)
	return groupReceipt(receipt, err)
}

func (service *Service) chatGroupSubscribe(ctx context.Context, request *mcp.CallToolRequest, input GroupInput) (*mcp.CallToolResult, GroupReceiptOutput, error) {
	_, caller, member, refusal, err := service.groupCaller(ctx, request)
	if err != nil || refusal != "" {
		return groupIdentityFailure(caller, refusal, err)
	}
	bus, err := service.groupBus()
	if err != nil {
		return nil, GroupReceiptOutput{}, err
	}
	receipt, err := bus.Subscribe(ctx, input.Group, member)
	return groupReceipt(receipt, err)
}

func (service *Service) chatGroupInvite(ctx context.Context, request *mcp.CallToolRequest, input GroupInviteInput) (*mcp.CallToolResult, GroupReceiptOutput, error) {
	injector, caller, member, refusal, err := service.groupCaller(ctx, request)
	if err != nil || refusal != "" {
		return groupIdentityFailure(caller, refusal, err)
	}
	bus, err := service.groupBus()
	if err != nil {
		return nil, GroupReceiptOutput{}, err
	}
	receipt, err := bus.Invite(ctx, input.Group, member, input.Target, injectorNudge(injector))
	return groupReceipt(receipt, err)
}

func (service *Service) chatGroupList(ctx context.Context, request *mcp.CallToolRequest, _ GroupListInput) (*mcp.CallToolResult, GroupListOutput, error) {
	_, _, member, refusal, err := service.groupCaller(ctx, request)
	if err != nil {
		return nil, GroupListOutput{}, err
	}
	if refusal != "" {
		return nil, GroupListOutput{}, fmt.Errorf("resolve chat group caller: %s", refusal)
	}
	bus, err := service.groupBus()
	if err != nil {
		return nil, GroupListOutput{}, err
	}
	groups, err := bus.List(ctx, member)
	return nil, GroupListOutput{Groups: groups, Count: len(groups), Member: member}, err
}

func (service *Service) chatGroupRead(ctx context.Context, request *mcp.CallToolRequest, input GroupReadInput) (*mcp.CallToolResult, GroupReadOutput, error) {
	_, _, member, refusal, err := service.groupCaller(ctx, request)
	if err != nil {
		return nil, GroupReadOutput{}, err
	}
	if refusal != "" {
		return nil, GroupReadOutput{}, fmt.Errorf("resolve chat group caller: %s", refusal)
	}
	bus, err := service.groupBus()
	if err != nil {
		return nil, GroupReadOutput{}, err
	}
	read, err := bus.Read(ctx, input.Group, member, input.Peek)
	return nil, GroupReadOutput{
		Group: read.Group, Member: read.Member, Messages: read.Messages, Count: len(read.Messages),
		Cursor: read.Cursor, Total: read.Total, Peek: read.Peek,
	}, err
}

func (service *Service) chatGroupSend(ctx context.Context, request *mcp.CallToolRequest, input GroupSendInput) (*mcp.CallToolResult, GroupSendOutput, error) {
	injector, _, member, refusal, err := service.groupCaller(ctx, request)
	if err != nil {
		return nil, GroupSendOutput{}, err
	}
	if refusal != "" {
		return nil, GroupSendOutput{Status: "not_found", Code: inject.CodeUnknown, Message: refusal}, nil
	}
	message, err := groupMessage(input)
	if err != nil {
		return nil, GroupSendOutput{}, err
	}
	bus, err := service.groupBus()
	if err != nil {
		return nil, GroupSendOutput{}, err
	}
	sent, err := bus.Send(ctx, input.Group, member, message, input.To, injectorNudge(injector))
	if err != nil {
		return nil, GroupSendOutput{}, err
	}
	return nil, GroupSendOutput{
		Status: "ok", Code: 0, Group: sent.Group, Sender: sent.Sender, Number: sent.Number,
		Record: sent.Record, Target: sent.Target, TargetMatches: sent.TargetMatches,
		Nudges: sent.Nudges, SpillPath: sent.SpillPath,
	}, nil
}

func (service *Service) groupBus() (*chatgroup.Bus, error) {
	home := strings.TrimSpace(service.backend.paths.Home)
	if home == "" {
		return nil, errors.New("chat group bus requires a configured home directory")
	}
	bus, err := chatgroup.New(chatgroup.DefaultRoot(home))
	if err != nil {
		return nil, err
	}
	bus.Recorder = service.backend.sharedState.RecordComms
	bus.WarningWriter = service.backend.warnings
	return bus, nil
}

func (service *Service) groupCaller(
	ctx context.Context,
	request *mcp.CallToolRequest,
) (injectionService, callerIdentity, string, string, error) {
	injector, caller, err := service.injectorForRequest(ctx, request)
	if err != nil {
		return nil, caller, "", "", err
	}
	if refused, detail := service.selfCallerRefusal(caller); refused {
		return injector, caller, "", detail, nil
	}
	if caller.valid {
		member := strings.TrimSpace(caller.row.Name)
		if member == "" {
			member = caller.row.Session
		}
		return injector, caller, member, "", nil
	}
	identifier, err := resolve.NewWhoami(resolve.WhoamiDependencies{})
	if err != nil {
		return injector, caller, "", "", err
	}
	identity, err := identifier.Identify(ctx)
	if err != nil {
		return injector, caller, "", "", fmt.Errorf("resolve ambient chat group caller: %w", err)
	}
	return injector, caller, identity.Session, "", nil
}

func injectorNudge(injector injectionService) chatgroup.NudgeFunc {
	return func(ctx context.Context, target, message string) error {
		result, err := injector.Inject(ctx, inject.Request{
			Target: target, Message: message, Origin: inject.OriginGroupNudge,
		})
		if err != nil {
			return err
		}
		if result.Code != 0 {
			return fmt.Errorf("inject rc=%d: %s", result.Code, result.Message)
		}
		return nil
	}
}

func groupReceipt(receipt chatgroup.Receipt, err error) (*mcp.CallToolResult, GroupReceiptOutput, error) {
	if err != nil {
		return nil, GroupReceiptOutput{}, err
	}
	return nil, GroupReceiptOutput{
		Status: "ok", Code: 0, Group: receipt.Group, Member: receipt.Member,
		Path: receipt.Path, Message: receipt.Message, Existing: receipt.Existing,
		MemberCount: receipt.MemberCount,
	}, nil
}

func groupIdentityFailure(caller callerIdentity, refusal string, err error) (*mcp.CallToolResult, GroupReceiptOutput, error) {
	if err != nil {
		return nil, GroupReceiptOutput{}, err
	}
	if refusal == "" {
		refusal = caller.detail
	}
	return nil, GroupReceiptOutput{Status: "not_found", Code: inject.CodeUnknown, Message: refusal}, nil
}

func groupMessage(input GroupSendInput) (string, error) {
	if input.File != "" && input.Message != "" {
		return "", errors.New("message and file are mutually exclusive")
	}
	if input.File == "" {
		if input.Caption != "" {
			return "", errors.New("caption requires file")
		}
		if strings.TrimSpace(input.Message) == "" {
			return "", errors.New("message or file is required")
		}
		return input.Message, nil
	}
	info, err := os.Stat(input.File)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("group message file does not exist: %s", input.File)
	}
	if err != nil {
		return "", fmt.Errorf("inspect group message file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("group message file is not regular: %s", input.File)
	}
	absolute, err := filepath.Abs(input.File)
	if err != nil {
		return "", fmt.Errorf("resolve group message file: %w", err)
	}
	message := "[long message — Read: " + absolute + "]"
	if caption := strings.TrimSpace(input.Caption); caption != "" {
		message += " " + caption
	}
	return message, nil
}
