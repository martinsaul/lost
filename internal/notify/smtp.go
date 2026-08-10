package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"

	"github.com/martinsaul/lost/internal/config"
)

// smtpNotifier delivers via a plain SMTP server with optional STARTTLS and AUTH.
// This is the baseline backend and also the transport used by the Posterboy
// backend in its default (smtp) mode.
type smtpNotifier struct {
	host     string
	port     int
	username string
	password string
	startTLS bool
	name     string
}

func newSMTP(c config.SMTPConfig) *smtpNotifier {
	return &smtpNotifier{
		host: c.Host, port: c.Port, username: c.Username,
		password: c.Password, startTLS: c.StartTLS, name: "smtp",
	}
}

func (s *smtpNotifier) Name() string { return s.name }

func (s *smtpNotifier) Send(ctx context.Context, msg Message) error {
	addr := net.JoinHostPort(s.host, fmt.Sprintf("%d", s.port))
	d := net.Dialer{Timeout: 15 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("%s dial: %w", s.name, err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("%s client: %w", s.name, err)
	}
	defer c.Close()

	if s.startTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.host}); err != nil {
				return fmt.Errorf("%s starttls: %w", s.name, err)
			}
		}
	}
	if s.username != "" {
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("%s auth: %w", s.name, err)
		}
	}
	if err := c.Mail(msg.From); err != nil {
		return fmt.Errorf("%s MAIL FROM: %w", s.name, err)
	}
	if err := c.Rcpt(msg.To); err != nil {
		return fmt.Errorf("%s RCPT TO: %w", s.name, err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("%s DATA: %w", s.name, err)
	}
	if _, err := w.Write(buildRFC822(msg)); err != nil {
		return fmt.Errorf("%s write: %w", s.name, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("%s close data: %w", s.name, err)
	}
	return c.Quit()
}
