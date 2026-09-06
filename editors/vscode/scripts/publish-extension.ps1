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

# Which version goes to the marketplace. package.json's version field is a
# floor whose patch is always 0, so it no longer names anything that shipped;
# the real number lives in the release history, which is where the release
# workflow puts it. This script asks that same history rather than reading a
# field that stopped carrying the answer.
#
# Two failures are possible here and they are kept apart, because they call for
# different things from whoever is reading. The listing itself can fail for
# reasons that have nothing to do with whether a release exists: no network, an
# expired token, a rate limit, a transient API error. Every one of those exits
# non-zero, so the exit status is checked immediately, before anything reads
# the text that came back, and the message says the list could not be retrieved.
# Only once the list has arrived is it asked whether it carries a release, and
# that question has its own message. Neither answer is ever read off the
# emptiness of the other one's output.
#
# Both captures redirect standard error into the variable. Piping a command's
# output into a variable in PowerShell captures only its success stream, so
# without the redirection the second call's own message never reaches $version
# and Fail would report an empty string on exactly the path that runs first in
# reality, which is the one where nothing has been released yet.
$Repo = 'paulmooreparks/dinah'
$tagsOutput = & gh api "repos/$Repo/releases" --paginate --jq '.[] | .tag_name' 2>&1
if ($LASTEXITCODE -ne 0) {
    Fail "The releases on $Repo could not be listed ($tagsOutput), so the newest published extension version is unknown. Nothing was packaged or published."
}
$version = $tagsOutput | node (Join-Path $extensionRoot 'scripts/print-newest-version.mjs') - 2>&1
if ($LASTEXITCODE -ne 0) {
    Fail "$version"
}
Write-Output "Publishing the newest released extension version, $version, paired with dinah $Tag."

Push-Location $extensionRoot
try {
    $env:DINAH_PAIRED_RELEASE = $Tag
    npm ci
    if ($LASTEXITCODE -ne 0) { Fail "npm ci failed." }
    npm run package -- --version $version
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
            vsce publish --packagePath $archive.FullName
            if ($LASTEXITCODE -ne 0) { Fail "Publishing $($archive.Name) failed." }
        }
    }
}
finally {
    Pop-Location
    Remove-Item Env:\DINAH_PAIRED_RELEASE -ErrorAction SilentlyContinue
}

Write-Output "Done."
