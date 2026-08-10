package mcpserv

import (
	"context"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"hostops/cc-fleet/internal/inject"
	"hostops/cc-fleet/internal/resolve"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// defaultCaptureBytes bounds a whole-scrollback capture for the default
	// caller; maxCaptureBytes is the ceiling an explicit max_bytes may ask for.
	defaultCaptureBytes = 256 << 10
	maxCaptureBytes     = 4 << 20
)

// Service owns one MCP server and its long-lived SQLite handle.
type Service struct {
	server  *mcp.Server
	backend *backend
}

// New creates the production stdio service.
func New(version string, warnings io.Writer) (*Service, error) {
	backend, err := newBackend(warnings)
	if err != nil {
		return nil, err
	}
	return newService(version, backend), nil
}

func newService(version string, backend *backend) *Service {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "cc-fleet",
		Version: version,
	}, &mcp.ServerOptions{
		Instructions: "Inspect, resolve, capture, search, read, and safely inject into the local cc-fleet chat fleet.",
	})
	service := &Service{server: server, backend: backend}
	service.register()
	return service
}

// Run serves until the transport closes.
func (service *Service) Run(ctx context.Context, transport mcp.Transport) error {
	return service.server.Run(ctx, transport)
}

// Server exposes the SDK server for in-memory protocol tests.
func (service *Service) Server() *mcp.Server {
	return service.server
}

// Close closes the long-lived store.
func (service *Service) Close() error {
	return service.backend.close()
}

func (service *Service) register() {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	mutating := &mcp.ToolAnnotations{ReadOnlyHint: false}
	mcp.AddTool(service.server, &mcp.Tool{
		Name:        "chat_ls",
		Description: "List live and resumable Claude/Codex chats as structured rows.",
		Annotations: readOnly,
	}, service.chatLS)
	mcp.AddTool(service.server, &mcp.Tool{
		Name:        "chat_resolve",
		Description: "Resolve an exact Claude label, tmux session, or Codex window name using chat.sh return-code semantics.",
		Annotations: readOnly,
	}, service.chatResolve)
	mcp.AddTool(service.server, &mcp.Tool{
		Name:        "chat_inject",
		Description: "Safely type and submit a message to a live chat after selector, busy, draft, and submit-confirm guards.",
		Annotations: mutating,
	}, service.chatInject)
	mcp.AddTool(service.server, &mcp.Tool{
		Name:        "chat_capture",
		Description: "Capture the whole retained scrollback of a resolved live chat, bounded by tail_lines and max_bytes.",
		Annotations: readOnly,
	}, service.chatCapture)
	mcp.AddTool(service.server, &mcp.Tool{
		Name:        "chat_whoami",
		Description: "Print THIS chat's own tmux session name — its identity, and the address another chat injects to.",
		Annotations: readOnly,
	}, service.chatWhoami)
	mcp.AddTool(service.server, &mcp.Tool{
		Name:        "chat_find",
		Description: "Find indexed Claude/Codex transcripts by literal name or prompt excerpt with file confirmation.",
		Annotations: readOnly,
	}, service.chatFind)
	mcp.AddTool(service.server, &mcp.Tool{
		Name:        "chat_read",
		Description: "Read bounded recent visible turns from an indexed Claude/Codex transcript.",
		Annotations: readOnly,
	}, service.chatRead)
}

func (service *Service) chatLS(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input LSInput,
) (*mcp.CallToolResult, LSOutput, error) {
	output, err := service.backend.list(ctx, input)
	return nil, output, err
}

func (service *Service) chatResolve(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input ResolveInput,
) (*mcp.CallToolResult, ResolveOutput, error) {
	kind := resolve.Kind(input.Kind)
	if kind != resolve.Label && kind != resolve.Session && kind != resolve.CxWindow {
		return nil, ResolveOutput{}, fmt.Errorf(
			"kind must be label, session, or cxwin",
		)
	}
	namespace, err := service.backend.resolver.Resolve(ctx, kind, input.Name)
	if err != nil {
		return nil, ResolveOutput{}, err
	}
	status := "ok"
	if namespace.Code == 1 {
		status = "not_found"
	} else if namespace.Code == 2 {
		status = "ambiguous"
	}
	socket, pane := parseResolved(namespace.Stdout)
	return nil, ResolveOutput{
		Status:     status,
		Code:       namespace.Code,
		SocketPath: socket,
		Pane:       pane,
		Candidates: namespace.Stderr,
	}, nil
}

