package artifact

import (
	"fmt"
	"regexp"
	"strings"
)

var mapFilePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.md$`)

func ParseLaneMembership(text string) (LaneMembership, error) {
	membership := make(LaneMembership)
	var problems []string
	for offset, row := range linesOf(text) {
		fields := strings.Split(row, "\t")
		mapFile := fieldAt(fields, 0)
		lane := fieldAt(fields, 1)
		if len(fields) != 2 || !mapFilePattern.MatchString(mapFile) || !laneSlugPattern.MatchString(lane) {
			problems = append(problems, fmt.Sprintf("line %d: invalid lane row", offset+1))
		} else if _, exists := membership[mapFile]; exists {
			problems = append(problems, fmt.Sprintf("line %d: duplicate lane row for %s", offset+1, mapFile))
		} else {
			membership[mapFile] = lane
		}
	}
	if len(problems) != 0 {
		return nil, parseFailure(problems...)
	}
	return membership, nil
}

func RenderLaneMembership(membership LaneMembership) string {
	mapFiles := make([]string, 0, len(membership))
	for mapFile := range membership {
		mapFiles = append(mapFiles, mapFile)
	}
	slicesSort(mapFiles)
	var rendered strings.Builder
	for _, mapFile := range mapFiles {
		fmt.Fprintf(&rendered, "%s\t%s\n", mapFile, membership[mapFile])
	}
	return rendered.String()
}
