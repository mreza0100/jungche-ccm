package dream

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"hostops/pfm/internal/dream/lane"
	"hostops/pfm/internal/dream/organ"
)

var hookGitObjectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

type legacySurfaceMap struct {
	path   string
	body   []byte
	lesson string
}

// migrateLegacySurface translates the pre-Go "title -> lesson" lane rows at
// the only place that still has to understand them: a hook consulting that
// surface. Modern surfaces never enter this path. The whole source is parsed,
// every destination is selected, and the final pointer surface is validated
// before the first durable write.
func migrateLegacySurface(
	hookContext organ.HookContext,
	path string,
) (string, error) {
	release, err := acquireRunnerLock(hookContext.Organ)
	if err != nil {
		return "", fmt.Errorf("lock legacy lane surface migration: %w", err)
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil {
			fmt.Fprintf(os.Stderr, "pfm dream hook: release legacy lane surface lock: %v\n", releaseErr)
		}
	}()

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("re-read legacy lane surface %s: %w", path, err)
	}
	if validated, validErr := lane.ValidSurface(string(raw)); validErr == nil {
		return validated, nil
	}
	if err := validateLegacySurfaceFile(path); err != nil {
		return "", err
	}

	relative, err := filepath.Rel(hookContext.OrganRoot, path)
	if err != nil || relative == "." || filepath.IsAbs(relative) ||
		relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("legacy lane surface is outside its repository boundary: %s", path)
	}
	relative = filepath.ToSlash(relative)

	rows, plans, err := planLegacySurface(hookContext.GitRoot, hookContext.Organ, relative, raw)
	if err != nil {
		return "", fmt.Errorf("migrate legacy lane surface %s: %w", path, err)
	}
	rewritten := strings.Join(rows, "\n") + "\n"
	canonical, err := lane.ValidSurface(rewritten)
	if err != nil {
		return "", fmt.Errorf("migrate legacy lane surface %s: validate rewritten surface: %w", path, err)
	}

	created := make([]string, 0, len(plans))
	for _, plan := range plans {
		if err := writeMigratedMapExclusive(plan.path, plan.body); err != nil {
			rollbackCreatedMaps(created)
			return "", fmt.Errorf("migrate legacy lane surface %s: create map %s: %w", path, plan.path, err)
		}
		created = append(created, plan.path)
	}
	if err := atomicPrivateReplace(path, []byte(rewritten)); err != nil {
		rollbackCreatedMaps(created)
		return "", fmt.Errorf("migrate legacy lane surface %s: replace source: %w", path, err)
	}
	return canonical, nil
}

func validateLegacySurfaceFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect legacy lane surface %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("legacy lane surface must be a regular non-symlink file: %s", path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("legacy lane surface is not caller-owned: %s", path)
	}
	return nil
}

func planLegacySurface(
	gitRoot, organRoot, relative string,
	raw []byte,
) ([]string, []legacySurfaceMap, error) {
	if len(raw) == 0 || bytes.Contains(raw, []byte{'\r'}) {
		return nil, nil, errors.New("legacy surface is empty or contains a carriage return")
	}
	canonical := strings.TrimSuffix(string(raw), "\n")
	if canonical == "" || strings.HasSuffix(canonical, "\n") {
		return nil, nil, errors.New("legacy surface contains a blank row")
	}

	blob, err := hookSurfaceBlob(gitRoot, relative)
	if err != nil {
		return nil, nil, err
	}
	rows := strings.Split(canonical, "\n")
	rewritten := make([]string, 0, len(rows))
	plans := make([]legacySurfaceMap, 0, len(rows))
	plannedByPath := make(map[string]legacySurfaceMap)
	legacyCount := 0
	for index, row := range rows {
		if index == 0 && row == lane.AgentSurfaceHeader {
			rewritten = append(rewritten, row)
			continue
		}
		if _, validErr := lane.ValidSurface(row); validErr == nil {
			rewritten = append(rewritten, row)
			continue
		}
		title, lesson, parseErr := parseLegacySurfaceRow(row)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("line %d: %w", index+1, parseErr)
		}
		legacyCount++
		slug := legacySurfaceSlug(title)
		if slug == "" {
			return nil, nil, fmt.Errorf("line %d: title has no ASCII slug", index+1)
		}
		mapName, create, selectErr := selectLegacyMap(
			filepath.Join(organRoot, "maps"), slug, lesson, plannedByPath,
		)
		if selectErr != nil {
			return nil, nil, fmt.Errorf("line %d: %w", index+1, selectErr)
		}
		if create {
			plan := legacySurfaceMap{
				path:   filepath.Join(organRoot, "maps", mapName),
				lesson: lesson,
				body: []byte(renderLegacySurfaceMap(
					title, lesson, relative, index+1, blob[:12],
				)),
			}
			plans = append(plans, plan)
			plannedByPath[plan.path] = plan
		}
		rewritten = append(rewritten, "- "+lesson+" -> maps/"+mapName)
	}
	if legacyCount == 0 {
		return nil, nil, errors.New("invalid surface contains no legacy rows")
	}
	return rewritten, plans, nil
}

