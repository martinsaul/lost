# lost

A self-hostable **QR lost-and-found**. Stick a QR code on your luggage, laptop,
keys, or bike. If someone finds it, they scan it, land on a private page, and can
message you — **without your email or phone being exposed** (unless you choose to
share them).

- Passwordless accounts (magic-link sign-in — no passwords, ever).
- Create named tags ("Martin's large luggage") and download a **high-resolution
  QR image** (vector SVG for any print size, plus PNG for thermal printers).
- Public `/{base}/found/<guid>` page with a contact form that never reveals the
  owner — or optionally shows opted-in email/phone for faster recovery.
- **Pluggable mail delivery**: plain SMTP, [Posterboy](https://github.com/martinsaul/posterboy), the
  Gmail API, or Amazon SQS.
- Single static Go binary with an embedded React SPA. **SQLite or Postgres.**
- Fully domain-configurable — run your own instance under any domain.

Live demo: <https://lost.zenithnetwork.com>.

## How it works

```
  sticker QR  ──scan──▶  /found/<guid>  ──form──▶  notifier  ──▶  owner's inbox
   (unguessable UUID)      (public page)          (smtp/posterboy/gmail/sqs)
```

1. Owner signs in by email (magic link), creates a tag, prints its QR.
2. A finder scans the sticker and sends a message through the form.
3. The owner is notified via the configured backend, with the finder's contact
   details set as `Reply-To` so they can respond directly.

## Quick start (Docker)

```bash
git clone https://github.com/martinsaul/lost && cd lost
cp .env.example .env
# Edit .env — at minimum set LOST_SESSION_SECRET (openssl rand -base64 48)
# and your notifier settings.
docker compose up -d          # SQLite, single file under ./data
```

Point your reverse proxy / DNS at the container's port 8080. For Postgres:

```bash
# In .env: LOST_DB_URL=postgres://lost:lost@postgres:5432/lost?sslmode=disable
docker compose --profile postgres up -d
```

## Run from source

Requires Go 1.24+ and Node 20+.

```bash
npm --prefix web install
npm --prefix web run build        # builds the SPA into web/dist (embedded)
go build -o lost .

LOST_SESSION_SECRET=dev-secret \
LOST_BASE_URL=http://localhost:8080 \
LOST_DB_URL=sqlite://./data/lost.db \
LOST_NOTIFIER=smtp SMTP_HOST=localhost SMTP_PORT=1025 \
./lost
```

Frontend dev server with hot reload (proxies the API to `:8080`):

```bash
npm --prefix web run dev
```

## Configuration

Everything is environment-driven. See [`.env.example`](.env.example) for the full
list. Highlights:

| Variable | Purpose |
|---|---|
| `LOST_BASE_URL` | Public origin; drives QR content and sign-in links. |
| `LOST_SESSION_SECRET` | **Required.** Long random string signing sessions. |
| `LOST_DB_URL` | `sqlite://<path>` or `postgres://…`. |
| `LOST_NOTIFIER` | `smtp` \| `posterboy` \| `gmail-api` \| `sqs`. |
| `LOST_FROM_ADDRESS`, `LOST_FROM_NAME` | Outbound mail identity. |
| `LOST_MAX_USERS` | Cap total registered users (`0` = unlimited). |
| `LOST_ADMIN_EMAILS` | Comma-separated admin emails (unlock `/app/admin`). |
| `LOST_GEO_PROVIDER` | `none` \| `ipapi` \| `maxmind` — IP geolocation on scans/reports. |
| `LOST_GEOIP_DB` | Path to a GeoLite2 City `.mmdb` (for `maxmind`). |
| `LOST_FINDER_MIN_INTERVAL` / `LOST_FINDER_DAILY_CAP` | Finder re-report throttle (default `2h` / `6`). |
| `LOST_TURNSTILE_SITE_KEY` / `_SECRET` | Optional Cloudflare Turnstile on the form. |

### Admin, limits & scan metadata

- **User cap** — `LOST_MAX_USERS` limits registrations; new sign-ups past the cap
  are refused (existing users unaffected).
- **Admin panel** — users whose email is in `LOST_ADMIN_EMAILS` get an **Admin**
  view (`/app/admin`) listing registered users with tag/report counts and usage.
- **Finder re-reports** — after messaging an owner, a finder can send an update;
  throttled to one per `LOST_FINDER_MIN_INTERVAL` and `LOST_FINDER_DAILY_CAP` per
  day (tracked by a cookie, with IP fallback).
- **Scan metadata** — each scan records the connection IP and (if a geo provider
  is configured) an approximate IP-based location. Finders may also **opt in** to
  share their device's precise location, which enriches the owner's notification
  with a map link. Nothing precise is collected without explicit consent.

### Notifiers

- **smtp** — any SMTP server (host/port/user/pass/STARTTLS). Baseline.
- **posterboy** — send via a [Posterboy](https://github.com/martinsaul/posterboy)
  relay. `POSTERBOY_MODE=smtp` (default) speaks SMTP to it; `http` POSTs JSON to
  `POSTERBOY_INGEST_URL`.
- **gmail-api** — Gmail API via OAuth2. Mount an OAuth client + a token minted
  with the `gmail.send` scope. Higher limits than SMTP.
- **sqs** — enqueues a JSON email onto an SQS queue; a separate consumer sends
  it. For fleets that centralize outbound egress. AWS creds via the standard
  chain.

## Security & privacy

- Owner identity is never exposed on the public page unless explicitly opted in.
- QR guids are unguessable UUIDv4s; the public form is the only public write path.
- Magic-link tokens are single-use, short-TTL, and stored only as SHA-256 hashes.
- Sessions are opaque, HttpOnly, `Secure` on HTTPS, and server-revocable.
- Public form defenses: per-IP+tag rate limiting, a honeypot field, and optional
  CAPTCHA.

## Architecture

```
main.go                     entrypoint; embeds web/dist
internal/config             env-driven configuration
internal/store              SQLite + Postgres behind one API (portable SQL)
internal/auth               magic-link tokens + session ids
internal/qr                 SVG (vector) + PNG QR rendering
internal/notify             Notifier interface + smtp/posterboy/gmail/sqs backends
internal/server             HTTP API, public found pages, embedded SPA
web/                        React SPA (Vite)
```

## License

[MIT](LICENSE).
