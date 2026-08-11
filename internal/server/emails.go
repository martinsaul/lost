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

	locText, locHTML := scanMetadata(r)

	text := fmt.Sprintf(
		"Someone found \"%s\"!\n\nThey left this message:\n\n%s\n\nHow to reach them:\n%s\n%s",
		tagName, r.Message, contactStr, locText)

	replyTo := r.FinderEmail // lets the owner reply straight to the finder

	body := fmt.Sprintf(`<p>Someone scanned the QR code on <strong>%s</strong> and reached out.</p>
<blockquote style="margin:12px 0;padding:10px 14px;border-left:3px solid #ddd;color:#333">%s</blockquote>
<p><strong>How to reach them:</strong><br>%s</p>%s`,
		html.EscapeString(tagName),
		html.EscapeString(r.Message),
		strings.ReplaceAll(html.EscapeString(contactStr), "\n", "<br>"),
		locHTML)

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

// scanMetadata renders the connection/location details attached to a report,
// for the owner-only notification email (plain text + an HTML block).
func scanMetadata(r *store.FoundReport) (text, htmlBlock string) {
	var t strings.Builder
	var h strings.Builder
	t.WriteString("\nWhere it was scanned:\n")
	h.WriteString(`<p style="margin-top:16px"><strong>Where it was scanned</strong></p><ul style="color:#333;font-size:14px">`)

	if r.RemoteIP != "" {
		fmt.Fprintf(&t, "- IP address: %s\n", r.RemoteIP)
		fmt.Fprintf(&h, "<li>IP address: %s</li>", html.EscapeString(r.RemoteIP))
	}
	if r.HasGeo {
		place := joinNonEmpty([]string{r.GeoCity, r.GeoRegion, r.GeoCountry}, ", ")
		maps := mapsLink(r.GeoLat, r.GeoLon)
		switch {
		case place != "":
			fmt.Fprintf(&t, "- Approx. location (from IP): %s  %s\n", place, maps)
			fmt.Fprintf(&h, `<li>Approx. location (from IP): %s — <a href="%s">map</a></li>`, html.EscapeString(place), maps)
		case r.GeoLat != 0 || r.GeoLon != 0:
			fmt.Fprintf(&t, "- Approx. location (from IP): %s\n", maps)
			fmt.Fprintf(&h, `<li>Approx. location (from IP): <a href="%s">map</a></li>`, maps)
		}
	}
	if r.HasPrecise {
		maps := mapsLink(r.PreciseLat, r.PreciseLon)
		acc := ""
		if r.PreciseAccuracy > 0 {
			acc = fmt.Sprintf(" (±%dm)", int(r.PreciseAccuracy))
		}
		fmt.Fprintf(&t, "- Precise location (finder shared): %.5f, %.5f%s  %s\n", r.PreciseLat, r.PreciseLon, acc, maps)
		fmt.Fprintf(&h, `<li><strong>Precise location (finder shared)</strong>: %.5f, %.5f%s — <a href="%s">map</a></li>`,
			r.PreciseLat, r.PreciseLon, html.EscapeString(acc), maps)
	}
	h.WriteString("</ul>")
	return t.String(), h.String()
}

func mapsLink(lat, lon float64) string {
	return fmt.Sprintf("https://www.google.com/maps?q=%.6f,%.6f", lat, lon)
}

func joinNonEmpty(parts []string, sep string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

func (s *Server) appName() string { return s.cfg.FromName }

func htmlWrap(inner string) string {
	return `<!doctype html><html><body style="font-family:-apple-system,Segoe UI,Roboto,sans-serif;max-width:520px;margin:0 auto;padding:16px;color:#111">` +
		inner + `</body></html>`
}
