package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/martinsaul/lost/internal/auth"
	"github.com/martinsaul/lost/internal/store"
)

// finderCookieName carries an opaque per-finder id used to throttle re-reports
// even when a finder is behind a shared IP.
const finderCookieName = "lost_finder"

type ctxKey int

const userKey ctxKey = iota

// withUser resolves the session cookie to a user and stashes it in the request
// context. Absence of a session is not an error here — requireAuth enforces it
// where needed.
func (s *Server) withUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(auth.SessionCookieName); err == nil && c.Value != "" {
			if u, err := s.store.SessionUser(c.Value); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), userKey, u))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func currentUser(r *http.Request) *store.User {
	u, _ := r.Context().Value(userKey).(*store.User)
	return u
}

// requireAuth wraps a handler, returning 401 when there is no live session.
func (s *Server) requireAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r) == nil {
			writeErr(w, http.StatusUnauthorized, "authentication required")
			return
		}
		h(w, r)
	}
}

// requireAdmin wraps a handler, returning 401 when unauthenticated and 403 when
// the signed-in user's email is not on the admin allowlist.
func (s *Server) requireAdmin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := currentUser(r)
		if u == nil {
			writeErr(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if !s.cfg.IsAdmin(u.Email) {
			writeErr(w, http.StatusForbidden, "admin access required")
			return
		}
		h(w, r)
	}
}

// finderKeyOrSet returns the finder's opaque id from the cookie, minting and
// setting one when absent.
func (s *Server) finderKeyOrSet(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(finderCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	id := auth.NewSessionID()
	http.SetCookie(w, &http.Cookie{
		Name:     finderCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secureCookies(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((30 * 24 * time.Hour) / time.Second),
	})
	return id
}

// readFinderKey returns the finder cookie value, or "" if absent.
func readFinderKey(r *http.Request) string {
	if c, err := r.Cookie(finderCookieName); err == nil {
		return c.Value
	}
	return ""
}

// userAgent returns a length-bounded User-Agent header.
func userAgent(r *http.Request) string {
	return trimTo(r.Header.Get("User-Agent"), 400)
}

// ---- JSON helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decodeJSON reads a small JSON body into v, rejecting oversized payloads.
func decodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)) // 64 KiB cap
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// clientIP extracts the best-guess client address, honoring a single hop of
// X-Forwarded-For (reverse proxies set it). Only used for rate-limiting and
// audit, never for trust decisions.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
