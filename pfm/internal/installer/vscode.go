package installer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
)

const (
	vscodeOwnershipName    = "vscode-ownership.json"
	vscodeOwnershipVersion = 1
	vscodeProfileName      = "PFM"
)

var errMalformedVSCodeSettings = errors.New("malformed VS Code settings")

type vscodeOwnershipDocument struct {
	Version int                     `json:"version"`
	Files   []vscodeOwnershipRecord `json:"files"`
}

type vscodeOwnershipRecord struct {
	Path                  string          `json:"path"`
	Platform              string          `json:"platform"`
	FileAdded             bool            `json:"fileAdded,omitempty"`
	ProfileOwned          bool            `json:"profileOwned,omitempty"`
	ProfilesPropertyAdded bool            `json:"profilesPropertyAdded,omitempty"`
	DefaultOwned          bool            `json:"defaultOwned,omitempty"`
	HadDefault            bool            `json:"hadDefault,omitempty"`
	PreviousDefault       json.RawMessage `json:"previousDefault,omitempty"`
	// ScalarOwned/HadScalar/PreviousScalar extend DefaultOwned/HadDefault/
	// PreviousDefault's exact relinquish-then-claim shape to every key in
	// vscodeScalarKeys: tmux (`tmux -L cc-*`) is a chat's survival layer, the
	// VS Code tab only a view onto it — enablePersistentSessions reconnects
	// the view across a window reload; persistentSessionReviveProcess stays
	// "never" because reviving a dead tab after a server death spawns one
	// live picker per tab, a CPU storm (2026-09-03, see internal/ui's
	// idle-picker backoff); showExitAlert off drops the per-chat exit toast;
	// remote.autoForwardPorts off stops VS Code auto-forwarding a fleet
	// chat's stray listening port.
	ScalarOwned    map[string]bool            `json:"scalarOwned,omitempty"`
	HadScalar      map[string]bool            `json:"hadScalar,omitempty"`
	PreviousScalar map[string]json.RawMessage `json:"previousScalar,omitempty"`
}

func (installer *engine) wireVSCode() error {
	ownershipPath := filepath.Join(installer.managedRoot, vscodeOwnershipName)
	ownership, ownershipRaw, err := readVSCodeOwnership(ownershipPath)
	if err != nil {
		return fmt.Errorf("read VS Code ownership %s: %w", ownershipPath, err)
	}
	if installer.options.Mode == ModeUninstall {
		return installer.unwireVSCode(ownershipPath, ownershipRaw, ownership)
	}
	if !installer.options.VSCode && len(ownership) == 0 {
		return nil
	}

	livePlatform, err := installer.vscodePlatform()
	if err != nil {
		return err
	}
	paths := make(map[string]bool, len(ownership))
	for path := range ownership {
		paths[path] = true
	}
	if installer.options.VSCode {
		for _, path := range installer.vscodeSettingsPaths() {
			paths[path] = true
		}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)

	for _, path := range ordered {
		record, alreadyOwned := ownership[path]
		if !alreadyOwned {
			record = vscodeOwnershipRecord{Path: path, Platform: livePlatform}
		}
		updated, next, changed, err := installer.mergeVSCodeSettings(path, record, alreadyOwned)
		if err != nil {
			if errors.Is(err, errMalformedVSCodeSettings) {
				installer.skip("VS Code settings skipped " + path + ": " + err.Error())
				continue
			}
			return err
		}
		if next.ProfileOwned || next.DefaultOwned || len(next.ScalarOwned) != 0 {
			ownership[path] = next
		} else {
			delete(ownership, path)
		}
		if !changed {
			installer.ok("VS Code PFM terminal profile " + path)
			continue
		}
		if err := installer.change("merge VS Code PFM terminal profile "+path, func() error {
			mode := fs.FileMode(0o600)
			if info, statErr := os.Stat(path); statErr == nil {
				mode = info.Mode().Perm()
				if err := copyBackup(path, availableBackup(path, installer.stamp)); err != nil {
					return err
				}
			} else if !errors.Is(statErr, fs.ErrNotExist) {
				return statErr
			}
			return atomicWrite(path, updated, mode)
		}); err != nil {
			return err
		}
	}
	return installer.writeVSCodeOwnership(ownershipPath, ownershipRaw, ownership)
}

