package action

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"hostops/pfm/internal/compose"
	pfmconfig "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
)

// hygiene is the launch-environment strip every fleet-born process carries
// for both Claude and Codex. A chat born inside
// another chat's Bash tool inherits that chat's session identity, config dir
// and cache mode, so each one is unset and then re-decided by the launcher.
//
// The ANTHROPIC_*/CLAUDE_CODE_* tail is CC_ENDPOINT_UNSET:
// a shell pointed at a local translating proxy would otherwise hand the next
// launch a foreign endpoint, and it would answer from a foreign model under an
// Anthropic medal. The launcher's verdict is the account; the environment gets
// no vote.
const hygiene = "env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u CLAUDE_CODE_CHILD_SESSION -u CLAUDE_CONFIG_DIR" +
	" -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M" +
	" -u ANTHROPIC_BASE_URL -u ANTHROPIC_AUTH_TOKEN -u ANTHROPIC_MODEL" +
	" -u ANTHROPIC_SMALL_FAST_MODEL -u CLAUDE_CODE_AUTO_COMPACT_WINDOW" +
	" -u CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC" +
	" -u CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK" +
	" -u CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY"

// LauncherRun is the one argv-preserving Claude spawn command used by the
// managed launcher. It shares the fleet hygiene list with picker/headless
// launches, but intentionally adds no autonomy or resume flags of its own.
func LauncherRun(real string, args []string, configDir string) (string, error) {
	values := append([]string{real, configDir}, args...)
	if hasNUL(values...) {
		return "", errors.New("launcher values cannot contain NUL")
	}
	if real == "" {
		return "", errors.New("real Claude binary is empty")
	}
	var command strings.Builder
	command.WriteString(hygiene)
	if configDir != "" {
		command.WriteString(" CLAUDE_CONFIG_DIR=")
		command.WriteString(Quote(configDir))
	}
	command.WriteString(" FORCE_PROMPT_CACHING_5M=1 ")
	command.WriteString(Quote(real))
	for _, argument := range args {
		command.WriteByte(' ')
		command.WriteString(Quote(argument))
	}
	return command.String(), nil
}

// autonomyFlags is CC_AUTONOMY_FLAGS — the full-autonomy
// posture every path that STARTS a Claude chat carries. `--allow-…` is the
// enabling half (the harness refuses the bypass without it), `--dangerously-…`
// the acting half; both are required. Chats run unattended overnight, so a
// mid-task approval prompt is a stalled chat with nobody awake to clear it.
//
// The resume routes append them AFTER the transcript argument, exactly as
// the interactive launcher does. The fresh-launch routes emit a `cc1`/`cc2`
// call instead, and _cc_run already prepends the flags there — passing them
// again would duplicate them in argv. Codex carries its own
// bypass flag and never these.
const autonomyFlags = "--allow-dangerously-skip-permissions --dangerously-skip-permissions"

