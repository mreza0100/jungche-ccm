package ui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderGoldens(t *testing.T) {
	tests := []struct {
		name string
		path string
		got  func() string
	}{
		{
			name: "ansi 80 columns",
			path: "ui_80.ansi",
			got: func() string {
				return quoteANSI(NewModel(fixtureSnapshot(80)).View().Content)
			},
		},
		{
			name: "ansi 120 columns",
			path: "ui_120.ansi",
			got: func() string {
				return quoteANSI(NewModel(fixtureSnapshot(120)).View().Content)
			},
		},
		{
			name: "plain",
			path: "ui_plain.txt",
			got: func() string {
				return RenderPlain(fixtureSnapshot(120))
			},
		},
		{
			name: "tsv",
			path: "ui.tsv",
			got: func() string {
				return RenderTSV(fixtureSnapshot(120))
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := test.got()
			path := filepath.Join("..", "..", "testdata", "golden", test.path)
			if os.Getenv("PFM_UPDATE_GOLDENS") == "1" {
				if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
					t.Fatalf("regenerate %s: %v", path, err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v\nactual:\n%s", path, err, actual)
			}
			if !bytes.Equal(want, []byte(actual)) {
				t.Fatalf(
					"%s mismatch\nwant:\n%s\nactual:\n%s",
					firstDifference(want, []byte(actual)),
					want,
					actual,
				)
			}
		})
	}
}

func TestAgentPaletteIsOrangeAndDistinctFromCodex(t *testing.T) {
	agent := fmt.Sprint(agentStyle.GetForeground())
	codex := fmt.Sprint(codexStyle.GetForeground())
	if agent != "{251 146 60 255}" {
		t.Fatalf("agent foreground = %q, want vivid orange", agent)
	}
	if agent == codex {
		t.Fatalf("agent and Codex foregrounds are both %q", agent)
	}
}

func TestFancyRenderNeverWrapsAtFixedWidths(t *testing.T) {
	for _, width := range []int{80, 120} {
		snapshot := fixtureSnapshot(width)
		content := NewModel(snapshot).View().Content
		lines := strings.Split(content, "\n")
		if len(lines) != snapshot.Height {
			t.Fatalf("width %d rendered %d lines, want %d", width, len(lines), snapshot.Height)
		}
		for index, line := range lines {
			if got := lipgloss.Width(line); got != width {
				t.Fatalf("width %d line %d has %d cells", width, index, got)
			}
		}
	}
}

func TestFancyRenderHasNoPreviewAtAnyWidth(t *testing.T) {
	for _, width := range []int{40, 80, 120, 180} {
		content := ansi.Strip(NewModel(fixtureSnapshot(width)).View().Content)
		if strings.Contains(content, " preview ") ||
			strings.Contains(content, "last prompt") {
			t.Fatalf("width %d still renders the retired preview panel:\n%s", width, content)
		}
	}
}

func TestHeaderSeparatesHiddenEmptyAndRefreshStatus(t *testing.T) {
	snapshot := fixtureSnapshot(120)
	snapshot.HiddenCount = 12
	snapshot.SuppressedCount = 153
	snapshot.Refreshing = true
	header := ansi.Strip(NewModel(snapshot).renderHeader(120))
	for _, want := range []string{"12 hidden", "153 empty", "⟳ refreshing"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header %q does not contain %q", header, want)
		}
	}
}

func TestRowColumnsHaveIdenticalDisplayPositions(t *testing.T) {
	names := []string{
		"RR",
		"123456789012345678901234567890",
		"123456789012345678901234567890X",
		"🚀🚀🚀🚀🚀 emoji",
		"界面列对齐测试名字",
		"\x1b[31mANSI name\x1b[0m",
	}
	var positions []int
	for _, name := range names {
		row := fixtureSnapshot(120).Rows[1]
		row.Name = name
		row.PromptCount = 37
		row.Size = 1536
		line := ansi.Strip(NewModel(fixtureSnapshot(120)).renderRow(
			row,
			false,
			78,
		))
		got := []int{
			displayIndex(line, "⬢"),
			displayIndex(line, "37p"),
			displayIndex(line, "1.5K"),
		}
		if positions == nil {
			positions = got
			continue
		}
		if !slices.Equal(got, positions) {
			t.Fatalf(
				"name %q columns=%v, want %v in %q",
				name,
				got,
				positions,
				line,
			)
		}
	}
}

func TestPlainAndTSVPickersShareRenderers(t *testing.T) {
	snapshot := fixtureSnapshot(120)
	var plain bytes.Buffer
	outcome, err := (PlainPicker{Writer: &plain}).Pick(
		context.Background(),
		snapshot,
	)
	if err != nil || plain.String() != RenderPlain(snapshot) ||
		outcome.Kind != OutcomeNone {
		t.Fatalf("plain outcome=%#v err=%v", outcome, err)
	}
	var tsv bytes.Buffer
	outcome, err = (TSVPicker{Writer: &tsv}).Pick(
		context.Background(),
		snapshot,
	)
	if err != nil || tsv.String() != RenderTSV(snapshot) ||
		outcome.Kind != OutcomeNone {
		t.Fatalf("TSV outcome=%#v err=%v", outcome, err)
	}
}

func quoteANSI(value string) string {
	lines := strings.Split(value, "\n")
	var output strings.Builder
	for _, line := range lines {
		fmt.Fprintln(&output, strconv.QuoteToGraphic(line))
	}
	return output.String()
}

func displayIndex(value, marker string) int {
	index := strings.Index(value, marker)
	if index < 0 {
		return -1
	}
	return lipgloss.Width(value[:index])
}

func firstDifference(want, got []byte) string {
	limit := minInt(len(want), len(got))
	for index := 0; index < limit; index++ {
		if want[index] != got[index] {
			return fmt.Sprintf(
				"byte %d: want %#x got %#x (lengths %d/%d)",
				index,
				want[index],
				got[index],
				len(want),
				len(got),
			)
		}
	}
	return fmt.Sprintf("lengths %d/%d", len(want), len(got))
}