func (installer *engine) mergeVSCodeSettings(path string, record vscodeOwnershipRecord, alreadyOwned bool) ([]byte, vscodeOwnershipRecord, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		raw = []byte("{}\n")
		if !alreadyOwned {
			record.FileAdded = true
		}
	} else if err != nil {
		return nil, record, false, fmt.Errorf("read VS Code settings %s: %w", path, err)
	}
	document, err := decodeJSONCObject(raw)
	if err != nil {
		return nil, record, false, fmt.Errorf("%w: decode VS Code settings %s: %v", errMalformedVSCodeSettings, path, err)
	}
	profileKey, defaultKey := vscodeSettingKeys(record.Platform)
	canonical := vscodeProfile()

	profilesValue, hasProfiles := document[profileKey]
	var profiles map[string]any
	if hasProfiles {
		var ok bool
		profiles, ok = profilesValue.(map[string]any)
		if !ok {
			return nil, record, false, fmt.Errorf("%w: VS Code settings %s: %s must be an object", errMalformedVSCodeSettings, path, profileKey)
		}
	} else {
		profiles = map[string]any{}
	}
	existingProfile, hasProfile := profiles[vscodeProfileName]
	// An exact previously managed profile is an upgrade, not an operator edit.
	upgradingProfile := alreadyOwned && record.ProfileOwned && isLegacyVSCodeProfile(existingProfile)
	profileRelinquished := false
	if alreadyOwned && record.ProfileOwned && hasProfile && !reflect.DeepEqual(existingProfile, canonical) && !upgradingProfile {
		// A user edit after installation wins. Relinquish this field instead of
		// rewriting it during an unrelated update.
		record.ProfileOwned = false
		record.ProfilesPropertyAdded = false
		profileRelinquished = true
	}
	if installer.options.VSCode || record.ProfileOwned {
		if hasProfile && !reflect.DeepEqual(existingProfile, canonical) && !upgradingProfile {
			if !profileRelinquished {
				return nil, record, false, fmt.Errorf("VS Code settings %s: profile %q already exists and is not PFM-owned", path, vscodeProfileName)
			}
		}
		if !hasProfile {
			record.ProfileOwned = true
			record.ProfilesPropertyAdded = !hasProfiles
		}
	}

	existingDefault, hasDefault := document[defaultKey]
	if alreadyOwned && record.DefaultOwned && (!hasDefault || existingDefault != vscodeProfileName) {
		// Selecting another default terminal is an intentional operator override.
		record.DefaultOwned = false
		record.HadDefault = false
		record.PreviousDefault = nil
	}
	if installer.options.VSCode && !record.DefaultOwned && (!hasDefault || existingDefault != vscodeProfileName) {
		record.DefaultOwned = true
		record.HadDefault = hasDefault
		if hasDefault {
			record.PreviousDefault, err = json.Marshal(existingDefault)
			if err != nil {
				return nil, record, false, fmt.Errorf("preserve VS Code default in %s: %w", path, err)
			}
		}
	}

	for _, key := range vscodeScalarKeys {
		want := vscodeScalarValue(key)
		existing, hasExisting := document[key]
		if alreadyOwned && record.ScalarOwned[key] && (!hasExisting || existing != want) {
			// An operator's own edit after installation wins, same as DefaultOwned.
			delete(record.ScalarOwned, key)
			delete(record.HadScalar, key)
			delete(record.PreviousScalar, key)
		}
		if installer.options.VSCode && !record.ScalarOwned[key] && (!hasExisting || existing != want) {
			if record.ScalarOwned == nil {
				record.ScalarOwned = map[string]bool{}
			}
			record.ScalarOwned[key] = true
			if hasExisting {
				previousRaw, marshalErr := json.Marshal(existing)
				if marshalErr != nil {
					return nil, record, false, fmt.Errorf("preserve VS Code %s in %s: %w", key, path, marshalErr)
				}
				if record.HadScalar == nil {
					record.HadScalar = map[string]bool{}
				}
				record.HadScalar[key] = true
				if record.PreviousScalar == nil {
					record.PreviousScalar = map[string]json.RawMessage{}
				}
				record.PreviousScalar[key] = previousRaw
			}
		}
	}

	updated := append([]byte(nil), raw...)
	changed := false
	if record.ProfileOwned && !reflect.DeepEqual(existingProfile, canonical) {
		profileRaw, marshalErr := json.MarshalIndent(canonical, "", "  ")
		if marshalErr != nil {
			return nil, record, false, marshalErr
		}
		if hasProfiles {
			root, parseErr := parseJSONCObject(updated, 0)
			if parseErr != nil {
				return nil, record, false, parseErr
			}
			property := root.byName[profileKey]
			updated, parseErr = setJSONCProperty(updated, property.valueStart, vscodeProfileName, profileRaw)
			if parseErr != nil {
				return nil, record, false, parseErr
			}
		} else {
			profilesRaw, marshalErr := json.MarshalIndent(map[string]any{vscodeProfileName: canonical}, "", "  ")
			if marshalErr != nil {
				return nil, record, false, marshalErr
			}
			updated, err = setJSONCProperty(updated, 0, profileKey, profilesRaw)
			if err != nil {
				return nil, record, false, err
			}
		}
		changed = true
	}
	if record.DefaultOwned && (!hasDefault || existingDefault != vscodeProfileName) {
		updated, err = setJSONCProperty(updated, 0, defaultKey, []byte(`"`+vscodeProfileName+`"`))
		if err != nil {
			return nil, record, false, err
		}
		changed = true
	}
	for _, key := range vscodeScalarKeys {
		if !record.ScalarOwned[key] {
			continue
		}
		want := vscodeScalarValue(key)
		if existing, hasExisting := document[key]; hasExisting && existing == want {
			continue
		}
		valueRaw, marshalErr := json.Marshal(want)
		if marshalErr != nil {
			return nil, record, false, marshalErr
		}
		updated, err = setJSONCProperty(updated, 0, key, valueRaw)
		if err != nil {
			return nil, record, false, err
		}
		changed = true
	}
	return updated, record, changed, nil
}

