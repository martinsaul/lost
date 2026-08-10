package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/martinsaul/lost/internal/store"
)

// foundPublicDTO is what an anonymous finder sees. It deliberately exposes only
// opted-in owner contact fields, never the owner's identity otherwise.
type foundPublicDTO struct {
	Name       string `json:"name"`                 // the tag's friendly label (may be empty)
	OwnerEmail string `json:"ownerEmail,omitempty"` // only if owner opted in
	OwnerPhone string `json:"ownerPhone,omitempty"` // only if owner opted in
}

func (s *Server) handleGetFound(w http.ResponseWriter, r *http.Request) {
	guid := r.PathValue("guid")
	tag, err := s.store.TagByGUID(guid)
	if err != nil {
		writeErr(w, http.StatusNotFound, "unknown code")
		return
	}
	dto := foundPublicDTO{Name: tag.Name}
	if tag.ShowEmail {
		if owner, err := s.store.UserByID(tag.UserID); err == nil {
			dto.OwnerEmail = owner.Email
		}
	}
	if tag.ShowPhone && tag.Phone != "" {
		dto.OwnerPhone = tag.Phone
	}
	writeJSON(w, http.StatusOK, dto)
}

type foundSubmitBody struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Website string `json:"website"` // honeypot: real users never fill this
	Token   string `json:"token"`   // Turnstile/CAPTCHA token (optional)
}

// handleSubmitFound accepts a finder's contact message and notifies the owner.
// The owner's identity is never returned to the finder.
func (s *Server) handleSubmitFound(w http.ResponseWriter, r *http.Request) {
	guid := r.PathValue("guid")

	var body foundSubmitBody
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request")
		return
	}

	// Silently accept honeypot hits so bots get no signal.
	if strings.TrimSpace(body.Website) != "" {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	tag, err := s.store.TagByGUID(guid)
	if err != nil {
		writeErr(w, http.StatusNotFound, "unknown code")
		return
	}

	// Rate limit per IP+tag so one finder can't spam an owner.
	ip := clientIP(r)
	if !s.foundLimiter.Allow(ip + "|" + guid) {
		writeErr(w, http.StatusTooManyRequests, "please wait a moment before sending again")
		return
	}

	if s.cfg.TurnstileEnabled() {
		if !s.verifyTurnstile(r.Context(), body.Token, ip) {
			writeErr(w, http.StatusForbidden, "captcha verification failed")
			return
		}
	}

	message := strings.TrimSpace(body.Message)
	if message == "" {
		writeErr(w, http.StatusBadRequest, "please include a message")
		return
	}
	report := &store.FoundReport{
		TagID:       tag.ID,
		FinderName:  trimTo(body.Name, 200),
		FinderEmail: trimTo(strings.TrimSpace(body.Email), 320),
		FinderPhone: trimTo(strings.TrimSpace(body.Phone), 64),
		Message:     trimTo(message, 4000),
		RemoteIP:    ip,
	}
	if err := s.store.CreateFoundReport(report); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not record message")
		return
	}

	owner, err := s.store.UserByID(tag.UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not reach the owner")
		return
	}
	// Deliver off the request path; the finder shouldn't wait on SMTP.
	go func() {
		ctx, cancel := contextWithTimeout(20 * time.Second)
		defer cancel()
		_ = s.notifier.Send(ctx, s.foundReportMessage(owner, tag, report))
	}()

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func trimTo(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