// Synthesize produces a deterministic action plan without touching tmux,
// processes, the filesystem, stdin, stdout, or /dev/tty.
func Synthesize(request Request) (Plan, error) {
	machine := normalizedMachineConfig(request.Config, request.Home)
	route, err := routeForKind(request.Row.Kind)
	if err != nil {
		return Plan{}, err
	}
	switch route {
	case ResumeOpencode:
		// One fleet-wide OpenCode seat: no per-account roster to satisfy, the
		// binary comes from the machine config's opencode section.
	case NewClaude, Agent, ResumeClaude:
		if _, found := machine.Account(request.PrimaryAccount); !found {
			return Plan{}, fmt.Errorf(
				"Claude account %d is not in the configured roster",
				request.PrimaryAccount,
			)
		}
	case NewCodex, ResumeCodex:
		if _, found := machine.CodexAccountByID(request.PrimaryAccount); !found {
			return Plan{}, fmt.Errorf(
				"Codex account %d is not in the configured roster",
				request.PrimaryAccount,
			)
		}
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
	) {
		return Plan{}, errors.New("action values cannot contain NUL")
	}

	plan := Plan{Route: route}
	switch route {
	case NewClaude:
		if request.Row.CWD == "" {
			return Plan{}, errors.New("new Claude action requires a project directory")
		}
		command := string(pfmengine.Claude) + strconv.Itoa(request.PrimaryAccount)
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
		account, _ := machine.CodexAccountByID(request.PrimaryAccount)
		plan.Line = "(cd -- " + Quote(request.Row.CWD) + " && CODEX_HOME=" +
			Quote(account.Home) + " cx)"
	case ResumeOpencode:
		if request.Row.ID == "" || request.Row.CWD == "" ||
			request.FreshSocket == "" {
			return Plan{}, errors.New(
				"OpenCode resume requires id, cwd, and fresh socket",
			)
		}
		var command strings.Builder
		command.WriteString(hygiene)
		command.WriteByte(' ')
		command.WriteString(binaryWord(
			machine.OpenCode.Binary,
			pfmengine.MustLookup(pfmengine.Opencode).Binary,
			machine.Source("opencode.binary") == pfmconfig.SourceFile,
		))
		command.WriteString(" --session ")
		command.WriteString(Quote(request.Row.ID))
		command.WriteByte(' ')
		command.WriteString(Quote(request.Row.CWD))
		plan.Run = opencodeContinuityBanner(request.Row) + command.String()
		plan.Line = newSessionLine(
			request.FreshSocket,
			request.Row.CWD,
			plan.Run,
			request.Bunker,
		)
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
		owningConfig := request.Row.ConfigDir
		if filepath.Clean(owningConfig) == filepath.Join(request.Home, ".claude") {
			owningConfig = ""
		}
		agentRun := agentCommand(
			request.Cache1H,
			request.Row.ID,
			request.Row.CWD,
			machine.Path,
			owningConfig,
		)
		resume := claudeCommand(
			request.Home,
			request.PrimaryAccount,
			request.Cache1H,
			machine,
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
			machine,
			"--resume",
			request.Row.ID,
		)
		agent := agentCommand(
			request.Cache1H,
			request.Row.ID,
			request.Row.CWD,
			machine.Path,
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
		plan.Run = continuityBanner(request.Row) +
			codexCommandFor(machine, request.PrimaryAccount, "resume", request.Row.ID)
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
	case compose.ResumeOpencode:
		return ResumeOpencode, nil
	default:
		return 0, fmt.Errorf("unsupported row kind %s", kind)
	}
}

func claudeCommand(
	home string,
	account int,
	cache1H bool,
	machine pfmconfig.Config,
	args ...string,
) string {
	return claudeCommandWith(hygiene, home, account, cache1H, machine, args...)
}

// claudeCommandWith is claudeCommand over a caller-chosen environment strip,
// so the headless route can widen the strip without owning a second copy of
// the account, cache and autonomy decisions.
func claudeCommandWith(
	environmentStrip string,
	home string,
	account int,
	cache1H bool,
	machine pfmconfig.Config,
	args ...string,
) string {
	var command strings.Builder
	command.WriteString(environmentStrip)
	// Account 1 is the default config dir (hygiene already unset it); every
	// other account is an explicit CLAUDE_CONFIG_DIR.
	if selected, found := machine.Account(account); found && !selected.Implicit {
		command.WriteString(" CLAUDE_CONFIG_DIR=")
		command.WriteString(Quote(selected.ConfigDir))
	}
	if cache1H {
		command.WriteString(" ENABLE_PROMPT_CACHING_1H=1")
	} else {
		command.WriteString(" FORCE_PROMPT_CACHING_5M=1")
	}
	command.WriteByte(' ')
	command.WriteString(binaryWord(
		machine.Claude.Binary,
		pfmengine.MustLookup(pfmengine.Claude).Binary,
		machine.Source("claude.binary") == pfmconfig.SourceFile,
	))
	for _, argument := range args {
		command.WriteByte(' ')
		command.WriteString(Quote(argument))
	}
	if machine.EffectiveClaude(account).PermissionMode == pfmconfig.PermissionBypass {
		command.WriteByte(' ')
		command.WriteString(autonomyFlags)
	}
	return command.String()
}

// continuityBanner is the first thing a resumed Codex pane prints, above
// Codex's own boot chrome.
//
// A resume lands on a fresh tmux server, so the pane opens with an empty
// scrollback and Codex repaints its first-run screen. That screen is
// indistinguishable from a cold start, and a reader who trusts it concludes
// the seat lost its memory — an orchestrator re-briefed a seat carrying 2.5M
// tokens as if it were a replacement.
//
// The banner therefore says what is CHECKABLE and asserts nothing it cannot
// see. It must never claim the context survived: whether Codex rehydrates a
// thread is its own decision, made after this line is printed, and a
// `history_mode = paginated` thread reloads from Codex's history store rather
// than from its rollout — that store can hold a handful of items for a
// conversation whose rollout is complete, and the resume then comes up empty
// while the transcript on disk is whole. A banner that asserted continuity
// there would be the very failure it exists to prevent: a surface reporting a
// state it never verified. So it names the id, points at the transcript, and
// tells the reader which pixel decides the question.
//
// Codex writes its TUI into the normal buffer rather than the alternate
// screen, so these lines stay the oldest entries in the pane's scrollback for
// the life of the seat — the exact position a reader checks when asking "did
// this start fresh?".
func continuityBanner(row compose.Row) string {
	lines := []string{
		"═══ RESUME of " + row.ID + " — verify before trusting this pane ═══",
		"    Loaded? Read the status line: no context/token counts means Codex" +
			" did NOT rehydrate this thread.",
		"    Scrollback is never restored — the prior tmux server is reaped on" +
			" kill.",
		"    Came up empty? The rollout is whole: pfm chat recover " + row.ID,
	}
	if row.Path != "" {
		lines = append(
			lines,
			"    Transcript, complete whatever this pane shows: "+row.Path,
		)
	}
	var banner strings.Builder
	banner.WriteString("printf '%s\\n'")
	for _, line := range lines {
		banner.WriteByte(' ')
		banner.WriteString(Quote(line))
	}
	banner.WriteString("; ")
	return banner.String()
}

// opencodeContinuityBanner is the OpenCode twin of continuityBanner, minus
// everything Codex-specific. It names the id and states what is checkable;
// whether OpenCode rehydrated the session is its own decision, so the banner
// asserts nothing about memory.
func opencodeContinuityBanner(row compose.Row) string {
	lines := []string{
		"═══ RESUME of " + row.ID + " — verify before trusting this pane ═══",
		"    History loaded? Check the session list above the composer before",
		"    treating this seat as continuing its prior work.",
	}
	var banner strings.Builder
	banner.WriteString("printf '%s\\n'")
	for _, line := range lines {
		banner.WriteByte(' ')
		banner.WriteString(Quote(line))
	}
	banner.WriteString("; ")
	return banner.String()
}

func codexCommand(machine pfmconfig.Config, args ...string) string {
	return codexCommandFor(machine, 1, args...)
}

func codexCommandFor(machine pfmconfig.Config, account int, args ...string) string {
	return codexCommandWithAccount(hygiene, machine, account, args...)
}

func codexCommandWith(
	environmentStrip string,
	machine pfmconfig.Config,
	args ...string,
) string {
	return codexCommandWithAccount(environmentStrip, machine, 1, args...)
}

func codexCommandWithAccount(
	environmentStrip string,
	machine pfmconfig.Config,
	account int,
	args ...string,
) string {
	var command strings.Builder
	policy := machine.EffectiveCodex(account)
	command.WriteString(environmentStrip)
	if selected, found := machine.CodexAccountByID(account); found {
		command.WriteString(" CODEX_HOME=")
		command.WriteString(Quote(selected.Home))
	}
	command.WriteByte(' ')
	command.WriteString(binaryWord(
		policy.Binary,
		pfmengine.MustLookup(pfmengine.Codex).Binary,
		policy.Binary != "" && policy.Binary != pfmengine.MustLookup(pfmengine.Codex).Binary,
	))
	if policy.Yolo {
		command.WriteString(" --dangerously-bypass-approvals-and-sandbox")
	} else {
		command.WriteString(" --sandbox workspace-write")
	}
	for _, argument := range args {
		command.WriteByte(' ')
		command.WriteString(Quote(argument))
	}
	return command.String()
}

func normalizedMachineConfig(machine pfmconfig.Config, home string) pfmconfig.Config {
	if machine.Version == pfmconfig.Version &&
		(len(machine.Accounts) != 0 || len(machine.CodexAccounts) != 0) {
		return machine
	}
	return pfmconfig.Defaults(home, nil)
}

func binaryWord(value, fallback string, quote bool) string {
	if value == "" {
		value = fallback
	}
	if quote {
		return Quote(value)
	}
	return value
}

func agentCommand(
	cache1H bool,
	id, cwd, machineConfigPath string,
	configDir ...string,
) string {
	var command strings.Builder
	command.WriteString(hygiene)
	if cache1H {
		command.WriteString(" ENABLE_PROMPT_CACHING_1H=1")
	} else {
		command.WriteString(" FORCE_PROMPT_CACHING_5M=1")
	}
	command.WriteString(" pfm")
	if machineConfigPath != "" {
		command.WriteString(" --config ")
		command.WriteString(Quote(machineConfigPath))
	}
	command.WriteString(" internal agent-open --id ")
	command.WriteString(Quote(id))
	command.WriteString(" --cwd ")
	command.WriteString(Quote(cwd))
	if len(configDir) != 0 && configDir[0] != "" {
		command.WriteString(" --config ")
		command.WriteString(Quote(configDir[0]))
	}
	return command.String()
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
