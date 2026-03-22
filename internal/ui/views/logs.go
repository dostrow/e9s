package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/theme"
)

const maxLogLines = 1000

// LogsLoadedMsg is sent when new log entries arrive.
type LogsLoadedMsg struct {
	Entries []aws.LogEntry
	LastTS  int64
}

// LogsErrorMsg is sent on log fetch errors.
type LogsErrorMsg struct{ Err error }

// LogsPrependedMsg is sent when older logs are loaded (backward fetch).
type LogsPrependedMsg struct {
	Entries []aws.LogEntry
}

type LogViewerModel struct {
	title    string
	client   *aws.Client
	logGroup string
	streams  []string

	lines    []logLine
	scroll   int
	follow   bool // auto-scroll to bottom
	tailMode bool // true if opened for live tailing; false for historical/jump
	tsMode   int  // 0=relative, 1=absolute local, 2=absolute UTC

	// Search
	search       string // current search pattern
	searchIdx    int    // index into matchIndices of the current match
	matchIndices []int  // indices into lines[] that match the search
	searching    bool
	searchInput  textinput.Model

	jumpTargetTS  int64 // if > 0, scroll to nearest line after first load
	initialLoaded bool  // whether first batch has loaded

	firstTS int64 // earliest timestamp in buffer (for backward fetch)
	lastTS  int64
	width  int
	height int
}

type logLine struct {
	timestamp int64
	stream    string
	message   string
}

func NewLogViewer(title string, client *aws.Client, logGroup string, streams []string) LogViewerModel {
	return NewLogViewerWithOptions(title, client, logGroup, streams, true, 5*time.Minute)
}

func NewLogViewerWithOptions(title string, client *aws.Client, logGroup string, streams []string, follow bool, lookback time.Duration) LogViewerModel {
	startTS := time.Now().Add(-lookback).UnixMilli()

	return LogViewerModel{
		title:    title,
		client:   client,
		logGroup: logGroup,
		streams:  streams,
		follow:   follow,
		tailMode: follow,
		lastTS:   startTS,
	}
}

func NewLogViewerWithSearch(title string, client *aws.Client, logGroup string, streams []string, follow bool, lookback time.Duration, search string) LogViewerModel {
	m := NewLogViewerWithOptions(title, client, logGroup, streams, follow, lookback)
	m.search = search
	return m
}

// NewLogViewerAtTimestamp creates a log viewer starting at an absolute timestamp.
// Used for jump-from-search: loads a window around the timestamp, paused, with search highlighted.
func NewLogViewerAtTimestamp(title string, client *aws.Client, logGroup string, streams []string, timestampMs int64, search string) LogViewerModel {
	// Start 0.5 seconds before the match so the result is immediately visible
	startTS := timestampMs - 5*100

	return LogViewerModel{
		title:        title,
		client:       client,
		logGroup:     logGroup,
		streams:      streams,
		follow:       false,
		lastTS:       startTS,
		search:       search,
		jumpTargetTS: timestampMs,
	}
}

func (m LogViewerModel) Init() tea.Cmd {
	return m.fetchLogs()
}

