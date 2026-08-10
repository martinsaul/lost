package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// verifyTurnstile validates a Cloudflare Turnstile token server-side. Only
// called when a site key + secret are configured. hCaptcha uses the same
// request/response shape at a different endpoint, so this doubles as a template.
func (s *Server) verifyTurnstile(ctx context.Context, token, ip string) bool {
	if token == "" {
		return false
	}
	form := url.Values{}
	form.Set("secret", s.cfg.TurnstileSecret)
	form.Set("response", token)
	if ip != "" {
		form.Set("remoteip", ip)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		strings.NewReader(form.Encode()))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false
	}
	return out.Success
}
