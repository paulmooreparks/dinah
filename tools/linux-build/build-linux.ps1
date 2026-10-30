# Builds Dinah for Linux (amd64) in a container: the command line, dinah, and
# the window, dinah-desktop. The window links the Linux webview (GTK 3 and
# WebKitGTK 4.1) through C, so it cannot be cross-compiled from Windows the way
# the command line can. Needs Docker. The image is built from the Dockerfile
# beside this script, and Go's module and build caches are kept between builds
# in the volumes dinah-go-mod-cache and dinah-go-build-cache.
#
# The programs are written to -OutDir as dinah-linux-amd64 and
# dinah-desktop-linux-amd64. -OutDir has to be inside the repository, because
# only the repository is mounted in the container, and it defaults to the top
# of the repository.
#
# -Version stamps both programs with a release version; without it they keep
# the versions in the source. -Test also runs go vet and go test on Linux, in
# the same container, before building.
#
# Usage:  pwsh tools/linux-build/build-linux.ps1 [-OutDir <folder>] [-Version 1.2.0] [-Test]
[CmdletBinding()]
param(
    [string]$OutDir = "",
    [string]$Version = "",
    [switch]$Test
)
$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw "the Linux build needs Docker" }
$out = if ($OutDir) { [IO.Path]::GetFullPath($OutDir, $root) } else { $root }
if (-not $out.StartsWith($root, [StringComparison]::OrdinalIgnoreCase)) { throw "the Linux build writes inside the repository; choose -OutDir there" }
$rel = $out.Substring($root.Length).TrimStart('\', '/').Replace('\', '/')
$dest = if ($rel) { "/src/$rel" } else { "/src" }

# In a git worktree, .git is a file that points to the main repository by its
# path on this machine, which the container cannot see, so Go is told not to
# stamp the commit. In a full clone the container's git is told to trust the
# mounted folder, which another user owns as far as the container is concerned.
$vcs = ""
$gitEnv = @()
if (Test-Path (Join-Path $root ".git") -PathType Leaf) {
    $vcs = "-buildvcs=false"
} else {
    $gitEnv = @("-e", "GIT_CONFIG_COUNT=1", "-e", "GIT_CONFIG_KEY_0=safe.directory", "-e", "GIT_CONFIG_VALUE_0=/src")
}

# A replace directive in go.mod that points to a local folder, such as a
# sibling checkout of dinah-extension-go, is mounted at the same place relative
# to /src, so the container builds the same source the host would.
$mounts = @()
foreach ($line in Get-Content (Join-Path $root "go.mod")) {
    if ($line -match '=>\s*(\.\.?/[^\s]+)') {
        $local = $Matches[1]
        $hostDir = [IO.Path]::GetFullPath($local, $root)
        if (-not (Test-Path $hostDir -PathType Container)) { throw "go.mod replaces a module with $local, which is not at $hostDir" }
        $item = Get-Item $hostDir
        if ($item.LinkTarget) { $hostDir = [IO.Path]::GetFullPath($item.LinkTarget, $item.Parent.FullName) }
        $parts = [Collections.Generic.List[string]]::new()
        foreach ($p in "src/$local".Split('/')) {
            if ($p -eq '..') { if ($parts.Count) { $parts.RemoveAt($parts.Count - 1) } }
            elseif ($p -and $p -ne '.') { $parts.Add($p) }
        }
        $inside = "/" + ($parts -join '/')
        $mounts += @("-v", "${hostDir}:$inside")
    }
}

$cliFlags = "-s -w"
$appFlags = "-s -w"
if ($Version) {
    $cliFlags += " -X studio/internal/mcpserver.Version=$Version"
    $appFlags += " -X main.appVersion=$Version"
}

docker build --quiet -t dinah-linux-build $PSScriptRoot | Out-Null
if ($LASTEXITCODE -ne 0) { throw "the Linux build image could not be built" }
$tags = "desktop,production,webkit2_41"
$script = "mkdir -p $dest" +
    " && CGO_ENABLED=0 go build -trimpath $vcs -ldflags '$cliFlags' -o $dest/dinah-linux-amd64 ./cmd/dinah" +
    " && go build -trimpath $vcs -tags $tags -ldflags '$appFlags' -o $dest/dinah-desktop-linux-amd64 ./cmd/dinah-desktop"
if ($Test) { $script = "go vet $vcs -tags $tags ./... && go test $vcs -tags $tags ./... && $script" }
docker run --rm @gitEnv -v "${root}:/src" @mounts -v dinah-go-mod-cache:/go/pkg/mod -v dinah-go-build-cache:/root/.cache/go-build -w /src dinah-linux-build sh -c $script
if ($LASTEXITCODE -ne 0) { throw "the Linux build failed" }
Write-Host "built $(Join-Path $out 'dinah-linux-amd64')"
Write-Host "built $(Join-Path $out 'dinah-desktop-linux-amd64')"
