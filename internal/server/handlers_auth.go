package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/martinsaul/lost/internal/auth"
	"github.com/martinsaul/lost/internal/store"
)

type authRequestBody struct {
	Email string `json:"email"`
}

// handleAuthRequest issues a magic link. To avoid leaking which emails have
// accounts, it always responds 200 regardless of outcome; the link is only
// actually sent for a syntactically valid address that passes rate limiting.
func (s *Server) handleAuthRequest(w http.ResponseWriter, r *http.Request) {
	var body authRequestBody
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	email := normalizeEmail(body.Email)
	ok := validEmail(email)

	// Always answer the same way; do the work only for a plausible address.
	if ok && s.loginLimiter.Allow(email) {
		go s.sendMagicLink(email)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// sendMagicLink mints a single-use token, stores its hash, and emails the link.
// Runs off the request goroutine so timing doesn't reveal account existence.
func (s *Server) sendMagicLink(email string) {
	raw, hash := auth.NewToken()
	if err := s.store.CreateLoginToken(hash, email, s.cfg.MagicLinkTTL); err != nil {
		return
	}
	link := s.cfg.BaseURL + "/api/auth/verify?token=" + url.QueryEscape(raw)
	msg := s.magicLinkMessage(email, link)
	ctx, cancel := contextWithTimeout(20 * time.Second)
	defer cancel()
	_ = s.notifier.Send(ctx, msg)
}

// handleAuthVerify consumes the token from the emailed link, creates a session,
// sets the cookie, and redirects into the app.
func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("token")
	if raw == "" {
		s.redirectAuthError(w, r, "missing token")
		return
	}
	email, err := s.store.ConsumeLoginToken(auth.HashToken(raw))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.redirectAuthError(w, r, "invalid link")
		} else {
			s.redirectAuthError(w, r, "expired or already-used link")
		}
		return
	}
	user, err := s.store.UpsertUser(email)
	if err != nil {
		s.redirectAuthError(w, r, "could not sign in")
		return
	}
	sid := auth.NewSessionID()
	if err := s.store.CreateSession(sid, user.ID, s.cfg.SessionTTL); err != nil {
		s.redirectAuthError(w, r, "could not create session")
		return
	}
	s.setSessionCookie(w, sid)
	http.Redirect(w, r, "/app", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.SessionCookieName); err == nil && c.Value != "" {
		_ = s.store.DeleteSession(c.Value)
	}
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "not signed in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"email": u.Email})
}

func (s *Server) redirectAuthError(w http.ResponseWriter, r *http.Request, reason string) {
	http.Redirect(w, r, "/?auth_error="+url.QueryEscape(reason), http.StatusFound)
}

// ---- cookie + validation helpers ----

func (s *Server) setSessionCookie(w http.ResponseWriter, sid string) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secureCookies(),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(s.cfg.SessionTTL),
		MaxAge:   int(s.cfg.SessionTTL / time.Second),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secureCookies(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Server) secureCookies() bool {
	return strings.HasPrefix(s.cfg.BaseURL, "https://")
}

func normalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}

// validEmail is a deliberately permissive sanity check — the real proof is that
// the recipient can click the emailed link.
func validEmail(e string) bool {
	at := strings.IndexByte(e, '@')
	if at <= 0 || at == len(e)-1 {
		return false
	}
	if strings.Contains(e, " ") || strings.Count(e, "@") != 1 {
		return false
	}
	return strings.Contains(e[at+1:], ".")
}
