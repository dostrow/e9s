package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPasswordInputMasksDatabasePassword(t *testing.T) {
	input := NewPasswordInput(InputSQLPassword, "Database password")
	var cmd tea.Cmd
	input, cmd = input.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("not-secret")})
	_ = cmd
	view := input.View()
	if strings.Contains(view, "not-secret") {
		t.Fatal("password input rendered its cleartext value")
	}
}
