package notify

import (
	"context"
	"encoding/json"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/martinsaul/lost/internal/config"
)

// sqsNotifier enqueues the email as a JSON message onto an Amazon SQS queue. It
// does NOT itself talk SMTP — a separate consumer (a Lambda, a worker, another
// service) reads the queue and performs the actual send. This suits fleets that
// centralize outbound egress or need durable, retryable delivery.
//
// The JSON body matches the backend-agnostic Message so any consumer can render
// it. AWS credentials come from the standard chain (env, shared config, IAM).
type sqsNotifier struct {
	client   *sqs.Client
	queueURL string
}

// sqsPayload is the wire format placed on the queue.
type sqsPayload struct {
	Type     string `json:"type"`
	To       string `json:"to"`
	ToName   string `json:"to_name,omitempty"`
	From     string `json:"from"`
	FromName string `json:"from_name,omitempty"`
	ReplyTo  string `json:"reply_to,omitempty"`
	Subject  string `json:"subject"`
	Text     string `json:"text"`
	HTML     string `json:"html,omitempty"`
}

func newSQS(c config.SQSConfig) (Notifier, error) {
	if c.QueueURL == "" {
		return nil, fmt.Errorf("sqs notifier requires SQS_QUEUE_URL")
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(c.Region))
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return &sqsNotifier{client: sqs.NewFromConfig(cfg), queueURL: c.QueueURL}, nil
}

func (s *sqsNotifier) Name() string { return "sqs" }

func (s *sqsNotifier) Send(ctx context.Context, msg Message) error {
	body, err := json.Marshal(sqsPayload{
		Type: "email", To: msg.To, ToName: msg.ToName,
		From: msg.From, FromName: msg.FromName, ReplyTo: msg.ReplyTo,
		Subject: msg.Subject, Text: msg.Text, HTML: msg.HTML,
	})
	if err != nil {
		return err
	}
	str := string(body)
	_, err = s.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    &s.queueURL,
		MessageBody: &str,
	})
	if err != nil {
		return fmt.Errorf("sqs send: %w", err)
	}
	return nil
}
