package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/martinsaul/lost/internal/config"
)

// posterboyNotifier targets a Posterboy relay
// (https://github.com/martinsaul/posterboy). Posterboy is fundamentally an SMTP
// server, so mode=smtp simply speaks SMTP to it and is the recommended default.
// mode=http POSTs a JSON
// payload to a Posterboy HTTP ingest endpoint for deployments that prefer HTTP.
type posterboyNotifier struct {
	mode string
	smtp *smtpNotifier // used when mode == "smtp"

	ingestURL string
	client    *http.Client
}

func newPosterboy(c config.PosterboyConfig) (Notifier, error) {
	switch c.Mode {
	case "", "smtp":
		s := newSMTP(config.SMTPConfig{
			Host: c.Host, Port: c.Port,
			Username: c.Username, Password: c.Password,
			StartTLS: false, // Posterboy typically runs plain on an internal network
		})
		s.name = "posterboy"
		return &posterboyNotifier{mode: "smtp", smtp: s}, nil
	case "http":
		if c.IngestURL == "" {
			return nil, fmt.Errorf("posterboy http mode requires POSTERBOY_INGEST_URL")
		}
		return &posterboyNotifier{
			mode:      "http",
			ingestURL: c.IngestURL,
			client:    &http.Client{Timeout: 15 * time.Second},
		}, nil
	default:
		return nil, fmt.Errorf("invalid POSTERBOY_MODE %q (want smtp|http)", c.Mode)
	}
}

func (p *posterboyNotifier) Name() string { return "posterboy" }

func (p *posterboyNotifier) Send(ctx context.Context, msg Message) error {
	if p.mode == "smtp" {
		return p.smtp.Send(ctx, msg)
	}
	payload := map[string]string{
		"from":      msg.From,
		"from_name": msg.FromName,
		"to":        msg.To,
		"subject":   msg.Subject,
		"text":      msg.Text,
		"html":      msg.HTML,
		"reply_to":  msg.ReplyTo,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.ingestURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("posterboy http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("posterboy http status %d: %s", resp.StatusCode, b)
	}
	return nil
}
