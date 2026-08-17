// Package resources resolves Dreamer runtime resources across explicit disk
// overlays and the prompt tree embedded in the pfm binary.
package resources

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hostops/pfm/internal/dream/artifact"
	"hostops/pfm/prompts"
)

type layer struct {
	fsys fs.FS
	root string
}

// Resources resolves a dream resource across ordered layers, first hit wins.
// The final layer is always the embedded tree.
type Resources struct {
	layers []layer
}

// NewResources adds nonempty disk roots in priority order, then the embedded
// tree. A missing disk root is an absent override, not an error.
func NewResources(overrideRoots ...string) Resources {
	layers := make([]layer, 0, len(overrideRoots)+1)
	for _, root := range overrideRoots {
		if root == "" {
			continue
		}
		layers = append(layers, layer{fsys: os.DirFS(root), root: root})
	}
	layers = append(layers, layer{fsys: prompts.Dreamer()})
	return Resources{layers: layers}
}

// ReadFile returns the first file declared by any layer.
func (resources Resources) ReadFile(name string) ([]byte, error) {
	raw, _, err := resources.ReadFileWithSource(name)
	return raw, err
}

// ReadFileWithSource also returns the winning disk path or embedded label.
func (resources Resources) ReadFileWithSource(name string) ([]byte, string, error) {
	if err := validateName(name); err != nil {
		return nil, "", err
	}
	for _, candidate := range resources.layers {
		raw, err := candidate.readFile(name)
		if err == nil {
			return raw, candidate.source(name), nil
		}
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return nil, "", artifact.ErrorAt(candidate.source(name), fmt.Errorf(
			"read dream resource %s from %s: %w",
			name,
			candidate.source(name),
			err,
		))
	}
	return nil, "", resources.notFound(name)
}

// ReadDir merges directory entries by name across every layer. The first
// layer to declare a name supplies that entry.
func (resources Resources) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := resources.ReadDirByLayer(name)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].Name() < entries[right].Name()
	})
	return entries, nil
}

// ReadDirByLayer returns the same first-declaration-wins merge while retaining
// layer priority; names are sorted only within each layer. Alias resolution
// uses this view so a later-sorting organ lane still beats an embedded lane.
func (resources Resources) ReadDirByLayer(name string) ([]fs.DirEntry, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	merged := make([]fs.DirEntry, 0)
	found := false
	for _, candidate := range resources.layers {
		entries, err := candidate.readDir(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf(
				"read dream resource directory %s from %s: %w",
				name,
				candidate.source(name),
				err,
			)
		}
		found = true
		sort.Slice(entries, func(left, right int) bool {
			return entries[left].Name() < entries[right].Name()
		})
		for _, entry := range entries {
			if _, exists := seen[entry.Name()]; !exists {
				seen[entry.Name()] = struct{}{}
				merged = append(merged, entry)
			}
		}
	}
	if !found {
		return nil, resources.notFound(name)
	}
	return merged, nil
}

func validateName(name string) error {
	if !fs.ValidPath(name) {
		return fmt.Errorf("invalid dream resource path %q: %w", name, fs.ErrInvalid)
	}
	return nil
}

func (resources Resources) notFound(name string) error {
	locations := make([]string, 0, len(resources.layers))
	for _, candidate := range resources.layers {
		locations = append(locations, candidate.source(name))
	}
	return fmt.Errorf(
		"dream resource %s not found; looked in %s: %w",
		name,
		strings.Join(locations, ", "),
		fs.ErrNotExist,
	)
}

func (candidate layer) source(name string) string {
	if candidate.root == "" {
		return "embedded:" + name
	}
	return filepath.Join(candidate.root, filepath.FromSlash(name))
}

func (candidate layer) readFile(name string) ([]byte, error) {
	if candidate.root != "" {
		if err := inspectDiskPath(candidate.root, name, false); err != nil {
			return nil, err
		}
	}
	raw, err := fs.ReadFile(candidate.fsys, name)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (candidate layer) readDir(name string) ([]fs.DirEntry, error) {
	if candidate.root != "" {
		if err := inspectDiskPath(candidate.root, name, true); err != nil {
			return nil, err
		}
	}
	return fs.ReadDir(candidate.fsys, name)
}

func inspectDiskPath(root, name string, wantDirectory bool) error {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("dream resource root is a symlink: %s", root)
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("dream resource root is not a directory: %s", root)
	}
	current := root
	if name != "." {
		parts := strings.Split(name, "/")
		for index, part := range parts {
			current = filepath.Join(current, part)
			info, err := os.Lstat(current)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("dream resource path is a symlink: %s", current)
			}
			last := index == len(parts)-1
			if !last && !info.IsDir() {
				return fmt.Errorf("dream resource path component is not a directory: %s", current)
			}
			if last && wantDirectory && !info.IsDir() {
				return fmt.Errorf("dream resource directory is not a directory: %s", current)
			}
			if last && !wantDirectory && !info.Mode().IsRegular() {
				return fmt.Errorf("dream resource is not a regular non-symlink file: %s", current)
			}
		}
		return nil
	}
	if !wantDirectory {
		return fmt.Errorf("dream resource is not a regular non-symlink file: %s", root)
	}
	return nil
}
