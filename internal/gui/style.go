package gui

import (
	_ "embed"
)

// styleCSS contains structural application styling only. Colors and backgrounds
// intentionally come from the active GTK theme.
//
//go:embed style.css
var styleCSS string
