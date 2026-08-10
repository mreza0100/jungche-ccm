// Package check compares the legacy zsh picker with cc-fleet's composed rows.
package check

import (
	"bufio"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"hostops/cc-fleet/internal/compose"
)

const (
	legacyRowMarker   = "__CC_FLEET_ROW__"
	legacyTupleMarker = "__CC_FLEET_TUPLE__"
)

// Tuple is the deliberately small parity contract shared by both pickers.
type Tuple struct {
	ID      string
	Kind    string
	Project string
	Prompts int64
}

// LegacyRow is a parsed legacy row before its display-only fields are reduced
// to the parity tuple.
type LegacyRow struct {
	Tuple
	Name string
}

// Direction identifies which side contains an unmatched tuple.
type Direction string

const (
	LegacyOnly Direction = "legacy-only"
	GoOnly     Direction = "go-only"
)

// Difference is one unmatched tuple and any matching allowlist reason.
type Difference struct {
	Direction Direction
	Tuple     Tuple
	Allowed   bool
	Class     string
	Reason    string
}

// AllowRule suppresses a precisely described, documented shadow deviation.
type AllowRule struct {
	Class     string
	Direction string
	ID        string
	Kind      string
	Project   string
	Reason    string
}

// TuplesFromRows drops synthetic new-chat actions, which the legacy no-fzf
// fallback never prints.
func TuplesFromRows(rows []compose.Row) []Tuple {
	result := make([]Tuple, 0, len(rows))
	for _, row := range rows {
		if row.Kind == compose.NewClaude || row.Kind == compose.NewCodex {
			continue
		}
		id := row.ID
		// The legacy fallback has no rollout identity column for a live Codex
		// row. Its stable observable identity is the cx-* server socket, which
		// gather also retains on the Go row.
		if row.Kind == compose.LiveCodex {
			id = row.Socket
		}
		if id == "" {
			id = row.Socket
		}
		result = append(result, Tuple{
			ID:      id,
			Kind:    row.Kind.String(),
			Project: clipRunes(row.Project, 14),
			Prompts: row.PromptCount,
		})
	}
	sortTuples(result)
	return result
}

// NormalizeLiveCodex collapses a multi-session cx server to one comparison
// tuple. Legacy repeats the first detected rollout's prompt metadata for every
// session, so prompt counts are not meaningful in that shape; parity is the
// socket identity plus the sorted project set.
func NormalizeLiveCodex(tuples []Tuple) []Tuple {
	type group struct {
		tuples   []Tuple
		projects map[string]struct{}
	}
	groups := make(map[string]*group)
	result := make([]Tuple, 0, len(tuples))
	for _, tuple := range tuples {
		if tuple.Kind != compose.LiveCodex.String() {
			result = append(result, tuple)
			continue
		}
		key := tuple.Kind + "\x00" + tuple.ID
		current := groups[key]
		if current == nil {
			current = &group{projects: make(map[string]struct{})}
			groups[key] = current
		}
		current.tuples = append(current.tuples, tuple)
		current.projects[tuple.Project] = struct{}{}
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		current := groups[key]
		if len(current.tuples) == 1 {
			result = append(result, current.tuples[0])
			continue
		}
		projects := make([]string, 0, len(current.projects))
		for project := range current.projects {
			projects = append(projects, project)
		}
		sort.Strings(projects)
		result = append(result, Tuple{
			ID:      current.tuples[0].ID,
			Kind:    current.tuples[0].Kind,
			Project: strings.Join(projects, ","),
			Prompts: 0,
		})
	}
	sortTuples(result)
	return result
}

