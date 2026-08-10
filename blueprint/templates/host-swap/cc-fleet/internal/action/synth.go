package action

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"hostops/cc-fleet/internal/compose"
)

const hygiene = "env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u CLAUDE_CONFIG_DIR -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M"

// Synthesize produces a deterministic action plan without touching tmux,
// processes, the filesystem, stdin, stdout, or /dev/tty.
func Synthesize(request Request) (Plan, error) {
	route, err := routeForKind(request.Row.Kind)
	if err != nil {
		return Plan{}, err
	}
	if request.PrimaryAccount < 1 || request.PrimaryAccount > 3 {
		return Plan{}, fmt.Errorf(
			"primary account must be 1, 2, or 3, got %d",
			request.PrimaryAccount,
		)
	}
	if hasNUL(
		request.Row.ID,
		request.Row.CWD,
		request.Row.Socket,
		request.Row.SessionName,
		request.Row.WindowName,
		request.Row.Name,
		request.Row.ConfigDir,
		request.Home,
		request.FreshSocket,
		request.AgentScript,
	) {
		return Plan{}, errors.New("action values cannot contain NUL")
	}

	plan := Plan{Route: route}
	switch route {
	case NewClaude:
		if request.Row.CWD == "" {
			return Plan{}, errors.New("new Claude action requires a project directory")
		}
		command := "cc" + strconv.Itoa(request.PrimaryAccount)
		armed := "CC_ARM_1H=0 ENABLE_PROMPT_CACHING_1H=0 "
		if request.Cache1H {
			armed = "CC_ARM_1H=1 "
		}
		plan.Line = "(cd -- " + Quote(request.Row.CWD) + " && " +
			armed + command + ")"
	case NewCodex:
		if request.Row.CWD == "" {
			return Plan{}, errors.New("new Codex action requires a project directory")
		}
		plan.Line = "(cd -- " + Quote(request.Row.CWD) + " && cx)"
	case Live:
		if request.Row.Socket == "" {
			return Plan{}, errors.New("live action requires a socket")
		}
		plan.Line = attachLine(
			request.Row.Socket,
			liveTarget(request.Row),
			request.Bunker,
		)
	case Agent:
		if request.Row.ID == "" || request.Row.CWD == "" ||
			request.FreshSocket == "" {
			return Plan{}, errors.New(
				"agent action requires id, cwd, and fresh socket",
			)
		}
		agentScript := agentScriptPath(request)
		owningConfig := request.Row.ConfigDir
		if filepath.Clean(owningConfig) == filepath.Join(request.Home, ".claude") {
			owningConfig = ""
		}
		agentRun := agentCommand(
			request.Cache1H,
			agentScript,
			request.Row.ID,
			request.Row.CWD,
			owningConfig,
		)
		resume := claudeCommand(
			request.Home,
			request.PrimaryAccount,
			request.Cache1H,
			"--resume",
			request.Row.ID,
		)
		plan.Run = agentRun +
			" || { echo; echo " +
			Quote("agent router failed — resuming fresh:") +
			"; exec " + resume + "; }"
		plan.Line = newSessionLine(
			request.FreshSocket,
			request.Row.CWD,
			plan.Run,
			request.Bunker,
		)
	case ResumeClaude:
		if request.Row.ID == "" || request.Row.CWD == "" ||
			request.FreshSocket == "" {
			return Plan{}, errors.New(
				"Claude resume requires id, cwd, and fresh socket",
			)
		}
		resume := claudeCommand(
			request.Home,
			request.PrimaryAccount,
			request.Cache1H,
			"--resume",
			request.Row.ID,
		)
		agent := agentCommand(
			request.Cache1H,
			agentScriptPath(request),
			request.Row.ID,
			request.Row.CWD,
		)
		plan.Run = resume +
			" || { echo; echo " +
			Quote("resume refused — session is live elsewhere:") +
			"; " + agent + "; }"
		plan.Line = newSessionLine(
			request.FreshSocket,
			request.Row.CWD,
			plan.Run,
			request.Bunker,
		)
	case ResumeCodex:
		if request.Row.ID == "" || request.Row.CWD == "" ||
			request.FreshSocket == "" {
			return Plan{}, errors.New(
				"Codex resume requires id, cwd, and fresh socket",
			)
		}
		plan.Run = codexCommand("resume", request.Row.ID)
		plan.CodexServer = &CodexServer{
			Socket: request.FreshSocket,
			CWD:    request.Row.CWD,
			Run:    plan.Run,
		}
		plan.Line = attachLine(
			request.FreshSocket,
			request.FreshSocket+":Codex",
			request.Bunker,
		)
	}
	return plan, nil
}