func (installer *engine) unwireVSCode(path string, existing []byte, ownership map[string]vscodeOwnershipRecord) error {
	ordered := make([]string, 0, len(ownership))
	for settings := range ownership {
		ordered = append(ordered, settings)
	}
	sort.Strings(ordered)
	for _, settings := range ordered {
		record := ownership[settings]
		raw, err := os.ReadFile(settings)
		if errors.Is(err, fs.ErrNotExist) {
			delete(ownership, settings)
			continue
		}
		if err != nil {
			return fmt.Errorf("read VS Code settings %s: %w", settings, err)
		}
		document, err := decodeJSONCObject(raw)
		if err != nil {
			installer.skip("VS Code settings skipped " + settings + ": " + fmt.Errorf("%w: decode VS Code settings: %v", errMalformedVSCodeSettings, err).Error())
			continue
		}
		profileKey, defaultKey := vscodeSettingKeys(record.Platform)
		updated := append([]byte(nil), raw...)
		changed := false
		if record.DefaultOwned && document[defaultKey] == vscodeProfileName {
			if record.HadDefault {
				updated, err = setJSONCProperty(updated, 0, defaultKey, record.PreviousDefault)
			} else {
				updated, err = removeJSONCProperty(updated, 0, defaultKey)
			}
			if err != nil {
				return err
			}
			changed = true
			record.DefaultOwned = false
			record.HadDefault = false
			record.PreviousDefault = nil
		}
		for _, key := range vscodeScalarKeys {
			if !record.ScalarOwned[key] || document[key] != vscodeScalarValue(key) {
				continue
			}
			if record.HadScalar[key] {
				updated, err = setJSONCProperty(updated, 0, key, record.PreviousScalar[key])
			} else {
				updated, err = removeJSONCProperty(updated, 0, key)
			}
			if err != nil {
				return err
			}
			changed = true
			delete(record.ScalarOwned, key)
			delete(record.HadScalar, key)
			delete(record.PreviousScalar, key)
		}
		profileRetained := false
		if record.ProfileOwned {
			// Re-decode after the root edit because byte offsets have changed.
			current, decodeErr := decodeJSONCObject(updated)
			if decodeErr != nil {
				return decodeErr
			}
			profiles, _ := current[profileKey].(map[string]any)
			profile, hasProfile := profiles[vscodeProfileName]
			if reflect.DeepEqual(profile, vscodeProfile()) || isLegacyVSCodeProfile(profile) {
				if record.ProfilesPropertyAdded && len(profiles) == 1 {
					updated, err = removeJSONCProperty(updated, 0, profileKey)
				} else {
					root, parseErr := parseJSONCObject(updated, 0)
					if parseErr != nil {
						return parseErr
					}
					updated, err = removeJSONCProperty(updated, root.byName[profileKey].valueStart, vscodeProfileName)
				}
				if err != nil {
					return err
				}
				changed = true
				record.ProfileOwned = false
				record.ProfilesPropertyAdded = false
			} else if hasProfile {
				profileRetained = true
			}
		}
		if changed {
			removeEmptyFile := false
			if record.FileAdded {
				current, decodeErr := decodeJSONCObject(updated)
				if decodeErr != nil {
					return decodeErr
				}
				removeEmptyFile = len(current) == 0 && !bytes.Contains(updated, []byte("//")) && !bytes.Contains(updated, []byte("/*"))
			}
			if err := installer.change("remove VS Code PFM terminal profile "+settings, func() error {
				if removeEmptyFile {
					return os.Remove(settings)
				}
				info, err := os.Stat(settings)
				if err != nil {
					return err
				}
				if err := copyBackup(settings, availableBackup(settings, installer.stamp)); err != nil {
					return err
				}
				return atomicWrite(settings, updated, info.Mode().Perm())
			}); err != nil {
				return err
			}
		} else {
			installer.ok("VS Code settings preserved " + settings)
		}
		if profileRetained {
			ownership[settings] = record
			installer.skip("VS Code PFM terminal profile was edited; left it in place and retained recovery ownership at " + settings)
		} else {
			delete(ownership, settings)
		}
	}
	return installer.writeVSCodeOwnership(path, existing, ownership)
}

