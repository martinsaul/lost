package server

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// handlePublicConfig exposes the small slice of config the SPA needs at runtime
// (so a single build works across deployments without rebuilding).
func (s *Server) handlePublicConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"appName":          s.cfg.FromName,
		"turnstileEnabled": s.cfg.TurnstileEnabled(),
		"turnstileSiteKey": s.cfg.TurnstileSiteKey,
	})
}

// serveSPA serves the embedded React build. Real files are served directly;
// anything else falls back to index.html so client-side routing works
// (/app, /found/<guid>, etc.).
func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// Never let /api/* fall through to the SPA — a missing API route is a 404.
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if clean == "" {
		s.serveSPAFile(w, r, "index.html")
		return
	}
	if f, err := s.spa.Open(clean); err == nil {
		if st, err := f.Stat(); err == nil && !st.IsDir() {
			_ = f.Close()
			s.serveSPAFile(w, r, clean)
			return
		}
		_ = f.Close()
	}
	// Unknown path -> SPA entrypoint.
	s.serveSPAFile(w, r, "index.html")
}

func (s *Server) serveSPAFile(w http.ResponseWriter, r *http.Request, name string) {
	f, err := s.spa.Open(name)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "spa error")
		return
	}
	// Hashed asset files are immutable; index.html must not be cached.
	if name != "index.html" && strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		// Fallback: copy without range support.
		w.Header().Set("Content-Type", contentType(name))
		_, _ = io.Copy(w, f)
		return
	}
	http.ServeContent(w, r, path.Base(name), st.ModTime(), rs)
}

func contentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

var _ = fs.ErrNotExist
