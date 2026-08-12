package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"hostops/cc-fleet/internal/action"
	"hostops/cc-fleet/internal/compose"
	"hostops/cc-fleet/internal/naming"
	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/spawn"
	"hostops/cc-fleet/internal/store"
)

// runRun starts a chat with the fleet's whole launch ceremony — the
// environment strip, the account's config dir, the cache mode, the autonomy
// flags, its own tmux server on a fleet socket — and then walks away from it.
// No terminal is attached and nothing is eval'd by the caller's shell, so it
// works from a script, a cron job, or another chat's Bash tool.
func runRun(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"run",
		"usage: cc-fleet run --name NAME [--engine cc|cx] [--cwd DIR] "+
			"[--account N] [--1h] [prompt]",
		stderr,
	)
	name := flags.String("name", "", "chat name (a _HIDE… name stays out of the list)")
	engine := flags.String("engine", "cc", "engine: cc|claude or cx|codex")
	cwd := flags.String("cwd", "", "project directory (default: the current one)")
	account := flags.Int("account", 0, "Claude account (default: the primary one)")
	cache1H := flags.Bool("1h", false, "arm 1h prompt caching")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if *name == "" {
		flags.Usage()
		return 2
	}
	engineName, ok := action.NormalizeEngine(*engine)
	if !ok {
		fmt.Fprintf(stderr, "cc-fleet run: unknown engine %q\n", *engine)
		return 2
	}

	resolved, err := paths.Resolve()
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet run: %v\n", err)
		return 1
	}
	directory, err := runDirectory(*cwd)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet run: %v\n", err)
		return 1
	}
	claudeAccount := *account
	if claudeAccount == 0 {
		claudeAccount = readPrimaryAccount(resolved)
	}
	plan, err := action.HeadlessRun(action.HeadlessRequest{
		Engine:         engineName,
		Name:           *name,
		CWD:            directory,
		Prompt:         strings.Join(flags.Args(), " "),
		Home:           resolved.Home,
		PrimaryAccount: claudeAccount,
		Cache1H:        *cache1H,
	})
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet run: %v\n", err)
		return 2
	}

	kind := compose.NewClaude
	if engineName == store.CodexEngine {
		kind = compose.NewCodex
	}
	result, err := spawn.Run(context.Background(), spawn.CommandTmux{
		TmuxDir: resolved.TmuxDir,
	}, spawn.Request{
		Engine: engineName,
		Name:   *name,
		Socket: freshSocket(kind),
		CWD:    directory,
		Run:    plan.Run,
		Prompt: promptForTUI(plan, flags.Args()),
		Width:  action.HeadlessWidth,
		Height: action.HeadlessHeight,
	})
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet run: %v\n", err)
		return 1
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(stderr, "cc-fleet run: %s\n", warning)
	}
	printRunResult(stdout, engineName, result)
	if !result.Named {
		return 1
	}
	return 0
}

// promptForTUI is the prompt the spawner must type, which is empty whenever
// the engine already took it on its command line.
func promptForTUI(plan action.HeadlessPlan, args []string) string {
	if plan.PromptOnCommandLine {
		return ""
	}
	return strings.Join(args, " ")
}

func runDirectory(requested string) (string, error) {
	directory := requested
	if directory == "" {
		current, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("read current directory: %w", err)
		}
		return current, nil
	}
	info, err := os.Stat(directory)
	if err != nil {
		return "", fmt.Errorf("project directory %s: %w", directory, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project directory %s is not a directory", directory)
	}
	return directory, nil
}

func printRunResult(
	stdout io.Writer,
	engineName string,
	result spawn.Result,
) {
	state := "named"
	if !result.Named {
		state = "UNNAMED"
	}
	listing := "listed"
	if naming.LabelHidden(result.Name) {
		listing = "hidden by its " + naming.HidePrefix + " name"
	}
	fmt.Fprintf(
		stdout,
		"%s\t%s\t%s\t%s\t%s\n",
		engineName,
		result.Name,
		result.Socket,
		state,
		listing,
	)
	fmt.Fprintf(
		stdout,
		"attach: tmux -L %s attach -t %s\n",
		result.Socket,
		result.Session,
	)
}
