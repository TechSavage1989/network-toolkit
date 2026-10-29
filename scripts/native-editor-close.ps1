param([Parameter(Mandatory=$true)][long]$WindowHandle,[Parameter(Mandatory=$true)][uint32]$OwnerProcessID,[ValidateSet('CancelEdit','RequestClose','WaitWarning')][string]$Action='CancelEdit',[switch]$Diagnostics)
$ErrorActionPreference='Stop'
function Write-FixtureStage([string]$stage){if(!$Diagnostics){return};[Console]::Out.WriteLine(('{0:o} action={1} stage={2}' -f [DateTime]::UtcNow,$Action,$stage));[Console]::Out.Flush()}
Write-FixtureStage 'load automation assemblies and native helper'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type @'
using System;using System.Runtime.InteropServices;using System.Text;
public static class EditorCloseFixture {
 [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
 [DllImport("user32.dll")] public static extern bool IsWindowEnabled(IntPtr h);
 [DllImport("user32.dll")] public static extern IntPtr GetParent(IntPtr h);
 [DllImport("user32.dll")] public static extern IntPtr GetLastActivePopup(IntPtr h);
 [DllImport("user32.dll")] public static extern IntPtr GetWindow(IntPtr h,uint relation);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h,StringBuilder s,int n);
 [DllImport("user32.dll",SetLastError=true)] public static extern bool PostMessage(IntPtr h,uint m,IntPtr w,IntPtr l);
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h,out uint p);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h,StringBuilder s,int n);
 [DllImport("user32.dll")] public static extern int GetDlgCtrlID(IntPtr h);
 delegate bool EnumChild(IntPtr h,IntPtr p);
 [DllImport("user32.dll")] static extern bool EnumChildWindows(IntPtr h,EnumChild callback,IntPtr p);
 public static IntPtr OnlyReadyButton(IntPtr dialog) {
  IntPtr found=IntPtr.Zero;int buttons=0,children=0;bool limit=false;
  EnumChildWindows(dialog,delegate(IntPtr child,IntPtr unused) {
   if(++children>20){limit=true;return false;}
   var cls=new StringBuilder(100);GetClassName(child,cls,cls.Capacity);
   if(GetParent(child)==dialog && cls.ToString()=="Button" && IsWindowVisible(child) && IsWindowEnabled(child)){found=child;buttons++;}
   return true;
  },IntPtr.Zero);
  return !limit && buttons==1 ? found : IntPtr.Zero;
 }
 public static string DescribeChildren(IntPtr dialog) {
  var result=new StringBuilder();int count=0;
  EnumChildWindows(dialog,delegate(IntPtr child,IntPtr unused) {
   var name=new StringBuilder(128);var cls=new StringBuilder(100);
   GetWindowText(child,name,name.Capacity);GetClassName(child,cls,cls.Capacity);
   result.Append(" [id=").Append(GetDlgCtrlID(child)).Append(" class=").Append(cls).Append(" visible=").Append(IsWindowVisible(child)).Append(" enabled=").Append(IsWindowEnabled(child)).Append(" text=").Append(name).Append("]");
   return ++count<20;
  },IntPtr.Zero);
  return result.ToString();
 }
}
'@
Write-FixtureStage 'validate exact HWND/PID/class/visibility'
$handle=[IntPtr]$WindowHandle;[uint32]$process=0
[void][EditorCloseFixture]::GetWindowThreadProcessId($handle,[ref]$process)
$class=New-Object System.Text.StringBuilder(100);[void][EditorCloseFixture]::GetClassName($handle,$class,100)
if(!$WindowHandle -or $process -ne $OwnerProcessID -or $class.ToString() -ne 'NetworkToolkit.WebView2.Window' -or ![EditorCloseFixture]::IsWindowVisible($handle)){throw 'Exact native editor fixture identity mismatch'}
if($Action -eq 'WaitWarning'){
 Write-FixtureStage 'wait for exact owned warning and ready OK control'
 # A rejected renderer close retains the native handshake until its bounded
 # timer shows this informational warning. Do not send another WM_CLOSE.
 function Test-OwnedWarning([IntPtr]$candidate){
  if($candidate -eq [IntPtr]::Zero -or $candidate -eq $handle){return $false}
  [uint32]$popupProcess=0;[void][EditorCloseFixture]::GetWindowThreadProcessId($candidate,[ref]$popupProcess)
  $popupClass=New-Object System.Text.StringBuilder(100);[void][EditorCloseFixture]::GetClassName($candidate,$popupClass,100)
  $title=New-Object System.Text.StringBuilder(256);[void][EditorCloseFixture]::GetWindowText($candidate,$title,256)
  return $popupProcess -eq $OwnerProcessID -and [EditorCloseFixture]::GetWindow($candidate,4) -eq $handle -and $popupClass.ToString() -eq '#32770' -and $title.ToString() -eq 'Close not approved' -and [EditorCloseFixture]::IsWindowVisible($candidate)
 }
 function Test-ReadyOK([IntPtr]$dialog,[IntPtr]$button){
  if($button -eq [IntPtr]::Zero -or [EditorCloseFixture]::GetParent($button) -ne $dialog){return $false}
  [uint32]$buttonProcess=0;[void][EditorCloseFixture]::GetWindowThreadProcessId($button,[ref]$buttonProcess)
  $buttonClass=New-Object System.Text.StringBuilder(100);[void][EditorCloseFixture]::GetClassName($button,$buttonClass,100)
  $caption=New-Object System.Text.StringBuilder(100);[void][EditorCloseFixture]::GetWindowText($button,$caption,100)
  return $buttonProcess -eq $OwnerProcessID -and $buttonClass.ToString() -eq 'Button' -and $caption.ToString() -ceq 'OK' -and [EditorCloseFixture]::IsWindowVisible($button) -and [EditorCloseFixture]::IsWindowEnabled($button)
 }
 $until=[DateTime]::UtcNow.AddSeconds(12);$popup=[IntPtr]::Zero;$ok=[IntPtr]::Zero;$lastOwned=[IntPtr]::Zero
 do {
  $candidate=[EditorCloseFixture]::GetLastActivePopup($handle)
  if(Test-OwnedWarning $candidate){
   $lastOwned=$candidate;$button=[EditorCloseFixture]::OnlyReadyButton($candidate)
   # The actual hosted warning's sole OK button uses control ID2, not IDOK1.
   # Identify its exact caption/identity instead of assuming any numeric ID.
   if(Test-ReadyOK $candidate $button){$popup=$candidate;$ok=$button;break}
  }
  Start-Sleep -Milliseconds 50
 }while([DateTime]::UtcNow -lt $until)
 if($popup -eq [IntPtr]::Zero){
  $diagnostic='no exact owned dialog';if(Test-OwnedWarning $lastOwned){$diagnostic=[EditorCloseFixture]::DescribeChildren($lastOwned)}
  throw ('Exact owned close-warning dialog and unique ready OK button did not appear: '+$diagnostic)
 }
 if(!(Test-OwnedWarning $popup) -or [EditorCloseFixture]::OnlyReadyButton($popup) -ne $ok -or !(Test-ReadyOK $popup $ok)){throw 'Owned close warning or expected OK identity changed before acknowledgment'}
 $controlID=[EditorCloseFixture]::GetDlgCtrlID($ok)
 Write-FixtureStage ('invoke owned OK control ID='+$controlID)
 if(![EditorCloseFixture]::PostMessage($ok,0x00F5,[IntPtr]::Zero,[IntPtr]::Zero)){throw 'Could not invoke the exact owned informational OK button'}
 $until=[DateTime]::UtcNow.AddSeconds(5)
 while([EditorCloseFixture]::IsWindowVisible($popup) -and [DateTime]::UtcNow -lt $until){Start-Sleep -Milliseconds 50}
 if([EditorCloseFixture]::IsWindowVisible($popup)){throw 'Owned close warning did not dismiss'}
 Write-FixtureStage 'owned warning dismissed'
 @{action='Acknowledge close warning';hwnd=$WindowHandle;popup=$popup.ToInt64();button=$ok.ToInt64();controlID=$controlID;caption='OK';pid=$OwnerProcessID;class='#32770';title='Close not approved'}|ConvertTo-Json -Compress
 exit 0
}
if($Action -eq 'RequestClose'){
 if(![EditorCloseFixture]::PostMessage($handle,0x0010,[IntPtr]::Zero,[IntPtr]::Zero)){throw 'Could not post close to exact fixture HWND'}
 @{action='WM_CLOSE';hwnd=$WindowHandle;pid=$OwnerProcessID;class=$class.ToString()}|ConvertTo-Json -Compress
 exit 0
}
function Find-EditorElement($name){Write-FixtureStage ('UIA find '+$name);$until=[DateTime]::UtcNow.AddSeconds(10);$condition=New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty,$name);do{$element=[System.Windows.Automation.AutomationElement]::FromHandle($handle).FindFirst([System.Windows.Automation.TreeScope]::Descendants,$condition);if(!$element){Start-Sleep -Milliseconds 100}}while(!$element -and [DateTime]::UtcNow -lt $until);if(!$element){throw ('Fixture control missing: '+$name)};Write-FixtureStage ('UIA found '+$name);return $element}
Write-FixtureStage 'UIA locate and invoke Cancel close'
(Find-EditorElement 'Cancel close').GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
Write-FixtureStage 'UIA locate and invoke Continue editing'
(Find-EditorElement 'Continue editing').GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
Write-FixtureStage 'UIA verify continued editing status'
[void](Find-EditorElement 'Native close cancelled; editing remains available')
Write-Output 'Native close held past five seconds, explicitly cancelled, and editing continued'
