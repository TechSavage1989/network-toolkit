[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$ArtifactDirectory
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# This test deliberately installs, repairs, and uninstalls into the default user profile.
if (-not $IsWindows -or $env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted') {
    throw 'Run the Windows setup smoke test only on a disposable GitHub-hosted Windows runner.'
}

function Assert-Same($Actual, $Expected, [string]$What) {
    if ($Actual -cne $Expected) {
        throw "$What did not match the expected value. Expected '$Expected', got '$Actual'."
    }
}

function Assert-Absent([string]$Path) {
    if (Test-Path -LiteralPath $Path) {
        throw "Unexpected path exists: $Path"
    }
}

function Read-RegistryValue([string]$Path, [string]$Name) {
    if (Test-Path -LiteralPath $Path) {
        $key = Get-Item -LiteralPath $Path
        try { $key.GetValue($Name) } finally { $key.Dispose() }
    }
}

function Get-UserChoice([string]$Extension) {
    $path = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts\$Extension\UserChoice"
    [pscustomobject]@{
        Exists = Test-Path -LiteralPath $path
        ProgID = Read-RegistryValue $path 'ProgId'
        Hash = Read-RegistryValue $path 'Hash'
    } | ConvertTo-Json -Compress
}

function Get-UrlUserChoice([string]$Scheme) {
    $path = "HKCU:\Software\Microsoft\Windows\Shell\Associations\UrlAssociations\$Scheme\UserChoice"
    [pscustomobject]@{
        Exists = Test-Path -LiteralPath $path
        ProgID = Read-RegistryValue $path 'ProgId'
        Hash = Read-RegistryValue $path 'Hash'
    } | ConvertTo-Json -Compress
}

# A canonical text form of a registry key: sorted values with their kinds and
# data, then each subkey. Used to prove another app's handler is untouched.
function Get-RegistryTree([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return '<absent>' }
    $key = Get-Item -LiteralPath $Path
    try {
        $lines = @(foreach ($name in ($key.GetValueNames() | Sort-Object)) {
            '{0}={1}:{2}' -f $name, $key.GetValueKind($name), $key.GetValue($name, $null, 'DoNotExpandEnvironmentNames')
        })
        $children = @($key.GetSubKeyNames() | Sort-Object)
    } finally { $key.Dispose() }
    foreach ($child in $children) { $lines += "[$child]{" + (Get-RegistryTree (Join-Path $Path $child)) + '}' }
    $lines -join "`n"
}

# The same canonical form for a key of the current user, read through .NET.
# The registry provider treats a key named "*" (all files) as a wildcard.
function Get-UserKeyTree([string]$Subkey) {
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey($Subkey)
    if ($null -eq $key) { return '<absent>' }
    try {
        $lines = @(foreach ($name in ($key.GetValueNames() | Sort-Object)) {
            '{0}={1}:{2}' -f $name, $key.GetValueKind($name), $key.GetValue($name, $null, 'DoNotExpandEnvironmentNames')
        })
        $children = @($key.GetSubKeyNames() | Sort-Object)
    } finally { $key.Dispose() }
    foreach ($child in $children) { $lines += "[$child]{" + (Get-UserKeyTree "$Subkey\$child") + '}' }
    $lines -join "`n"
}

# A value of a key of the current user, or $null when the key or value is absent.
function Read-UserValue([string]$Subkey, [string]$Name) {
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey($Subkey)
    if ($null -eq $key) { return $null }
    try { $key.GetValue($Name, $null, 'DoNotExpandEnvironmentNames') } finally { $key.Dispose() }
}

function Set-UserValue([string]$Subkey, [string]$Name, [string]$Value) {
    $key = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($Subkey)
    try { $key.SetValue($Name, $Value) } finally { $key.Dispose() }
}

# The icon groups of a program file as the shell reads a '"file.exe",N' icon
# location: their number, and that each one loads at large and small size.
Add-Type -Namespace NetworkToolkitSetupSmoke -Name IconResources -MemberDefinition @'
[DllImport("shell32.dll", CharSet = CharSet.Unicode)]
public static extern uint ExtractIconExW(string file, int index, IntPtr[] large, IntPtr[] small, uint count);
[DllImport("user32.dll")]
public static extern bool DestroyIcon(IntPtr icon);
'@
function Assert-IconGroups([string]$Path, [int]$Expected) {
    # Index -1 with no arrays asks for the number of icons in the file.
    Assert-Same ([int][NetworkToolkitSetupSmoke.IconResources]::ExtractIconExW($Path, -1, $null, $null, 0)) $Expected 'Number of icon groups in the program file'
    foreach ($index in 0..($Expected - 1)) {
        $large = [IntPtr[]]::new(1)
        $small = [IntPtr[]]::new(1)
        [void][NetworkToolkitSetupSmoke.IconResources]::ExtractIconExW($Path, $index, $large, $small, 1)
        try {
            if ($large[0] -eq [IntPtr]::Zero -or $small[0] -eq [IntPtr]::Zero) { throw "Icon $index of $Path could not be loaded." }
        } finally {
            foreach ($icon in @($large[0], $small[0])) {
                if ($icon -ne [IntPtr]::Zero) { [void][NetworkToolkitSetupSmoke.IconResources]::DestroyIcon($icon) }
            }
        }
    }
}

function Get-ExecutableVersion([string]$Path) {
    $info = [Diagnostics.FileVersionInfo]::GetVersionInfo($Path)
    [version]::new($info.FileMajorPart, $info.FileMinorPart, $info.FileBuildPart, $info.FilePrivatePart)
}

# Silent setup and uninstall show no message; they leave the reason they
# stopped in this log (replaced on every run, absent after a successful one).
function Get-SetupLog {
    $log = Join-Path ([IO.Path]::GetTempPath()) 'Network_Toolkit_Setup.log'
    if (-not (Test-Path -LiteralPath $log -PathType Leaf)) { return '' }
    return ([IO.File]::ReadAllText($log)).Trim()
}

# $ExpectedReason is a regular expression the logged reason must match.
function Invoke-Silent([string]$Path, [int]$ExpectedExitCode = 0, [string]$Arguments = '/S', [string]$ExpectedReason = '') {
    $process = Start-Process -FilePath $Path -ArgumentList $Arguments -Wait -PassThru
    try {
        $reason = Get-SetupLog
        $run = "$(Split-Path -Leaf $Path) $Arguments"
        if ($process.ExitCode -cne $ExpectedExitCode) {
            throw "Exit code from $run was $($process.ExitCode), expected $ExpectedExitCode. Reason given: $(if ($reason) { $reason } else { 'none' })"
        }
        if ($ExpectedExitCode -eq 0 -and $reason) { throw "$run succeeded but left a reason: $reason" }
        if ($ExpectedReason -and $reason -notmatch $ExpectedReason) {
            throw "$run stopped for an unexpected reason. Expected one matching '$ExpectedReason', got: $(if ($reason) { $reason } else { 'none' })"
        }
    } finally { $process.Dispose() }
}

# An NSIS uninstaller copies itself to a temporary folder, starts that copy and
# exits at once; the copy removes the files and registry entries. Wait for the
# copy (a child of the launched uninstaller) before checking what it removed.
function Invoke-Uninstaller([string]$Path) {
    $process = Start-Process -FilePath $Path -ArgumentList '/S' -PassThru
    try {
        $id = $process.Id
        $deadline = [DateTime]::UtcNow.AddMinutes(2)
        do {
            # Read the launcher's state before listing its children: once it
            # has exited, the copy it started already exists, so the listing
            # cannot miss it. The other order can see no copy yet and then an
            # exited launcher, and report "finished" while the copy still runs.
            $exited = $process.HasExited
            $children = @(Get-CimInstance Win32_Process -Filter "ParentProcessId=$id" -ErrorAction Stop)
            $running = -not $exited -or $children.Count -gt 0
            if ($running) { Start-Sleep -Milliseconds 200 }
        } while ($running -and [DateTime]::UtcNow -lt $deadline)
        if ($running) { throw "$(Split-Path -Leaf $Path) did not finish within two minutes." }
        Assert-Same $process.ExitCode 0 "Exit code from $(Split-Path -Leaf $Path)"
    } finally { $process.Dispose() }
}

# When uninstall leaves the program behind, say why: an uninstaller that
# refused (it keeps its own key), an uninstaller copy that is still working, a
# process still using a file in the folder, or a delete that Windows finishes
# only when another program closes the file. The registration is read first,
# before the slower process listings, so it describes the same moment as the
# file check. The check fails in every case; the wait only names the cause.
function Assert-UninstallFinished {
    if (-not (Test-Path -LiteralPath $installedExe)) { return }
    $refused = Test-Path -LiteralPath $uninstallKey
    $reason = Get-SetupLog
    $uninstallers = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        $_.ExecutablePath -and $_.ExecutablePath -like '*\~nsu*'
    } | ForEach-Object { '{0} ({1})' -f $_.Name, $_.ProcessId })
    $holders = @(Get-Process -ErrorAction SilentlyContinue | Where-Object {
        try { $_.Path -and $_.Path.StartsWith($installDir, [StringComparison]::OrdinalIgnoreCase) } catch { $false }
    } | ForEach-Object { '{0} ({1})' -f $_.ProcessName, $_.Id })
    $watch = [Diagnostics.Stopwatch]::StartNew()
    while ((Test-Path -LiteralPath $installedExe) -and $watch.Elapsed.TotalSeconds -lt 15) { Start-Sleep -Milliseconds 250 }
    $later = "It disappeared about $([int]$watch.Elapsed.TotalMilliseconds) ms later."
    if (Test-Path -LiteralPath $installedExe) { $later = 'It was still there 15 seconds later.' }
    throw ("Uninstall left $installedExe. Uninstall registration still present: $refused. " +
        "Uninstaller copies still running: $(if ($uninstallers) { $uninstallers -join ', ' } else { 'none' }). " +
        "Processes running from the install folder: $(if ($holders) { $holders -join ', ' } else { 'none' }). $later " +
        "Reason given by the uninstaller: $(if ($reason) { $reason } else { 'none' }).")
}

