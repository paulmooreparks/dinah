# Builds the dinah binaries and installs them into your bin directory.
# Wired to Ctrl-Shift-B in this workspace; run directly for the same result.
#
#   -SkipPull   Build whatever is currently checked out instead of updating to origin/main.
#   -Repo       Override the repository location (defaults to this script's own repository).
#   -BinDir     Override the install directory (defaults to bin under your home directory).

param(
    [switch]$SkipPull,
    [string]$Repo = (Split-Path -Parent $PSScriptRoot),
    [string]$BinDir = (Join-Path $HOME "bin")
)

$ErrorActionPreference = "Stop"

# Windows refuses to overwrite an executable that is currently running, and the
# Go linker surfaces that refusal as an opaque failure to copy a temporary file.
# The common workaround is to rename the running binary aside, which every
# updater does and which Microsoft documents nowhere, so this script detects the
# condition and names who caused it instead of relying on that behaviour.
function Test-FileLocked {
    param([Parameter(Mandatory)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) { return $false }
    try {
        $stream = [System.IO.File]::Open($Path, 'Open', 'Write', 'None')
        $stream.Close()
        return $false
    } catch [System.IO.IOException] {
        return $true
    }
}

# Best effort, and it may come back empty. A lock usually means the binary is
# running, but a scanner or an indexer holds one too and owns no matching image.
function Get-FileHolders {
    param([Parameter(Mandatory)][string]$Path)

    $leaf = Split-Path -Leaf $Path
    $full = (Resolve-Path -LiteralPath $Path).ProviderPath
    Get-CimInstance Win32_Process -Filter "Name='$leaf'" -ErrorAction SilentlyContinue |
        Where-Object { $_.ExecutablePath -eq $full } |
        ForEach-Object {
            $parent = Get-CimInstance Win32_Process -Filter "ProcessId=$($_.ParentProcessId)" -ErrorAction SilentlyContinue
            $origin = if ($parent) { "$($parent.Name) $($parent.ProcessId)" } else { "parent $($_.ParentProcessId), already exited" }
            "  PID {0,-8} {1}  (started by {2})" -f $_.ProcessId, $_.CommandLine, $origin
        }
}

if (-not (Test-Path (Join-Path $Repo "go.mod"))) { throw "no Go module found at $Repo" }
if (-not (Test-Path $BinDir)) { New-Item -ItemType Directory -Force $BinDir | Out-Null }

Push-Location $Repo
try {
    # Check every target before doing anything else. Failing here rather than
    # halfway through the build leaves the checkout where it was, which matters
    # because the pull below moves it.
    $blocked = @()
    foreach ($dir in Get-ChildItem -Directory (Join-Path $Repo "cmd")) {
        $exe = Join-Path $BinDir ("{0}.exe" -f $dir.Name)
        if (Test-FileLocked $exe) { $blocked += $exe }
    }
    if ($blocked) {
        # The detail goes out line by line rather than into the exception,
        # because PowerShell flattens a multi-line throw into one long run.
        Write-Host "Windows locks an executable while it runs, so installing over one of these would" -ForegroundColor Yellow
        Write-Host "fail partway through the build with a copy error from the linker." -ForegroundColor Yellow
        foreach ($exe in $blocked) {
            $holders = @(Get-FileHolders $exe)
            if ($holders) {
                Write-Host "$exe is held by:" -ForegroundColor Yellow
                $holders | ForEach-Object { Write-Host $_ -ForegroundColor Yellow }
            } else {
                Write-Host "$exe is locked, and no running copy of it accounts for the lock." -ForegroundColor Yellow
            }
        }
        Write-Host "Close what is listed above and run this again, or pass -BinDir to install elsewhere." -ForegroundColor Yellow
        throw "install target is locked by a running process"
    }

    if (-not $SkipPull) {
        # Only tracked-file changes matter here. A checkout leaves untracked files
        # alone, and agent tooling leaves plenty of them lying around.
        $dirty = git status --porcelain --untracked-files=no
        if ($dirty) {
            Write-Warning "Working tree has uncommitted changes; building it as-is (pull skipped). Use -SkipPull to silence this."
        } else {
            git fetch origin
            if ($LASTEXITCODE -ne 0) { throw "git fetch failed" }
            # The checkout is kept detached at the trunk; fast-forward the detachment.
            git checkout --detach origin/main -q
            if ($LASTEXITCODE -ne 0) { throw "git checkout origin/main failed" }
        }
    }

    $sha = git rev-parse --short HEAD
    Write-Host "Building dinah at $sha" -ForegroundColor Cyan

    $built = @()
    foreach ($dir in Get-ChildItem -Directory (Join-Path $Repo "cmd")) {
        $exe = Join-Path $BinDir ("{0}.exe" -f $dir.Name)
        go build -o $exe ("./cmd/{0}" -f $dir.Name)
        if ($LASTEXITCODE -ne 0) { throw "go build failed for cmd/$($dir.Name)" }
        $built += $exe
    }

    foreach ($exe in $built) {
        Write-Host "Installed $exe" -ForegroundColor Green
    }
    & (Join-Path $BinDir "dinah.exe") version 2>$null
} finally {
    Pop-Location
}
