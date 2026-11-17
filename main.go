// Command lost is a self-hostable QR lost-and-found service. A single static
// binary serves the JSON API, the public found pages, and the embedded React
// SPA; it persists to SQLite or Postgres and delivers mail through a pluggable
// notifier (smtp, posterboy, gmail-api, sqs).
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/martinsaul/lost/internal/config"
	"github.com/martinsaul/lost/internal/notify"
	"github.com/martinsaul/lost/internal/server"
	"github.com/martinsaul/lost/internal/store"
)

//go:embed all:web/dist
var embeddedSPA embed.FS

func main() {
	// `lost healthcheck` probes the local /healthz endpoint and exits 0/1. It is
	// used by the Dockerfile HEALTHCHECK: the distroless runtime image has no
	// shell, curl or wget, so the binary checks itself. It deliberately runs
	// before config.Load so it needs no secrets.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	st, err := store.Open(cfg.DBURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()
	log.Printf("store: %s", st.Dialect())

	notifier, err := notify.New(cfg)
	if err != nil {
		log.Fatalf("notifier: %v", err)
	}
	log.Printf("notifier: %s", notifier.Name())

	spa, err := fs.Sub(embeddedSPA, "web/dist")
	if err != nil {
		log.Fatalf("spa: %v", err)
	}

	srv := server.New(cfg, st, notifier, spa)
	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (base url %s)", cfg.Addr, cfg.BaseURL)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Printf("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

// healthcheck GETs /healthz on the loopback interface, using the port from
// LOST_ADDR (default :8080). Returns a process exit code.
func healthcheck() int {
	addr := os.Getenv("LOST_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		log.Printf("healthcheck: bad LOST_ADDR %q: %v", addr, err)
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	url := "http://" + net.JoinHostPort(strings.Trim(host, "[]"), port) + "/healthz"
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("healthcheck: %v", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("healthcheck: %s -> %d", url, resp.StatusCode)
		return 1
	}
	return 0
}
