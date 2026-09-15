param([string]$Mode = "all")

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
        # Ignore errors from docker compose version
    }
    return $null
}

function Get-ProcessDescendants {
    param([int]$ParentId, [array]$AllProcesses)

    # This function walks the process tree using CIM to find descendants.
    # The PID-alone approach fails because go run . spawns the compiled binary as a child process
    # and npm run dev spawns node as a child process, so killing only the recorded PID leaves
    # the real server (and the port it holds, 8085 or 3000) running — you must walk the CIM process tree.

    $descendants = @()
    foreach ($process in $AllProcesses) {
        if ($process.ParentProcessId -eq $ParentId) {
            $childDescendants = Get-ProcessDescendants -ParentId $process.ProcessId -AllProcesses $AllProcesses
            $descendants += $childDescendants
            $descendants += $process.ProcessId
        }
    }
    return $descendants
}

function Stop-LocalProcess {
    param([string]$Name)

    $pidFile = Join-Path $PidDir "$Name.pid"
    if (Test-Path $pidFile) {
        $localPidRaw = (Get-Content $pidFile -Raw)
        if (-not $localPidRaw) { $localPidRaw = "" }
        $localPidRaw = $localPidRaw.Trim()
        $localPid = 0
        $localPidValid = [int]::TryParse(($localPidRaw -replace '\s',''), [ref]$localPid) -and $localPid -gt 0
        if ($localPidValid -and (Get-Process -Id $localPid -ErrorAction SilentlyContinue)) {
            Write-Host "==> Stopping $Name (PID: $localPid)..." -ForegroundColor Blue
            $processes = Get-CimInstance Win32_Process
            $descendants = Get-ProcessDescendants -ParentId $localPid -AllProcesses $processes
            # Stop-Process -Force is an immediate/hard kill with no Windows equivalent of SIGTERM
            # so the Go backend's graceful-shutdown handler in backend/main.go does not get a chance to run on Windows.
            foreach ($desc in $descendants) {
                Stop-Process -Id $desc -Force -ErrorAction SilentlyContinue
            }
            Stop-Process -Id $localPid -Force -ErrorAction SilentlyContinue
            Start-Sleep -Milliseconds 500
            Write-Host "✓ $Name stopped." -ForegroundColor Green
        } else {
            Write-Host "$Name (PID: $localPidRaw) is not running." -ForegroundColor Yellow
        }
        Remove-Item $pidFile -Force -ErrorAction SilentlyContinue
    }
}

function Stop-DockerMode {
    $composeCmd = Get-DockerCompose
    if ($composeCmd) {
        Write-Host "==> Stopping Docker Compose services..." -ForegroundColor Blue
        $parts = $composeCmd -split '\s+'
        $exe = $parts[0]
        $composeArgs = @($parts | Select-Object -Skip 1) + @("down")
        Push-Location $ScriptDir
        & $exe $composeArgs
        Pop-Location
        Write-Host "✓ Docker services stopped." -ForegroundColor Green
    } else {
        Write-Host "Docker compose command not found, skipping Docker shutdown." -ForegroundColor Yellow
    }
}

function Stop-LocalMode {
    Write-Host "==> Stopping local processes..." -ForegroundColor Blue
    Stop-LocalProcess -Name "backend"
    Stop-LocalProcess -Name "frontend"
    Write-Host "✓ Local processes stopped." -ForegroundColor Green
}

switch ($Mode.ToLower()) {
    "docker" {
        Stop-DockerMode
    }
    "local" {
        Stop-LocalMode
    }
    "all" {
        Stop-DockerMode
        Stop-LocalMode
    }
    { $_ -in @("help", "-h", "--help") } {
        Write-Host "Usage: stop.ps1 [docker|local|all]"
        Write-Host "  all    : Stop both Docker services and local processes (default)"
        Write-Host "  docker : Stop Docker Compose services"
        Write-Host "  local  : Stop local background processes"
        exit 0
    }
    default {
        Write-Host "Unknown mode: $Mode" -ForegroundColor Red
        Write-Host "Usage: stop.ps1 [docker|local|all]"
        Write-Host "  all    : Stop both Docker services and local processes (default)"
        Write-Host "  docker : Stop Docker Compose services"
        Write-Host "  local  : Stop local background processes"
        exit 1
    }
}

