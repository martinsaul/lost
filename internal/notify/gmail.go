package notify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/martinsaul/lost/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// gmailNotifier sends through the Gmail API using OAuth2. It expects an OAuth
// client credentials file and a previously-obtained token file (with a refresh
// token) on disk. Higher send limits than SMTP and no app password required.
//
// One-time setup: create a Google Cloud OAuth client, run any standard
// installed-app flow to mint a token.json with the gmail.send scope, and mount
// both files. The refresh token keeps the sender authorized indefinitely.
type gmailNotifier struct {
	svc         *gmail.Service
	senderAlias string
}

func newGmail(c config.GmailConfig) (Notifier, error) {
	credBytes, err := os.ReadFile(c.CredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("gmail credentials: %w", err)
	}
	oauthCfg, err := google.ConfigFromJSON(credBytes, gmail.GmailSendScope)
	if err != nil {
		return nil, fmt.Errorf("gmail config: %w", err)
	}
	tokBytes, err := os.ReadFile(c.TokenPath)
	if err != nil {
		return nil, fmt.Errorf("gmail token (run the OAuth flow to create %s): %w", c.TokenPath, err)
	}
	var tok oauth2.Token
	if err := json.Unmarshal(tokBytes, &tok); err != nil {
		return nil, fmt.Errorf("gmail token parse: %w", err)
	}
	ctx := context.Background()
	// TokenSource auto-refreshes using the refresh token.
	client := oauthCfg.Client(ctx, &tok)
	svc, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("gmail service: %w", err)
	}
	return &gmailNotifier{svc: svc, senderAlias: c.SenderAlias}, nil
}

func (g *gmailNotifier) Name() string { return "gmail-api" }

func (g *gmailNotifier) Send(ctx context.Context, msg Message) error {
	// Honor a configured "Send mail as" alias, else use the message's From.
	if g.senderAlias != "" {
		msg.From = g.senderAlias
	}
	raw := base64.URLEncoding.EncodeToString(buildRFC822(msg))
	_, err := g.svc.Users.Messages.Send("me", &gmail.Message{Raw: raw}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("gmail send: %w", err)
	}
	return nil
}
