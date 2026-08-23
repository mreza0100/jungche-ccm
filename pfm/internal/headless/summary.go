package headless

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"hostops/pfm/internal/ask"
	pfmconfig "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/store"
	"hostops/pfm/internal/transcript"
)

const summaryPrompt = "Summarize this exchange in ≤ 40 words: what was asked, what was delivered or is still in flight."

type SummaryOptions struct {
	Config   pfmconfig.Config
	Database *store.Store
	TempDir  string
	Engine   pfmengine.ID
	Model    string
}

type SummaryResult struct {
	Text   string
	Cached bool
}

// Summarize isolates one exchange, consults its exact-offset cache, and pays
// an ask runner only on a miss. Every failure becomes visible summary text so
// an unavailable or crashed engine can never look like an empty exchange.
func Summarize(ctx context.Context, chat Chat, options SummaryOptions) SummaryResult {
	if options.Database == nil {
		return failedSummary(fmt.Errorf("summary cache is not configured"))
	}
	entries, offset, err := transcript.From(ctx, chat.Path, string(chat.Engine), 0)
	if err != nil {
		return failedSummary(fmt.Errorf("read exchange: %w", err))
	}
	prompt, response, ok := transcript.LastExchange(entries)
	if !ok {
		return SummaryResult{Text: "unavailable (no human exchange)"}
	}
	complete := len(entries) != 0 && entries[len(entries)-1].Role == transcript.RoleAssistant
	if complete {
		cached, found, cacheErr := options.Database.ChatSummary(ctx, chat.Path, offset)
		if cacheErr != nil {
			return failedSummary(cacheErr)
		}
		if found {
			return SummaryResult{Text: cached, Cached: true}
		}
	}

	engineName := options.Engine
	if engineName == "" {
		engineName, err = options.Config.DefaultEngine()
		if err != nil {
			return failedSummary(err)
		}
	}
	runner, err := ask.ResolveEngine(engineName, options.Config)
	if err != nil {
		var missing *ask.BinaryMissingError
		if errors.As(err, &missing) {
			return SummaryResult{Text: fmt.Sprintf("unavailable (%s binary MISSING)", missing.Engine)}
		}
		return failedSummary(err)
	}

	prepared, err := writePreparedExchange(options.TempDir, prompt, response, complete)
	if err != nil {
		return failedSummary(err)
	}
	input, resolveErr := ask.ResolveInput(ask.AskInput{
		ContentFiles: []string{prepared},
		SourceLabels: []string{chat.Name + " last exchange"},
		Prompt:       summaryPrompt,
		Engine:       engineName,
		Model:        options.Model,
	}, options.Config)
	if !complete {
		input.Prompt += " The response is PARTIAL because the seat is still working."
	}
	if resolveErr != nil {
		removeErr := os.Remove(prepared)
		return failedSummary(errors.Join(resolveErr, removeErr))
	}
	answer, runErr := runner.Run(ctx, input)
	removeErr := os.Remove(prepared)
	if runErr != nil || removeErr != nil {
		return failedSummary(errors.Join(runErr, removeErr))
	}
	text := strings.Join(strings.Fields(answer.Answer), " ")
	if !complete {
		text = "PARTIAL: " + text
	}
	text = limitSummaryWords(text, 40)
	if complete {
		if err := options.Database.PutChatSummary(ctx, chat.Path, offset, text); err != nil {
			return failedSummary(err)
		}
	}
	return SummaryResult{Text: text}
}

func writePreparedExchange(directory string, prompt, response []transcript.Entry, complete bool) (string, error) {
	if directory == "" {
		directory = filepath.Join("tmp", "chat-status")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create prepared exchange directory: %w", err)
	}
	file, err := os.CreateTemp(directory, "exchange-*.md")
	if err != nil {
		return "", fmt.Errorf("create prepared exchange: %w", err)
	}
	path := file.Name()
	var content strings.Builder
	content.WriteString("PROMPT\n")
	for _, entry := range prompt {
		content.WriteString(transcript.Condensed(entry))
		content.WriteByte('\n')
	}
	content.WriteString("RESPONSE\n")
	for _, entry := range response {
		content.WriteString(transcript.Condensed(entry))
		content.WriteByte('\n')
	}
	if complete {
		content.WriteString("STATE: COMPLETE\n")
	} else {
		content.WriteString("STATE: PARTIAL\n")
	}
	_, writeErr := file.WriteString(content.String())
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		removeErr := os.Remove(path)
		return "", fmt.Errorf("write prepared exchange: %w", errors.Join(writeErr, closeErr, removeErr))
	}
	return path, nil
}

func failedSummary(err error) SummaryResult {
	detail := "unknown error"
	if err != nil {
		detail = strings.Join(strings.Fields(err.Error()), " ")
		detail = transcript.Truncate(detail, transcript.TextCap)
	}
	return SummaryResult{Text: "failed (" + detail + ")"}
}

func limitSummaryWords(value string, limit int) string {
	words := strings.Fields(value)
	if len(words) > limit {
		words = words[:limit]
	}
	return strings.Join(words, " ")
}
