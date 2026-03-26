package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSQueue struct {
	Name string
	URL  string
}

type SQSQueueStats struct {
	URL                 string
	MessagesAvailable   int
	MessagesInFlight    int
	MessagesDelayed     int
	RetentionSeconds    int
	VisibilityTimeout   int
	DelaySeconds        int
	MaxMessageSize      int
	IsFIFO              bool
	DeadLetterTargetARN string
	MaxReceiveCount     int
}

type SQSMessage struct {
	MessageID     string
	ReceiptHandle string
	Body          string
	MD5           string
	Attributes    map[string]string                         // system attributes
	UserAttrs     map[string]sqstypes.MessageAttributeValue // user message attributes
	UserAttrsMap  map[string]string                         // simplified user attrs for display
}

// SQSSendTemplate is the JSON structure for composing a message in $EDITOR.
type SQSSendTemplate struct {
	Body            string             `json:"body"`
	GroupID         string             `json:"groupId"`
	DeduplicationID string             `json:"deduplicationId"`
	DelaySeconds    int                `json:"delaySeconds"`
	Attributes      map[string]SQSAttr `json:"attributes"`
}

type SQSAttr struct {
	DataType string `json:"dataType"` // String, Number, Binary
	Value    string `json:"value"`
}

// ListSQSQueues returns SQS queues. The SQS API only supports prefix matching,
// so for substring searches we fetch all queues and filter client-side.
func (c *Client) ListSQSQueues(ctx context.Context, filter string) ([]SQSQueue, error) {
	input := &sqs.ListQueuesInput{}

	var queues []SQSQueue
	lf := strings.ToLower(filter)
	paginator := sqs.NewListQueuesPaginator(c.SQS, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, url := range page.QueueUrls {
			name := queueNameFromURL(url)
			if filter == "" || strings.Contains(strings.ToLower(name), lf) {
				queues = append(queues, SQSQueue{Name: name, URL: url})
			}
		}
	}
	return queues, nil
}

// GetQueueStats returns queue attributes/statistics.
func (c *Client) GetQueueStats(ctx context.Context, queueURL string) (*SQSQueueStats, error) {
	out, err := c.SQS.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: &queueURL,
		AttributeNames: []sqstypes.QueueAttributeName{
			sqstypes.QueueAttributeNameAll,
		},
	})
	if err != nil {
		return nil, err
	}

	attrs := out.Attributes
	stats := &SQSQueueStats{
		URL:               queueURL,
		MessagesAvailable: atoi(attrs["ApproximateNumberOfMessages"]),
		MessagesInFlight:  atoi(attrs["ApproximateNumberOfMessagesNotVisible"]),
		MessagesDelayed:   atoi(attrs["ApproximateNumberOfMessagesDelayed"]),
		RetentionSeconds:  atoi(attrs["MessageRetentionPeriod"]),
		VisibilityTimeout: atoi(attrs["VisibilityTimeout"]),
		DelaySeconds:      atoi(attrs["DelaySeconds"]),
		MaxMessageSize:    atoi(attrs["MaximumMessageSize"]),
		IsFIFO:            strings.HasSuffix(queueNameFromURL(queueURL), ".fifo"),
	}

	if dlq, ok := attrs["RedrivePolicy"]; ok {
		var rp struct {
			DeadLetterTargetARN string `json:"deadLetterTargetArn"`
			MaxReceiveCount     int    `json:"maxReceiveCount"`
		}
		if err := json.Unmarshal([]byte(dlq), &rp); err == nil {
			stats.DeadLetterTargetARN = rp.DeadLetterTargetARN
			stats.MaxReceiveCount = rp.MaxReceiveCount
		}
	}

	return stats, nil
}

