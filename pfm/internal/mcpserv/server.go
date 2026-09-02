package mcpserv

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"hostops/pfm/internal/chatkeys"
	pfmconfig "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/inject"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/resolve"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// defaultCaptureBytes bounds a whole-scrollback capture for the default
	// caller; maxCaptureBytes is the ceiling an explicit max_bytes may ask for.
	defaultCaptureBytes = 256 << 10
	maxCaptureBytes     = 4 << 20
)

// selfCompactDescription is a named const so the registered text and the test
// that pins it read the same string. The STOP clause is not decoration: the
// --then waiter recognises the compaction turn by watching this pane yield and
// then go busy again, and a caller that keeps working erases that boundary.
const selfCompactDescription = "The ONLY answer to \"compact yourself\", \"give yourself a compact\", \"self-compact\", or \"compact at this milestone\" — this tool and nothing else, never a hand-typed /compact and never a reboot of the pane: it compacts the requesting chat in place after its active turn settles and KEEPS the session (crons, sub-agents, and the pane all survive). Inspect your current screen, author a single-line focus, and give exactly ONE post-compact steer in `then` — one string, never a list — that must not start with /compact; the waiter types it into the reborn chat so it resumes unattended. Compaction DISCARDS context: if the caller keeps durable state of its own — a ledger, a scratch prompt, a state file, a chat-specific memory — it MUST write everything it wants to survive into that state BEFORE calling this, because the focus line and the one steer are the only things that cross the boundary. END THE TURN IMMEDIATELY after this call returns: run no further tool, start no further work, just report that compaction is queued. The steer is delivered by a waiter that identifies the compaction turn by watching this pane, so a caller that keeps working after calling this makes its own turn indistinguishable from the compaction and the steer lands beside the compaction instead of after it."

var chatToolNames = []string{
	"chat_branch", "chat_capture", "chat_find", "chat_goal", "chat_inject",
	"chat_keys", "chat_kill", "chat_last", "chat_load", "chat_ls", "chat_name",
	"chat_new", "chat_open", "chat_read", "chat_reload", "chat_resolve",
	"chat_save", "chat_self_compact", "chat_status", "chat_unkill",
	"chat_whoami", "issue_servicedesk",
}

// ToolNames returns the canonical advertised chat MCP roster. The jailed
// protocol test compares it to tools/list, so a registered tool cannot vanish
// from daemon status and doctor through a second hand-maintained list.
func ToolNames() []string {
	return append([]string(nil), chatToolNames...)
}

// Service owns one MCP server and its long-lived SQLite handle.
type Service struct {
	server  *mcp.Server
	backend *backend
}

// Runtime is the already-loaded machine policy the stdio server shares with
// the command that started it. The server never re-reads machine config.
type Runtime struct {
	Paths        paths.Values
	Accounts     []pfmconfig.Account
	ConfigPath   string
	ClaudeBinary string
	CodexBinary  string
	// OpencodeBinary is the configured OpenCode launch command; empty means
	// the registered OpenCode descriptor's default binary.
	OpencodeBinary string
	Operations     SharedOperations
	Dispatch       Dispatch
	// AllowAmbientIdentity is reserved for the stdio server, whose process is
	// launched by the calling chat. A shared HTTP daemon must leave it false:
	// its environment and ancestry identify the daemon's launcher, not the MCP
	// request that happens to be using it now.
	AllowAmbientIdentity bool
}

// SharedOperations are the canonical chat operations supplied by the CLI
// command package. MCP only adapts structured input to these functions; it
// does not maintain a second implementation of list, find, or read.
type SharedOperations struct {
	List func(context.Context, LSInput) (LSOutput, error)
	Find func(context.Context, FindInput) (FindOutput, error)
	Read func(context.Context, ReadInput) (ReadOutput, error)
}

// Dispatch is the in-process command dispatcher used by stateful chat tools.
// The callback receives the same argv beginning with "chat" that the CLI
// receives after global flags have been handled.
type Dispatch func(context.Context, []string, io.Writer, io.Writer) int

