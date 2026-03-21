package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

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