func (service *Service) chatInject(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input InjectInput,
) (*mcp.CallToolResult, InjectOutput, error) {
	result, err := service.backend.injector.Inject(ctx, inject.Request{
		Target:   input.Target,
		Message:  input.Message,
		ForceNow: input.ForceNow,
		Then:     input.Then,
	})
	return nil, InjectOutput{
		Status:        result.Status,
		Code:          result.Code,
		Message:       result.Message,
		SocketPath:    result.SocketPath,
		Pane:          result.Pane,
		Proof:         result.Proof,
		Busy:          result.Busy,
		Interrupted:   result.Interrupted,
		DraftStashed:  result.DraftStashed,
		Typed:         result.Typed,
		SubmitRetries: result.SubmitRetries,
		Steers:        result.Steers,
		SteerLog:      result.SteerLog,
		Unsigned:      result.Unsigned,
	}, err
}

// chatWhoami answers with this process's own chat identity. It takes no
// arguments on purpose: identity is derived from the caller's own process
// chain, never accepted from a caller.
func (service *Service) chatWhoami(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ WhoamiInput,
) (*mcp.CallToolResult, WhoamiOutput, error) {
	identifier, err := resolve.NewWhoami(resolve.WhoamiDependencies{})
	if err != nil {
		return nil, WhoamiOutput{}, err
	}
	identity, err := identifier.Identify(ctx)
	if err != nil {
		return nil, WhoamiOutput{
			Status:  "not_found",
			Engine:  identity.Engine,
			ID:      identity.ID,
			Source:  identity.Source,
			Message: err.Error(),
		}, nil
	}
	return nil, WhoamiOutput{
		Status:     "ok",
		Session:    identity.Session,
		SocketPath: identity.SocketPath,
		SocketName: identity.SocketName,
		Pane:       identity.Pane,
		Engine:     identity.Engine,
		ID:         identity.ID,
		Source:     identity.Source,
		Recovered:  identity.Recovered,
	}, nil
}

func (service *Service) chatCapture(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input CaptureInput,
) (*mcp.CallToolResult, CaptureOutput, error) {
	lines := input.TailLines
	if lines == 0 {
		lines = 40
	}
	if lines < 1 || lines > 1000 {
		return nil, CaptureOutput{}, fmt.Errorf(
			"tail_lines must be between 1 and 1000",
		)
	}
	maxBytes := input.MaxBytes
	if maxBytes == 0 {
		maxBytes = defaultCaptureBytes
	}
	if maxBytes < 1 || maxBytes > maxCaptureBytes {
		return nil, CaptureOutput{}, fmt.Errorf(
			"max_bytes must be between 1 and %d",
			maxCaptureBytes,
		)
	}
	target, text, code, detail, err := service.backend.injector.Capture(
		ctx,
		input.Target,
		lines,
	)
	status := "ok"
	if code != 0 {
		status = "not_found"
	}
	// The engine captures the WHOLE scrollback; the byte bound is applied here,
	// after the capture, keeping the most recent screen when it has to cut.
	truncated := false
	if len(text) > maxBytes {
		text = tailBytes(text, maxBytes)
		truncated = true
	}
	return nil, CaptureOutput{
		Status:     status,
		Code:       code,
		SocketPath: target.SocketPath,
		Pane:       target.Pane,
		Text:       text,
		Message:    detail,
		Bytes:      len(text),
		Truncated:  truncated,
	}, err
}

// tailBytes keeps the last budget bytes of text without splitting a rune.
func tailBytes(text string, budget int) string {
	if len(text) <= budget {
		return text
	}
	cut := len(text) - budget
	for cut < len(text) && !utf8.RuneStart(text[cut]) {
		cut++
	}
	return text[cut:]
}

func (service *Service) chatFind(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input FindInput,
) (*mcp.CallToolResult, FindOutput, error) {
	output, err := service.backend.find(ctx, input)
	return nil, output, err
}

func (service *Service) chatRead(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input ReadInput,
) (*mcp.CallToolResult, ReadOutput, error) {
	output, err := service.backend.read(ctx, input)
	return nil, output, err
}

func parseResolved(value string) (string, string) {
	fields := strings.Split(strings.TrimSpace(value), "\t")
	if len(fields) != 2 {
		return "", ""
	}
	return fields[0], fields[1]
}