func (installer *engine) writeVSCodeOwnership(path string, existing []byte, ownership map[string]vscodeOwnershipRecord) error {
	if len(ownership) == 0 {
		if len(existing) == 0 {
			return nil
		}
		return installer.change("remove "+path, func() error { return os.Remove(path) })
	}
	document := vscodeOwnershipDocument{Version: vscodeOwnershipVersion}
	for _, record := range ownership {
		document.Files = append(document.Files, record)
	}
	sort.Slice(document.Files, func(i, j int) bool { return document.Files[i].Path < document.Files[j].Path })
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if bytes.Equal(existing, encoded) && sameFile(path, encoded, 0o600) {
		installer.ok(path)
		return nil
	}
	return installer.change("write "+path, func() error { return atomicWrite(path, encoded, 0o600) })
}

func readVSCodeOwnership(path string) (map[string]vscodeOwnershipRecord, []byte, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]vscodeOwnershipRecord{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var document vscodeOwnershipDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, nil, err
	}
	if document.Version != vscodeOwnershipVersion {
		return nil, nil, fmt.Errorf("unsupported version %d", document.Version)
	}
	records := make(map[string]vscodeOwnershipRecord, len(document.Files))
	for _, record := range document.Files {
		if !filepath.IsAbs(record.Path) || (record.Platform != "linux" && record.Platform != "osx") {
			return nil, nil, fmt.Errorf("invalid record path/platform %q/%q", record.Path, record.Platform)
		}
		if _, duplicate := records[record.Path]; duplicate {
			return nil, nil, fmt.Errorf("duplicate record %s", record.Path)
		}
		records[record.Path] = record
	}
	return records, raw, nil
}

