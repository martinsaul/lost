import React, { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../api.js'

export default function Dashboard() {
  const nav = useNavigate()
  const [me, setMe] = useState(null)
  const [tags, setTags] = useState([])
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    const { tags } = await api.listTags()
    setTags(tags || [])
  }

  useEffect(() => {
    api.me()
      .then((u) => { setMe(u); return refresh() })
      .catch(() => nav('/'))
      .finally(() => setLoading(false))
  }, [])

  async function create(e) {
    e.preventDefault()
    setBusy(true)
    try {
      const tag = await api.createTag({ name: name.trim() })
      setName('')
      await refresh()
      nav(`/app/tags/${tag.guid}`)
    } finally {
      setBusy(false)
    }
  }

  async function logout() {
    await api.logout()
    nav('/')
  }

  if (loading) return <div className="center"><p className="muted">Loading…</p></div>

  return (
    <>
      <header className="bar">
        <div className="brand">Lost &amp; Found</div>
        <div className="row">
          <span className="tiny">{me && me.email}</span>
          {me && me.isAdmin && <Link to="/app/admin"><button className="secondary">Admin</button></Link>}
          <button className="secondary" onClick={logout}>Sign out</button>
        </div>
      </header>
      <div className="wrap">
        <div className="card">
          <h2>New QR tag</h2>
          <form onSubmit={create} className="row">
            <input type="text" placeholder="e.g. Martin's large luggage" value={name}
              onChange={(e) => setName(e.target.value)} style={{ flex: 1, minWidth: 220 }} />
            <button type="submit" disabled={busy}>{busy ? 'Creating…' : 'Create tag'}</button>
          </form>
          <p className="tiny" style={{ marginTop: 8 }}>You'll get a QR image to print and a private contact page.</p>
        </div>

        <div className="card">
          <h2>Your tags ({tags.length})</h2>
          {tags.length === 0 && <p className="muted">No tags yet. Create your first one above.</p>}
          {tags.map((t) => (
            <Link className="tag" key={t.guid} to={`/app/tags/${t.guid}`}>
              <div>
                <div className="name">{t.name || 'Untitled tag'}</div>
                <div className="sub">/found/{t.guid.slice(0, 8)}…</div>
              </div>
              <div className="sub">{new Date(t.createdAt).toLocaleDateString()}</div>
            </Link>
          ))}
        </div>
      </div>
    </>
  )
}
