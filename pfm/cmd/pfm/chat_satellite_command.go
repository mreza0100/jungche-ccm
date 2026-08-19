package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"hostops/pfm/internal/action"
	"hostops/pfm/internal/compose"
	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/headless"
	"hostops/pfm/internal/naming"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/policy"
	"hostops/pfm/internal/shared"
	"hostops/pfm/internal/spawn"
	"hostops/pfm/internal/store"
	"hostops/pfm/internal/transcript"
)

type transcriptMatch struct {
	ID, Path, First, Last string
	Hits, Needles         int
}

func runChatFind(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet("chat find", "usage: pfm chat find <excerpt-file>", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	match, alternatives, err := findTranscript(flags.Arg(0), runtimes...)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat find: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "Matched session: %s (%d/%d needles hit)\n", match.ID, match.Hits, match.Needles)
	if match.First != "" || match.Last != "" {
		fmt.Fprintf(stderr, "Range: %s -> %s\n", match.First, match.Last)
	}
	if len(alternatives) > 0 {
		fmt.Fprintln(stderr, "Other candidates (hits, file):")
		for _, candidate := range alternatives {
			fmt.Fprintf(stderr, "  %d %s\n", candidate.Hits, candidate.Path)
		}
	}
	fmt.Fprintf(stdout, "%s\t%s\n", match.ID, match.Path)
	return 0
}

func findTranscript(excerptPath string, runtimes ...commandRuntime) (transcriptMatch, []transcriptMatch, error) {
	content, err := os.ReadFile(excerptPath)
	if err != nil {
		return transcriptMatch{}, nil, err
	}
	return findTranscriptContent(content, runtimes...)
}

func findTranscriptContent(content []byte, runtimes ...commandRuntime) (transcriptMatch, []transcriptMatch, error) {
	needles := excerptNeedles(string(content))
	if len(needles) == 0 {
		return transcriptMatch{}, nil, errors.New("excerpt has no line of 20+ characters to search for")
	}
	var resolved paths.Values
	var err error
	if len(runtimes) != 0 {
		resolved = runtimes[0].Paths
	} else {
		resolved, err = paths.Resolve()
		if err != nil {
			return transcriptMatch{}, nil, err
		}
	}
	includeLegacyPrimary := len(runtimes) == 0 ||
		runtimes[0].Config.Source("accounts") != pfmconfig.SourceFile
	files, err := claudeTranscriptFiles(resolved, includeLegacyPrimary)
	if err != nil {
		return transcriptMatch{}, nil, err
	}
	if len(files) == 0 {
		return transcriptMatch{}, nil, errors.New("no transcript registry is available")
	}
	current := os.Getenv("CLAUDE_CODE_SESSION_ID")
	var matches []transcriptMatch
	for _, path := range files {
		id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if id == current {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return transcriptMatch{}, nil, fmt.Errorf("read transcript %s: %w", path, err)
		}
		hits := 0
		for _, needle := range needles {
			encoded, _ := json.Marshal(needle)
			if len(encoded) >= 2 && bytes.Contains(raw, encoded[1:len(encoded)-1]) {
				hits++
			}
		}
		if hits == 0 {
			continue
		}
		first, last := transcriptRange(raw)
		matches = append(matches, transcriptMatch{ID: id, Path: path, First: first, Last: last, Hits: hits, Needles: len(needles)})
	}
	if len(matches) == 0 {
		return transcriptMatch{}, nil, errors.New("no session contains the excerpt; try a longer or more distinctive chunk")
	}
	sort.Slice(matches, func(left, right int) bool {
		if matches[left].Hits != matches[right].Hits {
			return matches[left].Hits > matches[right].Hits
		}
		return matches[left].Path < matches[right].Path
	})
	alternatives := matches[1:]
	if len(alternatives) > 4 {
		alternatives = alternatives[:4]
	}
	return matches[0], alternatives, nil
}

func excerptNeedles(value string) []string {
	var candidates []string
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSuffix(line, "\r")
		line = strings.TrimLeftFunc(line, func(r rune) bool {
			return unicode.IsSpace(r) || strings.ContainsRune(">#*-", r)
		})
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if len(line) >= 20 {
			candidates = append(candidates, line)
		}
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		return len(candidates[left]) > len(candidates[right])
	})
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	return candidates
}