$setupFiles = @(Get-ChildItem -LiteralPath $ArtifactDirectory -Filter '*_Windows_x64_Setup.exe' -File)
if ($setupFiles.Count -ne 1 -or $setupFiles[0].Name -cnotmatch '^Network_Toolkit_(\d+\.\d+\.\d+)_Windows_x64_Setup\.exe$') {
    throw 'The artifact must contain exactly one versioned Network Toolkit Windows setup executable.'
}
$version = $Matches[1]
$expectedVersion = [version]"$version.0"
$setup = $setupFiles[0].FullName
$portableExe = (Get-Item -LiteralPath (Join-Path $ArtifactDirectory 'Network_Toolkit.exe')).FullName
Assert-Same (Get-ExecutableVersion $setup) $expectedVersion 'Setup executable version'
Assert-Same (Get-ExecutableVersion $portableExe) $expectedVersion 'Portable executable version'
$portableHash = (Get-FileHash -LiteralPath $portableExe -Algorithm SHA256).Hash
# The separate Text Editor and Terminal programs (docs/SEPARATE_EXECUTABLES.md)
# are not release assets of their own; the portable ZIP of the same build holds
# the bytes Setup must install.
$portableZip = @(Get-ChildItem -LiteralPath $ArtifactDirectory -Filter "Network_Toolkit_${version}_Portable.zip" -File)
if ($portableZip.Count -ne 1) { throw 'The artifact must contain the portable ZIP of the same version.' }
Add-Type -AssemblyName System.IO.Compression.FileSystem
$separateHashes = @{}
$zip = [IO.Compression.ZipFile]::OpenRead($portableZip[0].FullName)
try {
    foreach ($name in 'Network_Toolkit_TextEditor.exe', 'Network_Toolkit_Terminal.exe') {
        $entry = $zip.GetEntry("Network_Toolkit_$version/$name")
        if ($null -eq $entry) { throw "The portable ZIP does not contain $name." }
        $stream = $entry.Open()
        try { $separateHashes[$name] = (Get-FileHash -InputStream $stream -Algorithm SHA256).Hash } finally { $stream.Dispose() }
    }
} finally { $zip.Dispose() }
# What Task Manager, the taskbar and Open with show for each program.
$programDescriptions = [ordered]@{
    'Network_Toolkit.exe' = 'Network Toolkit - Network Diagnostics'
    'Network_Toolkit_TextEditor.exe' = 'Network Toolkit Text Editor'
    'Network_Toolkit_Terminal.exe' = 'Network Toolkit Terminal'
}

$installDir = Join-Path $env:LOCALAPPDATA 'Programs\Network Toolkit'
$dataDir = Join-Path $env:LOCALAPPDATA 'NetworkToolkit'
$installedExe = Join-Path $installDir 'Network_Toolkit.exe'
$installedEditor = Join-Path $installDir 'Network_Toolkit_TextEditor.exe'
$installedTerminal = Join-Path $installDir 'Network_Toolkit_Terminal.exe'
$uninstaller = Join-Path $installDir 'Uninstall.exe'
$menu = Join-Path ([Environment]::GetFolderPath('Programs')) 'Network Toolkit'
$desktopShortcut = Join-Path ([Environment]::GetFolderPath('DesktopDirectory')) 'Network Toolkit.lnk'
$uninstallSubkey = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\NetworkToolkit'
$uninstallKey = "HKCU:\$uninstallSubkey"
$appKey = 'HKCU:\Software\Network Toolkit'
$capabilities = "$appKey\Capabilities"
$registeredApps = 'HKCU:\Software\RegisteredApplications'
$registeredName = 'Network Toolkit Text Editor'
$progID = 'NetworkToolkit.TextDocument'
$progKey = "HKCU:\Software\Classes\$progID"
$progSubkey = "Software\Classes\$progID"
# Zero-based icon groups of the program file (ICON_GROUPS in
# scripts/build_windows_resources.py): 0 app, 1 Text Editor, 2 Terminal,
# 3 text document. The program values (Default apps, right-click entry) use the
# editor icon; files use the sheet: the ProgID's DefaultIcon and the DefaultIcon
# of the Applications entries, which Explorer shows for files associated
# through them.
$editorIconIndex = 1
$documentIconIndex = 3
$iconGroupCount = 4
# The one list of text file types that Setup registers. windows.nsi includes
# this same file; scripts/text-extensions.test.cjs keeps the guide in step.
$extensionList = Join-Path $PSScriptRoot 'text-extensions.nsh'
$extensions = @([regex]::Matches([IO.File]::ReadAllText($extensionList), '(?m)^\s*!insertmacro \$\{ACTION\} "(\.[a-z0-9_-]+)"\s*$') | ForEach-Object { $_.Groups[1].Value })
if ($extensions.Count -lt 50) { throw "Only $($extensions.Count) text file types were read from $extensionList." }
if (@($extensions | Sort-Object -Unique).Count -ne $extensions.Count) { throw "A text file type is listed twice in $extensionList." }
foreach ($released in '.txt', '.log', '.ini', '.cfg', '.conf') {
    if ($extensions -cnotcontains $released) { throw "$released was registered by released versions and is missing from $extensionList." }
}
$editorCommandFormat = '"{0}" --editor-standalone --open-file "%1"'
# "Open with" for any file, the optional right-click entry, and Setup's record
# of the keys it created where another program's key could have been.
$applicationsSubkey = 'Software\Classes\Applications'
$applicationSubkey = "$applicationsSubkey\Network_Toolkit.exe"
# Windows writes this entry itself when the user picks the separate Text Editor
# program in "Open with": a command, no icon. Setup never creates it; when it
# starts the installed Text Editor program, Setup gives it the document icon and
# records the value (EditorApplicationIcon), and uninstall takes back only that.
$editorApplicationSubkey = "$applicationsSubkey\Network_Toolkit_TextEditor.exe"
$editorApplicationExpected = $false
$allFilesSubkey = 'Software\Classes\*'
$menuVerbSubkey = "$allFilesSubkey\shell\NetworkToolkit.TextEditor"
$editorKey = "$appKey\TextEditor"
$createdSubkey = 'Software\Network Toolkit\TextEditor\CreatedKeys'
$contextMenuExpected = $false
$rivalMenuTree = ''
# Each Start menu shortcut starts its own program without arguments; the
# separate programs' icon 0 is their app's icon.
$shortcutPrograms = [ordered]@{
    'Network Toolkit.lnk' = 'Network_Toolkit.exe'
    'Network Toolkit Text Editor.lnk' = 'Network_Toolkit_TextEditor.exe'
    'Network Toolkit Terminal.lnk' = 'Network_Toolkit_Terminal.exe'
}
$shortcutIcons = @{
    'Network Toolkit Text Editor.lnk' = 0
    'Network Toolkit Terminal.lnk' = 0
}
$terminalRegisteredName = 'Network Toolkit Terminal'
$terminalKey = "$appKey\Terminal"
$terminalCapabilities = "$terminalKey\Capabilities"
$linkProgID = 'NetworkToolkit.TerminalLink'
$linkProgKey = "HKCU:\Software\Classes\$linkProgID"
$linkSchemes = [ordered]@{ ssh = 'SSH Protocol'; telnet = 'Telnet Protocol'; serial = 'Serial Port Protocol' }

foreach ($path in @($installDir, $dataDir, $menu, $desktopShortcut, $uninstallKey, "HKLM:\$uninstallSubkey", $appKey, $progKey, $linkProgKey)) {
    Assert-Absent $path
}
Assert-Same (Read-RegistryValue $registeredApps $registeredName) $null 'Preexisting app registration'
Assert-Same (Read-RegistryValue $registeredApps $terminalRegisteredName) $null 'Preexisting terminal link registration'

# Another program's per-user telnet: handler. Setup must list the Terminal for
# telnet links without replacing, changing or removing this handler.
$fixtureTelnetSubkey = 'Software\Classes\telnet'
if (-not (Test-Path -LiteralPath "HKCU:\$fixtureTelnetSubkey")) {
    $fixtureTelnet = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($fixtureTelnetSubkey)
    try {
        $fixtureTelnet.SetValue('', 'URL:Synthetic Telnet Client')
        $fixtureTelnet.SetValue('URL Protocol', '')
        $fixtureCommand = $fixtureTelnet.CreateSubKey('shell\open\command')
        try { $fixtureCommand.SetValue('', '"C:\Synthetic Telnet\telnet.exe" "%1"') } finally { $fixtureCommand.Dispose() }
    } finally { $fixtureTelnet.Dispose() }
}
# Setup describes a scheme only when neither this user nor the machine has one.
$schemeOwnedBySetup = @{}
$schemeSnapshots = @{}
$originalUrlChoices = @{}
foreach ($scheme in $linkSchemes.Keys) {
    $schemeOwnedBySetup[$scheme] = -not (Test-Path -LiteralPath "HKCU:\Software\Classes\$scheme") -and -not (Test-Path -LiteralPath "HKLM:\Software\Classes\$scheme")
    $schemeSnapshots[$scheme] = Get-RegistryTree "HKCU:\Software\Classes\$scheme"
    $originalUrlChoices[$scheme] = Get-UrlUserChoice $scheme
}
if ($schemeOwnedBySetup['telnet']) { throw 'The unrelated telnet handler fixture is missing.' }

$fixtureProgID = "NetworkToolkit.SetupSmoke.Unrelated.$([Guid]::NewGuid().ToString('N'))"
$fixtureOpenWithSubkey = 'Software\Classes\.txt\OpenWithProgids'
$fixtureOpenWith = "HKCU:\$fixtureOpenWithSubkey"
$fixtureKey = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($fixtureOpenWithSubkey)
try { $fixtureKey.SetValue($fixtureProgID, '') } finally { $fixtureKey.Dispose() }

$originalChoices = @{}
foreach ($ext in $extensions) {
    $originalChoices[$ext] = Get-UserChoice $ext
    Assert-Same (Read-RegistryValue "HKCU:\Software\Classes\$ext\OpenWithProgids" $progID) $null "Preexisting $ext Open With registration"
}