// ParseLegacy strips terminal control sequences and accepts both the
// instrumented fallback protocol and ordinary displayed fallback rows.
func ParseLegacy(input string) ([]LegacyRow, error) {
	clean := ansi.Strip(strings.ToValidUTF8(input, "\uFFFD"))
	scanner := bufio.NewScanner(strings.NewReader(clean))
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	rows := make([]LegacyRow, 0)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" ||
			strings.HasPrefix(strings.TrimSpace(line), "cc-ls: fzf not found") {
			continue
		}
		var row LegacyRow
		var err error
		switch {
		case strings.HasPrefix(line, legacyRowMarker+"\t"):
			row, err = parseInstrumented(line)
		case strings.HasPrefix(line, legacyTupleMarker+"\t"):
			row, err = parseTupleLine(line)
		default:
			row, err = parseDisplayLine(line)
		}
		if err != nil {
			return nil, fmt.Errorf("legacy output line %d: %w", lineNumber, err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan legacy output: %w", err)
	}
	return rows, nil
}

func parseInstrumented(line string) (LegacyRow, error) {
	fields := strings.SplitN(line, "\t", 6)
	if len(fields) != 6 {
		return LegacyRow{}, fmt.Errorf("instrumented row has %d fields, want 6", len(fields))
	}
	legacyKind := fields[1]
	id := fields[3]
	if id == "" {
		id = fields[2]
	}
	row, err := parseDisplayLine(fields[5])
	if err != nil {
		return LegacyRow{}, err
	}
	switch legacyKind {
	case "R":
		row.Kind = "resume-claude"
	case "A":
		row.Kind = "agent"
	case "X":
		row.Kind = "resume-codex"
	case "L":
		// The display glyphs distinguish Claude, Codex, and split live rows.
	default:
		return LegacyRow{}, fmt.Errorf("unknown legacy kind %q", legacyKind)
	}
	row.ID = id
	return row, nil
}

func parseTupleLine(line string) (LegacyRow, error) {
	fields := strings.Split(line, "\t")
	if len(fields) != 5 {
		return LegacyRow{}, fmt.Errorf("tuple row has %d fields, want 5", len(fields))
	}
	prompts, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		return LegacyRow{}, fmt.Errorf("prompt count %q: %w", fields[4], err)
	}
	return LegacyRow{Tuple: Tuple{
		ID:      fields[1],
		Kind:    fields[2],
		Project: fields[3],
		Prompts: prompts,
	}}, nil
}

func parseDisplayLine(line string) (LegacyRow, error) {
	columns := strings.Split(line, "│")
	if len(columns) < 3 {
		return LegacyRow{}, fmt.Errorf("not a legacy picker row: %q", line)
	}
	head := strings.TrimSpace(columns[0])
	name := strings.TrimSpace(columns[1])
	promptText := strings.TrimSpace(columns[2])
	promptText = strings.TrimSuffix(promptText, "p")
	prompts, err := strconv.ParseInt(strings.TrimSpace(promptText), 10, 64)
	if err != nil {
		return LegacyRow{}, fmt.Errorf("prompt count %q: %w", columns[2], err)
	}

	kind := ""
	project := head
	switch {
	case strings.HasPrefix(head, "⚙"):
		kind = "agent"
		project = strings.TrimSpace(strings.TrimPrefix(head, "⚙"))
	case strings.HasPrefix(head, "↻"):
		project = strings.TrimSpace(strings.TrimPrefix(head, "↻"))
		if strings.HasPrefix(name, "⬢") {
			kind = "resume-codex"
		} else {
			kind = "resume-claude"
		}
	case strings.HasPrefix(head, "●"):
		project = strings.TrimSpace(strings.TrimPrefix(head, "●"))
		switch {
		case strings.Contains(line, "⊞"):
			kind = "live-split"
		case strings.HasPrefix(name, "⬢"):
			kind = "live-codex"
		default:
			kind = "live-claude"
		}
	default:
		return LegacyRow{}, fmt.Errorf("unknown legacy row glyph in %q", head)
	}
	if project == "" {
		project = "?"
	}
	return LegacyRow{
		Tuple: Tuple{
			Kind:    kind,
			Project: project,
			Prompts: prompts,
		},
		Name: name,
	}, nil
}

