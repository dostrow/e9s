//go:build gui && !vte && !windows

package gui

func terminalUnavailableDetail() string {
	return "Install the GTK 4 VTE development package and rebuild with the “gui vte” build tags."
}