# Another program that owns .cfg for this user: its ProgID is the default value
# of the extension. Setup must list the Text Editor beside it and change neither
# the default nor the program's own registration.
$rivalProgID = 'SyntheticConfigTool.Document'
$rivalProgSubkey = "Software\Classes\$rivalProgID"
Set-UserValue 'Software\Classes\.cfg' '' $rivalProgID
Set-UserValue 'Software\Classes\.cfg' 'Content Type' 'text/plain'
Set-UserValue $rivalProgSubkey '' 'Synthetic configuration document'
Set-UserValue "$rivalProgSubkey\shell\open\command" '' '"C:\Synthetic Config Tool\tool.exe" "%1"'
$rivalProgTree = Get-UserKeyTree $rivalProgSubkey

# Every key of the current user that Setup may add to, as it is before Setup
# runs. After a rolled back install and after each uninstall they must be
# exactly this again: Setup removes what it created and nothing else.
$watchedClassKeys = @($extensions | ForEach-Object { "Software\Classes\$_" }) + $allFilesSubkey, $applicationsSubkey, $applicationSubkey
$classSnapshots = @{}
$extensionDefaults = @{}
$extensionKeyExisted = @{}
$openWithExisted = @{}
function Save-ClassSnapshot([string]$Subkey) { $classSnapshots[$Subkey] = Get-UserKeyTree $Subkey }
foreach ($subkey in $watchedClassKeys) { Save-ClassSnapshot $subkey }
foreach ($ext in $extensions) {
    $extensionDefaults[$ext] = Read-UserValue "Software\Classes\$ext" ''
    $extensionKeyExisted[$ext] = $classSnapshots["Software\Classes\$ext"] -cne '<absent>'
    $openWithExisted[$ext] = (Get-UserKeyTree "Software\Classes\$ext\OpenWithProgids") -cne '<absent>'
}
if (-not $extensionKeyExisted['.cfg'] -or $openWithExisted['.cfg'] -or -not $openWithExisted['.txt']) {
    throw 'The fixtures for an owned extension (.cfg) and an existing Open With list (.txt) are not in place.'
}
$allFilesKeyExisted = $classSnapshots[$allFilesSubkey] -cne '<absent>'
$allFilesShellExisted = (Get-UserKeyTree "$allFilesSubkey\shell") -cne '<absent>'
$applicationsKeyExisted = $classSnapshots[$applicationsSubkey] -cne '<absent>'
$applicationOwnedBySetup = $classSnapshots[$applicationSubkey] -ceq '<absent>'
if (-not $applicationOwnedBySetup) { throw "This runner already has HKCU\$applicationSubkey; the Open With check needs a clean account." }
Assert-Same (Get-UserKeyTree $editorApplicationSubkey) '<absent>' 'Preexisting Open With entry of the Text Editor program'
Assert-Same (Get-UserKeyTree $menuVerbSubkey) '<absent>' 'Preexisting right-click entry'

function Assert-ClassKeysRestored([string]$When) {
    foreach ($subkey in $watchedClassKeys) {
        $now = Get-UserKeyTree $subkey
        if ($now -cne $classSnapshots[$subkey]) {
            throw "$When, HKCU\$subkey is not as it was before Setup ran.`nBefore:`n$($classSnapshots[$subkey])`nNow:`n$now"
        }
    }
    Assert-Same (Get-UserKeyTree $menuVerbSubkey) '<absent>' "$When, the right-click entry"
    Assert-Same (Get-UserKeyTree $createdSubkey) '<absent>' "$When, Setup's record of created keys"
    Assert-Absent $editorKey
}

# Setup only offers the Text Editor. No file type, known or unknown, may have
# it as the user's choice unless the user made that choice in Windows.
function Get-EditorUserChoices {
    $root = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts')
    if ($null -eq $root) { return @() }
    try {
        @(foreach ($name in $root.GetSubKeyNames()) {
            try { $choice = $root.OpenSubKey("$name\UserChoice") } catch { continue }
            if ($null -eq $choice) { continue }
            try { $chosen = [string]$choice.GetValue('ProgId') } finally { $choice.Dispose() }
            if ($chosen -eq $progID -or $chosen -like 'Applications\Network_Toolkit.exe*') { "$name=$chosen" }
        })
    } finally { $root.Dispose() }
}
$editorChoicesBefore = @(Get-EditorUserChoices) -join ', '

$fixtures = @{}
function Add-Fixture([string]$Path, [string]$Content) {
    [IO.File]::WriteAllText($Path, $Content)
    $fixtures[$Path] = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash
}

