package ui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type InputAction int

const (
	InputNone InputAction = iota
	InputScale
	InputSSMPath
	InputSSMSaveName
	InputExecCommand
	InputLogGroupPrefix
	InputLogSearchPattern
	InputLogSaveName
	InputSSMEditValue
	InputLogSaveFile
	InputSMFilter
	InputSMSaveName
	InputSMEditValue
	InputS3Search
	InputS3SaveName
	InputS3Download
	InputLambdaSearch
	InputLambdaSaveName
	InputDynamoSearch
	InputDynamoSaveName
	InputDynamoFilterAttr
	InputDynamoFilterValue
	InputDynamoPartiQL
	InputDynamoQuerySaveName
	InputS3KeySearch
	InputSMCloneName
	InputSQSSearch
	InputSQSSaveName
	InputLogSearchFrom
	InputLogSearchTo
	InputLogSearchGroupsSave
	InputTofuDir
	InputTofuSaveName
)

type InputModel struct {
	Active  bool
	Action  InputAction
	Label   string
	input   textinput.Model
}

type InputResultMsg struct {
	Action   InputAction
	Value    string
	Canceled bool
}

func NewInput(action InputAction, label, defaultValue string) InputModel {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 40
	if defaultValue != "" {
		ti.SetValue(defaultValue)
		ti.CursorEnd()
	}

	return InputModel{
		Active: true,
		Action: action,
		Label:  label,
		input:  ti,
	}
}

func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.Active = false
			return m, func() tea.Msg {
				return InputResultMsg{Action: m.Action, Value: m.input.Value()}
			}
		case "esc":
			m.Active = false
			return m, func() tea.Msg {
				return InputResultMsg{Action: m.Action, Canceled: true}
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m InputModel) View() string {
	if !m.Active {
		return ""
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorCyan).
		Padding(1, 3).
		Render(fmt.Sprintf("%s\n\n%s\n\n[enter] confirm  [esc] cancel", m.Label, m.input.View()))

	return box
}

func ParseScaleInput(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", value)
	}
	if n < 0 {
		return 0, fmt.Errorf("count must be >= 0")
	}
	return n, nil
}
