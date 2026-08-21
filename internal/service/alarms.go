package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

// AlarmsAPI is the low-level CloudWatch behavior used by shared alarm flows.
type AlarmsAPI interface {
	ListCWAlarms(context.Context, string) ([]model.Alarm, error)
	DescribeCWAlarm(context.Context, string) (*model.AlarmDetail, error)
	EnableAlarmActions(context.Context, string) error
	DisableAlarmActions(context.Context, string) error
	SetAlarmState(context.Context, string, string, string) error
}

// Alarms exposes CloudWatch alarm workflows without frontend dependencies.
type Alarms struct {
	api AlarmsAPI
}

func NewAlarms(api AlarmsAPI) *Alarms {
	return &Alarms{api: api}
}

func (s *Alarms) List(ctx context.Context, state string) ([]model.Alarm, error) {
	if !validAlarmState(state, true) {
		return nil, fmt.Errorf("list CloudWatch alarms: invalid state %q", state)
	}
	alarms, err := s.api.ListCWAlarms(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("list CloudWatch alarms: %w", err)
	}
	sort.SliceStable(alarms, func(i, j int) bool {
		return alarms[i].StateUpdatedAt.After(alarms[j].StateUpdatedAt)
	})
	return alarms, nil
}

func (s *Alarms) Detail(ctx context.Context, name string) (*model.AlarmDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("describe CloudWatch alarm: name is required")
	}
	detail, err := s.api.DescribeCWAlarm(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("describe CloudWatch alarm %q: %w", name, err)
	}
	if detail == nil {
		return nil, fmt.Errorf("describe CloudWatch alarm %q: alarm was not found", name)
	}
	return detail, nil
}

func (s *Alarms) SetActionsEnabled(ctx context.Context, name string, enabled bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("update CloudWatch alarm actions: name is required")
	}
	var err error
	if enabled {
		err = s.api.EnableAlarmActions(ctx, name)
	} else {
		err = s.api.DisableAlarmActions(ctx, name)
	}
	if err != nil {
		return fmt.Errorf("update CloudWatch alarm actions for %q: %w", name, err)
	}
	return nil
}

func (s *Alarms) SetState(ctx context.Context, name, state, reason string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("set CloudWatch alarm state: name is required")
	}
	if !validAlarmState(state, false) {
		return fmt.Errorf("set CloudWatch alarm state for %q: invalid state %q", name, state)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("set CloudWatch alarm state for %q: reason is required", name)
	}
	if err := s.api.SetAlarmState(ctx, name, state, reason); err != nil {
		return fmt.Errorf("set CloudWatch alarm state for %q: %w", name, err)
	}
	return nil
}

func validAlarmState(state string, allowEmpty bool) bool {
	if state == "" {
		return allowEmpty
	}
	switch state {
	case model.AlarmStateOK, model.AlarmStateAlarm, model.AlarmStateInsufficientData:
		return true
	default:
		return false
	}
}
