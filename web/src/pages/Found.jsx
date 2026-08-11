import React, { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../api.js'

// Public page a finder lands on after scanning the sticker. Lets them reach the
// owner without the owner's identity being exposed (unless opted in).
export default function Found() {
  const { guid } = useParams()
  const [info, setInfo] = useState(null)
  const [notFound, setNotFound] = useState(false)
  const [cfg, setCfg] = useState({ turnstileEnabled: false, appName: 'Lost & Found' })
  const [form, setForm] = useState({ name: '', message: '', email: '', phone: '', website: '' })
  const [sent, setSent] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [loc, setLoc] = useState(null)        // { lat, lon, accuracy } once shared
  const [locBusy, setLocBusy] = useState(false)
  const [locMsg, setLocMsg] = useState('')
  const tokenRef = useRef('')

  useEffect(() => {
    api.getFound(guid).then(setInfo).catch(() => setNotFound(true))
    api.config().then(setCfg).catch(() => {})
  }, [guid])

  // Lazily load the Cloudflare Turnstile widget when enabled.
  useEffect(() => {
    if (!cfg.turnstileEnabled || !cfg.turnstileSiteKey) return
    const s = document.createElement('script')
    s.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js'
    s.async = true
    document.body.appendChild(s)
    return () => { document.body.removeChild(s) }
  }, [cfg])

  function set(k, v) { setForm({ ...form, [k]: v }) }

  function shareLocation() {
    if (!navigator.geolocation) { setLocMsg('Location is not available in this browser.'); return }
    setLocBusy(true); setLocMsg('')
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setLoc({ lat: pos.coords.latitude, lon: pos.coords.longitude, accuracy: pos.coords.accuracy })
        setLocMsg(`Location attached (±${Math.round(pos.coords.accuracy)}m). Thank you!`)
        setLocBusy(false)
      },
      (err) => {
        setLocMsg(err.code === 1 ? 'Location permission denied — no problem, it is optional.' : 'Could not get your location.')
        setLocBusy(false)
      },
      { enableHighAccuracy: true, timeout: 10000, maximumAge: 60000 },
    )
  }

  async function submit(e) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const token = window.turnstile?.getResponse?.() || tokenRef.current || ''
      const payload = { ...form, token }
      if (loc) { payload.hasLocation = true; payload.lat = loc.lat; payload.lon = loc.lon; payload.accuracy = loc.accuracy }
      await api.submitFound(guid, payload)
      setSent(true)
    } catch (err) {
      setError(err.message || 'Could not send. Please try again.')
    } finally {
      setBusy(false)
    }
  }

  if (notFound) {
    return <div className="center"><div className="card" style={{ maxWidth: 420 }}>
      <h1>Unknown code</h1>
      <p className="muted">This QR code isn't registered. It may have been deleted.</p>
    </div></div>
  }
  if (!info) return <div className="center"><p className="muted">Loading…</p></div>

  return (
    <div className="center">
      <div className="card" style={{ width: 460, maxWidth: '100%' }}>
        <div className="brand">{cfg.appName}</div>
        <h1 style={{ marginTop: 12 }}>You found something!</h1>
        <p className="muted">
          {info.name ? <>This belongs to someone — it's labeled “<strong>{info.name}</strong>”. </> : <>This item belongs to someone. </>}
          Thank you for helping return it.
        </p>

        {(info.ownerEmail || info.ownerPhone) && (
          <div className="notice ok">
            The owner chose to share direct contact details:
            {info.ownerEmail && <div>Email: <a href={`mailto:${info.ownerEmail}`}>{info.ownerEmail}</a></div>}
            {info.ownerPhone && <div>Phone: <a href={`tel:${info.ownerPhone}`}>{info.ownerPhone}</a></div>}
          </div>
        )}

        {sent ? (
          <>
            <div className="notice ok">Message sent — the owner has been notified. Thank you!</div>
            <p className="tiny">Need to correct or add something? You can send one more update.</p>
            <button className="secondary" onClick={() => { setSent(false); setError('') }}>Send an update</button>
          </>
        ) : (
          <form onSubmit={submit}>
            <label htmlFor="msg">Message to the owner</label>
            <textarea id="msg" required value={form.message} onChange={(e) => set('message', e.target.value)}
              placeholder="I found this at… Here's how to get it back to you." />

            <label htmlFor="fn">Your name (optional)</label>
            <input id="fn" type="text" value={form.name} onChange={(e) => set('name', e.target.value)} />

            <div className="row">
              <div style={{ flex: 1, minWidth: 180 }}>
                <label htmlFor="fe">Your email (optional)</label>
                <input id="fe" type="email" value={form.email} onChange={(e) => set('email', e.target.value)} />
              </div>
              <div style={{ flex: 1, minWidth: 150 }}>
                <label htmlFor="fp">Your phone (optional)</label>
                <input id="fp" type="tel" value={form.phone} onChange={(e) => set('phone', e.target.value)} />
              </div>
            </div>
            <p className="tiny">Leave a way to reach you so the owner can reply and arrange pickup.</p>

            {/* Optional precise location to help the owner locate the item. */}
            <div className="card" style={{ padding: 12, marginTop: 8, background: '#fafafa' }}>
              {loc ? (
                <div className="notice ok" style={{ margin: 0 }}>{locMsg}</div>
              ) : (
                <>
                  <button type="button" className="secondary" onClick={shareLocation} disabled={locBusy}>
                    {locBusy ? 'Getting location…' : '📍 Share my current location (optional)'}
                  </button>
                  <p className="tiny" style={{ marginBottom: 0 }}>
                    Shares your device's GPS location with the owner to help arrange pickup. Optional.
                  </p>
                  {locMsg && <p className="tiny" style={{ color: '#b91c1c' }}>{locMsg}</p>}
                </>
              )}
            </div>

            {/* Honeypot: hidden from humans, tempting to bots. */}
            <input type="text" name="website" tabIndex="-1" autoComplete="off"
              value={form.website} onChange={(e) => set('website', e.target.value)}
              style={{ position: 'absolute', left: '-9999px' }} aria-hidden="true" />

            {cfg.turnstileEnabled && cfg.turnstileSiteKey && (
              <div className="cf-turnstile" data-sitekey={cfg.turnstileSiteKey} style={{ marginTop: 12 }} />
            )}

            {error && <div className="notice err">{error}</div>}
            <div style={{ marginTop: 16 }}>
              <button type="submit" disabled={busy}>{busy ? 'Sending…' : 'Send message'}</button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}
