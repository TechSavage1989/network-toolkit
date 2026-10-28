param([Parameter(Mandatory=$true)][string]$SourceDirectory,[Parameter(Mandatory=$true)][long]$WindowHandle,[Parameter(Mandatory=$true)][uint32]$OwnerProcessID)
# Hosted test only: operates on one caller-created synthetic Explorer folder and
# the caller's exact native fixture HWND. No clipboard or arbitrary UI access.
$ErrorActionPreference='Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type @'
using System;
using System.Runtime.InteropServices;
using System.Text;
public static class ExplorerDropMouse {
 [DllImport("user32.dll")] public static extern bool IsWindow(IntPtr h);
 [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h,out uint process);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h,StringBuilder text,int count);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h,StringBuilder text,int count);
 [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr h,IntPtr after,int x,int y,int w,int z,uint flags);
 [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
 [DllImport("user32.dll")] public static extern bool SetCursorPos(int x,int y);
 [DllImport("user32.dll")] public static extern void mouse_event(uint flags,uint dx,uint dy,uint data,UIntPtr extra);
}
'@
$native=[IntPtr]$WindowHandle
[uint32]$nativeProcess=0
[void][ExplorerDropMouse]::GetWindowThreadProcessId($native,[ref]$nativeProcess)
$nativeClass=New-Object System.Text.StringBuilder(256)
[void][ExplorerDropMouse]::GetClassName($native,$nativeClass,$nativeClass.Capacity)
if(!$WindowHandle -or ![ExplorerDropMouse]::IsWindow($native) -or ![ExplorerDropMouse]::IsWindowVisible($native) -or $nativeProcess -ne $OwnerProcessID -or $nativeClass.ToString() -ne 'NetworkToolkit.WebView2.Window'){throw 'Native fixture HWND identity/visibility validation failed'}
$nativeCaption=New-Object System.Text.StringBuilder(256)
[void][ExplorerDropMouse]::GetWindowText($native,$nativeCaption,$nativeCaption.Capacity)
Write-Output ('Exact ready fixture HWND '+$WindowHandle+' owner '+$nativeProcess+' caption ['+$nativeCaption.ToString()+']')
[void][ExplorerDropMouse]::SetWindowPos($native,[IntPtr]::Zero,0,0,500,720,0x0040)
$shell=New-Object -ComObject Shell.Application
$folder=(Get-Item -LiteralPath $SourceDirectory).FullName
# Compare the same Shell namespace representation on both sides: hosted Temp
# paths can contain an 8.3 alias while Explorer exposes the long pathname.
$sourceNamespace=$shell.NameSpace($folder)
if(!$sourceNamespace -or !$sourceNamespace.Self.IsFolder){throw 'Synthetic Explorer source folder could not be resolved'}
$folder=$sourceNamespace.Self.Path
Start-Process explorer.exe -ArgumentList @('/n,',('"'+$folder+'"'))
$window=$null
$until=[DateTime]::UtcNow.AddSeconds(20)
do {foreach($candidate in @($shell.Windows())){try{if($candidate.Document.Folder.Self.Path -eq $folder){$window=$candidate;break}}catch{}};if(!$window){Start-Sleep -Milliseconds 200}}while(!$window -and [DateTime]::UtcNow -lt $until)
if(!$window){throw 'Synthetic Explorer folder did not open'}
try {
 $explorer=[IntPtr]([long]$window.HWND)
 [void][ExplorerDropMouse]::SetWindowPos($explorer,[IntPtr]::Zero,520,0,500,720,0x0040)
 $window.Document.CurrentViewMode=4
 Start-Sleep -Milliseconds 500
 function Find-Element($handle,$name){
  $condition=New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty,$name)
  $until=[DateTime]::UtcNow.AddSeconds(10)
  do{$element=[System.Windows.Automation.AutomationElement]::FromHandle($handle).FindFirst([System.Windows.Automation.TreeScope]::Descendants,$condition);if(!$element){Start-Sleep -Milliseconds 100}}while(!$element -and [DateTime]::UtcNow -lt $until)
  if(!$element){throw ('UIA element absent: '+$name)};return $element
 }
 function Drag-SelectedFile {
  [void][ExplorerDropMouse]::SetForegroundWindow($explorer)
  $element=Find-Element $explorer $file.Name
  $r=$element.Current.BoundingRectangle
  if($r.Width -le 0 -or $r.Height -le 0){throw 'Explorer source item is not visible'}
  $x=[int]($r.Left+[Math]::Min(45,$r.Width/2));$y=[int]($r.Top+$r.Height/2)
  [void][ExplorerDropMouse]::SetCursorPos($x,$y)
  [ExplorerDropMouse]::mouse_event(0x0002,0,0,0,[UIntPtr]::Zero)
  try{for($i=1;$i -le 24;$i++){[void][ExplorerDropMouse]::SetCursorPos([int]($x+(250-$x)*$i/24),[int]($y+(200-$y)*$i/24));Start-Sleep -Milliseconds 30}}finally{[ExplorerDropMouse]::mouse_event(0x0004,0,0,0,[UIntPtr]::Zero)}
 }
 $file=$window.Document.Folder.ParseName('synthetic-drop107.txt')
 $dir=$window.Document.Folder.ParseName('unsupported-folder')
 if(!$file -or !$dir){throw 'Synthetic source fixture missing'}
 # Actual mixed file/folder OS drag must be rejected as a whole before upload.
 $window.Document.SelectItem($dir,5);$window.Document.SelectItem($file,1)
 Drag-SelectedFile
 [void](Find-Element $native 'Folder drop rejected - no files imported')
 # Actual Explorer file drag must reach browser File bytes and explicit review.
 $window.Document.SelectItem($file,29)
 Drag-SelectedFile
 $confirm=Find-Element $native 'Copy files'
 $invoke=$confirm.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
 $invoke.Invoke()
 [void](Find-Element $native 'Native Explorer copy verified')
 Write-Output 'Actual Explorer mixed-folder rejection and File/DataTransfer copy confirmed'
} finally { $window.Quit() }
