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

if (-not (Test-Path (Join-Path $Repo "go.mod"))) { throw "no Go module found at $Repo" }
if (-not (Test-Path $BinDir)) { New-Item -ItemType Directory -Force $BinDir | Out-Null }

Push-Location $Repo
try {
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
