package aws

import (
	"context"

	awslib "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/dostrow/e9s/internal/model"
)

// Backward-compatible aliases for callers migrating to the UI-neutral model.
type CWAlarm = model.Alarm
type CWAlarmDetail = model.AlarmDetail
type CWAlarmHistoryItem = model.AlarmHistoryItem

// ListCWAlarms returns all CloudWatch metric alarms, optionally filtered by state.
func (c *Client) ListCWAlarms(ctx context.Context, stateFilter string) ([]CWAlarm, error) {
	input := &cloudwatch.DescribeAlarmsInput{
		AlarmTypes: []cwtypes.AlarmType{cwtypes.AlarmTypeMetricAlarm},
	}
	if stateFilter != "" {
		input.StateValue = cwtypes.StateValue(stateFilter)
	}

	var alarms []CWAlarm
	paginator := cloudwatch.NewDescribeAlarmsPaginator(c.CW, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, a := range page.MetricAlarms {
			alarms = append(alarms, cwAlarmFromMetric(a))
		}
	}
	return alarms, nil
}

// DescribeCWAlarm fetches full detail for a single alarm by name.
func (c *Client) DescribeCWAlarm(ctx context.Context, alarmName string) (*CWAlarmDetail, error) {
	out, err := c.CW.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{
		AlarmNames: []string{alarmName},
		AlarmTypes: []cwtypes.AlarmType{cwtypes.AlarmTypeMetricAlarm},
	})
	if err != nil {
		return nil, err
	}
	if len(out.MetricAlarms) == 0 {
		return nil, nil
	}

	a := out.MetricAlarms[0]
	detail := &CWAlarmDetail{
		Alarm:        cwAlarmFromMetric(a),
		Description:  derefStrAws(a.AlarmDescription),
		Threshold:    derefFloat64(a.Threshold),
		EvalPeriods:  int(derefInt32(a.EvaluationPeriods)),
		Period:       int(derefInt32(a.Period)),
		TreatMissing: derefStrAws(a.TreatMissingData),
	}
	if a.ComparisonOperator != "" {
		detail.ComparisonOp = string(a.ComparisonOperator)
	}
	if a.Statistic != "" {
		detail.Statistic = string(a.Statistic)
	}
	detail.Dimensions = make(map[string]string)
	for _, d := range a.Dimensions {
		if d.Name != nil && d.Value != nil {
			detail.Dimensions[*d.Name] = *d.Value
		}
	}
	detail.AlarmActions = append(detail.AlarmActions, a.AlarmActions...)
	detail.OKActions = append(detail.OKActions, a.OKActions...)
	detail.InsufficientActions = append(detail.InsufficientActions, a.InsufficientDataActions...)

	// Fetch recent history
	hist, err := c.CW.DescribeAlarmHistory(ctx, &cloudwatch.DescribeAlarmHistoryInput{
		AlarmName:  &alarmName,
		MaxRecords: awslib.Int32(20),
	})
	if err == nil {
		for _, h := range hist.AlarmHistoryItems {
			detail.History = append(detail.History, CWAlarmHistoryItem{
				Timestamp: derefTimeAws(h.Timestamp),
				Type:      string(h.HistoryItemType),
				Summary:   derefStrAws(h.HistorySummary),
			})
		}
	}

	return detail, nil
}

// EnableAlarmActions enables actions for the given alarm.
func (c *Client) EnableAlarmActions(ctx context.Context, alarmName string) error {
	_, err := c.CW.EnableAlarmActions(ctx, &cloudwatch.EnableAlarmActionsInput{
		AlarmNames: []string{alarmName},
	})
	return err
}

// DisableAlarmActions disables actions for the given alarm.
func (c *Client) DisableAlarmActions(ctx context.Context, alarmName string) error {
	_, err := c.CW.DisableAlarmActions(ctx, &cloudwatch.DisableAlarmActionsInput{
		AlarmNames: []string{alarmName},
	})
	return err
}

// SetAlarmState overrides an alarm's state (for testing).
func (c *Client) SetAlarmState(ctx context.Context, alarmName, state, reason string) error {
	_, err := c.CW.SetAlarmState(ctx, &cloudwatch.SetAlarmStateInput{
		AlarmName:   &alarmName,
		StateValue:  cwtypes.StateValue(state),
		StateReason: &reason,
	})
	return err
}

func cwAlarmFromMetric(a cwtypes.MetricAlarm) CWAlarm {
	return CWAlarm{
		Name:           derefStrAws(a.AlarmName),
		State:          string(a.StateValue),
		StateReason:    derefStrAws(a.StateReason),
		StateUpdatedAt: derefTimeAws(a.StateUpdatedTimestamp),
		MetricName:     derefStrAws(a.MetricName),
		Namespace:      derefStrAws(a.Namespace),
		ActionsEnabled: a.ActionsEnabled != nil && *a.ActionsEnabled,
		ARN:            derefStrAws(a.AlarmArn),
	}
}

func derefFloat64(p *float64) float64 {
	if p != nil {
		return *p
	}
	return 0
}

func derefInt32(p *int32) int32 {
	if p != nil {
		return *p
	}
	return 0
}
