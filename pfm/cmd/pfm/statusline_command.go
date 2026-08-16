package main

import (
	"context"
	"fmt"
	"io"

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
	return statusline.GPTOptions{ReadRateLimits: statusline.ReadGPTRateLimits}
}

func runStatusline(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
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
		if err := statusline.RefreshGPT(ctx, statuslineGPTOptions()); err != nil {
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

func runUsageHook(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("usage-hook", "usage: pfm usage-hook", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	message, err := usagehook.Evaluate(context.Background(), usagehook.Options{})
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
