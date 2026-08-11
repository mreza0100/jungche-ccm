package action

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"hostops/cc-fleet/internal/compose"
)

// MaxAccount is the size of the Claude account roster. SOURCE OF TRUTH:
// cc-db.sh's primary-get / primary-set, whose `case "$n" in 1|2)` guard is a
// shell case statement with no machine-readable form — so the roster is
// mirrored here as a constant and every account check in this tree derives
// from it. Hardcoding a roster in each caller is what froze the picker's
// account cycle at 2 the day the roster stopped being three.
const MaxAccount = 2

// hygiene is the launch-environment strip every fleet-born process carries
// (cc-fleet.zsh:105, :119 for claude and :151 for codex). A chat born inside
// another chat's Bash tool inherits that chat's session identity, config dir
// and cache mode, so each one is unset and then re-decided by the launcher.
//
// The ANTHROPIC_*/CLAUDE_CODE_* tail is CC_ENDPOINT_UNSET (cc-fleet.zsh:80-84):
// a shell pointed at a local translating proxy would otherwise hand the next
// launch a foreign endpoint, and it would answer from a foreign model under an
// Anthropic medal. The launcher's verdict is the account; the environment gets
// no vote.
const hygiene = "env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u CLAUDE_CONFIG_DIR" +
	" -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M" +
	" -u ANTHROPIC_BASE_URL -u ANTHROPIC_AUTH_TOKEN -u ANTHROPIC_MODEL" +
	" -u ANTHROPIC_SMALL_FAST_MODEL -u CLAUDE_CODE_AUTO_COMPACT_WINDOW" +
	" -u CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC" +
	" -u CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK" +
	" -u CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY"

// autonomyFlags is CC_AUTONOMY_FLAGS (cc-fleet.zsh:75) — the full-autonomy
// posture every path that STARTS a Claude chat carries. `--allow-…` is the
// enabling half (the harness refuses the bypass without it), `--dangerously-…`
// the acting half; both are required. Chats run unattended overnight, so a
// mid-task approval prompt is a stalled chat with nobody awake to clear it.
//
// The resume routes append them AFTER the transcript argument, exactly as
// cc-fleet.zsh:872 and :1481 do. The fresh-launch routes emit a `cc1`/`cc2`
// call instead, and _cc_run already prepends the flags there — passing them
// again would duplicate them in argv (cc-fleet.zsh:138). Codex carries its own
// bypass flag and never these.
const autonomyFlags = "--allow-dangerously-skip-permissions --dangerously-skip-permissions"

// Synthesize produces a deterministic action plan without touching tmux,
// processes, the filesystem, stdin, stdout, or /dev/tty.
func Synthesize(request Request) (Plan, error) {
	route, err := routeForKind(request.Row.Kind)
	if err != nil {
		return Plan{}, err
	}
	if request.PrimaryAccount < 1 || request.PrimaryAccount > MaxAccount {
		return Plan{}, fmt.Errorf(
			"primary account must be 1-%d, got %d",
			MaxAccount,
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
	// A booting row carries no other identity than its socket, so Enter can
	// only ever attach it — the same Live route an ordinary live row takes,
	// which is exactly the "no other operations" the fix promises: it is
	// never resumable (no transcript exists yet to resume) and Open's own
	// mismatched-account gate and dead-socket resume fallback stay unreached
	// because this Kind is deliberately absent from the Kind list Open()
	// special-cases (executor.go).
	case compose.Booting:
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
	// Account 1 is the default config dir (hygiene already unset it); every
	// other account is an explicit CLAUDE_CONFIG_DIR (cc-fleet.zsh:88).
	if account >= 2 && account <= MaxAccount {
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
	command.WriteByte(' ')
	command.WriteString(autonomyFlags)
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
	// The installed location: install.sh symlinks the bundle's scripts into
	// ~/.claude/bin. The repo copy under work/host-ops/ was a pre-move path
	// that no longer exists on the host.
	return filepath.Join(request.Home, ".claude", "bin", "cc-agent-open.sh")
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
