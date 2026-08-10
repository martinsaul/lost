// Package notify is the pluggable outbound-delivery layer. One Notifier
// interface, selected at startup by config, is used for BOTH magic-link login
// emails and found-report notifications. Backends: smtp, posterboy, gmail-api,
// sqs.
package notify

import (
	"context"
	"fmt"

	"github.com/martinsaul/lost/internal/config"
)

// Message is a backend-agnostic email to send.
type Message struct {
	To       string // recipient address
	ToName   string // recipient display name (optional)
	From     string // sender address
	FromName string // sender display name
	ReplyTo  string // optional Reply-To (e.g. a finder's address)
	Subject  string
	Text     string // plaintext body (required)
	HTML     string // html body (optional)
}

// Notifier delivers a Message. Implementations must be safe for concurrent use.
type Notifier interface {
	Name() string
	Send(ctx context.Context, msg Message) error
}

// New constructs the configured notifier. It fails fast on misconfiguration so
// a broken deploy surfaces at startup, not on the first lost bag.
func New(cfg *config.Config) (Notifier, error) {
	switch cfg.Notifier {
	case "smtp":
		return newSMTP(cfg.SMTP), nil
	case "posterboy":
		return newPosterboy(cfg.Posterboy)
	case "gmail-api", "gmail":
		return newGmail(cfg.Gmail)
	case "sqs":
		return newSQS(cfg.SQS)
	default:
		return nil, fmt.Errorf("unknown LOST_NOTIFIER %q (want smtp|posterboy|gmail-api|sqs)", cfg.Notifier)
	}
}