func claudeTranscriptFiles(resolved paths.Values, includeLegacyPrimary bool) ([]string, error) {
	roots := append([]string(nil), resolved.ClaudeRoots...)
	if includeLegacyPrimary {
		roots = append([]string{filepath.Join(resolved.Home, ".claude", "projects")}, roots...)
	}
	seen := make(map[string]struct{})
	var files []string
	for _, root := range roots {
		projects, err := os.ReadDir(root)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read transcript registry %s: %w", root, err)
		}
		for _, project := range projects {
			if !project.IsDir() {
				continue
			}
			directory := filepath.Join(root, project.Name())
			entries, err := os.ReadDir(directory)
			if err != nil {
				return nil, fmt.Errorf("read transcript project %s: %w", directory, err)
			}
			for _, entry := range entries {
				if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
					continue
				}
				path := filepath.Join(directory, entry.Name())
				if _, exists := seen[path]; exists {
					continue
				}
				seen[path] = struct{}{}
				files = append(files, path)
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

func transcriptRange(raw []byte) (string, string) {
	var first, last string
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	for scanner.Scan() {
		var record struct {
			Timestamp string `json:"timestamp"`
		}
		if json.Unmarshal(scanner.Bytes(), &record) == nil && record.Timestamp != "" {
			if first == "" {
				first = record.Timestamp
			}
			last = record.Timestamp
		}
	}
	return first, last
}

func runChatReadExcerpt(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(stderr, "usage: pfm chat read <excerpt-file> [last-N-lines]")
		return 2
	}
	limit := 0
	if len(args) == 2 {
		value, err := strconv.Atoi(args[1])
		if err != nil || value < 1 {
			fmt.Fprintln(stderr, "usage: pfm chat read <excerpt-file> [last-N-lines]")
			return 2
		}
		limit = value
	}
	match, _, err := findTranscript(args[0], runtimes...)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat read: %v\n", err)
		return 2
	}
	entries, err := transcript.All(context.Background(), match.Path, store.ClaudeEngine)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat read: %v\n", err)
		return 1
	}
	body := renderTranscript(entries)
	if limit > 0 {
		body = lastLines(body, limit)
	}
	if err := os.MkdirAll(filepath.Join("tmp", "chat-loads"), 0o700); err != nil {
		fmt.Fprintf(stderr, "pfm chat read: create output directory: %v\n", err)
		return 1
	}
	out := filepath.Join("tmp", "chat-loads", match.ID+".md")
	var document strings.Builder
	fmt.Fprintf(&document, "# Loaded chat — session %s\n\nSource: %s\nRange: %s -> %s\nVisible chat text only — thinking and tool outputs are not recorded here.\n", match.ID, match.Path, match.First, match.Last)
	if limit > 0 {
		fmt.Fprintf(&document, "(last %d lines)\n", limit)
	}
	document.WriteString("\n")
	document.WriteString(body)
	if err := os.WriteFile(out, []byte(document.String()), 0o600); err != nil {
		fmt.Fprintf(stderr, "pfm chat read: write %s: %v\n", out, err)
		return 1
	}
	fmt.Fprintf(stdout, "Extracted -> %s (%d lines)\n", out, strings.Count(document.String(), "\n"))
	return 0
}

func renderTranscript(entries []transcript.Entry) string {
	var output strings.Builder
	for _, entry := range entries {
		switch entry.Role {
		case transcript.RoleUser:
			fmt.Fprintf(&output, "## USER\n\n%s\n\n", entry.Text)
		case transcript.RoleAssistant:
			fmt.Fprintf(&output, "## ASSISTANT\n\n%s\n\n", entry.Text)
		case transcript.RoleSummary:
			fmt.Fprintf(&output, "## [COMPACTION SUMMARY]\n\n%s\n\n", entry.Text)
		case transcript.RoleTool:
			fmt.Fprintf(&output, "> [tools: %s]\n\n", entry.Tool)
		}
	}
	return output.String()
}