function Assert-Preserved {
    foreach ($path in $fixtures.Keys) {
        Assert-Same (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash $fixtures[$path] "Fixture $path"
    }
    Assert-Same (Read-RegistryValue $fixtureOpenWith $fixtureProgID) '' 'Unrelated Open With entry'
    foreach ($ext in $extensions) {
        if ((Get-UserChoice $ext) -cne $originalChoices[$ext]) {
            throw "The current default app selection changed for $ext."
        }
        # The extension's default value names the program that owns the type.
        Assert-Same (Read-UserValue "Software\Classes\$ext" '') $extensionDefaults[$ext] "Default value of HKCU\Software\Classes\$ext"
    }
    Assert-Same (@(Get-EditorUserChoices) -join ', ') $editorChoicesBefore 'File types with the Text Editor as the user choice (Setup must never write a UserChoice)'
    Assert-Same (Read-UserValue 'Software\Classes\.cfg' '') $rivalProgID "Another program's ownership of .cfg"
    Assert-Same (Read-UserValue 'Software\Classes\.cfg' 'Content Type') 'text/plain' "Another program's .cfg content type"
    Assert-Same (Get-UserKeyTree $rivalProgSubkey) $rivalProgTree "Another program's .cfg registration"
    if ($rivalMenuTree) {
        Assert-Same (Get-UserKeyTree "$allFilesSubkey\shell\SyntheticEdit") $rivalMenuTree "Another program's right-click entry for all files"
    }
    if (-not $applicationOwnedBySetup) {
        Assert-Same (Get-UserKeyTree $applicationSubkey) $classSnapshots[$applicationSubkey] "Another program's Open With entry named Network_Toolkit.exe"
    }
    foreach ($scheme in $linkSchemes.Keys) {
        if ((Get-UrlUserChoice $scheme) -cne $originalUrlChoices[$scheme]) {
            throw "The current default app selection changed for $scheme links."
        }
        if (-not $schemeOwnedBySetup[$scheme]) {
            Assert-Same (Get-RegistryTree "HKCU:\Software\Classes\$scheme") $schemeSnapshots[$scheme] "Existing $scheme link handler"
        }
    }
}

function Assert-LinkRegistrationAbsent {
    foreach ($path in @($terminalKey, $linkProgKey)) {
        Assert-Absent $path
    }
    Assert-Same (Read-RegistryValue $registeredApps $terminalRegisteredName) $null 'Removed terminal link registration'
    foreach ($scheme in $linkSchemes.Keys) {
        if ($schemeOwnedBySetup[$scheme]) {
            Assert-Absent "HKCU:\Software\Classes\$scheme"
        }
    }
}

function Assert-Installed([string]$ExpectedHash = $portableHash) {
    Assert-Same (Get-FileHash -LiteralPath $installedExe -Algorithm SHA256).Hash $ExpectedHash 'Installed executable SHA-256'
    foreach ($name in $separateHashes.Keys) {
        Assert-Same (Get-FileHash -LiteralPath (Join-Path $installDir $name) -Algorithm SHA256).Hash $separateHashes[$name] "Installed $name SHA-256"
    }
    foreach ($name in $programDescriptions.Keys) {
        if ($name -eq 'Network_Toolkit.exe' -and $ExpectedHash -cne $portableHash) { continue }
        $info = [Diagnostics.FileVersionInfo]::GetVersionInfo((Join-Path $installDir $name))
        Assert-Same $info.FileDescription $programDescriptions[$name] "$name description"
        Assert-Same $info.OriginalFilename $name "$name original file name"
        Assert-Same $info.ProductName 'Network Toolkit' "$name product name"
        Assert-Same ([version]::new($info.FileMajorPart, $info.FileMinorPart, $info.FileBuildPart, $info.FilePrivatePart)) $expectedVersion "$name version"
    }
    if (-not (Test-Path -LiteralPath $uninstaller -PathType Leaf)) { throw 'The uninstaller is missing.' }
    Assert-Absent "HKLM:\$uninstallSubkey"
    Assert-Same (Read-RegistryValue $uninstallKey 'DisplayName') 'Network Toolkit' 'Installed app name'
    Assert-Same (Read-RegistryValue $uninstallKey 'DisplayVersion') $version 'Installed app version'
    Assert-Same (Read-RegistryValue $uninstallKey 'DisplayIcon') $installedExe 'Installed app icon'
    Assert-Same (Read-RegistryValue $uninstallKey 'InstallLocation') $installDir 'Installed app location'
    Assert-Same (Read-RegistryValue $uninstallKey 'UninstallString') ('"{0}"' -f $uninstaller) 'Uninstall command'
    Assert-Same (Read-RegistryValue $uninstallKey 'QuietUninstallString') ('"{0}" /S' -f $uninstaller) 'Silent uninstall command'
    Assert-Same (Read-RegistryValue $registeredApps $registeredName) 'Software\Network Toolkit\Capabilities' 'Default Apps registration'
    Assert-Same (Read-RegistryValue $capabilities 'ApplicationName') $registeredName 'Default Apps display name'
    Assert-Same (Read-RegistryValue $capabilities 'ApplicationIcon') ('"{0}",{1}' -f $installedExe, $editorIconIndex) 'Default Apps editor icon'
    # Files of the associated types show the paper sheet, not the program tile.
    Assert-Same (Read-UserValue "$progSubkey\DefaultIcon" '') ('"{0}",{1}' -f $installedExe, $documentIconIndex) 'Text document icon of associated files'
    if (-not (Read-RegistryValue $capabilities 'ApplicationDescription')) { throw 'The Default Apps description is missing.' }
    $editorCommand = $editorCommandFormat -f $installedExe
    Assert-Same (Read-RegistryValue "$progKey\shell\open\command" '') $editorCommand 'Editor open command'
    $editorClass = Get-Item -LiteralPath $progKey
    try {
        Assert-Same ($editorClass.GetValueKind('AllowSilentDefaultTakeOver')) ([Microsoft.Win32.RegistryValueKind]::None) 'Default takeover opt-out'
    } finally { $editorClass.Dispose() }
    $registeredTypes = Get-Item -LiteralPath "$capabilities\FileAssociations"
    try { Assert-Same $registeredTypes.ValueCount $extensions.Count 'Number of file types listed in Default Apps' } finally { $registeredTypes.Dispose() }
    foreach ($ext in $extensions) {
        Assert-Same (Read-RegistryValue "$capabilities\FileAssociations" $ext) $progID "$ext Default Apps registration"
        Assert-Same (Read-RegistryValue "HKCU:\Software\Classes\$ext\OpenWithProgids" $progID) '' "$ext Open With registration"
        # Setup records a key only when it created it, and writes nothing else there.
        if (-not $extensionKeyExisted[$ext]) {
            Assert-Same (Get-UserKeyTree "Software\Classes\$ext") "[OpenWithProgids]{$progID=String:}" "Everything Setup wrote under HKCU\Software\Classes\$ext"
            Assert-Same (Read-UserValue $createdSubkey $ext) 1 "Record that Setup created the $ext key"
        } else {
            Assert-Same (Read-UserValue $createdSubkey $ext) $null "Record for the $ext key, which existed before Setup"
            $expectedRecord = if ($openWithExisted[$ext]) { $null } else { 1 }
            Assert-Same (Read-UserValue $createdSubkey "$ext\OpenWithProgids") $expectedRecord "Record for the Open With list of $ext"
        }
    }
    # "Open with" for any file: the program is registered under Applications
    # with its command and no SupportedTypes, which would limit it to a list.
    # Its DefaultIcon is the icon of the files associated through it: the sheet.
    if ($applicationOwnedBySetup) {
        $expectedApplication = "FriendlyAppName=String:$registeredName`n[DefaultIcon]{=String:`"$installedExe`",$documentIconIndex}`n[shell]{[open]{[command]{=String:$editorCommand}}}"
        Assert-Same (Get-UserKeyTree $applicationSubkey) $expectedApplication 'Open With registration for any file'
        Assert-Same (Read-UserValue $createdSubkey 'Applications\Network_Toolkit.exe') 1 'Record that Setup created the Open With registration'
        $expectedRecord = if ($applicationsKeyExisted) { $null } else { 1 }
        Assert-Same (Read-UserValue $createdSubkey 'Applications') $expectedRecord 'Record for the Applications key'
    } else {
        Assert-Same (Read-UserValue $createdSubkey 'Applications\Network_Toolkit.exe') $null "Record for another program's Open With entry"
        Assert-Same (Read-UserValue $createdSubkey 'Applications') $null "Record for the Applications key beside another program's entry"
    }
    # Windows' own entry of the installed Text Editor program: the sheet, its
    # command exactly as Windows wrote it, and Setup's record of the icon.
    if ($editorApplicationExpected) {
        $documentIcon = '"{0}",{1}' -f $installedExe, $documentIconIndex
        Assert-Same (Get-UserKeyTree $editorApplicationSubkey) "[DefaultIcon]{=String:$documentIcon}`n[shell]{[open]{[command]{=String:`"$installedEditor`" `"%1`"}}}" "Windows' own Open With entry of the Text Editor program"
        Assert-Same (Read-UserValue 'Software\Network Toolkit\TextEditor' 'EditorApplicationIcon') $documentIcon 'Record of the icon Setup wrote into that entry'
    }
    # The right-click entry exists only when it was chosen.
    if ($contextMenuExpected) {
        $expectedVerb = "=String:Edit with $registeredName`nIcon=String:`"$installedExe`",$editorIconIndex`n[command]{=String:$editorCommand}"
        Assert-Same (Get-UserKeyTree $menuVerbSubkey) $expectedVerb 'Right-click entry for all files'
        Assert-Same (Read-RegistryValue $uninstallKey 'ContextMenu') 1 'Remembered right-click menu choice'
        $expectedRecord = if ($allFilesKeyExisted) { $null } else { 1 }
        Assert-Same (Read-UserValue $createdSubkey '*') $expectedRecord 'Record for the all-files key'
        $expectedRecord = if ($allFilesKeyExisted -and -not $allFilesShellExisted) { 1 } else { $null }
        Assert-Same (Read-UserValue $createdSubkey '*\shell') $expectedRecord 'Record for the all-files shell key'
    } else {
        Assert-Same (Get-UserKeyTree $menuVerbSubkey) '<absent>' 'Right-click entry that was not chosen'
        Assert-Same (Read-RegistryValue $uninstallKey 'ContextMenu') $null 'Right-click menu choice that was not made'
        Assert-Same (Get-UserKeyTree $allFilesSubkey) $classSnapshots[$allFilesSubkey] 'All-files key without the right-click entry'
    }
    Assert-Same (Read-RegistryValue $registeredApps $terminalRegisteredName) 'Software\Network Toolkit\Terminal\Capabilities' 'Terminal Default Apps registration'
    Assert-Same (Read-RegistryValue $terminalCapabilities 'ApplicationName') $terminalRegisteredName 'Terminal Default Apps display name'
    Assert-Same (Read-RegistryValue $terminalCapabilities 'ApplicationIcon') ('"{0}",2' -f $installedExe) 'Terminal Default Apps icon'
    if (-not (Read-RegistryValue $terminalCapabilities 'ApplicationDescription')) { throw 'The terminal Default Apps description is missing.' }
    Assert-Same (Read-RegistryValue $linkProgKey '') 'Network Toolkit Terminal link' 'Terminal link type name'
    Assert-Same (Read-RegistryValue $linkProgKey 'URL Protocol') '' 'Terminal link URL protocol marker'
    Assert-Same (Read-RegistryValue "$linkProgKey\DefaultIcon" '') ('"{0}",2' -f $installedExe) 'Terminal link icon'
    Assert-Same (Read-RegistryValue "$linkProgKey\shell\open\command" '') ('"{0}" --terminal --open-url "%1"' -f $installedExe) 'Terminal link open command'
    foreach ($scheme in $linkSchemes.Keys) {
        Assert-Same (Read-RegistryValue "$terminalCapabilities\URLAssociations" $scheme) $linkProgID "$scheme link Default Apps registration"
        $schemeKey = "HKCU:\Software\Classes\$scheme"
        if ($schemeOwnedBySetup[$scheme]) {
            # Only the scheme description; Windows asks the user to pick the app.
            Assert-Same (Get-RegistryTree $schemeKey) ("=String:URL:{0}`nURL Protocol=String:" -f $linkSchemes[$scheme]) "$scheme link scheme description"
            Assert-Same (Read-RegistryValue "$terminalKey\CreatedSchemes" $scheme) 1 "$scheme scheme ownership record"
        } else {
            Assert-Same (Read-RegistryValue "$terminalKey\CreatedSchemes" $scheme) $null "$scheme scheme ownership record"
        }
    }

    $shell = New-Object -ComObject WScript.Shell
    try {
        foreach ($name in $shortcutPrograms.Keys) {
            $path = Join-Path $menu $name
            if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Start menu shortcut is missing: $name" }
            $program = Join-Path $installDir $shortcutPrograms[$name]
            if ($keptShortcuts.ContainsKey($name)) { continue }
            $link = $shell.CreateShortcut($path)
            try {
                Assert-Same $link.TargetPath $program "$name target"
                Assert-Same $link.Arguments '' "$name arguments"
                if ($shortcutIcons.ContainsKey($name)) {
                    Assert-Same ($link.IconLocation -replace ',\s+', ',') ('{0},{1}' -f $program, $shortcutIcons[$name]) "$name icon"
                }
            } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($link) }
        }
    } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($shell) }
}

# Start menu shortcuts the user changed: Setup must keep them as they are.
$keptShortcuts = @{}

New-Item -Path $dataDir -ItemType Directory | Out-Null
Add-Fixture (Join-Path $dataDir 'Network_Toolkit.settings.json') '{"theme":"dark"}'
Add-Fixture (Join-Path $dataDir 'Network_Toolkit.macros.json') '{"syntheticFixture":true}'
Add-Fixture (Join-Path $dataDir 'Network_Toolkit.credentials.v2.vault') 'Synthetic installer preservation fixture'

Write-Host 'Checking setup preserves files in an occupied custom folder.'
$occupiedDir = Join-Path $env:RUNNER_TEMP 'Occupied Network Toolkit Install'
New-Item -Path $occupiedDir -ItemType Directory | Out-Null
Add-Fixture (Join-Path $occupiedDir 'Uninstall.exe') 'Unrelated program in a custom installation directory'
Invoke-Silent $setup 2 "/S /D=$occupiedDir" 'without an existing Network Toolkit executable or uninstaller'
Assert-Absent $uninstallKey
Assert-Preserved

Write-Host 'Checking setup refuses a custom folder beneath a junction.'
$junctionPath = Join-Path $env:RUNNER_TEMP 'Redirected Network Toolkit Install'
$junctionTarget = Join-Path $env:RUNNER_TEMP 'Physical Network Toolkit Fixture'
New-Item -Path $junctionTarget -ItemType Directory | Out-Null
New-Item -Path $junctionPath -ItemType Junction -Target $junctionTarget | Out-Null
try {
    Invoke-Silent $setup 2 "/S /D=$junctionPath\Nested" 'does not redirect to another location'
    Assert-Absent (Join-Path $junctionTarget 'Nested')
    Assert-Absent $uninstallKey
} finally { [IO.Directory]::Delete($junctionPath) }

