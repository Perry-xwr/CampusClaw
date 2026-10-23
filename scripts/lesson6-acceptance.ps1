param(
  [string]$BaseUrl = 'http://localhost:8080',
  [int]$HealthTimeoutSeconds = 240
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $root

$reportDir = Join-Path $root 'apitest/reports/lesson6-local'
$logPath = Join-Path $reportDir 'acceptance.log'
$summaryPath = Join-Path $reportDir 'summary.md'
$script:Results = [System.Collections.Generic.List[object]]::new()
$script:Secrets = [System.Collections.Generic.List[string]]::new()
$script:McpToken = $null
$script:OriginalToken = [Environment]::GetEnvironmentVariable('MCP_AUTH_TOKEN', 'Process')
$script:OriginalTrace = [Environment]::GetEnvironmentVariable('CAMPUSCLAW_TEST_TRACE', 'Process')
$script:EngineAvailable = $false
$script:StartedAt = Get-Date
$script:BlockingError = ''

New-Item -ItemType Directory -Path $reportDir -Force | Out-Null
Set-Content -LiteralPath $logPath -Value "CampusClaw Lesson 6 local acceptance`r`nStarted: $($script:StartedAt.ToString('o'))`r`n" -Encoding utf8

function Add-Result {
  param([string]$Area, [string]$Name, [string]$State, [string]$Detail)
  $script:Results.Add([pscustomobject]@{ Area = $Area; Name = $Name; State = $State; Detail = (Protect-Text $Detail) })
}

function Protect-Text {
  param([string]$Text)
  $safe = $Text
  foreach ($secret in $script:Secrets) {
    if ($secret) { $safe = $safe.Replace($secret, '[REDACTED]') }
  }
  return $safe
}

function Invoke-Native {
  param([string]$Area, [string]$Name, [string]$Executable, [string[]]$Arguments)
  Add-Content -LiteralPath $logPath -Value "`r`n## $Area / $Name`r`n> $Executable $($Arguments -join ' ')" -Encoding utf8
  $previousErrorAction = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  try {
    $lines = @(& $Executable @Arguments 2>&1)
    $code = $LASTEXITCODE
  }
  finally {
    $ErrorActionPreference = $previousErrorAction
  }
  foreach ($line in $lines) {
    Add-Content -LiteralPath $logPath -Value (Protect-Text ([string]$line)) -Encoding utf8
  }
  Add-Content -LiteralPath $logPath -Value "ExitCode=$code" -Encoding utf8
  return [pscustomobject]@{ ExitCode = $code; Output = ($lines | ForEach-Object { Protect-Text ([string]$_) }) -join "`n" }
}

function Require-NativeSuccess {
  param([string]$Area, [string]$Name, [string]$Executable, [string[]]$Arguments)
  $result = Invoke-Native $Area $Name $Executable $Arguments
  if ($result.ExitCode -ne 0) {
    Add-Result $Area $Name 'FAIL' "退出码 $($result.ExitCode)；详见 acceptance.log"
    throw "$Name failed with exit code $($result.ExitCode)"
  }
  return $result
}

function Get-ContainerHealth {
  param([string]$Service, [string]$Docker)
  $ids = @(& $Docker compose ps -q $Service 2>$null)
  if ($LASTEXITCODE -ne 0 -or $ids.Count -eq 0 -or -not $ids[0]) { return 'missing' }
  $state = & $Docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}no-healthcheck{{end}}' $ids[0] 2>$null
  if ($LASTEXITCODE -ne 0) { return 'inspect-error' }
  return ([string]$state).Trim()
}

