package compose

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestGoldenRowScenarios(t *testing.T) {
	var rendered bytes.Buffer
	scenarios := []struct {
		name  string
		input Input
	}{
		{name: "default", input: fixtureInput(DefaultView)},
		{name: "all", input: fixtureInput(AllView)},
		{name: "hidden", input: fixtureInput(HiddenView)},
		{name: "split-and-multiserver", input: splitFixtureInput()},
		{name: "caps-overflow", input: capsFixtureInput()},
	}
	for _, scenario := range scenarios {
		renderScenario(&rendered, scenario.name, Compose(scenario.input))
	}
	rendered.Truncate(rendered.Len() - 1)

	goldenPath := filepath.Join("..", "..", "testdata", "golden", "compose.tsv")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read compose golden: %v\nactual:\n%s", err, rendered.String())
	}
	if !bytes.Equal(rendered.Bytes(), want) {
		t.Fatalf(
			"compose golden mismatch (%s)\nwant:\n%s\nactual:\n%s",
			firstDifference(want, rendered.Bytes()),
			want,
			rendered.String(),
		)
	}
}

func firstDifference(want, got []byte) string {
	limit := len(want)
	if len(got) < limit {
		limit = len(got)
	}
	for index := 0; index < limit; index++ {
		if want[index] != got[index] {
			return fmt.Sprintf(
				"byte %d: want %#x, got %#x; lengths %d/%d",
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

func renderScenario(output *bytes.Buffer, name string, result Output) {
	fmt.Fprintf(output, "SCENARIO\t%s\n", name)
	fmt.Fprintf(
		output,
		"META\thidden=%d\tempty=%d\tprojects=%s\tbaselines=%s\tunhide=%s\n",
		result.HiddenCount,
		result.SuppressedCount,
		strings.Join(result.ProjectOrder, ","),
		renderBaselines(result.BaselineUpdates),
		strings.Join(result.UnhideIDs, ","),
	)
	projects := make([]string, 0, len(result.ProjectDirs))
	for project := range result.ProjectDirs {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	for _, project := range projects {
		fmt.Fprintf(output, "DIR\t%s\t%s\n", project, result.ProjectDirs[project])
	}
	for _, row := range result.Rows {
		fmt.Fprintf(
			output,
			"ROW\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\n",
			row.Kind,
			row.ID,
			row.Name,
			row.Project,
			row.CWD,
			row.Socket,
			row.PaneID,
			row.Size,
			row.PromptCount,
			row.ActivityNS,
			row.AgeNS,
			strconv.Itoa(row.Account),
			renderInts(row.Accounts),
			renderBool(row.Hidden),
			renderBool(row.BG),
			renderBool(row.C1H),
			renderBool(row.Attached),
			boolInt(row.Here),
			row.ServerCount,
			row.SplitCount,
		)
	}
	output.WriteByte('\n')
}

func renderBaselines(updates []BaselineUpdate) string {
	values := make([]string, 0, len(updates))
	for _, update := range updates {
		values = append(values, fmt.Sprintf(
			"%s:%s:%d",
			update.ID,
			update.Engine,
			update.BaselinePrompts,
		))
	}
	return strings.Join(values, ",")
}

func renderInts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}

func renderBool(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