func routeForKind(kind compose.Kind) (Route, error) {
	switch kind {
	case compose.NewClaude:
		return NewClaude, nil
	case compose.NewCodex:
		return NewCodex, nil
	case compose.LiveClaude, compose.LiveCodex, compose.LiveSplit:
		return Live, nil
	case compose.Agent:
		return Agent, nil
	case compose.ResumeClaude:
		return ResumeClaude, nil
	case compose.ResumeCodex:
		return ResumeCodex, nil
	default:
		return 0, fmt.Errorf("unsupported row kind %s", kind)
	}
}

func claudeCommand(
	home string,
	account int,
	cache1H bool,
	args ...string,
) string {
	var command strings.Builder
	command.WriteString(hygiene)
	if account == 2 || account == 3 {
		command.WriteString(" CLAUDE_CONFIG_DIR=")
		command.WriteString(Quote(filepath.Join(home, ".cc", strconv.Itoa(account))))
	}
	if cache1H {
		command.WriteString(" ENABLE_PROMPT_CACHING_1H=1")
	} else {
		command.WriteString(" FORCE_PROMPT_CACHING_5M=1")
	}
	command.WriteString(" claude")
	for _, argument := range args {
		command.WriteByte(' ')
		command.WriteString(Quote(argument))
	}
	return command.String()
}

func codexCommand(args ...string) string {
	var command strings.Builder
	command.WriteString(hygiene)
	command.WriteString(" codex --dangerously-bypass-approvals-and-sandbox")
	for _, argument := range args {
		command.WriteByte(' ')
		command.WriteString(Quote(argument))
	}
	return command.String()
}

func agentCommand(
	cache1H bool,
	script, id, cwd string,
	configDir ...string,
) string {
	var command strings.Builder
	command.WriteString(hygiene)
	if cache1H {
		command.WriteString(" ENABLE_PROMPT_CACHING_1H=1")
	} else {
		command.WriteString(" FORCE_PROMPT_CACHING_5M=1")
	}
	command.WriteString(" bash ")
	command.WriteString(Quote(script))
	command.WriteByte(' ')
	command.WriteString(Quote(id))
	command.WriteByte(' ')
	command.WriteString(Quote(cwd))
	if len(configDir) != 0 {
		command.WriteByte(' ')
		command.WriteString(Quote(configDir[0]))
	}
	return command.String()
}

func agentScriptPath(request Request) string {
	if request.AgentScript != "" {
		return request.AgentScript
	}
	return filepath.Join(
		request.Home,
		"work",
		"host-ops",
		"oldbox",
		"scripts",
		"cc-agent-open.sh",
	)
}

func newSessionLine(socket, cwd, run string, bunker bool) string {
	prefix := "TMUX= "
	if bunker {
		prefix += "exec "
	}
	return prefix + "tmux -L " + Quote(socket) +
		" new-session -s " + Quote(socket) +
		" -c " + Quote(cwd) + " " + Quote(run)
}

func attachLine(socket, target string, bunker bool) string {
	prefix := "TMUX= "
	if bunker {
		prefix += "exec "
	}
	line := prefix + "tmux -L " + Quote(socket) + " attach"
	if target != "" {
		line += " -t " + Quote(target)
	}
	return line
}

func liveTarget(row compose.Row) string {
	if row.Kind == compose.LiveCodex {
		if row.SessionName != "" && row.WindowName != "" {
			return row.SessionName + ":" + row.WindowName
		}
	}
	if row.SessionName != "" {
		return row.SessionName
	}
	return row.Socket
}
