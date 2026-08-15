package action

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGoldenCommandLines(t *testing.T) {
	var actual bytes.Buffer
	lastRoute := Route(0)
	for _, request := range stressRequests() {
		plan, err := Synthesize(request)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Route != lastRoute {
			fmt.Fprintln(&actual, goldenSourceComment(plan.Route))
			lastRoute = plan.Route
		}
		line := plan.Line
		if plan.Run != "" {
			line = replaceGoldenRun(line, plan.Run)
		}
		fmt.Fprintf(
			&actual,
			"%c\tb=%t\ta=%d\th=%t\trun=%s\tline=%s\n",
			plan.Route,
			request.Bunker,
			request.PrimaryAccount,
			request.Cache1H,
			plan.Run,
			line,
		)
	}
	path := filepath.Join("..", "..", "testdata", "golden", "cmdlines.txt")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read command golden: %v\nactual:\n%s", err, actual.String())
	}
	if !bytes.Equal(want, actual.Bytes()) {
		t.Fatalf(
			"command golden mismatch (%s)\nwant:\n%s\nactual:\n%s",
			firstGoldenDifference(want, actual.Bytes()),
			want,
			actual.String(),
		)
	}
}

func replaceGoldenRun(line, run string) string {
	quotedRun := Quote(run)
	index := bytes.Index([]byte(line), []byte(quotedRun))
	if index < 0 {
		return line
	}
	return line[:index] + "'<RUN>'" + line[index+len(quotedRun):]
}

func goldenSourceComment(route Route) string {
	switch route {
	case NewClaude:
		return "# N — cc-fleet.zsh:67-124 (CC_AUTONOMY_FLAGS, CC_ENDPOINT_UNSET, _cc_run), 893-894 (PROJDIR), 1430-1433 (dispatch)"
	case NewCodex:
		return "# C — cc-fleet.zsh:145-157 (cx), 893-894 (PROJDIR), 1434-1438 (dispatch)"
	case Live:
		return "# L — cc-fleet.zsh:326-347 (_cc_selfswitch), 753-801 (_cc_solo), 1448-1456 (dispatch)"
	case Agent:
		return "# A — cc-fleet.zsh:1380-1384 (cache env), 1457-1467 (agent router); WP7 failure-net extension"
	case ResumeClaude:
		return "# R — cc-fleet.zsh:86-124 (_cc_run hygiene), 1468-1487 (resume + agent failure net)"
	case ResumeCodex:
		return "# X — cc-fleet.zsh:163-168 (_cx_server), 1439-1447 (Codex resume)"
	default:
		panic("unknown golden route")
	}
}

func firstGoldenDifference(want, got []byte) string {
	limit := len(want)
	if len(got) < limit {
		limit = len(got)
	}
	for index := 0; index < limit; index++ {
		if want[index] != got[index] {
			return fmt.Sprintf(
				"byte %d: want %#x got %#x; lengths %d/%d",
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
