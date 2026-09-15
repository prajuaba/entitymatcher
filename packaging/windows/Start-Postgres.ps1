param([int]$Port = 5432)
$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot

$PgRoot = Join-Path $Root 'pgsql'
$PgBin = Join-Path $PgRoot 'bin'
$DataDir = Join-Path $Root 'pgsql-data'
$PgLog = Join-Path $Root 'postgres.log'
$PwFile = Join-Path $Root 'pg-password.txt'

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
if ($principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "Error: PostgreSQL refuses to run under an Administrator account on Windows." -ForegroundColor Red
    Write-Host "Close this window and run Start-Postgres.ps1 from a normal (non-elevated) PowerShell window instead." -ForegroundColor Red
    exit 1
}

if (-not (Test-Path (Join-Path $PgBin 'postgres.exe'))) {
    Write-Host "Error: PostgreSQL executable not found. The package appears to be missing the bundled database." -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $DataDir)) {
    $password = [System.Guid]::NewGuid().ToString("N") + [System.Guid]::NewGuid().ToString("N")
    $password | Out-File -FilePath $PwFile -Encoding ascii -NoNewline

    & (Join-Path $PgBin 'initdb.exe') -D $DataDir -U postgres --encoding=UTF8 --locale=C --auth-host=scram-sha-256 --auth-local=trust --pwfile=$PwFile
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error: initdb failed to initialize PostgreSQL data directory." -ForegroundColor Red
        exit 1
    }

    Write-Host "PostgreSQL superuser password generated and stored in: $PwFile" -ForegroundColor Yellow
    Write-Host "This is a per-machine password, regenerated each time pgsql-data is recreated." -ForegroundColor Yellow
} else {
    if (-not (Test-Path $PwFile)) {
        Write-Host "Error: PostgreSQL data directory exists but password file is missing. Please delete pgsql-data to re-initialize from scratch (this will lose all data)." -ForegroundColor Red
        exit 1
    }
    $password = (Get-Content $PwFile -Raw).Trim()
}

if ((Test-Path (Join-Path $DataDir 'postmaster.pid'))) {
    $statusOutput = & (Join-Path $PgBin 'pg_ctl.exe') -D $DataDir status
    $statusExitCode = $LASTEXITCODE
    if ($statusExitCode -eq 3) {
        Remove-Item (Join-Path $DataDir 'postmaster.pid') -Force -ErrorAction SilentlyContinue
        Write-Host "Warning: Removed stale lock file from unclean shutdown." -ForegroundColor Yellow
    }
}

$statusOutput = & (Join-Path $PgBin 'pg_ctl.exe') -D $DataDir status
if ($LASTEXITCODE -eq 0) {
    # pg_ctl status only answers "is a server running for this data directory",
    # not "is it on the port I asked for", which is why the extra check below is needed.
    $runningPort = $null
    if (Test-Path (Join-Path $DataDir 'postmaster.pid')) {
        $pidLines = @(Get-Content (Join-Path $DataDir 'postmaster.pid'))
        if ($pidLines.Count -ge 4) {
            $parsedPort = 0
            if ([int]::TryParse($pidLines[3].Trim(), [ref]$parsedPort)) {
                $runningPort = $parsedPort
            }
        }
    }
    
    if ($runningPort -ne $null) {
        if ($runningPort -ne $Port) {
            Write-Host "Error: PostgreSQL is already running on port $runningPort, not the requested port $Port." -ForegroundColor Red
            Write-Host "Either re-run this script with '-PgPort $runningPort' to use the already-running instance," -ForegroundColor Red
            Write-Host "or run '.\Stop-Postgres.ps1' first and then start again on the port you want." -ForegroundColor Red
            exit 1
        }
        Write-Host "PostgreSQL is already running." -ForegroundColor Green
    } else {
        # Could not determine running port, so probe the requested port directly.
        & (Join-Path $PgBin 'pg_isready.exe') -h 127.0.0.1 -p $Port
        $readyExitCode = $LASTEXITCODE
        if ($readyExitCode -ne 0) {
            Write-Host "Error: A PostgreSQL server is running for this data directory but is not reachable on port $Port." -ForegroundColor Red
            Write-Host "Please run '.\Stop-Postgres.ps1' and then retry." -ForegroundColor Red
            exit 1
        }
        Write-Host "PostgreSQL is already running." -ForegroundColor Green
    }
} else {
    # -h 127.0.0.1 binds loopback only so the bundled database is never exposed to the network
    # -w makes pg_ctl wait for the server to report ready before returning
    & (Join-Path $PgBin 'pg_ctl.exe') -D $DataDir -l $PgLog -o "-p $Port -h 127.0.0.1" -w start
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error: Failed to start PostgreSQL server." -ForegroundColor Red
        if (Test-Path $PgLog) {
            Get-Content $PgLog -Tail 20 | ForEach-Object { Write-Host $_ }
        }
        exit 1
    }
}

$env:PGPASSWORD = $password
$exists = & (Join-Path $PgBin 'psql.exe') -h 127.0.0.1 -p $Port -U postgres -tAc "SELECT 1 FROM pg_database WHERE datname='entity_matcher'"
$exists = ($exists -join "").Trim()
if ($exists -ne "1") {
    & (Join-Path $PgBin 'createdb.exe') -h 127.0.0.1 -p $Port -U postgres entity_matcher
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error: Failed to create database entity_matcher." -ForegroundColor Red
        Remove-Item Env:\PGPASSWORD -ErrorAction SilentlyContinue
        exit 1
    }
}
Remove-Item Env:\PGPASSWORD -ErrorAction SilentlyContinue

$encodedPassword = [uri]::EscapeDataString($password)
$dsn = "postgres://postgres:$encodedPassword@127.0.0.1:$Port/entity_matcher?sslmode=disable"
Write-Output $dsn

