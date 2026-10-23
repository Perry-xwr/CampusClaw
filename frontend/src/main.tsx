import { StrictMode, useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { ApiError, api, clearToken, getToken, setToken, type ChatMessage, type ChatSession, type Material, type MCPAudit, type MCPServer, type MCPTool, type Prompt, type SearchHit, type SearchMode, type Skill, type User } from './api'
import './styles.css'

function Login({ onLogin }: { onLogin: (user: User, selectedRole: User['role']) => void }) {
  const [selectedRole, setSelectedRole] = useState<User['role']>('teacher')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(event: React.FormEvent) {
    event.preventDefault(); setError(''); setBusy(true)
    try {
      const response = await api.login(username, password)
      setToken(response.token)
      onLogin(response.user, selectedRole)
    } catch (reason) { setError((reason as Error).message) } finally { setBusy(false) }
  }
  function chooseRole(role: User['role']) {
    setSelectedRole(role)
    setUsername('')
    setPassword('')
    setError('')
  }
  return <main className="login-page"><section className="login-intro"><div className="login-brand"><div className="brand-mark large">C</div><span className="brand">CampusClaw</span></div><span className="eyebrow">CLASSROOM KNOWLEDGE ASSISTANT</span><h1>让班级知识有来源，<br />让每次回答可追溯。</h1><p>面向班级知识管理与可追溯问答的智能教学平台</p><div className="feature-grid">
    <div><span aria-hidden="true">库</span><strong>班级知识库</strong><small>材料统一沉淀与管理</small></div>
    <div><span aria-hidden="true">检</span><strong>可追溯检索</strong><small>定位原文与来源片段</small></div>
    <div><span aria-hidden="true">问</span><strong>AI 知识问答</strong><small>依据本班资料生成回答</small></div>
    <div><span aria-hidden="true">教</span><strong>教师材料管理</strong><small>上传并构建课程知识</small></div>
  </div><p className="login-footnote">CampusClaw · 面向教学场景的可信知识助手</p></section>
  <section className="login-side"><form className="card login" onSubmit={submit}><div className="login-card-heading"><span className="eyebrow">WELCOME BACK</span><h2>登录 CampusClaw</h2><p className="muted">选择入口并使用你的班级账号登录</p></div>
    <div className="role-tabs" role="tablist" aria-label="登录身份"><button type="button" role="tab" aria-selected={selectedRole === 'teacher'} className={selectedRole === 'teacher' ? 'active teacher' : ''} onClick={() => chooseRole('teacher')}><span aria-hidden="true">教</span><div><strong>教师登录</strong><small>材料管理与知识问答</small></div></button><button type="button" role="tab" aria-selected={selectedRole === 'student'} className={selectedRole === 'student' ? 'active student' : ''} onClick={() => chooseRole('student')}><span aria-hidden="true">学</span><div><strong>学生登录</strong><small>知识检索与课程问答</small></div></button></div>
    <label>用户名<input autoComplete="username" value={username} onChange={e => setUsername(e.target.value)} placeholder={selectedRole === 'teacher' ? '请输入教师账号' : '请输入学生账号'} required /></label>
    <label>密码<input type="password" autoComplete="current-password" value={password} onChange={e => setPassword(e.target.value)} placeholder="请输入密码" required /></label>
    {error && <p className="error login-error" role="alert"><span aria-hidden="true">!</span>{error}</p>}
    <button className="login-submit" disabled={busy}>{busy ? <><span className="spinner" />登录中…</> : `进入${selectedRole === 'teacher' ? '教师端' : '学生端'}`}</button>
    <div className="demo-account"><div><span>演示账号</span><small>密码请使用课程提供的账号密码</small></div><button type="button" className="account-suggestion" onClick={() => setUsername(selectedRole === 'teacher' ? 'teacher_a' : 'student_a1')}>{selectedRole === 'teacher' ? 'teacher_a' : 'student_a1'}</button></div>
    <p className="security-note"><span aria-hidden="true">✓</span>身份与权限以服务器认证结果为准</p>
  </form></section></main>
}

function Materials({ user, onLogout, identityNotice }: { user: User; onLogout: () => void; identityNotice: string }) {
  const [activeApp, setActiveApp] = useState(() => new URLSearchParams(window.location.search).get('app') ?? '')
  const [items, setItems] = useState<Material[]>([])
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  async function refresh() {
    try { setItems((await api.materials()).materials); setError('') } catch (reason) { setError((reason as Error).message) }
  }
  useEffect(() => { void refresh() }, [])
  useEffect(() => {
    const onPopState = () => setActiveApp(new URLSearchParams(window.location.search).get('app') ?? '')
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [])
  function navigateApp(app: string) {
    const url = new URL(window.location.href)
    if (app) url.searchParams.set('app', app)
    else url.searchParams.delete('app')
    window.history.pushState({}, '', url)
    setActiveApp(app)
  }
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
  async function download(item: Material) {
    await downloadSource(item.id, item.original_filename)
  }
  async function deleteMaterial(item: Material) {
    const action = item.deletion_pending ? '重试删除' : '删除'
    if (!window.confirm(`确定${action}“${item.title}”吗？此操作会移除本班材料文件及知识库索引，但不会删除历史会话。`)) return
    setError(''); setNotice(''); setBusy(true)
    try {
      await api.deleteMaterial(item.id)
      setNotice(`“${item.title}”已删除`)
      await refresh()
    } catch (reason) {
      setError((reason as Error).message)
      await refresh()
    } finally { setBusy(false) }
  }
  async function downloadSource(id: number, filename: string) {
    try {
      const { blob } = await api.download(id)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = filename
      document.body.appendChild(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(url)
    } catch (reason) { setError((reason as Error).message) }
  }
  const roleName = user.role === 'teacher' ? '教师' : '学生'
  return <><header className="app-header"><div className="header-inner"><div className="header-brand"><div className="brand-mark" aria-hidden="true">C</div><div><div className="brand">CampusClaw</div><span>{user.role === 'teacher' ? '教师端' : '学生端'} · 可追溯班级知识库</span></div></div><div className="class-heading"><strong>{user.class_name}知识库</strong><span>{activeApp === 'course-qa' ? '课程知识问答' : activeApp === 'guided-solver' ? 'AI 解题引导' : '课程 AI 应用中心'}</span></div><div className="account"><span className={`role-badge ${user.role}`}>{roleName}身份</span><span className="user-chip"><b>{user.username}</b><small>{user.class_name}</small></span><a className="header-link" href="#materials">材料</a><a className="header-link" href="/docs/" target="_blank" rel="noreferrer">API 文档</a><button className="secondary compact" onClick={onLogout}>退出</button></div></div></header>
    {identityNotice && <div className="identity-notice" role="status">{identityNotice}，系统已按服务器认证的真实身份进入{user.role === 'teacher' ? '教师端' : '学生端'}。</div>}
    <nav className="application-nav" aria-label="课程应用"><button type="button" className={!activeApp ? 'selected' : ''} onClick={() => navigateApp('')}>应用中心</button><button type="button" className={activeApp === 'course-qa' ? 'selected' : ''} onClick={() => navigateApp('course-qa')}>课程知识问答</button><button type="button" className={activeApp === 'guided-solver' ? 'selected' : ''} onClick={() => navigateApp('guided-solver')}>AI 解题引导</button></nav>
    <main className="layout"><section>{!activeApp && <ApplicationHub onOpen={navigateApp} />} {activeApp === 'course-qa' && <KnowledgeWorkspace onSource={(hit) => void downloadSource(hit.material_id, hit.material_title)} />} {activeApp === 'guided-solver' && <ConversationWorkspace user={user} onSource={(hit) => void downloadSource(hit.material_id, hit.material_title)} />}<div id="materials" className="materials-heading section-title"><div><span className="eyebrow">CLASS MATERIALS</span><h1>教学材料</h1><p className="muted">仅显示你所在班级的材料，所有来源均受权限保护</p></div><button className="secondary compact" onClick={() => void refresh()}>刷新列表</button></div>
      {error && <p className="error" role="alert">{error}</p>}{notice && <p className="success">{notice}</p>}
      <div className="material-list">{items.length === 0 ? <div className="empty-state compact-empty"><span className="empty-icon" aria-hidden="true">□</span><strong>本班暂无材料</strong></div> : items.map(item => <article className={`material${item.deletion_pending ? ' deletion-pending' : ''}`} key={item.id}><div className="material-icon" aria-hidden="true">文</div><div className="material-copy"><h2>{item.title}{item.deletion_pending && <span className="pending-badge">删除待重试</span>}</h2><p>{item.original_filename} · 上传者 {item.uploader}</p><time>{new Date(item.created_at).toLocaleString()}</time></div>{!item.deletion_pending && <button className="secondary compact" onClick={() => void download(item)}>下载</button>}{user.role === 'teacher' && <button className={item.deletion_pending ? 'danger compact' : 'secondary compact'} disabled={busy} onClick={() => void deleteMaterial(item)}>{item.deletion_pending ? '重试删除' : '删除'}</button>}</article>)}</div>
    </section>
    {user.role === 'teacher' && <aside className="card upload-card"><div className="aside-icon" aria-hidden="true">＋</div><span className="eyebrow">TEACHER TOOLS</span><h2>上传教学材料</h2><p className="muted">上传后自动切片并加入本班知识库</p><form onSubmit={upload}><label>材料标题 <span>可选</span><input name="title" maxLength={200} placeholder="例如：实验室安全须知" /></label><label>选择文件 <span>TXT / MD · 最大 1 MiB</span><input name="file" type="file" accept=".txt,.md,text/plain,text/markdown" required /></label><button disabled={busy}>{busy ? <><span className="spinner" />处理中…</> : '上传并入库'}</button></form><p className="upload-note">仅支持非空 UTF-8 文本文件</p></aside>}
    </main></>
}

function ApplicationHub({ onOpen }: { onOpen: (app: string) => void }) {
  return <section className="application-hub card"><span className="eyebrow">CAMPUSCLAW AI APPLICATIONS</span><h1>课程 AI 应用</h1><p className="muted">两个独立入口，分别使用课程知识问答与带技能的学习会话流程。</p><div className="application-cards"><button type="button" className="application-card" onClick={() => onOpen('course-qa')}><span className="application-icon">知</span><span className="eyebrow">COURSE KNOWLEDGE QA</span><strong>课程知识问答</strong><small>直接检索本班课程资料，调用知识问答接口并展示依据与引用来源。</small><span className="application-api">POST /api/knowledge/search · POST /api/ask</span></button><button type="button" className="application-card" onClick={() => onOpen('guided-solver')}><span className="application-icon">引</span><span className="eyebrow">GUIDED STUDY SESSION</span><strong>AI 解题引导</strong><small>创建学习会话，选择教师提示词与技能，接收 SSE 回答和工具状态。</small><span className="application-api">POST /api/sessions · POST /api/sessions/{'{id}'}/messages</span></button></div></section>
}

function ConversationWorkspace({ user, onSource }: { user: User; onSource: (hit: SearchHit) => void }) {
  const [prompts, setPrompts] = useState<Prompt[]>([])
  const [skills, setSkills] = useState<Skill[]>([])
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [current, setCurrent] = useState<number | null>(null)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [promptID, setPromptID] = useState('')
  const [skillID, setSkillID] = useState('')
  const [question, setQuestion] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [promptName, setPromptName] = useState('')
  const [promptContent, setPromptContent] = useState('')
  const [promptActive, setPromptActive] = useState(true)
  const [promptEditID, setPromptEditID] = useState<number | undefined>()
  const [skillSlug, setSkillSlug] = useState('guided-solution')
  const [skillName, setSkillName] = useState('解题引导')
  const [skillInstructions, setSkillInstructions] = useState('通过提问引导学生拆解题目条件并回忆相关概念；只给提示，不直接给出最终答案。')
  const [skillActive, setSkillActive] = useState(true)
  const [skillEditID, setSkillEditID] = useState<number | undefined>()
  const [skillTools, setSkillTools] = useState<string[]>([])
  const [skillFileName, setSkillFileName] = useState('')
  const [mcpServers, setMcpServers] = useState<MCPServer[]>([])
  const [mcpTools, setMcpTools] = useState<Array<MCPTool & { ref: string }>>([])
  const [mcpAudit, setMcpAudit] = useState<MCPAudit[]>([])
  const [mcpName, setMcpName] = useState('mcp-demo')
  const [mcpBaseURL, setMcpBaseURL] = useState('http://mcp-demo:8090')

  async function refresh() {
    try {
      const [promptResult, skillResult, sessionResult] = await Promise.all([api.prompts(), api.skills(), api.sessions()])
      setPrompts(promptResult.prompts); setSkills(skillResult.skills); setSessions(sessionResult.sessions)
      if (user.role === 'teacher') {
        const [serverResult, auditResult] = await Promise.all([api.mcpServers(), api.mcpAudit()])
        setMcpServers(serverResult.servers); setMcpAudit(auditResult.tool_calls)
        const discovered = await Promise.all(serverResult.servers.filter(server => server.connected).map(async server => {
          try { return (await api.mcpTools(server.id)).tools.map(tool => ({ ...tool, ref: `${tool.name}@${server.name}` })) } catch { return [] }
        }))
        setMcpTools(discovered.flat())
      }
      const activePrompts = promptResult.prompts.filter(item => item.is_active)
      const activeSkills = skillResult.skills.filter(item => item.is_active)
      setPromptID(value => activePrompts.some(item => String(item.id) === value) ? value : String(activePrompts[0]?.id ?? ''))
      setSkillID(value => activeSkills.some(item => String(item.id) === value) ? value : String(activeSkills.find(item => item.slug === 'guided-solution')?.id ?? activeSkills[0]?.id ?? ''))
      return sessionResult.sessions
    } catch (reason) { setError((reason as Error).message); return [] }
  }
  async function loadMessages(sessionID: number) {
    setCurrent(sessionID)
    try { setMessages((await api.messages(sessionID)).messages); setError('') } catch (reason) { setError((reason as Error).message) }
  }
  useEffect(() => { void refresh().then(items => { if (items.length) void loadMessages(items[0].id) }) }, [])
  async function newSession() {
    if (!promptID || !skillID) { setError('当前班级尚无可用提示词或技能'); return }
    setError('')
    try { const item = await api.createSession(Number(promptID), Number(skillID)); const items = await refresh(); setSessions(items); await loadMessages(item.id) } catch (reason) { setError((reason as Error).message) }
  }
  async function send(event: React.FormEvent) {
    event.preventDefault()
    if (!current || !question.trim() || busy) return
    const content = question.trim(); setQuestion(''); setError(''); setBusy(true)
    const localUser: ChatMessage = { id: -Date.now(), role: 'user', content, citations: [], created_at: new Date().toISOString() }
    const localAssistant: ChatMessage = { id: -Date.now() - 1, role: 'assistant', content: '', citations: [], created_at: new Date().toISOString() }
    setMessages(items => [...items, localUser, localAssistant])
    try {
      await api.sendMessage(current, content,
        text => setMessages(items => items.map(message => message.id === localAssistant.id ? { ...message, content: message.content + text } : message)),
        citations => setMessages(items => items.map(message => message.id === localAssistant.id ? { ...message, citations } : message)),
        calls => setMessages(items => items.map(message => message.id === localAssistant.id ? { ...message, tool_calls: calls } : message)))
      await loadMessages(current)
      const items = await refresh(); setSessions(items)
    } catch (reason) { setError((reason as Error).message) } finally { setBusy(false) }
  }
  function editPrompt(id: number) {
    const item = prompts.find(value => value.id === id); if (!item) return
    setPromptEditID(item.id); setPromptName(item.name); setPromptContent(item.content); setPromptActive(item.is_active)
  }
  async function savePrompt(event: React.FormEvent) {
    event.preventDefault(); setError('')
    try { const item = await api.savePrompt({ name: promptName, content: promptContent, is_active: promptActive }, promptEditID); await refresh(); setPromptID(String(item.id)); setPromptEditID(item.id) } catch (reason) { setError((reason as Error).message) }
  }
  function editSkill(id: number) {
    const item = skills.find(value => value.id === id); if (!item) return
    setSkillEditID(item.id); setSkillSlug(item.slug); setSkillName(item.name); setSkillInstructions(item.instructions); setSkillTools((item.tools ?? []).map(tool => tool.ref)); setSkillActive(item.is_active)
  }
  async function saveSkill(event: React.FormEvent) {
    event.preventDefault(); setError('')
    try { const item = await api.saveSkill({ slug: skillSlug, name: skillName, instructions: skillInstructions, tools: skillTools.map(ref => ({ ref })), is_active: skillActive }, skillEditID); await refresh(); setSkillID(String(item.id)); setSkillEditID(item.id) } catch (reason) { setError((reason as Error).message) }
  }
  async function importSkillFile(file?: File) {
    if (!file) return
    if (!/\.(md|txt)$/i.test(file.name) || file.size > 20000) { setError('技能文件仅支持 20 KB 以内的 .md / .txt'); return }
    try {
      const text = await file.text()
      if (!text.trim()) { setError('技能文件内容不能为空'); return }
      setSkillInstructions(text); setSkillFileName(file.name)
      if (!skillEditID) {
        setSkillName(file.name.replace(/\.(md|txt)$/i, '').slice(0, 120))
        setSkillSlug(file.name.replace(/\.(md|txt)$/i, '').toLowerCase().replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 80) || 'uploaded-skill')
      }
      setError('')
    } catch { setError('无法读取技能文件，请确认其为 UTF-8 文本。') }
  }
  async function addMcpServer(event: React.FormEvent) {
    event.preventDefault(); setError('')
    try { await api.createMcpServer(mcpName.trim(), mcpBaseURL.trim()); await refresh() } catch (reason) { setError((reason as Error).message) }
  }
  async function toggleMcpServer(server: MCPServer) {
    setError('')
    try { if (server.connected) await api.disconnectMcpServer(server.id); else await api.connectMcpServer(server.id); await refresh() } catch (reason) { setError((reason as Error).message); await refresh() }
  }
  return <section className="conversation-panel card">
    <div className="panel-heading"><div><span className="eyebrow">STREAMING CLASSROOM CHAT</span><h1>AI 学习会话</h1><p className="muted">使用当前班级知识库检索；回答逐步流式显示并附带来源</p></div><span className="scope-badge">Token 班级隔离</span></div>
    {user.role === 'teacher' && <details className="prompt-manager"><summary>教师管理提示词与技能</summary><div className="manager-grid">
      <form onSubmit={savePrompt}><h3>系统提示词</h3><select aria-label="选择提示词" value={promptEditID ?? ''} onChange={event => editPrompt(Number(event.target.value))}><option value="">新建提示词</option>{prompts.map(item => <option key={item.id} value={item.id}>{item.name}{item.is_active ? '' : '（停用）'}</option>)}</select><input value={promptName} onChange={event => setPromptName(event.target.value)} placeholder="提示词名称" required /><textarea value={promptContent} onChange={event => setPromptContent(event.target.value)} placeholder="提示词模板，可用 {{class_name}}、{{username}}、{{question}}" required /><small className="field-hint">运行时变量：{'{{class_name}}'}、{'{{username}}'}、{'{{question}}'}；由服务端按当前用户与问题填充。</small><label className="manager-active"><input type="checkbox" checked={promptActive} onChange={event => setPromptActive(event.target.checked)} />启用此提示词</label><button>{promptEditID ? '保存提示词' : '创建提示词'}</button></form>
      <form onSubmit={saveSkill}><h3>学习技能</h3><label className="skill-file-import">导入 .md / .txt 技能文件（最多 20 KB）<input type="file" accept=".md,.txt,text/markdown,text/plain" onChange={event => void importSkillFile(event.target.files?.[0])} /></label>{skillFileName && <small className="field-hint">已读取 {skillFileName}；保存后文件全文作为技能指令写入本班技能记录。</small>}<select aria-label="选择技能" value={skillEditID ?? ''} onChange={event => { setSkillFileName(''); if (event.target.value) editSkill(Number(event.target.value)); else { setSkillEditID(undefined); setSkillSlug(''); setSkillName(''); setSkillInstructions(''); setSkillTools([]); setSkillActive(true) } }}><option value="">新建技能</option>{skills.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select><input value={skillSlug} onChange={event => setSkillSlug(event.target.value)} placeholder="唯一标识，例如 guided-solution" required /><input value={skillName} onChange={event => setSkillName(event.target.value)} placeholder="技能名称" required /><textarea value={skillInstructions} onChange={event => setSkillInstructions(event.target.value)} placeholder="技能文件内容或指令" required /><fieldset className="tool-bindings"><legend>绑定已发现工具</legend>{mcpTools.length === 0 ? <small>暂无已连接服务；请先登记并连接 MCP 服务</small> : mcpTools.map(tool => <label key={tool.ref}><input type="checkbox" checked={skillTools.includes(tool.ref)} onChange={event => setSkillTools(value => event.target.checked ? [...value, tool.ref] : value.filter(ref => ref !== tool.ref))} /><span><b>{tool.ref}</b><small>{tool.description}</small></span></label>)}</fieldset><label className="manager-active"><input type="checkbox" checked={skillActive} disabled={skillSlug === 'guided-solution'} onChange={event => setSkillActive(event.target.checked)} />启用此技能 {skillSlug === 'guided-solution' && '（必备引导技能不可停用）'}</label><button>{skillEditID ? '保存技能' : '创建技能'}</button></form>
    </div><section className="mcp-manager"><h3>MCP 工具服务</h3><p className="muted">仅允许 Compose 内部演示服务；令牌保存在服务端，不会返回浏览器。当前 model-mock 不生成原生 tool_calls，演示由服务端按明确规则选择已绑定工具。</p><form className="mcp-register" onSubmit={addMcpServer}><input aria-label="MCP 服务名" value={mcpName} onChange={event => setMcpName(event.target.value)} placeholder="服务名，例如 mcp-demo" required /><input aria-label="MCP 内网地址" value={mcpBaseURL} onChange={event => setMcpBaseURL(event.target.value)} required /><button>登记服务</button></form>{mcpServers.map(server => <article className="mcp-server-row" key={server.id}><div><b>{server.name}</b><small>{server.base_url} · {server.connected ? '已连接' : '未连接'} · 服务端令牌{server.has_token ? '已配置' : '未配置'}</small></div><button type="button" className={server.connected ? 'secondary compact' : 'compact'} onClick={() => void toggleMcpServer(server)}>{server.connected ? '断开并清缓存' : '连接并发现工具'}</button></article>)}<details className="mcp-audit"><summary>最近工具调用审计（{mcpAudit.length}）</summary>{mcpAudit.length === 0 ? <p className="muted">暂无调用记录</p> : mcpAudit.slice(0, 12).map(call => <p key={call.id}><b>{call.tool_name}@{call.server_name}</b> · {call.username} · {call.status}{call.error_code ? ` / ${call.error_code}` : ''} · {new Date(call.created_at).toLocaleString()}</p>)}</details></section></details>}
    <div className="session-toolbar"><label>提示词<select value={promptID} onChange={event => setPromptID(event.target.value)}>{prompts.filter(item => item.is_active).map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label><label>技能<select value={skillID} onChange={event => setSkillID(event.target.value)}>{skills.filter(item => item.is_active).map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label><button type="button" className="secondary compact" onClick={() => { setPromptEditID(undefined); setPromptName(''); setPromptContent(''); setPromptActive(true); setSkillEditID(undefined); setSkillSlug('guided-solution'); setSkillName('解题引导'); setSkillInstructions('通过提问引导学生拆解题目条件并回忆相关概念；只给提示，不直接给出最终答案。'); setSkillActive(true) }}>重置编辑</button><button type="button" onClick={() => void newSession()}>＋ 新建会话</button></div>
    <div className="session-body"><nav className="session-list" aria-label="历史会话">{sessions.length === 0 ? <p className="muted">还没有会话</p> : sessions.map(item => <button type="button" key={item.id} className={current === item.id ? 'selected' : ''} onClick={() => void loadMessages(item.id)}>{item.title}</button>)}</nav>
      <div className="chat-area">{!current ? <div className="empty-state"><span className="empty-icon">✦</span><strong>创建一个学习会话</strong><p>选择提示词与技能后，开启本班知识问答</p></div> : <><div className="chat-messages">{messages.map(message => <article className={`chat-message ${message.role}`} key={message.id}><span>{message.role === 'user' ? '你' : '学习助手'}</span><p>{message.content || (busy ? '正在组织依据回答…' : '')}</p>{message.tool_calls?.length ? <div className="tool-call-status">{message.tool_calls.map((call, index) => <span key={`${call.ref}-${index}`} className={`tool-state ${call.status}`}>工具 {call.ref} · {call.status === 'ok' ? '成功' : call.status === 'denied' ? '已拒绝' : '失败'}</span>)}</div> : null}{message.citations.length > 0 && <div className="chat-citations">{message.citations.map((hit, index) => <div className="source-card" key={hit.chunk_id}><div className="source-card-top"><span className="reference-tag">[{index + 1}]</span><div><h2>{hit.material_title}</h2><p>chunk {hit.chunk_index} · 字符 {hit.start_offset}–{hit.end_offset}</p></div><button type="button" className="secondary source" onClick={() => onSource(hit)}>查看</button></div><blockquote>{hit.excerpt}</blockquote></div>)}</div>}</article>)}</div><form className="chat-input" onSubmit={send}><textarea aria-label="发送消息" value={question} onChange={event => setQuestion(event.target.value)} placeholder="基于本班资料提出问题…" required /><button disabled={busy || !question.trim()}>{busy ? '回答中…' : '发送'}</button></form></>}</div></div>
    {error && <p className="error" role="alert">{error}</p>}
  </section>
}

function KnowledgeWorkspace({ onSource }: { onSource: (hit: SearchHit) => void }) {
  const [tab, setTab] = useState<'search' | 'ask'>('search')
  return <div className="knowledge-workspace">
    <div className="knowledge-tabs" role="tablist" aria-label="知识库功能">
      <button type="button" role="tab" aria-selected={tab === 'search'} className={tab === 'search' ? 'active' : ''} onClick={() => setTab('search')}><span aria-hidden="true">⌕</span>知识检索</button>
      <button type="button" role="tab" aria-selected={tab === 'ask'} className={tab === 'ask' ? 'active' : ''} onClick={() => setTab('ask')}><span aria-hidden="true">✦</span>知识问答</button>
    </div>
    {tab === 'search' ? <KnowledgeSearch onSource={onSource} /> : <KnowledgeAsk onSource={onSource} />}
  </div>
}

function KnowledgeSearch({ onSource }: { onSource: (hit: SearchHit) => void }) {
  const [query, setQuery] = useState('')
  const [mode, setMode] = useState<SearchMode>('hybrid')
  const [hits, setHits] = useState<SearchHit[]>([])
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(event: React.FormEvent) {
    event.preventDefault(); setBusy(true); setError(''); setMessage('')
    try {
      const result = await api.search(query, mode)
      setHits(result.hits)
      setMessage(result.message ?? '')
    } catch (reason) {
      setHits([])
      const apiError = reason as ApiError
      setError(apiError.status === 503 ? '检索依赖暂不可用，请稍后重试。' : apiError.message)
    } finally { setBusy(false) }
  }
  return <section className="search-panel card"><div className="panel-heading"><div><span className="eyebrow">KNOWLEDGE RETRIEVAL</span><h1>知识库检索</h1><p className="muted">在本班已索引材料中查找可追溯内容</p></div><span className="scope-badge">仅限本班</span></div>
    <form className="search-form" onSubmit={submit}><div className="input-shell"><span aria-hidden="true">⌕</span><input aria-label="检索问题" value={query} onChange={event => setQuery(event.target.value)} placeholder="输入关键词或自然语言问题" required /></div><select aria-label="检索模式" value={mode} onChange={event => setMode(event.target.value as SearchMode)}><option value="hybrid">混合检索</option><option value="keyword">关键词</option><option value="vector">向量</option></select><button disabled={busy}>{busy ? <><span className="spinner" />检索中…</> : '开始检索'}</button></form>
    {busy && <p className="loading-line"><span className="spinner dark" />正在检索本班知识库…</p>}{error && <p className="error" role="alert">{error}</p>}{message && <div className="empty-state"><span className="empty-icon" aria-hidden="true">⌕</span><strong>未找到相关资料</strong><p>当前班级知识库中没有足够相关的内容</p></div>}
    {hits.length > 0 && <div className="result-summary"><strong>检索结果</strong><span>共 {hits.length} 条可追溯片段</span></div>}
    <div className="search-results">{hits.map(hit => <article className="source-card" key={hit.chunk_id}><div className="source-card-top"><span className="reference-tag">#{hit.rank}</span><div><h2>{hit.material_title}</h2><p>chunk {hit.chunk_index} · 字符 {hit.start_offset}–{hit.end_offset}</p></div><button type="button" className="secondary source" onClick={() => onSource(hit)}>查看材料</button></div><blockquote>{hit.excerpt}</blockquote><div className="score-row"><span>{mode === 'hybrid' ? 'RRF 融合分数' : '相关度'} {hit.score.toFixed(4)}</span>{hit.keyword_rank && <span>关键词 #{hit.keyword_rank}</span>}{hit.vector_rank && <span>向量 #{hit.vector_rank}</span>}</div></article>)}</div>
  </section>
}

function KnowledgeAsk({ onSource }: { onSource: (hit: SearchHit) => void }) {
  const [question, setQuestion] = useState('')
  const [answer, setAnswer] = useState('')
  const [submittedQuestion, setSubmittedQuestion] = useState('')
  const [citations, setCitations] = useState<SearchHit[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(event: React.FormEvent) {
    event.preventDefault(); setBusy(true); setError(''); setAnswer(''); setCitations([]); setSubmittedQuestion(question)
    try {
      const result = await api.ask(question)
      setAnswer(result.answer)
      setCitations(result.citations)
    } catch (reason) {
      const apiError = reason as ApiError
      setError(apiError.status === 503 ? '问答依赖暂不可用，请稍后重试。' : apiError.message)
    } finally { setBusy(false) }
  }
  const noResults = answer === '资料中未找到相关内容' && citations.length === 0
  return <section className="search-panel card"><div className="panel-heading"><div><span className="eyebrow">GROUNDED QA</span><h1>知识问答</h1><p className="muted">基于本班知识库进行检索增强问答，回答仅依据可追溯资料生成</p></div><span className="scope-badge">依据可追溯</span></div>
    <form className="ask-form" onSubmit={submit}><textarea aria-label="知识问答问题" value={question} onChange={event => setQuestion(event.target.value)} placeholder="请输入你想查询的问题，例如：做实验之前需要先进行什么准备？" required /><div className="ask-actions"><span>回答将基于本班知识库，并附带引用来源</span><button disabled={busy}>{busy ? <><span className="spinner" />正在生成…</> : <><span aria-hidden="true">✦</span>提问</>}</button></div></form>
    {busy && <div className="thinking-card"><span className="spinner dark" /><div><strong>正在查找相关资料</strong><p>CampusClaw 正在检索本班知识库并组织依据回答…</p></div></div>}
    {error && <p className="error" role="alert">{error}</p>}
    {!busy && answer && submittedQuestion && <div className="question-card"><span>你的问题</span><p>{submittedQuestion}</p></div>}
    {!busy && noResults && <div className="empty-state ask-empty"><span className="empty-icon" aria-hidden="true">⌕</span><strong>未找到相关资料</strong><p>当前班级知识库中没有足够相关的内容</p></div>}
    {!busy && answer && !noResults && <div className="answer-card"><div className="answer-heading"><span className="answer-symbol" aria-hidden="true">✦</span><div><span className="answer-label">依据回答</span><small>内容由本班资料支持</small></div></div><p>{highlightCitations(answer)}</p></div>}
    {!busy && citations.length > 0 && <div className="citation-list"><div className="result-summary"><strong>引用来源</strong><span>{citations.length} 条资料片段</span></div>{citations.map((citation, index) => <article className="source-card" key={citation.chunk_id}>
      <div className="source-card-top"><span className="reference-tag">[{index + 1}]</span><div><h2>{citation.material_title}</h2><p>chunk {citation.chunk_index} · 字符 {citation.start_offset}–{citation.end_offset}</p></div><button type="button" className="secondary source" onClick={() => onSource(citation)}>查看材料</button></div>
      <blockquote>{citation.excerpt}</blockquote>
    </article>)}</div>}
  </section>
}

function highlightCitations(answer: string) {
  return answer.split(/(\[\d+\])/g).map((part, index) => /^\[\d+\]$/.test(part) ? <mark className="inline-citation" key={`${part}-${index}`}>{part}</mark> : part)
}

function App() {
  const [user, setUser] = useState<User | null | undefined>(undefined)
  const [identityNotice, setIdentityNotice] = useState('')
  useEffect(() => {
    if (!getToken()) { setUser(null); return }
    api.me().then(setUser).catch(reason => {
      if (reason instanceof ApiError && reason.status === 401) clearToken()
      setUser(null)
    })
  }, [])
  if (user === undefined) return <main className="center"><p>正在载入…</p></main>
  if (!user) return <Login onLogin={(authenticatedUser, selectedRole) => { setIdentityNotice(authenticatedUser.role === selectedRole ? '' : `当前账号为${authenticatedUser.role === 'teacher' ? '教师' : '学生'}账号`); setUser(authenticatedUser) }} />
  return <Materials user={user} identityNotice={identityNotice} onLogout={async () => { await api.logout().catch(() => undefined); clearToken(); setIdentityNotice(''); setUser(null) }} />
}

createRoot(document.getElementById('root')!).render(<StrictMode><App /></StrictMode>)
