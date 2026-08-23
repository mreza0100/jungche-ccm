package inject

import (
	"context"
	"errors"
	"fmt"
	pfmengine "hostops/pfm/internal/engine"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	defaultBodyMaxAge = 7 * 24 * time.Hour
	captionMaxRunes   = 120
)

// PreparedMessage is the exact text sent to a pane or appended to a dormant
// transcript. AutoFilePath is non-empty when the original body was persisted
// and Message is the short pointer replacing it.
type PreparedMessage struct {
	Message      string
	AutoFilePath string
	Unsigned     bool
	Warnings     []string
}

// PrepareForResume applies the same measured auto-file boundary as live
// delivery. Dormant transcript injection is still transport: an 8 KiB body
// belongs in a durable file, not as a giant synthetic user turn.
func (engine *Engine) PrepareForResume(
	ctx context.Context,
	target, engineName, body string,
) (PreparedMessage, error) {
	signed, unsigned := engine.SignForResume(ctx, body)
	return engine.prepareMessage(ctx, target, engineName, body, signed, unsigned, false, true)
}

func (engine *Engine) prepareLiveMessage(
	ctx context.Context,
	target Target,
	body string,
	interrupted bool,
) (PreparedMessage, error) {
	signed, unsigned := engine.signedMessage(ctx, body, interrupted)
	if isHarnessCommand(body) {
		return PreparedMessage{Message: signed, Unsigned: unsigned}, nil
	}
	name := filepath.Base(target.SocketPath)
	if target.Pane != "" {
		name += "-" + target.Pane
	}
	return engine.prepareMessage(
		ctx,
		name,
		target.Engine,
		body,
		signed,
		unsigned,
		interrupted,
		false,
	)
}

func (engine *Engine) prepareMessage(
	ctx context.Context,
	target, engineName, body, signed string,
	unsigned, interrupted, resume bool,
) (PreparedMessage, error) {
	if utf8.RuneCountInString(signed) <= engine.autoFileThreshold(engineName) {
		return PreparedMessage{Message: signed, Unsigned: unsigned}, nil
	}
	path, warnings, err := engine.persistBody(body, target)
	if err != nil {
		return PreparedMessage{}, err
	}
	pointer := bodyCaption(body) + " — read " + path + " fully"
	pointerUnsigned := false
	if resume {
		pointer, pointerUnsigned = engine.SignForResume(ctx, pointer)
	} else {
		pointer, pointerUnsigned = engine.signedMessage(ctx, pointer, interrupted)
	}
	return PreparedMessage{
		Message:      pointer,
		AutoFilePath: path,
		Unsigned:     pointerUnsigned,
		Warnings:     warnings,
	}, nil
}

func (engine *Engine) autoFileThreshold(engineName string) int {
	// OpenCode inherits Codex's conservative bound: its composer paste edge is
	// unverified, so it gets the smaller of the two measured thresholds rather
	// than an invented one.
	if engineName == "cx" || engineName == string(pfmengine.Opencode) {
		return engine.options.CodexAutoFileMax
	}
	return engine.options.ClaudeAutoFileMax
}

func (engine *Engine) persistBody(body, target string) (string, []string, error) {
	root := engine.options.BodyRoot
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", nil, fmt.Errorf("create inject body directory %q: %w", root, err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return "", nil, fmt.Errorf("secure inject body directory %q: %w", root, err)
	}
	warnings, err := engine.pruneBodies(root)
	if err != nil {
		return "", nil, err
	}
	stamp := engine.options.Now().UTC().Format("20060102T150405.000000000Z")
	stem := stamp + "-" + safeBodyTarget(target)
	for sequence := 0; sequence < 1000; sequence++ {
		name := stem + ".md"
		if sequence > 0 {
			name = fmt.Sprintf("%s-%03d.md", stem, sequence)
		}
		path := filepath.Join(root, name)
		err := writeExclusive(path, body)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", warnings, fmt.Errorf("write inject body %q: %w", path, err)
		}
		return path, warnings, nil
	}
	return "", warnings, fmt.Errorf("allocate inject body name under %q: 1000 collisions", root)
}

func (engine *Engine) pruneBodies(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("enumerate inject bodies in %q: %w", root, err)
	}
	cutoff := engine.options.Now().Add(-engine.options.BodyMaxAge)
	warnings := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("stat stale body candidate %q: %v", entry.Name(), err))
			continue
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if err := os.Remove(path); err != nil {
			warnings = append(warnings, fmt.Sprintf("prune stale body %q: %v", path, err))
		}
	}
	return warnings, nil
}

func writeExclusive(path, body string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(file, body)
	closeErr := file.Close()
	if writeErr == nil && closeErr == nil {
		return nil
	}
	cleanupErr := os.Remove(path)
	return errors.Join(writeErr, closeErr, cleanupErr)
}

func bodyCaption(body string) string {
	line := body
	if newline := strings.IndexByte(line, '\n'); newline >= 0 {
		line = line[:newline]
	}
	line = strings.Join(strings.Fields(line), " ")
	if line == "" {
		line = "Long inject body"
	}
	runes := []rune(line)
	if len(runes) > captionMaxRunes {
		line = string(runes[:captionMaxRunes-1]) + "…"
	}
	return line
}

func safeBodyTarget(target string) string {
	value := strings.Map(func(character rune) rune {
		switch {
		case unicode.IsLetter(character), unicode.IsDigit(character),
			character == '-', character == '_', character == '.':
			return character
		default:
			return '-'
		}
	}, target)
	value = strings.Trim(value, "-.")
	if value == "" {
		return "chat"
	}
	return headRunes(value, 96)
}
