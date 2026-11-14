param(
    [Parameter(Mandatory=$true)][string]$Before,
    [Parameter(Mandatory=$true)][string]$After
)
$ErrorActionPreference='Stop'
& (Join-Path $PSScriptRoot 'check-bus-growth-receipt.ps1') -Path $Before
& (Join-Path $PSScriptRoot 'check-bus-growth-receipt.ps1') -Path $After
$old=@(Get-Content -LiteralPath $Before -Raw | ConvertFrom-Json)
$new=@(Get-Content -LiteralPath $After -Raw | ConvertFrom-Json)
$checked=0
foreach ($factor in @(1,10,100)) {
    $a=@($old | Where-Object factor -eq $factor)[0]
    $b=@($new | Where-Object factor -eq $factor)[0]
    if ($a.messages -ne $b.messages -or $a.deliveries -ne $b.deliveries) {
        throw "Fixture counts differ at factor $factor."
    }
    $names=@($a.queries.PSObject.Properties.Name | Sort-Object)
    if (($names -join ',') -ne (@($b.queries.PSObject.Properties.Name | Sort-Object) -join ',')) {
        throw "Query names differ at factor $factor."
    }
    foreach ($name in $names) {
        $left=$a.queries.$name
        $right=$b.queries.$name
        if ($left.resultHash -notmatch '^[0-9a-f]{64}$' -or $right.resultHash -ne $left.resultHash -or $left.rows -ne $right.rows) {
            throw "Result rows differ for $name at factor $factor."
        }
        $checked++
    }
}
Write-Output "All $checked query results have identical row counts and complete-result SHA256 hashes."
