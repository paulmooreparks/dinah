# Install Dinah on Windows, into %LOCALAPPDATA%\dinah\bin, with no
# administrator privilege.
#
# Set DINAH_CHANNEL to choose a channel. It defaults to dev.
#
#   irm https://raw.githubusercontent.com/paulmooreparks/dinah/main/scripts/install.ps1 | iex
#
# The download is staged inside the install directory rather than in the
# system temporary folder, so the final move stays on one volume. %TEMP% is
# not guaranteed to sit on the same volume as %LOCALAPPDATA%, and a move
# across a volume boundary is a copy followed by a delete, which can leave a
# truncated binary at the install path if it is interrupted.

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

function Write-Failure($message) {
    [Console]::Error.WriteLine($message)
}

$channel = if ($env:DINAH_CHANNEL) { $env:DINAH_CHANNEL } else { 'dev' }
$installDir = Join-Path $env:LOCALAPPDATA 'dinah\bin'
$manifestUrl = "https://github.com/paulmooreparks/dinah/releases/download/channels/$channel.json"

# Step 1: work out which binary this machine needs.
$goarch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { '' }
}
if (-not $goarch) {
    Write-Failure "no Dinah build is published for Windows on $($env:PROCESSOR_ARCHITECTURE); build one from source with: go build -o dinah ./cmd/dinah"
    exit 1
}
$binary = "dinah-windows-$goarch.exe"

# Step 2: confirm the install directory can be written before anything is
# fetched, so a permission problem is never mistaken for a failed download.
try {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    $marker = Join-Path $installDir '.write-test'
    New-Item -ItemType File -Path $marker -Force | Out-Null
    Remove-Item -Path $marker -Force
}
catch {
    Write-Failure "cannot write to $installDir; check that you have permission to write there"
    exit 1
}

# Step 3: read the channel manifest.
try {
    $response = Invoke-WebRequest -Uri $manifestUrl -UseBasicParsing
    $manifest = $response.Content | ConvertFrom-Json
    $entry = $manifest.binaries.PSObject.Properties[$binary].Value
    $wantSha = $entry.sha256
    $downloadBase = $manifest.downloadBase
}
catch {
    Write-Failure 'could not fetch the release manifest from GitHub; check your network connection and try again'
    exit 1
}
if (-not $wantSha -or -not $downloadBase) {
    Write-Failure 'could not fetch the release manifest from GitHub; check your network connection and try again'
    exit 1
}

# Step 4: stage the download inside the install directory itself. A GUID
# suffix cannot collide with anything already in that directory, including a
# dinah.exe from an earlier run.
$tmpfile = Join-Path $installDir ('dinah.tmp.' + [System.Guid]::NewGuid().ToString('N'))
try {
    # Step 5: download.
    try {
        Invoke-WebRequest -Uri ($downloadBase + $binary) -OutFile $tmpfile -UseBasicParsing
    }
    catch {
        Write-Failure "download of $binary did not complete (network error); nothing was installed, and it is safe to run this script again"
        exit 1
    }

    # Step 6: verify the bytes. Reaching here means the transfer finished, so
    # a mismatch is corruption or a manifest that no longer describes what is
    # being served, which is a different failure from the one above.
    $gotSha = (Get-FileHash -Path $tmpfile -Algorithm SHA256).Hash
    if ($gotSha -ne $wantSha) {
        Write-Failure "downloaded file's checksum does not match the manifest for $binary; the download will not be installed"
        exit 1
    }

    # Step 7: the one step that changes what sits at the install path, and it
    # runs only on a complete, verified download.
    Move-Item -Path $tmpfile -Destination (Join-Path $installDir 'dinah.exe') -Force
    Write-Host "Installed $binary as $(Join-Path $installDir 'dinah.exe')"
}
finally {
    if (Test-Path -Path $tmpfile) {
        Remove-Item -Path $tmpfile -Force -ErrorAction SilentlyContinue
    }
}

# Step 8: put the install directory on the user's PATH, which needs no
# administrator privilege. A shell started after this can run dinah by name.
$userPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
$alreadyThere = $userPath -and (($userPath -split ';') -contains $installDir)
if (-not $alreadyThere) {
    $newPath = if ([string]::IsNullOrEmpty($userPath)) { $installDir } else { "$userPath;$installDir" }
    [Environment]::SetEnvironmentVariable('PATH', $newPath, 'User')
    Write-Host "Added $installDir to your PATH. Open a new shell to pick it up."
}
