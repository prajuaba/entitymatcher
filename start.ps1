param([string]$Mode = "docker")

$ErrorActionPreference = "Stop"
$ScriptDir = $PSScriptRoot
$PidDir = Join-Path $ScriptDir ".run"

function Get-DockerCompose {
    if (Get-Command docker-compose -ErrorAction SilentlyContinue) {
        return "docker-compose"
    }
    try {
        docker compose version > $null 2>&1
        if ($LASTEXITCODE -eq 0) {
            return "docker compose"
        }
    } catch {
        # ignore
    }
    return $null
}

function Start-DockerMode {
    $DockerComposeCmd = Get-DockerCompose
    if (-not $DockerComposeCmd) {
        Write-Host "Error: neither 'docker compose' nor 'docker-compose' found." -ForegroundColor Red
        Write-Host "Please install Docker Desktop or run in local mode: .\start.ps1 local" -ForegroundColor Yellow
        exit 1
    }

    Write-Host "==> Starting Entity Matcher stack with Docker Compose..." -ForegroundColor Blue

    $originalLocation = Get-Location
    try {
        Set-Location $ScriptDir
        $parts = $DockerComposeCmd -split '\s+'
        $exe = $parts[0]
        $composeArgs = @($parts | Select-Object -Skip 1) + @("up", "-d", "--build")
        & $exe $composeArgs
    } finally {
        Set-Location $originalLocation
    }

    Write-Host "==> Waiting for services to become healthy..." -ForegroundColor Blue

    $maxAttempts = 30
    $attempt = 1
    $backendReady = $false

    while ($attempt -le $maxAttempts) {
        try {
            $response = Invoke-WebRequest -Uri "http://localhost:8085/api/health" -UseBasicParsing -TimeoutSec 2
            if ($response.StatusCode -eq 200) {
                $backendReady = $true
                break
            }
        } catch {
            # ignore exceptions (i.e., not ready yet)
        }
        Start-Sleep -Seconds 1
        $attempt++
    }

    Write-Host ""
    if ($backendReady) {
        Write-Host "✓ System started successfully!" -ForegroundColor Green
    } else {
        Write-Host "! System started (backend health check took longer than expected)." -ForegroundColor Yellow
    }

    Write-Host "--------------------------------------------------------"
    Write-Host " Frontend UI : http://localhost:3000" -ForegroundColor Green
    Write-Host " Backend API : http://localhost:8085 (Health: http://localhost:8085/api/health)" -ForegroundColor Green
    Write-Host " Postgres DB : localhost:5432" -ForegroundColor Green
    Write-Host "--------------------------------------------------------"
    Write-Host "To view logs: $DockerComposeCmd logs -f" -ForegroundColor Yellow
    Write-Host "To stop:      .\stop.ps1" -ForegroundColor Yellow
}

