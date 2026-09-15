$Root = $PSScriptRoot
$pidFile = Join-Path $Root 'server.pid'

if (-not (Test-Path $pidFile)) {
    Write-Host "Entity Matcher does not appear to be running (no server.pid found)." -ForegroundColor Yellow
    exit 0
}

$pidContent = Get-Content $pidFile -Raw
$pidContent = $pidContent.Trim()
$serverPid = 0
if (-not ([int]::TryParse($pidContent, [ref]$serverPid)) -or $serverPid -le 0) {
    Write-Host "Entity Matcher does not appear to be running (invalid PID in server.pid)." -ForegroundColor Yellow
    Remove-Item $pidFile -Force -ErrorAction SilentlyContinue
    exit 0
}

if (-not (Get-Process -Id $serverPid -ErrorAction SilentlyContinue)) {
    Write-Host "Entity Matcher is not running (stale PID $serverPid)." -ForegroundColor Yellow
    Remove-Item $pidFile -Force -ErrorAction SilentlyContinue
    exit 0
}

# Windows has no SIGTERM equivalent, so calling this forcibly terminates the process
# and the server's graceful shutdown path does not get a chance to run.
Stop-Process -Id $serverPid -Force -ErrorAction SilentlyContinue

Write-Host "Entity Matcher stopped (PID $serverPid)." -ForegroundColor Green

# server.exe spawns no child processes, so unlike a Node/npm-based launcher there is
# no need to walk the process tree here -- stopping the single recorded PID is sufficient.
Remove-Item $pidFile -Force -ErrorAction SilentlyContinue

