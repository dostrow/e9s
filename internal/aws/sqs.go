package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/dostrow/e9s/internal/model"
)

type SQSQueue = model.SQSQueue
type SQSQueueStats = model.SQSQueueStats
type SQSMessage = model.SQSMessage
type SQSSendTemplate = model.SQSSendTemplate
type SQSAttr = model.SQSAttr

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
		MessageSystemAttributeNames: []sqstypes.MessageSystemAttributeName{sqstypes.MessageSystemAttributeNameAll},
		MessageAttributeNames:       []string{"All"},
	})
	if err != nil {
		return nil, err
	}

	var messages []SQSMessage
	for _, m := range out.Messages {
		msg := SQSMessage{
			MessageID:      derefStrAws(m.MessageId),
			ReceiptHandle:  derefStrAws(m.ReceiptHandle),
			Body:           derefStrAws(m.Body),
			MD5:            derefStrAws(m.MD5OfBody),
			Attributes:     m.Attributes,
			UserAttributes: make(map[string]model.SQSMessageAttribute, len(m.MessageAttributes)),
		}
		for k, v := range m.MessageAttributes {
			msg.UserAttributes[k] = model.SQSMessageAttribute{
				DataType: derefStrAws(v.DataType), StringValue: derefStrAws(v.StringValue),
				BinaryValue: append([]byte(nil), v.BinaryValue...),
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
			attribute := sqstypes.MessageAttributeValue{DataType: &v.DataType}
			if strings.HasPrefix(v.DataType, "Binary") {
				value, err := base64.StdEncoding.DecodeString(v.Value)
				if err != nil {
					return "", fmt.Errorf("decode binary message attribute %q: %w", k, err)
				}
				attribute.BinaryValue = value
			} else {
				attribute.StringValue = &v.Value
			}
			attrs[k] = attribute
		}
		input.MessageAttributes = attrs
	}

	out, err := c.SQS.SendMessage(ctx, input)
	if err != nil {
		return "", err
	}
	return derefStrAws(out.MessageId), nil
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