function Start-LocalMode {
    Write-Host "==> Starting Entity Matcher in Local Development Mode..." -ForegroundColor Blue

    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host "Error: 'go' is not installed or not in PATH." -ForegroundColor Red
        exit 1
    }

    if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
        Write-Host "Error: 'npm' is not installed or not in PATH." -ForegroundColor Red
        exit 1
    }

    New-Item -ItemType Directory -Force -Path $PidDir | Out-Null

    # Backend
    $backendAlreadyRunning = $false
    if (Test-Path (Join-Path $PidDir "backend.pid")) {
        $backendPidRaw = Get-Content (Join-Path $PidDir "backend.pid") -Raw
        $backendPid = 0
        $backendPidValid = [int]::TryParse(($backendPidRaw -replace '\s',''), [ref]$backendPid) -and $backendPid -gt 0
        if ($backendPidValid -and (Get-Process -Id $backendPid -ErrorAction SilentlyContinue)) {
            Write-Host "Backend is already running with PID $backendPid" -ForegroundColor Yellow
            $backendAlreadyRunning = $true
        }
    }

    if (-not $backendAlreadyRunning) {
        Write-Host "==> Starting Go backend on port 8085..." -ForegroundColor Blue
        $env:PORT = "8085"
        if (-not $env:JWT_SECRET) { $env:JWT_SECRET = "dev-secret-change-me-in-production" }
        # Note: backend serves the SPA from the relative path ../frontend/dist, so the working directory MUST be the backend subdirectory.
        $proc = Start-Process -FilePath "go" -ArgumentList "run","." -WorkingDirectory (Join-Path $ScriptDir "backend") -RedirectStandardOutput (Join-Path $PidDir "backend.log") -RedirectStandardError (Join-Path $PidDir "backend.err.log") -PassThru -NoNewWindow
        $proc.Id | Out-File -FilePath (Join-Path $PidDir "backend.pid") -Encoding ascii -NoNewline
        Write-Host "✓ Backend process started (PID: $($proc.Id))" -ForegroundColor Green
    }

    # Frontend
    Set-Location (Join-Path $ScriptDir "frontend")
    if (-not (Test-Path node_modules)) {
        Write-Host "==> Installing frontend dependencies..." -ForegroundColor Blue
        & npm.cmd install
    }
    Set-Location $ScriptDir

    $frontendAlreadyRunning = $false
    if (Test-Path (Join-Path $PidDir "frontend.pid")) {
        $frontendPidRaw = Get-Content (Join-Path $PidDir "frontend.pid") -Raw
        $frontendPid = 0
        $frontendPidValid = [int]::TryParse(($frontendPidRaw -replace '\s',''), [ref]$frontendPid) -and $frontendPid -gt 0
        if ($frontendPidValid -and (Get-Process -Id $frontendPid -ErrorAction SilentlyContinue)) {
            Write-Host "Frontend is already running with PID $frontendPid" -ForegroundColor Yellow
            $frontendAlreadyRunning = $true
        }
    }

    if (-not $frontendAlreadyRunning) {
        Write-Host "==> Starting Vite frontend on port 3000..." -ForegroundColor Blue
        $proc = Start-Process -FilePath "npm.cmd" -ArgumentList "run","dev" -WorkingDirectory (Join-Path $ScriptDir "frontend") -RedirectStandardOutput (Join-Path $PidDir "frontend.log") -RedirectStandardError (Join-Path $PidDir "frontend.err.log") -PassThru -NoNewWindow
        $proc.Id | Out-File -FilePath (Join-Path $PidDir "frontend.pid") -Encoding ascii -NoNewline
        Write-Host "✓ Frontend process started (PID: $($proc.Id))" -ForegroundColor Green
    }

    Write-Host "==> Waiting for services to be ready..." -ForegroundColor Blue
    Start-Sleep -Seconds 2

    Write-Host ""
    Write-Host "✓ Local environment running!" -ForegroundColor Green
    Write-Host "--------------------------------------------------------"
    Write-Host " Frontend UI : http://localhost:3000" -ForegroundColor Green
    Write-Host " Backend API : http://localhost:8085 (Health: http://localhost:8085/api/health)" -ForegroundColor Green
    Write-Host " Backend Log : $(Join-Path $PidDir 'backend.log')" -ForegroundColor Yellow
    Write-Host " Frontend Log: $(Join-Path $PidDir 'frontend.log')" -ForegroundColor Yellow
    Write-Host "--------------------------------------------------------"
    Write-Host "To stop: .\stop.ps1 local" -ForegroundColor Yellow
}

switch ($Mode.ToLower()) {
    "docker" { Start-DockerMode }
    "local"  { Start-LocalMode }
    { $_ -in @("help", "-h", "--help") } {
        Write-Host "Usage: start.ps1 [docker|local]"
        Write-Host "  docker : Start using Docker Compose (default)"
        Write-Host "  local  : Start using local Go and Node.js runtimes"
        exit 0
    }
    default {
        Write-Host "Unknown mode: $Mode" -ForegroundColor Red
        Write-Host "Usage: start.ps1 [docker|local]" -ForegroundColor Red
        exit 1
    }
}

