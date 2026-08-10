# Deploying `lost`

`lost` ships as a single Docker image (multi-stage build: React SPA → embedded in
a static Go binary). Any Docker host with a reverse proxy in front works.

## Quick start

```bash
cp .env.example .env
# Set at least LOST_SESSION_SECRET (openssl rand -base64 48), LOST_BASE_URL,
# and the notifier settings.
docker compose up -d            # SQLite under ./data
# or: docker compose --profile postgres up -d
```

The image builds the SPA and the Go binary inside Docker, so the host needs only
Docker — no Go or Node toolchain.

## Behind a reverse proxy

Put a TLS-terminating reverse proxy (nginx, Caddy, …) in front of the published
port `8080`, and point `LOST_BASE_URL` at your public HTTPS origin. The
app:

- reads the real client IP from a single `X-Forwarded-For` hop (used only for
  rate-limiting and audit), so make sure your proxy sets it;
- marks session cookies `Secure` whenever `LOST_BASE_URL` is `https://`;
- serves the whole app (API, public `/found/<guid>` pages, and the SPA) on one
  port, so a single proxy route to `:8080` is all that's needed.

## Notifier

Pick a delivery backend with `LOST_NOTIFIER` (`smtp` | `posterboy` | `gmail-api`
| `sqs`) and set its variables in `.env` — see [`.env.example`](.env.example) and
the [README](README.md#notifiers). Both sign-in links and found-report emails go
through the configured notifier.
