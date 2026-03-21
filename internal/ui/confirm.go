package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type ConfirmAction int

const (
	ConfirmNone ConfirmAction = iota
	ConfirmForceDeploy
	ConfirmStopTask
	ConfirmSSMUpdate
	ConfirmSMUpdate
)

type ConfirmModel struct {
	Active  bool
	Action  ConfirmAction
	Message string
}

type ConfirmResultMsg struct {
	Action    ConfirmAction
	Confirmed bool
}

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.Active = false
			return m, func() tea.Msg {
				return ConfirmResultMsg{Action: m.Action, Confirmed: true}
			}
		case "n", "N", "esc":
			m.Active = false
			return m, func() tea.Msg {
				return ConfirmResultMsg{Action: m.Action, Confirmed: false}
			}
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	if !m.Active {
		return ""
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorYellow).
		Padding(1, 3).
		Render(fmt.Sprintf("%s\n\n[y] Yes  [n] No", m.Message))

	return "\n" + box + "\n"
}

func NewConfirm(action ConfirmAction, message string) ConfirmModel {
	return ConfirmModel{
		Active:  true,
		Action:  action,
		Message: message,
	}
}
