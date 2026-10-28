param(
 [Parameter(Mandatory=$true)][uint32]$OwnerProcessID,
 [Parameter(Mandatory=$true)][string]$ExecutablePath,
 [Parameter(Mandatory=$true)][string]$OutputFile
)
# Hosted acceptance only. Enumerate this known fixture PID's single visible
# toolkit host, then PrintWindow that exact HWND; never capture the desktop.
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Drawing
Add-Type @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
public static class EditorNativeWindow {
 public delegate bool EnumWindow(IntPtr h,IntPtr p);
 [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindow callback,IntPtr p);
 [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h,out uint p);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h,StringBuilder s,int n);
 [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h,out RECT r);
 [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr h,out RECT r);
 [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h,IntPtr dc,uint flags);
 [DllImport("user32.dll")] public static extern bool RedrawWindow(IntPtr h,IntPtr r,IntPtr region,uint flags);
 [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left,Top,Right,Bottom; }
 public static IntPtr[] Find(uint pid) {
  var found=new List<IntPtr>();
  EnumWindows((h,p)=>{uint owner;GetWindowThreadProcessId(h,out owner);var name=new StringBuilder(256);GetClassName(h,name,256);if(owner==pid&&IsWindowVisible(h)&&name.ToString()=="NetworkToolkit.WebView2.Window")found.Add(h);return true;},IntPtr.Zero);
  return found.ToArray();
 }
}
'@
$process=Get-Process -Id $OwnerProcessID -ErrorAction Stop
if([IO.Path]::GetFullPath($process.Path) -ne [IO.Path]::GetFullPath($ExecutablePath)){throw 'Native fixture executable identity mismatch'}
$until=[DateTime]::UtcNow.AddSeconds(15)
do {
 $windows=@([EditorNativeWindow]::Find($OwnerProcessID))
 if($windows.Count -eq 1){break}
 Start-Sleep -Milliseconds 100
}while([DateTime]::UtcNow -lt $until)
if($windows.Count -ne 1){throw ('Expected exactly one visible native fixture host for PID '+$OwnerProcessID+', got '+$windows.Count)}
$hwnd=$windows[0]
$rect=New-Object EditorNativeWindow+RECT
if(![EditorNativeWindow]::GetWindowRect($hwnd,[ref]$rect)){throw 'Native HWND dimensions unavailable'}
$width=$rect.Right-$rect.Left;$height=$rect.Bottom-$rect.Top
if($width -lt 100 -or $height -lt 100 -or $width -gt 4096 -or $height -gt 4096){throw 'Unexpected native fixture size'}
$bitmap=New-Object Drawing.Bitmap($width,$height)
$graphics=[Drawing.Graphics]::FromImage($bitmap)
try {
 [void][EditorNativeWindow]::RedrawWindow($hwnd,[IntPtr]::Zero,[IntPtr]::Zero,0x585)
 $dc=$graphics.GetHdc()
 try {if(![EditorNativeWindow]::PrintWindow($hwnd,$dc,2)){throw 'PrintWindow could not render this fixture HWND'}} finally {$graphics.ReleaseHdc($dc)}
 # Ignore title/borders. A colorful caption around a blank WebView is not proof.
 $colors=New-Object 'Collections.Generic.HashSet[int]';$nonblack=0;$samples=0
 for($y=80;$y -lt $height-20;$y+=4){for($x=25;$x -lt $width-25;$x+=4){$pixel=$bitmap.GetPixel($x,$y);$samples++;if($pixel.R -gt 16 -or $pixel.G -gt 16 -or $pixel.B -gt 16){$nonblack++};if($colors.Count -lt 16){[void]$colors.Add($pixel.ToArgb())}}}
 if($colors.Count -lt 4 -or $nonblack -lt $samples/10){throw 'Native editor client is blank or unpainted'}
 [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($OutputFile)))|Out-Null
 if(Test-Path -LiteralPath $OutputFile){throw 'Refusing to replace existing native evidence'}
 $bitmap.Save($OutputFile,[Drawing.Imaging.ImageFormat]::Png)
 @{hwnd=$hwnd.ToInt64();pid=$OwnerProcessID;class='NetworkToolkit.WebView2.Window';width=$width;height=$height;painted=$true;capture='PrintWindow PW_RENDERFULLCONTENT; exact fixture PID and executable'}|ConvertTo-Json -Compress
} finally {$graphics.Dispose();$bitmap.Dispose()}
