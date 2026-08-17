// Package prompts owns the runtime prompt assets carried by the pfm binary.
package prompts

import (
	"embed"
	"io/fs"
)

//go:embed dreamer
var embedded embed.FS

// Dreamer is the embedded dreamer resource tree, rooted at the directory that
// contains the prompt files and the lanes/ directory.
func Dreamer() fs.FS {
	tree, err := fs.Sub(embedded, "dreamer")
	if err != nil {
		panic("open embedded dreamer resources: " + err.Error())
	}
	return tree
}
