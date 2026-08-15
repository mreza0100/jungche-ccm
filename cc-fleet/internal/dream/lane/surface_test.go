package lane

import (
	"path/filepath"
	"strings"
	"testing"

	"hostops/cc-fleet/internal/dream/artifact"
)

func TestRenderSurfacesSortsGloballyAndPerLaneAndPreservesNonMapBullets(t *testing.T) {
	maps := t.TempDir()
	mustWrite(t, filepath.Join(maps, "z-map.md"), "# Zulu\n")
	mustWrite(t, filepath.Join(maps, "b-map.md"), "# Beta\n")
	mustWrite(t, filepath.Join(maps, "a-map.md"), "# Alpha\n")
	membership := artifact.LaneMembership{
		"z-map.md": "explorer",
		"b-map.md": "qa",
		"a-map.md": "explorer",
	}
	old := "# old\n- retained second\n- Old pointer -> maps/old-map.md\n- retained first\n"
	first, err := RenderSurfaces(maps, old, membership)
	if err != nil {
		t.Fatalf("RenderSurfaces() error = %v", err)
	}
	second, err := RenderSurfaces(maps, old, membership)
	if err != nil {
		t.Fatalf("RenderSurfaces(second) error = %v", err)
	}
	if first.STM != second.STM || !equalStringMap(first.Agents, second.Agents) {
		t.Fatal("RenderSurfaces() is not byte-stable")
	}
	wantSTM := SurfaceHeader + "\n" +
		"- Alpha -> maps/a-map.md\n" +
		"- Beta -> maps/b-map.md\n" +
		"- Zulu -> maps/z-map.md\n" +
		"- retained second\n" +
		"- retained first\n"
	if first.STM != wantSTM {
		t.Fatalf("STM = %q, want %q", first.STM, wantSTM)
	}
	if first.Agents["explorer"] != AgentSurfaceHeader+"\n- Alpha -> maps/a-map.md\n- Zulu -> maps/z-map.md\n" {
		t.Fatalf("explorer surface = %q", first.Agents["explorer"])
	}
	if first.Agents["qa"] != AgentSurfaceHeader+"\n- Beta -> maps/b-map.md\n" {
		t.Fatalf("qa surface = %q", first.Agents["qa"])
	}
	if !strings.HasSuffix(first.STM, "\n") || strings.HasSuffix(first.STM, "\n\n") {
		t.Fatalf("STM does not carry exactly one final newline: %q", first.STM)
	}
	for lane, body := range first.Agents {
		if !strings.HasSuffix(body, "\n") || strings.HasSuffix(body, "\n\n") {
			t.Fatalf("agent %s does not carry exactly one final newline: %q", lane, body)
		}
	}
}

func TestRenderSurfacesRejectsMissingMembershipUnsafeAndDuplicateTitles(t *testing.T) {
	t.Run("missing membership", func(t *testing.T) {
		maps := t.TempDir()
		mustWrite(t, filepath.Join(maps, "map.md"), "# Subject\n")
		_, err := RenderSurfaces(maps, "", artifact.LaneMembership{})
		assertErrorContains(t, err, "map carries no lane row: map.md")
	})
	t.Run("unsafe title", func(t *testing.T) {
		maps := t.TempDir()
		mustWrite(t, filepath.Join(maps, "map.md"), "# Subject -> maps/injected.md\n")
		_, err := RenderSurfaces(maps, "", artifact.LaneMembership{"map.md": "explorer"})
		assertErrorContains(t, err, "unsafe map title during surface render")
	})
	t.Run("duplicate title", func(t *testing.T) {
		maps := t.TempDir()
		mustWrite(t, filepath.Join(maps, "one.md"), "# Same\n")
		mustWrite(t, filepath.Join(maps, "two.md"), "# Same\n")
		_, err := RenderSurfaces(maps, "", artifact.LaneMembership{"one.md": "explorer", "two.md": "qa"})
		assertErrorContains(t, err, "duplicate map titles prevent deterministic surface generation")
	})
}

