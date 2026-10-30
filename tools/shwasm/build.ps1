# Builds the Terminal's POSIX shell (main.go, mvdan.cc/sh compiled to
# WebAssembly) into cmd/dinah-desktop/frontend/vendor/gosh, with the
# wasm_exec.js of the Go that built it, mvdan.cc/sh's licence, and
# SOURCE.sha256, the hash of the source it was built from. The build is
# committed, so the app builds without this step; TestShellBuildIsCurrent
# fails when the source changes and the build is not made again.
$ErrorActionPreference = 'Stop'
$here = $PSScriptRoot
$out = Join-Path $here '..\..\cmd\dinah-desktop\frontend\vendor\gosh'
New-Item -ItemType Directory -Force $out | Out-Null
Push-Location $here
try {
    $env:GOOS = 'js'; $env:GOARCH = 'wasm'
    go build -trimpath -ldflags='-s -w' -o (Join-Path $out 'sh.wasm') .
    if ($LASTEXITCODE -ne 0) { throw 'the shell did not build' }
    Remove-Item Env:GOOS, Env:GOARCH
    $root = go env GOROOT
    $exec = Join-Path $root 'lib\wasm\wasm_exec.js'
    if (-not (Test-Path $exec)) { $exec = Join-Path $root 'misc\wasm\wasm_exec.js' }
    Copy-Item $exec (Join-Path $out 'wasm_exec.js') -Force
    $sh = go list -m -f '{{.Dir}}' mvdan.cc/sh/v3
    Copy-Item (Join-Path $sh 'LICENSE') (Join-Path $out 'LICENSE-mvdan-sh') -Force
    Copy-Item (Join-Path $root 'LICENSE') (Join-Path $out 'LICENSE-go') -Force
    $hash = & (Join-Path $here 'source-hash.ps1')
    Set-Content -NoNewline -Path (Join-Path $out 'SOURCE.sha256') -Value $hash
    "built $((Get-Item (Join-Path $out 'sh.wasm')).Length) bytes, source $hash, with $(go version)"
} finally {
    Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
    Pop-Location
}