Write-Host 'Checking setup refuses a folder with a % sign in its path.'
# Explorer expands %1, %L and the like anywhere in a registered command.
$percentDir = Join-Path $env:RUNNER_TEMP 'Network Toolkit 100%L'
Invoke-Silent $setup 2 "/S /D=$percentDir" 'without a % sign in its path'
Assert-Absent $percentDir
Assert-Absent $uninstallKey
Assert-Preserved

Write-Host 'Checking a fresh setup rolls back when it cannot create Start menu shortcuts.'
$lateFailureDir = Join-Path $env:RUNNER_TEMP 'Failed Network Toolkit Install'
[IO.File]::WriteAllText($menu, 'Synthetic item occupying the Start menu folder')
$blockedMenuHash = (Get-FileHash -LiteralPath $menu -Algorithm SHA256).Hash
try {
    # With the right-click entry chosen: every registration is written before
    # the Start menu step fails, and all of it must be taken back.
    Invoke-Silent $setup 2 "/S /ContextMenu /D=$lateFailureDir" 'could not finish while creating the Start menu (folder|shortcuts)'
    Assert-Absent (Join-Path $lateFailureDir 'Network_Toolkit.exe')
    Assert-Absent (Join-Path $lateFailureDir 'Network_Toolkit_TextEditor.exe')
    Assert-Absent (Join-Path $lateFailureDir 'Network_Toolkit_Terminal.exe')
    Assert-Absent (Join-Path $lateFailureDir 'Uninstall.exe')
    Assert-Absent $uninstallKey
    Assert-Absent $capabilities
    Assert-Absent $progKey
    Assert-Same (Read-RegistryValue $registeredApps $registeredName) $null 'Rolled back editor registration'
    Assert-LinkRegistrationAbsent
    Assert-ClassKeysRestored 'After a rolled back install'
    foreach ($ext in $extensions) {
        Assert-Same (Read-RegistryValue "HKCU:\Software\Classes\$ext\OpenWithProgids" $progID) $null "Rolled back $ext Open With registration"
    }
    Assert-Same (Get-FileHash -LiteralPath $menu -Algorithm SHA256).Hash $blockedMenuHash 'Unrelated Start menu item'
    Assert-Preserved
} finally { Remove-Item -LiteralPath $menu }

# Another program's right-click entry for all files, added only now so that the
# rolled back install above had to create and remove the all-files key itself.
Set-UserValue "$allFilesSubkey\shell\SyntheticEdit" '' 'Edit with Synthetic Editor'
Set-UserValue "$allFilesSubkey\shell\SyntheticEdit\command" '' '"C:\Synthetic Editor\edit.exe" "%1"'
$rivalMenuTree = Get-UserKeyTree "$allFilesSubkey\shell\SyntheticEdit"
Save-ClassSnapshot $allFilesSubkey
$allFilesKeyExisted = $true
$allFilesShellExisted = $true

Write-Host 'Checking silent install, shortcuts, editor and terminal link registration.'
Invoke-Silent $setup
Assert-Installed
Assert-Preserved
Assert-Absent $desktopShortcut
# Every icon a registry value or shortcut names exists: the text document icon
# is the last group, after the three that released Setups already use.
Assert-IconGroups $installedExe $iconGroupCount
# A second Applications entry would list the Text Editor twice in Open with.
Assert-Same (Get-UserKeyTree $editorApplicationSubkey) '<absent>' "Windows' own Open With entry of the Text Editor program, which Setup never creates"

Write-Host 'Checking a terminal link reaches the running Terminal window without connecting.'
# A headless Terminal owner stands in for the open window; the link launch
# exits 0 only after that window validated and queued the connection form.
# 192.0.2.10 is documentation address space; nothing connects to it.
$terminalInstance = Join-Path $dataDir 'Network_Toolkit.Terminal\Network_Toolkit.instance.json'
# The separate Terminal program opens the Terminal without a switch.
$owner = Start-Process -FilePath $installedTerminal -ArgumentList '--no-browser' -PassThru
$null = $owner.Handle
try {
    $deadline = [DateTime]::UtcNow.AddSeconds(45)
    while (-not (Test-Path -LiteralPath $terminalInstance -PathType Leaf)) {
        if ($owner.HasExited) { throw "The Terminal window stopped during startup (exit code $($owner.ExitCode))." }
        if ([DateTime]::UtcNow -gt $deadline) { throw 'The Terminal window did not register within 45 seconds.' }
        Start-Sleep -Milliseconds 200
    }
    $linkLaunch = Start-Process -FilePath $installedTerminal -ArgumentList '--open-url "ssh://operator:synthetic-password@192.0.2.10:2222"' -PassThru
    $null = $linkLaunch.Handle
    try {
        if (-not $linkLaunch.WaitForExit(30000)) { throw 'The link launch did not hand its connection to the running Terminal window.' }
        Assert-Same $linkLaunch.ExitCode 0 'Exit code from the terminal link launch'
    } finally {
        if (-not $linkLaunch.HasExited) { $linkLaunch.Kill() }
        $linkLaunch.Dispose()
    }
    if ($owner.HasExited) { throw 'The Terminal window stopped after receiving a link.' }
    # The registered command and a released shortcut still start
    # Network_Toolkit.exe --terminal: it hands the same request to the separate
    # Terminal program, which hands it to the running window. One Terminal
    # stays open, whichever program was started.
    foreach ($arguments in '--terminal --open-url "telnet://192.0.2.10:2323"', '--terminal --no-browser') {
        $forwarded = Start-Process -FilePath $installedExe -ArgumentList $arguments -PassThru
        $null = $forwarded.Handle
        try {
            if (-not $forwarded.WaitForExit(30000)) { throw "Network_Toolkit.exe $arguments did not finish." }
            Assert-Same $forwarded.ExitCode 0 "Exit code from Network_Toolkit.exe $arguments"
        } finally {
            if (-not $forwarded.HasExited) { $forwarded.Kill() }
            $forwarded.Dispose()
        }
        $deadline = [DateTime]::UtcNow.AddSeconds(30)
        do {
            $others = @(Get-Process -ErrorAction SilentlyContinue | Where-Object {
                $_.Id -ne $owner.Id -and $(try { $_.Path -and $_.Path.StartsWith($installDir, [StringComparison]::OrdinalIgnoreCase) } catch { $false })
            })
            if ($others.Count -gt 0) { Start-Sleep -Milliseconds 200 }
        } while ($others.Count -gt 0 -and [DateTime]::UtcNow -lt $deadline)
        if ($others.Count -gt 0) { throw "A second program stayed open after Network_Toolkit.exe ${arguments}: $(($others | ForEach-Object { $_.ProcessName }) -join ', ')" }
        if ($owner.HasExited) { throw "The Terminal window stopped after Network_Toolkit.exe $arguments." }
    }
    if (Get-NetTCPConnection -OwningProcess $owner.Id -RemoteAddress '192.0.2.10' -ErrorAction SilentlyContinue) {
        throw 'Opening a terminal link started a connection before the user confirmed it.'
    }
} finally {
    if (-not $owner.HasExited) {
        $owner.Kill()
        [void]$owner.WaitForExit(15000)
    }
    $owner.Dispose()
}
Assert-Preserved

Write-Host 'Checking install and uninstall leave a locked program intact.'
$temporaryUninstaller = Join-Path $env:RUNNER_TEMP "ntk-uninstall-$([Guid]::NewGuid().ToString('N')).exe"
# Each of the three programs in turn: nothing is replaced or removed while one
# of them is in use.
foreach ($lockedProgram in $installedExe, $installedEditor, $installedTerminal) {
    $programLock = [IO.File]::Open($lockedProgram, 'Open', 'Read', 'Read')
    try {
        Invoke-Silent $setup 2 '/S' 'could not finish while checking that Network Toolkit and its Text Editor and Terminal programs are closed'
        Assert-Installed
        # NSIS normally relaunches from a temp copy, hiding the worker's exit code.
        # Run our own copy with _?= last and unquoted, including spaces in the path.
        Copy-Item -LiteralPath $uninstaller -Destination $temporaryUninstaller -Force
        Invoke-Silent $temporaryUninstaller 2 "/S _?=$installDir" 'Close Network Toolkit and any separate Editor or Terminal windows before uninstalling'
        Assert-Installed
        Assert-Preserved
    } finally {
        $programLock.Dispose()
        if (Test-Path -LiteralPath $temporaryUninstaller) { Remove-Item -LiteralPath $temporaryUninstaller }
    }
}

Write-Host 'Checking a running separate Terminal program blocks uninstall.'
# The Terminal stopped above left its record; wait for this one's own.
Remove-Item -LiteralPath $terminalInstance -ErrorAction SilentlyContinue
$runningTerminal = Start-Process -FilePath $installedTerminal -ArgumentList '--no-browser' -PassThru
$null = $runningTerminal.Handle
try {
    $deadline = [DateTime]::UtcNow.AddSeconds(45)
    while (-not (Test-Path -LiteralPath $terminalInstance -PathType Leaf)) {
        if ($runningTerminal.HasExited) { throw "The Terminal program stopped during startup (exit code $($runningTerminal.ExitCode))." }
        if ([DateTime]::UtcNow -gt $deadline) { throw 'The Terminal program did not register within 45 seconds.' }
        Start-Sleep -Milliseconds 200
    }
    Copy-Item -LiteralPath $uninstaller -Destination $temporaryUninstaller -Force
    Invoke-Silent $temporaryUninstaller 2 "/S _?=$installDir" 'Close Network Toolkit and any separate Editor or Terminal windows before uninstalling'
    Assert-Installed
    Assert-Preserved
} finally {
    if (-not $runningTerminal.HasExited) {
        $runningTerminal.Kill()
        [void]$runningTerminal.WaitForExit(15000)
    }
    $runningTerminal.Dispose()
    if (Test-Path -LiteralPath $temporaryUninstaller) { Remove-Item -LiteralPath $temporaryUninstaller }
}

