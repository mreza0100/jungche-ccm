package harvest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordStatCreatesCacheDirectoryForFirstFailedFetch(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "not-created-yet", "cache")
	harvester := &Harvester{options: Options{CacheDir: cache}}
	harvester.recordStat("missing-source", Result{Error: "missing", ErrorKind: "unresolvable_path"})

	raw, err := os.ReadFile(filepath.Join(cache, statsFilename))
	if err != nil {
		t.Fatalf("first failed fetch did not create its stats scoreboard: %v", err)
	}
	for _, want := range []string{`"item":"missing-source"`, `"ok":false`, `"detail":"unresolvable_path"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("stats scoreboard omitted %q: %s", want, raw)
		}
	}
}
