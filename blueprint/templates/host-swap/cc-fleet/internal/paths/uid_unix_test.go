//go:build unix

package paths

import (
	"os"
	"strconv"
)

func strconvUID() string {
	return strconv.Itoa(os.Getuid())
}