Write-Host 'Checking repair replaces the executable and preserves data.'
Add-Fixture (Join-Path $installDir 'user-created.txt') 'Unrelated file in the install directory'
[IO.File]::WriteAllText($installedExe, 'Synthetic fixture for installer repair')
[IO.File]::WriteAllText($installedTerminal, 'Synthetic fixture for installer repair')
# Version 1.21.35 created this shortcut with --editor, and every released Setup
# up to 1.21.46 the Terminal shortcut as Network_Toolkit.exe --terminal. An
# upgrade points both at the separate programs.
$legacyShell = New-Object -ComObject WScript.Shell
try {
    foreach ($legacy in @(@('Network Toolkit Text Editor.lnk', '--editor', 1), @('Network Toolkit Terminal.lnk', '--terminal', 2))) {
        $legacyLink = $legacyShell.CreateShortcut((Join-Path $menu $legacy[0]))
        try {
            $legacyLink.TargetPath = $installedExe
            $legacyLink.Arguments = $legacy[1]
            $legacyLink.IconLocation = '{0},{1}' -f $installedExe, $legacy[2]
            $legacyLink.Save()
        } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($legacyLink) }
    }
} finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($legacyShell) }
# When the user picks the program in "Open with", Windows writes this entry
# itself, with the bare file name. Setup replaces the command of an entry that
# starts the program it installs.
Set-UserValue "$applicationSubkey\shell\open\command" '' ('"{0}" "%1"' -f $installedExe)
# Setup up to 1.21.51 gave that entry the editor icon, which Explorer shows for
# the files associated through it; repair and upgrade write the sheet.
Set-UserValue "$applicationSubkey\DefaultIcon" '' ('"{0}",{1}' -f $installedExe, $editorIconIndex)
# The icon that versions up to 1.21.44 gave the associated files; repair and
# upgrade replace it with the text document icon (checked by Assert-Installed).
Set-UserValue "$progSubkey\DefaultIcon" '' ('"{0}",{1}' -f $installedExe, $editorIconIndex)
Invoke-Silent $setup 0 '/S /DesktopShortcut'
Assert-Installed
Assert-Preserved
Assert-Same (Read-RegistryValue $uninstallKey 'DesktopShortcut') 1 'Selected desktop shortcut'

Write-Host 'Checking an upgrade keeps a Start menu shortcut the user changed.'
$customTerminal = Join-Path $menu 'Network Toolkit Terminal.lnk'
$customShell = New-Object -ComObject WScript.Shell
try {
    $customLink = $customShell.CreateShortcut($customTerminal)
    try {
        $customLink.TargetPath = $installedExe
        $customLink.Arguments = '--terminal --data-dir "C:\Synthetic Data"'
        $customLink.Save()
    } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($customLink) }
} finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($customShell) }
$customHash = (Get-FileHash -LiteralPath $customTerminal -Algorithm SHA256).Hash
$keptShortcuts['Network Toolkit Terminal.lnk'] = $true
Invoke-Silent $setup
Assert-Installed
Assert-Same (Get-FileHash -LiteralPath $customTerminal -Algorithm SHA256).Hash $customHash 'Start menu shortcut the user changed'
# Uninstall leaves it too; put Setup's own shortcut back for the checks below.
$keptShortcuts.Remove('Network Toolkit Terminal.lnk')
Remove-Item -LiteralPath $customTerminal
Invoke-Silent $setup
Assert-Installed

Write-Host 'Checking setup refuses to overwrite a newer executable.'
# An inbox PE has a real fixed version resource. It is copied only, never launched.
$newerExe = Join-Path $env:SystemRoot 'System32\cmd.exe'
if ((Get-ExecutableVersion $newerExe) -le $expectedVersion) { throw 'The downgrade fixture needs a newer executable version.' }
$newerHash = (Get-FileHash -LiteralPath $newerExe -Algorithm SHA256).Hash
$uninstallHash = (Get-FileHash -LiteralPath $uninstaller -Algorithm SHA256).Hash
Copy-Item -LiteralPath $newerExe -Destination $installedExe
Invoke-Silent $setup 2 '/S' 'A newer version of Network Toolkit'
# Refused before anything is placed: the separate programs are untouched too.
Assert-Installed $newerHash
Assert-Same (Get-FileHash -LiteralPath $uninstaller -Algorithm SHA256).Hash $uninstallHash 'Uninstaller after refused downgrade'
Assert-Preserved
Copy-Item -LiteralPath $portableExe -Destination $installedExe

Write-Host 'Checking uninstall removes owned registrations and preserves data.'
Invoke-Uninstaller $uninstaller
Assert-UninstallFinished
foreach ($path in @($installedExe, $installedEditor, $installedTerminal, $uninstaller, $menu, $desktopShortcut, $uninstallKey, "HKLM:\$uninstallSubkey", $capabilities, $progKey, $appKey)) {
    Assert-Absent $path
}
Assert-Same (Read-RegistryValue $registeredApps $registeredName) $null 'Removed Default Apps registration'
Assert-LinkRegistrationAbsent
foreach ($ext in $extensions) {
    Assert-Same (Read-RegistryValue "HKCU:\Software\Classes\$ext\OpenWithProgids" $progID) $null "Removed $ext Open With registration"
}
Assert-ClassKeysRestored 'After uninstall'
Assert-Preserved

Write-Host 'Checking custom location, optional shortcut and right-click entry, and in-place upgrade.'
# An entry named after the program file that starts a copy somewhere else (a
# portable copy the user picked in "Open with"). It is not this Setup's.
Set-UserValue "$applicationSubkey\shell\open\command" '' '"C:\Synthetic Portable\Network_Toolkit.exe" "%1"'
Save-ClassSnapshot $applicationsSubkey
Save-ClassSnapshot $applicationSubkey
$applicationsKeyExisted = $true
$applicationOwnedBySetup = $false
$defaultInstallDir = $installDir
$installDir = Join-Path $env:RUNNER_TEMP 'Custom Network Toolkit Install'
$installedExe = Join-Path $installDir 'Network_Toolkit.exe'
$installedEditor = Join-Path $installDir 'Network_Toolkit_TextEditor.exe'
$installedTerminal = Join-Path $installDir 'Network_Toolkit_Terminal.exe'
$uninstaller = Join-Path $installDir 'Uninstall.exe'
# The user picked this folder's Text Editor program in "Open with" (from a
# copy left behind, below), so Windows wrote its own entry for it: a command,
# no icon. Setup gives it the sheet and uninstall leaves it as Windows wrote it
# (Assert-ClassKeysRestored compares the Applications key with this snapshot).
Set-UserValue "$editorApplicationSubkey\shell\open\command" '' ('"{0}" "%1"' -f $installedEditor)
Save-ClassSnapshot $applicationsSubkey
# An uninstaller of 1.21.46 or older, after an in-app update added the separate
# programs, leaves them behind. A fresh setup in that folder replaces them once
# they are closed, a read-only one included, and refuses while one is open.
New-Item -Path $installDir -ItemType Directory | Out-Null
[IO.File]::WriteAllText($installedEditor, 'Separate program left by an older uninstaller')
[IO.File]::WriteAllText($installedTerminal, 'Separate program left by an older uninstaller')
$leftoverLock = [IO.File]::Open($installedTerminal, 'Open', 'Read', 'Read')
try {
    Invoke-Silent $setup 2 "/S /D=$installDir" 'could not finish while checking that Network Toolkit and its Text Editor and Terminal programs are closed'
    Assert-Absent $installedExe
    Assert-Absent $uninstallKey
    Assert-Same ([IO.File]::ReadAllText($installedEditor)) 'Separate program left by an older uninstaller' 'Left-behind Text Editor program while another one is open'
    Assert-Same (Get-UserKeyTree $editorApplicationSubkey) ('[shell]{[open]{[command]{=String:"' + $installedEditor + '" "%1"}}}') "Windows' own Open With entry of the Text Editor program after a refused setup"
} finally { $leftoverLock.Dispose() }
(Get-Item -LiteralPath $installedTerminal).IsReadOnly = $true
Invoke-Silent $setup 0 "/S /DesktopShortcut /ContextMenu /D=$installDir"
$contextMenuExpected = $true
$editorApplicationExpected = $true
Assert-Installed
Assert-Preserved
$desktopShell = New-Object -ComObject WScript.Shell
try {
    if (-not (Test-Path -LiteralPath $desktopShortcut -PathType Leaf)) { throw 'Selected desktop shortcut is missing.' }
    $desktopLink = $desktopShell.CreateShortcut($desktopShortcut)
    try {
        Assert-Same $desktopLink.TargetPath $installedExe 'Desktop shortcut target'
        Assert-Same $desktopLink.Arguments '' 'Desktop shortcut arguments'
    } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($desktopLink) }
} finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($desktopShell) }
Assert-Absent (Join-Path $defaultInstallDir 'Network_Toolkit.exe')
Assert-Absent (Join-Path $defaultInstallDir 'Network_Toolkit_TextEditor.exe')
Assert-Absent (Join-Path $defaultInstallDir 'Network_Toolkit_Terminal.exe')

# Re-running Setup with no location switch must reuse the registered custom folder.
Invoke-Silent $setup
Assert-Installed
Assert-Same (Read-RegistryValue $uninstallKey 'DesktopShortcut') 1 'Preserved desktop shortcut choice'
Assert-Same (Read-RegistryValue $uninstallKey 'ContextMenu') 1 'Preserved right-click menu choice'

Write-Host "Checking an upgrade keeps an icon the user gave Windows' own entry of the Text Editor program."
$userEditorIcon = 'C:\Synthetic Icons\editor-files.ico'
Set-UserValue "$editorApplicationSubkey\DefaultIcon" '' $userEditorIcon
Invoke-Silent $setup
$editorApplicationExpected = $false
Assert-Installed
Assert-Same (Get-UserKeyTree $editorApplicationSubkey) ("[DefaultIcon]{=String:$userEditorIcon}`n" + '[shell]{[open]{[command]{=String:"' + $installedEditor + '" "%1"}}}') "An icon the user set on Windows' own entry of the Text Editor program"
# Back to the icon Setup recorded: the next upgrade and uninstall own it again.
Set-UserValue "$editorApplicationSubkey\DefaultIcon" '' ('"{0}",{1}' -f $installedExe, $documentIconIndex)
$editorApplicationExpected = $true
Invoke-Silent $setup
Assert-Installed

