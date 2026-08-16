package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"hostops/pfm/internal/paths"
)

type resumeTarget struct {
	ID   string
	Path string
}

// resolveResumeTarget is the dormant half of the documented inject ladder.
// It runs only after every live namespace missed, so a live tmux session name
// always wins over a transcript id, path, or excerpt that happens to resemble
// it.
func resolveResumeTarget(target string) (resumeTarget, bool, error) {
	if info, err := os.Stat(target); err == nil {
		if !info.Mode().IsRegular() {
			return resumeTarget{}, false, fmt.Errorf("transcript target is not a regular file: %s", target)
		}
		path, err := filepath.Abs(target)
		if err != nil {
			return resumeTarget{}, false, fmt.Errorf("resolve transcript path %q: %w", target, err)
		}
		return resumeTarget{ID: transcriptIDFromPath(path), Path: path}, true, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return resumeTarget{}, false, fmt.Errorf("stat transcript target %q: %w", target, err)
	}

	resolved, err := paths.Resolve()
	if err != nil {
		return resumeTarget{}, false, err
	}
	files, err := claudeTranscriptFiles(resolved)
	if err != nil {
		return resumeTarget{}, false, err
	}
	if sessionToken(target) {
		exact := make([]resumeTarget, 0, 1)
		prefix := make([]resumeTarget, 0, 2)
		for _, path := range files {
			id := transcriptIDFromPath(path)
			switch {
			case id == target:
				exact = append(exact, resumeTarget{ID: id, Path: path})
			case len(target) >= 8 && strings.HasPrefix(id, target):
				prefix = append(prefix, resumeTarget{ID: id, Path: path})
			}
		}
		matches := exact
		if len(matches) == 0 {
			matches = prefix
		}
		switch len(matches) {
		case 1:
			return matches[0], true, nil
		case 0:
		default:
			return resumeTarget{}, false, fmt.Errorf("session id %q is ambiguous across %d transcripts", target, len(matches))
		}
	}

	if len(excerptNeedles(target)) == 0 {
		return resumeTarget{}, false, nil
	}
	match, alternatives, err := findTranscriptContent([]byte(target))
	if err != nil {
		if strings.Contains(err.Error(), "no session contains the excerpt") ||
			strings.Contains(err.Error(), "no transcript registry is available") {
			return resumeTarget{}, false, nil
		}
		return resumeTarget{}, false, err
	}
	if len(alternatives) > 0 && alternatives[0].Hits == match.Hits {
		return resumeTarget{}, false, fmt.Errorf(
			"excerpt is ambiguous between %s and %s",
			match.ID,
			alternatives[0].ID,
		)
	}
	return resumeTarget{ID: match.ID, Path: match.Path}, true, nil
}

func sessionToken(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') &&
			character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func transcriptIDFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func runningClaudeSession(procRoot, id string) (bool, error) {
	entries, err := os.ReadDir(procRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read process jail %q: %w", procRoot, err)
	}
	needle := []byte("CLAUDE_CODE_SESSION_ID=" + id + "\x00")
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil || !entry.IsDir() {
			continue
		}
		path := filepath.Join(procRoot, entry.Name(), "environ")
		raw, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("read process environment %q: %w", path, err)
		}
		if bytes.Contains(raw, needle) {
			return true, nil
		}
	}
	return false, nil
}

type resumeReceipt struct {
	SessionID string
	EventID   string
	ParentID  string
	Backup    string
}

