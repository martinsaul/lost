// Package server wires the HTTP API, the public found pages, and the embedded
// React SPA into a single http.Handler.
package server

import (
	"io/fs"
	"net/http"

	"github.com/martinsaul/lost/internal/config"
	"github.com/martinsaul/lost/internal/notify"
	"github.com/martinsaul/lost/internal/store"
)

type Server struct {
	cfg      *config.Config
	store    *store.Store
	notifier notify.Notifier
	spa      fs.FS
	mux      *http.ServeMux

	foundLimiter *rateLimiter // public found-form submissions
	loginLimiter *rateLimiter // magic-link requests
}

// New builds the server. spa is the built React app (embedded dist) served for
// all non-API routes.
func New(cfg *config.Config, st *store.Store, n notify.Notifier, spa fs.FS) *Server {
	s := &Server{
		cfg:          cfg,
		store:        st,
		notifier:     n,
		spa:          spa,
		mux:          http.NewServeMux(),
		foundLimiter: newRateLimiter(0.2, 5), // ~1 per 5s, burst 5, per IP+tag
		loginLimiter: newRateLimiter(0.1, 3), // ~1 per 10s, burst 3, per email
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	// Load the session user (if any) for every request, then serve.
	return s.withUser(s.mux)
}

func (s *Server) routes() {
	// --- Auth ---
	s.mux.HandleFunc("POST /api/auth/request", s.handleAuthRequest)
	s.mux.HandleFunc("GET /api/auth/verify", s.handleAuthVerify)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/me", s.handleMe)

	// --- Tags (owner, auth required) ---
	s.mux.HandleFunc("GET /api/tags", s.requireAuth(s.handleListTags))
	s.mux.HandleFunc("POST /api/tags", s.requireAuth(s.handleCreateTag))
	s.mux.HandleFunc("GET /api/tags/{guid}", s.requireAuth(s.handleGetTag))
	s.mux.HandleFunc("PATCH /api/tags/{guid}", s.requireAuth(s.handleUpdateTag))
	s.mux.HandleFunc("DELETE /api/tags/{guid}", s.requireAuth(s.handleDeleteTag))
	s.mux.HandleFunc("GET /api/tags/{guid}/qr.svg", s.requireAuth(s.handleTagQRSVG))
	s.mux.HandleFunc("GET /api/tags/{guid}/qr.png", s.requireAuth(s.handleTagQRPNG))

	// --- Public found page ---
	s.mux.HandleFunc("GET /api/found/{guid}", s.handleGetFound)
	s.mux.HandleFunc("POST /api/found/{guid}", s.handleSubmitFound)

	// --- Public runtime config for the SPA (e.g. Turnstile site key) ---
	s.mux.HandleFunc("GET /api/config", s.handlePublicConfig)

	// --- Health ---
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// --- SPA (everything else) ---
	s.mux.HandleFunc("/", s.serveSPA)
}