func lastLines(value string, count int) string {
	lines := strings.Split(value, "\n")
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	return strings.Join(lines, "\n")
}

func runChatSave(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(stderr, "usage: pfm chat save <target-file> [transcript-jsonl]")
		return 2
	}
	target := args[0]
	transcriptPath := ""
	if len(args) == 2 {
		transcriptPath = args[1]
	} else {
		id := os.Getenv("CLAUDE_CODE_SESSION_ID")
		if id == "" {
			fmt.Fprintln(stderr, "pfm chat save: CLAUDE_CODE_SESSION_ID is not set and no transcript path was given")
			return 1
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat save: current directory: %v\n", err)
			return 1
		}
		transcriptPath = currentClaudeTranscriptPath(id, cwd, runtimes...)
		if transcriptPath == "" {
			fmt.Fprintln(stderr, "pfm chat save: could not resolve the current Claude transcript")
			return 1
		}
	}
	entries, err := transcript.All(context.Background(), transcriptPath, store.ClaudeEngine)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat save: read transcript: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil && filepath.Dir(target) != "." {
		fmt.Fprintf(stderr, "pfm chat save: create target directory: %v\n", err)
		return 1
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat save: open target: %v\n", err)
		return 1
	}
	defer file.Close()
	if _, err := fmt.Fprintf(file, "\n---\n\n# FULL TRANSCRIPT (script-dumped, verbatim)\n\nVisible chat text only — thinking and tool outputs are not recorded here.\nSource: %s\n\n%s---\n\n# ENVIRONMENT SNAPSHOT (script-dumped)\n\n", transcriptPath, renderTranscript(entries)); err != nil {
		fmt.Fprintf(stderr, "pfm chat save: write transcript: %v\n", err)
		return 1
	}
	writeRepositorySnapshot(file)
	if err := file.Close(); err != nil {
		fmt.Fprintf(stderr, "pfm chat save: close target: %v\n", err)
		return 1
	}
	info, err := os.Stat(target)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat save: stat target: %v\n", err)
		return 1
	}
	users := 0
	for _, entry := range entries {
		if entry.Role == transcript.RoleUser {
			users++
		}
	}
	fmt.Fprintf(stdout, "Appended transcript (%d user records) + env snapshot -> %s (%d bytes total)\n", users, target, info.Size())
	return 0
}

func writeRepositorySnapshot(writer io.Writer) {
	inside := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	if err := inside.Run(); err != nil {
		fmt.Fprintln(writer, "(not a git repository)")
		return
	}
	branch, branchErr := exec.Command("git", "branch", "--show-current").Output()
	status, statusErr := exec.Command("git", "status", "--short").Output()
	worktrees, worktreeErr := exec.Command("git", "worktree", "list").Output()
	if branchErr != nil || statusErr != nil || worktreeErr != nil {
		fmt.Fprintf(writer, "(repository snapshot failed: branch=%v status=%v worktrees=%v)\n", branchErr, statusErr, worktreeErr)
		return
	}
	fmt.Fprintf(writer, "Branch: %s\n\n```\n%s```\n\nWorktrees:\n```\n%s```\n", strings.TrimSpace(string(branch)), status, worktrees)
}

func runChatLoad(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: pfm chat load <dir-or-file>...")
		return 2
	}
	unique := make(map[string]struct{})
	for _, target := range args {
		info, err := os.Stat(target)
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(stderr, "WARN: not found: %s\n", target)
			continue
		}
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat load: stat %s: %v\n", target, err)
			return 1
		}
		if !info.IsDir() {
			unique[target] = struct{}{}
			continue
		}
		if err := filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() && path != target && skippedLoadDir(entry.Name()) {
				return filepath.SkipDir
			}
			if !entry.IsDir() {
				unique[path] = struct{}{}
			}
			return nil
		}); err != nil {
			fmt.Fprintf(stderr, "pfm chat load: walk %s: %v\n", target, err)
			return 1
		}
	}
	files := make([]string, 0, len(unique))
	for path := range unique {
		files = append(files, path)
	}
	sort.Strings(files)
	count, total := 0, 0
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "pfm chat load: read %s: %v\n", path, err)
			return 1
		}
		if len(raw) == 0 || bytes.IndexByte(raw, 0) >= 0 {
			continue
		}
		lines := bytes.Count(raw, []byte{'\n'})
		fmt.Fprintf(stdout, "%7d  %s\n", lines, path)
		count++
		total += lines
	}
	fmt.Fprintln(stdout, "---")
	fmt.Fprintf(stdout, "%d files, %d total lines. READ EVERY ONE in full with the Read tool — no skim, no sampling. Write nothing.\n", count, total)
	return 0
}

