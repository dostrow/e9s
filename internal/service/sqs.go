package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

// SQSAPI is the low-level queue behavior shared by both frontends.
type SQSAPI interface {
	ListSQSQueues(context.Context, string) ([]model.SQSQueue, error)
	GetQueueStats(context.Context, string) (*model.SQSQueueStats, error)
	ReceiveSQSMessages(context.Context, string, int, int) ([]model.SQSMessage, error)
	ChangeSQSMessageVisibility(context.Context, string, string, int) error
	DeleteSQSMessage(context.Context, string, string) error
	SendSQSMessage(context.Context, string, model.SQSSendTemplate) (string, error)
	GetQueueURL(context.Context, string) (string, error)
	GetSQSMetrics(context.Context, string, time.Duration) (*model.MetricSnapshot, error)
}

type SQS struct{ api SQSAPI }

func NewSQS(api SQSAPI) *SQS { return &SQS{api: api} }

func (s *SQS) Queues(ctx context.Context, filter string) ([]model.SQSQueue, error) {
	filter = strings.TrimSpace(filter)
	queues, err := s.api.ListSQSQueues(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list SQS queues: %w", err)
	}
	sort.SliceStable(queues, func(i, j int) bool {
		return strings.ToLower(queues[i].Name) < strings.ToLower(queues[j].Name)
	})
	return queues, nil
}

func (s *SQS) Queue(ctx context.Context, queueURL string) (*model.SQSQueueStats, error) {
	queueURL = strings.TrimSpace(queueURL)
	if queueURL == "" {
		return nil, fmt.Errorf("read SQS queue: queue URL is required")
	}
	stats, err := s.api.GetQueueStats(ctx, queueURL)
	if err != nil {
		return nil, fmt.Errorf("read SQS queue %q: %w", QueueNameFromURL(queueURL), err)
	}
	if stats == nil {
		return nil, fmt.Errorf("read SQS queue %q: queue was not found", QueueNameFromURL(queueURL))
	}
	return stats, nil
}

func (s *SQS) Metrics(ctx context.Context, queue model.SQSQueue, window time.Duration) (*model.MetricSnapshot, error) {
	queueName := strings.TrimSpace(queue.Name)
	if queueName == "" {
		queueName = QueueNameFromURL(queue.URL)
	}
	if queueName == "" {
		return nil, fmt.Errorf("read SQS metrics: queue name is required")
	}
	if window <= 0 {
		return nil, fmt.Errorf("read SQS metrics for %q: time range must be positive", queueName)
	}
	metrics, err := s.api.GetSQSMetrics(ctx, queueName, window)
	if err != nil {
		return nil, fmt.Errorf("read SQS metrics for %q: %w", queueName, err)
	}
	return metrics, nil
}

func (s *SQS) Messages(ctx context.Context, request model.SQSReceiveRequest) ([]model.SQSMessage, error) {
	request.QueueURL = strings.TrimSpace(request.QueueURL)
	if request.QueueURL == "" {
		return nil, fmt.Errorf("poll SQS messages: queue URL is required")
	}
	if request.MaxMessages <= 0 {
		request.MaxMessages = 10
	}
	if request.MaxMessages > 10 {
		request.MaxMessages = 10
	}
	if request.WaitSeconds < 0 || request.WaitSeconds > 20 {
		return nil, fmt.Errorf("poll SQS messages: wait time must be between 0 and 20 seconds")
	}
	messages, err := s.api.ReceiveSQSMessages(ctx, request.QueueURL, request.MaxMessages, request.WaitSeconds)
	if err != nil {
		return nil, fmt.Errorf("poll SQS queue %q: %w", QueueNameFromURL(request.QueueURL), err)
	}
	return messages, nil
}

func (s *SQS) ResolveQueueURL(ctx context.Context, queueName string) (string, error) {
	queueName = strings.TrimSpace(queueName)
	if queueName == "" {
		return "", fmt.Errorf("resolve SQS queue: queue name is required")
	}
	url, err := s.api.GetQueueURL(ctx, queueName)
	if err != nil {
		return "", fmt.Errorf("resolve SQS queue %q: %w", queueName, err)
	}
	return url, nil
}

func (s *SQS) DeleteMessage(ctx context.Context, queueURL, receiptHandle string) error {
	queueURL, receiptHandle = strings.TrimSpace(queueURL), strings.TrimSpace(receiptHandle)
	if queueURL == "" || receiptHandle == "" {
		return fmt.Errorf("delete SQS message: queue URL and receipt handle are required")
	}
	if err := s.api.DeleteSQSMessage(ctx, queueURL, receiptHandle); err != nil {
		return fmt.Errorf("delete SQS message from %q: %w", QueueNameFromURL(queueURL), err)
	}
	return nil
}

