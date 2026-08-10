package notify

import (
	"fmt"
	"mime"
	"strings"
	"time"

	"github.com/google/uuid"
)

// buildRFC822 renders a Message as a MIME email. When HTML is present it emits a
// multipart/alternative body; otherwise a plain text/plain message. Used by the
// SMTP and Posterboy(SMTP) and Gmail backends.
func buildRFC822(msg Message) []byte {
	var b strings.Builder
	writeHeader := func(k, v string) {
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}

	writeHeader("From", formatAddr(msg.FromName, msg.From))
	writeHeader("To", formatAddr(msg.ToName, msg.To))
	if msg.ReplyTo != "" {
		writeHeader("Reply-To", msg.ReplyTo)
	}
	writeHeader("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	writeHeader("MIME-Version", "1.0")
	writeHeader("Date", time.Now().UTC().Format(time.RFC1123Z))
	writeHeader("Message-ID", fmt.Sprintf("<%s@lost>", uuid.NewString()))

	if msg.HTML == "" {
		writeHeader("Content-Type", "text/plain; charset=utf-8")
		b.WriteString("\r\n")
		b.WriteString(normalizeCRLF(msg.Text))
		return []byte(b.String())
	}

	boundary := "b_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	writeHeader("Content-Type", "multipart/alternative; boundary="+boundary)
	b.WriteString("\r\n")

	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString(normalizeCRLF(msg.Text))
	b.WriteString("\r\n")

	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
	b.WriteString(normalizeCRLF(msg.HTML))
	b.WriteString("\r\n")

	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
}

func formatAddr(name, addr string) string {
	if name == "" {
		return addr
	}
	return fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", name), addr)
}

func normalizeCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