// ReconcileIDs fills IDs lost by an uninstrumented legacy fallback using a
// unique kind/project/count match from the Go snapshot. Ambiguities remain
// explicit and therefore surface as differences.
func ReconcileIDs(legacy []LegacyRow, own []Tuple) []Tuple {
	result := make([]Tuple, 0, len(legacy))
	for index, row := range legacy {
		if row.ID != "" {
			result = append(result, row.Tuple)
			continue
		}
		candidates := make([]Tuple, 0, 1)
		for _, tuple := range own {
			if tuple.Kind == row.Kind &&
				tuple.Prompts == row.Prompts &&
				clipRunes(tuple.Project, 14) == row.Project {
				candidates = append(candidates, tuple)
			}
		}
		if len(candidates) == 1 {
			row.ID = candidates[0].ID
			row.Project = candidates[0].Project
		} else {
			row.ID = fmt.Sprintf("unresolved-display-row-%d", index+1)
		}
		result = append(result, row.Tuple)
	}
	sortTuples(result)
	return result
}

// ParseAllowlist reads tab-separated
// direction, id-glob, kind-glob, project-glob, reason rows.
func ParseAllowlist(reader io.Reader) ([]AllowRule, error) {
	scanner := bufio.NewScanner(reader)
	rules := make([]AllowRule, 0)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) == 3 && strings.TrimSpace(fields[0]) == "class" {
			class := strings.TrimSpace(fields[1])
			reason := strings.TrimSpace(fields[2])
			if !validClass(class) {
				return nil, fmt.Errorf(
					"allowlist line %d has invalid class %q",
					lineNumber,
					class,
				)
			}
			if reason == "" {
				return nil, fmt.Errorf(
					"allowlist line %d has an empty class reason",
					lineNumber,
				)
			}
			rules = append(rules, AllowRule{Class: class, Reason: reason})
			continue
		}
		if len(fields) != 5 {
			return nil, fmt.Errorf("allowlist line %d has %d fields, want 5", lineNumber, len(fields))
		}
		for index := range fields {
			fields[index] = strings.TrimSpace(fields[index])
		}
		if fields[0] != string(LegacyOnly) &&
			fields[0] != string(GoOnly) &&
			fields[0] != "either" {
			return nil, fmt.Errorf("allowlist line %d has invalid direction %q", lineNumber, fields[0])
		}
		if fields[1] == "" || fields[2] == "" || fields[3] == "" || fields[4] == "" {
			return nil, fmt.Errorf("allowlist line %d contains an empty field", lineNumber)
		}
		for _, pattern := range fields[1:4] {
			if _, err := path.Match(pattern, "probe"); err != nil {
				return nil, fmt.Errorf("allowlist line %d pattern %q: %w", lineNumber, pattern, err)
			}
		}
		rules = append(rules, AllowRule{
			Direction: fields[0],
			ID:        fields[1],
			Kind:      fields[2],
			Project:   fields[3],
			Reason:    fields[4],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan allowlist: %w", err)
	}
	return rules, nil
}

func validClass(class string) bool {
	switch class {
	case "codex-120-bound",
		"codex-prompt-semantics",
		"claude-prompt-semantics",
		"live-prompt-drift",
		"dormant-account",
		"legacy-dead-server",
		"codex-legacy-source",
		"legacy-nonfleet-multiwindow":
		return true
	default:
		return false
	}
}

// Verification contains the live facts required by semantic allowlist
// classes. A class is never accepted merely because a tuple looks similar.
type Verification struct {
	CodexCandidateIDs map[string]struct{}
	CodexLegacySource map[string]struct{}
	CodexLineageRoots map[string]string
	DormantAccountIDs map[string]struct{}
	LiveCodexSockets  map[string]struct{}
}