func skippedLoadDir(name string) bool {
	switch name {
	case ".git", "node_modules", ".venv", "__pycache__":
		return true
	default:
		return false
	}
}

func runChatLS(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	all := false
	for _, arg := range args {
		switch arg {
		case "--all", "-a", "all":
			all = true
		default:
			fmt.Fprintln(stderr, "usage: pfm chat ls [--all]")
			return 2
		}
	}
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat ls: %v\n", err)
		return 1
	}
	defer database.Close()
	request := scanRequest{View: compose.AllView, ReadOnly: true}
	if len(runtimes) != 0 {
		request.Runtime = &runtimes[0]
	}
	scan, err := scanFleet(context.Background(), database, request, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat ls: %v\n", err)
		return 1
	}
	root, err := repositoryRoot()
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat ls: %v\n", err)
		return 1
	}
	if all {
		fmt.Fprintln(stdout, "live chats everywhere (session · state · dir · last activity):")
	} else {
		fmt.Fprintln(stdout, "live chats in this repo (session · state · last activity):")
	}
	found, elsewhere, hidden := 0, 0, 0
	for _, row := range scan.Output.Rows {
		if !isLiveKind(row.Kind) {
			continue
		}
		if row.Hidden || row.NameHidden {
			hidden++
			continue
		}
		if !all && !withinDirectory(row.CWD, root) {
			elsewhere++
			continue
		}
		chat := chatFromRow(row)
		status, inspectErr := headless.Inspect(context.Background(), chat, time.Now())
		state := "unknown"
		if inspectErr != nil {
			fmt.Fprintf(stderr, "pfm chat ls: inspect %s: %v\n", chat.Name, inspectErr)
		} else {
			state = status.State
		}
		handle := row.Socket
		if row.SessionName != "" {
			handle = row.SessionName
		}
		location := ""
		if all {
			location = strings.Replace(row.CWD, scan.Paths.Home, "~", 1) + "  "
		}
		fmt.Fprintf(stdout, "  %-24s %-7s %s%s\n", handle, state, location, transcript.Truncate(row.LastPrompt, 64))
		found++
	}
	if found == 0 {
		fmt.Fprintln(stdout, "  (none)")
	}
	if !all && elsewhere > 0 {
		fmt.Fprintf(stdout, "  (+%d live in other dirs — pfm chat ls --all to see them)\n", elsewhere)
	}
	if hidden > 0 {
		fmt.Fprintf(stdout, "  (+%d hidden — pfm ls --hidden to manage)\n", hidden)
	}
	return 0
}

func repositoryRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	for current := directory; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return directory, nil
		}
	}
}

