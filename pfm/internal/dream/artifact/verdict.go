package artifact

import (
	"fmt"
	"regexp"
	"strings"
)

var mapPathPattern = regexp.MustCompile(`^maps/[a-z0-9][a-z0-9-]*\.md$`)

func ParseVerdicts(text string) ([]Verdict, error) {
	var verdicts []Verdict
	var problems []string
	for offset, row := range linesOf(text) {
		if row == "" {
			continue
		}
		fields := strings.Split(row, "\t")
		if len(fields) != 3 {
			problems = append(problems, fmt.Sprintf("line %d: %s", offset+1, row))
			continue
		}
		kind, ok := parseSeatVerdict(fields[0])
		if !ok || !mapPathPattern.MatchString(fields[1]) || fields[2] == "" {
			problems = append(problems, fmt.Sprintf("line %d: %s", offset+1, row))
			continue
		}
		verdicts = append(verdicts, Verdict{Kind: kind, MapPath: fields[1], Evidence: fields[2]})
	}
	if len(problems) != 0 {
		return nil, parseFailure(problems...)
	}
	return verdicts, nil
}

// NormalizeVerdicts rejects duplicate and unknown verdicts. A survivor omitted
// by the verifier is intentionally retained as UNRULED so it cannot be applied.
func NormalizeVerdicts(survivors []string, verdicts []Verdict) ([]NormalizedVerdict, error) {
	expected := uniqueSorted(survivors)
	expectedSet := make(map[string]struct{}, len(expected))
	for _, path := range expected {
		expectedSet[path] = struct{}{}
	}
	counts := make(map[string]int, len(verdicts))
	var unknown []string
	for _, verdict := range verdicts {
		counts[verdict.MapPath]++
		if _, ok := expectedSet[verdict.MapPath]; !ok {
			unknown = append(unknown, verdict.MapPath)
		}
	}
	var duplicates []string
	for path, count := range counts {
		if count > 1 {
			duplicates = append(duplicates, path)
		}
	}
	slicesSort(duplicates)
	unknown = uniqueSorted(unknown)
	var problems []string
	if len(duplicates) != 0 {
		problems = append(problems, "DUPLICATE MAPS:\n"+strings.Join(duplicates, "\n"))
	}
	if len(unknown) != 0 {
		problems = append(problems, "UNKNOWN MAPS:\n"+strings.Join(unknown, "\n"))
	}
	if len(problems) != 0 {
		return nil, parseFailure(problems...)
	}

	byMap := make(map[string]Verdict, len(verdicts))
	for _, verdict := range verdicts {
		byMap[verdict.MapPath] = verdict
	}
	normalized := make([]NormalizedVerdict, 0, len(expected))
	for _, path := range expected {
		verdict, ok := byMap[path]
		if !ok {
			normalized = append(normalized, NormalizedVerdict{
				Kind: NormalizedUnruled, MapPath: path, Evidence: "no verifier verdict; not applied",
			})
			continue
		}
		normalized = append(normalized, NormalizedVerdict{
			Kind: NormalizedVerdictKind(verdict.Kind), MapPath: verdict.MapPath, Evidence: verdict.Evidence,
		})
	}
	return normalized, nil
}

func RenderNormalizedVerdicts(rows []NormalizedVerdict) string {
	var rendered strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&rendered, "%s\t%s\t%s\n", row.Kind, row.MapPath, row.Evidence)
	}
	return rendered.String()
}

func parseSeatVerdict(value string) (SeatVerdict, bool) {
	switch SeatVerdict(value) {
	case VerdictConfirm:
		return VerdictConfirm, true
	case VerdictAmend:
		return VerdictAmend, true
	case VerdictRefute:
		return VerdictRefute, true
	default:
		return "", false
	}
}
