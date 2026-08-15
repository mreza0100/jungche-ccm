package transcript

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
)

// metaWindow is how much of the tail is read for Meta. The newest records are
// the ones that carry the live model and token counts, and a chat's transcript
// can be hundreds of megabytes — a status command must not walk all of it.
const metaWindow = 512 << 10

// Meta is what a transcript says about the run itself, as opposed to what was
// said in it. Every field is best-effort: a transcript that does not state a
// value leaves it zero, and a caller must report it as unknown rather than
// invent one.
type Meta struct {
	Model          string
	ContextTokens  int64
	ContextWindow  int64
	SizeBytes      int64
	ModifiedUnixNS int64
}

// ContextPercent is the share of the context window in use, or 0 when the
// transcript never stated a window.
func (meta Meta) ContextPercent() float64 {
	if meta.ContextWindow <= 0 || meta.ContextTokens <= 0 {
		return 0
	}
	return float64(meta.ContextTokens) / float64(meta.ContextWindow) * 100
}

type metaRecord struct {
	Type    string `json:"type"`
	Message struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens         int64 `json:"input_tokens"`
			OutputTokens        int64 `json:"output_tokens"`
			CacheReadTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
	Payload struct {
		Type  string `json:"type"`
		Model string `json:"model"`
		Info  struct {
			TotalTokenUsage struct {
				TotalTokens int64 `json:"total_tokens"`
			} `json:"total_token_usage"`
			ModelContextWindow int64 `json:"model_context_window"`
		} `json:"info"`
	} `json:"payload"`
}

// ReadMeta scans the tail of a transcript for the run's own facts.
func ReadMeta(path, engine string) (Meta, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Meta{}, err
	}
	meta := Meta{
		SizeBytes:      info.Size(),
		ModifiedUnixNS: info.ModTime().UnixNano(),
	}
	file, err := os.Open(path)
	if err != nil {
		return meta, err
	}
	defer file.Close()

	offset := int64(0)
	if info.Size() > metaWindow {
		offset = info.Size() - metaWindow
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return meta, err
	}
	reader := bufio.NewReaderSize(file, 64<<10)
	if offset > 0 {
		// The first line of a mid-file window is a fragment.
		if _, err := reader.ReadBytes('\n'); err != nil {
			return meta, nil
		}
	}
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			applyMeta(&meta, line, engine)
		}
		if readErr != nil {
			return meta, nil
		}
	}
}

func applyMeta(meta *Meta, line []byte, engine string) {
	var parsed metaRecord
	if err := json.Unmarshal(line, &parsed); err != nil {
		return
	}
	if engine == "cx" {
		if parsed.Payload.Model != "" {
			meta.Model = parsed.Payload.Model
		}
		if parsed.Payload.Type == "token_count" {
			if total := parsed.Payload.Info.TotalTokenUsage.TotalTokens; total > 0 {
				meta.ContextTokens = total
			}
			if window := parsed.Payload.Info.ModelContextWindow; window > 0 {
				meta.ContextWindow = window
			}
		}
		return
	}
	if parsed.Type != "assistant" {
		return
	}
	// "<synthetic>" is the harness's own marker on records it wrote itself
	// (interrupts, tool errors); it is not a model anybody chose.
	if parsed.Message.Model != "" && parsed.Message.Model != "<synthetic>" {
		meta.Model = parsed.Message.Model
	}
	usage := parsed.Message.Usage
	// What the next request will carry: everything already in the window plus
	// what this turn produced. Cache reads count — they are context, not a
	// discount on it.
	if used := usage.InputTokens + usage.CacheReadTokens +
		usage.CacheCreationTokens + usage.OutputTokens; used > 0 {
		meta.ContextTokens = used
	}
}
