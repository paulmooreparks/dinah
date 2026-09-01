# Publishes the Dinah VS Code extension to the Visual Studio Marketplace.
#
# Usage: pwsh ./scripts/publish-extension.ps1 -Tag v0.1.42-dev
#
# Publishing is manual and it is local. release.yml fires on most pushes to
# main, so a marketplace publish per commit would push an update notification
# at every installed user several times a day. It runs here rather than in
# Actions because the credentials are a publisher login on the operator's own
# machine, which is how XferLangVSCode has always been shipped, and nothing
# about them ever enters this repository.
#
# The accepted cost is that publishing depends on that one machine and that
# one login. Nobody else can cut a marketplace release, and the step is not
# reproducible from a clean checkout. Building, testing and packaging are all
# still in CI, which is where a change is actually gated; only the publish
# is here.

[CmdletBinding()]
param(
    # The dinah release tag whose binaries the platform packages carry.
    [Parameter(Mandatory = $true)]
    [string]$Tag,

    # Package everything and stop before the publish.
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'

$extensionRoot = Split-Path -Parent $PSScriptRoot

function Fail([string]$message) {
    Write-Error $message
    exit 1
}

Write-Output "Checking for vsce..."
if (-not (Get-Command vsce -ErrorAction SilentlyContinue)) {
    Write-Output "vsce not found. Installing @vscode/vsce globally..."
    npm install -g @vscode/vsce
    if ($LASTEXITCODE -ne 0) {
        Fail "Could not install @vscode/vsce. Install it by hand with 'npm install -g @vscode/vsce' and run this again."
    }
}

Write-Output "Checking for a logged-in marketplace publisher..."
$publishers = vsce ls-publishers 2>&1 | Out-String
if ($publishers -match 'No publishers found') {
    Fail "No marketplace publisher is logged in on this machine. Run 'vsce login paulmooreparks' and follow the prompts, then run this again. The extension publishes as paulmooreparks.dinah, and that identifier is permanent from the first publish."
}

Write-Output "Checking for the GitHub CLI..."
if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    Fail "The GitHub CLI is not installed, and it is what downloads the release binaries each platform package carries. Install it from https://cli.github.com/ and run this again."
}

# The extension's version is its own and is read rather than computed. The
# tag still decides which binaries the platform packages carry, and it is
# recorded inside the build as provenance, but nothing derives one project's
# number from the other's any more.
$manifestPath = Join-Path $extensionRoot 'package.json'
$version = (Get-Content -Path $manifestPath -Raw | ConvertFrom-Json).version
if ([string]::IsNullOrWhiteSpace($version)) {
    Fail "package.json carries no version, so there is nothing to publish. Set the extension's own version there and run this again."
}
Write-Output "Publishing extension version $version, built against the binaries from dinah $Tag."

$binaries = Join-Path $extensionRoot '.release-binaries'
if (Test-Path $binaries) {
    Remove-Item -Recurse -Force $binaries
}
New-Item -ItemType Directory -Path $binaries | Out-Null

Write-Output "Downloading the release binaries for $Tag..."
gh release download $Tag --repo paulmooreparks/dinah --dir $binaries --pattern 'dinah-*' --clobber
if ($LASTEXITCODE -ne 0) {
    Fail "Could not download the binaries for $Tag. Check that the release exists and that 'gh auth status' reports you as logged in."
}
Remove-Item -Path (Join-Path $binaries 'SHA256SUMS.txt') -ErrorAction SilentlyContinue

# --published tells the packaging step that these archives are the ones going
# to the marketplace, so they carry the committed version above rather than the
# unpublished ordinal every other build gets.
Push-Location $extensionRoot
try {
    $env:DINAH_PAIRED_RELEASE = $Tag
    npm ci
    if ($LASTEXITCODE -ne 0) { Fail "npm ci failed." }
    npm run package -- --binaries $binaries --published
    if ($LASTEXITCODE -ne 0) { Fail "Packaging failed." }
    npm run verify-package
    if ($LASTEXITCODE -ne 0) { Fail "The packaged archives did not carry the binaries they should. Nothing was published." }

    $archives = Get-ChildItem -Path (Join-Path $extensionRoot 'vsix') -Filter '*.vsix'
    if ($DryRun) {
        Write-Output "Dry run. These would be published:"
        $archives | ForEach-Object { Write-Output "  $($_.Name)" }
    }
    else {
        foreach ($archive in $archives) {
            Write-Output "Publishing $($archive.Name)..."
            vsce publish --pre-release --packagePath $archive.FullName
            if ($LASTEXITCODE -ne 0) { Fail "Publishing $($archive.Name) failed." }
        }
    }
}
finally {
    Pop-Location
    Remove-Item Env:\DINAH_PAIRED_RELEASE -ErrorAction SilentlyContinue
}

Write-Output "Done."
