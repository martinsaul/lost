package server

import (
	"fmt"
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

	// Give the finder a stable id so re-report throttling survives across submits,
	// and record the scan's connection metadata (geo resolved best-effort).
	s.finderKeyOrSet(w, r)
	go s.recordScan(tag.ID, clientIP(r), userAgent(r))

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

// recordScan resolves the IP's approximate location and stores a scan event.
// Runs off the request goroutine — the finder never waits on a geo lookup.
func (s *Server) recordScan(tagID, ip, ua string) {
	ctx, cancel := contextWithTimeout(5 * time.Second)
	defer cancel()
	g := s.geo.Lookup(ctx, ip)
	_ = s.store.CreateScanEvent(&store.ScanEvent{
		TagID: tagID, RemoteIP: ip, UserAgent: ua,
		HasGeo: g.OK, GeoCountry: g.Country, GeoRegion: g.Region, GeoCity: g.City, GeoLat: g.Lat, GeoLon: g.Lon,
	})
}

type foundSubmitBody struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Website string `json:"website"` // honeypot: real users never fill this
	Token   string `json:"token"`   // Turnstile/CAPTCHA token (optional)

	// Precise location, only sent when the finder explicitly consents in the UI.
	HasLocation bool    `json:"hasLocation"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Accuracy    float64 `json:"accuracy"`
}

// handleSubmitFound accepts a finder's contact message and notifies the owner.
// The owner's identity is never returned to the finder. A finder may send again
// (to update their info) subject to a per-finder throttle.
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

	ip := clientIP(r)
	ua := userAgent(r)
	finderKey := s.finderKeyOrSet(w, r)

	// Coarse burst guard against rapid-fire abuse.
	if !s.foundLimiter.Allow(ip + "|" + guid) {
		writeErr(w, http.StatusTooManyRequests, "please wait a moment before sending again")
		return
	}

	// Re-report throttle: at most one per FinderMinInterval, and FinderDailyCap
	// per rolling 24h, per finder (cookie, falling back to IP).
	now := time.Now().UTC()
	if last, count24h, err := s.store.FinderReportStats(tag.ID, finderKey, ip, now.Add(-24*time.Hour)); err == nil {
		if s.cfg.FinderDailyCap > 0 && count24h >= s.cfg.FinderDailyCap {
			writeErr(w, http.StatusTooManyRequests, "daily message limit reached — please try again tomorrow")
			return
		}
		if !last.IsZero() && now.Sub(last) < s.cfg.FinderMinInterval {
			wait := s.cfg.FinderMinInterval - now.Sub(last)
			writeErr(w, http.StatusTooManyRequests,
				"you can send another update in about "+humanizeDuration(wait))
			return
		}
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

	// Resolve the finder's approximate location from their IP.
	geoCtx, cancel := contextWithTimeout(4 * time.Second)
	g := s.geo.Lookup(geoCtx, ip)
	cancel()

	report := &store.FoundReport{
		TagID:       tag.ID,
		FinderName:  trimTo(body.Name, 200),
		FinderEmail: trimTo(strings.TrimSpace(body.Email), 320),
		FinderPhone: trimTo(strings.TrimSpace(body.Phone), 64),
		Message:     trimTo(message, 4000),
		RemoteIP:    ip,
		UserAgent:   ua,
		FinderKey:   finderKey,
		HasGeo:      g.OK,
		GeoCountry:  g.Country,
		GeoRegion:   g.Region,
		GeoCity:     g.City,
		GeoLat:      g.Lat,
		GeoLon:      g.Lon,
	}
	// Precise, consented location enriches the report if valid.
	if body.HasLocation && validCoord(body.Lat, body.Lon) {
		report.HasPrecise = true
		report.PreciseLat = body.Lat
		report.PreciseLon = body.Lon
		report.PreciseAccuracy = body.Accuracy
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

// validCoord sanity-checks browser-supplied coordinates.
func validCoord(lat, lon float64) bool {
	return lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 && !(lat == 0 && lon == 0)
}

// humanizeDuration renders a short, friendly "time remaining" string.
func humanizeDuration(d time.Duration) string {
	if d < time.Minute {
		return "a minute"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		return fmt.Sprintf("%d minute%s", m, plural(m))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%d hour%s", h, plural(h))
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
