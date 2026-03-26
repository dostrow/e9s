package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type R53RecordDetailModel struct {
	record    *aws.R53Record
	zoneName  string
	zoneID    string
	dnsAnswer *aws.R53DNSAnswer
	scroll    int
	width     int
	height    int
}

func NewR53RecordDetail(record *aws.R53Record, zoneName, zoneID string) R53RecordDetailModel {
	return R53RecordDetailModel{record: record, zoneName: zoneName, zoneID: zoneID}
}

func (m R53RecordDetailModel) Update(msg tea.Msg) (R53RecordDetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up), msg.String() == "k":
			if m.scroll > 0 {
				m.scroll--
			}
		case key.Matches(msg, theme.Keys.Down), msg.String() == "j":
			m.scroll++
		case msg.String() == "pgup":
			m.scroll = max(0, m.scroll-m.visibleRows())
		case msg.String() == "pgdown":
			m.scroll += m.visibleRows()
		case msg.String() == "g":
			m.scroll = 0
		case msg.String() == "G":
			m.scroll = 999
		}
	}
	return m, nil
}

func (m R53RecordDetailModel) View() string {
	if m.record == nil {
		return theme.HelpStyle.Render("  No record selected")
	}

	r := m.record
	var lines []string

	lines = append(lines, theme.TitleStyle.Render(fmt.Sprintf("  Record: %s", r.Name)))
	lines = append(lines, "")

	typeStyle := recordTypeStyle(r.Type)
	lines = append(lines, fmt.Sprintf("  %-22s %s", "Type:", typeStyle.Render(r.Type)))
	if r.TTL > 0 {
		lines = append(lines, fmt.Sprintf("  %-22s %d seconds", "TTL:", r.TTL))
	}
	if r.RoutingPolicy != "Simple" {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Routing:", r.RoutingPolicy))
	}
	if r.SetIdentifier != "" {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Set Identifier:", r.SetIdentifier))
	}
	if r.Weight > 0 {
		lines = append(lines, fmt.Sprintf("  %-22s %d", "Weight:", r.Weight))
	}
	if r.Region != "" {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Region:", r.Region))
	}
	if r.Failover != "" {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Failover:", r.Failover))
	}
	if r.HealthCheckID != "" {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Health Check:", r.HealthCheckID))
	}
	lines = append(lines, "")

	// Values
	if r.AliasTarget != "" {
		lines = append(lines, theme.TitleStyle.Render("  Alias Target"))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  %-22s %s", "DNS Name:", r.AliasTarget))
		if r.AliasZoneID != "" {
			lines = append(lines, fmt.Sprintf("  %-22s %s", "Hosted Zone ID:", r.AliasZoneID))
		}
	} else if len(r.Values) > 0 {
		lines = append(lines, theme.TitleStyle.Render(fmt.Sprintf("  Values (%d)", len(r.Values))))
		lines = append(lines, "")
		for _, v := range r.Values {
			lines = append(lines, fmt.Sprintf("  %s", v))
		}
	}
	lines = append(lines, "")

	// DNS Test Result
	if m.dnsAnswer != nil {
		a := m.dnsAnswer
		lines = append(lines, theme.TitleStyle.Render("  DNS Test Result"))
		lines = append(lines, "")

		codeStyle := lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
		if a.ResponseCode != "NOERROR" {
			codeStyle = lipgloss.NewStyle().Foreground(theme.ColorRed).Bold(true)
		}
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Response:", codeStyle.Render(a.ResponseCode)))
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Nameserver:", a.Nameserver))
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Protocol:", a.Protocol))
		if len(a.RecordData) > 0 {
			lines = append(lines, fmt.Sprintf("  %-22s", "Resolved:"))
			for _, d := range a.RecordData {
				lines = append(lines, fmt.Sprintf("    %s", d))
			}
		}
	}

	// Scrolling
	visRows := m.visibleRows()
	if m.scroll > len(lines)-visRows {
		m.scroll = max(0, len(lines)-visRows)
	}
	end := min(m.scroll+visRows, len(lines))
	visible := lines[m.scroll:end]

	return strings.Join(visible, "\n")
}

func (m R53RecordDetailModel) SetDNSAnswer(answer *aws.R53DNSAnswer) R53RecordDetailModel {
	m.dnsAnswer = answer
	return m
}

func (m R53RecordDetailModel) Record() *aws.R53Record { return m.record }
func (m R53RecordDetailModel) ZoneName() string       { return m.zoneName }
func (m R53RecordDetailModel) ZoneID() string         { return m.zoneID }

func (m R53RecordDetailModel) visibleRows() int {
	rows := m.height - 2
	if rows < 5 {
		return 20
	}
	return rows
}

func (m R53RecordDetailModel) SetSize(w, h int) R53RecordDetailModel {
	m.width = w
	m.height = h
	return m
}
