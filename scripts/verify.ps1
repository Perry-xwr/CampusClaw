param(
  [string]$BaseUrl = 'http://localhost:8080'
)

$ErrorActionPreference = 'Stop'
$PSDefaultParameterValues['Invoke-WebRequest:UseBasicParsing'] = $true
$teacherSession = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$studentSession = New-Object Microsoft.PowerShell.Commands.WebRequestSession
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

function Login {
  param($Session, [string]$Username, [string]$Password)
  $body = @{ username = $Username; password = $Password } | ConvertTo-Json
  Invoke-WebRequest -Uri "$BaseUrl/api/login" -Method Post -ContentType 'application/json' -Body $body -WebSession $Session
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
    $Session,
    [string]$File,
    [string]$Title = '',
    [string]$Role = '',
    [string]$ClassId = ''
  )
  $cookie = $Session.Cookies.GetCookies([uri]$BaseUrl)['campusclaw_session'].Value
  if (-not $cookie) { throw 'Session cookie is missing' }
  $responseFile = Join-Path ([System.IO.Path]::GetTempPath()) ("campusclaw-response-" + [guid]::NewGuid() + '.json')
  $arguments = @('-sS', '-o', $responseFile, '-w', '%{http_code}', '-H', "Cookie: campusclaw_session=$cookie", '-F', "file=@$File")
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
Assert-Status { Invoke-WebRequest -Uri "$BaseUrl/api/materials" } 401 'unauthenticated API'
Assert-Status { Login $teacherSession 'teacher_a' 'wrong-password' } 401 'wrong password'
Login $teacherSession 'teacher_a' 'TeacherA123!' | Out-Null
Login $studentSession 'student_a1' 'StudentA123!' | Out-Null

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("campusclaw-verify-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tempDir | Out-Null
try {
  $valid = Join-Path $tempDir 'valid.md'
  $exe = Join-Path $tempDir 'invalid.exe'
  $empty = Join-Path $tempDir 'empty.txt'
  $invalidUtf8 = Join-Path $tempDir 'invalid-utf8.txt'
  $large = Join-Path $tempDir 'large.txt'
  Set-Content -LiteralPath $valid -Value '# Iteration 1 verification' -Encoding UTF8
  Set-Content -LiteralPath $exe -Value 'not supported' -Encoding UTF8
  New-Item -ItemType File -Path $empty | Out-Null
  [System.IO.File]::WriteAllBytes($invalidUtf8, [byte[]](0xff, 0xfe))
  [System.IO.File]::WriteAllBytes($large, (New-Object byte[] 1100000))

  $before = @((Db-Count 'materials'), (Db-Count 'knowledge_entries'), (Upload-Count))
  Assert-Status { Invoke-UploadRaw $studentSession $valid 'forged' 'teacher' } 403 'student forged-role upload'
  $after = @((Db-Count 'materials'), (Db-Count 'knowledge_entries'), (Upload-Count))
  if (($before -join ',') -ne ($after -join ',')) { throw "Student upload left residue: before=$before after=$after" }
  Write-Output 'PASS student rejection has no residue'

  $aList = Invoke-RestMethod -Uri "$BaseUrl/api/materials?class_id=2" -WebSession $studentSession
  if ($aList.materials.id -contains 2) { throw 'Class query leaked B material' }
  Write-Output 'PASS client class_id is ignored'
  Assert-Status { Invoke-WebRequest -Uri "$BaseUrl/api/materials/2" -WebSession $studentSession } 404 'cross-class direct ID'
  $beforeInvalid = @((Db-Count 'materials'), (Db-Count 'knowledge_entries'), (Upload-Count))
  Assert-Status { Invoke-UploadRaw $teacherSession $exe } 400 'exe upload'
  Assert-Status { Invoke-UploadRaw $teacherSession $empty } 400 'empty upload'
  Assert-Status { Invoke-UploadRaw $teacherSession $invalidUtf8 } 400 'invalid UTF-8 upload'
  Assert-Status { Invoke-UploadRaw $teacherSession $large } 413 'oversized upload'
  $afterInvalid = @((Db-Count 'materials'), (Db-Count 'knowledge_entries'), (Upload-Count))
  if (($beforeInvalid -join ',') -ne ($afterInvalid -join ',')) { throw "Invalid uploads left residue: before=$beforeInvalid after=$afterInvalid" }
  Write-Output 'PASS invalid uploads have no residue'

  $beforeSuccess = @((Db-Count 'materials'), (Db-Count 'knowledge_entries'), (Upload-Count))
  $createdResponse = Invoke-UploadRaw $teacherSession $valid 'Class A verification material' '' '2'
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
} finally {
  Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}
