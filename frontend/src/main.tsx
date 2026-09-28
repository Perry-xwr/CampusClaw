import { StrictMode, useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { api, type Material, type User } from './api'
import './styles.css'

function Login({ onLogin }: { onLogin: (user: User) => void }) {
  const [username, setUsername] = useState('teacher_a')
  const [password, setPassword] = useState('TeacherA123!')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(event: React.FormEvent) {
    event.preventDefault(); setError(''); setBusy(true)
    try { onLogin(await api.login(username, password)) } catch (reason) { setError((reason as Error).message) } finally { setBusy(false) }
  }
  return <main className="center"><form className="card login" onSubmit={submit}>
    <div className="brand">CampusClaw</div><h1>登录</h1><p className="muted">教学材料与知识库 · Iteration 1</p>
    <label>用户名<input autoComplete="username" value={username} onChange={e => setUsername(e.target.value)} /></label>
    <label>密码<input type="password" autoComplete="current-password" value={password} onChange={e => setPassword(e.target.value)} /></label>
    {error && <p className="error" role="alert">{error}</p>}
    <button disabled={busy}>{busy ? '登录中…' : '登录'}</button>
  </form></main>
}

function Materials({ user, onLogout }: { user: User; onLogout: () => void }) {
  const [items, setItems] = useState<Material[]>([])
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  async function refresh() {
    try { setItems((await api.materials()).materials); setError('') } catch (reason) { setError((reason as Error).message) }
  }
  useEffect(() => { void refresh() }, [])
  async function upload(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError(''); setNotice(''); setBusy(true)
    try {
      const data = new FormData(event.currentTarget)
      const result = await api.upload(data)
      setNotice(`“${result.title}”上传并入库成功`)
      event.currentTarget.reset()
      await refresh()
    } catch (reason) { setError((reason as Error).message) } finally { setBusy(false) }
  }
  return <><header><div><div className="brand">CampusClaw</div><span className="muted">Iteration 1</span></div><div className="account"><span>{user.username} · {user.role} · {user.class_name}</span><button className="secondary" onClick={onLogout}>登出</button></div></header>
    <main className="layout"><section><div className="section-title"><div><h1>教学材料</h1><p className="muted">仅显示你所在班级的材料</p></div><button className="secondary" onClick={() => void refresh()}>刷新</button></div>
      {error && <p className="error" role="alert">{error}</p>}{notice && <p className="success">{notice}</p>}
      <div className="material-list">{items.length === 0 ? <div className="empty">本班暂无材料</div> : items.map(item => <article className="material" key={item.id}><div><h2>{item.title}</h2><p>{item.original_filename} · 上传者 {item.uploader}</p><time>{new Date(item.created_at).toLocaleString()}</time></div><a className="download" href={`/api/materials/${item.id}/file`}>下载</a></article>)}</div>
    </section>
    {user.role === 'teacher' && <aside className="card"><h2>上传材料</h2><p className="muted">支持非空 UTF-8 .txt / .md，最大 1 MiB</p><form onSubmit={upload}><label>标题（可选）<input name="title" maxLength={200} /></label><label>文件<input name="file" type="file" accept=".txt,.md,text/plain,text/markdown" required /></label><button disabled={busy}>{busy ? '处理中…' : '上传并入库'}</button></form></aside>}
    </main></>
}

function App() {
  const [user, setUser] = useState<User | null | undefined>(undefined)
  useEffect(() => { api.me().then(setUser).catch(() => setUser(null)) }, [])
  if (user === undefined) return <main className="center"><p>正在载入…</p></main>
  if (!user) return <Login onLogin={setUser} />
  return <Materials user={user} onLogout={async () => { await api.logout().catch(() => undefined); setUser(null) }} />
}

createRoot(document.getElementById('root')!).render(<StrictMode><App /></StrictMode>)

