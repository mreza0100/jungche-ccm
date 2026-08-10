package action

import "strings"

// Quote returns one POSIX-shell single-quoted word. Literal newlines remain
// inside the quotes, so they are data rather than command separators.
func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func hasNUL(values ...string) bool {
	for _, value := range values {
		if strings.IndexByte(value, 0) >= 0 {
			return true
		}
	}
	return false
}
