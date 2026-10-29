param([Parameter(Mandatory=$true)][long]$WindowHandle,[Parameter(Mandatory=$true)][uint32]$OwnerProcessID,[ValidateSet('CancelEdit','RequestClose','WaitWarning')][string]$Action='CancelEdit')
$ErrorActionPreference='Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type @'
using System;using System.Runtime.InteropServices;using System.Text;
public static class EditorCloseFixture {
 [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
 [DllImport("user32.dll")] public static extern IntPtr GetLastActivePopup(IntPtr h);
 [DllImport("user32.dll")] public static extern IntPtr GetWindow(IntPtr h,uint relation);
 [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr h,int id);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h,StringBuilder s,int n);
 [DllImport("user32.dll",SetLastError=true)] public static extern bool PostMessage(IntPtr h,uint m,IntPtr w,IntPtr l);
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h,out uint p);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h,StringBuilder s,int n);
}
'@
$handle=[IntPtr]$WindowHandle;[uint32]$process=0
[void][EditorCloseFixture]::GetWindowThreadProcessId($handle,[ref]$process)
$class=New-Object System.Text.StringBuilder(100);[void][EditorCloseFixture]::GetClassName($handle,$class,100)
if(!$WindowHandle -or $process -ne $OwnerProcessID -or $class.ToString() -ne 'NetworkToolkit.WebView2.Window' -or ![EditorCloseFixture]::IsWindowVisible($handle)){throw 'Exact native editor fixture identity mismatch'}
if($Action -eq 'WaitWarning'){
 # A rejected renderer close retains the native handshake until its bounded
 # timer shows this informational warning. Do not send another WM_CLOSE.
 $until=[DateTime]::UtcNow.AddSeconds(12);$popup=[IntPtr]::Zero
 do {
  $candidate=[EditorCloseFixture]::GetLastActivePopup($handle)
  if($candidate -ne [IntPtr]::Zero -and $candidate -ne $handle){
   [uint32]$popupProcess=0;[void][EditorCloseFixture]::GetWindowThreadProcessId($candidate,[ref]$popupProcess)
   $popupClass=New-Object System.Text.StringBuilder(100);[void][EditorCloseFixture]::GetClassName($candidate,$popupClass,100)
   $title=New-Object System.Text.StringBuilder(256);[void][EditorCloseFixture]::GetWindowText($candidate,$title,256)
   if($popupProcess -eq $OwnerProcessID -and [EditorCloseFixture]::GetWindow($candidate,4) -eq $handle -and $popupClass.ToString() -eq '#32770' -and $title.ToString() -eq 'Close not approved' -and [EditorCloseFixture]::IsWindowVisible($candidate)){$popup=$candidate;break}
  }
  Start-Sleep -Milliseconds 50
 }while([DateTime]::UtcNow -lt $until)
 if($popup -eq [IntPtr]::Zero){throw 'Exact owned close-warning dialog did not appear'}
 if([EditorCloseFixture]::GetDlgItem($popup,1) -eq [IntPtr]::Zero){throw 'Owned close warning has no expected OK control'}
 if(![EditorCloseFixture]::PostMessage($popup,0x0111,[IntPtr]1,[IntPtr]::Zero)){throw 'Could not acknowledge owned informational close warning'}
 $until=[DateTime]::UtcNow.AddSeconds(5)
 while([EditorCloseFixture]::IsWindowVisible($popup) -and [DateTime]::UtcNow -lt $until){Start-Sleep -Milliseconds 50}
 if([EditorCloseFixture]::IsWindowVisible($popup)){throw 'Owned close warning did not dismiss'}
 @{action='Acknowledge close warning';hwnd=$WindowHandle;popup=$popup.ToInt64();pid=$OwnerProcessID;class='#32770';title='Close not approved'}|ConvertTo-Json -Compress
 exit 0
}
if($Action -eq 'RequestClose'){
 if(![EditorCloseFixture]::PostMessage($handle,0x0010,[IntPtr]::Zero,[IntPtr]::Zero)){throw 'Could not post close to exact fixture HWND'}
 @{action='WM_CLOSE';hwnd=$WindowHandle;pid=$OwnerProcessID;class=$class.ToString()}|ConvertTo-Json -Compress
 exit 0
}
function Find-EditorElement($name){$until=[DateTime]::UtcNow.AddSeconds(10);$condition=New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty,$name);do{$element=[System.Windows.Automation.AutomationElement]::FromHandle($handle).FindFirst([System.Windows.Automation.TreeScope]::Descendants,$condition);if(!$element){Start-Sleep -Milliseconds 100}}while(!$element -and [DateTime]::UtcNow -lt $until);if(!$element){throw ('Fixture control missing: '+$name)};return $element}
(Find-EditorElement 'Cancel close').GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
(Find-EditorElement 'Continue editing').GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
[void](Find-EditorElement 'Native close cancelled; editing remains available')
Write-Output 'Native close held past five seconds, explicitly cancelled, and editing continued'
