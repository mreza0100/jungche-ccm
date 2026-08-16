package stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"hostops/pfm/internal/compose"
)

type tokenCacheEntry struct {
	offset       int64
	modifiedNS   int64
	partial      []byte
	claudeTokens int64
	codexTokens  int64
	known        bool
	startNS      int64
}

type tokenMeasure struct {
	total   int64
	known   bool
	startNS int64
}

type tokenRecord struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Message   struct {
		Usage struct {
			InputTokens         int64 `json:"input_tokens"`
			OutputTokens        int64 `json:"output_tokens"`
			CacheReadTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
	Payload struct {
		Type string `json:"type"`
		Info struct {
			TotalTokenUsage struct {
				TotalTokens int64 `json:"total_tokens"`
			} `json:"total_token_usage"`
		} `json:"info"`
	} `json:"payload"`
}

func (sampler *Sampler) attachTokenUsage(rows []compose.Row, chats []Chat, now int64) []string {
	pathsBySocket := make(map[string]map[string]bool)
	for _, row := range rows {
		if row.Socket == "" || row.Path == "" || !liveKind(row.Kind) {
			continue
		}
		paths := pathsBySocket[row.Socket]
		if paths == nil {
			paths = make(map[string]bool)
			pathsBySocket[row.Socket] = paths
		}
		paths[row.Path] = true
	}

	sampler.tokenMu.Lock()
	defer sampler.tokenMu.Unlock()
	if sampler.tokenCache == nil {
		sampler.tokenCache = make(map[string]*tokenCacheEntry)
	}
	usedPaths := make(map[string]bool)
	usageBySocket := make(map[string]tokenMeasure)
	var warnings []string
	for socket, paths := range pathsBySocket {
		combined := tokenMeasure{}
		for path := range paths {
			usedPaths[path] = true
			measure, pathWarnings := sampler.readTokenUsageLocked(path)
			warnings = append(warnings, pathWarnings...)
			if measure.known {
				combined.total += measure.total
				combined.known = true
			}
			if measure.startNS > 0 && (combined.startNS == 0 || measure.startNS < combined.startNS) {
				combined.startNS = measure.startNS
			}
		}
		usageBySocket[socket] = combined
	}
	for path := range sampler.tokenCache {
		if !usedPaths[path] {
			delete(sampler.tokenCache, path)
		}
	}
	for index := range chats {
		measure := usageBySocket[chats[index].Socket]
		chats[index].TokenCount = measure.total
		chats[index].TokensKnown = measure.known
		if measure.known && measure.startNS > 0 && now > measure.startNS {
			hours := float64(now-measure.startNS) / float64(time.Hour)
			chats[index].TokensPerHour = float64(measure.total) / hours
			chats[index].TokenRateValid = true
		}
	}
	return warnings
}

func (sampler *Sampler) readTokenUsageLocked(path string) (tokenMeasure, []string) {
	info, err := os.Stat(path)
	if err != nil {
		return tokenMeasure{}, []string{fmt.Sprintf("read chat token transcript %s: %v", path, err)}
	}
	if !info.Mode().IsRegular() {
		return tokenMeasure{}, []string{fmt.Sprintf("read chat token transcript %s: not a regular file", path)}
	}
	entry := sampler.tokenCache[path]
	if entry == nil || info.Size() < entry.offset ||
		(info.Size() == entry.offset && info.ModTime().UnixNano() != entry.modifiedNS) {
		entry = &tokenCacheEntry{}
		sampler.tokenCache[path] = entry
	}
	if info.Size() == entry.offset {
		return tokenMeasure{
			total: entry.claudeTokens + entry.codexTokens,
			known: entry.known, startNS: entry.startNS,
		}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return tokenMeasure{}, []string{fmt.Sprintf("open chat token transcript %s: %v", path, err)}
	}
	if _, err := file.Seek(entry.offset, io.SeekStart); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return tokenMeasure{}, []string{fmt.Sprintf("seek chat token transcript %s: %v; close after seek failure: %v", path, err, closeErr)}
		}
		return tokenMeasure{}, []string{fmt.Sprintf("seek chat token transcript %s: %v", path, err)}
	}

	reader := bufio.NewReaderSize(file, 64<<10)
	var warnings []string
	for {
		chunk, readErr := reader.ReadBytes('\n')
		entry.offset += int64(len(chunk))
		if len(chunk) > 0 {
			line := append(entry.partial, chunk...)
			entry.partial = nil
			if line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
				if parseErr := applyTokenRecord(entry, line); parseErr != nil {
					warnings = append(warnings, fmt.Sprintf("parse chat token transcript %s: %v", path, parseErr))
				}
			} else {
				entry.partial = line
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				warnings = append(warnings, fmt.Sprintf("read chat token transcript %s: %v", path, readErr))
			}
			break
		}
	}
	postInfo, statErr := file.Stat()
	if statErr != nil {
		warnings = append(warnings, fmt.Sprintf("stat chat token transcript %s after read: %v", path, statErr))
		entry.modifiedNS = info.ModTime().UnixNano()
	} else {
		entry.modifiedNS = postInfo.ModTime().UnixNano()
	}
	if closeErr := file.Close(); closeErr != nil {
		warnings = append(warnings, fmt.Sprintf("close chat token transcript %s: %v", path, closeErr))
	}
	return tokenMeasure{
		total: entry.claudeTokens + entry.codexTokens,
		known: entry.known, startNS: entry.startNS,
	}, warnings
}

func applyTokenRecord(entry *tokenCacheEntry, line []byte) error {
	if len(strings.TrimSpace(string(line))) == 0 {
		return nil
	}
	var record tokenRecord
	if err := json.Unmarshal(line, &record); err != nil {
		return err
	}
	var timestampErr error
	if record.Timestamp != "" {
		parsed, err := time.Parse(time.RFC3339Nano, record.Timestamp)
		if err != nil {
			timestampErr = fmt.Errorf("parse timestamp %q: %w", record.Timestamp, err)
		} else {
			stamp := parsed.UnixNano()
			if entry.startNS == 0 || stamp < entry.startNS {
				entry.startNS = stamp
			}
		}
	}
	if record.Type == "assistant" {
		usage := record.Message.Usage
		if usage.InputTokens < 0 || usage.OutputTokens < 0 ||
			usage.CacheReadTokens < 0 || usage.CacheCreationTokens < 0 {
			return fmt.Errorf("assistant usage contains a negative token count")
		}
		total := usage.InputTokens + usage.OutputTokens +
			usage.CacheReadTokens + usage.CacheCreationTokens
		if total > 0 {
			entry.claudeTokens += total
			entry.known = true
		}
	}
	if record.Payload.Type == "token_count" {
		total := record.Payload.Info.TotalTokenUsage.TotalTokens
		if total < 0 {
			return fmt.Errorf("Codex lifetime usage contains a negative token count")
		}
		if total > 0 {
			entry.codexTokens = total
			entry.known = true
		}
	}
	return timestampErr
}
