package model

import "time"

// SQSQueue identifies one queue returned by ListQueues.
type SQSQueue struct {
	Name string
	URL  string
}

// SQSQueueStats contains the queue configuration and approximate counters used
// by both frontends.
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

type SQSMessageAttribute struct {
	DataType    string
	StringValue string
	BinaryValue []byte
}

type SQSMessage struct {
	MessageID      string
	ReceiptHandle  string
	Body           string
	MD5            string
	CapturedAt     time.Time
	Attributes     map[string]string
	UserAttributes map[string]SQSMessageAttribute
}

// SQSSendTemplate is the portable JSON document edited by either frontend.
// Binary attribute values are base64 strings.
type SQSSendTemplate struct {
	Body            string             `json:"body"`
	GroupID         string             `json:"groupId"`
	DeduplicationID string             `json:"deduplicationId"`
	DelaySeconds    int                `json:"delaySeconds"`
	Attributes      map[string]SQSAttr `json:"attributes"`
}

type SQSAttr struct {
	DataType string `json:"dataType"`
	Value    string `json:"value"`
}

type SQSReceiveRequest struct {
	QueueURL    string
	MaxMessages int
	WaitSeconds int
}

type SQSSendRequest struct {
	QueueURL string
	Template SQSSendTemplate
}