function Write-Summary {
  $passed = @($script:Results | Where-Object State -eq 'PASS').Count
  $failed = @($script:Results | Where-Object State -eq 'FAIL').Count
  $notRun = @($script:Results | Where-Object State -eq 'NOT RUN').Count
  $table = @($script:Results | ForEach-Object { "| $($_.Area) | $($_.Name) | $($_.State) | $($_.Detail -replace '\|', '\|') |" })
  $peerChecks = @(
    '| 同伴交叉验证 | 待同学实际操作并提供可核验证据 | NOT TESTED | 不伪造同伴身份、操作或截图 |',
    '| 同伴证据 1 | 同学 A 在教师端登记、连接 MCP Server 并发现工具 | 待提供 | 请保留操作截图或脱敏录屏 |',
    '| 同伴证据 2 | 同学 B 绑定技能并由学生会话触发工具调用 | 待提供 | 请保留 SSE meta.tool_calls 截图 |',
    '| 同伴证据 3 | 同学 C 验证学生权限限制和班级隔离 | 待提供 | 请保留 HTTP 状态与账号班级说明 |',
    '| 同伴证据 4 | 同学 D 检查调用审计及断开后的拒绝调用 | 待提供 | 请保留审计记录和断开后结果 |'
  )
  $healthLines = @()
  if ($script:Docker) {
    foreach ($service in @('db', 'qdrant', 'model-mock', 'mcp-demo', 'api', 'web')) {
      $healthLines += "| $service | $(Get-ContainerHealth $service $script:Docker) |"
    }
  }
  $content = [System.Collections.Generic.List[string]]::new()
  [void]$content.Add('# CampusClaw Lesson 6 本地验收报告')
  [void]$content.Add('')
  [void]$content.Add("- 开始时间：$($script:StartedAt.ToString('o'))")
  [void]$content.Add("- 结束时间：$((Get-Date).ToString('o'))")
  [void]$content.Add("- 目标地址：$BaseUrl")
  [void]$content.Add("- 结果计数：PASS=$passed / FAIL=$failed / NOT RUN=$notRun")
  [void]$content.Add('- 课程交叉验证：NOT TESTED；四项同伴证据待真实补充，自动化通过不代表课程最终验收完成。')
  [void]$content.Add("- 阻塞原因：$(if ($script:BlockingError) { $script:BlockingError } else { '无' })")
  [void]$content.Add('- MCP Token：使用本次进程内生成的随机值，仅传入 Compose 子进程；未写入 `.env`、源码、镜像构建参数或报告。')
  [void]$content.Add('- Go 单测：API Dockerfile 构建步骤包含 `go test ./...`；请根据 acceptance.log 的 BuildKit 输出区分实际执行和缓存命中。')
  [void]$content.Add('')
  [void]$content.Add('## 阶段结果')
  [void]$content.Add('')
  [void]$content.Add('| 阶段 | 检查 | 状态 | 说明 |')
  [void]$content.Add('|---|---|---|---|')
  foreach ($row in $table) { [void]$content.Add([string]$row) }
  [void]$content.Add('')
  [void]$content.Add('## Compose 服务状态')
  [void]$content.Add('')
  [void]$content.Add('| 服务 | 状态 |')
  [void]$content.Add('|---|---|')
  foreach ($row in $healthLines) { [void]$content.Add([string]$row) }
  [void]$content.Add('')
  [void]$content.Add('## MCP 端到端覆盖')
  [void]$content.Add('')
  [void]$content.Add('全量 apitest 同时覆盖 Lesson 5 的提示词、技能、会话、SSE 与 Lesson 6 的 MCP 集成用例。Lesson 6 用例覆盖 initialize/tools/list（教师连接）、工具调用、技能绑定、学生 SSE meta.tool_calls、审计记录、断开后拒绝调用及权限/班级隔离。逐项结果以 apitest HTML 报告为准；若 apitest 未运行或失败，不得视为通过。')
  [void]$content.Add('')
  [void]$content.Add('## 同伴交叉验证证据（需真实补充）')
  [void]$content.Add('')
  [void]$content.Add('| 项目 | 要求 | 状态 | 证据 |')
  [void]$content.Add('|---|---|---|---|')
  foreach ($row in $peerChecks) { [void]$content.Add([string]$row) }
  [void]$content.Add('')
  [void]$content.Add('## 产物')
  [void]$content.Add('')
  [void]$content.Add("- 详细命令日志：``$logPath``（敏感值已脱敏）")
  [void]$content.Add('- apitest HTML 报告：`apitest/reports/` 下由 runner 实际生成的报告。')
  [void]$content.Add('- 本地报告目录由 `.gitignore` 忽略；未执行 commit、merge、push。')
  Set-Content -LiteralPath $summaryPath -Value ($content -join "`r`n") -Encoding utf8
  Write-Output "ACCEPTANCE_SUMMARY PASS=$passed FAIL=$failed NOT_RUN=$notRun"
  Write-Output 'PEER_CROSS_VALIDATION=NOT_TESTED; four real peer evidence items remain outstanding'
  Write-Output "SUMMARY=$summaryPath"
  Write-Output "LOG=$logPath"
}

