# Prints the hash of the shell's source: SHA-256, in lower-case hex, of each
# of go.mod, go.sum and main.go in that order, as its name, a line break,
# its text with CRLF line ends made LF, and a line break. TestShellBuildIsCurrent
# in cmd/dinah-desktop computes the same.
$text = ''
foreach ($name in 'go.mod', 'go.sum', 'main.go') {
    $content = [System.IO.File]::ReadAllText((Join-Path $PSScriptRoot $name)) -replace "`r`n", "`n"
    $text += $name + "`n" + $content + "`n"
}
$bytes = [System.Text.Encoding]::UTF8.GetBytes($text)
$sha = [System.Security.Cryptography.SHA256]::Create()
-join ($sha.ComputeHash($bytes) | ForEach-Object { $_.ToString('x2') })
