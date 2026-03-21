package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

var AWSRegions = []string{
	"us-east-1",
	"us-east-2",
	"us-west-1",
	"us-west-2",
	"eu-west-1",
	"eu-west-2",
	"eu-west-3",
	"eu-central-1",
	"eu-north-1",
	"ap-northeast-1",
	"ap-northeast-2",
	"ap-southeast-1",
	"ap-southeast-2",
	"ap-south-1",
	"sa-east-1",
	"ca-central-1",
	"me-south-1",
	"af-south-1",
}

type RegionSwitchMsg struct {
	Region string
}

type RegionPickerModel struct {
	Active        bool
	regions       []string
	cursor        int
	currentRegion string
}

func NewRegionPicker(currentRegion string) RegionPickerModel {
	cursor := 0
	for i, r := range AWSRegions {
		if r == currentRegion {
			cursor = i
			break
		}
	}
	return RegionPickerModel{
		Active:        true,
		regions:       AWSRegions,
		cursor:        cursor,
		currentRegion: currentRegion,
	}
}

func (m RegionPickerModel) Update(msg tea.Msg) (RegionPickerModel, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			if m.cursor < len(m.regions)-1 {
				m.cursor++
			}
		case key.Matches(msg, theme.Keys.Enter):
			m.Active = false
			region := m.regions[m.cursor]
			return m, func() tea.Msg {
				return RegionSwitchMsg{Region: region}
			}
		case key.Matches(msg, theme.Keys.Back), msg.String() == "q":
			m.Active = false
			return m, nil
		}
	}
	return m, nil
}

func (m RegionPickerModel) View() string {
	if !m.Active {
		return ""
	}

	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render("  Switch Region"))
	b.WriteString("\n\n")

	for i, r := range m.regions {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "► "
			style = theme.SelectedRowStyle
		}

		indicator := "  "
		if r == m.currentRegion {
			indicator = "* "
		}

		b.WriteString(style.Render(fmt.Sprintf("%s%s%s", cursor, indicator, r)))
		b.WriteString("\n")
	}

	return b.String()
}
