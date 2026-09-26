# Install Dinah on Windows, into %LOCALAPPDATA%\dinah\bin, with no
# administrator privilege. It installs two programs from one release:
# dinah.exe, and dinah-tui.exe, the terminal UI's program, which dinah tui
# starts.
#
# Set DINAH_CHANNEL to choose a channel. It defaults to dev. Set DINAH_NO_PATH
# to any value and this script leaves your PATH alone.
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
    Write-Failure "no Dinah build is published for Windows on $($env:PROCESSOR_ARCHITECTURE); build both programs from source with: go build -o dinah.exe ./cmd/dinah; go build -tags tui -o dinah-tui.exe ./cmd/dinah"
    exit 1
}
$binary = "dinah-windows-$goarch.exe"
$tuiBinary = "dinah-tui-windows-$goarch.exe"

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

# Step 3: read the channel manifest. ConvertFrom-Json reads the document as
# JSON rather than as lines of text, so however the manifest is laid out when
# it is published, this reads the same values out of it.
try {
    $response = Invoke-WebRequest -Uri $manifestUrl -UseBasicParsing
    $manifest = $response.Content | ConvertFrom-Json
}
catch {
    Write-Failure 'could not fetch the release manifest from GitHub; check your network connection and try again'
    exit 1
}
$downloadBase = $manifest.downloadBase
if (-not $downloadBase) {
    Write-Failure 'the release manifest from GitHub named no download location, so there is nothing to fetch; the release may still be publishing, so try again in a few minutes'
    exit 1
}
# Install-Program downloads the published file named $published, verifies
# it against the size and SHA-256 the manifest gives for it, and installs it
# in the install directory as $installed. $left is what a failure leaves in
# place, which each failure message ends with.
function Install-Program($published, $installed, $left) {
    $wantSha = $null
    $wantSize = $null
    if ($manifest.binaries) {
        $entry = $manifest.binaries.PSObject.Properties[$published]
        if ($entry) {
            $wantSha = $entry.Value.sha256
            $wantSize = $entry.Value.size
        }
    }
    if (-not $wantSha) {
        Write-Failure "the $channel channel publishes no $published, so there is no build for your machine to install; $left; build both programs from source with: go build -o dinah.exe ./cmd/dinah; go build -tags tui -o dinah-tui.exe ./cmd/dinah"
        exit 1
    }

    # Stage the download inside the install directory itself. A GUID suffix
    # cannot collide with anything already in that directory, including a
    # program from an earlier run.
    $tmpfile = Join-Path $installDir ('dinah.tmp.' + [System.Guid]::NewGuid().ToString('N'))
    try {
        # Download.
        try {
            Invoke-WebRequest -Uri ($downloadBase + $published) -OutFile $tmpfile -UseBasicParsing
        }
        catch {
            Write-Failure "download of $published did not complete (network error); $left, and it is safe to run this script again"
            exit 1
        }

        # Confirm the transfer actually finished before trusting it enough to
        # hash. When a proxy or a CDN edge cuts a download short and closes
        # the connection cleanly, Invoke-WebRequest reports no error and
        # -OutFile simply stops writing, leaving a short file that never
        # reaches the catch above. The manifest already carries each binary's
        # size, so a length mismatch here is reported as a short download,
        # distinct from both the network-error message above and the
        # checksum-mismatch message below; without this check a short
        # download reaches the hash compare and is misreported as corruption
        # instead.
        $gotSize = (Get-Item -Path $tmpfile).Length
        if ($wantSize -and $gotSize -ne $wantSize) {
            Write-Failure "download of $published is incomplete ($gotSize of $wantSize bytes); $left, and it is safe to run this script again"
            exit 1
        }

        # Verify the bytes. Reaching here means the transfer finished, so a
        # mismatch is corruption or a manifest that no longer describes what
        # is being served, which is a different failure from the one above.
        #
        # The digest is computed with the .NET SHA256 class directly, over a
        # FileStream, rather than with Get-FileHash. Get-FileHash is exported
        # by the Microsoft.PowerShell.Utility module (Microsoft Learn's own
        # cmdlet reference states this), not one of the cmdlets Windows
        # PowerShell's default session already carries, so Windows PowerShell
        # reaches it only by autoloading that module off PSModulePath
        # (documented in about_Modules and about_PSModulePath). A machine with
        # both PowerShell editions installed can hand Windows PowerShell a
        # PSModulePath that lists PowerShell 7's module directory ahead of
        # Windows PowerShell's own; Windows PowerShell then autoloads
        # PowerShell 7's copy of that module, which exposes no Get-FileHash to
        # it, and the script would die here with CommandNotFoundException
        # after a successful download instead of verifying it.
        # System.Security.Cryptography.SHA256 is a base class library type,
        # not a module export, so it needs no autoload and does not depend on
        # PSModulePath at all.
        try {
            $sha256 = [System.Security.Cryptography.SHA256]::Create()
            try {
                $stream = [System.IO.File]::OpenRead($tmpfile)
                try {
                    $hashBytes = $sha256.ComputeHash($stream)
                }
                finally {
                    $stream.Dispose()
                }
            }
            finally {
                $sha256.Dispose()
            }
            $gotSha = ([BitConverter]::ToString($hashBytes)).Replace('-', '')
        }
        catch {
            Write-Failure "could not compute a SHA-256 checksum for $published on this machine, so the download cannot be verified; $left"
            exit 1
        }
        if ($gotSha -ne $wantSha) {
            Write-Failure "downloaded file's checksum does not match the manifest for $published; the download will not be installed, and $left"
            exit 1
        }

        # The one step that changes what sits at the install path, and it
        # runs only on a complete, verified download.
        Move-Item -Path $tmpfile -Destination (Join-Path $installDir $installed) -Force
        Write-Host "Installed $published as $(Join-Path $installDir $installed)"
    }
    finally {
        if (Test-Path -Path $tmpfile) {
            Remove-Item -Path $tmpfile -Force -ErrorAction SilentlyContinue
        }
    }
}

