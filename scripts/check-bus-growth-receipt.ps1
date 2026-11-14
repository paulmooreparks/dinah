param([Parameter(Mandatory = $true)][string]$Path)
$ErrorActionPreference = 'Stop'
$receipts = @(Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json)
if ($receipts.Count -ne 3) { throw 'The receipt must contain all three sizes.' }
$lastBytes = 0L
foreach ($factor in @(1, 10, 100)) {
    $sizedReceipts = @($receipts | Where-Object { $_.factor -eq $factor })
    if ($sizedReceipts.Count -ne 1) { throw "Missing or repeated factor $factor." }
    $receipt = $sizedReceipts[0]
    if ($receipt.messages -ne 7825 * $factor) { throw "Wrong message count at factor $factor." }
    if ($receipt.deliveries -lt $receipt.messages * 4) { throw "Missing delivery fan-out at factor $factor." }
    if ($receipt.databaseBytes -le $lastBytes) { throw 'Database sizes did not increase.' }
    $lastBytes = $receipt.databaseBytes
    $expectedRows = @{wake=1; doorbell=5; decisions=7; decisionsFreshBoundary=7; group100=100; attentionImport=5; delivery5=5; deliveryEmpty=0}
    foreach ($name in $expectedRows.Keys) {
        $query = $receipt.queries.$name
        if ($null -eq $query -or $query.rows -ne $expectedRows[$name]) { throw "Wrong $name result at factor $factor." }
        if ($query.medianMs -lt 0 -or $query.p95Ms -lt $query.medianMs) { throw "Invalid timing summary for $name." }
    }
    if ($receipt.pinnedWalBytes -le 0 -or $receipt.unpinnedWalBytes -ne 0) { throw "Unexpected WAL sizes at factor $factor." }
}
Write-Output 'All three receipts have the expected query results and file states.'
