import React, { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api } from '../api.js'

// Print sizes offered for the rasterized PNG (pixel dimensions).
const PNG_SIZES = [
  { label: 'Small (512px)', px: 512 },
  { label: 'Medium (1024px)', px: 1024 },
  { label: 'Large (2048px)', px: 2048 },
]

export default function TagDetail() {
  const { guid } = useParams()
  const nav = useNavigate()
  const [tag, setTag] = useState(null)
  const [saved, setSaved] = useState(false)
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    api.getTag(guid).then(setTag).catch(() => nav('/app'))
  }, [guid])

  if (!tag) return <div className="center"><p className="muted">Loading…</p></div>

  function set(k, v) { setTag({ ...tag, [k]: v }); setSaved(false) }

  async function save(e) {
    e.preventDefault()
    setBusy(true)
    try {
      const updated = await api.updateTag(guid, {
        name: tag.name, showEmail: tag.showEmail, showPhone: tag.showPhone, phone: tag.phone || '',
      })
      setTag(updated)
      setSaved(true)
    } finally {
      setBusy(false)
    }
  }

  async function remove() {
    if (!confirm('Delete this tag? The QR code will stop working.')) return
    await api.deleteTag(guid)
    nav('/app')
  }

  function copyUrl() {
    navigator.clipboard?.writeText(tag.foundUrl)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  return (
    <>
      <header className="bar">
        <div className="brand">Lost &amp; Found</div>
        <Link to="/app"><button className="secondary">← All tags</button></Link>
      </header>
      <div className="wrap">
        <div className="card qr">
          <h2 style={{ alignSelf: 'flex-start' }}>QR code</h2>
          <img src={`/api/tags/${guid}/qr.png?size=512`} alt="QR code" />
          <div className="row">
            <a href={`/api/tags/${guid}/qr.svg`}><button className="secondary">Download SVG (vector)</button></a>
            {PNG_SIZES.map((s) => (
              <a key={s.px} href={`/api/tags/${guid}/qr.png?size=${s.px}`}>
                <button className="secondary">PNG · {s.label}</button>
              </a>
            ))}
          </div>
          <div className="row" style={{ width: '100%' }}>
            <input type="text" readOnly value={tag.foundUrl} style={{ flex: 1 }} />
            <button className="secondary" onClick={copyUrl}>{copied ? 'Copied' : 'Copy link'}</button>
          </div>
          <p className="tiny">SVG is resolution-independent — best for printing at any size, including thermal labels.</p>
        </div>

        <form className="card" onSubmit={save}>
          <h2>Details</h2>
          <label htmlFor="n">Name (private — only you see this)</label>
          <input id="n" type="text" value={tag.name} onChange={(e) => set('name', e.target.value)}
            placeholder="e.g. Martin's large luggage" />

          <h2 style={{ marginTop: 20 }}>Public contact</h2>
          <p className="tiny">By default finders reach you through a form without seeing your details. Opt in to show them directly for faster recovery.</p>

          <div className="check">
            <input id="se" type="checkbox" checked={tag.showEmail} onChange={(e) => set('showEmail', e.target.checked)} />
            <label htmlFor="se" style={{ margin: 0 }}>Show my email on the found page</label>
          </div>
          <div className="check">
            <input id="sp" type="checkbox" checked={tag.showPhone} onChange={(e) => set('showPhone', e.target.checked)} />
            <label htmlFor="sp" style={{ margin: 0 }}>Show a phone number on the found page</label>
          </div>
          {tag.showPhone && (
            <input type="tel" value={tag.phone || ''} onChange={(e) => set('phone', e.target.value)}
              placeholder="+1 555 010 0000" />
          )}

          {saved && <div className="notice ok">Saved.</div>}
          <div className="row" style={{ marginTop: 16 }}>
            <button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save changes'}</button>
            <div className="spacer" />
            <button type="button" className="danger" onClick={remove}>Delete tag</button>
          </div>
        </form>
      </div>
    </>
  )
}
