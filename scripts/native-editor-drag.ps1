param(
 [Parameter(Mandatory=$true)][ValidateSet('Arrange','Drag','Maximize','Restore')][string]$Action,
 [Parameter(Mandatory=$true)][uint32]$OwnerProcessID,
 [Parameter(Mandatory=$true)][string]$ExecutablePath,
 [uint32]$TargetProcessID=0,[string]$TargetExecutablePath='',
 [double]$FromX=0,[double]$FromY=0,[double]$FromWidth=0,[double]$FromHeight=0,
 [double]$ToX=0,[double]$ToY=0,[double]$ToWidth=0,[double]$ToHeight=0,
 [ValidateSet('Tab','Detach','Cross','Escape','Outside')][string]$Mode='Tab'
)
# Hosted synthetic acceptance only. Input goes to exact supplied fixture PIDs and
# executable identities. No clipboard, synthetic DOM drag, debug port, security
# setting, unrelated window activation, or full-desktop screenshot is used.
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
public static class EditorDragInput {
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
 $found=@([EditorDragInput]::Find($processID));if($found.Count -ne 1){throw 'Expected one visible host owned by exact fixture PID'};return $found[0]
}
function Client-Point($handle,[double]$x,[double]$y,[double]$width,[double]$height){
 $r=New-Object EditorDragInput+RECT
 if(![EditorDragInput]::GetClientRect($handle,[ref]$r) -or $width -le 0 -or $height -le 0 -or $x -lt 0 -or $y -lt 0 -or $x -ge $width -or $y -ge $height){throw 'Invalid fixture viewport point'}
 $p=New-Object EditorDragInput+POINT;$p.X=[int]($x*($r.Right-$r.Left)/$width);$p.Y=[int]($y*($r.Bottom-$r.Top)/$height)
 if(![EditorDragInput]::ClientToScreen($handle,[ref]$p)){throw 'Cannot locate fixture client point'};return $p
}
function Visible-Point($handle,$point){if([EditorDragInput]::GetAncestor([EditorDragInput]::WindowFromPoint($point),2) -ne $handle){throw 'Expected fixture point is covered by another HWND'}}
$owner=Fixture-Window $OwnerProcessID $ExecutablePath
$target=$owner;if($TargetProcessID){$target=Fixture-Window $TargetProcessID $TargetExecutablePath}
$work=[Windows.Forms.Screen]::PrimaryScreen.WorkingArea
if($Action -eq 'Arrange'){
 $width=[Math]::Min(820,[Math]::Max(720,[int]($work.Width/2)));$height=[Math]::Min(720,$work.Height-36)
 [void][EditorDragInput]::ShowWindow($owner,9)
 [void][EditorDragInput]::SetWindowPos($owner,[IntPtr]::Zero,$work.Left,$work.Top,$width,$height,0x0040)
 if($target -ne $owner){[void][EditorDragInput]::ShowWindow($target,9);[void][EditorDragInput]::SetWindowPos($target,[IntPtr]::Zero,$work.Right-$width,$work.Top,$width,$height,0x0040)}
 Start-Sleep -Milliseconds 350
 @{action=$Action;sourcePID=$OwnerProcessID;targetPID=$TargetProcessID;width=$width;height=$height;actualWindowInput=$true}|ConvertTo-Json -Compress;exit
}
if($Action -eq 'Maximize' -or $Action -eq 'Restore'){
 $show=3;if($Action -eq 'Restore'){$show=9}
 [void][EditorDragInput]::ShowWindow($owner,$show);[void][EditorDragInput]::SetForegroundWindow($owner);Start-Sleep -Milliseconds 500
 $maximized=[EditorDragInput]::IsZoomed($owner)
 if(($Action -eq 'Maximize') -ne $maximized){throw 'Native maximize/restore state did not change'}
 @{action=$Action;sourcePID=$OwnerProcessID;maximized=$maximized;actualWindowInput=$true}|ConvertTo-Json -Compress;exit
}
$from=Client-Point $owner $FromX $FromY $FromWidth $FromHeight
$to=$null
if($Mode -eq 'Outside'){$to=New-Object EditorDragInput+POINT;$to.X=$work.Right-4;$to.Y=$work.Bottom-4;if([EditorDragInput]::GetAncestor([EditorDragInput]::WindowFromPoint($to),2) -eq $owner){throw 'No safe outside-fixture cancellation point'}}
elseif($Mode -ne 'Detach'){$to=Client-Point $target $ToX $ToY $ToWidth $ToHeight}
[void][EditorDragInput]::SetForegroundWindow($owner);Start-Sleep -Milliseconds 100;Visible-Point $owner $from
[void][EditorDragInput]::SetCursorPos($from.X,$from.Y)
[EditorDragInput]::mouse_event(0x0002,0,0,0,[UIntPtr]::Zero)
try{
 # Pointerdown captures the UID locally; actual dragstart begins registration.
 # This is real OS input, not a fast-drop timing claim; deferred unit tests
 # separately exercise registration ordering and cancellation tombstones.
 Start-Sleep -Milliseconds 350
 [void][EditorDragInput]::SetCursorPos($from.X+12,$from.Y+2);Start-Sleep -Milliseconds 150
 if($Mode -eq 'Detach'){
  $condition=New-Object Windows.Automation.PropertyCondition([Windows.Automation.AutomationElement]::NameProperty,'Drop tab here to open a new editor window')
  $until=[DateTime]::UtcNow.AddSeconds(5);$element=$null
  do{$element=[Windows.Automation.AutomationElement]::FromHandle($owner).FindFirst([Windows.Automation.TreeScope]::Descendants,$condition);if(!$element){Start-Sleep -Milliseconds 100}}while(!$element -and [DateTime]::UtcNow -lt $until)
  if(!$element){throw 'Actual tab drag did not show the explicit detach target'}
  $r=$element.Current.BoundingRectangle;if($r.Width -le 0 -or $r.Height -le 0){throw 'Detach target is not visible'}
  $to=New-Object EditorDragInput+POINT;$to.X=[int]($r.Left+$r.Width/2);$to.Y=[int]($r.Top+$r.Height/2)
 }
 if($Mode -eq 'Cross'){Visible-Point $target $to}
 for($i=1;$i -le 28;$i++){[void][EditorDragInput]::SetCursorPos([int]($from.X+($to.X-$from.X)*$i/28),[int]($from.Y+($to.Y-$from.Y)*$i/28));Start-Sleep -Milliseconds 25}
 if($Mode -eq 'Escape'){[EditorDragInput]::keybd_event(27,0,0,[UIntPtr]::Zero);[EditorDragInput]::keybd_event(27,0,2,[UIntPtr]::Zero);Start-Sleep -Milliseconds 100}
}finally{[EditorDragInput]::mouse_event(0x0004,0,0,0,[UIntPtr]::Zero)}
Start-Sleep -Milliseconds 150
@{action=$Action;mode=$Mode;sourcePID=$OwnerProcessID;targetPID=$TargetProcessID;actualMouseDrag=$true;from=@($from.X,$from.Y);to=@($to.X,$to.Y)}|ConvertTo-Json -Compress
