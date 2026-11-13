package ui

import (
	"fmt"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/command"
	"github.com/dostrow/e9s/internal/runbook"
	"github.com/dostrow/e9s/internal/ui/views"
)

// savedItems lists saved bookmarks (W) from enabled modes. Each opens
// through the same picker result its mode's entry picker emits, so the
// palette and ctrl+p share one code path.
func (a App) savedItems() []paletteItem {
	if a.cfg == nil {
		return nil
	}
	enabled := a.enabledModes()
	var items []paletteItem
	add := func(mode topMode, kind, title, target string, run func(App) (App, tea.Cmd)) *paletteItem {
		if !enabled[mode] {
			return nil
		}
		items = append(items, paletteItem{
			Entry: command.Entry{
				Command: command.Command{
					ID:       "saved." + kind + "." + paletteSlug(title),
					Title:    title,
					Category: modeShortName(mode) + " · Saved " + strings.ReplaceAll(kind, "-", " "),
					Keywords: []string{"saved", "bookmark", modeShortName(mode), modeDisplayName(mode)},
				},
				Target:  target,
				Section: command.SectionSaved,
			},
			run: run,
		})
		return &items[len(items)-1]
	}
	pick := func(mode topMode, action PickerAction, index int) func(App) (App, tea.Cmd) {
		return func(a App) (App, tea.Cmd) {
			a.mode = mode
			a.err = nil
			model, cmd := a.Update(PickerResultMsg{Action: action, Index: index})
			return model.(App), cmd
		}
	}

	for i, p := range a.cfg.LogPaths {
		target := p.LogGroup
		if len(p.LogGroups) > 1 {
			target = fmt.Sprintf("%d groups", len(p.LogGroups))
		}
		add(modeCWLogs, "log-paths", p.Name, target, pick(modeCWLogs, PickerLogPath, i))
	}
	for i, p := range a.cfg.SSMPrefixes {
		add(modeSSM, "prefixes", p.Name, p.Prefix, pick(modeSSM, PickerSSMPrefix, i))
	}
	for i, f := range a.cfg.SMFilters {
		add(modeSM, "filters", f.Name, f.Filter, pick(modeSM, PickerSMFilter, i))
	}
	for i, s := range a.cfg.S3Searches {
		add(modeS3, "bucket-searches", s.Name, s.Filter, pick(modeS3, PickerS3Search, i))
	}
	for i, s := range a.cfg.LambdaSearches {
		add(modeLambda, "function-searches", s.Name, s.Filter, pick(modeLambda, PickerLambdaSearch, i))
	}
	for i, q := range a.cfg.SQSQueues {
		add(modeSQS, "queues", q.Name, "", pick(modeSQS, PickerSQSQueue, i))
	}
	for i, t := range a.cfg.DynamoTables {
		add(modeDynamoDB, "tables", t.Name, t.Table, pick(modeDynamoDB, PickerDynamoTable, i))
	}
	for i, q := range a.cfg.DynamoQueries {
		item := add(modeDynamoDB, "queries", q.Name, truncateRunes(q.Statement, 40), pick(modeDynamoDB, PickerDynamoQuery, i))
		// Saved PartiQL results render into the open table's item list.
		if item != nil && a.state != viewDynamoItems {
			item.Disabled = "Available in " + viewCategory(viewDynamoItems)
		}
	}
	for i, d := range a.cfg.TofuDirs {
		add(modeTofu, "workspaces", d.Name, d.Dir, pick(modeTofu, PickerTofuDir, i))
	}
	for i, v := range a.cfg.CostViews {
		index := i
		add(modeCostExplorer, "cost-views", v.Name, "", func(a App) (App, tea.Cmd) {
			a.mode = modeCostExplorer
			a.err = nil
			return a.selectCostView(4 + index)
		})
	}
	return items
}

// pluginItems lists every configured plugin action.
func (a App) pluginItems() []paletteItem {
	if !a.enabledModes()[modePlugins] {
		return nil
	}
	items := make([]paletteItem, 0, len(a.runbookActions))
	for _, configured := range a.runbookActions {
		action := configured
		keywords := []string{"plugin", "runbook", action.PluginName, action.PluginDisplayName}
		if action.Action.Description != "" {
			keywords = append(keywords, action.Action.Description)
		}
		items = append(items, paletteItem{
			Entry: command.Entry{
				Command: command.Command{
					ID:       "plugin." + paletteSlug(action.Key()),
					Title:    action.Action.Title,
					Category: "Plugins · " + action.PluginDisplayName,
					Keywords: keywords,
					Danger:   action.Action.Risk == "destructive" || action.Action.Risk == "production",
				},
				Section: command.SectionPlugins,
			},
			run: func(a App) (App, tea.Cmd) {
				a.mode = modePlugins
				a.state = viewPlugins
				a.err = nil
				a.runbookForm = NewRunbookForm(action, runbook.Context{Region: a.region(), Profile: a.awsProfile}).SetHeight(a.height - 6)
				return a, nil
			},
		})
	}
	return items
}

// regionChildren lists AWS regions for the nested "Switch AWS region" step.
func (a App) regionChildren() []paletteItem {
	current := a.region()
	items := make([]paletteItem, 0, len(views.AWSRegions))
	for _, r := range views.AWSRegions {
		region := r
		item := paletteItem{
			Entry: command.Entry{
				Command: command.Command{ID: "region." + region, Title: region, Category: "Region", Keywords: []string{"region"}},
				Section: command.SectionGoTo,
			},
			run: func(a App) (App, tea.Cmd) { return a, a.switchRegion(region) },
		}
		if region == current {
			item.Target = "current"
		}
		items = append(items, item)
	}
	return items
}

func (a App) region() string {
	if a.client == nil {
		return ""
	}
	return a.client.Region()
}

func (a App) enabledModes() map[topMode]bool {
	enabled := make(map[topMode]bool, len(a.modeTabs))
	for _, tab := range a.modeTabs {
		enabled[tab.Mode] = true
	}
	return enabled
}

// paletteSlug normalizes a name into a stable ID fragment.
func paletteSlug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
