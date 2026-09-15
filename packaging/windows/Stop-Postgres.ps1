param()
$Root = $PSScriptRoot

$PgRoot = Join-Path $Root 'pgsql'
$PgBin = Join-Path $PgRoot 'bin'
$DataDir = Join-Path $Root 'pgsql-data'

if (-not (Test-Path $DataDir)) {
    Write-Host "No bundled database found (pgsql-data does not exist)." -ForegroundColor Yellow
    exit 0
}

# -m fast rolls back any open transactions and shuts down cleanly rather than waiting for clients to disconnect on their own
& (Join-Path $PgBin 'pg_ctl.exe') -D $DataDir -m fast -w stop

if ($LASTEXITCODE -ne 0) {
    Write-Host "PostgreSQL may not have been running (pg_ctl failed with exit code $LASTEXITCODE)." -ForegroundColor Yellow
    exit 0
}

Write-Host "Bundled PostgreSQL server stopped." -ForegroundColor Green
