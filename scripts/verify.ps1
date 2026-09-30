param(
  [string]$BaseUrl = 'http://localhost:8080'
)

$ErrorActionPreference = 'Stop'
$PSDefaultParameterValues['Invoke-WebRequest:UseBasicParsing'] = $true
$settings = @{}
Get-Content -LiteralPath '.env' | Where-Object { $_ -match '^[^#][^=]*=' } | ForEach-Object {
  $parts = $_ -split '=', 2
  $settings[$parts[0].Trim()] = $parts[1].Trim()
}
$dbUser = $settings['MYSQL_USER']
$dbPassword = $settings['MYSQL_PASSWORD']
$dbName = $settings['MYSQL_DATABASE']

function Assert-Status {
  param([scriptblock]$Request, [int]$Expected, [string]$Name)
  try {
    $response = & $Request
    $actual = [int]$response.StatusCode
  } catch {
    $actual = [int]$_.Exception.Response.StatusCode
  }
  if ($actual -ne $Expected) { throw "$Name failed: expected HTTP $Expected, got $actual" }
  Write-Output "PASS $Name ($actual)"
}

function Bearer-Headers([string]$Token) {
  return @{ Authorization = "Bearer $Token" }
}

function Login {
  param([string]$Username, [string]$Password)
  $body = @{ username = $Username; password = $Password } | ConvertTo-Json
  Invoke-RestMethod -Uri "$BaseUrl/api/login" -Method Post -ContentType 'application/json' -Body $body
}

function Db-Count([string]$Table) {
  $query = "SELECT COUNT(*) FROM $Table"
  $line = docker compose exec -T -e "MYSQL_PWD=$dbPassword" db mysql -N "-u$dbUser" $dbName -e $query
  if ($LASTEXITCODE -ne 0) { throw "Could not count $Table" }
  return [int]($line | Select-Object -Last 1)
}

function Upload-Count {
  $line = docker compose exec -T api sh -c "find /data/uploads -type f ! -name '.incoming-*' | wc -l"
  if ($LASTEXITCODE -ne 0) { throw 'Could not count uploads' }
  return [int]($line | Select-Object -Last 1)
}

function Invoke-UploadRaw {
  param(
    [string]$Token,
    [string]$File,
    [string]$Title = '',
    [string]$Role = '',
    [string]$ClassId = ''
  )
  if (-not $Token) { throw 'Bearer token is missing' }
  $responseFile = Join-Path ([System.IO.Path]::GetTempPath()) ("campusclaw-response-" + [guid]::NewGuid() + '.json')
  $arguments = @('-sS', '-o', $responseFile, '-w', '%{http_code}', '-H', "Authorization: Bearer $Token", '-F', "file=@$File")
  if ($Title) { $arguments += @('-F', "title=$Title") }
  if ($Role) { $arguments += @('-F', "role=$Role") }
  if ($ClassId) { $arguments += @('-F', "class_id=$ClassId") }
  try {
    $status = & curl.exe @arguments "$BaseUrl/api/materials"
    if ($LASTEXITCODE -ne 0) { throw "curl upload failed with exit $LASTEXITCODE" }
    $body = Get-Content -LiteralPath $responseFile -Raw
    return [pscustomobject]@{ StatusCode = [int]$status; Body = $body }
  } finally {
    Remove-Item -LiteralPath $responseFile -Force -ErrorAction SilentlyContinue
  }
}

Assert-Status { Invoke-WebRequest -Uri "$BaseUrl/health" } 200 'health is public'
Assert-Status { Invoke-WebRequest -Uri "$BaseUrl/api/materials" } 401 'missing Authorization is rejected'
Assert-Status { Invoke-WebRequest -Uri "$BaseUrl/api/materials" -Headers @{ Authorization = 'Basic abc' } } 401 'non-Bearer Authorization is rejected'
Assert-Status { Invoke-WebRequest -Uri "$BaseUrl/api/materials" -Headers @{ Authorization = 'Bearer ' } } 401 'empty Bearer token is rejected'
Assert-Status { Invoke-WebRequest -Uri "$BaseUrl/api/materials" -Headers @{ Authorization = 'Bearer invalid-random-token' } } 401 'random Bearer token is rejected'
Assert-Status {
  $body = @{ username = 'teacher_a'; password = 'wrong-password' } | ConvertTo-Json
  Invoke-WebRequest -Uri "$BaseUrl/api/login" -Method Post -ContentType 'application/json' -Body $body
} 401 'wrong password'

$teacherLogin = Login 'teacher_a' 'TeacherA123!'
$studentLogin = Login 'student_a1' 'StudentA123!'
if (-not $teacherLogin.token -or $teacherLogin.user.username -ne 'teacher_a') { throw 'Teacher login response lacks token or nested user' }
if (-not $studentLogin.token -or $studentLogin.user.username -ne 'student_a1') { throw 'Student login response lacks token or nested user' }
$teacherToken = [string]$teacherLogin.token
$studentToken = [string]$studentLogin.token
Write-Output 'PASS login returns raw token and nested user'
Assert-Status { Invoke-WebRequest -Uri "$BaseUrl/api/materials" -Headers @{ Cookie = "campusclaw_session=$studentToken" } } 401 'legacy Cookie alone is rejected'