// NewConfigured creates the production service over one command runtime.
func NewConfigured(version string, warnings io.Writer, runtime Runtime) (*Service, error) {
	backend, err := newBackendConfigured(warnings, runtime)
	if err != nil {
		return nil, err
	}
	return newService(version, backend), nil
}

func newService(version string, backend *backend) *Service {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "pfm",
		Version: version,
	}, &mcp.ServerOptions{
		Instructions: "The local pfm chat fleet: inspect, resolve, capture, search, read/load complete file sets, branch, compile/fire goals, name, kill, reload, save, and safely inject. Routing for common asks — any phrasing of delivering text to another chat (\"send\", \"tell\", \"message\", \"reply to\", \"inject into\" chat X) is chat_inject; \"give yourself a compact\" / \"self-compact at this milestone\" is chat_self_compact (single-line focus plus exactly ONE continuation steer); \"who are you / what is your address\" is chat_whoami; \"what chats are running\" is chat_ls; \"spawn/start a new chat\" is chat_new. Excluded interactive/plumbing verbs: end, modal, watch, stream, recover, and history.",
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
		Name: "chat_branch", Description: "Fork the requesting Claude/Codex conversation into a new detached chat without changing the caller's pane.", Annotations: mutating,
	}, service.chatBranch)
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
		Description: "Send a message to a live chat — the verb behind every \"send / tell / message / reply to chat X\" request. Safely types and submits after selector, busy, draft, and submit-confirm guards.",
		Annotations: mutating,
	}, service.chatInject)
	mcp.AddTool(service.server, &mcp.Tool{
		Name:        "chat_self_compact",
		Description: selfCompactDescription,
		Annotations: mutating,
	}, service.chatSelfCompact)
	mcp.AddTool(service.server, &mcp.Tool{
		Name: "chat_goal", Description: "Fire an already-compiled, single-line /goal body (maximum 4000 characters) at a live chat or the requesting chat.", Annotations: mutating,
	}, service.chatGoal)
	mcp.AddTool(service.server, &mcp.Tool{
		Name: "chat_load", Description: "Return every complete non-empty text file beneath the requested paths; fail atomically if the complete set exceeds max_bytes.", Annotations: readOnly,
	}, service.chatLoad)
	mcp.AddTool(service.server, &mcp.Tool{
		Name:        "chat_keys",
		Description: "Press validated tmux key names or explicitly type literal key text into a live chat.",
		Annotations: mutating,
	}, service.chatKeys)
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
	mcp.AddTool(service.server, &mcp.Tool{
		Name: "chat_last", Description: "Return the newest assistant answer from a chat.", Annotations: readOnly,
	}, service.chatLast)
	mcp.AddTool(service.server, &mcp.Tool{
		Name: "chat_status", Description: "Inspect one chat, optionally summarizing its last human exchange or asking its current status from a live pane capture plus that exchange.", Annotations: readOnly,
	}, service.chatStatus)
	mcp.AddTool(service.server, &mcp.Tool{
		Name: "chat_new", Description: "Create a detached named chat through the canonical pfm chat new dispatcher.", Annotations: mutating,
	}, service.chatNew)
	mcp.AddTool(service.server, &mcp.Tool{
		Name: "chat_open", Description: "Open a resumable chat through the canonical pfm chat open dispatcher.", Annotations: mutating,
	}, service.chatOpen)
	mcp.AddTool(service.server, &mcp.Tool{
		Name: "chat_name", Description: "Name a live chat through the canonical pfm chat name dispatcher.", Annotations: mutating,
	}, service.chatName)
	mcp.AddTool(service.server, &mcp.Tool{
		Name: "chat_kill", Description: "Kill a chat through the canonical pfm chat kill dispatcher.", Annotations: mutating,
	}, service.chatKill)
	mcp.AddTool(service.server, &mcp.Tool{
		Name: "chat_unkill", Description: "Unkill a chat through the canonical pfm chat unkill dispatcher.", Annotations: mutating,
	}, service.chatUnkill)
	mcp.AddTool(service.server, &mcp.Tool{
		Name: "chat_reload", Description: "Reboot a chat in place through the canonical pfm chat reload dispatcher. Takes a target ONLY: no account switch, no cache mode, no --then follow-up. For any of those run `pfm chat reload [--account N] [--1h on|off] [--then \"prompt\"]` yourself — every setting there is a flag, never a bare word.", Annotations: mutating,
	}, service.chatReload)
	mcp.AddTool(service.server, &mcp.Tool{
		Name: "chat_save", Description: "Append a transcript snapshot through the canonical pfm chat save dispatcher.", Annotations: mutating,
	}, service.chatSave)
	mcp.AddTool(service.server, &mcp.Tool{
		Name:        "issue_servicedesk",
		Description: "File a durable complaint about Professor itself — what went wrong, and where — for a human to triage later. Reporter identity is captured automatically and can never be supplied by the caller.",
		Annotations: mutating,
	}, service.issueServicedesk)
}