func (installer *engine) vscodePlatform() (string, error) {
	platform := installer.options.vscodePlatform
	if platform == "" {
		platform = runtime.GOOS
	}
	switch platform {
	case "darwin", "osx":
		return "osx", nil
	case "linux":
		return "linux", nil
	default:
		return "", fmt.Errorf("VS Code PFM terminal profile is unsupported on %s", platform)
	}
}

func (installer *engine) vscodeSettingsPaths() []string {
	if installer.options.vscodeSettingsPaths != nil {
		return cleanUniquePaths(installer.options.vscodeSettingsPaths)
	}
	home := installer.options.Home
	platform, _ := installer.vscodePlatform()
	var canonical string
	var candidates []string
	if platform == "osx" {
		canonical = filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json")
		candidates = []string{
			canonical,
			filepath.Join(home, "Library", "Application Support", "Code - Insiders", "User", "settings.json"),
			filepath.Join(home, "Library", "Application Support", "VSCodium", "User", "settings.json"),
		}
	} else {
		configRoot := os.Getenv("XDG_CONFIG_HOME")
		if !filepath.IsAbs(configRoot) {
			configRoot = filepath.Join(home, ".config")
		}
		canonical = filepath.Join(configRoot, "Code", "User", "settings.json")
		candidates = []string{
			canonical,
			filepath.Join(configRoot, "Code - Insiders", "User", "settings.json"),
			filepath.Join(configRoot, "VSCodium", "User", "settings.json"),
			filepath.Join(home, ".vscode-server", "data", "Machine", "settings.json"),
			filepath.Join(home, ".vscode-server-insiders", "data", "Machine", "settings.json"),
		}
	}
	if portable := os.Getenv("VSCODE_PORTABLE"); filepath.IsAbs(portable) {
		candidates = append(candidates, filepath.Join(portable, "data", "user-data", "User", "settings.json"))
	}
	var found []string
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			found = append(found, candidate)
			continue
		}
		if info, err := os.Stat(filepath.Dir(candidate)); err == nil && info.IsDir() {
			found = append(found, candidate)
		}
	}
	if len(found) == 0 {
		found = []string{canonical}
	}
	return cleanUniquePaths(found)
}

func cleanUniquePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	var result []string
	for _, path := range paths {
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if !seen[path] {
			seen[path] = true
			result = append(result, path)
		}
	}
	sort.Strings(result)
	return result
}

func vscodeSettingKeys(platform string) (string, string) {
	return "terminal.integrated.profiles." + platform, "terminal.integrated.defaultProfile." + platform
}

func isLegacyVSCodeProfile(profile any) bool {
	legacy := vscodeProfile()
	legacy["env"] = map[string]any{"CC_AUTO_OPEN": "pfm"}
	return reflect.DeepEqual(profile, legacy)
}

func vscodeProfile() map[string]any {
	return map[string]any{
		"path": "/bin/zsh",
		"args": []any{"-l"},
		"env":  map[string]any{"PFM_AUTO_OPEN": "pfm"},
	}
}

// vscodeScalarKeys are the fixed-value settings pfm install owns besides the
// default-profile key (handled separately — it names this profile, not a
// constant). Unlike terminal.integrated.profiles.<platform>/defaultProfile,
// these are not platform-suffixed: VS Code reads them the same on every OS.
var vscodeScalarKeys = []string{
	"terminal.integrated.enablePersistentSessions",
	"terminal.integrated.persistentSessionReviveProcess",
	"terminal.integrated.showExitAlert",
	"remote.autoForwardPorts",
}

