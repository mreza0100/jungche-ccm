package installer

import (
	"embed"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed assets
var embeddedAssets embed.FS

type assetFile struct {
	path string
	mode fs.FileMode
}

func assetFiles() ([]assetFile, error) {
	var files []assetFile
	err := fs.WalkDir(embeddedAssets, "assets", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(name, "assets/")
		mode := fs.FileMode(0o644)
		if path.Ext(relative) == ".sh" {
			mode = 0o755
		}
		files = append(files, assetFile{path: relative, mode: mode})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(left, right int) bool {
		return files[left].path < files[right].path
	})
	return files, nil
}

func readAsset(name string) ([]byte, error) {
	return embeddedAssets.ReadFile(path.Join("assets", name))
}