func appendResumeInjection(
	ctx context.Context,
	resolved paths.Values,
	target resumeTarget,
	message string,
) (resumeReceipt, error) {
	if err := ctx.Err(); err != nil {
		return resumeReceipt{}, err
	}
	file, err := os.OpenFile(target.Path, os.O_RDWR, 0)
	if err != nil {
		return resumeReceipt{}, fmt.Errorf("open transcript %q: %w", target.Path, err)
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return resumeReceipt{}, fmt.Errorf("lock transcript %q: %w", target.Path, err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return resumeReceipt{}, fmt.Errorf("rewind transcript %q: %w", target.Path, err)
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		return resumeReceipt{}, fmt.Errorf("read transcript %q: %w", target.Path, err)
	}
	tail, err := lastUUIDEvent(raw)
	if err != nil {
		return resumeReceipt{}, fmt.Errorf("read transcript tail %q: %w", target.Path, err)
	}
	parent, _ := tail["uuid"].(string)
	sessionID, _ := tail["sessionId"].(string)
	if parent == "" || sessionID == "" {
		return resumeReceipt{}, fmt.Errorf("transcript %q has no uuid-bearing session tail", target.Path)
	}
	eventID, err := randomUUID()
	if err != nil {
		return resumeReceipt{}, fmt.Errorf("create injected event id: %w", err)
	}
	promptID, err := randomUUID()
	if err != nil {
		return resumeReceipt{}, fmt.Errorf("create injected prompt id: %w", err)
	}
	event := map[string]any{
		"type":        "user",
		"userType":    "external",
		"entrypoint":  "cli",
		"sessionId":   sessionID,
		"parentUuid":  parent,
		"uuid":        eventID,
		"promptId":    promptID,
		"timestamp":   time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"isSidechain": false,
		"isMeta":      false,
		"message": map[string]any{
			"role":    "user",
			"content": message,
		},
	}
	for _, key := range []string{"cwd", "version", "gitBranch"} {
		if value, exists := tail[key]; exists {
			event[key] = value
		}
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return resumeReceipt{}, fmt.Errorf("encode injected transcript event: %w", err)
	}
	backupDir := filepath.Join(resolved.Home, ".claude-sessions", ".chat-inject-backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return resumeReceipt{}, fmt.Errorf("create transcript backup directory %q: %w", backupDir, err)
	}
	backup := filepath.Join(
		backupDir,
		fmt.Sprintf("%s-%d-%s.jsonl", sessionID, time.Now().UTC().UnixNano(), eventID[:8]),
	)
	backupFile, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return resumeReceipt{}, fmt.Errorf("create transcript backup %q: %w", backup, err)
	}
	if _, err := backupFile.Write(raw); err != nil {
		_ = backupFile.Close()
		return resumeReceipt{}, fmt.Errorf("write transcript backup %q: %w", backup, err)
	}
	if err := backupFile.Sync(); err != nil {
		_ = backupFile.Close()
		return resumeReceipt{}, fmt.Errorf("sync transcript backup %q: %w", backup, err)
	}
	if err := backupFile.Close(); err != nil {
		return resumeReceipt{}, fmt.Errorf("close transcript backup %q: %w", backup, err)
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return resumeReceipt{}, fmt.Errorf("seek transcript %q for append: %w", target.Path, err)
	}
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		if _, err := file.Write([]byte{'\n'}); err != nil {
			return resumeReceipt{}, fmt.Errorf("separate transcript append %q: %w", target.Path, err)
		}
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return resumeReceipt{}, fmt.Errorf("append transcript %q: %w", target.Path, err)
	}
	if err := file.Sync(); err != nil {
		return resumeReceipt{}, fmt.Errorf("sync transcript %q: %w", target.Path, err)
	}
	return resumeReceipt{SessionID: sessionID, EventID: eventID, ParentID: parent, Backup: backup}, nil
}

func lastUUIDEvent(raw []byte) (map[string]any, error) {
	var tail map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64<<10), 32<<20)
	for scanner.Scan() {
		var event map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode JSONL line: %w", err)
		}
		if uuid, _ := event["uuid"].(string); uuid != "" {
			tail = event
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if tail == nil {
		return nil, errors.New("no uuid-bearing event")
	}
	return tail, nil
}

func randomUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := make([]byte, 32)
	hex.Encode(encoded, raw[:])
	return string(encoded[0:8]) + "-" + string(encoded[8:12]) + "-" +
		string(encoded[12:16]) + "-" + string(encoded[16:20]) + "-" +
		string(encoded[20:32]), nil
}