// Diff returns the symmetric tuple difference in deterministic order.
func Diff(legacy, own []Tuple, rules []AllowRule) []Difference {
	legacyCount := countTuples(legacy)
	ownCount := countTuples(own)
	keys := make(map[string]Tuple, len(legacyCount)+len(ownCount))
	for _, tuple := range append(append([]Tuple(nil), legacy...), own...) {
		keys[tupleKey(tuple)] = tuple
	}
	differences := make([]Difference, 0)
	for key, tuple := range keys {
		delta := legacyCount[key] - ownCount[key]
		direction := LegacyOnly
		if delta < 0 {
			direction = GoOnly
			delta = -delta
		}
		for range delta {
			difference := Difference{Direction: direction, Tuple: tuple}
			for _, rule := range rules {
				if ruleMatches(rule, difference) {
					difference.Allowed = true
					difference.Reason = rule.Reason
					break
				}
			}
			differences = append(differences, difference)
		}
	}
	sort.Slice(differences, func(left, right int) bool {
		a, b := differences[left], differences[right]
		if a.Direction != b.Direction {
			return a.Direction < b.Direction
		}
		return tupleKey(a.Tuple) < tupleKey(b.Tuple)
	})
	return differences
}

// DiffVerified applies semantic class entries only after verifying their
// defining invariant against both sides or independently collected live facts.
func DiffVerified(
	legacy, own []Tuple,
	rules []AllowRule,
	verification Verification,
) []Difference {
	differences := Diff(legacy, own, rules)
	classReasons := make(map[string]string)
	for _, rule := range rules {
		if rule.Class != "" {
			classReasons[rule.Class] = rule.Reason
		}
	}

	type baseKey struct {
		ID      string
		Kind    string
		Project string
	}
	legacyOnly := make(map[baseKey][]int)
	goOnly := make(map[baseKey][]int)
	for index, difference := range differences {
		if difference.Allowed {
			continue
		}
		key := baseKey{
			ID:      difference.Tuple.ID,
			Kind:    difference.Tuple.Kind,
			Project: difference.Tuple.Project,
		}
		if difference.Direction == LegacyOnly {
			legacyOnly[key] = append(legacyOnly[key], index)
		} else {
			goOnly[key] = append(goOnly[key], index)
		}
	}
	for key, left := range legacyOnly {
		right := goOnly[key]
		pairs := min(len(left), len(right))
		class := promptMismatchClass(key.Kind)
		reason, allowed := classReasons[class]
		if !allowed {
			continue
		}
		for index := 0; index < pairs; index++ {
			markClass(&differences[left[index]], class, reason)
			markClass(&differences[right[index]], class, reason)
		}
	}

	legacyEmptyWindows := make(map[string]int)
	ownLiveClaude := make(map[string]struct{})
	for _, tuple := range legacy {
		if tuple.Kind == compose.LiveClaude.String() && tuple.Prompts == 0 {
			legacyEmptyWindows[tuple.ID]++
		}
	}
	for _, tuple := range own {
		if tuple.Kind == compose.LiveClaude.String() {
			ownLiveClaude[tuple.ID] = struct{}{}
		}
	}
	if reason, allowed := classReasons["legacy-nonfleet-multiwindow"]; allowed {
		for index := range differences {
			difference := &differences[index]
			if difference.Allowed ||
				difference.Direction != LegacyOnly ||
				difference.Tuple.Kind != compose.LiveClaude.String() ||
				difference.Tuple.Prompts != 0 ||
				legacyEmptyWindows[difference.Tuple.ID] < 2 ||
				isFleetSocketID(difference.Tuple.ID) {
				continue
			}
			if _, live := ownLiveClaude[difference.Tuple.ID]; !live {
				markClass(
					difference,
					"legacy-nonfleet-multiwindow",
					reason,
				)
			}
		}
	}

	candidateRoots := verifiedCodexRoots(
		verification.CodexCandidateIDs,
		verification.CodexLineageRoots,
	)
	legacySourceRoots := verifiedCodexRoots(
		verification.CodexLegacySource,
		verification.CodexLineageRoots,
	)
	goOnlyByLineage := make(map[string]int)
	for _, difference := range differences {
		if difference.Allowed ||
			difference.Direction != GoOnly ||
			difference.Tuple.Kind != compose.ResumeCodex.String() {
			continue
		}
		root := verifiedCodexRoot(
			difference.Tuple.ID,
			verification.CodexLineageRoots,
		)
		goOnlyByLineage[root]++
	}

	for index := range differences {
		difference := &differences[index]
		if difference.Allowed {
			continue
		}
		if difference.Direction == LegacyOnly &&
			difference.Tuple.Kind == compose.LiveCodex.String() &&
			verification.LiveCodexSockets != nil {
			if _, live := verification.LiveCodexSockets[difference.Tuple.ID]; !live {
				if reason, allowed := classReasons["legacy-dead-server"]; allowed {
					markClass(difference, "legacy-dead-server", reason)
				}
			}
			continue
		}
		if difference.Direction != GoOnly {
			continue
		}
		if difference.Tuple.Kind == compose.ResumeCodex.String() {
			root := verifiedCodexRoot(
				difference.Tuple.ID,
				verification.CodexLineageRoots,
			)
			if _, insideBound := candidateRoots[root]; !insideBound {
				if reason, allowed := classReasons["codex-120-bound"]; allowed &&
					verification.CodexCandidateIDs != nil &&
					goOnlyByLineage[root] == 1 {
					markClass(difference, "codex-120-bound", reason)
				}
			} else if _, unrecognized := legacySourceRoots[root]; unrecognized {
				if reason, allowed := classReasons["codex-legacy-source"]; allowed {
					markClass(difference, "codex-legacy-source", reason)
				}
			}
		}
		if difference.Tuple.Kind == compose.ResumeClaude.String() {
			if _, verified := verification.DormantAccountIDs[difference.Tuple.ID]; verified {
				if reason, allowed := classReasons["dormant-account"]; allowed {
					markClass(difference, "dormant-account", reason)
				}
			}
		}
	}
	return differences
}