type requestScopedInjector interface {
	WithIdentity(resolve.Identity, string) *inject.Engine
}

func requestMeta(request *mcp.CallToolRequest) mcp.Meta {
	if request == nil || request.Params == nil {
		return nil
	}
	return request.Params.Meta
}

func selfTarget(target string) bool {
	return target == "self" || target == "me"
}

// noAmbientCallerRemedy explains a missing-identity refusal in terms an MCP
// caller can act on, instead of the config fact chat.sh's CLI-oriented
// wording states. The shared HTTP daemon (one process serving every chat on
// the machine) genuinely cannot derive who is calling it over this
// transport: Claude Code attaches no per-call caller identity, so there is
// nothing here to sign or resolve "self" against. A Codex chat is
// unaffected — its MCP client attaches _meta.threadId to every call, and
// that is what this daemon reads. The remedy is architectural, not
// something available to THIS call: run the equivalent `pfm chat ...`
// command from the chat's own shell, since that process IS the chat and can
// derive its own identity; or ask the operator to move this chat's MCP
// transport onto per-chat stdio (`pfm mcp chat serve`), which inherits
// ambient identity deliberately.
const noAmbientCallerRemedy = "MCP request has no _meta.threadId, and this " +
	"server is pfm's shared HTTP daemon (one process serving every chat on " +
	"the machine), so it cannot derive who is calling: Claude Code does not " +
	"attach per-call caller identity over this transport. Run the " +
	"equivalent `pfm chat ...` command from the chat's own shell instead — " +
	"that process IS the chat. A Codex chat should resolve automatically; " +
	"if it does not, its MCP client is not attaching _meta.threadId to this call."

// selfCompactNoAmbientRemedy replaces noAmbientCallerRemedy for
// chat_self_compact alone: that message's remedy — "run the equivalent
// `pfm chat ...` command" — dangles here, because self-compact has no CLI
// twin. The working equivalent from a Claude chat's own shell is an inject
// into its own pane: the /compact travels as keystrokes and the steers ride
// the same --then waiter every inject uses.
const selfCompactNoAmbientRemedy = "MCP request has no _meta.threadId, and " +
	"this server is pfm's shared HTTP daemon (one process serving every chat " +
	"on the machine), so it cannot derive who is calling: Claude Code does " +
	"not attach per-call caller identity over this transport — and " +
	"chat_self_compact has no `pfm chat` CLI twin to fall back on. From the " +
	"chat's own shell, queue the same choreography with `pfm chat inject " +
	"--force-now --then '<steer>' $(pfm whoami) '/compact <focus>'`. A Codex " +
	"chat should resolve automatically; if it does not, its MCP client is " +
	"not attaching _meta.threadId to this call."

func (service *Service) selfCallerRefusal(caller callerIdentity) (bool, string) {
	if caller.valid {
		return false, ""
	}
	if caller.present {
		return true, caller.detail
	}
	if !service.backend.allowAmbientIdentity {
		return true, noAmbientCallerRemedy
	}
	return false, ""
}

