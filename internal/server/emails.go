package server

import (
	"fmt"
	"html"
	"strings"

	"github.com/martinsaul/lost/internal/notify"
	"github.com/martinsaul/lost/internal/store"
)

// magicLinkMessage builds the passwordless sign-in email.
func (s *Server) magicLinkMessage(toEmail, link string) notify.Message {
	text := fmt.Sprintf(
		"Sign in to %s\n\nClick the link below to sign in. It expires shortly and can be used once:\n\n%s\n\nIf you didn't request this, you can ignore this email.\n",
		s.appName(), link)
	body := fmt.Sprintf(`<p>Click below to sign in to <strong>%s</strong>. This link expires shortly and can be used once.</p>
<p><a href="%s" style="display:inline-block;padding:10px 18px;background:#111;color:#fff;text-decoration:none;border-radius:6px">Sign in</a></p>
<p style="color:#666;font-size:13px">Or paste this URL: %s</p>
<p style="color:#999;font-size:12px">If you didn't request this, ignore this email.</p>`,
		html.EscapeString(s.appName()), html.EscapeString(link), html.EscapeString(link))
	return notify.Message{
		To:       toEmail,
		From:     s.cfg.FromAddress,
		FromName: s.cfg.FromName,
		Subject:  "Sign in to " + s.appName(),
		Text:     text,
		HTML:     htmlWrap(body),
	}
}

// foundReportMessage builds the notification sent to a tag's owner when a finder
// submits the contact form. The owner's address is never exposed to the finder;
// this email flows only to the owner.
func (s *Server) foundReportMessage(owner *store.User, tag *store.Tag, r *store.FoundReport) notify.Message {
	tagName := tag.Name
	if tagName == "" {
		tagName = "your item"
	}
	var contact strings.Builder
	if r.FinderName != "" {
		fmt.Fprintf(&contact, "Name: %s\n", r.FinderName)
	}
	if r.FinderEmail != "" {
		fmt.Fprintf(&contact, "Email: %s\n", r.FinderEmail)
	}
	if r.FinderPhone != "" {
		fmt.Fprintf(&contact, "Phone: %s\n", r.FinderPhone)
	}
	contactStr := contact.String()
	if contactStr == "" {
		contactStr = "(the finder left no contact details)\n"
	}

	text := fmt.Sprintf(
		"Someone found \"%s\"!\n\nThey left this message:\n\n%s\n\nHow to reach them:\n%s\n",
		tagName, r.Message, contactStr)

	replyTo := r.FinderEmail // lets the owner reply straight to the finder

	body := fmt.Sprintf(`<p>Someone scanned the QR code on <strong>%s</strong> and reached out.</p>
<blockquote style="margin:12px 0;padding:10px 14px;border-left:3px solid #ddd;color:#333">%s</blockquote>
<p><strong>How to reach them:</strong><br>%s</p>`,
		html.EscapeString(tagName),
		html.EscapeString(r.Message),
		strings.ReplaceAll(html.EscapeString(contactStr), "\n", "<br>"))

	return notify.Message{
		To:       owner.Email,
		From:     s.cfg.FromAddress,
		FromName: s.cfg.FromName,
		ReplyTo:  replyTo,
		Subject:  fmt.Sprintf("Someone found \"%s\"", tagName),
		Text:     text,
		HTML:     htmlWrap(body),
	}
}

func (s *Server) appName() string { return s.cfg.FromName }

func htmlWrap(inner string) string {
	return `<!doctype html><html><body style="font-family:-apple-system,Segoe UI,Roboto,sans-serif;max-width:520px;margin:0 auto;padding:16px;color:#111">` +
		inner + `</body></html>`
}
