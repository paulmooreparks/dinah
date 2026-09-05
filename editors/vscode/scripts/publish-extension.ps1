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
    # The dinah release this build of the extension is stamped as paired
    # with. It is recorded inside the build as provenance and shown in the
    # status bar; it decides nothing about what the archive contains.
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

# The extension's version is its own and is read rather than computed. The
# tag is recorded inside the build as provenance, and nothing derives one
# project's number from the other's.
$manifestPath = Join-Path $extensionRoot 'package.json'
$version = (Get-Content -Path $manifestPath -Raw | ConvertFrom-Json).version
if ([string]::IsNullOrWhiteSpace($version)) {
    Fail "package.json carries no version, so there is nothing to publish. Set the extension's own version there and run this again."
}
Write-Output "Publishing extension version $version, paired with dinah $Tag."

# --published tells the packaging step that this archive is the one going to
# the marketplace, so it carries the committed version above rather than the
# unpublished ordinal every other build gets.
Push-Location $extensionRoot
try {
    $env:DINAH_PAIRED_RELEASE = $Tag
    npm ci
    if ($LASTEXITCODE -ne 0) { Fail "npm ci failed." }
    npm run package -- --published
    if ($LASTEXITCODE -ne 0) { Fail "Packaging failed." }
    npm run verify-package
    if ($LASTEXITCODE -ne 0) { Fail "The packaged archive did not carry what it should. Nothing was published." }

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
