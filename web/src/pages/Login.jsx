import React, { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../api.js'

export default function Login() {
  const nav = useNavigate()
  const [params] = useSearchParams()
  const [email, setEmail] = useState('')
  const [sent, setSent] = useState(false)
  const [busy, setBusy] = useState(false)
  const [appName, setAppName] = useState('Lost & Found')
  const authError = params.get('auth_error')

  useEffect(() => {
    // Already signed in? Jump to the dashboard.
    api.me().then(() => nav('/app')).catch(() => {})
    api.config().then((c) => c && c.appName && setAppName(c.appName)).catch(() => {})
  }, [])

  async function submit(e) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.requestLink(email)
      setSent(true)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="center">
      <div className="card" style={{ width: 380, maxWidth: '100%' }}>
        <div className="brand">{appName} <small>· lost &amp; found</small></div>
        <h1 style={{ marginTop: 16 }}>Sign in</h1>
        <p className="muted">Register QR tags for your belongings. No password — we email you a sign-in link.</p>

        {authError && <div className="notice err">Sign-in link {authError}. Request a new one below.</div>}

        {sent ? (
          <div className="notice ok">
            Check your inbox. If an account exists for <strong>{email}</strong>, a sign-in link is on its way.
          </div>
        ) : (
          <form onSubmit={submit}>
            <label htmlFor="email">Email address</label>
            <input id="email" type="email" required autoFocus value={email}
              onChange={(e) => setEmail(e.target.value)} placeholder="you@example.com" />
            <div style={{ marginTop: 16 }}>
              <button type="submit" disabled={busy}>{busy ? 'Sending…' : 'Email me a link'}</button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}