func TestRenderSurfacesWithNoMapsStillHasExactHeader(t *testing.T) {
	rendered, err := RenderSurfaces(t.TempDir(), "- keeper\n", artifact.LaneMembership{})
	if err != nil {
		t.Fatalf("RenderSurfaces() error = %v", err)
	}
	if rendered.STM != SurfaceHeader+"\n- keeper\n" {
		t.Fatalf("STM = %q", rendered.STM)
	}
	if len(rendered.Agents) != 0 {
		t.Fatalf("Agents = %#v", rendered.Agents)
	}
}

func TestValidSurfaceDistinguishesEmptyFromMalformedNonempty(t *testing.T) {
	if got, err := ValidSurface(""); err != nil || got != "" {
		t.Fatalf("ValidSurface(empty) = %q, %v", got, err)
	}
	valid := "- Alpha -> maps/a-map.md\n- Beta -> maps/b-map.md\n"
	got, err := ValidSurface(valid)
	if err != nil {
		t.Fatalf("ValidSurface(valid) error = %v", err)
	}
	if got != strings.TrimSuffix(valid, "\n") {
		t.Fatalf("ValidSurface(valid) = %q", got)
	}

	malformed := []string{
		"\n",
		"# header\n",
		"- Alpha -> maps/a-map.md\n\n",
		"- Alpha -> maps/a-map.md -> maps/b-map.md\n",
		"- Alpha -> maps/a-map.md\n- Alpha -> maps/b-map.md\n",
		"- Alpha -> maps/a-map.md\n- Beta -> maps/a-map.md\n",
		"-  Alpha -> maps/a-map.md\n",
	}
	for _, body := range malformed {
		if _, err := ValidSurface(body); err == nil {
			t.Errorf("ValidSurface(%q) error = nil", body)
		}
	}
}

func equalStringMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

// The agent surface floats the lesson, not the title: a title names a subject
// and costs a file read before it teaches anything. A map predating the section
// still indexes on its title, and that fallback is counted rather than silent.
func TestRenderSurfacesFloatsLessonAndCountsMissingOnes(t *testing.T) {
	maps := t.TempDir()
	withLesson := "# Cortex unit selection plugin trap\n\n## Lesson\n\n" +
		"Selecting Cortex unit tests by path pulls the plugin's own conftest into the run — select by marker.\n\n" +
		"## Question\n\nq\n\n## Answer\n\na\n\n## Derivation trail\n\nt\n\n" +
		"Provenance: 2026-08-13 · sid 98039987\n\n## Anchors\n\n" +
		"- `a.py` — blob `0123456789ab`\n- `b.py` — blob `0123456789ac`\n"
	withoutLesson := "# Legacy shaped map\n\n## Question\n\nq\n\n## Answer\n\na\n\n" +
		"## Derivation trail\n\nt\n\nProvenance: 2026-08-13 · sid 98039987\n\n## Anchors\n\n" +
		"- `c.py` — blob `0123456789ad`\n- `d.py` — blob `0123456789ae`\n"
	mustWrite(t, filepath.Join(maps, "with-lesson.md"), withLesson)
	mustWrite(t, filepath.Join(maps, "legacy.md"), withoutLesson)

	rendered, err := RenderSurfaces(maps, "", artifact.LaneMembership{
		"with-lesson.md": "explorer",
		"legacy.md":      "explorer",
	})
	if err != nil {
		t.Fatalf("RenderSurfaces: %v", err)
	}
	wantLesson := "- Selecting Cortex unit tests by path pulls the plugin's own conftest into the run — select by marker. -> maps/with-lesson.md"
	if !strings.Contains(rendered.STM, wantLesson) {
		t.Fatalf("surface does not float the lesson:\n%s", rendered.STM)
	}
	if strings.Contains(rendered.STM, "- Cortex unit selection plugin trap ->") {
		t.Fatalf("surface floated the title over the lesson:\n%s", rendered.STM)
	}
	if !strings.Contains(rendered.STM, "- Legacy shaped map -> maps/legacy.md") {
		t.Fatalf("legacy map lost its title fallback:\n%s", rendered.STM)
	}
	if rendered.MissingLesson != 1 {
		t.Fatalf("MissingLesson = %d, want 1", rendered.MissingLesson)
	}
}
