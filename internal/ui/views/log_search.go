package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/theme"
)

// LogSearchResultsMsg is sent when search results arrive.
type LogSearchResultsMsg struct {
	Results []aws.LogEntry
	Err     error
}

// LogSearchJumpMsg is sent when the user selects a search result to jump to.
type LogSearchJumpMsg struct {
	LogGroup    string
	Stream      string
	Timestamp   int64
	Pattern     string
}

type LogSearchModel struct {
	logGroup string
	stream   string // optional
	pattern  string
	results  []aws.LogEntry
	cursor   int
	scroll   int
	tsMode   int // 0=relative, 1=absolute
	loading  bool
	err      error
	width    int
	height   int
}

func NewLogSearch(logGroup, stream, pattern string) LogSearchModel {
	return LogSearchModel{
		logGroup: logGroup,
		stream:   stream,
		pattern:  pattern,
		loading:  true,
	}
}

func (m LogSearchModel) Update(msg tea.Msg) (LogSearchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case LogSearchResultsMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.results = msg.Results
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Enter):
			if len(m.results) > 0 && m.cursor < len(m.results) {
				entry := m.results[m.cursor]
				stream := m.stream
				if stream == "" {
					stream = entry.Stream
				}
				return m, func() tea.Msg {
					return LogSearchJumpMsg{
						LogGroup:  m.logGroup,
						Stream:    stream,
						Timestamp: entry.Timestamp,
						Pattern:   m.pattern,
					}
				}
			}
		case key.Matches(msg, theme.Keys.Up):
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			}
		case key.Matches(msg, theme.Keys.Down):
			if m.cursor < len(m.results)-1 {
				m.cursor++
				m.adjustScroll()
			}
		case msg.String() == "t", msg.String() == "T":
			m.tsMode = (m.tsMode + 1) % 2
		case msg.String() == "g":
			m.cursor = 0
			m.scroll = 0
		case msg.String() == "G":
			if len(m.results) > 0 {
				m.cursor = len(m.results) - 1
				m.adjustScroll()
			}
		}
	}
	return m, nil
}

func (m LogSearchModel) View() string {
	var b strings.Builder

	title := fmt.Sprintf("  Search: %q", m.pattern)
	b.WriteString(theme.TitleStyle.Render(title))

	scope := m.logGroup
	if m.stream != "" {
		scope += " / " + m.stream
	}
	b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("  in %s", scope)))
	b.WriteString(fmt.Sprintf("  [%d results]", len(m.results)))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(theme.HelpStyle.Render("  Searching..."))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(theme.ErrorStyle.Render(fmt.Sprintf("  Error: %v", m.err)))
		return b.String()
	}

	if len(m.results) == 0 {
		b.WriteString(theme.HelpStyle.Render("  No results found"))
		return b.String()
	}

	visible := m.visibleLines()
	start := m.scroll
	end := start + visible
	if end > len(m.results) {
		end = len(m.results)
	}

	for i := start; i < end; i++ {
		entry := m.results[i]
		ts := m.formatTimestamp(entry.Timestamp)
		tsStr := theme.HelpStyle.Render(ts)

		cursor := "  "
		if i == m.cursor {
			cursor = "► "
		}

		msg := strings.TrimRight(entry.Message, "\n")

		// Highlight the search pattern in the message
		highlighted := highlightPattern(msg, m.pattern)

		streamLabel := ""
		if entry.Stream != "" && m.stream == "" {
			// Show stream name when viewing group-level results
			short := entry.Stream
			if len(short) > 30 {
				short = "..." + short[len(short)-27:]
			}
			streamLabel = theme.HelpStyle.Render(fmt.Sprintf("[%s] ", short))
		}

		b.WriteString(fmt.Sprintf("%s%s  %s%s\n", cursor, tsStr, streamLabel, highlighted))
	}

	if len(m.results) > visible {
		info := fmt.Sprintf("\n  %d–%d of %d", start+1, end, len(m.results))
		if start > 0 {
			info += " ↑"
		}
		if end < len(m.results) {
			info += " ↓"
		}
		b.WriteString(theme.HelpStyle.Render(info))
	}

	return b.String()
}

func (m *LogSearchModel) adjustScroll() {
	visible := m.visibleLines()
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+visible {
		m.scroll = m.cursor - visible + 1
	}
}

func (m LogSearchModel) visibleLines() int {
	h := m.height - 6
	if h < 5 {
		return 20
	}
	return h
}

func (m LogSearchModel) formatTimestamp(ts int64) string {
	t := time.UnixMilli(ts)
	switch m.tsMode {
	case 1:
		return t.Format("2006-01-02 15:04:05.000")
	default:
		d := time.Since(t)
		if d < time.Second {
			return "now        "
		}
		if d < time.Minute {
			return fmt.Sprintf("%-11s", fmt.Sprintf("%ds ago", int(d.Seconds())))
		}
		if d < time.Hour {
			return fmt.Sprintf("%-11s", fmt.Sprintf("%dm%ds ago", int(d.Minutes()), int(d.Seconds())%60))
		}
		if d < 24*time.Hour {
			return fmt.Sprintf("%-11s", fmt.Sprintf("%dh%dm ago", int(d.Hours()), int(d.Minutes())%60))
		}
		return t.Format("Jan 02 15:04")
	}
}

func highlightPattern(msg, pattern string) string {
	if pattern == "" {
		return msg
	}
	lower := strings.ToLower(msg)
	lowerPat := strings.ToLower(pattern)

	idx := strings.Index(lower, lowerPat)
	if idx == -1 {
		return msg
	}

	// Highlight the matched portion
	before := msg[:idx]
	match := msg[idx : idx+len(pattern)]
	after := msg[idx+len(pattern):]

	highlighted := theme.ErrorStyle.Render(match) // red+bold for visibility
	return before + highlighted + after
}

func (m LogSearchModel) SetSize(w, h int) LogSearchModel {
	m.width = w
	m.height = h
	return m
}
