package ui

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- Route53 ---

func (a App) openR53Zones() (App, tea.Cmd) {
	a.mode = modeRoute53
	a.state = viewR53Zones
	a.r53ZonesView = views.NewR53Zones()
	a.r53ZonesView = a.r53ZonesView.SetSize(a.width-3, a.height-6)
	a.loading = true
	client := a.client
	return a, func() tea.Msg {
		zones, err := client.ListR53Zones(context.Background(), "")
		if err != nil {
			return errMsg{err}
		}
		return r53ZonesLoadedMsg{zones}
	}
}

func (a App) openR53Records(zoneName, zoneID string) (App, tea.Cmd) {
	a.state = viewR53Records
	a.r53RecordsView = views.NewR53Records(zoneName, zoneID)
	a.r53RecordsView = a.r53RecordsView.SetSize(a.width-3, a.height-6)
	a.loading = true
	client := a.client
	return a, func() tea.Msg {
		records, err := client.ListR53Records(context.Background(), zoneID)
		if err != nil {
			return errMsg{err}
		}
		return r53RecordsLoadedMsg{records}
	}
}

func (a App) openR53RecordDetail() (App, tea.Cmd) {
	rec := a.r53RecordsView.SelectedRecord()
	if rec == nil {
		return a, nil
	}
	a.state = viewR53RecordDetail
	a.r53DetailView = views.NewR53RecordDetail(rec,
		a.r53RecordsView.ZoneName(), a.r53RecordsView.ZoneID())
	a.r53DetailView = a.r53DetailView.SetSize(a.width-3, a.height-6)
	return a, nil
}

func (a App) testR53DNS() (App, tea.Cmd) {
	rec := a.r53DetailView.Record()
	if rec == nil {
		return a, nil
	}
	zoneID := a.r53DetailView.ZoneID()
	client := a.client
	name := rec.Name
	rType := rec.Type
	a.loading = true
	return a, func() tea.Msg {
		answer, err := client.TestR53DNS(context.Background(), zoneID, name, rType)
		if err != nil {
			return errMsg{err}
		}
		return r53DNSAnswerMsg{answer}
	}
}

func (a App) createR53Record() (App, tea.Cmd) {
	zoneID := a.r53RecordsView.ZoneID()
	if zoneID == "" {
		return a, nil
	}
	template := aws.BuildR53RecordTemplate(nil)
	tmpFile, err := os.CreateTemp("", "e9s-r53-*.json")
	if err != nil {
		a.err = err
		return a, nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(template)
	tmpFile.Close()

	a.r53EditZoneID = zoneID
	editor := NewEditorCmd(tmpPath)
	return a, tea.Exec(editor, func(err error) tea.Msg {
		defer os.Remove(tmpPath)
		if err != nil {
			return errMsg{err}
		}
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return errMsg{err}
		}
		record, err := aws.ParseR53RecordTemplate(string(data))
		if err != nil {
			return errMsg{err}
		}
		return r53RecordEditedMsg{record: record, isNew: true}
	})
}

func (a App) editR53Record() (App, tea.Cmd) {
	rec := a.r53DetailView.Record()
	if rec == nil {
		return a, nil
	}
	zoneID := a.r53DetailView.ZoneID()
	template := aws.BuildR53RecordTemplate(rec)
	tmpFile, err := os.CreateTemp("", "e9s-r53-*.json")
	if err != nil {
		a.err = err
		return a, nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(template)
	tmpFile.Close()

	a.r53EditZoneID = zoneID
	a.r53EditOriginal = rec
	editor := NewEditorCmd(tmpPath)
	return a, tea.Exec(editor, func(err error) tea.Msg {
		defer os.Remove(tmpPath)
		if err != nil {
			return errMsg{err}
		}
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return errMsg{err}
		}
		record, err := aws.ParseR53RecordTemplate(string(data))
		if err != nil {
			return errMsg{err}
		}
		return r53RecordEditedMsg{record: record, isNew: false}
	})
}

func (a App) deleteR53Record() (App, tea.Cmd) {
	rec := a.r53DetailView.Record()
	if rec == nil {
		return a, nil
	}
	// Don't allow deleting NS or SOA records
	if rec.Type == "NS" || rec.Type == "SOA" {
		a.err = fmt.Errorf("cannot delete %s records", rec.Type)
		return a, nil
	}
	a.confirm = NewConfirm(ConfirmR53Delete,
		fmt.Sprintf("Delete %s record %q?", rec.Type, rec.Name))
	return a, nil
}

func (a App) doCreateR53Record() tea.Cmd {
	client := a.client
	zoneID := a.r53EditZoneID
	record := a.r53EditRecord
	return func() tea.Msg {
		err := client.CreateR53Record(context.Background(), zoneID, *record)
		if err != nil {
			return errMsg{err}
		}
		return r53ActionDoneMsg{fmt.Sprintf("Created %s record %s", record.Type, record.Name)}
	}
}

func (a App) doUpdateR53Record() tea.Cmd {
	client := a.client
	zoneID := a.r53EditZoneID
	record := a.r53EditRecord
	return func() tea.Msg {
		err := client.UpdateR53Record(context.Background(), zoneID, *record)
		if err != nil {
			return errMsg{err}
		}
		return r53ActionDoneMsg{fmt.Sprintf("Updated %s record %s", record.Type, record.Name)}
	}
}

func (a App) doDeleteR53Record() tea.Cmd {
	client := a.client
	zoneID := a.r53DetailView.ZoneID()
	record := a.r53DetailView.Record()
	if record == nil {
		return nil
	}
	rec := *record
	return func() tea.Msg {
		err := client.DeleteR53Record(context.Background(), zoneID, rec)
		if err != nil {
			return errMsg{err}
		}
		return r53ActionDoneMsg{fmt.Sprintf("Deleted %s record %s", rec.Type, rec.Name)}
	}
}

func (a App) refreshR53Zones() tea.Cmd {
	client := a.client
	return func() tea.Msg {
		zones, err := client.ListR53Zones(context.Background(), "")
		if err != nil {
			return errMsg{err}
		}
		return r53ZonesLoadedMsg{zones}
	}
}

func (a App) refreshR53Records() tea.Cmd {
	zoneID := a.r53RecordsView.ZoneID()
	client := a.client
	return func() tea.Msg {
		records, err := client.ListR53Records(context.Background(), zoneID)
		if err != nil {
			return errMsg{err}
		}
		return r53RecordsLoadedMsg{records}
	}
}

func (a App) handleR53Action(msg r53ActionDoneMsg) (App, tea.Cmd) {
	a.flashMessage = msg.message
	a.flashExpiry = time.Now().Add(5 * time.Second)
	a.loading = false
	// Go back to records list and refresh
	if a.state == viewR53RecordDetail {
		a.state = viewR53Records
	}
	return a, a.refreshR53Records()
}
