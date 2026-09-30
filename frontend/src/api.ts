export type User = { id: number; username: string; role: 'teacher' | 'student'; class_id: number; class_name: string }
export type Material = { id: number; title: string; original_filename: string; uploader: string; created_at: string }
export type LoginResponse = { token: string; user: User }
export type SearchMode = 'keyword' | 'vector' | 'hybrid'
export type SearchHit = { material_id: number; material_title: string; knowledge_entry_id: number; chunk_id: number; chunk_index: number; start_offset: number; end_offset: number; excerpt: string; material_url: string; rank: number; score: number; keyword_rank?: number; vector_rank?: number }
export type SearchResponse = { query: string; mode: SearchMode; hits: SearchHit[]; message?: string }
export type AskResponse = { answer: string; citations: SearchHit[] }

const tokenKey = 'campusclaw_token'

export function setToken(token: string) { localStorage.setItem(tokenKey, token) }
export function getToken() { return localStorage.getItem(tokenKey) }
export function clearToken() { localStorage.removeItem(tokenKey) }

export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message) }
}

async function authorizedFetch(url: string, options?: RequestInit) {
  const headers = new Headers(options?.headers)
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  return fetch(url, { ...options, headers })
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await authorizedFetch(url, options)
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: `请求失败 (${response.status})` }))
    throw new ApiError(response.status, body.error ?? `请求失败 (${response.status})`)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export const api = {
  me: () => request<User>('/api/me'),
  login: (username: string, password: string) => request<LoginResponse>('/api/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username, password }) }),
  logout: () => request<void>('/api/logout', { method: 'POST' }),
  materials: () => request<{ materials: Material[] }>('/api/materials'),
  upload: (data: FormData) => request<{ id: number; title: string }>('/api/materials', { method: 'POST', body: data }),
  search: (query: string, mode: SearchMode = 'hybrid') => request<SearchResponse>('/api/knowledge/search', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ query, mode }) }),
  ask: (question: string) => request<AskResponse>('/api/ask', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ messages: [{ role: 'user', content: question }] }) }),
  async download(id: number) {
    const response = await authorizedFetch(`/api/materials/${id}/file`)
    if (!response.ok) {
      const body = await response.json().catch(() => ({ error: `下载失败 (${response.status})` }))
      throw new ApiError(response.status, body.error ?? `下载失败 (${response.status})`)
    }
    return { blob: await response.blob(), disposition: response.headers.get('Content-Disposition') }
  },
}
