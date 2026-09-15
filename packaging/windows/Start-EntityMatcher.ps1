param([int]$Port = 8085, [string]$DatabaseUrl = "", [switch]$Foreground, [switch]$Postgres, [int]$PgPort = 5432)

$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot

if (-not (Test-Path (Join-Path $Root 'backend\server.exe'))) {
    Write-Host "Error: backend\server.exe not found." -ForegroundColor Red
    exit 1
}

if (-not (Test-Path (Join-Path $Root 'frontend\dist\index.html'))) {
    Write-Host "Error: frontend\dist\index.html not found." -ForegroundColor Red
    exit 1
}

$connectorDataExists = Test-Path (Join-Path $Root 'connector-data')
if (-not $connectorDataExists) {
    Write-Host "Warning: connector-data folder not found. Connector file_path endpoints will be disabled." -ForegroundColor Yellow
}

# Handle -Postgres parameter
if ($Postgres -and $DatabaseUrl) {
    Write-Host "Error: Both -Postgres and -DatabaseUrl were specified. Please specify only one." -ForegroundColor Red
    exit 1
}

if ($Postgres -and -not (Test-Path (Join-Path $Root 'pgsql\bin\postgres.exe'))) {
    Write-Host "Error: The package was built without the bundled PostgreSQL database." -ForegroundColor Red
    exit 1
}

if ($Postgres) {
    $dsn = & (Join-Path $Root 'Start-Postgres.ps1') -Port $PgPort
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($dsn)) {
        Write-Host "Error: bundled PostgreSQL failed to start." -ForegroundColor Red
        exit 1
    }
    $env:DATABASE_URL = $dsn
}

$env:PORT = $Port

if ([string]::IsNullOrWhiteSpace($env:JWT_SECRET)) {
    $guid1 = [System.Guid]::NewGuid().ToString("N")
    $guid2 = [System.Guid]::NewGuid().ToString("N")
    $env:JWT_SECRET = $guid1 + $guid2
    Write-Host "Warning: JWT_SECRET was not set. A new one has been generated for this session." -ForegroundColor Yellow
    Write-Host "  Tokens will not survive a restart. Set JWT_SECRET explicitly for production use." -ForegroundColor Yellow
}

if ($DatabaseUrl) {
    $env:DATABASE_URL = $DatabaseUrl
}

if ([string]::IsNullOrWhiteSpace($env:DATABASE_URL)) {
    Write-Host "Using in-memory store -- all data is lost when the server stops" -ForegroundColor Yellow
} else {
    Write-Host "Using PostgreSQL storage" -ForegroundColor Green
}

# CONNECTOR_FILE_ROOT is scoped to the bundled connector-data folder for safety.
# Leaving it unset disables the connector file_path endpoints entirely.
if ([string]::IsNullOrWhiteSpace($env:CONNECTOR_FILE_ROOT)) {
    if ($connectorDataExists) {
        $connectorPath = (Resolve-Path (Join-Path $Root 'connector-data')).Path
        $env:CONNECTOR_FILE_ROOT = $connectorPath
        Write-Host "Connector file root set to: $connectorPath" -ForegroundColor Yellow
    }
}

# Demo accounts (password: password123):
Write-Host "Demo accounts (password: password123):" -ForegroundColor Green
Write-Host "  admin" -ForegroundColor Green
Write-Host "  engineer_alex" -ForegroundColor Green
Write-Host "  reviewer_sarah" -ForegroundColor Green
Write-Host "  auditor_mike" -ForegroundColor Green
Write-Host ""
Write-Host "!  These accounts must be changed before any real use." -ForegroundColor Red

$LogDir = $Root
$PidFile = Join-Path $LogDir 'server.pid'
$LogFile = Join-Path $LogDir 'server.log'
$ErrLogFile = Join-Path $LogDir 'server.err.log'

if ($Foreground) {
    try {
        Push-Location (Join-Path $Root 'backend')
        & (Join-Path $Root 'backend\server.exe')
    } finally {
        Pop-Location
    }
} else {
    $proc = Start-Process -FilePath (Join-Path $Root 'backend\server.exe') `
        -WorkingDirectory (Join-Path $Root 'backend') `
        -RedirectStandardOutput $LogFile `
        -RedirectStandardError $ErrLogFile `
        -PassThru -NoNewWindow
    $proc.Id | Out-File -FilePath $PidFile -Encoding ascii -NoNewline

    Write-Host "==> Waiting for server to become healthy..." -ForegroundColor Blue

    $maxAttempts = 30
    $attempt = 1
    $serverReady = $false

    while ($attempt -le $maxAttempts) {
        if ($proc.HasExited) {
            break
        }
        try {
            $response = Invoke-WebRequest -Uri "http://localhost:$Port/api/health" -UseBasicParsing -TimeoutSec 2
            if ($response.StatusCode -eq 200) {
                $serverReady = $true
                break
            }
        } catch {
            # ignore exceptions (i.e., not ready yet)
        }
        Start-Sleep -Seconds 1
        $attempt++
    }

    if ($proc.HasExited) {
        Write-Host "Server failed to start (exited with code $($proc.ExitCode))." -ForegroundColor Red

        if (Test-Path $ErrLogFile) {
            $errLogContent = Get-Content $ErrLogFile -Tail 20
            if ($errLogContent) {
                $errLogContent | ForEach-Object { Write-Host $_ }
            } else {
                if (Test-Path $LogFile) {
                    Get-Content $LogFile -Tail 20 | ForEach-Object { Write-Host $_ }
                }
            }
        } else {
            if (Test-Path $LogFile) {
                Get-Content $LogFile -Tail 20 | ForEach-Object { Write-Host $_ }
            }
        }

        Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
        exit 1
    }

    Write-Host ""
    if ($serverReady) {
        Write-Host "✓ Server started successfully!" -ForegroundColor Green
    } else {
        Write-Host "! Server started (health check took longer than expected)." -ForegroundColor Yellow
    }

    Write-Host "--------------------------------------------------------"
    Write-Host " Server URL  : http://localhost:$Port" -ForegroundColor Green
    Write-Host " Server Log  : $LogFile" -ForegroundColor Yellow
    Write-Host " Error Log   : $ErrLogFile" -ForegroundColor Yellow
    if ([string]::IsNullOrWhiteSpace($env:DATABASE_URL)) {
        Write-Host " Storage     : in-memory store (data lost on restart)" -ForegroundColor Yellow
    } else {
        Write-Host " Storage     : PostgreSQL" -ForegroundColor Green
    }
    if ($Postgres) {
        Write-Host "             : bundled database running, stop with .\Stop-Postgres.ps1" -ForegroundColor Yellow
    }
    Write-Host " To stop     : .\Stop-EntityMatcher.ps1" -ForegroundColor Yellow
    Write-Host "--------------------------------------------------------"
}

