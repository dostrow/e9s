//go:build gui && !vte && windows

package gui

func terminalUnavailableDetail() string {
	return "Embedded terminals are not yet available on Windows. A native ConPTY backend is planned; AWS browsing and non-terminal actions remain available."
}