func (m LogViewerModel) Update(msg tea.Msg) (LogViewerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case LogsLoadedMsg:
		for _, e := range msg.Entries {
			m.lines = append(m.lines, logLine{
				timestamp: e.Timestamp,
				stream:    e.Stream,
				message:   strings.TrimRight(e.Message, "\n"),
			})
		}
		if len(m.lines) > maxLogLines {
			trimmed := len(m.lines) - maxLogLines
			m.lines = m.lines[trimmed:]
			// Adjust scroll position so viewport doesn't drift
			if !m.follow {
				m.scroll -= trimmed
				m.scroll = max(0, m.scroll)
			}
			m.rebuildMatchIndices()
		}
		// Track earliest timestamp in buffer
		if len(m.lines) > 0 && (m.firstTS == 0 || m.lines[0].timestamp < m.firstTS) {
			m.firstTS = m.lines[0].timestamp
		}
		if msg.LastTS > m.lastTS {
			m.lastTS = msg.LastTS + 1
		}
		// Update match indices for new lines
		if m.search != "" {
			lowerSearch := strings.ToLower(m.search)
			startIdx := len(m.lines) - len(msg.Entries)
			startIdx = max(0, startIdx)
			for i := startIdx; i < len(m.lines); i++ {
				if strings.Contains(strings.ToLower(m.lines[i].message), lowerSearch) {
					m.matchIndices = append(m.matchIndices, i)
				}
			}
		}
		// On first load with a jump target, scroll to the target timestamp
		if m.jumpTargetTS > 0 && !m.initialLoaded {
			m.initialLoaded = true
			m.scrollToTimestamp(m.jumpTargetTS)
			// If we have a search, also set the searchIdx to the nearest match
			if len(m.matchIndices) > 0 {
				m.searchIdx = 0
				for i, idx := range m.matchIndices {
					if m.lines[idx].timestamp >= m.jumpTargetTS {
						m.searchIdx = i
						break
					}
				}
			}
			return m, nil // don't fetch more — we have our window
		}
		if m.follow {
			m.scrollToBottom()
			return m, m.scheduleRefresh()
		}
		return m, nil

	case LogsPrependedMsg:
		if len(msg.Entries) == 0 {
			return m, nil
		}
		var older []logLine
		for _, e := range msg.Entries {
			older = append(older, logLine{
				timestamp: e.Timestamp,
				stream:    e.Stream,
				message:   strings.TrimRight(e.Message, "\n"),
			})
		}
		// Prepend and adjust scroll so viewport stays on the same content
		m.lines = append(older, m.lines...)
		m.scroll += len(older)
		// Update firstTS
		if len(m.lines) > 0 {
			m.firstTS = m.lines[0].timestamp
		}
		// Cap buffer from the end if needed
		if len(m.lines) > maxLogLines {
			excess := len(m.lines) - maxLogLines
			m.lines = m.lines[:maxLogLines]
			_ = excess // trimmed from end, no scroll adjust needed
		}
		m.rebuildMatchIndices()
		return m, nil

	case LogsErrorMsg:
		if m.follow {
			return m, m.scheduleRefresh()
		}
		return m, nil

	case LogTickMsg:
		if m.follow {
			return m, m.fetchLogs()
		}
		return m, nil

	case tea.KeyMsg:
		if m.searching {
			return m.handleSearchInput(msg)
		}
		switch {
		case key.Matches(msg, theme.Keys.Up):
			m.follow = false
			if m.scroll > 0 {
				m.scroll--
			}
		case key.Matches(msg, theme.Keys.Down):
			visible := m.visibleLines()
			maxScroll := len(m.lines) - visible
			maxScroll = max(0, maxScroll)
			m.scroll++
			if m.scroll >= maxScroll {
				m.scroll = maxScroll
				if m.tailMode && !m.follow {
					m.follow = true
					return m, m.fetchLogs()
				}
			}
		case msg.String() == "pgup":
			m.follow = false
			m.scroll -= m.visibleLines()
			m.scroll = max(0, m.scroll)
		case msg.String() == "pgdown":
			visible := m.visibleLines()
			maxScroll := len(m.lines) - visible
			maxScroll = max(0, maxScroll)
			m.scroll += visible
			if m.scroll >= maxScroll {
				m.scroll = maxScroll
				if m.tailMode && !m.follow {
					m.follow = true
					return m, m.fetchLogs()
				}
			}
		case msg.String() == "f", msg.String() == "F":
			m.follow = !m.follow
			if m.follow {
				m.tailMode = true // explicit toggle promotes to tail mode
				m.scrollToBottom()
				return m, m.fetchLogs()
			}
		case msg.String() == "t", msg.String() == "T":
			m.tsMode = (m.tsMode + 1) % 3
		case msg.String() == "/":
			m.searching = true
			m.searchInput = textinput.New()
			m.searchInput.Placeholder = "search..."
			m.searchInput.SetValue(m.search)
			m.searchInput.Focus()
			m.searchInput.CharLimit = 100
			m.searchInput.Width = 40
			return m, m.searchInput.Focus()
		case msg.String() == "n":
			m.jumpToNextMatch()
		case msg.String() == "N":
			m.jumpToPrevMatch()
		case msg.String() == "[":
			return m, m.fetchOlderLogs()
		case msg.String() == "]":
			return m, m.fetchNewerLogs()
		case msg.String() == "g":
			m.scroll = 0
			m.follow = false
		case msg.String() == "G":
			m.scrollToBottom()
			if m.tailMode && !m.follow {
				m.follow = true
				return m, m.fetchLogs()
			}
		case key.Matches(msg, theme.Keys.Back):
			if m.search != "" {
				// Esc clears search first, second Esc goes back
				m.search = ""
				m.matchIndices = nil
				m.searchIdx = 0
				return m, nil
			}
		}
	}

	return m, nil
}