// vscodeScalarValue is the value pfm install owns key to. See
// vscodeOwnershipRecord.ScalarOwned for why each one is what it is.
func vscodeScalarValue(key string) any {
	switch key {
	case "terminal.integrated.enablePersistentSessions":
		return true
	case "terminal.integrated.persistentSessionReviveProcess":
		return "never"
	case "terminal.integrated.showExitAlert":
		return false
	case "remote.autoForwardPorts":
		return false
	default:
		return nil
	}
}

// The editor accepts JSONC, so installer edits must not round-trip the whole
// document through encoding/json and erase the operator's comments. These
// helpers parse enough structure to replace one object property surgically.
type jsoncProperty struct {
	name                 string
	keyStart, valueStart int
	valueEnd, commaStart int
	commaEnd             int
}

type jsoncObject struct {
	open, close int
	properties  []jsoncProperty
	byName      map[string]jsoncProperty
}

func decodeJSONCObject(raw []byte) (map[string]any, error) {
	sanitized, err := sanitizeJSONC(raw)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := json.Unmarshal(sanitized, &document); err != nil {
		return nil, err
	}
	if _, err := parseJSONCObject(raw, 0); err != nil {
		return nil, err
	}
	return document, nil
}

func sanitizeJSONC(raw []byte) ([]byte, error) {
	clean := append([]byte(nil), raw...)
	inString, escaped := false, false
	for index := 0; index < len(clean); index++ {
		character := clean[index]
		if inString {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == '"' {
				inString = false
			}
			continue
		}
		if character == '"' {
			inString = true
			continue
		}
		if character != '/' || index+1 >= len(clean) {
			continue
		}
		switch clean[index+1] {
		case '/':
			clean[index], clean[index+1] = ' ', ' '
			index += 2
			for index < len(clean) && clean[index] != '\n' {
				clean[index] = ' '
				index++
			}
			index--
		case '*':
			clean[index], clean[index+1] = ' ', ' '
			index += 2
			closed := false
			for index < len(clean) {
				if index+1 < len(clean) && clean[index] == '*' && clean[index+1] == '/' {
					clean[index], clean[index+1] = ' ', ' '
					index++
					closed = true
					break
				}
				if clean[index] != '\n' && clean[index] != '\r' {
					clean[index] = ' '
				}
				index++
			}
			if !closed {
				return nil, errors.New("unterminated block comment")
			}
		}
	}
	if inString {
		return nil, errors.New("unterminated string")
	}
	// A comma is trailing — VS Code accepts it, encoding/json does not — when
	// nothing but whitespace and/or MORE commas separates it from the object
	// or array's closing bracket. A hand edit that leaves two properties
	// each ending in their own comma back to back (a block moved, or two
	// edits landing on the same spot) is exactly this, one comma deeper than
	// the single-comma case a lookahead of whitespace alone would catch.
	for index, character := range clean {
		if character != ',' {
			continue
		}
		next := index + 1
		for next < len(clean) && (isJSONWhitespace(clean[next]) || clean[next] == ',') {
			next++
		}
		if next < len(clean) && (clean[next] == '}' || clean[next] == ']') {
			clean[index] = ' '
		}
	}
	return clean, nil
}