// ReleaseMessage immediately returns a captured message to the queue by
// resetting its visibility timeout. The receipt handle remains valid only for
// the receive operation that produced it.
func (s *SQS) ReleaseMessage(ctx context.Context, queueURL, receiptHandle string) error {
	queueURL, receiptHandle = strings.TrimSpace(queueURL), strings.TrimSpace(receiptHandle)
	if queueURL == "" || receiptHandle == "" {
		return fmt.Errorf("release SQS message: queue URL and receipt handle are required")
	}
	if err := s.api.ChangeSQSMessageVisibility(ctx, queueURL, receiptHandle, 0); err != nil {
		return fmt.Errorf("release SQS message to %q: %w", QueueNameFromURL(queueURL), err)
	}
	return nil
}

func (s *SQS) SendMessage(ctx context.Context, request model.SQSSendRequest) (string, error) {
	request.QueueURL = strings.TrimSpace(request.QueueURL)
	if request.QueueURL == "" {
		return "", fmt.Errorf("send SQS message: queue URL is required")
	}
	if err := ValidateSQSSendTemplate(request.QueueURL, request.Template); err != nil {
		return "", err
	}
	id, err := s.api.SendSQSMessage(ctx, request.QueueURL, request.Template)
	if err != nil {
		return "", fmt.Errorf("send SQS message to %q: %w", QueueNameFromURL(request.QueueURL), err)
	}
	return id, nil
}

func ValidateSQSSendTemplate(queueURL string, template model.SQSSendTemplate) error {
	if template.Body == "" {
		return fmt.Errorf("send SQS message: message body is required")
	}
	if template.DelaySeconds < 0 || template.DelaySeconds > 900 {
		return fmt.Errorf("send SQS message: delay must be between 0 and 900 seconds")
	}
	if strings.HasSuffix(strings.ToLower(QueueNameFromURL(queueURL)), ".fifo") && strings.TrimSpace(template.GroupID) == "" {
		return fmt.Errorf("send SQS message: FIFO queues require a message group ID")
	}
	for name, attribute := range template.Attributes {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("send SQS message: attribute name is required")
		}
		baseType := strings.SplitN(attribute.DataType, ".", 2)[0]
		switch baseType {
		case "String", "Number":
		case "Binary":
			if _, err := base64.StdEncoding.DecodeString(attribute.Value); err != nil {
				return fmt.Errorf("send SQS message: binary attribute %q must be base64: %w", name, err)
			}
		default:
			return fmt.Errorf("send SQS message: attribute %q has unsupported data type %q", name, attribute.DataType)
		}
	}
	return nil
}

func BuildSQSSendTemplate(isFIFO bool) string {
	template := model.SQSSendTemplate{Attributes: map[string]model.SQSAttr{}}
	if isFIFO {
		template.GroupID = ""
		template.DeduplicationID = ""
	}
	data, _ := json.MarshalIndent(template, "", "  ")
	return string(data)
}

func BuildSQSSendTemplateFromMessage(message model.SQSMessage) string {
	template := model.SQSSendTemplate{
		Body:       message.Body,
		Attributes: make(map[string]model.SQSAttr, len(message.UserAttributes)),
	}
	if groupID, found := message.Attributes["MessageGroupId"]; found {
		template.GroupID = groupID
	}
	for name, attribute := range message.UserAttributes {
		value := attribute.StringValue
		if len(attribute.BinaryValue) > 0 {
			value = base64.StdEncoding.EncodeToString(attribute.BinaryValue)
		}
		template.Attributes[name] = model.SQSAttr{DataType: attribute.DataType, Value: value}
	}
	data, _ := json.MarshalIndent(template, "", "  ")
	return string(data)
}

func ParseSQSSendTemplate(document string) (*model.SQSSendTemplate, error) {
	var template model.SQSSendTemplate
	if err := json.Unmarshal([]byte(document), &template); err != nil {
		return nil, fmt.Errorf("parse SQS message template: %w", err)
	}
	if template.Body == "" {
		return nil, fmt.Errorf("parse SQS message template: message body is required")
	}
	if template.Attributes == nil {
		template.Attributes = map[string]model.SQSAttr{}
	}
	return &template, nil
}

func SQSMessageAttributeDisplay(attribute model.SQSMessageAttribute) string {
	if len(attribute.BinaryValue) > 0 {
		return fmt.Sprintf("(%s, %d bytes)", attribute.DataType, len(attribute.BinaryValue))
	}
	return attribute.StringValue
}

func QueueNameFromARN(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) >= 6 {
		return parts[5]
	}
	return arn
}

func QueueNameFromURL(url string) string {
	url = strings.TrimRight(url, "/")
	if index := strings.LastIndex(url, "/"); index >= 0 {
		return url[index+1:]
	}
	return url
}