# Steps 4 to 8: install dinah.exe, then dinah-tui.exe, the terminal UI's
# program, which dinah tui starts and looks for beside dinah.exe first. The two
# come from the same release, which is what dinah tui checks. A dinah-tui.exe
# that cannot be installed leaves the new dinah.exe in place, and says that
# dinah tui waits on it.
Install-Program $binary 'dinah.exe' 'nothing was installed'
Install-Program $tuiBinary 'dinah-tui.exe' 'dinah.exe is installed and works, but dinah tui needs dinah-tui.exe, so run this script again'

# Step 9: put the install directory on the user's PATH, which needs no
# administrator privilege. A shell started after this can run dinah by name.
#
# Set DINAH_NO_PATH to any value to keep this script away from your PATH. The
# one-liner that pipes this script into PowerShell cannot pass an argument, so
# an environment variable is the way to say no.
#
# Your PATH is read straight out of the registry without expanding anything in
# it, and written back as an expandable value. An entry you wrote as
# %USERPROFILE%\bin stays written that way, rather than being frozen to
# whatever it stood for on the day you installed Dinah.
if ($env:DINAH_NO_PATH) {
    Write-Host "DINAH_NO_PATH is set, so your PATH is unchanged. Run Dinah as $(Join-Path $installDir 'dinah.exe'), or put $installDir on your PATH yourself."
    exit 0
}

# An entry that differs only by a trailing backslash is the same directory, and
# Windows compares paths without regard to case, which -eq also does. This
# session's PATH and the persisted registry PATH are two different questions:
# writing the registry does not change what this already-running process can
# see, so a person who just ran this script in one window and typed dinah in
# the same window needs to be told that, rather than reading advice that only
# makes sense for the next shell they open.
$wanted = $installDir.TrimEnd('\')
$sessionHasIt = @($env:Path -split ';' | ForEach-Object { $_.Trim().TrimEnd('\') } |
    Where-Object { $_ -eq $wanted }).Count -gt 0

$environmentKey = 'HKCU:\Environment'
try {
    $rawPath = (Get-Item -Path $environmentKey).GetValue(
        'PATH', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
}
catch {
    if ($sessionHasIt) {
        Write-Host "$installDir is on this session's PATH. Run: dinah version"
    }
    else {
        Write-Host "Could not read your PATH, so it is unchanged. Run Dinah as $(Join-Path $installDir 'dinah.exe'), or put $installDir on your PATH yourself."
    }
    exit 0
}

$alreadyThere = @($rawPath -split ';' | ForEach-Object { $_.Trim().TrimEnd('\') } |
    Where-Object { $_ -eq $wanted }).Count -gt 0
if ($alreadyThere) {
    if ($sessionHasIt) {
        Write-Host "$installDir is on your PATH. Run: dinah version"
    }
    else {
        Write-Host "$installDir is already configured on your PATH, but this session started before that took effect. Open a new PowerShell window to pick it up, or run this to use dinah now:"
        Write-Host "    `$env:Path = `"$installDir;`$env:Path`""
    }
    exit 0
}

# Trailing semicolons are dropped before appending. An empty entry in PATH
# means the current directory on Windows, and putting one there would make
# every command you type look in whatever directory you happen to be in.
$newPath = if ([string]::IsNullOrEmpty($rawPath)) { $installDir } else { $rawPath.TrimEnd(';') + ';' + $installDir }
Set-ItemProperty -Path $environmentKey -Name 'PATH' -Value $newPath -Type ExpandString

# Writing the registry does not tell programs already running to read it again.
# Broadcasting WM_SETTINGCHANGE for 'Environment' is how Windows is told, and
# it is what lets a shell you open next find Dinah.
$broadcast = $true
try {
    if (-not ('Dinah.NativeMethods' -as [type])) {
        Add-Type -Namespace 'Dinah' -Name 'NativeMethods' -MemberDefinition @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(
    IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam,
    uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
    }
    $unused = [UIntPtr]::Zero
    # HWND_BROADCAST, WM_SETTINGCHANGE, SMTO_ABORTIFHUNG, five seconds.
    [void][Dinah.NativeMethods]::SendMessageTimeout(
        [IntPtr]0xffff, 0x1A, [UIntPtr]::Zero, 'Environment', 0x0002, 5000, [ref]$unused)
}
catch {
    $broadcast = $false
}

if ($sessionHasIt) {
    Write-Host "Added $installDir to your PATH for future sessions. It is already on this session's PATH, so you can run dinah now."
}
elseif ($broadcast) {
    Write-Host "Added $installDir to your PATH. Open a new shell to pick it up, or run this to use dinah now:"
    Write-Host "    `$env:Path = `"$installDir;`$env:Path`""
}
else {
    Write-Host "Added $installDir to your PATH. Open a new shell to pick it up, and if Dinah is still not found there, sign out and back in. To use dinah now in this session, run:"
    Write-Host "    `$env:Path = `"$installDir;`$env:Path`""
}