func withinDirectory(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func runChatBranch(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	id := os.Getenv("CLAUDE_CODE_SESSION_ID")
	if id == "" {
		fmt.Fprintln(stderr, "pfm chat branch: CLAUDE_CODE_SESSION_ID is not set")
		return 1
	}
	runtime, err := optionalCommandRuntime(runtimes)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat branch: load config: %v\n", err)
		return 1
	}
	if _, err := exec.LookPath(runtime.Config.Claude.Binary); err != nil {
		fmt.Fprintf(stderr, "pfm chat branch: configured Claude binary %q is not executable: %v\n", runtime.Config.Claude.Binary, err)
		return 1
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		fmt.Fprintln(stderr, "pfm chat branch: tmux is not on PATH")
		return 1
	}
	resolved := runtime.Paths
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat branch: current directory: %v\n", err)
		return 1
	}
	name := sanitizeBranchName(strings.Join(args, " "))
	if strings.TrimSpace(name) == "" {
		name = defaultBranchName(id)
	}
	model := currentClaudeModel(id, runtime)
	command := branchClaudeCommand(id, name, model, resolved.Home, runtime.Config)
	socket := freshSocket(compose.ResumeClaude)
	tmux := spawn.CommandTmux{TmuxDir: resolved.TmuxDir}
	if err := tmux.NewSession(context.Background(), spawn.SessionSpec{
		Socket: socket, Session: socket, Window: spawn.WindowName(name),
		CWD: cwd, Run: command,
		Width: action.HeadlessWidth, Height: action.HeadlessHeight,
	}); err != nil {
		rollbackErr := killChatServer(context.Background(), resolved, socket)
		if rollbackErr != nil {
			fmt.Fprintf(stderr, "pfm chat branch: create detached seat: %v; rollback: %v\n", err, rollbackErr)
		} else {
			fmt.Fprintf(stderr, "pfm chat branch: create detached seat: %v\n", err)
		}
		return 1
	}
	state := shared.Open(context.Background(), resolved)
	recordErr := state.RecordBranchSeat(context.Background(), socket, id, time.Now().Unix())
	closeErr := state.Close()
	if recordErr != nil || closeErr != nil {
		rollbackErr := killChatServer(context.Background(), resolved, socket)
		failure := errors.Join(recordErr, closeErr)
		if rollbackErr != nil {
			failure = errors.Join(failure, fmt.Errorf("rollback detached seat: %w", rollbackErr))
		}
		fmt.Fprintf(stderr, "pfm chat branch: record detached seat: %v\n", failure)
		return 1
	}
	fmt.Fprintf(stdout, "Branched %s…", transcript.Truncate(id, 8))
	fmt.Fprintf(stdout, " as %q", name)
	if model != "" {
		fmt.Fprintf(stdout, " on %s", model)
	}
	fmt.Fprintf(
		stdout,
		" into detached socket %s — waiting in pfm ls; open later with pfm chat open %q.\n",
		socket,
		name,
	)
	return 0
}

func sanitizeBranchName(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(" ._-", r) {
			return r
		}
		return -1
	}, value)
}

func defaultBranchName(id string) string {
	parent := ""
	database, err := store.Open(store.WithWarningWriter(io.Discard))
	if err == nil {
		if indexed, found, queryErr := database.Transcript(context.Background(), id); queryErr == nil && found {
			parent = naming.DisplayName(indexed.CustomTitle, indexed.AITitle, indexed.FirstPrompt)
		}
		_ = database.Close()
	}
	if strings.TrimSpace(parent) == "" {
		parent = transcript.Truncate(id, 8)
	}
	name := strings.TrimSpace(sanitizeBranchName(parent + "-branch"))
	if name == "" {
		return "branch"
	}
	return name
}

func currentClaudeModel(id string, runtimes ...commandRuntime) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	path := currentClaudeTranscriptPath(id, cwd, runtimes...)
	if path == "" {
		return ""
	}
	meta, err := transcript.ReadMeta(path, store.ClaudeEngine)
	if err != nil {
		return ""
	}
	switch {
	case strings.Contains(meta.Model, "opus"):
		return "opus[1m]"
	case strings.Contains(meta.Model, "sonnet"):
		return "sonnet[1m]"
	case strings.Contains(meta.Model, "haiku"):
		return "haiku"
	case strings.Contains(meta.Model, "fable"):
		return "fable"
	default:
		return ""
	}
}

