// Package command holds the frontend-independent model behind e9s's command
// palette: command metadata, fuzzy matching, ranking, and recency tracking.
// The TUI and GUI each resolve commands against their own state and key
// bindings, then hand the resulting entries to Filter for display.
package command

// Command describes something a user can invoke. IDs are shared between
// frontends ("ecs.force-redeploy") even when the key bindings differ.
type Command struct {
	ID       string
	Title    string
	Category string
	Keywords []string
	Danger   bool
}

// Section groups entries in the palette's browse view (empty query). Lower
// values sort first.
type Section int

const (
	SectionView Section = iota
	SectionRecent
	SectionSaved
	SectionPlugins
	SectionGlobal
	SectionGoTo
	SectionResources
	SectionUnavailable
)

// Title returns the heading shown above a section in the browse view.
func (s Section) Title() string {
	switch s {
	case SectionRecent:
		return "Recent"
	case SectionView:
		return "This view"
	case SectionSaved:
		return "Saved"
	case SectionPlugins:
		return "Plugins"
	case SectionGlobal:
		return "Global"
	case SectionGoTo:
		return "Go to"
	case SectionResources:
		return "Resources"
	case SectionUnavailable:
		return "Not available here"
	}
	return ""
}

// Entry is a Command resolved for the current frontend context.
type Entry struct {
	Command
	// Key is the display form of the shortcut that runs the command
	// directly, or empty when there is none.
	Key string
	// Target names the object the command acts on (a task ID, a queue).
	Target string
	// Section places the entry in the browse view.
	Section Section
	// Disabled, when non-empty, explains why the command cannot run here.
	Disabled string
}

// Available reports whether the entry can run in the current context.
func (e Entry) Available() bool { return e.Disabled == "" }