// injectorForRequest binds a valid Codex _meta.threadId to a fresh injection
// engine. A malformed or unknown metadata value remains usable only for an
// explicit target; self is refused by each stateful handler below.
func (service *Service) injectorForRequest(
	ctx context.Context,
	request *mcp.CallToolRequest,
) (injectionService, callerIdentity, error) {
	caller, err := service.backend.callerForRequest(ctx, requestMeta(request))
	if err != nil {
		return nil, caller, err
	}
	if !caller.valid {
		return service.backend.injector, caller, nil
	}
	scoped, ok := service.backend.injector.(requestScopedInjector)
	if !ok {
		return nil, caller, fmt.Errorf("request-scoped MCP caller identity is unsupported by the injection engine")
	}
	return scoped.WithIdentity(caller.identity, caller.row.Name), caller, nil
}

func (service *Service) chatKeys(
	ctx context.Context,
	request *mcp.CallToolRequest,
	input KeysInput,
) (*mcp.CallToolResult, KeysOutput, error) {
	if strings.TrimSpace(input.Target) == "" {
		return nil, KeysOutput{}, fmt.Errorf("target is required")
	}
	if len(input.Keys) == 0 {
		return nil, KeysOutput{}, fmt.Errorf("keys must contain at least one key")
	}
	if !input.Literal {
		for _, key := range input.Keys {
			if !chatkeys.Valid(key) {
				return nil, KeysOutput{}, fmt.Errorf(
					"%q is not a tmux key; set literal=true to type it as text, or use one of: %s",
					key,
					chatkeys.Names(),
				)
			}
		}
	}
	delay := 120 * time.Millisecond
	if input.DelayMS != 0 {
		if input.DelayMS < 0 {
			return nil, KeysOutput{}, fmt.Errorf("delay_ms must not be negative")
		}
		delay = time.Duration(input.DelayMS) * time.Millisecond
	}
	injector, caller, err := service.injectorForRequest(ctx, request)
	if err != nil {
		return nil, KeysOutput{}, err
	}
	if refused, _ := service.selfCallerRefusal(caller); selfTarget(input.Target) && refused {
		return nil, KeysOutput{Status: "not_found", Code: inject.CodeUnknown, Keys: append([]string(nil), input.Keys...)}, nil
	}
	target, code, detail, err := injector.Resolve(ctx, input.Target)
	if err != nil {
		return nil, KeysOutput{}, err
	}
	if code != 0 {
		return nil, KeysOutput{Status: "not_found", Code: code, Keys: append([]string(nil), input.Keys...)}, fmt.Errorf("resolve %q: %s", input.Target, detail)
	}
	tmux := inject.CommandTmux{}
	for index, key := range input.Keys {
		if index > 0 && delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, KeysOutput{}, ctx.Err()
			case <-timer.C:
			}
		}
		var sendErr error
		if input.Literal {
			sendErr = tmux.SendLiteral(ctx, target.SocketPath, target.Pane, key)
		} else {
			sendErr = tmux.SendKey(ctx, target.SocketPath, target.Pane, key)
		}
		if sendErr != nil {
			return nil, KeysOutput{
				Status: "dead", Code: inject.CodeDead, SocketPath: target.SocketPath,
				Pane: target.Pane, Count: index, Keys: append([]string(nil), input.Keys...),
			}, fmt.Errorf("send %q: %w", key, sendErr)
		}
	}
	output := KeysOutput{
		Status: "ok", Code: 0, SocketPath: target.SocketPath, Pane: target.Pane,
		Count: len(input.Keys), Keys: append([]string(nil), input.Keys...),
	}
	if input.Capture {
		_, text, captureCode, detail, captureErr := injector.Capture(ctx, input.Target, 0)
		if captureErr != nil {
			return nil, output, fmt.Errorf("capture: %w", captureErr)
		}
		if captureCode != 0 {
			return nil, output, fmt.Errorf("capture: %s", detail)
		}
		output.Text = text
	}
	return nil, output, nil
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
	if kind == resolve.CxWindow {
		target, code, detail, err := service.backend.injector.ResolveEngine(
			ctx, input.Name, string(pfmengine.Codex),
		)
		if err != nil {
			return nil, ResolveOutput{}, err
		}
		status := "ok"
		switch code {
		case 0:
		case inject.CodeUnknown:
			status, code = "not_found", 1
		case inject.CodeAmbiguous:
			status, code = "ambiguous", 2
		default:
			return nil, ResolveOutput{}, fmt.Errorf(
				"resolve target %q failed with code %d: %s", input.Name, code, detail,
			)
		}
		return nil, ResolveOutput{
			Status: status, Code: code, SocketPath: target.SocketPath,
			Pane: target.Pane, Candidates: detail,
		}, nil
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
	request *mcp.CallToolRequest,
	input InjectInput,
) (*mcp.CallToolResult, InjectOutput, error) {
	injector, caller, err := service.injectorForRequest(ctx, request)
	if err != nil {
		return nil, InjectOutput{}, err
	}
	if refused, detail := service.selfCallerRefusal(caller); selfTarget(input.Target) && refused {
		return nil, InjectOutput{
			Status: "not_found", Code: inject.CodeUnknown, Message: detail,
		}, nil
	}
	result, err := injector.Inject(ctx, inject.Request{
		Target:   input.Target,
		Message:  input.Message,
		ForceNow: input.ForceNow,
		Then:     input.Then,
	})
	return nil, outputFromInject(result), err
}

