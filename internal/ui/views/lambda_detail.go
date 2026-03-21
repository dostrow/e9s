package views

import (
	"fmt"
	"strings"

	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type LambdaDetailModel struct {
	fn     *aws.LambdaFunction
	width  int
	height int
}

func NewLambdaDetail(fn *aws.LambdaFunction) LambdaDetailModel {
	return LambdaDetailModel{fn: fn}
}

func (m LambdaDetailModel) View() string {
	if m.fn == nil {
		return theme.HelpStyle.Render("  No function selected")
	}

	fn := m.fn
	var lines []string

	lines = append(lines, theme.TitleStyle.Render(fmt.Sprintf("  Lambda: %s", fn.Name)))
	lines = append(lines, "")

	stateStyle := theme.StatusStyle(fn.State)
	if fn.State == "Active" {
		stateStyle = theme.HealthStyle("healthy")
	} else if fn.State == "Failed" {
		stateStyle = theme.HealthStyle("unhealthy")
	}

	lines = append(lines, fmt.Sprintf("  %-18s %s", "State:", stateStyle.Render(fn.State)))
	lines = append(lines, fmt.Sprintf("  %-18s %s", "Runtime:", fn.Runtime))
	lines = append(lines, fmt.Sprintf("  %-18s %s", "Handler:", fn.Handler))
	lines = append(lines, fmt.Sprintf("  %-18s %d MB", "Memory:", fn.MemoryMB))
	lines = append(lines, fmt.Sprintf("  %-18s %ds", "Timeout:", fn.TimeoutSec))
	lines = append(lines, fmt.Sprintf("  %-18s %s", "Code Size:", formatBytesLambda(fn.CodeSize)))
	lines = append(lines, fmt.Sprintf("  %-18s %s", "Log Group:", fn.LogGroup))
	if fn.Description != "" {
		lines = append(lines, fmt.Sprintf("  %-18s %s", "Description:", fn.Description))
	}
	if !fn.LastModified.IsZero() {
		lines = append(lines, fmt.Sprintf("  %-18s %s (%s ago)", "Last Modified:", fn.LastModified.Format("2006-01-02 15:04:05"), formatAge(fn.LastModified)))
	}
	lines = append(lines, fmt.Sprintf("  %-18s %s", "ARN:", fn.ARN))

	if len(fn.EnvVars) > 0 {
		lines = append(lines, fmt.Sprintf("  %-18s %d vars (press E to view)", "Env Vars:", len(fn.EnvVars)))
	}

	maxLines := m.height - 2
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	return strings.Join(lines, "\n")
}

func formatBytesLambda(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func (m LambdaDetailModel) Function() *aws.LambdaFunction {
	return m.fn
}

func (m LambdaDetailModel) SetSize(w, h int) LambdaDetailModel {
	m.width = w
	m.height = h
	return m
}