func parseJSONCObject(raw []byte, start int) (jsoncObject, error) {
	open, err := skipJSONCTrivia(raw, start)
	if err != nil {
		return jsoncObject{}, err
	}
	if open >= len(raw) || raw[open] != '{' {
		return jsoncObject{}, fmt.Errorf("expected object at byte %d", open)
	}
	result := jsoncObject{open: open, byName: map[string]jsoncProperty{}}
	index := open + 1
	for {
		index, err = skipJSONCTrivia(raw, index)
		if err != nil {
			return jsoncObject{}, err
		}
		if index >= len(raw) {
			return jsoncObject{}, errors.New("unterminated object")
		}
		if raw[index] == '}' {
			result.close = index
			return result, nil
		}
		if raw[index] == ',' { // trailing commas and the separator after a prior member
			index++
			continue
		}
		if raw[index] != '"' {
			return jsoncObject{}, fmt.Errorf("expected object key at byte %d", index)
		}
		keyStart := index
		keyEnd, err := scanJSONString(raw, index)
		if err != nil {
			return jsoncObject{}, err
		}
		var name string
		if err := json.Unmarshal(raw[index:keyEnd], &name); err != nil {
			return jsoncObject{}, err
		}
		if _, duplicate := result.byName[name]; duplicate {
			return jsoncObject{}, fmt.Errorf("duplicate object key %q", name)
		}
		index, err = skipJSONCTrivia(raw, keyEnd)
		if err != nil || index >= len(raw) || raw[index] != ':' {
			return jsoncObject{}, fmt.Errorf("expected colon after %q", name)
		}
		valueStart, err := skipJSONCTrivia(raw, index+1)
		if err != nil {
			return jsoncObject{}, err
		}
		valueEnd, err := scanJSONCValue(raw, valueStart)
		if err != nil {
			return jsoncObject{}, fmt.Errorf("scan %q: %w", name, err)
		}
		after, err := skipJSONCTrivia(raw, valueEnd)
		if err != nil {
			return jsoncObject{}, err
		}
		property := jsoncProperty{name: name, keyStart: keyStart, valueStart: valueStart, valueEnd: valueEnd, commaStart: -1, commaEnd: -1}
		if after < len(raw) && raw[after] == ',' {
			property.commaStart, property.commaEnd = after, after+1
			index = after + 1
		} else if after < len(raw) && raw[after] == '}' {
			index = after
		} else {
			return jsoncObject{}, fmt.Errorf("expected comma or object end after %q", name)
		}
		result.properties = append(result.properties, property)
		result.byName[name] = property
	}
}

func setJSONCProperty(raw []byte, objectStart int, name string, value []byte) ([]byte, error) {
	if !json.Valid(value) {
		return nil, fmt.Errorf("replacement for %q is not JSON", name)
	}
	object, err := parseJSONCObject(raw, objectStart)
	if err != nil {
		return nil, err
	}
	if property, found := object.byName[name]; found {
		return spliceBytes(raw, property.valueStart, property.valueEnd, value), nil
	}
	closingIndent := lineIndent(raw, object.close)
	childIndent := closingIndent + "  "
	formatted := indentJSON(value, childIndent)
	prefix := ""
	if len(object.properties) != 0 && object.properties[len(object.properties)-1].commaStart < 0 {
		prefix = ","
	}
	// No trailing comma after the inserted property: it is the new last
	// member, sitting directly against the closing bracket, so the result
	// is valid strict JSON there, not merely JSONC. A LATER insertion into
	// the same object still lands correctly — it is the "last property has
	// no comma" case the prefix check above already exists to handle.
	insertion := []byte(prefix + "\n" + childIndent + string(mustJSON(name)) + ": " + string(formatted) + "\n" + closingIndent)
	return spliceBytes(raw, object.close, object.close, insertion), nil
}

func removeJSONCProperty(raw []byte, objectStart int, name string) ([]byte, error) {
	object, err := parseJSONCObject(raw, objectStart)
	if err != nil {
		return nil, err
	}
	property, found := object.byName[name]
	if !found {
		return raw, nil
	}
	if property.commaStart >= 0 {
		return spliceBytes(raw, property.keyStart, property.commaEnd, nil), nil
	}
	// A formatter may have removed the trailing comma PFM writes. Remove only
	// the preceding separator byte; comments and whitespace around it survive.
	for index := len(object.properties) - 1; index >= 0; index-- {
		prior := object.properties[index]
		if prior.keyStart >= property.keyStart {
			continue
		}
		withoutProperty := spliceBytes(raw, property.keyStart, property.valueEnd, nil)
		if prior.commaStart >= 0 {
			return spliceBytes(withoutProperty, prior.commaStart, prior.commaEnd, nil), nil
		}
		break
	}
	return spliceBytes(raw, property.keyStart, property.valueEnd, nil), nil
}