try {
  $git = (Get-Command git.exe -ErrorAction Stop).Source
  $ignore = Invoke-Native '预检' '确认验收产物目录被 Git 忽略' $git @('check-ignore', '--quiet', 'apitest/reports/lesson6-local/acceptance.log')
  if ($ignore.ExitCode -ne 0) { throw '验收报告目录未被 Git 忽略；为避免敏感日志进入 Git，已停止。' }
  Add-Result '预检' '验收报告目录 Git 忽略规则' 'PASS' 'apitest/reports/lesson6-local/ 已忽略'

  if (-not (Test-Path -LiteralPath (Join-Path $root '.env'))) { throw '缺少项目 .env；脚本不会创建或覆盖它。' }
  $dotenv = @{}
  foreach ($line in Get-Content -LiteralPath (Join-Path $root '.env')) {
    if ($line -match '^\s*([^#\s][^=]*)=(.*)$') { $dotenv[$matches[1].Trim()] = $matches[2].Trim().Trim('"').Trim("'") }
  }
  foreach ($key in @('MYSQL_DATABASE', 'MYSQL_USER', 'MYSQL_PASSWORD', 'MYSQL_ROOT_PASSWORD')) {
    if (-not $dotenv[$key]) { throw ".env 缺少必需变量 $key；未修改 .env。" }
  }
  foreach ($key in $dotenv.Keys) {
    if ($key -match '(?i)(PASSWORD|TOKEN|SECRET|API.?KEY)') { $script:Secrets.Add([string]$dotenv[$key]) }
  }
  $python = (Get-Command python.exe -ErrorAction SilentlyContinue).Source
  if (-not $python) { $python = (Get-Command py.exe -ErrorAction SilentlyContinue).Source }
  if (-not $python) { throw '未找到 Python；请安装 Python 3.10+ 后重试。' }
  $pythonVersion = Invoke-Native '预检' 'Python 版本与标准库' $python @('-c', 'import sys; assert sys.version_info >= (3,10), sys.version; import urllib.request, html.parser; print(sys.version.split()[0])')
  if ($pythonVersion.ExitCode -ne 0) { throw 'apitest 要求 Python 3.10+ 且标准库可用。' }
  Add-Result '预检' 'Python 3.10+ / apitest 标准库' 'PASS' $pythonVersion.Output.Trim()

  $dockerCommand = Get-Command docker.exe -ErrorAction SilentlyContinue
  if (-not $dockerCommand) { throw '未找到 docker.exe。' }
  $script:Docker = $dockerCommand.Source
  $dockerInfo = Invoke-Native '预检' 'Docker Engine 可访问' $script:Docker @('info', '--format', '{{.ServerVersion}}')
  if ($dockerInfo.ExitCode -ne 0) { throw 'Docker Engine 不可访问；请确认宿主机 Docker Desktop 正常运行。' }
  $script:EngineAvailable = $true
  Add-Result '预检' 'Docker Engine' 'PASS' "Server $($dockerInfo.Output.Trim())"

  $tokenBytes = New-Object byte[] 32
  $random = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try { $random.GetBytes($tokenBytes) } finally { $random.Dispose() }
  $script:McpToken = [Convert]::ToBase64String($tokenBytes).TrimEnd('=').Replace('+', '-').Replace('/', '_')
  $script:Secrets.Add($script:McpToken)
  [Environment]::SetEnvironmentVariable('MCP_AUTH_TOKEN', $script:McpToken, 'Process')
  [Environment]::SetEnvironmentVariable('CAMPUSCLAW_TEST_TRACE', 'true', 'Process')

  $composeConfig = Require-NativeSuccess '预检' 'Compose 配置解析' $script:Docker @('compose', 'config', '--quiet')
  Add-Result '预检' 'Compose 配置解析' 'PASS' "退出码 $($composeConfig.ExitCode)"
  $services = @('db', 'qdrant', 'model-mock')
  foreach ($service in $services) {
    $health = Get-ContainerHealth $service $script:Docker
    Add-Content -LiteralPath $logPath -Value "PreflightHealth $service=$health" -Encoding utf8
    if ($health -ne 'running healthy') { throw "现有依赖服务 $service 状态为 '$health'，要求 running healthy；停止以保护现有环境。" }
    Add-Result '预检' "$service 健康状态" 'PASS' $health
  }

  $requiredImages = @('docker.m.daocloud.io/library/mysql:8.0', 'qdrant/qdrant:v1.12.6', 'python:3.13-alpine', 'golang:1.24-alpine', 'alpine:3.21', 'node:24-alpine', 'nginx:1.29-alpine')
  foreach ($image in $requiredImages) {
    $imageCheck = Invoke-Native '预检' "本地基础镜像 $image" $script:Docker @('image', 'inspect', $image, '--format', '{{.Id}}')
    if ($imageCheck.ExitCode -ne 0) { throw "缺少本地基础镜像 $image；脚本不会主动拉取镜像。" }
    Add-Result '预检' "本地基础镜像 $image" 'PASS' '已存在；后续构建使用 --pull=false'
  }

  foreach ($service in @('model-mock', 'mcp-demo', 'api', 'web')) {
    $build = Require-NativeSuccess '构建' "$service 镜像构建（不拉取基础镜像）" $script:Docker @('compose', '--progress', 'plain', 'build', '--pull=false', $service)
    Add-Result '构建' "$service 镜像构建" 'PASS' 'docker compose build --pull=false 成功；详细 BuildKit 输出见日志'
  }

  Require-NativeSuccess '启动' '以本次临时 MCP Token 启动测试追踪版 model-mock 与 mcp-demo/api/web' $script:Docker @('compose', 'up', '-d', '--no-build', 'model-mock', 'mcp-demo', 'api', 'web') | Out-Null
  Add-Result '启动' 'Compose 更新服务' 'PASS' '只更新 model-mock、mcp-demo、api、web；未使用 down 或清理数据卷'
  # Nginx resolves the API service address when it starts. If Compose replaced
  # only the API container, restart web so its proxy no longer targets the old IP.
  $webProxy = Require-NativeSuccess '启动' '重启 web 刷新 API 服务地址' $script:Docker @('compose', 'restart', 'web')
  Add-Result '启动' 'web 反向代理刷新 API 地址' 'PASS' 'web 已重启以重新解析 Compose 内部 api 服务地址'

  $deadline = (Get-Date).AddSeconds($HealthTimeoutSeconds)
  $unhealthy = @()
  do {
    $unhealthy = @()
    foreach ($service in @('db', 'qdrant', 'model-mock', 'mcp-demo', 'api', 'web')) {
      $health = Get-ContainerHealth $service $script:Docker
      if ($health -ne 'running healthy') { $unhealthy += "$service=$health" }
    }
    if ($unhealthy.Count -gt 0) { Start-Sleep -Seconds 5 }
  } while ($unhealthy.Count -gt 0 -and (Get-Date) -lt $deadline)
  if ($unhealthy.Count -gt 0) {
    Add-Result '启动' '六个 Compose 服务 Healthy' 'FAIL' "健康检查超时：$($unhealthy -join ', ')"
    throw "健康检查超时：$($unhealthy -join ', ')"
  }
  Add-Result '启动' '六个 Compose 服务 Healthy' 'PASS' 'db、qdrant、model-mock、mcp-demo、api、web 均为 running healthy'
  $composePs = Require-NativeSuccess '启动' 'docker compose ps' $script:Docker @('compose', 'ps')
  Add-Result '启动' 'docker compose ps' 'PASS' '输出已写入验收日志'

  $healthResponse = Invoke-Native 'HTTP' 'GET /health' $python @('-c', "import urllib.request; r=urllib.request.urlopen('$BaseUrl/health',timeout=10); print(r.status, r.read().decode('utf-8'))")
  if ($healthResponse.ExitCode -ne 0) { Add-Result 'HTTP' 'API /health' 'FAIL' "退出码 $($healthResponse.ExitCode)"; throw '/health HTTP 检查失败。' }
  Add-Result 'HTTP' 'API /health' 'PASS' $healthResponse.Output.Trim()

  $apiTests = Invoke-Native '测试' '全部 apitest（包含 Lesson 6 MCP 用例）' $python @('apitest/run_all.py', '--base-url', $BaseUrl)
  $summaryMatch = [regex]::Match($apiTests.Output, 'API_TEST_SUMMARY PASS=(\d+) FAIL=(\d+)')
  if (-not $summaryMatch.Success) {
    Add-Result '测试' 'apitest 汇总输出' 'FAIL' "runner exit code=$($apiTests.ExitCode)；未找到 API_TEST_SUMMARY"
    throw 'apitest did not emit its real PASS/FAIL summary.'
  }
  $reportMatch = [regex]::Match($apiTests.Output, 'HTML_REPORT=(.+)')
  if (-not $reportMatch.Success) {
    Add-Result '测试' 'apitest HTML 报告路径' 'FAIL' 'runner 未输出 HTML_REPORT 路径'
    throw 'apitest did not report its HTML output path.'
  }
  $reportPath = $reportMatch.Groups[1].Value.Trim()
  if (-not [System.IO.Path]::IsPathRooted($reportPath)) { $reportPath = Join-Path $root $reportPath }
  if (-not (Test-Path -LiteralPath $reportPath -PathType Leaf)) {
    Add-Result '测试' 'apitest HTML 报告生成' 'FAIL' 'runner 输出的 HTML 报告文件不存在'
    throw 'apitest HTML report was not generated.'
  }
  Add-Result '测试' 'apitest HTML 报告生成' 'PASS' $reportPath
  $html = Get-Content -LiteralPath $reportPath -Raw -Encoding utf8
  $casePattern = "<tr class='(pass|fail)'><td>(.*?)</td><td>(PASS|FAIL)</td><td>(.*?)</td><td>([0-9.]+)s</td></tr>"
  $caseMatches = [regex]::Matches($html, $casePattern, [System.Text.RegularExpressions.RegexOptions]::Singleline)
  foreach ($case in $caseMatches) {
    $caseName = [System.Net.WebUtility]::HtmlDecode($case.Groups[2].Value)
    $caseState = $case.Groups[3].Value
    $caseDetail = [System.Net.WebUtility]::HtmlDecode($case.Groups[4].Value)
    Add-Result 'apitest' $caseName $caseState $caseDetail
  }
  $casePass = @($caseMatches | Where-Object { $_.Groups[1].Value -eq 'pass' }).Count
  $caseFail = @($caseMatches | Where-Object { $_.Groups[1].Value -eq 'fail' }).Count
  Add-Result '测试' 'apitest runner 汇总' 'INFO' "exit code=$($apiTests.ExitCode), PASS=$($summaryMatch.Groups[1].Value), FAIL=$($summaryMatch.Groups[2].Value); HTML cases PASS=$casePass FAIL=$caseFail"
  if ($caseMatches.Count -eq 0 -or $casePass -ne [int]$summaryMatch.Groups[1].Value -or $caseFail -ne [int]$summaryMatch.Groups[2].Value) {
    Add-Result '测试' 'apitest 汇总与 HTML 明细一致性' 'FAIL' "runner PASS/FAIL=$($summaryMatch.Groups[1].Value)/$($summaryMatch.Groups[2].Value), HTML PASS/FAIL=$casePass/$caseFail"
    throw 'apitest summary does not match its individual HTML case rows.'
  }
  if ($apiTests.ExitCode -ne 0 -or $summaryMatch.Groups[2].Value -ne '0') {
    throw 'apitest runner returned failure; individual cases and exact errors are listed above.'
  }
  $experiments = Invoke-Native '实验' '五项 LLM/MCP 边界取证与数据库核对' $python @('apitest/personal_stage_experiments.py')
  if ($experiments.ExitCode -ne 0 -or $experiments.Output -notmatch 'EXPERIMENTS_SUMMARY PASS=\d+ FAIL=0') {
    Add-Result '实验' '个人项目五项边界实验' 'FAIL' "退出码 $($experiments.ExitCode)；详见 acceptance.log"
    throw 'personal-stage evidence experiments failed.'
  }
  $evidenceDir = Join-Path $root 'apitest/reports/personal-stage-acceptance-20261021'
  foreach ($evidenceFile in @('experiments.json', 'model-http-traces.json', 'mcp-http-traces.json')) {
    if (-not (Test-Path -LiteralPath (Join-Path $evidenceDir $evidenceFile) -PathType Leaf)) { throw "Missing experiment evidence file $evidenceFile" }
  }
  Add-Result '实验' '五项 LLM/MCP 边界实验' 'PASS' $experiments.Output.Trim()
  Add-Result '回归' 'Lesson 5 API/SSE（全量 apitest 覆盖）' 'INFO' "PASS=$casePass FAIL=0；逐项结果见 apitest 明细"
  $mcpRows = [regex]::Matches($html, "<tr class='(pass|fail)'><td>([^<]+)</td>") | Where-Object { $_.Groups[2].Value -match 'api/mcp/|api/skills/(post|id/put)|api/sessions/(post|get|id/messages/post)' }
  $mcpPass = @($mcpRows | Where-Object { $_.Groups[1].Value -eq 'pass' }).Count
  $mcpFail = @($mcpRows | Where-Object { $_.Groups[1].Value -eq 'fail' }).Count
  if ($mcpRows.Count -eq 0) { Add-Result 'Lesson 6' 'MCP 端到端用例' 'NOT RUN' '无法从 HTML 报告提取 MCP 用例行' }
  else { Add-Result 'Lesson 6' 'MCP 端到端用例' 'INFO' "已从逐项 apitest 明细识别 PASS=$mcpPass FAIL=$mcpFail" }

  $verify = Invoke-Native '回归' 'scripts/verify.ps1（Lesson 3/4）' 'powershell.exe' @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', 'scripts/verify.ps1', '-BaseUrl', $BaseUrl)
  if ($verify.ExitCode -ne 0) {
    Add-Result '回归' 'scripts/verify.ps1' 'FAIL' "退出码 $($verify.ExitCode)；详见 acceptance.log"
    throw 'scripts/verify.ps1 returned non-zero.'
  }
  foreach ($lesson in @('LESSON3_SUMMARY PASS=22 FAIL=0', 'LESSON4_SUMMARY PASS=22 FAIL=0')) {
    if ($verify.Output.Contains($lesson)) { Add-Result '回归' $lesson 'PASS' '真实输出匹配预期' }
    else { Add-Result '回归' $lesson 'FAIL' '脚本退出码为 0，但未找到该汇总行' }
  }
  if (@($script:Results | Where-Object { $_.Area -eq '回归' -and $_.State -eq 'FAIL' }).Count -gt 0) { throw '回归汇总与预期不匹配。' }

  Add-Result '安全' '测试账号数据清理' 'PASS' 'apitest runner 返回 0；本轮清理无失败报告，历史用户、材料及数据卷不清理'
  Add-Result '人工项' '四项同伴交叉验证' 'INFO' '自动化未执行；四项证据在报告中逐项标记为 NOT TESTED'
}
catch {
  $script:BlockingError = Protect-Text ([string]$_.Exception.Message)
  if (-not @($script:Results | Where-Object State -eq 'FAIL').Count) { Add-Result '流程' '本次验收' 'FAIL' $script:BlockingError }
  Add-Content -LiteralPath $logPath -Value "`r`n## STOPPED`r`n$(Protect-Text ([string]$_.Exception.ToString()))" -Encoding utf8
  if ($script:EngineAvailable) {
    $null = Invoke-Native '故障诊断' '最近 200 行 MCP/API/Web 容器日志（脱敏）' $script:Docker @('compose', 'logs', '--tail', '200', 'mcp-demo', 'api', 'web')
  }
}
finally {
  foreach ($service in @('model-mock', 'mcp-demo', 'api', 'web')) {
    $alreadyRecorded = @($script:Results | Where-Object { $_.Area -eq '构建' -and $_.Name -like "$service 镜像构建*" }).Count -gt 0
    if (-not $alreadyRecorded) { Add-Result '构建' "$service 镜像构建" 'NOT RUN' '流程在该镜像构建前提前停止' }
  }
  if (-not @($script:Results | Where-Object { $_.Area -eq '启动' -and $_.Name -eq 'Compose 更新服务' }).Count) {
    Add-Result '启动' 'Compose 更新服务' 'NOT RUN' '流程未成功完成 Compose 服务更新'
  }
  if (-not @($script:Results | Where-Object { $_.Area -eq '启动' -and $_.Name -eq '六个 Compose 服务 Healthy' }).Count) {
    Add-Result '启动' '六个 Compose 服务 Healthy' 'NOT RUN' '六服务健康检查未全部完成'
  }
  if (-not @($script:Results | Where-Object { $_.Area -eq 'HTTP' -and $_.Name -eq 'API /health' }).Count) {
    Add-Result 'HTTP' 'API /health' 'NOT RUN' '流程未到达 HTTP 健康检查'
  }
  if (-not @($script:Results | Where-Object { $_.Area -eq '测试' -and $_.Name -eq 'apitest HTML 报告生成' }).Count) {
    Add-Result '测试' '全量 apitest' 'NOT RUN' '流程未完成 apitest 报告生成'
  }
  if (-not @($script:Results | Where-Object { $_.Area -eq '回归' }).Count) {
    Add-Result '回归' 'Lesson 3/4 verify.ps1' 'NOT RUN' 'apitest 未全部通过，或流程未到达回归阶段'
  }
  if ($script:EngineAvailable -and $script:McpToken -and $script:Docker) {
    [Environment]::SetEnvironmentVariable('CAMPUSCLAW_TEST_TRACE', 'false', 'Process')
    $traceOff = Invoke-Native '安全收尾' '关闭测试追踪并保持相同 MCP Token' $script:Docker @('compose', 'up', '-d', '--no-build', 'model-mock', 'mcp-demo', 'api', 'web')
    if ($traceOff.ExitCode -eq 0) {
      $traceHealthDeadline = (Get-Date).AddSeconds($HealthTimeoutSeconds)
      $traceHealth = @()
      do {
        $traceHealth = @()
        foreach ($service in @('db', 'qdrant', 'model-mock', 'mcp-demo', 'api', 'web')) {
          $health = Get-ContainerHealth $service $script:Docker
          if ($health -ne 'running healthy') { $traceHealth += "$service=$health" }
        }
        if ($traceHealth.Count -gt 0) { Start-Sleep -Seconds 5 }
      } while ($traceHealth.Count -gt 0 -and (Get-Date) -lt $traceHealthDeadline)
      if ($traceHealth.Count -eq 0) { Add-Result '安全收尾' '追踪关闭且六服务恢复 Healthy' 'PASS' '临时追踪已关闭；API 与 MCP Server 保持同一进程内随机令牌' }
      else { Add-Result '安全收尾' '追踪关闭后服务健康' 'FAIL' ($traceHealth -join ', ') }
    } else {
      Add-Result '安全收尾' '关闭测试追踪' 'FAIL' "退出码 $($traceOff.ExitCode)；详见 acceptance.log"
    }
  }
  if ($null -eq $script:OriginalToken) { [Environment]::SetEnvironmentVariable('MCP_AUTH_TOKEN', $null, 'Process') }
  else { [Environment]::SetEnvironmentVariable('MCP_AUTH_TOKEN', $script:OriginalToken, 'Process') }
  if ($null -eq $script:OriginalTrace) { [Environment]::SetEnvironmentVariable('CAMPUSCLAW_TEST_TRACE', $null, 'Process') }
  else { [Environment]::SetEnvironmentVariable('CAMPUSCLAW_TEST_TRACE', $script:OriginalTrace, 'Process') }
  Add-Content -LiteralPath $logPath -Value "`r`nFinished: $((Get-Date).ToString('o'))" -Encoding utf8
  Write-Summary
}

if (@($script:Results | Where-Object { $_.State -eq 'FAIL' -or $_.State -eq 'NOT RUN' }).Count -gt 0) { exit 1 }
exit 0
