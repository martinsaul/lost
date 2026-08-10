# lost — QR lost-and-found

Public, self-hostable service that turns a QR-code sticker into a private contact
channel for lost property. Print a sticker on your luggage; a finder scans it,
lands on `https://<domain>/found/<guid>`, and can reach you through a contact
form — without your email or phone being exposed (unless you opt in to show them).

The domain is fully configurable so anyone can run their own instance.

## Core concepts

- **Account** — identified by email only. No password. Auth is a magic link
  emailed on demand (passwordless).
- **Tag** — a registered QR code owned by an account. Has:
  - `guid` (public, in the URL — unguessable UUIDv4)
  - `name` (private label, e.g. "large blue suitcase")
  - optional public contact opt-in: `show_email`, `show_phone`, `phone`
  - a downloadable high-resolution QR image (vector SVG + rasterized PNG at
    chosen DPI) encoding `<base_url>/found/<guid>`.
- **Found report** — a message a finder submits on `/found/<guid>`. Delivered to
  the owner via the configured notifier. The finder may optionally leave their
  own email/phone so the owner can reply directly.

## User flows

1. **Register / sign in** — enter email → receive magic link → session cookie set
   (HttpOnly, SameSite=Lax, signed). No password ever.
2. **Create a tag** — name it; a `guid` is minted. Owner can toggle public
   contact info and download the QR image (SVG + PNG, several print sizes).
3. **Finder** — scans sticker → `/found/<guid>`:
   - shows the tag's public name (optional) + a contact form (name, message,
     optional finder email/phone).
   - if owner opted in, shows owner's email/phone directly.
   - submitting the form notifies the owner. Never reveals owner identity unless
     opted in.

## Pluggable outbound delivery (notifiers)

A single `Notifier` abstraction with config-selected backend(s). Used for BOTH
magic-link login emails and found-report notifications.

- **smtp** — plain SMTP (host/port/user/pass/STARTTLS). Baseline.
- **posterboy** — send to a Posterboy relay (SMTP to `posterboy:2525`, or its HTTP
  ingest), which fans messages out to chat/email.
- **gmail-api** — Gmail API with OAuth2 (higher limits, no SMTP).
- **sqs** — enqueue a JSON message to an Amazon SQS queue; a separate worker (out
  of scope of this app, or a bundled consumer) does the actual send. For fleets
  that centralize egress.

Selected via `LOST_NOTIFIER=smtp|posterboy|gmail-api|sqs` (+ backend-specific env).

## Anti-abuse (public surface)

- Unguessable guids; `/found/<guid>` is the only public write path.
- Per-IP + per-tag rate limiting on form submits.
- Honeypot field + optional CAPTCHA (Cloudflare Turnstile / hCaptcha) — toggle.
- Magic-link tokens: single-use, short TTL, constant-time compare.

## Configuration (all env-driven)

- `LOST_BASE_URL` — e.g. `https://lost.example.com` (drives QR content).
- `LOST_DB_URL` — datastore connection (SQLite or Postgres).
- `LOST_SESSION_SECRET` — cookie/token signing.
- `LOST_NOTIFIER` + backend vars (SMTP_*, POSTERBOY_*, GMAIL_*, SQS_*).
- `LOST_FROM_ADDRESS`, `LOST_FROM_NAME`.
- `LOST_TURNSTILE_*` (optional).

## Deployment

Single Docker image (SPA embedded in a static Go binary) behind any TLS-terminating
reverse proxy. See [DEPLOY.md](DEPLOY.md). Everything is env-configured with sane
defaults and a documented [`.env.example`](.env.example); no build-time secrets.