func (m LogViewerModel) handleSearchInput(msg tea.KeyMsg) (LogViewerModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.search = m.searchInput.Value()
		m.searching = false
		m.rebuildMatchIndices()
		m.follow = false
		// Jump to first match at or after current scroll position
		if len(m.matchIndices) > 0 {
			m.searchIdx = 0
			// Find the first match visible from current scroll
			for i, idx := range m.matchIndices {
				if idx >= m.scroll {
					m.searchIdx = i
					break
				}
			}
			m.scrollToMatch(m.searchIdx)
		}
		return m, nil
	case "esc":
		m.searching = false
		return m, nil
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m *LogViewerModel) rebuildMatchIndices() {
	m.matchIndices = nil
	if m.search == "" {
		return
	}
	lowerSearch := strings.ToLower(m.search)
	for i, l := range m.lines {
		if strings.Contains(strings.ToLower(l.message), lowerSearch) {
			m.matchIndices = append(m.matchIndices, i)
		}
	}
}

func (m *LogViewerModel) jumpToNextMatch() {
	if len(m.matchIndices) == 0 {
		return
	}
	m.follow = false
	m.searchIdx++
	if m.searchIdx >= len(m.matchIndices) {
		m.searchIdx = 0 // wrap
	}
	m.scrollToMatch(m.searchIdx)
}

func (m *LogViewerModel) jumpToPrevMatch() {
	if len(m.matchIndices) == 0 {
		return
	}
	m.follow = false
	m.searchIdx--
	if m.searchIdx < 0 {
		m.searchIdx = len(m.matchIndices) - 1 // wrap
	}
	m.scrollToMatch(m.searchIdx)
}

func (m *LogViewerModel) scrollToTimestamp(ts int64) {
	// Find the first line at or after the target timestamp
	targetLine := len(m.lines) - 1
	for i, l := range m.lines {
		if l.timestamp >= ts {
			targetLine = i
			break
		}
	}
	visible := m.visibleLines()
	// Center the target in the viewport
	m.scroll = targetLine - visible/2
	m.scroll = max(0, m.scroll)
	maxScroll := len(m.lines) - visible
	maxScroll = max(0, maxScroll)
	m.scroll = min(m.scroll, maxScroll)
}

func (m *LogViewerModel) scrollToMatch(matchIdx int) {
	if matchIdx < 0 || matchIdx >= len(m.matchIndices) {
		return
	}
	lineIdx := m.matchIndices[matchIdx]
	visible := m.visibleLines()
	// Center the match in the viewport
	m.scroll = lineIdx - visible/2
	m.scroll = max(0, m.scroll)
	maxScroll := len(m.lines) - visible
	maxScroll = max(0, maxScroll)
	m.scroll = min(m.scroll, maxScroll)
}