func (service *Service) chatSelfCompact(
	ctx context.Context,
	request *mcp.CallToolRequest,
	input SelfCompactInput,
) (*mcp.CallToolResult, InjectOutput, error) {
	focus := strings.TrimSpace(input.Focus)
	if focus == "" || strings.ContainsAny(focus, "\r\n\x00") {
		return nil, InjectOutput{}, fmt.Errorf("focus must be one non-empty line")
	}
	injector, caller, err := service.injectorForRequest(ctx, request)
	if err != nil {
		return nil, InjectOutput{}, err
	}
	if refused, detail := service.selfCallerRefusal(caller); refused {
		if detail == noAmbientCallerRemedy {
			detail = selfCompactNoAmbientRemedy
		}
		return nil, InjectOutput{
			Status: "not_found", Code: inject.CodeUnknown, Message: detail,
		}, nil
	}
	// focus is validated above (single line, non-empty, no control characters),
	// which is exactly what makes it safe to concatenate onto the slash
	// command. isHarnessCommand only checks for a leading "/", so the composed
	// string still routes through the paced-literal command transport.
	//
	// Codex is the exception, and it is an ASSUMPTION HELD, not one disproved:
	// an earlier investigation recorded that Codex accepts no inline arguments
	// on /compact. Nothing in this repo re-tests that — TESTPLAN's /compact
	// rows are all Claude jail tests — so the claim stands until a real Codex
	// composer says otherwise, and the focus is composed only where the target
	// is NOT known to be Codex. (selfCompactNoAmbientRemedy's "/compact <focus>"
	// example does not contradict it: that text fires only on the Claude-over-
	// HTTP path, and its "a Codex chat should resolve automatically" clause is
	// about threadId identity resolution, not about command arguments.)
	//
	// An unresolved caller reaches here only via the per-chat stdio server
	// (selfCallerRefusal already refused the shared daemon), which pfm install
	// wires for "chat" on the Claude side alone — Codex stays on HTTP, where
	// its client always attaches a threadId and caller.valid is true. So an
	// ambient caller is Claude, and composing for it is correct.
	message := "/compact " + focus
	if caller.valid && caller.row.Engine == pfmengine.Codex {
		message = "/compact"
	}
	// One steer, by the operator's rule. The engine's own guards still run on
	// it — a steer is required, and it must not start with /compact — and a
	// blank string reaches them as no steer at all rather than as an empty one.
	var then []string
	if steer := strings.TrimSpace(input.Then); steer != "" {
		then = []string{steer}
	}
	result, err := injector.ScheduleAfterCurrentTurn(ctx, inject.Request{
		Target:  "self",
		Message: message,
		Then:    then,
	})
	// The stop notice is appended by the engine itself
	// (inject.SelfCompactStopNotice), which is the single writer for every
	// caller — MCP tool and `pfm chat inject` alike. Restating it here would
	// double it on the MCP path only.
	return nil, outputFromInject(result), err
}

