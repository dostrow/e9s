package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/config"
)

// --- Config Editor ---

func (a App) openConfigEditor() (App, tea.Cmd) {
	path := config.Path()
	if path == "" {
		a.err = fmt.Errorf("could not determine config file path")
		return a, nil
	}

	// Ensure the file exists with current config
	if err := a.cfg.Save(); err != nil {
		a.err = err
		return a, nil
	}

	editor := NewEditorCmd(path)
	return a, tea.Exec(editor, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err}
		}
		return configEditedMsg{}
	})
}

// --- Multi-Region ---

func (a App) switchRegion(region string) tea.Cmd {
	client := a.client
	return func() tea.Msg {
		err := client.SwitchRegion(context.Background(), region)
		if err != nil {
			return errMsg{err}
		}
		return regionSwitchedMsg{}
	}
}

// --- Saved Entry Deletion ---

func (a App) handlePickerDelete(msg PickerDeleteMsg) (App, tea.Cmd) {
	switch msg.Action {
	case PickerSSMPrefix:
		if msg.Index < len(a.cfg.SSMPrefixes) {
			name := a.cfg.SSMPrefixes[msg.Index].Name
			a.cfg.RemoveSSMPrefix(name)
			if err := a.cfg.Save(); err != nil {
				a.err = err
				return a, nil
			}
			a.flashMessage = fmt.Sprintf("Deleted SSM prefix %q", name)
			a.flashExpiry = time.Now().Add(3 * time.Second)
			return a.promptSSMPath()
		}
	case PickerLogPath:
		if msg.Index < len(a.cfg.LogPaths) {
			name := a.cfg.LogPaths[msg.Index].Name
			a.cfg.RemoveLogPath(name)
			if err := a.cfg.Save(); err != nil {
				a.err = err
				return a, nil
			}
			a.flashMessage = fmt.Sprintf("Deleted log path %q", name)
			a.flashExpiry = time.Now().Add(3 * time.Second)
			return a.promptCloudWatchBrowser()
		}
	case PickerSMFilter:
		if msg.Index < len(a.cfg.SMFilters) {
			name := a.cfg.SMFilters[msg.Index].Name
			a.cfg.RemoveSMFilter(name)
			if err := a.cfg.Save(); err != nil {
				a.err = err
				return a, nil
			}
			a.flashMessage = fmt.Sprintf("Deleted SM filter %q", name)
			a.flashExpiry = time.Now().Add(3 * time.Second)
			return a.promptSMFilter()
		}
	case PickerLambdaSearch:
		if msg.Index < len(a.cfg.LambdaSearches) {
			name := a.cfg.LambdaSearches[msg.Index].Name
			a.cfg.RemoveLambdaSearch(name)
			if err := a.cfg.Save(); err != nil {
				a.err = err
				return a, nil
			}
			a.flashMessage = fmt.Sprintf("Deleted Lambda search %q", name)
			a.flashExpiry = time.Now().Add(3 * time.Second)
			return a.promptLambdaBrowser()
		}
	case PickerDynamoTable:
		if msg.Index < len(a.cfg.DynamoTables) {
			name := a.cfg.DynamoTables[msg.Index].Name
			a.cfg.RemoveDynamoTable(name)
			if err := a.cfg.Save(); err != nil {
				a.err = err
				return a, nil
			}
			a.flashMessage = fmt.Sprintf("Deleted DynamoDB table %q", name)
			a.flashExpiry = time.Now().Add(3 * time.Second)
			return a.promptDynamoBrowser()
		}
	case PickerDynamoQuery:
		if msg.Index < len(a.cfg.DynamoQueries) {
			name := a.cfg.DynamoQueries[msg.Index].Name
			a.cfg.RemoveDynamoQuery(name)
			if err := a.cfg.Save(); err != nil {
				a.err = err
				return a, nil
			}
			a.flashMessage = fmt.Sprintf("Deleted PartiQL query %q", name)
			a.flashExpiry = time.Now().Add(3 * time.Second)
			return a.promptDynamoPartiQL()
		}
	case PickerS3Search:
		if msg.Index < len(a.cfg.S3Searches) {
			name := a.cfg.S3Searches[msg.Index].Name
			a.cfg.RemoveS3Search(name)
			if err := a.cfg.Save(); err != nil {
				a.err = err
				return a, nil
			}
			a.flashMessage = fmt.Sprintf("Deleted S3 search %q", name)
			a.flashExpiry = time.Now().Add(3 * time.Second)
			return a.promptS3Browser()
		}
	}
	return a, nil
}
