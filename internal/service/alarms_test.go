package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

type fakeAlarmsAPI struct {
	alarms        []model.Alarm
	detail        *model.AlarmDetail
	err           error
	listState     string
	actionsName   string
	actionsEnable bool
	stateName     string
	state         string
	reason        string
}

func (f *fakeAlarmsAPI) ListCWAlarms(_ context.Context, state string) ([]model.Alarm, error) {
	f.listState = state
	return append([]model.Alarm(nil), f.alarms...), f.err
}

func (f *fakeAlarmsAPI) DescribeCWAlarm(context.Context, string) (*model.AlarmDetail, error) {
	return f.detail, f.err
}

func (f *fakeAlarmsAPI) EnableAlarmActions(_ context.Context, name string) error {
	f.actionsName, f.actionsEnable = name, true
	return f.err
}

func (f *fakeAlarmsAPI) DisableAlarmActions(_ context.Context, name string) error {
	f.actionsName, f.actionsEnable = name, false
	return f.err
}

func (f *fakeAlarmsAPI) SetAlarmState(_ context.Context, name, state, reason string) error {
	f.stateName, f.state, f.reason = name, state, reason
	return f.err
}

func TestAlarmsListValidatesAndSorts(t *testing.T) {
	now := time.Now()
	api := &fakeAlarmsAPI{alarms: []model.Alarm{
		{Name: "older", StateUpdatedAt: now.Add(-time.Hour)},
		{Name: "newer", StateUpdatedAt: now},
	}}
	got, err := NewAlarms(api).List(context.Background(), model.AlarmStateAlarm)
	if err != nil {
		t.Fatal(err)
	}
	if api.listState != model.AlarmStateAlarm || !reflect.DeepEqual([]string{got[0].Name, got[1].Name}, []string{"newer", "older"}) {
		t.Fatalf("List() = %#v, state %q", got, api.listState)
	}
	if _, err := NewAlarms(api).List(context.Background(), "BROKEN"); err == nil {
		t.Fatal("invalid state was accepted")
	}
}

func TestAlarmsDetailReportsMissingAlarm(t *testing.T) {
	_, err := NewAlarms(&fakeAlarmsAPI{}).Detail(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Detail() error = %v", err)
	}
}

func TestAlarmsMutationsValidateAndRoute(t *testing.T) {
	api := &fakeAlarmsAPI{}
	svc := NewAlarms(api)
	if err := svc.SetActionsEnabled(context.Background(), "api", false); err != nil {
		t.Fatal(err)
	}
	if api.actionsName != "api" || api.actionsEnable {
		t.Fatalf("action route = %q, %t", api.actionsName, api.actionsEnable)
	}
	if err := svc.SetState(context.Background(), "api", model.AlarmStateOK, "manual test"); err != nil {
		t.Fatal(err)
	}
	if api.stateName != "api" || api.state != model.AlarmStateOK || api.reason != "manual test" {
		t.Fatalf("state route = %q %q %q", api.stateName, api.state, api.reason)
	}
	if err := svc.SetState(context.Background(), "api", "BROKEN", "reason"); err == nil {
		t.Fatal("invalid mutation state was accepted")
	}
	if err := svc.SetState(context.Background(), "api", model.AlarmStateOK, ""); err == nil {
		t.Fatal("empty reason was accepted")
	}
}

func TestAlarmsWrapsAPIErrors(t *testing.T) {
	api := &fakeAlarmsAPI{err: errors.New("denied")}
	if _, err := NewAlarms(api).List(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "list CloudWatch alarms") {
		t.Fatalf("List() error = %v", err)
	}
}
