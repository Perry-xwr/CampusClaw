export type User = { id: number; username: string; role: 'teacher' | 'student'; class_id: number; class_name: string }
export type Material = { id: number; title: string; original_filename: string; uploader: string; created_at: string }
export type LoginResponse = { token: string; user: User }
export type SearchMode = 'keyword' | 'vector' | 'hybrid'
export type SearchHit = { material_id: number; material_title: string; knowledge_entry_id: number; chunk_id: number; chunk_index: number; start_offset: number; end_offset: number; excerpt: string; material_url: string; rank: number; score: number; keyword_rank?: number; vector_rank?: number }
export type SearchResponse = { query: string; mode: SearchMode; hits: SearchHit[]; message?: string }
export type AskResponse = { answer: string; citations: SearchHit[] }
export type Prompt = { id: number; name: string; content: string; is_active: boolean; updated_at: string }
export type Skill = { id: number; slug: string; name: string; instructions: string; is_active: boolean; updated_at: string }
export type ChatSession = { id: number; title: string; prompt_id?: number; skill_id?: number; created_at: string; updated_at: string }
export type ChatMessage = { id: number; role: 'user' | 'assistant'; content: string; citations: SearchHit[]; created_at: string }

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
  prompts: () => request<{ prompts: Prompt[] }>('/api/prompts'),
  savePrompt: (prompt: Pick<Prompt, 'name' | 'content' | 'is_active'>, id?: number) => request<Prompt>(id ? `/api/prompts/${id}` : '/api/prompts', { method: id ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(prompt) }),
  skills: () => request<{ skills: Skill[] }>('/api/skills'),
  saveSkill: (skill: Pick<Skill, 'slug' | 'name' | 'instructions' | 'is_active'>, id?: number) => request<Skill>(id ? `/api/skills/${id}` : '/api/skills', { method: id ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(skill) }),
  sessions: () => request<{ sessions: ChatSession[] }>('/api/sessions'),
  createSession: (prompt_id: number, skill_id: number, title?: string) => request<ChatSession>('/api/sessions', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ prompt_id, skill_id, title }) }),
  messages: (id: number) => request<{ messages: ChatMessage[] }>(`/api/sessions/${id}/messages`),
  async sendMessage(id: number, content: string, onDelta: (text: string) => void, onSources: (citations: SearchHit[]) => void) {
    const response = await authorizedFetch(`/api/sessions/${id}/messages`, { method: 'POST', headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' }, body: JSON.stringify({ content }) })
    if (!response.ok) {
      const body = await response.json().catch(() => ({ error: `请求失败 (${response.status})` }))
      throw new ApiError(response.status, body.error ?? `请求失败 (${response.status})`)
    }
    if (!response.body) throw new Error('浏览器不支持流式响应')
    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let answer = ''
    while (true) {
      const { value, done } = await reader.read()
      buffer += decoder.decode(value, { stream: !done })
      const events = buffer.split(/\r?\n\r?\n/)
      buffer = events.pop() ?? ''
      for (const raw of events) {
        const event = raw.match(/^event:\s*(.+)$/m)?.[1]
        const data = raw.match(/^data:\s*(.+)$/m)?.[1]
        if (!event || !data) continue
        const payload = JSON.parse(data)
        if (event === 'sources') onSources(payload.citations ?? [])
        if (event === 'delta') { answer += payload.text ?? ''; onDelta(payload.text ?? '') }
        if (event === 'error') throw new Error(payload.error ?? '回答生成失败')
      }
      if (done) break
    }
    return answer
  },
  async download(id: number) {
    const response = await authorizedFetch(`/api/materials/${id}/file`)
    if (!response.ok) {
      const body = await response.json().catch(() => ({ error: `下载失败 (${response.status})` }))
      throw new ApiError(response.status, body.error ?? `下载失败 (${response.status})`)
    }
    return { blob: await response.blob(), disposition: response.headers.get('Content-Disposition') }
  },
}
