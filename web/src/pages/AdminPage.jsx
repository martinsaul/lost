import React, { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../api.js'

// Admin-only view: registration usage and the list of registered users.
// Access is gated server-side (email allowlist); this just redirects non-admins.
export default function AdminPage() {
  const nav = useNavigate()
  const [stats, setStats] = useState(null)
  const [users, setUsers] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.me()
      .then((me) => {
        if (!me.isAdmin) { nav('/app'); return null }
        return Promise.all([api.adminStats(), api.adminUsers()])
      })
      .then((res) => {
        if (!res) return
        setStats(res[0])
        setUsers(res[1].users || [])
      })
      .catch(() => nav('/'))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="center"><p className="muted">Loading…</p></div>

  return (
    <>
      <header className="bar">
        <div className="brand">Lost &amp; Found · Admin</div>
        <Link to="/app"><button className="secondary">← Dashboard</button></Link>
      </header>
      <div className="wrap">
        {stats && (
          <div className="card">
            <h2>Overview</h2>
            <div className="row" style={{ gap: 28 }}>
              <Stat label="Users" value={stats.maxUsers > 0 ? `${stats.users} / ${stats.maxUsers}` : stats.users} />
              <Stat label="QR scans" value={stats.scans} />
              <Stat label="Found reports" value={stats.reports} />
              <Stat label="Geo provider" value={stats.geoProvider || 'none'} />
            </div>
            {stats.maxUsers > 0 && stats.users >= stats.maxUsers && (
              <div className="notice err" style={{ marginTop: 12 }}>Registration is full — the user cap has been reached.</div>
            )}
          </div>
        )}

        <div className="card">
          <h2>Registered users ({users.length})</h2>
          {users.length === 0 && <p className="muted">No users yet.</p>}
          {users.length > 0 && (
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
              <thead>
                <tr style={{ textAlign: 'left', color: 'var(--muted)' }}>
                  <th style={{ padding: '6px 4px' }}>Email</th>
                  <th>Joined</th>
                  <th>Tags</th>
                  <th>Reports</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u) => (
                  <tr key={u.email} style={{ borderTop: '1px solid var(--line)' }}>
                    <td style={{ padding: '8px 4px' }}>{u.email}</td>
                    <td>{new Date(u.createdAt).toLocaleDateString()}</td>
                    <td>{u.tags}</td>
                    <td>{u.reports}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </>
  )
}

function Stat({ label, value }) {
  return (
    <div>
      <div style={{ fontSize: 22, fontWeight: 700, letterSpacing: '-0.02em' }}>{value}</div>
      <div className="tiny">{label}</div>
    </div>
  )
}