Write-Host 'Checking /NoContextMenu removes the right-click entry and the two switches together are refused.'
Invoke-Silent $setup 2 '/S /ContextMenu /NoContextMenu' 'Choose either /ContextMenu or /NoContextMenu, not both'
Assert-Installed
Invoke-Silent $setup 0 '/S /NoContextMenu'
$contextMenuExpected = $false
# The entry, the remembered choice and Setup's record for it are gone; the
# other program's entry beside it and every other registration are as before.
Assert-Installed
Assert-Preserved
Assert-Same (Read-UserValue $createdSubkey '*') $null 'Record for the all-files key after /NoContextMenu'
Assert-Same (Read-UserValue $createdSubkey '*\shell') $null 'Record for the all-files shell key after /NoContextMenu'
# A later upgrade without a switch does not bring it back.
Invoke-Silent $setup
Assert-Installed
Invoke-Silent $setup 0 '/S /ContextMenu'
$contextMenuExpected = $true
Assert-Installed

Write-Host 'Checking repair and uninstall preserve a replacement desktop shortcut.'
$replacementShell = New-Object -ComObject WScript.Shell
try {
    $replacementLink = $replacementShell.CreateShortcut($desktopShortcut)
    try {
        $replacementLink.TargetPath = $newerExe
        $replacementLink.Arguments = '/d /c exit'
        $replacementLink.Save()
    } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($replacementLink) }
} finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($replacementShell) }
$fixtures[$desktopShortcut] = (Get-FileHash -LiteralPath $desktopShortcut -Algorithm SHA256).Hash
Invoke-Silent $setup
Assert-Installed
Assert-Preserved

