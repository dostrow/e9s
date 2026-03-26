package tofu

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// PlanResult represents a parsed tofu plan.
type PlanResult struct {
	Changes     []ResourceChange
	CreateCount int
	UpdateCount int
	DeleteCount int
	ReplaceCount int
	NoOpCount   int
}

// ResourceChange represents a single resource change from a plan.
type ResourceChange struct {
	Address    string // e.g. "aws_ecs_service.api"
	Module     string // e.g. "module.vpc"
	Type       string // e.g. "aws_ecs_service"
	Name       string // e.g. "api"
	Action     string // "create", "update", "delete", "replace", "read", "no-op"
	Before     map[string]any
	After      map[string]any
	Diffs      []AttrDiff // only changed attributes
}

// AttrDiff represents a single attribute change.
type AttrDiff struct {
	Path   string
	Before string
	After  string
	Action string // "add", "change", "remove"
}

// planJSON is the top-level structure of `tofu show -json planfile`
type planJSON struct {
	ResourceChanges []resourceChangeJSON `json:"resource_changes"`
	OutputChanges   map[string]outputChangeJSON `json:"output_changes"`
}

type resourceChangeJSON struct {
	Address      string `json:"address"`
	ModuleAddress string `json:"module_address"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	Change       changeJSON `json:"change"`
}

type changeJSON struct {
	Actions []string       `json:"actions"`
	Before  map[string]any `json:"before"`
	After   map[string]any `json:"after"`
}

type outputChangeJSON struct {
	Actions []string `json:"actions"`
	Before  any      `json:"before"`
	After   any      `json:"after"`
}

// ParsePlan parses JSON plan output into a structured PlanResult.
func ParsePlan(jsonData string) (*PlanResult, error) {
	var plan planJSON
	if err := json.Unmarshal([]byte(jsonData), &plan); err != nil {
		return nil, fmt.Errorf("parse plan JSON: %w", err)
	}

	result := &PlanResult{}
	for _, rc := range plan.ResourceChanges {
		action := resolveAction(rc.Change.Actions)
		if action == "no-op" || action == "read" {
			result.NoOpCount++
			continue
		}

		change := ResourceChange{
			Address: rc.Address,
			Module:  rc.ModuleAddress,
			Type:    rc.Type,
			Name:    rc.Name,
			Action:  action,
			Before:  rc.Change.Before,
			After:   rc.Change.After,
		}

		// Compute diffs for updates
		if action == "update" || action == "replace" {
			change.Diffs = computeDiffs(rc.Change.Before, rc.Change.After)
		} else if action == "create" {
			change.Diffs = createDiffs(rc.Change.After)
		} else if action == "delete" {
			change.Diffs = deleteDiffs(rc.Change.Before)
		}

		result.Changes = append(result.Changes, change)

		switch action {
		case "create":
			result.CreateCount++
		case "update":
			result.UpdateCount++
		case "delete":
			result.DeleteCount++
		case "replace":
			result.ReplaceCount++
		}
	}

	// Sort: delete first, then replace, update, create
	sort.Slice(result.Changes, func(i, j int) bool {
		return actionOrder(result.Changes[i].Action) < actionOrder(result.Changes[j].Action)
	})

	return result, nil
}

func resolveAction(actions []string) string {
	if len(actions) == 0 {
		return "no-op"
	}
	if len(actions) == 1 {
		switch actions[0] {
		case "create":
			return "create"
		case "update":
			return "update"
		case "delete":
			return "delete"
		case "read":
			return "read"
		case "no-op":
			return "no-op"
		}
	}
	// ["delete", "create"] or ["create", "delete"] = replace
	for _, a := range actions {
		if a == "delete" {
			return "replace"
		}
	}
	return actions[0]
}

func actionOrder(action string) int {
	order := map[string]int{
		"delete":  0,
		"replace": 1,
		"update":  2,
		"create":  3,
	}
	if o, ok := order[action]; ok {
		return o
	}
	return 9
}

func computeDiffs(before, after map[string]any) []AttrDiff {
	var diffs []AttrDiff

	// Keys in both, check for changes
	allKeys := make(map[string]bool)
	for k := range before {
		allKeys[k] = true
	}
	for k := range after {
		allKeys[k] = true
	}

	keys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		bVal, bOk := before[k]
		aVal, aOk := after[k]

		bStr := formatValue(bVal)
		aStr := formatValue(aVal)

		if !bOk {
			diffs = append(diffs, AttrDiff{Path: k, After: aStr, Action: "add"})
		} else if !aOk {
			diffs = append(diffs, AttrDiff{Path: k, Before: bStr, Action: "remove"})
		} else if bStr != aStr {
			diffs = append(diffs, AttrDiff{Path: k, Before: bStr, After: aStr, Action: "change"})
		}
	}
	return diffs
}

func createDiffs(after map[string]any) []AttrDiff {
	var diffs []AttrDiff
	keys := sortedKeys(after)
	for _, k := range keys {
		v := formatValue(after[k])
		if v != "" && v != "null" && v != "<nil>" {
			diffs = append(diffs, AttrDiff{Path: k, After: v, Action: "add"})
		}
	}
	return diffs
}

func deleteDiffs(before map[string]any) []AttrDiff {
	var diffs []AttrDiff
	keys := sortedKeys(before)
	for _, k := range keys {
		v := formatValue(before[k])
		if v != "" && v != "null" && v != "<nil>" {
			diffs = append(diffs, AttrDiff{Path: k, Before: v, Action: "remove"})
		}
	}
	return diffs
}

func formatValue(v any) string {
	if v == nil {
		return "<nil>"
	}
	switch val := v.(type) {
	case string:
		if len(val) > 80 {
			return val[:77] + "..."
		}
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case map[string]any:
		b, _ := json.Marshal(val)
		s := string(b)
		if len(s) > 80 {
			return s[:77] + "..."
		}
		return s
	case []any:
		b, _ := json.Marshal(val)
		s := string(b)
		if len(s) > 80 {
			return s[:77] + "..."
		}
		return s
	default:
		return fmt.Sprintf("%v", v)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// FormatPlanSummary returns a human-readable summary like "3 to create, 1 to update, 0 to delete"
func FormatPlanSummary(p *PlanResult) string {
	var parts []string
	if p.CreateCount > 0 {
		parts = append(parts, fmt.Sprintf("%d to create", p.CreateCount))
	}
	if p.UpdateCount > 0 {
		parts = append(parts, fmt.Sprintf("%d to update", p.UpdateCount))
	}
	if p.ReplaceCount > 0 {
		parts = append(parts, fmt.Sprintf("%d to replace", p.ReplaceCount))
	}
	if p.DeleteCount > 0 {
		parts = append(parts, fmt.Sprintf("%d to delete", p.DeleteCount))
	}
	if len(parts) == 0 {
		return "No changes"
	}
	return strings.Join(parts, ", ")
}
