# Builds the dinah binaries and installs them into your bin directory, then
# packages the VS Code extension and installs that too.
# Wired to Ctrl-Shift-B in this workspace; run directly for the same result.
#
#   -SkipPull        Build whatever is currently checked out instead of updating to origin/main.
#   -SkipExtension   Install the binaries alone and leave the VS Code extension as it is.
#   -Repo            Override the repository location (defaults to this script's own repository).
#   -BinDir          Override the install directory (defaults to bin under your home directory).
#
# The extension is packaged and installed on every run, and -SkipExtension is
# how you decline it. Keeping the two in step is the whole reason for doing the
# extension here rather than by hand: the universal archive carries no binary,
# so an editor left on an older build talks to whatever dinah it finds and
# reports things that are not true. An opt-in switch would be one more thing to
# remember, and forgetting is what let the two drift apart in the first place.
# Packaging is not free, because it runs a type check, a lint and a compile
# before it writes the archive, so reach for -SkipExtension when you are
# iterating on Go and the extension has not moved.

param(
    [switch]$SkipPull,
    [switch]$SkipExtension,
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

    # The editor's own command is the other install target, so it is checked
    # here for the same reason the locks are: a run that cannot finish should
    # say so before the build rather than after it. A missing command is not
    # fatal, though. The binaries are the main event and they install without
    # any help from VS Code, so this records the finding and the extension step
    # at the bottom acts on it.
    $codeCmd = $null
    if (-not $SkipExtension) {
        $codeCmd = (Get-Command code -ErrorAction SilentlyContinue).Source
        if (-not $codeCmd) {
            Write-Host "The 'code' command is not on this PATH, so the VS Code extension cannot be installed." -ForegroundColor Yellow
            Write-Host "VS Code adds it from the command palette, under 'Shell Command: Install code command in PATH'." -ForegroundColor Yellow
            Write-Host "The binaries still build and install below, and the extension is left exactly as it is." -ForegroundColor Yellow
        }
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
            # Stay on the branch rather than detaching at the same commit. A
            # detached head leaves local main behind forever, and the release
            # workflow tags every commit on the trunk, so git names the position
            # by that tag and the checkout looks like it has wandered onto a
            # release. Fast-forward only, so a diverged main is reported instead
            # of merged.
            git checkout main -q
            if ($LASTEXITCODE -ne 0) { throw "git checkout main failed" }
            git merge --ff-only origin/main -q
            if ($LASTEXITCODE -ne 0) {
                throw "main has diverged from origin/main; reconcile it by hand, or pass -SkipPull to build what is checked out"
            }
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

    if ($SkipExtension) {
        Write-Host "Skipped the VS Code extension, so the editor keeps whatever build it already had." -ForegroundColor DarkGray
    } elseif (-not $codeCmd) {
        Write-Host "Left the VS Code extension alone, because the 'code' command was missing as reported above." -ForegroundColor Yellow
    } else {
        $extension = Join-Path $Repo "editors/vscode"

        # Packaging runs the extension's own toolchain out of node_modules, and
        # a fresh clone or a new worktree has none. Installing them here keeps
        # the first run in a checkout working, instead of failing inside a
        # missing tsc with nothing useful to say.
        if (-not (Test-Path (Join-Path $extension "node_modules"))) {
            Write-Host "Installing the extension's dependencies, which this checkout does not have yet" -ForegroundColor Cyan
            npm --prefix $extension ci
            if ($LASTEXITCODE -ne 0) { throw "installing the extension's dependencies failed" }
        }

        Write-Host "Packaging the VS Code extension" -ForegroundColor Cyan
        # A type check, a lint and a compile run ahead of the archive, so an
        # extension that does not build stops here rather than being installed.
        npm --prefix $extension run package
        if ($LASTEXITCODE -ne 0) {
            throw "packaging the VS Code extension failed; fix what npm reported above, or pass -SkipExtension to install the binaries alone"
        }

        $vsix = Join-Path $extension "vsix/dinah-universal.vsix"
        if (-not (Test-Path $vsix)) { throw "packaging reported success, but $vsix was not written" }

        # --force answers the prompts VS Code would otherwise ask, which is what
        # this needs: the extension's version number does not change between
        # local builds, so every install after the first one replaces a copy
        # that is already there at the same version.
        & $codeCmd --install-extension $vsix --force
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Installing the extension failed, and only the editor is affected. The binaries above are" -ForegroundColor Yellow
            Write-Host "installed and current, so nothing needs rebuilding." -ForegroundColor Yellow
            Write-Host "Close every VS Code window and run this again. An extension directory a running editor is" -ForegroundColor Yellow
            Write-Host "still holding has made an install crawl for minutes and then fail." -ForegroundColor Yellow
            Write-Host "If it fails a second time, remove the extension from the Extensions view and install" -ForegroundColor Yellow
            Write-Host "$vsix by hand from there." -ForegroundColor Yellow
            throw "installing the VS Code extension failed"
        }

        Write-Host "Installed the Dinah extension from $vsix" -ForegroundColor Green
        Write-Host "Reload your VS Code windows to pick it up." -ForegroundColor Green
    }
} finally {
    Pop-Location
}
