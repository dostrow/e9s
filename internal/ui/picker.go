package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type PickerAction int

const (
	PickerNone PickerAction = iota
	PickerExecContainer
	PickerLogContainer
	PickerEnvContainer
	PickerSSMPrefix
	PickerLogPath
	PickerLogSearchTimeRange
	PickerSMFilter
	PickerS3Search
	PickerLambdaSearch
	PickerDynamoTable
	PickerDynamoQuery
	PickerDynamoFilterOp
	PickerSQSQueue
	PickerCWAlarmState
	PickerSetAlarmState
)

type PickerModel struct {
	Active    bool
	Action    PickerAction
	Title     string
	Items     []string
	cursor    int
	Deletable int // number of items from the start that are deletable (saved entries)
}

type PickerResultMsg struct {
	Action   PickerAction
	Index    int
	Value    string
	Canceled bool
}

type PickerDeleteMsg struct {
	Action PickerAction
	Index  int
}

func NewPicker(action PickerAction, title string, items []string) PickerModel {
	return PickerModel{
		Active: true,
		Action: action,
		Title:  title,
		Items:  items,
	}
}

func NewPickerWithDelete(action PickerAction, title string, items []string, deletableCount int) PickerModel {
	return PickerModel{
		Active:    true,
		Action:    action,
		Title:     title,
		Items:     items,
		Deletable: deletableCount,
	}
}

func (m PickerModel) Update(msg tea.Msg) (PickerModel, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.Items)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			m.Active = false
			return m, func() tea.Msg {
				return PickerResultMsg{
					Action: m.Action,
					Index:  m.cursor,
					Value:  m.Items[m.cursor],
				}
			}
		case "d", "delete", "backspace":
			if m.Deletable > 0 && m.cursor < m.Deletable {
				idx := m.cursor
				m.Active = false
				return m, func() tea.Msg {
					return PickerDeleteMsg{Action: m.Action, Index: idx}
				}
			}
		case "esc", "q":
			m.Active = false
			return m, func() tea.Msg {
				return PickerResultMsg{Action: m.Action, Canceled: true}
			}
		default:
			if s := msg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
				idx := int(s[0] - '1')
				if idx < len(m.Items) {
					m.Active = false
					return m, func() tea.Msg {
						return PickerResultMsg{
							Action: m.Action,
							Index:  idx,
							Value:  m.Items[idx],
						}
					}
				}
			}
		}
	}
	return m, nil
}

func (m PickerModel) View() string {
	if !m.Active {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.Title + "\n\n")

	for i, item := range m.Items {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "► "
			style = theme.SelectedRowStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%d) %s", cursor, i+1, item)))
		b.WriteString("\n")
	}

	helpLine := "\n[enter] select  [esc] cancel"
	if m.Deletable > 0 {
		helpLine = "\n[enter] select  [d] delete saved  [esc] cancel"
	}
	b.WriteString(helpLine)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorCyan).
		Padding(1, 3).
		Render(b.String())

	return box
}
