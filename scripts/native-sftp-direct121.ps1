param(
 [Parameter(Mandatory=$true)][uint32]$OwnerProcessID,
 [Parameter(Mandatory=$true)][string]$ExecutablePath,
 [string]$DestinationDirectory,
 [string]$FinishedFile,
 [string]$MaterializingFile='', [string]$ExpectedPath='',
 [double]$FromX,[double]$FromY,
 [double]$FromWidth,[double]$FromHeight,
 [ValidateSet('Arrange','Copy','CancelBefore','CancelDuring')][string]$Mode='Copy'
)
# Hosted synthetic acceptance only. Input goes to exact supplied fixture PIDs and
# executable identities. No clipboard, synthetic DOM drag, debug port, security
# setting, unrelated window activation, or full-desktop screenshot is used.
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
public static class SFTPDirectInput {
 public delegate bool EnumWindow(IntPtr h,IntPtr p);
 [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindow callback,IntPtr p);
 [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
 [DllImport("user32.dll")] public static extern bool IsZoomed(IntPtr h);
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h,out uint p);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h,StringBuilder s,int n);
 [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr h,out RECT r);
 [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr h,ref POINT p);
 [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr h,IntPtr after,int x,int y,int w,int z,uint flags);
 [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h,int show);
 [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
 [DllImport("user32.dll")] public static extern bool SetCursorPos(int x,int y);
 [DllImport("user32.dll")] public static extern void mouse_event(uint flags,uint dx,uint dy,uint data,UIntPtr extra);
 [DllImport("user32.dll")] public static extern void keybd_event(byte key,byte scan,uint flags,UIntPtr extra);
 [DllImport("user32.dll")] public static extern IntPtr WindowFromPoint(POINT p);
 [DllImport("user32.dll")] public static extern IntPtr GetAncestor(IntPtr h,uint flags);
 [StructLayout(LayoutKind.Sequential)] public struct RECT {public int Left,Top,Right,Bottom;}
 [StructLayout(LayoutKind.Sequential)] public struct POINT {public int X,Y;}
 public static IntPtr[] Find(uint pid){var found=new List<IntPtr>();EnumWindows((h,p)=>{uint owner;GetWindowThreadProcessId(h,out owner);var s=new StringBuilder(256);GetClassName(h,s,256);if(owner==pid&&IsWindowVisible(h)&&s.ToString()=="NetworkToolkit.WebView2.Window")found.Add(h);return true;},IntPtr.Zero);return found.ToArray();}
}
'@
function Fixture-Window([uint32]$processID,[string]$executable){
 $process=Get-Process -Id $processID -ErrorAction Stop
 if([IO.Path]::GetFullPath($process.Path) -ne [IO.Path]::GetFullPath($executable)){throw 'Native gesture fixture executable identity mismatch'}
 $found=@([SFTPDirectInput]::Find($processID));if($found.Count -ne 1){throw 'Expected one visible host owned by exact fixture PID'};return $found[0]
}
function Client-Point($handle,[double]$x,[double]$y,[double]$width,[double]$height){
 $r=New-Object SFTPDirectInput+RECT
 if(![SFTPDirectInput]::GetClientRect($handle,[ref]$r) -or $width -le 0 -or $height -le 0 -or $x -lt 0 -or $y -lt 0 -or $x -ge $width -or $y -ge $height){throw 'Invalid fixture viewport point'}
 $p=New-Object SFTPDirectInput+POINT;$p.X=[int]($x*($r.Right-$r.Left)/$width);$p.Y=[int]($y*($r.Bottom-$r.Top)/$height)
 if(![SFTPDirectInput]::ClientToScreen($handle,[ref]$p)){throw 'Cannot locate fixture client point'};return $p
}
function Visible-Point($handle,$point){if([SFTPDirectInput]::GetAncestor([SFTPDirectInput]::WindowFromPoint($point),2) -ne $handle){throw 'Expected fixture point is covered by another HWND'}}
$owner=Fixture-Window $OwnerProcessID $ExecutablePath
$work=[Windows.Forms.Screen]::PrimaryScreen.WorkingArea
if($Mode -eq 'Arrange'){
 [void][SFTPDirectInput]::ShowWindow($owner,9)
 $width=[Math]::Min(1100,$work.Width-220);$height=[Math]::Min(900,$work.Height-36)
 [void][SFTPDirectInput]::SetWindowPos($owner,[IntPtr]::Zero,$work.Left,$work.Top,$width,$height,0x0040)
 Start-Sleep -Milliseconds 350
 @{mode=$Mode;sourcePID=$OwnerProcessID;width=$width;height=$height;actualWindowResize=$true}|ConvertTo-Json -Compress;exit
}
if(!$DestinationDirectory -or !$FinishedFile){throw 'Synthetic copy needs owned destination and finish marker'}
$source=Client-Point $owner $FromX $FromY $FromWidth $FromHeight
$shell=New-Object -ComObject Shell.Application
$namespace=$shell.NameSpace((Get-Item -LiteralPath $DestinationDirectory).FullName)
if(!$namespace -or !$namespace.Self.IsFolder){throw 'Synthetic Explorer destination not found'}
$directory=$namespace.Self.Path
Start-Process explorer.exe -ArgumentList @('/n,',('"'+$directory+'"'))
$window=$null;$until=[DateTime]::UtcNow.AddSeconds(18)
do{foreach($w in @($shell.Windows())){try{if($w.Document.Folder.Self.Path -eq $directory){$window=$w;break}}catch{}};if(!$window){Start-Sleep -Milliseconds 100}}while(!$window -and [DateTime]::UtcNow -lt $until)
if(!$window){throw 'Owned synthetic Explorer destination did not open'}
try{
 $explorer=[IntPtr]([long]$window.HWND);$work=[Windows.Forms.Screen]::PrimaryScreen.WorkingArea
 [void][SFTPDirectInput]::SetWindowPos($explorer,[IntPtr]::Zero,$work.Right-420,$work.Top,420,[Math]::Min(720,$work.Height-20),0x0040)
 Start-Sleep -Milliseconds 300
 $to=New-Object SFTPDirectInput+POINT;$to.X=$work.Right-80;$to.Y=$work.Top+430
 [void][SFTPDirectInput]::SetForegroundWindow($owner);Start-Sleep -Milliseconds 100;Visible-Point $owner $source;Visible-Point $explorer $to
 [void][SFTPDirectInput]::SetCursorPos($source.X,$source.Y);[SFTPDirectInput]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
 try{
  Start-Sleep -Milliseconds 150
  [void][SFTPDirectInput]::SetCursorPos($source.X+10,$source.Y+2);Start-Sleep -Milliseconds 200
  for($i=1;$i -le 28;$i++){[void][SFTPDirectInput]::SetCursorPos([int]($source.X+($to.X-$source.X)*$i/28),[int]($source.Y+($to.Y-$source.Y)*$i/28));Start-Sleep -Milliseconds 25}
  Start-Sleep -Milliseconds 500
  if($Mode -eq 'CancelBefore'){[SFTPDirectInput]::keybd_event(27,0,0,[UIntPtr]::Zero);Start-Sleep -Milliseconds 150;[SFTPDirectInput]::keybd_event(27,0,2,[UIntPtr]::Zero)}
 }finally{[SFTPDirectInput]::mouse_event(4,0,0,0,[UIntPtr]::Zero)}
 if($Mode -eq 'CancelDuring'){
  $until=[DateTime]::UtcNow.AddSeconds(15);while(!(Test-Path -LiteralPath $MaterializingFile) -and [DateTime]::UtcNow -lt $until){Start-Sleep -Milliseconds 20}
  if(!(Test-Path -LiteralPath $MaterializingFile)){throw 'Actual loopback bulk response never began materializing'}
  [SFTPDirectInput]::keybd_event(27,0,0,[UIntPtr]::Zero);Start-Sleep -Milliseconds 200;[SFTPDirectInput]::keybd_event(27,0,2,[UIntPtr]::Zero)
 }
 $until=[DateTime]::UtcNow.AddSeconds(25);while(!(Test-Path -LiteralPath $FinishedFile) -and [DateTime]::UtcNow -lt $until){Start-Sleep -Milliseconds 50}
 if(!(Test-Path -LiteralPath $FinishedFile)){throw 'Native copy gesture never sent its authenticated terminal finish'}
 if($Mode -eq 'Copy'){
  $until=[DateTime]::UtcNow.AddSeconds(15);while(!(Test-Path -LiteralPath $ExpectedPath) -and [DateTime]::UtcNow -lt $until){Start-Sleep -Milliseconds 50}
  if(!(Test-Path -LiteralPath $ExpectedPath)){throw 'Actual Explorer destination file did not arrive'}
 }else{if(@(Get-ChildItem -LiteralPath $DestinationDirectory -Force).Count -ne 0){throw 'Cancelled gesture published files into destination'}}
 @{mode=$Mode;sourcePID=$OwnerProcessID;actualMouseDrag=$true;actualExplorer=$true;from=@($source.X,$source.Y);to=@($to.X,$to.Y);materializationObserved=([bool]$MaterializingFile -and (Test-Path -LiteralPath $MaterializingFile));finished=$true}|ConvertTo-Json -Compress
}finally{[SFTPDirectInput]::mouse_event(4,0,0,0,[UIntPtr]::Zero);try{$window.Quit()}catch{}}
