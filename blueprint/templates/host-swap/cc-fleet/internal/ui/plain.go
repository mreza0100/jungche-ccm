package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (picker PlainPicker) Pick(
	_ context.Context,
	snapshot Snapshot,
) (Outcome, error) {
	if picker.Writer == nil {
		return Outcome{}, errors.New("plain picker requires a writer")
	}
	_, err := io.WriteString(picker.Writer, RenderPlain(snapshot))
	return passiveOutcome(snapshot), err
}

func (picker TSVPicker) Pick(
	_ context.Context,
	snapshot Snapshot,
) (Outcome, error) {
	if picker.Writer == nil {
		return Outcome{}, errors.New("TSV picker requires a writer")
	}
	_, err := io.WriteString(picker.Writer, RenderTSV(snapshot))
	return passiveOutcome(snapshot), err
}

func passiveOutcome(snapshot Snapshot) Outcome {
	return Outcome{
		Kind:           OutcomeNone,
		PrimaryAccount: validAccount(snapshot.PrimaryAccount),
		Cache1H:        snapshot.Cache1H,
		Rotation:       snapshot.Rotation,
		Query:          snapshot.InitialQuery,
	}
}

// RenderPlain is the non-ANSI human-readable twin of the fancy list.
func RenderPlain(snapshot Snapshot) string {
	model := NewModel(snapshot)
	var output strings.Builder
	previous := ""
	for _, row := range model.VisibleRows() {
		project := cleanField(row.Project)
		if project == "" {
			project = "?"
		}
		if project != previous {
			if output.Len() != 0 {
				output.WriteByte('\n')
			}
			fmt.Fprintf(&output, "[%s]\n", project)
			previous = project
		}
		parts := []string{
			rowMarker(row.Kind) + " " +
				clipRunes(cleanField(row.Name), 30),
		}
		if badges := stripANSI(rowBadges(row)); badges != "" {
			parts = append(parts, badges)
		}
		parts = append(
			parts,
			fmt.Sprintf("%dp", row.PromptCount),
			formatSize(row.Size),
			formatAge(row, snapshot.NowNS),
		)
		fmt.Fprintln(&output, strings.Join(parts, "  "))
	}
	return output.String()
}

// RenderTSV is stable and lossless for identifiers while sanitizing embedded
// tabs/newlines in display fields.
func RenderTSV(snapshot Snapshot) string {
	model := NewModel(snapshot)
	var output strings.Builder
	output.WriteString(
		"kind\tid\tproject\tcwd\tname\tprompts\tsize\tactivity_ns\taccount\thidden\tsocket\n",
	)
	for _, row := range model.VisibleRows() {
		fields := []string{
			row.Kind.String(),
			row.ID,
			cleanField(row.Project),
			cleanField(row.CWD),
			cleanField(row.Name),
			strconv.FormatInt(row.PromptCount, 10),
			strconv.FormatInt(row.Size, 10),
			strconv.FormatInt(row.ActivityNS, 10),
			strconv.Itoa(row.Account),
			strconv.FormatBool(row.Hidden),
			row.Socket,
		}
		for index := range fields {
			fields[index] = tsvField(fields[index])
		}
		output.WriteString(strings.Join(fields, "\t"))
		output.WriteByte('\n')
	}
	return output.String()
}

func tsvField(value string) string {
	return strings.Map(func(runeValue rune) rune {
		switch runeValue {
		case '\t', '\n', '\r', 0:
			return ' '
		default:
			return runeValue
		}
	}, value)
}

func stripANSI(value string) string {
	return ansi.Strip(value)
}

var (
	_ Picker = PlainPicker{}
	_ Picker = TSVPicker{}
)