func (m LogViewerModel) View() string {
	var b strings.Builder

	// Title line
	titleStr := fmt.Sprintf("  Logs: %s", m.title)
	b.WriteString(theme.TitleStyle.Render(titleStr))

	followIndicator := theme.HelpStyle.Render(" (paused)")
	if m.follow {
		followIndicator = theme.HealthStyle("healthy").Render(" (following)")
	}
	b.WriteString(followIndicator)

	b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("  [%s]", m.tsLabel())))

	if m.search != "" {
		matchInfo := fmt.Sprintf("  search: %q", m.search)
		if len(m.matchIndices) > 0 {
			matchInfo += fmt.Sprintf(" [%d/%d]", m.searchIdx+1, len(m.matchIndices))
		} else {
			matchInfo += " [no matches]"
		}
		b.WriteString(theme.HelpStyle.Render(matchInfo))
	}
	fmt.Fprintf(&b, "  [%d lines]", len(m.lines))
	b.WriteString("\n\n")

	if m.searching {
		b.WriteString("  / " + m.searchInput.View() + "\n\n")
	}

	visible := m.visibleLines()

	start := m.scroll
	if start > len(m.lines)-visible {
		start = len(m.lines) - visible
	}
	start = max(0, start)
	end := start + visible
	if end > len(m.lines) {
		end = len(m.lines)
	}

	if len(m.lines) == 0 {
		b.WriteString(theme.HelpStyle.Render("  Waiting for logs..."))
	}

	// Build a set of match line indices for quick lookup
	matchSet := map[int]bool{}
	currentMatchLine := -1
	for i, idx := range m.matchIndices {
		matchSet[idx] = true
		if i == m.searchIdx {
			currentMatchLine = idx
		}
	}

	indent := "  " + strings.Repeat(" ", tsWidth) + "  "

	for i := start; i < end; i++ {
		line := m.lines[i]
		ts := m.formatTimestamp(line.timestamp)
		tsStr := theme.HelpStyle.Render(ts)

		msgLines := strings.Split(line.message, "\n")
		firstLine := msgLines[0]

		// Highlight search matches on all lines
		if m.search != "" {
			firstLine = highlightSearch(firstLine, m.search)
		}

		// Mark the current match with a marker
		marker := "  "
		if i == currentMatchLine {
			marker = "» "
		}

		fmt.Fprintf(&b, "%s%s  %s\n", marker, tsStr, firstLine)
		for _, cont := range msgLines[1:] {
			if m.search != "" {
				cont = highlightSearch(cont, m.search)
			}
			b.WriteString(indent + cont + "\n")
		}
	}

	return b.String()
}

func highlightSearch(msg, pattern string) string {
	if pattern == "" {
		return msg
	}
	lower := strings.ToLower(msg)
	lowerPat := strings.ToLower(pattern)

	var result strings.Builder
	pos := 0
	for {
		idx := strings.Index(lower[pos:], lowerPat)
		if idx == -1 {
			result.WriteString(msg[pos:])
			break
		}
		result.WriteString(msg[pos : pos+idx])
		matchEnd := pos + idx + len(pattern)
		result.WriteString(theme.ErrorStyle.Render(msg[pos+idx : matchEnd]))
		pos = matchEnd
	}
	return result.String()
}

func (m LogViewerModel) visibleLines() int {
	h := m.height - 5
	if m.searching {
		h -= 2
	}
	if h < 1 {
		h = 20
	}
	return h
}

func (m *LogViewerModel) scrollToBottom() {
	visible := m.visibleLines()
	m.scroll = len(m.lines) - visible
	m.scroll = max(0, m.scroll)
}

const tsWidth = 23

func (m LogViewerModel) formatTimestamp(ts int64) string {
	t := time.UnixMilli(ts)
	switch m.tsMode {
	case 1:
		return t.Local().Format("2006-01-02 15:04:05.000")
	case 2:
		return t.UTC().Format("2006-01-02 15:04:05.000") + "Z"
	default:
		d := time.Since(t)
		var rel string
		switch {
		case d < time.Second:
			rel = "now"
		case d < time.Minute:
			rel = fmt.Sprintf("%ds ago", int(d.Seconds()))
		case d < time.Hour:
			rel = fmt.Sprintf("%dm%ds ago", int(d.Minutes()), int(d.Seconds())%60)
		default:
			rel = fmt.Sprintf("%dh%dm ago", int(d.Hours()), int(d.Minutes())%60)
		}
		return fmt.Sprintf("%-*s", tsWidth, rel)
	}
}

func (m LogViewerModel) tsLabel() string {
	switch m.tsMode {
	case 1:
		return "local"
	case 2:
		return "UTC"
	default:
		return "relative"
	}
}

