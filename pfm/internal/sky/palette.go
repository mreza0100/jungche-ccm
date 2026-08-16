package sky

import lipgloss "charm.land/lipgloss/v2"

// styleClass enumerates every color role a cell can take. Each engine gets
// four brightness levels; the depth cue drops the far body one level.
type styleClass uint8

const (
	clsNone   styleClass = iota
	clsBgStar            // fixed faint background stars

	clsClaude0 // far-dim steel blue (faint)
	clsClaude1 // dim blue
	clsClaude2 // bright blue
	clsClaude3 // white-hot core

	clsCodex0 // far-dim amber (faint)
	clsCodex1 // dim amber
	clsCodex2 // bright gold
	clsCodex3 // bold gold core

	clsCometHead
	clsCometTail
	clsCometFade

	clsEmberHot
	clsEmberMid
	clsEmberDark

	numClasses
)

// engine selects which star a body renders as.
type engine uint8

const (
	engClaude engine = iota
	engCodex
)

// bodyCls maps engine x brightness level (0..3, clamped) to a styleClass.
func bodyCls(e engine, lvl int) styleClass {
	if lvl < 0 {
		lvl = 0
	}
	if lvl > 3 {
		lvl = 3
	}
	if e == engClaude {
		return clsClaude0 + styleClass(lvl)
	}
	return clsCodex0 + styleClass(lvl)
}

// styleTab pre-renders one lipgloss style per class. 16-color ANSI only,
// same VGA-homage discipline as the sword prototype.
var styleTab = func() [numClasses]lipgloss.Style {
	fg := func(col string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(col))
	}
	var t [numClasses]lipgloss.Style
	t[clsNone] = lipgloss.NewStyle()
	t[clsBgStar] = fg("8").Faint(true)

	t[clsClaude0] = fg("4").Faint(true)
	t[clsClaude1] = fg("4")
	t[clsClaude2] = fg("12")
	t[clsClaude3] = fg("15").Bold(true)

	t[clsCodex0] = fg("3").Faint(true)
	t[clsCodex1] = fg("3")
	t[clsCodex2] = fg("11")
	t[clsCodex3] = fg("11").Bold(true)

	t[clsCometHead] = fg("15").Bold(true)
	t[clsCometTail] = fg("7")
	t[clsCometFade] = fg("8").Faint(true)

	t[clsEmberHot] = fg("9").Bold(true)
	t[clsEmberMid] = fg("1")
	t[clsEmberDark] = fg("1").Faint(true)
	return t
}()