// currentClaudeTranscriptPath follows the current process's explicit config
// first. When that variable is intentionally absent for an implicit account,
// the already-loaded runtime owns the configured account roots and can locate
// the session without falling back to ~/.claude.
func currentClaudeTranscriptPath(id, cwd string, runtimes ...commandRuntime) string {
	slug := strings.NewReplacer("/", "-", ".", "-").Replace(cwd)
	if config := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); config != "" {
		return filepath.Join(config, "projects", slug, id+".jsonl")
	}
	if len(runtimes) != 0 {
		roots := runtimes[0].Paths.ClaudeRoots
		for _, root := range roots {
			candidate := filepath.Join(root, slug, id+".jsonl")
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
		if len(roots) != 0 {
			return filepath.Join(roots[0], slug, id+".jsonl")
		}
		if runtimes[0].Paths.Home != "" {
			return filepath.Join(runtimes[0].Paths.Home, ".claude", "projects", slug, id+".jsonl")
		}
	}
	resolved, err := paths.Resolve()
	if err != nil {
		return ""
	}
	return filepath.Join(resolved.Home, ".claude", "projects", slug, id+".jsonl")
}

func branchClaudeCommand(id, name, model, home string, machines ...pfmconfig.Config) string {
	machine := pfmconfig.Config{
		Claude: pfmconfig.Claude{PermissionMode: pfmconfig.PermissionBypass, Binary: "claude"},
	}
	if len(machines) != 0 {
		machine = machines[0]
	}
	unsets := []string{"-u", "CLAUDE_CODE_SESSION_ID", "-u", "CLAUDECODE"}
	var sets []string
	if config := os.Getenv("CLAUDE_CONFIG_DIR"); config == "" {
		unsets = append(unsets, "-u", "CLAUDE_CONFIG_DIR")
	} else {
		sets = append(sets, "CLAUDE_CONFIG_DIR="+config)
	}
	if os.Getenv("FORCE_PROMPT_CACHING_5M") == "1" {
		unsets = append(unsets, "-u", "ENABLE_PROMPT_CACHING_1H")
		sets = append(sets, "FORCE_PROMPT_CACHING_5M=1")
	} else if os.Getenv("ENABLE_PROMPT_CACHING_1H") == "1" {
		unsets = append(unsets, "-u", "FORCE_PROMPT_CACHING_5M")
		sets = append(sets, "ENABLE_PROMPT_CACHING_1H=1")
	}
	parts := append([]string{"env"}, unsets...)
	parts = append(parts, sets...)
	parts = append(parts, machine.Claude.Binary, "--resume", id, "--fork-session")
	if model != "" {
		parts = append(parts, "--model", model)
	}
	if name != "" {
		parts = append(parts, "--name", name)
	}
	if machine.Claude.PermissionMode != pfmconfig.PermissionPrompt && policy.Autonomy(home) {
		parts = append(parts, "--allow-dangerously-skip-permissions", "--dangerously-skip-permissions")
	}
	quoted := make([]string, len(parts))
	for index, part := range parts {
		quoted[index] = action.Quote(part)
	}
	return strings.Join(quoted, " ")
}

func runChatModal(args []string, stdout, stderr io.Writer) int {
	if len(args) != 3 || args[1] != "deny" {
		fmt.Fprintln(stderr, "usage: pfm chat modal <tmux-session> deny <down-count>")
		return 2
	}
	count, err := strconv.Atoi(args[2])
	if err != nil || count < 0 {
		fmt.Fprintln(stderr, "usage: pfm chat modal <tmux-session> deny <down-count>")
		return 2
	}
	socketPath, err := chatSocketPath(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat modal: %v\n", err)
		return 1
	}
	if _, err := os.Stat(socketPath); err != nil {
		fmt.Fprintf(stderr, "pfm chat modal: no tmux socket for %q: %v\n", args[0], err)
		return 1
	}
	for index := 0; index < count; index++ {
		if output, err := exec.Command("tmux", "-S", socketPath, "send-keys", "Down").CombinedOutput(); err != nil {
			fmt.Fprintf(stderr, "pfm chat modal: send Down: %v: %s\n", err, strings.TrimSpace(string(output)))
			return 1
		}
		time.Sleep(200 * time.Millisecond)
	}
	if output, err := exec.Command("tmux", "-S", socketPath, "send-keys", "Enter").CombinedOutput(); err != nil {
		fmt.Fprintf(stderr, "pfm chat modal: send Enter: %v: %s\n", err, strings.TrimSpace(string(output)))
		return 1
	}
	fmt.Fprintf(stdout, "modal denied on %s\n", args[0])
	return 0
}