Write-Host 'Checking uninstall keeps a link scheme that another app has since taken over.'
$takenScheme = @($linkSchemes.Keys | Where-Object { $schemeOwnedBySetup[$_] }) | Select-Object -First 1
if ($takenScheme) {
    $takenKey = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey("Software\Classes\$takenScheme\shell\open\command")
    try { $takenKey.SetValue('', '"C:\Synthetic Client\client.exe" "%1"') } finally { $takenKey.Dispose() }
    $takenTree = Get-RegistryTree "HKCU:\Software\Classes\$takenScheme"
}
Invoke-Uninstaller $uninstaller
Assert-UninstallFinished
foreach ($path in @($installedExe, $installedEditor, $installedTerminal, $uninstaller, $menu, $uninstallKey, $terminalKey, $linkProgKey, $appKey)) {
    Assert-Absent $path
}
Assert-Same (Read-RegistryValue $registeredApps $terminalRegisteredName) $null 'Removed terminal link registration'
if ($takenScheme) {
    Assert-Same (Get-RegistryTree "HKCU:\Software\Classes\$takenScheme") $takenTree "Another app's $takenScheme handler"
    [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKeyTree("Software\Classes\$takenScheme")
}
# The right-click entry and every other registration are gone; the other
# programs' entries beside them are as they were.
Assert-ClassKeysRestored 'After uninstall with the right-click entry'
Assert-Preserved

Write-Host 'Checking an upgrade from a released version adopts only the Open With keys that hold nothing but the Text Editor.'
# Setup up to 1.21.43 registered five types and kept no record of the keys it
# created. Install, then make the registry what such a version left behind.
$releasedTypes = '.txt', '.log', '.ini', '.cfg', '.conf'
if ($extensionKeyExisted['.ini'] -or $extensionKeyExisted['.log'] -or $extensionKeyExisted['.conf']) {
    throw 'The upgrade check needs an account without its own .ini, .log and .conf keys.'
}
Invoke-Silent $setup 0 "/S /D=$installDir"
$contextMenuExpected = $false
Assert-Installed
[Microsoft.Win32.Registry]::CurrentUser.DeleteSubKeyTree('Software\Network Toolkit\TextEditor')
foreach ($ext in $extensions) {
    if ($releasedTypes -ccontains $ext) { continue }
    if (-not $extensionKeyExisted[$ext]) {
        [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKeyTree("Software\Classes\$ext")
    } elseif (-not $openWithExisted[$ext]) {
        [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKeyTree("Software\Classes\$ext\OpenWithProgids")
    } else {
        $openWith = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey("Software\Classes\$ext\OpenWithProgids", $true)
        try { $openWith.DeleteValue($progID) } finally { $openWith.Dispose() }
    }
}
# Since then another program has added itself to the Open With list of .ini.
Set-UserValue 'Software\Classes\.ini\OpenWithProgids' 'SyntheticIniTool.Document' ''
$extensionKeyExisted['.ini'] = $true
$openWithExisted['.ini'] = $true
$classSnapshots['Software\Classes\.ini'] = '[OpenWithProgids]{SyntheticIniTool.Document=String:}'
# Versions up to 1.21.44 gave the associated files the editor icon.
Set-UserValue "$progSubkey\DefaultIcon" '' ('"{0}",{1}' -f $installedExe, $editorIconIndex)
Invoke-Silent $setup
# Assert-Installed checks the upgrade replaced it with the text document icon.
Assert-Installed
Assert-Preserved
# .log and .conf hold nothing but the Text Editor: adopted, key and list.
# .cfg belongs to another program: only the list Setup added is adopted.
# .txt and .ini hold another program's value: left unrecorded.
Assert-Same (Read-UserValue $createdSubkey '.log') 1 'Adopted .log key of a released version'
Assert-Same (Read-UserValue $createdSubkey '.conf') 1 'Adopted .conf key of a released version'
Assert-Same (Read-UserValue $createdSubkey '.cfg\OpenWithProgids') 1 'Adopted Open With list of .cfg'
Assert-Same (Read-UserValue $createdSubkey '.cfg') $null "Record for another program's .cfg key"
foreach ($shared in '.txt', '.ini') {
    Assert-Same (Read-UserValue $createdSubkey $shared) $null "Record for the $shared key, whose Open With list another program uses"
    Assert-Same (Read-UserValue $createdSubkey "$shared\OpenWithProgids") $null "Record for the Open With list of $shared, which another program uses"
}
# A later upgrade finds the record and adopts nothing more.
Invoke-Silent $setup
Assert-Installed
Invoke-Uninstaller $uninstaller
Assert-UninstallFinished
Assert-Same (Get-UserKeyTree 'Software\Classes\.ini') '[OpenWithProgids]{SyntheticIniTool.Document=String:}' "Another program's value in the Open With list of .ini"
Assert-ClassKeysRestored 'After uninstalling an upgrade from a released version'
Assert-Preserved

# A portable copy can write Setup's registrations for itself ("Use this copy's
# Text Editor in Windows"). The hidden switch --text-editor-registration runs
# one such action and exits (0 done, 3 the installed program is registered, 4
# another copy is registered, 6 nothing of this copy's to remove, 7 this
# copy's location is refused).
Write-Host 'Checking a portable copy registers its Text Editor like Setup and removes exactly what it added.'
$portableSubkey = 'Software\Network Toolkit\PortableTextEditor'
$portableKey = "HKCU:\$portableSubkey"
$portableCreatedSubkey = "$portableSubkey\CreatedKeys"
# Under the user's own folder: a copy that other users can change is refused
# (security review 11), and the runner's temporary folder may let them.
$portableRoot = Join-Path $env:LOCALAPPDATA 'NTK Portable Check'
$portableDirA = Join-Path $portableRoot 'Portable Copy A'
$portableDirB = Join-Path $portableRoot 'Portable Copy B'
$portableDirShared = Join-Path $portableRoot 'Shared Copy'
foreach ($dir in $portableDirA, $portableDirB, $portableDirShared) {
    New-Item -Path $dir -ItemType Directory | Out-Null
    Copy-Item -LiteralPath $portableExe -Destination (Join-Path $dir 'Network_Toolkit.exe')
}
$portableA = Join-Path $portableDirA 'Network_Toolkit.exe'
$portableB = Join-Path $portableDirB 'Network_Toolkit.exe'
$portableShared = Join-Path $portableDirShared 'Network_Toolkit.exe'
# BUILTIN\Users may change this one, as a folder on a shared drive often allows.
& icacls $portableDirShared /grant '*S-1-5-32-545:(OI)(CI)M' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'icacls could not open the shared copy folder to Users.' }
function Invoke-Registration([string]$Exe, [string]$Action, [int]$ExpectedExitCode) {
    $process = Start-Process -FilePath $Exe -ArgumentList "--text-editor-registration=$Action" -Wait -PassThru
    try { Assert-Same $process.ExitCode $ExpectedExitCode "Exit code from --text-editor-registration=$Action of $Exe" } finally { $process.Dispose() }
}
# All of HKCU\Software\Classes except Local Settings (MuiCache notes every
# program the shell starts, which says nothing about registrations).
function Get-ClassesTree {
    $classes = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Software\Classes')
    try {
        $lines = @(foreach ($name in ($classes.GetValueNames() | Sort-Object)) {
            '{0}={1}:{2}' -f $name, $classes.GetValueKind($name), $classes.GetValue($name, $null, 'DoNotExpandEnvironmentNames')
        })
        $children = @($classes.GetSubKeyNames() | Where-Object { $_ -ne 'Local Settings' } | Sort-Object)
    } finally { $classes.Dispose() }
    foreach ($child in $children) { $lines += "[$child]{" + (Get-UserKeyTree "Software\Classes\$child") + '}' }
    $lines -join "`n"
}
function Assert-PortableRegistered([string]$Launched, [bool]$Menu) {
    # The program records its own path as Windows resolved it (long names,
    # the case on disk); every command must name exactly that path.
    $Exe = Read-UserValue $portableSubkey 'Executable'
    if (-not $Exe -or $Exe -ine $Launched) { throw "The portable registration record names '$Exe', expected '$Launched'." }
    $command = $editorCommandFormat -f $Exe
    Assert-Same (Read-UserValue "Software\Classes\$progID\shell\open\command" '') $command 'Portable editor open command'
    if ((Read-UserValue "Software\Classes\$progID\DefaultIcon" '') -notlike ('"{0}",*' -f $Exe)) { throw 'The document icon does not name the portable copy.' }
    Assert-Same (Read-RegistryValue $registeredApps $registeredName) 'Software\Network Toolkit\Capabilities' 'Portable Default Apps registration'
    Assert-Same (Read-RegistryValue $capabilities 'ApplicationIcon') ('"{0}",1' -f $Exe) 'Portable Default Apps editor icon'
    $registeredTypes = Get-Item -LiteralPath "$capabilities\FileAssociations"
    try { Assert-Same $registeredTypes.ValueCount $extensions.Count 'Number of file types the portable copy lists in Default Apps' } finally { $registeredTypes.Dispose() }
    foreach ($ext in $extensions) {
        Assert-Same (Read-RegistryValue "HKCU:\Software\Classes\$ext\OpenWithProgids" $progID) '' "$ext Open With registration of the portable copy"
        if (-not $extensionKeyExisted[$ext]) {
            Assert-Same (Get-UserKeyTree "Software\Classes\$ext") "[OpenWithProgids]{$progID=String:}" "Everything the portable copy wrote under HKCU\Software\Classes\$ext"
            Assert-Same (Read-UserValue $portableCreatedSubkey $ext) 1 "Portable record that it created the $ext key"
        } else {
            Assert-Same (Read-UserValue $portableCreatedSubkey $ext) $null "Portable record for the $ext key, which existed before"
            $expectedRecord = if ($openWithExisted[$ext]) { $null } else { 1 }
            Assert-Same (Read-UserValue $portableCreatedSubkey "$ext\OpenWithProgids") $expectedRecord "Portable record for the Open With list of $ext"
        }
    }
    $expectedApplication = "FriendlyAppName=String:$registeredName`n[DefaultIcon]{=String:`"$Exe`",$documentIconIndex}`n[shell]{[open]{[command]{=String:$command}}}"
    Assert-Same (Get-UserKeyTree $applicationSubkey) $expectedApplication 'Portable Open With registration for any file'
    Assert-Same (Read-UserValue $portableCreatedSubkey 'Applications\Network_Toolkit.exe') 1 'Portable record of the Open With registration'
    # Windows' own Open With entry of copy A's Text Editor program (set up
    # below) shows A's sheet; the portable record keeps the value written, also
    # through B's takeover, so that whoever removes the registration takes it back.
    $entryIcon = Read-UserValue "$editorApplicationSubkey\DefaultIcon" ''
    if (-not $entryIcon -or $entryIcon -ine ('"{0}",{1}' -f $portableA, $documentIconIndex)) { throw "Windows' own Open With entry of copy A's Text Editor program has the icon '$entryIcon'." }
    Assert-Same (Read-UserValue $portableSubkey 'EditorApplicationIcon') $entryIcon 'Portable record of the icon written into that entry'
    Assert-Same (Read-UserValue "$editorApplicationSubkey\shell\open\command" '') $portableEditorCommandA "Command of Windows' own entry of copy A's Text Editor program"
    if ($Menu) {
        Assert-Same (Get-UserKeyTree $menuVerbSubkey) "=String:Edit with $registeredName`nIcon=String:`"$Exe`",1`n[command]{=String:$command}" 'Portable right-click entry'
        Assert-Same (Read-UserValue $portableSubkey 'ContextMenu') 1 'Portable right-click choice'
    } else {
        Assert-Same (Get-UserKeyTree $menuVerbSubkey) '<absent>' 'Portable right-click entry that was not chosen'
    }
    Assert-Absent $uninstallKey
    Assert-Preserved
}
# The Open With entry of another copy (from the check above) goes, so that the
# portable copy and then Setup own "Open with" for any file.
[Microsoft.Win32.Registry]::CurrentUser.DeleteSubKeyTree($applicationSubkey)
# Windows' own entry of the separate Text Editor program now starts copy A's
# (the user picked it in "Open with"); the program file need not exist for the
# registry checks. Copy A's registration gives it A's sheet.
$portableEditorCommandA = '"{0}" "%1"' -f (Join-Path $portableDirA 'Network_Toolkit_TextEditor.exe')
Set-UserValue "$editorApplicationSubkey\shell\open\command" '' $portableEditorCommandA
Assert-Same (Get-UserKeyTree "$editorApplicationSubkey\DefaultIcon") '<absent>' "Icon of Windows' own entry of the Text Editor program before the portable checks"
$editorApplicationExpected = $false
# Its parent, Applications, stays (now without that entry): snapshot both.
Save-ClassSnapshot $applicationsSubkey
Save-ClassSnapshot $applicationSubkey
$applicationOwnedBySetup = $true
Invoke-Registration $portableA 'remove' 6
$classesBefore = Get-ClassesTree
Assert-Absent $appKey
Invoke-Registration $portableA 'register-menu' 0
Assert-PortableRegistered $portableA $true
Invoke-Registration $portableA 'register' 0
Assert-PortableRegistered $portableA $false
Invoke-Registration $portableA 'remove' 0
Assert-Same (Get-ClassesTree) $classesBefore 'HKCU\Software\Classes after a portable register and remove'
Assert-Absent $appKey
Assert-Same (Read-RegistryValue $registeredApps $registeredName) $null 'Default Apps registration after a portable remove'
Assert-ClassKeysRestored 'After a portable register and remove'

Write-Host 'Checking a portable copy that other users can change is refused.'
# Every double-clicked text file would run that copy.
Invoke-Registration $portableShared 'register' 7
Invoke-Registration $portableShared 'takeover-menu' 7
Assert-Same (Get-ClassesTree) $classesBefore 'HKCU\Software\Classes after a refused registration of a shared copy'
Assert-Absent $appKey
Remove-Item -LiteralPath $portableDirShared -Recurse

Write-Host 'Checking a second portable copy takes over only when asked, and a copy whose program is gone can be cleaned up.'
Invoke-Registration $portableA 'register-menu' 0
Invoke-Registration $portableB 'register' 4
Assert-PortableRegistered $portableA $true
Invoke-Registration $portableB 'takeover' 0
Assert-PortableRegistered $portableB $false
Invoke-Registration $portableA 'remove' 6
# B still exists: there is nothing to clean up.
Invoke-Registration $portableA 'cleanup' 6
Assert-PortableRegistered $portableB $false
Remove-Item -LiteralPath $portableDirB -Recurse
Invoke-Registration $portableA 'cleanup' 0
Assert-Same (Get-ClassesTree) $classesBefore 'HKCU\Software\Classes after cleaning up a deleted copy'
Assert-Absent $appKey

Write-Host 'Checking Setup takes over a portable registration and the portable record is retired.'
Invoke-Registration $portableA 'register-menu' 0
Assert-PortableRegistered $portableA $true
# The portable copy chose the right-click entry: Setup carries the choice over.
Invoke-Silent $setup 0 "/S /D=$installDir"
$contextMenuExpected = $true
Assert-Installed
Assert-Preserved
Assert-Absent $portableKey
# The entry starts copy A's Text Editor program, not the installed one: Setup
# leaves its icon and takes over the record of it, so uninstall takes it back.
$portableEntryIcon = Read-UserValue "$editorApplicationSubkey\DefaultIcon" ''
if (-not $portableEntryIcon -or $portableEntryIcon -ine ('"{0}",{1}' -f $portableA, $documentIconIndex)) { throw "Setup changed the icon of Windows' own entry of copy A's Text Editor program to '$portableEntryIcon'." }
Assert-Same (Read-UserValue 'Software\Network Toolkit\TextEditor' 'EditorApplicationIcon') $portableEntryIcon "Setup's record of the icon the portable copy wrote"
Assert-Same (Read-UserValue "$editorApplicationSubkey\shell\open\command" '') $portableEditorCommandA "Command of Windows' own entry of copy A's Text Editor program after Setup"
Write-Host 'Checking the portable record is retired and the installed program keeps every command.'
$installedClasses = Get-ClassesTree
$installedAppKey = Get-UserKeyTree 'Software\Network Toolkit'
foreach ($action in 'register', 'register-menu', 'takeover', 'remove', 'cleanup') {
    Invoke-Registration $portableA $action 3
}
Assert-Same (Get-ClassesTree) $installedClasses 'HKCU\Software\Classes after a portable copy tried to change the installed registration'
Assert-Same (Get-UserKeyTree 'Software\Network Toolkit') $installedAppKey 'Setup registration after a portable copy tried to change it'
Invoke-Uninstaller $uninstaller
Assert-UninstallFinished
Assert-Absent $appKey
Assert-ClassKeysRestored 'After uninstalling a Setup that took over a portable registration'
Assert-Same (Get-ClassesTree) $classesBefore 'HKCU\Software\Classes after uninstalling a Setup that took over a portable registration'
Assert-Preserved

Write-Host 'Checking a portable copy refuses to register while Setup is installed.'
Invoke-Silent $setup 0 "/S /D=$installDir"
$contextMenuExpected = $false
Assert-Installed
$installedClasses = Get-ClassesTree
Invoke-Registration $portableA 'register' 3
Invoke-Registration $portableA 'takeover-menu' 3
Assert-Same (Get-ClassesTree) $installedClasses 'HKCU\Software\Classes after a refused portable registration'
Assert-Absent $portableKey
Invoke-Uninstaller $uninstaller
Assert-UninstallFinished
Assert-ClassKeysRestored 'After uninstalling Setup that a portable copy left alone'
Assert-Same (Get-ClassesTree) $classesBefore 'HKCU\Software\Classes after the portable checks'
Assert-Preserved
Remove-Item -LiteralPath $portableRoot -Recurse

foreach ($synthetic in "$allFilesSubkey\shell\SyntheticEdit", $applicationSubkey, $editorApplicationSubkey, $rivalProgSubkey, 'Software\Classes\.cfg', 'Software\Classes\.ini') {
    # The synthetic Open With entry was removed for the portable checks above.
    [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKeyTree($synthetic, $false)
}
Write-Host "Windows setup smoke passed for $version."
