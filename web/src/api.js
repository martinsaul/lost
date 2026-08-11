// Thin fetch wrapper. All requests send cookies so the session travels with the
// SPA. Errors surface as thrown Error with the server's message when present.

async function req(method, url, body) {
  const opts = {
    method,
    credentials: 'include',
    headers: {},
  }
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(url, opts)
  if (res.status === 204) return null
  const text = await res.text()
  let data = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = text
  }
  if (!res.ok) {
    const msg = (data && data.error) || res.statusText || 'request failed'
    const err = new Error(msg)
    err.status = res.status
    throw err
  }
  return data
}

export const api = {
  config: () => req('GET', '/api/config'),
  me: () => req('GET', '/api/me'),
  requestLink: (email) => req('POST', '/api/auth/request', { email }),
  logout: () => req('POST', '/api/auth/logout'),

  listTags: () => req('GET', '/api/tags'),
  createTag: (t) => req('POST', '/api/tags', t),
  getTag: (guid) => req('GET', `/api/tags/${guid}`),
  updateTag: (guid, t) => req('PATCH', `/api/tags/${guid}`, t),
  deleteTag: (guid) => req('DELETE', `/api/tags/${guid}`),

  getFound: (guid) => req('GET', `/api/found/${guid}`),
  submitFound: (guid, body) => req('POST', `/api/found/${guid}`, body),

  adminStats: () => req('GET', '/api/admin/stats'),
  adminUsers: () => req('GET', '/api/admin/users'),
}
