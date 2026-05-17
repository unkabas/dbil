// apiFetch is the single entry point for talking to the dbil backend from
// the browser. Pulls the bearer token from localStorage, attaches it to
// every request, and throws a typed ApiError for 4xx/5xx so callers (and
// react-query) can switch on status code.

const TOKEN_KEY = 'dbil_token'

export interface ApiErrorBody {
  error?: string
  reason?: string
}

export class ApiError extends Error {
  status: number
  body: ApiErrorBody

  constructor(status: number, body: ApiErrorBody, message?: string) {
    super(message || body.error || `HTTP ${status}`)
    this.status = status
    this.body = body
  }
}

export function getStoredToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setStoredToken(t: string | null) {
  if (t) localStorage.setItem(TOKEN_KEY, t)
  else localStorage.removeItem(TOKEN_KEY)
}

export interface FetchOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
  body?: unknown
  headers?: Record<string, string>
  // When true, omit Authorization (used by /api/auth/login).
  unauthed?: boolean
}

export async function apiFetch<T>(path: string, opts: FetchOptions = {}): Promise<T> {
  const headers: Record<string, string> = { ...opts.headers }
  if (!opts.unauthed) {
    const token = getStoredToken()
    if (token) headers['Authorization'] = `Bearer ${token}`
  }
  if (opts.body !== undefined && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json'
  }

  const r = await fetch(path, {
    method: opts.method ?? (opts.body !== undefined ? 'POST' : 'GET'),
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })

  if (r.status === 204) return undefined as T

  const text = await r.text()
  let parsed: unknown = null
  if (text) {
    try {
      parsed = JSON.parse(text)
    } catch {
      parsed = { error: text }
    }
  }

  if (!r.ok) {
    throw new ApiError(r.status, (parsed as ApiErrorBody) ?? {})
  }
  return parsed as T
}