// mcpUnsignedMessage restates inject.ErrUnsigned's refusal for an MCP
// caller. The engine's own wording (inject/body.go) is CLI-oriented — it
// tells the reader to set an environment variable or pass --allow-unsigned,
// both unreachable from an MCP tool call, and it blames a detached process
// chain that is not what happened here. The daemon-level cause is real (this
// process still derived no identity of its own), so only the remedy half is
// replaced with one an MCP caller can act on.
func mcpUnsignedMessage() string {
	return inject.ErrUnsigned.Error() +
		": this process derived no identity of its own, so the recipient " +
		"would be asked to act on an instruction from nobody. " +
		noAmbientCallerRemedy
}

func outputFromInject(result inject.Result) InjectOutput {
	message := result.Message
	if strings.Contains(message, inject.ErrUnsigned.Error()) {
		message = mcpUnsignedMessage()
	}
	return InjectOutput{
		Status:        result.Status,
		Code:          result.Code,
		Message:       message,
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
		AutoFilePath:  result.AutoFilePath,
		LiteralChunks: result.LiteralChunks,
	}
}

// chatWhoami answers with the requesting chat's identity. Codex sends its
// thread id in reserved protocol metadata. Only the per-chat stdio transport
// may use environment/ancestry; a shared daemon has no ambient caller identity.
func (service *Service) chatWhoami(
	ctx context.Context,
	request *mcp.CallToolRequest,
	_ WhoamiInput,
) (*mcp.CallToolResult, WhoamiOutput, error) {
	caller, err := service.backend.callerForRequest(ctx, requestMeta(request))
	if err != nil {
		return nil, WhoamiOutput{}, err
	}
	if caller.present {
		if !caller.valid {
			return nil, WhoamiOutput{Status: "not_found", Message: caller.detail}, nil
		}
		identity := caller.identity
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
	if !service.backend.allowAmbientIdentity {
		return nil, WhoamiOutput{
			Status:  "not_found",
			Message: noAmbientCallerRemedy,
		}, nil
	}
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
	request *mcp.CallToolRequest,
	input CaptureInput,
) (*mcp.CallToolResult, CaptureOutput, error) {
	lines := 40
	if input.TailLines != nil {
		lines = *input.TailLines
	}
	if lines < 1 || lines > 1000 {
		return nil, CaptureOutput{}, fmt.Errorf(
			"tail_lines must be between 1 and 1000",
		)
	}
	maxBytes := defaultCaptureBytes
	if input.MaxBytes != nil {
		maxBytes = *input.MaxBytes
	}
	if maxBytes < 1 || maxBytes > maxCaptureBytes {
		return nil, CaptureOutput{}, fmt.Errorf(
			"max_bytes must be between 1 and %d",
			maxCaptureBytes,
		)
	}
	injector, caller, err := service.injectorForRequest(ctx, request)
	if err != nil {
		return nil, CaptureOutput{}, err
	}
	if refused, detail := service.selfCallerRefusal(caller); selfTarget(input.Target) && refused {
		return nil, CaptureOutput{Status: "not_found", Code: inject.CodeUnknown, Message: detail}, nil
	}
	target, text, code, detail, err := injector.Capture(
		ctx,
		input.Target,
		lines,
	)
	status := "ok"
	if code == inject.CodeAmbiguous {
		status = "ambiguous"
	} else if code != 0 {
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
