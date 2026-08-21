package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"hostops/pfm/internal/statusline"
	"hostops/pfm/internal/usagehook"
)

var statuslineVertexOptions = func(ctx context.Context, log io.Writer) (statusline.VertexOptions, error) {
	project, err := statusline.ResolveVertexProject(ctx)
	if err != nil {
		return statusline.VertexOptions{}, err
	}
	return statusline.VertexOptions{
		Project:     project,
		AccessToken: statusline.GCloudAccessToken,
		Log:         log,
	}, nil
}

var statuslineGPTOptions = func() statusline.GPTOptions {
	return statusline.GPTOptions{}
}

func runStatusline(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	runtime, err := loadCommandRuntime("")
	if err != nil {
		fmt.Fprintf(stderr, "pfm statusline: load config (fail-open): %v\n", err)
		return 0
	}
	return runStatuslineWithRuntime(args, stdin, stdout, stderr, runtime)
}

func runStatuslineWithRuntime(
	args []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
	machine commandRuntime,
) int {
	flags := newFlagSet(
		"statusline",
		"usage: pfm statusline [--refresh-vertex | --refresh-gpt]",
		stderr,
	)
	refreshVertex := flags.Bool("refresh-vertex", false, "refresh the Vertex spend cache")
	refreshGPT := flags.Bool("refresh-gpt", false, "refresh the GPT usage cache")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || (*refreshVertex && *refreshGPT) {
		flags.Usage()
		return 2
	}

	ctx := context.Background()
	if *refreshVertex {
		options, err := statuslineVertexOptions(ctx, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "pfm statusline: resolve Vertex project: %v\n", err)
			return 1
		}
		if err := statusline.RefreshVertex(ctx, options); err != nil {
			fmt.Fprintf(stderr, "pfm statusline: refresh Vertex cache: %v\n", err)
			return 1
		}
		return 0
	}
	if *refreshGPT {
		options := statuslineGPTOptions()
		account := accountForCodexHome(machine.Config, os.Getenv("CODEX_HOME"))
		options.Binary = machine.Config.EffectiveCodex(account).Binary
		if err := statusline.RefreshGPT(ctx, options); err != nil {
			fmt.Fprintf(stderr, "pfm statusline: refresh GPT cache: %v\n", err)
			return 1
		}
		return 0
	}

	raw, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
	if err != nil {
		fmt.Fprintf(stderr, "pfm statusline: read input (fail-open): %v\n", err)
		return 0
	}
	runtime, err := statusline.DefaultRuntime()
	if err != nil {
		fmt.Fprintf(stderr, "pfm statusline: resolve runtime (fail-open): %v\n", err)
		return 0
	}
	if runtime.Engine == "codex" {
		runtime.AccountDirs = make(map[string]int, len(machine.Config.CodexAccounts))
		runtime.AccountEmojis = make(map[int]string, len(machine.Config.CodexAccounts))
		for _, account := range machine.Config.CodexAccounts {
			runtime.AccountDirs[canonicalAccountPath(account.Home)] = account.ID
			runtime.AccountEmojis[account.ID] = machine.Config.CodexEmojiFor(account.ID)
		}
	} else {
		runtime.AccountDirs = make(map[string]int, len(machine.Config.Accounts))
		runtime.AccountEmojis = make(map[int]string, len(machine.Config.Accounts))
		for _, account := range machine.Config.Accounts {
			runtime.AccountDirs[canonicalAccountPath(account.ConfigDir)] = account.ID
			runtime.AccountEmojis[account.ID] = machine.Config.EmojiFor(account.ID)
		}
	}
	runtime.Spawn = statusline.SpawnDetached
	rendered, err := statusline.Render(ctx, raw, runtime)
	if err != nil {
		fmt.Fprintf(stderr, "pfm statusline: render (fail-open): %v\n", err)
		return 0
	}
	if _, err := io.WriteString(stdout, rendered); err != nil {
		fmt.Fprintf(stderr, "pfm statusline: write output (fail-open): %v\n", err)
	}
	return 0
}

func canonicalAccountPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	if absolute, err := filepath.Abs(path); err == nil {
		return filepath.Clean(absolute)
	}
	return filepath.Clean(path)
}

func runUsageHook(args []string, stdout, stderr io.Writer) int {
	runtime, err := loadCommandRuntime("")
	if err != nil {
		fmt.Fprintf(stderr, "pfm usage-hook: load config (fail-open): %v\n", err)
		return 0
	}
	return runUsageHookWithRuntime(args, stdout, stderr, runtime)
}

func runUsageHookWithRuntime(
	args []string,
	stdout, stderr io.Writer,
	runtime commandRuntime,
) int {
	flags := newFlagSet("usage-hook", "usage: pfm usage-hook", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	if statusline.EngineFromEnvironment(os.Getenv) == "codex" {
		return 0
	}
	accountDirs := make(map[string]int, len(runtime.Config.Accounts))
	for _, account := range runtime.Config.Accounts {
		accountDirs[account.ConfigDir] = account.ID
	}
	message, err := usagehook.Evaluate(context.Background(), usagehook.Options{
		AccountDirs: accountDirs,
	})
	if err != nil {
		fmt.Fprintf(stderr, "pfm usage-hook: evaluate (fail-open): %v\n", err)
		return 0
	}
	if message != "" {
		if _, err := io.WriteString(stdout, message); err != nil {
			fmt.Fprintf(stderr, "pfm usage-hook: write output (fail-open): %v\n", err)
		}
	}
	return 0
}
