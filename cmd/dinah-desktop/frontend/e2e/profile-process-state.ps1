param([int]$NativePid, [string]$ProfileRoot)
$ErrorActionPreference = 'Stop'
Add-Type @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
public static class ProfileTrialWindows {
 public delegate bool Callback(IntPtr window, IntPtr parameter);
 [DllImport("user32.dll")] static extern bool EnumWindows(Callback callback, IntPtr parameter);
 [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr window, out uint pid);
 [DllImport("user32.dll")] static extern bool IsWindowVisible(IntPtr window);
 public static bool[] Visibility(int pid) {
  var values = new List<bool>();
  EnumWindows((window, parameter) => { uint owner; GetWindowThreadProcessId(window, out owner); if(owner == pid) values.Add(IsWindowVisible(window)); return true; }, IntPtr.Zero);
  return values.ToArray();
 }
}
'@
$views = @(Get-CimInstance Win32_Process -Filter "Name='msedgewebview2.exe'" | ForEach-Object {
 $line = $_.CommandLine
 if ($line -match '--user-data-dir=(?:"([^"]+)"|([^\s]+))') {
  $folder = if ($Matches[1]) { $Matches[1] } else { $Matches[2] }
  if ($folder.StartsWith($ProfileRoot, [StringComparison]::OrdinalIgnoreCase)) {
   [pscustomobject]@{ pid = $_.ProcessId; parentPid = $_.ParentProcessId; userDataPath = $folder }
  }
 }
})
$windows = @([ProfileTrialWindows]::Visibility($NativePid))
[pscustomobject]@{ windows = $windows; webviews = $views } | ConvertTo-Json -Depth 5 -Compress
