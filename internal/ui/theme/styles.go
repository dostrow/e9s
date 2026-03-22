// Package theme defines ANSI-adaptive colors and lipgloss styles for the TUI.
package theme

import "github.com/charmbracelet/lipgloss"

// Using ANSI color indices (0-15) so colors adapt to the terminal's
// color scheme. The basic 16 ANSI colors are defined by the terminal
// theme, so dark themes, light themes, and custom palettes all work.
var (
	ColorGreen   = lipgloss.Color("10") // bright green
	ColorYellow  = lipgloss.Color("11") // bright yellow
	ColorRed     = lipgloss.Color("9")  // bright red
	ColorBlue    = lipgloss.Color("12") // bright blue
	ColorCyan    = lipgloss.Color("14") // bright cyan
	ColorMagenta = lipgloss.Color("13") // bright magenta
	ColorDim     = lipgloss.Color("8")  // bright black (gray)
	ColorWhite   = lipgloss.Color("15") // bright white
	ColorMauve   = lipgloss.Color("4")  // bright mauve

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorMauve)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorDim).
			Padding(0, 1)

	SelectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorWhite)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBlue).
			Underline(true)

	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorDim)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorRed).
			Bold(true)
)

func HealthStyle(status string) lipgloss.Style {
	switch status {
	case "healthy", "HEALTHY":
		return lipgloss.NewStyle().Foreground(ColorGreen)
	case "deploying", "PENDING", "PROVISIONING", "ACTIVATING":
		return lipgloss.NewStyle().Foreground(ColorYellow)
	case "degraded":
		return lipgloss.NewStyle().Foreground(ColorYellow)
	case "unhealthy", "STOPPED", "DEACTIVATING", "STOPPING":
		return lipgloss.NewStyle().Foreground(ColorRed)
	default:
		return lipgloss.NewStyle().Foreground(ColorWhite)
	}
}

func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "ACTIVE", "RUNNING":
		return lipgloss.NewStyle().Foreground(ColorGreen)
	case "DRAINING", "DEPROVISIONING":
		return lipgloss.NewStyle().Foreground(ColorYellow)
	case "INACTIVE", "STOPPED":
		return lipgloss.NewStyle().Foreground(ColorRed)
	default:
		return lipgloss.NewStyle().Foreground(ColorWhite)
	}
}
