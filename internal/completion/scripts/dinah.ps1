# dinah completion for PowerShell, protocol __DINAH_PROTOCOL__.
# Load it from your profile:  dinah completion powershell | Out-String | Invoke-Expression
#
# The script runs the program named by the first word of the line, so the
# candidates always come from the binary that will run the command. It hands
# the words to the callback in the environment rather than on the command
# line, because Windows PowerShell does not escape an embedded quotation mark
# when it passes an argument to a native program, and it restores the exit
# code, the output encoding and the variable before it returns.
Register-ArgumentCompleter -Native -CommandName 'dinah', 'dinah.exe' -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $elements = $commandAst.CommandElements
    $first = $elements[0]
    $program = if ($first -is [System.Management.Automation.Language.StringConstantExpressionAst]) { $first.Value } else { $first.Extent.Text }
    if ($program.StartsWith('~/') -or $program.StartsWith('~\')) { $program = Join-Path $HOME $program.Substring(2) }
    $source = $commandAst.Extent.Text
    $base = $commandAst.Extent.StartOffset
    $words = [System.Collections.Generic.List[string]]::new()
    $current = ''
    for ($i = 1; $i -lt $elements.Count; $i++) {
        $e = $elements[$i]
        if ($e.Extent.StartOffset -lt $cursorPosition -and $e.Extent.EndOffset -ge $cursorPosition) {
            $current = $source.Substring($e.Extent.StartOffset - $base, $cursorPosition - $e.Extent.StartOffset)
            break
        }
        if ($e.Extent.EndOffset -gt $cursorPosition) { break }
        if ($e -is [System.Management.Automation.Language.StringConstantExpressionAst]) { $words.Add($e.Value) } else { $words.Add($e.Extent.Text) }
    }
    $words.Add($current)
    $payload = [ordered]@{ words = [string[]]$words.ToArray(); replacing = [string]$wordToComplete }
    $savedExit = $global:LASTEXITCODE
    $savedWords = $env:DINAH_COMPLETE_WORDS
    $savedEncoding = $null
    $lines = @()
    try {
        $savedEncoding = [Console]::OutputEncoding
        [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
        $env:DINAH_COMPLETE_WORDS = ConvertTo-Json -Compress -InputObject $payload
        $lines = @(& $program __complete __DINAH_PROTOCOL__ powershell 2>$null)
    } catch {
        $lines = @()
    } finally {
        if ($null -ne $savedEncoding) { [Console]::OutputEncoding = $savedEncoding }
        $env:DINAH_COMPLETE_WORDS = $savedWords
        $global:LASTEXITCODE = $savedExit
    }
    if ($lines.Count -eq 0) { return }
    if (-not ($lines[0] -match '^dinah-complete __DINAH_PROTOCOL__ (words|nospace|files|dirs)$')) { return }
    $mode = $Matches[1]
    if ($mode -eq 'files' -or $mode -eq 'dirs') {
        $folder = ''
        $cut = [Math]::Max($wordToComplete.LastIndexOf('/'), $wordToComplete.LastIndexOf('\'))
        if ($cut -ge 0) { $folder = $wordToComplete.Substring(0, $cut + 1) }
        $entries = @(Get-ChildItem -Path ($wordToComplete + '*') -Force -ErrorAction SilentlyContinue)
        foreach ($entry in $entries) {
            if ($mode -eq 'dirs' -and -not $entry.PSIsContainer) { continue }
            $text = $folder + $entry.Name
            if ($text -match '[\s`''"$&@(){};,|<>#]') { $text = "'" + $text.Replace("'", "''") + "'" }
            $kind = if ($entry.PSIsContainer) { 'ProviderContainer' } else { 'ProviderItem' }
            [System.Management.Automation.CompletionResult]::new($text, $entry.Name, $kind, $text)
        }
        return
    }
    if ($lines.Count -lt 2) { return }
    foreach ($line in $lines[1..($lines.Count - 1)]) {
        $tab = $line.IndexOf("`t")
        if ($tab -lt 1) { continue }
        $insert = $line.Substring(0, $tab)
        $tip = $line.Substring($tab + 1)
        if ($tip -eq '') { $tip = $insert }
        [System.Management.Automation.CompletionResult]::new($insert, $insert, 'ParameterValue', $tip)
    }
}