type LogTickMsg struct{}

func (m LogViewerModel) scheduleRefresh() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return LogTickMsg{}
	})
}

func (m LogViewerModel) fetchLogs() tea.Cmd {
	client := m.client
	logGroup := m.logGroup
	streams := m.streams
	startTime := m.lastTS

	return func() tea.Msg {
		var entries []aws.LogEntry
		var lastTS int64
		var err error

		if len(streams) == 1 {
			entries, lastTS, err = client.FetchLogs(
				context.Background(), logGroup, streams[0], startTime, 100)
		} else if len(streams) > 1 {
			entries, lastTS, err = client.FetchMultiStreamLogs(
				context.Background(), logGroup, streams, startTime, 100)
		} else {
			entries, lastTS, err = client.FetchLogGroup(
				context.Background(), logGroup, startTime, 100)
		}

		if err != nil {
			return LogsErrorMsg{Err: err}
		}
		return LogsLoadedMsg{Entries: entries, LastTS: lastTS}
	}
}

func (m LogViewerModel) fetchOlderLogs() tea.Cmd {
	client := m.client
	logGroup := m.logGroup
	streams := m.streams
	// Fetch 30 seconds before the earliest line
	endTime := m.firstTS
	startTime := endTime - 30*1000
	startTime = max(0, startTime)

	return func() tea.Msg {
		var entries []aws.LogEntry
		var err error

		if len(streams) == 1 {
			entries, _, err = client.FetchLogs(
				context.Background(), logGroup, streams[0], startTime, 100)
		} else if len(streams) > 1 {
			entries, _, err = client.FetchMultiStreamLogs(
				context.Background(), logGroup, streams, startTime, 100)
		} else {
			entries, _, err = client.FetchLogGroup(
				context.Background(), logGroup, startTime, 100)
		}

		if err != nil {
			return LogsErrorMsg{Err: err}
		}
		// Filter to only entries before our current earliest
		var older []aws.LogEntry
		for _, e := range entries {
			if e.Timestamp < endTime {
				older = append(older, e)
			}
		}
		return LogsPrependedMsg{Entries: older}
	}
}

func (m LogViewerModel) fetchNewerLogs() tea.Cmd {
	client := m.client
	logGroup := m.logGroup
	streams := m.streams
	startTime := m.lastTS

	return func() tea.Msg {
		var entries []aws.LogEntry
		var lastTS int64
		var err error

		if len(streams) == 1 {
			entries, lastTS, err = client.FetchLogs(
				context.Background(), logGroup, streams[0], startTime, 100)
		} else if len(streams) > 1 {
			entries, lastTS, err = client.FetchMultiStreamLogs(
				context.Background(), logGroup, streams, startTime, 100)
		} else {
			entries, lastTS, err = client.FetchLogGroup(
				context.Background(), logGroup, startTime, 100)
		}

		if err != nil {
			return LogsErrorMsg{Err: err}
		}
		return LogsLoadedMsg{Entries: entries, LastTS: lastTS}
	}
}

func (m LogViewerModel) SetSearch(pattern string) LogViewerModel {
	m.search = pattern
	// Match indices will be built as lines arrive
	return m
}

// ExportLines returns all buffered log lines formatted for file output.
func (m LogViewerModel) ExportLines() []string {
	out := make([]string, 0, len(m.lines))
	for _, l := range m.lines {
		ts := time.UnixMilli(l.timestamp).UTC().Format("2006-01-02T15:04:05.000Z")
		stream := l.stream
		if stream != "" {
			out = append(out, fmt.Sprintf("%s  [%s]  %s", ts, stream, l.message))
		} else {
			out = append(out, fmt.Sprintf("%s  %s", ts, l.message))
		}
	}
	return out
}

func (m LogViewerModel) IsFiltering() bool {
	return m.searching
}

func (m LogViewerModel) SetSize(w, h int) LogViewerModel {
	m.width = w
	m.height = h
	return m
}

func (m LogViewerModel) LogGroup() string {
	return m.logGroup
}