func scanJSONCValue(raw []byte, start int) (int, error) {
	if start >= len(raw) {
		return 0, errors.New("missing value")
	}
	if raw[start] == '"' {
		return scanJSONString(raw, start)
	}
	if raw[start] == '{' || raw[start] == '[' {
		stack := []byte{raw[start]}
		inString, escaped := false, false
		for index := start + 1; index < len(raw); index++ {
			character := raw[index]
			if inString {
				if escaped {
					escaped = false
				} else if character == '\\' {
					escaped = true
				} else if character == '"' {
					inString = false
				}
				continue
			}
			if character == '"' {
				inString = true
				continue
			}
			if character == '/' && index+1 < len(raw) && (raw[index+1] == '/' || raw[index+1] == '*') {
				next, err := skipOneJSONCComment(raw, index)
				if err != nil {
					return 0, err
				}
				index = next - 1
				continue
			}
			switch character {
			case '{', '[':
				stack = append(stack, character)
			case '}', ']':
				open := stack[len(stack)-1]
				if (open == '{' && character != '}') || (open == '[' && character != ']') {
					return 0, fmt.Errorf("mismatched %c at byte %d", character, index)
				}
				stack = stack[:len(stack)-1]
				if len(stack) == 0 {
					return index + 1, nil
				}
			}
		}
		return 0, errors.New("unterminated composite value")
	}
	index := start
	for index < len(raw) && raw[index] != ',' && raw[index] != '}' && raw[index] != ']' && !isJSONWhitespace(raw[index]) {
		if raw[index] == '/' && index+1 < len(raw) && (raw[index+1] == '/' || raw[index+1] == '*') {
			break
		}
		index++
	}
	if index == start {
		return 0, errors.New("empty primitive value")
	}
	return index, nil
}

func scanJSONString(raw []byte, start int) (int, error) {
	escaped := false
	for index := start + 1; index < len(raw); index++ {
		if escaped {
			escaped = false
			continue
		}
		if raw[index] == '\\' {
			escaped = true
			continue
		}
		if raw[index] == '"' {
			return index + 1, nil
		}
	}
	return 0, errors.New("unterminated string")
}

func skipJSONCTrivia(raw []byte, start int) (int, error) {
	index := start
	for index < len(raw) {
		if isJSONWhitespace(raw[index]) {
			index++
			continue
		}
		if raw[index] == '/' && index+1 < len(raw) && (raw[index+1] == '/' || raw[index+1] == '*') {
			next, err := skipOneJSONCComment(raw, index)
			if err != nil {
				return 0, err
			}
			index = next
			continue
		}
		break
	}
	return index, nil
}

func skipOneJSONCComment(raw []byte, start int) (int, error) {
	if raw[start+1] == '/' {
		index := start + 2
		for index < len(raw) && raw[index] != '\n' {
			index++
		}
		return index, nil
	}
	for index := start + 2; index+1 < len(raw); index++ {
		if raw[index] == '*' && raw[index+1] == '/' {
			return index + 2, nil
		}
	}
	return 0, errors.New("unterminated block comment")
}

func isJSONWhitespace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\r' || character == '\n'
}

func spliceBytes(raw []byte, start, end int, replacement []byte) []byte {
	result := make([]byte, 0, len(raw)-(end-start)+len(replacement))
	result = append(result, raw[:start]...)
	result = append(result, replacement...)
	result = append(result, raw[end:]...)
	return result
}

func lineIndent(raw []byte, position int) string {
	start := bytes.LastIndexByte(raw[:position], '\n') + 1
	end := start
	for end < position && (raw[end] == ' ' || raw[end] == '\t') {
		end++
	}
	return string(raw[start:end])
}

func indentJSON(raw []byte, indent string) []byte {
	return bytes.ReplaceAll(raw, []byte("\n"), []byte("\n"+indent))
}

func mustJSON(value string) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}
