package installer

import (
	"embed"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

//go:embed assets
var embeddedAssets embed.FS

type assetFile struct {
	path string
	mode fs.FileMode
}

func assetFiles() ([]assetFile, error) {
	var files []assetFile
	err := fs.WalkDir(embeddedAssets, "assets", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(name, "assets/")
		if !schedulerAsset(relative) {
			// The other platform's scheduler files are embedded (one binary
			// serves both) but never staged: an operator on macOS should not
			// find a ~/.config/systemd/user of units nothing will read, and an
			// operator on Linux should not find a launch agent.
			return nil
		}
		mode := fs.FileMode(0o644)
		if path.Ext(relative) == ".sh" {
			mode = 0o755
		}
		files = append(files, assetFile{path: relative, mode: mode})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(left, right int) bool {
		return files[left].path < files[right].path
	})
	return files, nil
}

// schedulerAsset reports whether an embedded asset belongs on this platform.
// The launch agent is not staged even on macOS: launchd refuses a symlinked
// plist, so wireLaunchAgent writes a real file into ~/Library/LaunchAgents
// instead of linking one out of the managed root.
func schedulerAsset(relative string) bool {
	switch {
	case strings.HasPrefix(relative, "systemd/"):
		return !schedulerIsLaunchd
	case strings.HasPrefix(relative, "launchd/"):
		return false
	default:
		return true
	}
}

func mcpSchedulerAsset(relative string) bool {
	return relative == "systemd/pfm-mcp.service" || relative == "launchd/com.professor.pfm.mcp.plist"
}

func renderShimAsset(content []byte, options Options) []byte {
	claude := []string{"typeset -gA PFM_CLAUDE_PROMPTED=("}
	for _, account := range sortedBoolKeys(options.ClaudePrompted) {
		value := 0
		if options.ClaudePrompted[account] {
			value = 1
		}
		claude = append(claude, "  ["+strconv.Itoa(account)+"]="+strconv.Itoa(value))
	}
	claude = append(claude, ")")
	codex := []string{"typeset -gA PFM_CODEX_YOLO=("}
	for _, account := range sortedBoolKeys(options.CodexYolo) {
		value := 0
		if options.CodexYolo[account] {
			value = 1
		}
		codex = append(codex, "  ["+strconv.Itoa(account)+"]="+strconv.Itoa(value))
	}
	codex = append(codex, ")")
	text := strings.Replace(string(content), "typeset -gA PFM_CLAUDE_PROMPTED=()", strings.Join(claude, "\n"), 1)
	text = strings.Replace(text, "typeset -gA PFM_CODEX_YOLO=()", strings.Join(codex, "\n"), 1)
	return []byte(text)
}

func sortedBoolKeys(values map[int]bool) []int {
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

func readAsset(name string) ([]byte, error) {
	return embeddedAssets.ReadFile(path.Join("assets", name))
}
