param([Parameter(Mandatory=$true)][string]$Out)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$folder = [IO.Path]::GetFullPath($Out)
if (Test-Path -LiteralPath $folder) { throw 'Use a new output folder so earlier evidence is preserved.' }
New-Item -ItemType Directory -Path $folder | Out-Null
$versions = @('0.0.0-profiletrial.1', '0.0.0-profiletrial.2')
$binaries = @()
Push-Location $root
try {
 foreach ($version in $versions) {
  $exe = Join-Path $folder ($version + '.exe')
  & go build -buildvcs=false -tags desktop,production -ldflags "-H windowsgui -X main.appVersion=$version" -o $exe ./cmd/dinah-desktop
  if ($LASTEXITCODE -ne 0) { throw "Build failed for $version" }
  $binaries += $exe
 }
 $package = (& go list -tags desktop,production -json ./cmd/dinah-desktop | ConvertFrom-Json)
 if ($LASTEXITCODE -ne 0) { throw 'Asset manifest failed' }
 [IO.File]::WriteAllText((Join-Path $folder 'source.json'),(ConvertTo-Json @{commit=(& git rev-parse HEAD);packageDirectory=$package.Dir;buildVcs=$false}))
 $files = @($package.EmbedFiles | Where-Object { $_.StartsWith('frontend/') } | ForEach-Object { $_.Substring(9) } | Sort-Object)
 $manifest = Join-Path $folder 'assets.json'
 [IO.File]::WriteAllText($manifest, (ConvertTo-Json -InputObject $files))
 $driver = Join-Path $folder 'driver'
 & npm.cmd install --prefix $driver --ignore-scripts --no-audit --no-fund playwright-core@1.64.0
 if ($LASTEXITCODE -ne 0) { throw 'Playwright driver installation failed' }
 $env:NODE_PATH = Join-Path $driver 'node_modules'
 & node (Join-Path $root 'cmd/dinah-desktop/frontend/e2e/profile-version-hidden.cjs') $binaries[0] $binaries[1] $folder $manifest
 if ($LASTEXITCODE -ne 0) { throw 'Profile trial failed; inspect receipt.json' }
 $hashes = @($binaries | ForEach-Object { Get-FileHash -LiteralPath $_ -Algorithm SHA256 | Select-Object Path,Hash })
 [IO.File]::WriteAllText((Join-Path $folder 'builds.json'),(ConvertTo-Json -InputObject $hashes))
} finally { Pop-Location }