func parseLegacySurfaceRow(row string) (string, string, error) {
	if !strings.HasPrefix(row, "- ") {
		return "", "", errors.New("row is neither a pointer nor a legacy lesson")
	}
	content := strings.TrimPrefix(row, "- ")
	separator := strings.Index(content, " -> ")
	if separator <= 0 || separator+4 >= len(content) ||
		strings.Contains(content[separator+4:], " -> ") {
		return "", "", errors.New("legacy row grammar mismatch")
	}
	title := content[:separator]
	lesson := content[separator+4:]
	if looksLikeLegacyDate(title) {
		date := title[:10]
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return "", "", fmt.Errorf("invalid legacy date %q", date)
		}
		title = title[11:]
	}
	if err := validateLegacySurfaceText(title, "title"); err != nil {
		return "", "", err
	}
	if err := validateLegacySurfaceText(lesson, "lesson"); err != nil {
		return "", "", err
	}
	return title, lesson, nil
}

func looksLikeLegacyDate(value string) bool {
	if len(value) < 11 || value[10] != ' ' || value[4] != '-' || value[7] != '-' {
		return false
	}
	for _, index := range []int{0, 1, 2, 3, 5, 6, 8, 9} {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func validateLegacySurfaceText(value, label string) error {
	if value == "" || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("invalid legacy %s", label)
	}
	if strings.Contains(value, " -> ") || strings.Contains(value, " -> maps/") {
		return fmt.Errorf("unsafe legacy %s", label)
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return fmt.Errorf("unsafe legacy %s", label)
		}
	}
	return nil
}

func legacySurfaceSlug(title string) string {
	var result strings.Builder
	separator := false
	for _, character := range title {
		switch {
		case character >= 'A' && character <= 'Z':
			if separator && result.Len() > 0 {
				result.WriteByte('-')
			}
			separator = false
			result.WriteByte(byte(character + ('a' - 'A')))
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			if separator && result.Len() > 0 {
				result.WriteByte('-')
			}
			separator = false
			result.WriteRune(character)
		default:
			separator = result.Len() > 0
		}
	}
	return strings.Trim(result.String(), "-")
}

func selectLegacyMap(
	mapsDirectory, slug, lesson string,
	planned map[string]legacySurfaceMap,
) (string, bool, error) {
	for sequence := 1; ; sequence++ {
		candidateSlug := slug
		if sequence > 1 {
			candidateSlug = fmt.Sprintf("%s-%d", slug, sequence)
		}
		name := candidateSlug + ".md"
		path := filepath.Join(mapsDirectory, name)
		if plan, found := planned[path]; found {
			if plan.lesson == lesson {
				return name, false, nil
			}
			continue
		}
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return name, true, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("inspect map collision %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", false, fmt.Errorf("map collision is not a regular non-symlink file: %s", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", false, fmt.Errorf("read map collision %s: %w", path, err)
		}
		if exactLegacyLesson(string(raw), lesson) {
			return name, false, nil
		}
	}
}

func exactLegacyLesson(body, wanted string) bool {
	if strings.Contains(body, "\r") {
		return false
	}
	rows := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	heading := -1
	for index, row := range rows {
		if row != "## Lesson" {
			continue
		}
		if heading >= 0 {
			return false
		}
		heading = index
	}
	if heading < 0 {
		return false
	}
	lesson := ""
	for _, row := range rows[heading+1:] {
		if strings.HasPrefix(row, "## ") {
			break
		}
		if row == "" {
			continue
		}
		if lesson != "" {
			return false
		}
		lesson = row
	}
	return lesson == wanted
}

func renderLegacySurfaceMap(
	title, lesson, relative string,
	line int,
	blob string,
) string {
	return fmt.Sprintf(
		"# %s\n\n## Lesson\n\n%s\n\n## Anchors\n\n- `%s:%d` — blob `%s`\n",
		title,
		lesson,
		relative,
		line,
		blob,
	)
}

func hookSurfaceBlob(gitRoot, relative string) (string, error) {
	revision := "HEAD:" + relative
	command := exec.Command("git", "-C", gitRoot, "rev-parse", "--verify", "-q", revision)
	command.Env = hookGitReadEnvironment()
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return "", fmt.Errorf("surface is not tracked at hook worktree HEAD: %s", relative)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("resolve hook worktree surface %s: %s", relative, detail)
	}
	value := strings.TrimSpace(string(output))
	if !hookGitObjectID.MatchString(value) {
		return "", fmt.Errorf("resolve hook worktree surface %s returned invalid object id %q", relative, value)
	}
	return value, nil
}

func hookGitReadEnvironment() []string {
	environment := os.Environ()
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if strings.HasPrefix(entry, "GIT_OPTIONAL_LOCKS=") {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "GIT_OPTIONAL_LOCKS=0")
}

func writeMigratedMapExclusive(path string, body []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	clean := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	if _, err := file.Write(body); err != nil {
		clean()
		return err
	}
	if err := file.Sync(); err != nil {
		clean()
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func rollbackCreatedMaps(paths []string) {
	for index := len(paths) - 1; index >= 0; index-- {
		if err := os.Remove(paths[index]); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "pfm dream hook: roll back legacy map %s: %v\n", paths[index], err)
		}
	}
}