func verifiedCodexRoots(
	ids map[string]struct{},
	roots map[string]string,
) map[string]struct{} {
	if ids == nil {
		return nil
	}
	result := make(map[string]struct{}, len(ids))
	for id := range ids {
		result[verifiedCodexRoot(id, roots)] = struct{}{}
	}
	return result
}

func verifiedCodexRoot(id string, roots map[string]string) string {
	if root := roots[id]; root != "" {
		return root
	}
	return id
}

func isFleetSocketID(value string) bool {
	return strings.HasPrefix(value, "cc-") || strings.HasPrefix(value, "cx-")
}

func promptMismatchClass(kind string) string {
	switch kind {
	case "live-claude", "live-codex":
		return "live-prompt-drift"
	case "resume-codex":
		return "codex-prompt-semantics"
	case "resume-claude":
		return "claude-prompt-semantics"
	default:
		return ""
	}
}

func markClass(difference *Difference, class, reason string) {
	difference.Allowed = true
	difference.Class = class
	difference.Reason = reason
}

func ruleMatches(rule AllowRule, difference Difference) bool {
	if rule.Direction != "either" && rule.Direction != string(difference.Direction) {
		return false
	}
	id, _ := path.Match(rule.ID, difference.Tuple.ID)
	kind, _ := path.Match(rule.Kind, difference.Tuple.Kind)
	project, _ := path.Match(rule.Project, difference.Tuple.Project)
	return id && kind && project
}

func countTuples(tuples []Tuple) map[string]int {
	result := make(map[string]int, len(tuples))
	for _, tuple := range tuples {
		result[tupleKey(tuple)]++
	}
	return result
}

func tupleKey(tuple Tuple) string {
	return tuple.ID + "\x00" + tuple.Kind + "\x00" + tuple.Project + "\x00" +
		strconv.FormatInt(tuple.Prompts, 10)
}

func sortTuples(tuples []Tuple) {
	sort.Slice(tuples, func(left, right int) bool {
		return tupleKey(tuples[left]) < tupleKey(tuples[right])
	})
}

func clipRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
