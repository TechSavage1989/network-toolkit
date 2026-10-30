param(
 [Parameter(Mandatory=$true)][uint32]$OwnerProcessID,
 [Parameter(Mandatory=$true)][string]$ExecutablePath,
 [string]$DestinationDirectory,
 [string]$FinishedFile,
 [string]$MaterializingFile='', [string]$ExpectedPath='',
 [double]$FromX,[double]$FromY,
 [double]$FromWidth,[double]$FromHeight,
 [ValidateSet('Arrange','Copy','CancelBefore','CancelDuring','BaselineSaveDialog')][string]$Mode='Copy',
 [ValidateSet('Fast','Normal','Slow')][string]$Speed='Normal'
)
# Terminal123 bounded extension of the proven native-sftp-direct121 input path.
# Negative control alone dwells to exercise the original real hover DownloadURL.
# Hosted synthetic acceptance only. Input goes to exact supplied fixture PIDs and
# executable identities. No clipboard, synthetic DOM drag, debug port, security
# setting, unrelated window activation, or full-desktop screenshot is used.
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName WindowsBase
Add-Type @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
public static class SFTPDirectInput {
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h,StringBuilder text,int length);
 [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr h,uint message,IntPtr w,IntPtr l);
 public static IntPtr[] Dialogs(uint pid){var found=new List<IntPtr>();EnumWindows((h,p)=>{uint owner;GetWindowThreadProcessId(h,out owner);var s=new StringBuilder(256);GetClassName(h,s,256);if(owner==pid&&IsWindowVisible(h)&&s.ToString()=="#32770")found.Add(h);return true;},IntPtr.Zero);return found.ToArray();}
 public static string Title(IntPtr h){var text=new StringBuilder(512);GetWindowText(h,text,512);return text.ToString();}
 public delegate bool EnumWindow(IntPtr h,IntPtr p);
 [DllImport("kernel32.dll")] public static extern ulong GetTickCount64();
 [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindow callback,IntPtr p);
 [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
 [DllImport("user32.dll")] public static extern bool IsZoomed(IntPtr h);
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h,out uint p);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h,StringBuilder s,int n);
 [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h,out RECT r);
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
 [DllImport("kernel32.dll",SetLastError=true)] public static extern IntPtr OpenProcess(uint access,bool inherit,uint pid);
 [DllImport("kernel32.dll")] public static extern bool CloseHandle(IntPtr h);
 [DllImport("advapi32.dll",SetLastError=true)] public static extern bool OpenProcessToken(IntPtr process,uint access,out IntPtr token);
 [DllImport("advapi32.dll",SetLastError=true)] public static extern bool GetTokenInformation(IntPtr token,int type,IntPtr data,int size,out int need);
 [DllImport("advapi32.dll")] public static extern IntPtr GetSidSubAuthorityCount(IntPtr sid);
 [DllImport("advapi32.dll")] public static extern IntPtr GetSidSubAuthority(IntPtr sid,uint n);
 public static int[] TokenFacts(uint pid){var facts=new int[]{-1,-1,0};IntPtr p=OpenProcess(0x1000,false,pid),t=IntPtr.Zero,b=IntPtr.Zero;try{if(p==IntPtr.Zero||!OpenProcessToken(p,8,out t)){facts[2]=Marshal.GetLastWin32Error();return facts;}int n;b=Marshal.AllocHGlobal(4);if(GetTokenInformation(t,20,b,4,out n))facts[0]=Marshal.ReadInt32(b);Marshal.FreeHGlobal(b);b=IntPtr.Zero;GetTokenInformation(t,25,IntPtr.Zero,0,out n);if(n>0&&n<65536){b=Marshal.AllocHGlobal(n);if(GetTokenInformation(t,25,b,n,out n)){var sid=Marshal.ReadIntPtr(b);byte count=Marshal.ReadByte(GetSidSubAuthorityCount(sid));if(count>0)facts[1]=Marshal.ReadInt32(GetSidSubAuthority(sid,(uint)(count-1)));}}return facts;}finally{if(b!=IntPtr.Zero)Marshal.FreeHGlobal(b);if(t!=IntPtr.Zero)CloseHandle(t);if(p!=IntPtr.Zero)CloseHandle(p);}}
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
$timeline=New-Object System.Collections.Generic.List[object]
function Mark-Input([string]$kind){$timeline.Add(@{kind=$kind;tickMS=[SFTPDirectInput]::GetTickCount64()})}
$timing=@{press=150;threshold=200;steps=28;step=25;dwell=500}
if($Speed -eq 'Fast'){$timing=@{press=60;threshold=40;steps=12;step=12;dwell=40}}
if($Speed -eq 'Slow'){$timing=@{press=250;threshold=300;steps=40;step=30;dwell=750}}
$inputEvidence=$null
$saveDialogs=New-Object System.Collections.Generic.List[object]
function Check-OwnedDialogs {
 # Cancellation is HWND-targeted and only for the exact supplied fixture process,
 # executable identity, native common-dialog class and exact application title.
 foreach($dialog in @([SFTPDirectInput]::Dialogs($OwnerProcessID))){
  $null=Fixture-Window $OwnerProcessID $ExecutablePath
  $title=[SFTPDirectInput]::Title($dialog)
  if($title -ne 'Save Network Toolkit export'){throw ('Unexpected fixture-owned dialog: '+$title)}
  $saveDialogs.Add(@{title=$title;class='#32770';sourcePID=$OwnerProcessID;tickMS=[SFTPDirectInput]::GetTickCount64();cancel='HWND WM_COMMAND IDCANCEL'})
  if(![SFTPDirectInput]::PostMessage($dialog,0x111,[IntPtr]2,[IntPtr]::Zero)){throw 'Could not safely cancel exact fixture-owned Save dialog'}
  $until=[DateTime]::UtcNow.AddSeconds(3)
  while([SFTPDirectInput]::IsWindowVisible($dialog) -and [DateTime]::UtcNow -lt $until){Start-Sleep -Milliseconds 20}
  if([SFTPDirectInput]::IsWindowVisible($dialog)){throw 'Fixture-owned Save dialog did not close'}
  if($Mode -ne 'BaselineSaveDialog'){throw 'Unexpected native Save/export dialog during ordinary Terminal SFTP copy'}
 }
}
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
 # Resolve the actual Explorer Items View before OLE; the destination must be
 # its content area, outside the source client, not merely any child HWND.
 $condition=New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ClassNameProperty,'UIItemsView')
 $items=$null;$until=[DateTime]::UtcNow.AddSeconds(15)
 do{$items=[System.Windows.Automation.AutomationElement]::FromHandle($explorer).FindFirst([System.Windows.Automation.TreeScope]::Descendants,$condition);if(!$items){Start-Sleep -Milliseconds 100}}while(!$items -and [DateTime]::UtcNow -lt $until)
 if(!$items){throw 'Actual Explorer Items View is absent'}
 $bounds=$items.Current.BoundingRectangle;$ownerBounds=New-Object SFTPDirectInput+RECT
 if(![SFTPDirectInput]::GetWindowRect($owner,[ref]$ownerBounds)){throw 'Cannot locate source owner bounds'}
 $left=[Math]::Max($bounds.Left+20,$ownerBounds.Right+20);$right=$bounds.Right-20
 if($right-$left -lt 12 -or $bounds.Height -lt 50){throw 'Explorer file content has no exposed drop area outside source'}
 $to=New-Object SFTPDirectInput+POINT;$to.X=[int](($left+$right)/2);$to.Y=[int]($bounds.Top+$bounds.Height*.55)
 [void][SFTPDirectInput]::SetForegroundWindow($owner);Start-Sleep -Milliseconds 100;Visible-Point $owner $source;Visible-Point $explorer $to
 $hit=[System.Windows.Automation.AutomationElement]::FromPoint((New-Object System.Windows.Point($to.X,$to.Y)))
 $targetVerified=$false
 for($depth=0;$hit -and $depth -lt 12;$depth++){if($hit.Current.ClassName -eq 'UIItemsView'){$targetVerified=$true;break};$hit=[System.Windows.Automation.TreeWalker]::ControlViewWalker.GetParent($hit)}
 if(!$targetVerified){throw 'Drop point is not in actual Explorer Items View'}
 [uint32]$explorerPID=0;[void][SFTPDirectInput]::GetWindowThreadProcessId($explorer,[ref]$explorerPID)
 $inputEvidence=@{sourceToken=[SFTPDirectInput]::TokenFacts($OwnerProcessID);targetToken=[SFTPDirectInput]::TokenFacts($explorerPID);tokenFactFields=@('elevated','integrityRID','error');source=@($source.X,$source.Y);target=@($to.X,$to.Y);targetClass='UIItemsView';targetBounds=@($bounds.Left,$bounds.Top,$bounds.Width,$bounds.Height);targetVerified=$true;mode=$Mode;speed=$Speed;timing=$timing;clock='Windows GetTickCount64 milliseconds since boot'}
 $inputEvidence|ConvertTo-Json -Depth 8 -Compress|Set-Content -LiteralPath ($FinishedFile+'.input.json') -Encoding UTF8
 [void][SFTPDirectInput]::SetCursorPos($source.X,$source.Y);if($Mode -eq 'BaselineSaveDialog'){Mark-Input 'baseline-real-hover';Start-Sleep -Milliseconds 1500};Mark-Input 'press-before';[SFTPDirectInput]::mouse_event(2,0,0,0,[UIntPtr]::Zero);Mark-Input 'press-after'
 try{
  Start-Sleep -Milliseconds $timing.press
  Mark-Input 'threshold-before';[void][SFTPDirectInput]::SetCursorPos($source.X+10,$source.Y+2);Mark-Input 'threshold-after';Start-Sleep -Milliseconds $timing.threshold
  Mark-Input 'travel-start'
  for($i=1;$i -le $timing.steps;$i++){[void][SFTPDirectInput]::SetCursorPos([int]($source.X+($to.X-$source.X)*$i/$timing.steps),[int]($source.Y+($to.Y-$source.Y)*$i/$timing.steps));Mark-Input ('move-'+$i);Start-Sleep -Milliseconds $timing.step;Check-OwnedDialogs}
  Mark-Input 'target-arrival';Start-Sleep -Milliseconds $timing.dwell
  if($Mode -eq 'CancelBefore'){Mark-Input 'escape-before';[SFTPDirectInput]::keybd_event(27,0,0,[UIntPtr]::Zero);Start-Sleep -Milliseconds 150;[SFTPDirectInput]::keybd_event(27,0,2,[UIntPtr]::Zero)}
 }finally{Mark-Input 'release-before';[SFTPDirectInput]::mouse_event(4,0,0,0,[UIntPtr]::Zero);Mark-Input 'release-after'}
 Check-OwnedDialogs
 if($Mode -eq 'BaselineSaveDialog'){
  $until=[DateTime]::UtcNow.AddSeconds(15);while($saveDialogs.Count -eq 0 -and [DateTime]::UtcNow -lt $until){Check-OwnedDialogs;Start-Sleep -Milliseconds 50}
  if($saveDialogs.Count -ne 1){throw 'Original122 negative control did not reproduce exactly one native Save dialog'}
  if(@(Get-ChildItem -LiteralPath $DestinationDirectory -Force).Count -ne 0){throw 'Negative control unexpectedly wrote Explorer destination'}
  @{mode=$Mode;sourcePID=$OwnerProcessID;actualMouseDrag=$true;actualExplorer=$true;saveDialogs=$saveDialogs.ToArray();saveSafelyCancelled=$true;expectedFailureReproduced=$true;finished=$false}|ConvertTo-Json -Depth 8 -Compress
  exit
 }
 if($Mode -eq 'CancelDuring'){
  $until=[DateTime]::UtcNow.AddSeconds(15);while(!(Test-Path -LiteralPath $MaterializingFile) -and [DateTime]::UtcNow -lt $until){Check-OwnedDialogs;Start-Sleep -Milliseconds 20}
  if(!(Test-Path -LiteralPath $MaterializingFile)){throw 'Actual loopback bulk response never began materializing'}
  Mark-Input 'escape-during';[SFTPDirectInput]::keybd_event(27,0,0,[UIntPtr]::Zero);Start-Sleep -Milliseconds 200;[SFTPDirectInput]::keybd_event(27,0,2,[UIntPtr]::Zero)
 }
 $until=[DateTime]::UtcNow.AddSeconds(25);while(!(Test-Path -LiteralPath $FinishedFile) -and [DateTime]::UtcNow -lt $until){Check-OwnedDialogs;Start-Sleep -Milliseconds 50}
 if(!(Test-Path -LiteralPath $FinishedFile)){throw 'Native copy gesture never sent its authenticated terminal finish'}
 Mark-Input 'finish-marker-observed'
 if($Mode -eq 'Copy'){
  $until=[DateTime]::UtcNow.AddSeconds(15);while(!(Test-Path -LiteralPath $ExpectedPath) -and [DateTime]::UtcNow -lt $until){Check-OwnedDialogs;Start-Sleep -Milliseconds 50}
  if(!(Test-Path -LiteralPath $ExpectedPath)){throw 'Actual Explorer destination file did not arrive'}
 }else{if(@(Get-ChildItem -LiteralPath $DestinationDirectory -Force).Count -ne 0){throw 'Cancelled gesture published files into destination'}}
 Check-OwnedDialogs
 @{mode=$Mode;sourcePID=$OwnerProcessID;actualMouseDrag=$true;actualExplorer=$true;saveDialogCount=$saveDialogs.Count;fixtureHostWindowCount=@([SFTPDirectInput]::Find($OwnerProcessID)).Count;from=@($source.X,$source.Y);to=@($to.X,$to.Y);materializationObserved=([bool]$MaterializingFile -and (Test-Path -LiteralPath $MaterializingFile));finished=$true}|ConvertTo-Json -Compress
}finally{Mark-Input 'script-finally';[SFTPDirectInput]::mouse_event(4,0,0,0,[UIntPtr]::Zero);if($inputEvidence){$inputEvidence.timeline=$timeline.ToArray();$inputEvidence.saveDialogs=$saveDialogs.ToArray();$inputEvidence|ConvertTo-Json -Depth 8 -Compress|Set-Content -LiteralPath ($FinishedFile+'.input.json') -Encoding UTF8};try{$window.Quit()}catch{}}
