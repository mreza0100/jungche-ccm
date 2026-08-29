package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"hostops/pfm/internal/installer"
)

var initTemplatePaths = []struct{ source, target string }{
	{source: "project/CLAUDE.md", target: "CLAUDE.md"},
	{source: "project/CLAUDE.md", target: "AGENTS.md"},
	{source: "project/settings.json", target: ".claude/settings.json"},
	{source: "project/commands", target: ".claude/commands"},
	{source: "project/agents", target: ".claude/agents"},
	{source: "project/skills", target: ".claude/skills"},
}

func runInit(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet(
		"init",
		"usage: pfm init [dir] [--force]",
		stderr,
	)
	force := flags.Bool("force", false, "replace the scaffold in an existing .claude directory")
	positional, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(positional) > 1 {
		flags.Usage()
		return 2
	}
	target := "."
	if len(positional) == 1 {
		target = positional[0]
	}
	target, err := filepath.Abs(target)
	if err != nil {
		fmt.Fprintf(stderr, "pfm init: resolve target: %v\n", err)
		return 1
	}
	runtime, err := optionalCommandRuntime(runtimes)
	if err != nil {
		fmt.Fprintf(stderr, "pfm init: config: %v\n", err)
		return 1
	}
	source, err := installer.ReadSourceRepoMarker(runtime.Paths.Home)
	if err != nil {
		fmt.Fprintf(stderr, "pfm init: %v\n", err)
		return 1
	}
	if err := initScaffold(source, target, *force); err != nil {
		fmt.Fprintf(stderr, "pfm init: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "initialized %s from %s\n", target, source)
	fmt.Fprintf(stdout, "open Claude here and follow %s\n", filepath.Join(source, "docs", "SETUP.md"))
	return 0
}

func initScaffold(source, target string, force bool) error {
	templatesRoot := filepath.Join(source, "templates")
	type plannedCopy struct {
		source string
		target string
		info   fs.FileInfo
	}
	plan := make([]plannedCopy, 0, len(initTemplatePaths))
	for _, mapping := range initTemplatePaths {
		sourcePath := filepath.Join(templatesRoot, filepath.FromSlash(mapping.source))
		info, err := os.Stat(sourcePath)
		if err != nil {
			return fmt.Errorf("recorded clone templates dir is missing %s: %w", mapping.source, err)
		}
		plan = append(plan, plannedCopy{
			source: sourcePath,
			target: filepath.Join(target, filepath.FromSlash(mapping.target)),
			info:   info,
		})
	}
	claudeTarget := filepath.Join(target, ".claude")
	if _, err := os.Stat(claudeTarget); err == nil && !force {
		return errors.New("target already has .claude; rerun with --force to replace the scaffold")
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect target .claude: %w", err)
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		return fmt.Errorf("create target %s: %w", target, err)
	}
	if force {
		for _, entry := range plan {
			if err := os.RemoveAll(entry.target); err != nil {
				return fmt.Errorf("replace stale scaffold path %s: %w", entry.target, err)
			}
		}
	}
	for _, entry := range plan {
		if entry.info.IsDir() {
			if err := copyInitTree(entry.source, entry.target); err != nil {
				return fmt.Errorf("copy %s: %w", entry.source, err)
			}
			continue
		}
		if err := copyInitFile(entry.source, entry.target, entry.info.Mode().Perm()); err != nil {
			return fmt.Errorf("copy %s: %w", entry.source, err)
		}
	}
	return nil
}

func copyInitTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		mapped := target
		if relative != "." {
			mapped = filepath.Join(target, relative)
		}
		if entry.IsDir() {
			return os.MkdirAll(mapped, 0o700)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("symlink template entries are not allowed")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return copyInitFile(path, mapped, info.Mode().Perm())
	})
}

func copyInitFile(source, target string, mode os.FileMode) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".pfm-init-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode.Perm()); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, target)
}

// discoverSourceRepo finds the clone when install is launched from it. It is
// deliberately filesystem-only: installation never shells out to git.
func discoverSourceRepo() string {
	if value := strings.TrimSpace(os.Getenv("PFM_SOURCE_REPO")); value != "" {
		if absolute, err := filepath.Abs(value); err == nil {
			return absolute
		}
	}
	current, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if isSourceRepo(current) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func isSourceRepo(root string) bool {
	for _, relative := range []string{"CLAUDE.md", "AGENTS.md", ".claude/settings.json"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
			return false
		}
	}
	return true
}
