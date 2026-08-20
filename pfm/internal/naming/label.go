package naming

import (
	"strings"
	"unicode"
)

// BookmarkLabel scrapes a chat's own 🔖 label off a captured pane. It is the
// single implementation of that read (K3): the label resolver, the inject
// sender signature, and the claude half of window-name convergence all ask the
// same question of the same screen text, and three spellings of it drifted
// apart once already.
//
// The anchor is 🔖 PLUS an account medal, never 🌿: a labelled chat outside a
// repository renders no 🌿 and would go unaddressable. The medal set is
// whatever the STATUSLINE can emit, which outlives the fleet's account roster —
// a chat born under a retired account still renders its old medal until it
// exits, so dropping that medal here would silently stop resolving it.
//
// The LAST matching line wins: the statusline is at the bottom of the capture,
// and a transcript quoting an older label appears above it.
func BookmarkLabel(capture string) string {
	return BookmarkLabelFor(capture, nil)
}

// BookmarkLabelFor is BookmarkLabel with the configured account emoji set
// supplied by the caller. Legacy medals remain accepted for old live labels.
func BookmarkLabelFor(capture string, configured []string) string {
	label := ""
	for _, line := range strings.Split(capture, "\n") {
		if !strings.Contains(line, "🔖") || !ContainsMedalFor(line, configured) {
			continue
		}
		index := strings.LastIndex(line, "🔖")
		candidate := strings.TrimLeftFunc(
			line[index+len("🔖"):],
			unicode.IsSpace,
		)
		if separator := strings.Index(candidate, "│"); separator >= 0 {
			candidate = candidate[:separator]
		}
		label = strings.TrimRightFunc(candidate, unicode.IsSpace)
	}
	return label
}

// ContainsMedal reports whether a captured line carries an account medal, the
// marker that tells a 🔖 label line apart from ordinary transcript text. 🍀 is
// the retired account-4 medal: the account is gone, but chats labelled while it
// was live still render it, and without it here those chats are unresolvable.
func ContainsMedal(value string) bool {
	return ContainsMedalFor(value, nil)
}

// ContainsMedalFor recognizes configured account emoji plus the historical
// medals so labels from retired rosters remain resolvable.
func ContainsMedalFor(value string, configured []string) bool {
	for _, emoji := range configured {
		if emoji != "" && emoji != "·" && strings.Contains(value, emoji) {
			return true
		}
	}
	return strings.Contains(value, "🥇") ||
		strings.Contains(value, "🥈") ||
		strings.Contains(value, "🥉") ||
		strings.Contains(value, "🍀")
}
