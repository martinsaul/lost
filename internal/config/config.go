package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config is the fully resolved runtime configuration, sourced entirely from
// environment variables so the app is portable across hosts and self-hosters.
type Config struct {
	BaseURL       string // public origin, e.g. https://lost.example.com
	Addr          string // listen address, e.g. :8080
	DBURL         string // sqlite:///data/lost.db  OR  postgres://user:pass@host/db
	SessionSecret string // HMAC key for signing cookies/tokens
	FromAddress   string // envelope + header From for outbound mail
	FromName      string // display name for outbound mail

	MagicLinkTTL time.Duration
	SessionTTL   time.Duration

	// Notifier selection + backends
	Notifier  string // smtp | posterboy | gmail-api | sqs
	SMTP      SMTPConfig
	Posterboy PosterboyConfig
	Gmail     GmailConfig
	SQS       SQSConfig

	// Optional bot deterrent on the public form
	TurnstileSiteKey string
	TurnstileSecret  string
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	StartTLS bool
}

// PosterboyConfig targets a Posterboy relay. Posterboy speaks SMTP (default) on
// its listen port; mode=http posts JSON to an ingest endpoint instead.
type PosterboyConfig struct {
	Mode      string // smtp | http
	Host      string
	Port      int
	Username  string
	Password  string
	IngestURL string
}

type GmailConfig struct {
	CredentialsPath string // OAuth client credentials json
	TokenPath       string // stored refresh token json
	SenderAlias     string // "Send mail as" alias
}

type SQSConfig struct {
	QueueURL string
	Region   string
	// Credentials come from the standard AWS chain (env, shared config, IAM role).
}

func Load() (*Config, error) {
	c := &Config{
		BaseURL:       env("LOST_BASE_URL", "http://localhost:8080"),
		Addr:          env("LOST_ADDR", ":8080"),
		DBURL:         env("LOST_DB_URL", "sqlite://./data/lost.db"),
		SessionSecret: os.Getenv("LOST_SESSION_SECRET"),
		FromAddress:   env("LOST_FROM_ADDRESS", "lost@localhost"),
		FromName:      env("LOST_FROM_NAME", "Lost & Found"),
		MagicLinkTTL:  envDuration("LOST_MAGIC_LINK_TTL", 15*time.Minute),
		SessionTTL:    envDuration("LOST_SESSION_TTL", 30*24*time.Hour),
		Notifier:      strings.ToLower(env("LOST_NOTIFIER", "smtp")),
		SMTP: SMTPConfig{
			Host:     env("SMTP_HOST", "localhost"),
			Port:     envInt("SMTP_PORT", 587),
			Username: os.Getenv("SMTP_USERNAME"),
			Password: os.Getenv("SMTP_PASSWORD"),
			StartTLS: envBool("SMTP_STARTTLS", true),
		},
		Posterboy: PosterboyConfig{
			Mode:      strings.ToLower(env("POSTERBOY_MODE", "smtp")),
			Host:      env("POSTERBOY_HOST", "posterboy"),
			Port:      envInt("POSTERBOY_PORT", 2525),
			Username:  os.Getenv("POSTERBOY_USERNAME"),
			Password:  os.Getenv("POSTERBOY_PASSWORD"),
			IngestURL: os.Getenv("POSTERBOY_INGEST_URL"),
		},
		Gmail: GmailConfig{
			CredentialsPath: env("GMAIL_CREDENTIALS_PATH", "/data/gmail/credentials.json"),
			TokenPath:       env("GMAIL_TOKEN_PATH", "/data/gmail/token.json"),
			SenderAlias:     os.Getenv("GMAIL_SENDER_ALIAS"),
		},
		SQS: SQSConfig{
			QueueURL: os.Getenv("SQS_QUEUE_URL"),
			Region:   env("AWS_REGION", "us-east-1"),
		},
		TurnstileSiteKey: os.Getenv("LOST_TURNSTILE_SITE_KEY"),
		TurnstileSecret:  os.Getenv("LOST_TURNSTILE_SECRET"),
	}

	if c.SessionSecret == "" {
		return nil, fmt.Errorf("LOST_SESSION_SECRET is required (a long random string)")
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	return c, nil
}

// TurnstileEnabled reports whether CAPTCHA verification should run.
func (c *Config) TurnstileEnabled() bool {
	return c.TurnstileSiteKey != "" && c.TurnstileSecret != ""
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func envBool(k string, def bool) bool {
	switch strings.ToLower(os.Getenv(k)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

func envDuration(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
