export type User = { id: number; username: string; role: 'teacher' | 'student'; class_id: number; class_name: string }
export type Material = { id: number; title: string; original_filename: string; uploader: string; created_at: string }

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, { credentials: 'same-origin', ...options })
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: `请求失败 (${response.status})` }))
    throw new Error(body.error ?? `请求失败 (${response.status})`)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export const api = {
  me: () => request<User>('/api/me'),
  login: (username: string, password: string) => request<User>('/api/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username, password }) }),
  logout: () => request<void>('/api/logout', { method: 'POST' }),
  materials: () => request<{ materials: Material[] }>('/api/materials'),
  upload: (data: FormData) => request<{ id: number; title: string }>('/api/materials', { method: 'POST', body: data }),
}

