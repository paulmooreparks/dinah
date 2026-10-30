param([Parameter(Mandatory)][string]$Root, [ValidateSet('inspect','maximize','unminimize','hide')][string]$Action='inspect')
$ErrorActionPreference='Stop'
$Root=[IO.Path]::GetFullPath($Root)
if (!(Split-Path $Root -Leaf).StartsWith('dinah-fullscreen-live-')) { throw 'A disposable fullscreen test directory is required.' }
$targetId=[int](Get-Content (Join-Path $Root 'pid.txt'))
$process=Get-Process -Id $targetId
$expected=Get-Content (Join-Path $Root 'native-executable.txt') -Raw
if ($process.Path -ne [IO.Path]::GetFullPath($expected.Trim())) { throw 'The recorded native test executable does not match the process.' }
Add-Type @'
using System;
using System.Runtime.InteropServices;
using System.Text;
public static class NativeProbe {
 [StructLayout(LayoutKind.Sequential)] public struct Rect { public int Left,Top,Right,Bottom; }
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern IntPtr FindWindowEx(IntPtr parent,IntPtr after,string cls,string title);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] static extern int GetWindowText(IntPtr hwnd,StringBuilder text,int count);
 public static string Title(IntPtr hwnd) { var text=new StringBuilder(512); GetWindowText(hwnd,text,512); return text.ToString(); }
 public static IntPtr Next(IntPtr previous) { return FindWindowEx(IntPtr.Zero,previous,null,null); }
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hwnd,out uint pid);
 [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hwnd,out Rect rect);
 [DllImport("user32.dll",EntryPoint="GetWindowLongPtrW")] public static extern IntPtr GetWindowLongPtr(IntPtr hwnd,int index);
 [DllImport("user32.dll")] public static extern bool IsZoomed(IntPtr hwnd);
 [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr hwnd);
 [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hwnd,int command);
}
'@
$handle=[IntPtr]::Zero
do {
 $handle=[NativeProbe]::Next($handle)
 if($handle -eq [IntPtr]::Zero){throw 'The native test window was not found.'}
 [uint32]$ownerId=0
 [void][NativeProbe]::GetWindowThreadProcessId($handle,[ref]$ownerId)
}while($ownerId -ne $targetId -or [NativeProbe]::Title($handle) -ne 'Dinah')
if($Action -ne 'inspect') {
 $command=if($Action -eq 'maximize'){3}elseif($Action -eq 'unminimize'){9}else{0}
 [void][NativeProbe]::ShowWindow($handle,$command)
 [void][NativeProbe]::ShowWindow($handle,0)
}
$rect=[NativeProbe+Rect]::new()
if(![NativeProbe]::GetWindowRect($handle,[ref]$rect)){throw 'Native rectangle query failed.'}
@{left=$rect.Left;top=$rect.Top;width=$rect.Right-$rect.Left;height=$rect.Bottom-$rect.Top;caption=(([NativeProbe]::GetWindowLongPtr($handle,-16).ToInt64() -band 0x00C00000) -ne 0);maximized=[NativeProbe]::IsZoomed($handle);minimized=[NativeProbe]::IsIconic($handle)} | ConvertTo-Json -Compress