// ReceiveSQSMessages polls for messages from a queue.
func (c *Client) ReceiveSQSMessages(ctx context.Context, queueURL string, maxMessages int, waitSeconds int) ([]SQSMessage, error) {
	if maxMessages <= 0 {
		maxMessages = 10
	}
	if maxMessages > 10 {
		maxMessages = 10
	}
	wait := int32(waitSeconds)
	max := int32(maxMessages)

	out, err := c.SQS.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:                    &queueURL,
		MaxNumberOfMessages:         max,
		WaitTimeSeconds:             wait,
		AttributeNames:              []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameAll},
		MessageSystemAttributeNames: []sqstypes.MessageSystemAttributeName{sqstypes.MessageSystemAttributeNameAll},
		MessageAttributeNames:       []string{"All"},
	})
	if err != nil {
		return nil, err
	}

	var messages []SQSMessage
	for _, m := range out.Messages {
		msg := SQSMessage{
			MessageID:     derefStrAws(m.MessageId),
			ReceiptHandle: derefStrAws(m.ReceiptHandle),
			Body:          derefStrAws(m.Body),
			MD5:           derefStrAws(m.MD5OfBody),
			Attributes:    m.Attributes,
			UserAttrs:     m.MessageAttributes,
			UserAttrsMap:  make(map[string]string),
		}
		for k, v := range m.MessageAttributes {
			if v.StringValue != nil {
				msg.UserAttrsMap[k] = *v.StringValue
			} else {
				msg.UserAttrsMap[k] = fmt.Sprintf("(%s)", derefStrAws(v.DataType))
			}
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// DeleteSQSMessage deletes a message from a queue (acknowledges it).
func (c *Client) DeleteSQSMessage(ctx context.Context, queueURL, receiptHandle string) error {
	_, err := c.SQS.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &queueURL,
		ReceiptHandle: &receiptHandle,
	})
	return err
}

// SendSQSMessage sends a message to a queue using the template structure.
func (c *Client) SendSQSMessage(ctx context.Context, queueURL string, tmpl SQSSendTemplate) (string, error) {
	input := &sqs.SendMessageInput{
		QueueUrl:    &queueURL,
		MessageBody: &tmpl.Body,
	}

	if tmpl.GroupID != "" {
		input.MessageGroupId = &tmpl.GroupID
	}
	if tmpl.DeduplicationID != "" {
		input.MessageDeduplicationId = &tmpl.DeduplicationID
	}
	if tmpl.DelaySeconds > 0 {
		delay := int32(tmpl.DelaySeconds)
		input.DelaySeconds = delay
	}
	if len(tmpl.Attributes) > 0 {
		attrs := make(map[string]sqstypes.MessageAttributeValue)
		for k, v := range tmpl.Attributes {
			attrs[k] = sqstypes.MessageAttributeValue{
				DataType:    &v.DataType,
				StringValue: &v.Value,
			}
		}
		input.MessageAttributes = attrs
	}

	out, err := c.SQS.SendMessage(ctx, input)
	if err != nil {
		return "", err
	}
	return derefStrAws(out.MessageId), nil
}

// BuildSendTemplate creates a JSON template string for editing in $EDITOR.
func BuildSendTemplate(isFIFO bool) string {
	tmpl := SQSSendTemplate{
		Body:       "",
		Attributes: map[string]SQSAttr{},
	}
	if isFIFO {
		tmpl.GroupID = ""
		tmpl.DeduplicationID = ""
	}
	b, _ := json.MarshalIndent(tmpl, "", "  ")
	return string(b)
}

// BuildSendTemplateFromMessage creates a template pre-filled from a received message.
func BuildSendTemplateFromMessage(msg SQSMessage) string {
	tmpl := SQSSendTemplate{
		Body:       msg.Body,
		Attributes: map[string]SQSAttr{},
	}
	if gid, ok := msg.Attributes["MessageGroupId"]; ok {
		tmpl.GroupID = gid
	}
	for k, v := range msg.UserAttrs {
		tmpl.Attributes[k] = SQSAttr{
			DataType: derefStrAws(v.DataType),
			Value:    derefStrAws(v.StringValue),
		}
	}
	b, _ := json.MarshalIndent(tmpl, "", "  ")
	return string(b)
}

// ParseSendTemplate parses a JSON template back into a SQSSendTemplate.
func ParseSendTemplate(jsonStr string) (*SQSSendTemplate, error) {
	var tmpl SQSSendTemplate
	if err := json.Unmarshal([]byte(jsonStr), &tmpl); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if tmpl.Body == "" {
		return nil, fmt.Errorf("message body is required")
	}
	return &tmpl, nil
}

// GetQueueURL resolves a queue name to its URL.
func (c *Client) GetQueueURL(ctx context.Context, queueName string) (string, error) {
	out, err := c.SQS.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		return "", err
	}
	if out.QueueUrl == nil {
		return "", fmt.Errorf("queue %q not found", queueName)
	}
	return *out.QueueUrl, nil
}

// QueueNameFromARN extracts the queue name from an SQS ARN.
func QueueNameFromARN(arn string) string {
	// ARN format: arn:aws:sqs:region:account:queue-name
	parts := strings.Split(arn, ":")
	if len(parts) >= 6 {
		return parts[5]
	}
	return arn
}

func queueNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
