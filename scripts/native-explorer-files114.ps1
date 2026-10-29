param([Parameter(Mandatory=$true)][string]$SourceDirectory,[Parameter(Mandatory=$true)][long]$WindowHandle,[Parameter(Mandatory=$true)][uint32]$OwnerProcessID,[Parameter(Mandatory=$true)][string]$DestinationDirectory)
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
 [DllImport("user32.dll")] public static extern void keybd_event(byte key,byte scan,uint flags,UIntPtr extra);
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
  $until=[DateTime]::UtcNow.AddSeconds(15)
  do{$element=[System.Windows.Automation.AutomationElement]::FromHandle($handle).FindFirst([System.Windows.Automation.TreeScope]::Descendants,$condition);if(!$element){Start-Sleep -Milliseconds 100}}while(!$element -and [DateTime]::UtcNow -lt $until)
  if(!$element){throw ('UIA element absent: '+$name)};return $element
 }
 function Invoke-Button($name){$element=Find-Element $native $name;$element.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()}
 function Wait-Copy-Finished {
  # File existence/mouse release can precede the native finish POST. Wait for
  # the actual page's reconciled terminal state before invoking Close once.
  $finishedName='finished '+[char]0xB7+' 5 items '+[char]0xB7+' 56 B'
  $condition=New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty,$finishedName)
  $until=[DateTime]::UtcNow.AddSeconds(15)
  do{$finished=[System.Windows.Automation.AutomationElement]::FromHandle($native).FindFirst([System.Windows.Automation.TreeScope]::Descendants,$condition);if(!$finished){Start-Sleep -Milliseconds 100}}while(!$finished -and [DateTime]::UtcNow -lt $until)
  if(!$finished){throw 'Native drag completion did not reach the rendered finished status'}
 }
 function Close-Copy {
  Invoke-Button 'Cancel / Close'
  $condition=New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty,'Drag prepared files and folders to Windows Explorer')
  $until=[DateTime]::UtcNow.AddSeconds(15)
  do{$old=[System.Windows.Automation.AutomationElement]::FromHandle($native).FindFirst([System.Windows.Automation.TreeScope]::Descendants,$condition);if($old){Start-Sleep -Milliseconds 100}}while($old -and [DateTime]::UtcNow -lt $until)
  if($old){throw 'Copy modal did not close after its Close request'}
 }

 function Drag-Point($x,$y,$toX,$toY,$cancel){
  [void][ExplorerDropMouse]::SetCursorPos($x,$y);[ExplorerDropMouse]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
  try {
   [void][ExplorerDropMouse]::SetCursorPos($x+8,$y+2);Start-Sleep -Milliseconds 450
   for($i=1;$i -le 24;$i++){[void][ExplorerDropMouse]::SetCursorPos([int]($x+($toX-$x)*$i/24),[int]($y+($toY-$y)*$i/24));Start-Sleep -Milliseconds 30}
   if($cancel){[ExplorerDropMouse]::keybd_event(27,0,0,[UIntPtr]::Zero);Start-Sleep -Milliseconds 100;[ExplorerDropMouse]::keybd_event(27,0,2,[UIntPtr]::Zero);Start-Sleep -Milliseconds 150}
  } finally {[ExplorerDropMouse]::mouse_event(4,0,0,0,[UIntPtr]::Zero)}
 }
 # Real OS folder -> WebView FileSystemEntry enumeration, then explicit copy.
 $file=$window.Document.Folder.ParseName('incoming-folder');if(!$file){throw 'Synthetic incoming tree missing'}
 $window.Document.SelectItem($file,29);[void][ExplorerDropMouse]::SetForegroundWindow($explorer)
 $r=(Find-Element $explorer $file.Name).Current.BoundingRectangle
 Drag-Point ([int]($r.Left+30)) ([int]($r.Top+$r.Height/2)) 250 170 $false
 Invoke-Button 'Copy files'
 [void](Find-Element $native 'Incoming folder bytes verified')
 Invoke-Button 'Close'
 # A real NTFS junction must be rejected or omitted by WebView, never traversed.
 $linkTree=$window.Document.Folder.ParseName('junction-tree');if(!$linkTree){throw 'Synthetic junction tree absent'}
 $window.Document.SelectItem($linkTree,29);[void][ExplorerDropMouse]::SetForegroundWindow($explorer)
 $r=(Find-Element $explorer $linkTree.Name).Current.BoundingRectangle
 Drag-Point ([int]($r.Left+30)) ([int]($r.Top+$r.Height/2)) 250 170 $false
 [void](Find-Element $native 'Link target not traversed')
 # Navigate only this owned synthetic Explorer window to its destination.
 $dest=$shell.NameSpace((Get-Item -LiteralPath $DestinationDirectory).FullName).Self.Path
 $window.Navigate2($dest);$until=[DateTime]::UtcNow.AddSeconds(15)
 do{Start-Sleep -Milliseconds 100}while($window.Document.Folder.Self.Path -ne $dest -and [DateTime]::UtcNow -lt $until)
 if($window.Document.Folder.Self.Path -ne $dest){throw 'Explorer destination did not load'}
 [void][ExplorerDropMouse]::SetForegroundWindow($native);Invoke-Button 'Prepare Explorer copy'
 $until=[DateTime]::UtcNow.AddSeconds(15)
 do{$button=Find-Element $native 'Drag prepared files and folders to Windows Explorer';if(!$button.Current.IsEnabled){Start-Sleep -Milliseconds 100}}while(!$button.Current.IsEnabled -and [DateTime]::UtcNow -lt $until)
 if(!$button.Current.IsEnabled){throw 'Prepared-copy handle never enabled'}
 # Quick press/release must not arm a later ordinary click.
 $r=$button.Current.BoundingRectangle;$x=[int]($r.Left+$r.Width/2);$y=[int]($r.Top+$r.Height/2)
 [void][ExplorerDropMouse]::SetCursorPos($x,$y);[ExplorerDropMouse]::mouse_event(2,0,0,0,[UIntPtr]::Zero);[void][ExplorerDropMouse]::SetCursorPos($x+8,$y+2);[ExplorerDropMouse]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
 Start-Sleep -Milliseconds 300
 Invoke-Button 'Record quick release';[void](Find-Element $native 'Quick baseline recorded')
 [void][ExplorerDropMouse]::SetCursorPos(20,680);[ExplorerDropMouse]::mouse_event(2,0,0,0,[UIntPtr]::Zero);Start-Sleep -Milliseconds 250;[ExplorerDropMouse]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
 Start-Sleep -Milliseconds 300
 Invoke-Button 'Check quick release';[void](Find-Element $native 'Quick release did not arm ordinary click')
 Close-Copy;Invoke-Button 'Prepare Explorer copy'
 $until=[DateTime]::UtcNow.AddSeconds(15)
 do{$button=Find-Element $native 'Drag prepared files and folders to Windows Explorer';if(!$button.Current.IsEnabled){Start-Sleep -Milliseconds 100}}while(!$button.Current.IsEnabled -and [DateTime]::UtcNow -lt $until)
 if(!$button.Current.IsEnabled){throw 'Copy re-preparation after quick gesture timed out'}
 # Resolve UIA before OLE owns the mouse; no UIA calls during drag.
 $r=$button.Current.BoundingRectangle
 Drag-Point ([int]($r.Left+$r.Width/2)) ([int]($r.Top+$r.Height/2)) 760 460 $false
 $until=[DateTime]::UtcNow.AddSeconds(20)
 do{Start-Sleep -Milliseconds 100}while(!(Test-Path -LiteralPath (Join-Path $DestinationDirectory 'outgoing-folder\nested\out.txt')) -and [DateTime]::UtcNow -lt $until)
 if(!(Test-Path -LiteralPath (Join-Path $DestinationDirectory 'outgoing-file.txt'))){throw 'Real file export missing'}
 if(!(Test-Path -LiteralPath (Join-Path $DestinationDirectory 'outgoing-folder\nested\out.txt'))){throw 'Real folder export missing'}
 Wait-Copy-Finished
 Close-Copy
 [void][ExplorerDropMouse]::SetForegroundWindow($native);Invoke-Button 'Prepare Explorer copy'
 $until=[DateTime]::UtcNow.AddSeconds(15)
 do{$button=Find-Element $native 'Drag prepared files and folders to Windows Explorer';if(!$button.Current.IsEnabled){Start-Sleep -Milliseconds 100}}while(!$button.Current.IsEnabled -and [DateTime]::UtcNow -lt $until)
 if(!$button.Current.IsEnabled){throw 'Second copy preparation timed out'}
 $r=$button.Current.BoundingRectangle
 Drag-Point ([int]($r.Left+$r.Width/2)) ([int]($r.Top+$r.Height/2)) 750 460 $true
 Wait-Copy-Finished
 Close-Copy
 # One final real held drag is cancelled by its owning native context, not input injection.
 Invoke-Button 'Prepare Explorer copy'
 $until=[DateTime]::UtcNow.AddSeconds(15)
 do{$button=Find-Element $native 'Drag prepared files and folders to Windows Explorer';if(!$button.Current.IsEnabled){Start-Sleep -Milliseconds 100}}while(!$button.Current.IsEnabled -and [DateTime]::UtcNow -lt $until)
 if(!$button.Current.IsEnabled){throw 'Owner-cancellation preparation timed out'}
 $r=$button.Current.BoundingRectangle
 Invoke-Button 'Cancel owner during next drag';[void](Find-Element $native 'Owner cancellation armed')
 [void][ExplorerDropMouse]::SetCursorPos([int]($r.Left+$r.Width/2),[int]($r.Top+$r.Height/2));[ExplorerDropMouse]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
 try{[void][ExplorerDropMouse]::SetCursorPos([int]($r.Left+$r.Width/2+8),[int]($r.Top+$r.Height/2+2));Start-Sleep -Milliseconds 3500}finally{[ExplorerDropMouse]::mouse_event(4,0,0,0,[UIntPtr]::Zero)}
 Write-Output 'Actual Windows Explorer folder import, file+folder export, Escape and owner-context cancellation completed'
} finally {$window.Quit()}
