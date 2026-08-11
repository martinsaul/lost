// Package geo resolves an IP address to an approximate location. It is pluggable
// so self-hosters can choose: "none" (default, no lookups), "ipapi" (the free
// ip-api.com service — no key, but sends the IP to a third party), or "maxmind"
// (a local MaxMind GeoLite2 City database — fully offline, privacy-preserving).
package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
)

// Info is an approximate, IP-derived location. OK is false when unknown.
type Info struct {
	OK      bool
	Country string
	Region  string
	City    string
	Lat     float64
	Lon     float64
}

// Provider resolves an IP to an Info. Implementations must be concurrency-safe
// and must never block a request for long (callers pass a short-timeout context).
type Provider interface {
	Lookup(ctx context.Context, ip string) Info
}

// New builds the configured provider. dbPath is only used for "maxmind".
func New(provider, dbPath string) (Provider, error) {
	switch provider {
	case "", "none":
		return noneProvider{}, nil
	case "ipapi":
		return newCached(&ipapiProvider{client: &http.Client{Timeout: 4 * time.Second}}), nil
	case "maxmind":
		if dbPath == "" {
			return nil, fmt.Errorf("geo provider maxmind requires LOST_GEOIP_DB (path to a GeoLite2 City .mmdb)")
		}
		db, err := geoip2.Open(dbPath)
		if err != nil {
			return nil, fmt.Errorf("open geoip db: %w", err)
		}
		return newCached(&maxmindProvider{db: db}), nil
	default:
		return nil, fmt.Errorf("unknown LOST_GEO_PROVIDER %q (want none|ipapi|maxmind)", provider)
	}
}

// skip reports whether an IP should not be looked up (unparseable or non-public).
func skip(ip string) bool {
	p := net.ParseIP(ip)
	if p == nil {
		return true
	}
	return p.IsLoopback() || p.IsPrivate() || p.IsLinkLocalUnicast() || p.IsUnspecified()
}

type noneProvider struct{}

func (noneProvider) Lookup(context.Context, string) Info { return Info{} }

// ---- ip-api.com ----

type ipapiProvider struct{ client *http.Client }

func (p *ipapiProvider) Lookup(ctx context.Context, ip string) Info {
	if skip(ip) {
		return Info{}
	}
	u := "http://ip-api.com/json/" + url.PathEscape(ip) + "?fields=status,country,regionName,city,lat,lon"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Info{}
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return Info{}
	}
	defer resp.Body.Close()
	var out struct {
		Status     string  `json:"status"`
		Country    string  `json:"country"`
		RegionName string  `json:"regionName"`
		City       string  `json:"city"`
		Lat        float64 `json:"lat"`
		Lon        float64 `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Status != "success" {
		return Info{}
	}
	return Info{OK: true, Country: out.Country, Region: out.RegionName, City: out.City, Lat: out.Lat, Lon: out.Lon}
}

// ---- MaxMind GeoLite2 (local, offline) ----

type maxmindProvider struct{ db *geoip2.Reader }

func (p *maxmindProvider) Lookup(_ context.Context, ip string) Info {
	if skip(ip) {
		return Info{}
	}
	rec, err := p.db.City(net.ParseIP(ip))
	if err != nil || rec == nil {
		return Info{}
	}
	region := ""
	if len(rec.Subdivisions) > 0 {
		region = rec.Subdivisions[0].Names["en"]
	}
	info := Info{
		OK:      true,
		Country: rec.Country.Names["en"],
		Region:  region,
		City:    rec.City.Names["en"],
		Lat:     rec.Location.Latitude,
		Lon:     rec.Location.Longitude,
	}
	if info.Country == "" && info.City == "" && info.Lat == 0 && info.Lon == 0 {
		return Info{}
	}
	return info
}

// ---- caching wrapper ----

// cached memoizes lookups per IP for a TTL, so repeat scans (and ip-api's rate
// limit) are handled gracefully.
type cached struct {
	inner Provider
	ttl   time.Duration
	mu    sync.Mutex
	m     map[string]entry
}

type entry struct {
	info Info
	at   time.Time
}

func newCached(inner Provider) *cached {
	return &cached{inner: inner, ttl: 6 * time.Hour, m: make(map[string]entry)}
}

func (c *cached) Lookup(ctx context.Context, ip string) Info {
	now := time.Now()
	c.mu.Lock()
	if e, ok := c.m[ip]; ok && now.Sub(e.at) < c.ttl {
		c.mu.Unlock()
		return e.info
	}
	c.mu.Unlock()

	info := c.inner.Lookup(ctx, ip)

	c.mu.Lock()
	// Bound the map so a flood of distinct IPs can't grow it unbounded.
	if len(c.m) > 10000 {
		c.m = make(map[string]entry)
	}
	c.m[ip] = entry{info: info, at: now}
	c.mu.Unlock()
	return info
}