$me = Invoke-RestMethod -Uri "$BaseUrl/api/me" -Headers (Bearer-Headers $teacherToken)
if ($me.username -ne 'teacher_a' -or $me.role -ne 'teacher' -or [int]$me.class_id -ne 1) { throw 'Bearer /api/me returned wrong identity' }
Write-Output 'PASS Bearer token restores current user'

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("campusclaw-verify-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tempDir | Out-Null
try {
  $valid = Join-Path $tempDir 'valid.md'
  $exe = Join-Path $tempDir 'invalid.exe'
  $empty = Join-Path $tempDir 'empty.txt'
  $invalidUtf8 = Join-Path $tempDir 'invalid-utf8.txt'
  $large = Join-Path $tempDir 'large.txt'
  Set-Content -LiteralPath $valid -Value '# Bearer verification' -Encoding UTF8
  Set-Content -LiteralPath $exe -Value 'not supported' -Encoding UTF8
  New-Item -ItemType File -Path $empty | Out-Null
  [System.IO.File]::WriteAllBytes($invalidUtf8, [byte[]](0xff, 0xfe))
  [System.IO.File]::WriteAllBytes($large, (New-Object byte[] 1100000))

  $before = @((Db-Count 'materials'), (Db-Count 'knowledge_entries'), (Upload-Count))
  Assert-Status { Invoke-UploadRaw $studentToken $valid 'forged' 'teacher' } 403 'student forged-role upload'
  $after = @((Db-Count 'materials'), (Db-Count 'knowledge_entries'), (Upload-Count))
  if (($before -join ',') -ne ($after -join ',')) { throw "Student upload left residue: before=$before after=$after" }
  Write-Output 'PASS student rejection has no residue'

  $aList = Invoke-RestMethod -Uri "$BaseUrl/api/materials?class_id=2" -Headers (Bearer-Headers $studentToken)
  if ($aList.materials.id -contains 2) { throw 'Class query leaked B material' }
  Write-Output 'PASS client class_id is ignored'
  Assert-Status { Invoke-WebRequest -Uri "$BaseUrl/api/materials/2" -Headers (Bearer-Headers $studentToken) } 404 'cross-class direct ID'
  $beforeInvalid = @((Db-Count 'materials'), (Db-Count 'knowledge_entries'), (Upload-Count))
  Assert-Status { Invoke-UploadRaw $teacherToken $exe } 400 'exe upload'
  Assert-Status { Invoke-UploadRaw $teacherToken $empty } 400 'empty upload'
  Assert-Status { Invoke-UploadRaw $teacherToken $invalidUtf8 } 400 'invalid UTF-8 upload'
  Assert-Status { Invoke-UploadRaw $teacherToken $large } 413 'oversized upload'
  $afterInvalid = @((Db-Count 'materials'), (Db-Count 'knowledge_entries'), (Upload-Count))
  if (($beforeInvalid -join ',') -ne ($afterInvalid -join ',')) { throw "Invalid uploads left residue: before=$beforeInvalid after=$afterInvalid" }
  Write-Output 'PASS invalid uploads have no residue'

  $beforeSuccess = @((Db-Count 'materials'), (Db-Count 'knowledge_entries'), (Upload-Count))
  $createdResponse = Invoke-UploadRaw $teacherToken $valid 'Class A Bearer verification material' '' '2'
  if ($createdResponse.StatusCode -ne 201) { throw "Teacher upload failed: HTTP $($createdResponse.StatusCode) $($createdResponse.Body)" }
  $created = $createdResponse.Body | ConvertFrom-Json
  if (-not $created.id) { throw 'Teacher upload returned no id' }
  $classQuery = "SELECT class_id FROM materials WHERE id=$($created.id)"
  $storedClass = docker compose exec -T -e "MYSQL_PWD=$dbPassword" db mysql -N "-u$dbUser" $dbName -e $classQuery | Select-Object -Last 1
  if ([int]$storedClass -ne 1) { throw "Forged class_id was trusted: $storedClass" }
  $afterSuccess = @((Db-Count 'materials'), (Db-Count 'knowledge_entries'), (Upload-Count))
  for ($index = 0; $index -lt 3; $index++) {
    if ($afterSuccess[$index] -ne ($beforeSuccess[$index] + 1)) { throw "Teacher upload count mismatch: before=$beforeSuccess after=$afterSuccess" }
  }
  Write-Output 'PASS teacher upload uses session class and creates knowledge entry'
  Assert-Status { Invoke-WebRequest -Uri "$BaseUrl/api/materials/$($created.id)/file" -Headers (Bearer-Headers $teacherToken) } 200 'Bearer file download'

  Assert-Status { Invoke-WebRequest -Uri "$BaseUrl/api/logout" -Method Post -Headers (Bearer-Headers $teacherToken) } 204 'Bearer logout'
  Assert-Status { Invoke-WebRequest -Uri "$BaseUrl/api/me" -Headers (Bearer-Headers $teacherToken) } 401 'logged-out token is revoked'
} finally {
  Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}
