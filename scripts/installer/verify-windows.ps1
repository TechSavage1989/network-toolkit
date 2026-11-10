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

function Get-ExecutableVersion([string]$Path) {
    $info = [Diagnostics.FileVersionInfo]::GetVersionInfo($Path)
    [version]::new($info.FileMajorPart, $info.FileMinorPart, $info.FileBuildPart, $info.FilePrivatePart)
}

function Invoke-Silent([string]$Path, [int]$ExpectedExitCode = 0, [string]$Arguments = '/S') {
    $process = Start-Process -FilePath $Path -ArgumentList $Arguments -Wait -PassThru
    try {
        Assert-Same $process.ExitCode $ExpectedExitCode "Exit code from $(Split-Path -Leaf $Path)"
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
            $children = @(Get-CimInstance Win32_Process -Filter "ParentProcessId=$id" -ErrorAction SilentlyContinue)
            $running = -not $process.HasExited -or $children.Count -gt 0
            if ($running) { Start-Sleep -Milliseconds 200 }
        } while ($running -and [DateTime]::UtcNow -lt $deadline)
        if ($running) { throw "$(Split-Path -Leaf $Path) did not finish within two minutes." }
        Assert-Same $process.ExitCode 0 "Exit code from $(Split-Path -Leaf $Path)"
    } finally { $process.Dispose() }
}

# When uninstall leaves the program behind, say why: a process still using a
# file in the folder, or an uninstaller that refused (it keeps its own key).
function Assert-UninstallFinished {
    if (-not (Test-Path -LiteralPath $installedExe)) { return }
    $holders = @(Get-Process -ErrorAction SilentlyContinue | Where-Object {
        try { $_.Path -and $_.Path.StartsWith($installDir, [StringComparison]::OrdinalIgnoreCase) } catch { $false }
    } | ForEach-Object { '{0} ({1})' -f $_.ProcessName, $_.Id })
    $refused = Test-Path -LiteralPath $uninstallKey
    throw ("Uninstall left $installedExe. Uninstall registration still present: $refused. " +
        "Processes running from the install folder: $(if ($holders) { $holders -join ', ' } else { 'none' }).")
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

$installDir = Join-Path $env:LOCALAPPDATA 'Programs\Network Toolkit'
$dataDir = Join-Path $env:LOCALAPPDATA 'NetworkToolkit'
$installedExe = Join-Path $installDir 'Network_Toolkit.exe'
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
$extensions = '.txt', '.log', '.ini', '.cfg', '.conf'
$shortcutArguments = [ordered]@{
    'Network Toolkit.lnk' = ''
    'Network Toolkit Text Editor.lnk' = '--editor-standalone'
    'Network Toolkit Terminal.lnk' = '--terminal'
}
$shortcutIcons = @{
    'Network Toolkit Text Editor.lnk' = 1
    'Network Toolkit Terminal.lnk' = 2
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
    Assert-Same (Read-RegistryValue $capabilities 'ApplicationIcon') ('"{0}",1' -f $installedExe) 'Default Apps editor icon'
    if (-not (Read-RegistryValue $capabilities 'ApplicationDescription')) { throw 'The Default Apps description is missing.' }
    Assert-Same (Read-RegistryValue "$progKey\shell\open\command" '') ('"{0}" --editor-standalone --open-file "%1"' -f $installedExe) 'Editor open command'
    $editorClass = Get-Item -LiteralPath $progKey
    try {
        Assert-Same ($editorClass.GetValueKind('AllowSilentDefaultTakeOver')) ([Microsoft.Win32.RegistryValueKind]::None) 'Default takeover opt-out'
    } finally { $editorClass.Dispose() }
    foreach ($ext in $extensions) {
        Assert-Same (Read-RegistryValue "$capabilities\FileAssociations" $ext) $progID "$ext Default Apps registration"
        Assert-Same (Read-RegistryValue "HKCU:\Software\Classes\$ext\OpenWithProgids" $progID) '' "$ext Open With registration"
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
        foreach ($name in $shortcutArguments.Keys) {
            $path = Join-Path $menu $name
            if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Start menu shortcut is missing: $name" }
            $link = $shell.CreateShortcut($path)
            try {
                Assert-Same $link.TargetPath $installedExe "$name target"
                Assert-Same $link.Arguments $shortcutArguments[$name] "$name arguments"
                if ($shortcutIcons.ContainsKey($name)) {
                    Assert-Same ($link.IconLocation -replace ',\s+', ',') ('{0},{1}' -f $installedExe, $shortcutIcons[$name]) "$name icon"
                }
            } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($link) }
        }
    } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($shell) }
}

New-Item -Path $dataDir -ItemType Directory | Out-Null
Add-Fixture (Join-Path $dataDir 'Network_Toolkit.settings.json') '{"theme":"dark"}'
Add-Fixture (Join-Path $dataDir 'Network_Toolkit.macros.json') '{"syntheticFixture":true}'
Add-Fixture (Join-Path $dataDir 'Network_Toolkit.credentials.v2.vault') 'Synthetic installer preservation fixture'

Write-Host 'Checking setup preserves files in an occupied custom folder.'
$occupiedDir = Join-Path $env:RUNNER_TEMP 'Occupied Network Toolkit Install'
New-Item -Path $occupiedDir -ItemType Directory | Out-Null
Add-Fixture (Join-Path $occupiedDir 'Uninstall.exe') 'Unrelated program in a custom installation directory'
Invoke-Silent $setup 2 "/S /D=$occupiedDir"
Assert-Absent $uninstallKey
Assert-Preserved

Write-Host 'Checking setup refuses a custom folder beneath a junction.'
$junctionPath = Join-Path $env:RUNNER_TEMP 'Redirected Network Toolkit Install'
$junctionTarget = Join-Path $env:RUNNER_TEMP 'Physical Network Toolkit Fixture'
New-Item -Path $junctionTarget -ItemType Directory | Out-Null
New-Item -Path $junctionPath -ItemType Junction -Target $junctionTarget | Out-Null
try {
    Invoke-Silent $setup 2 "/S /D=$junctionPath\Nested"
    Assert-Absent (Join-Path $junctionTarget 'Nested')
    Assert-Absent $uninstallKey
} finally { [IO.Directory]::Delete($junctionPath) }

Write-Host 'Checking a fresh setup rolls back when it cannot create Start menu shortcuts.'
$lateFailureDir = Join-Path $env:RUNNER_TEMP 'Failed Network Toolkit Install'
[IO.File]::WriteAllText($menu, 'Synthetic item occupying the Start menu folder')
$blockedMenuHash = (Get-FileHash -LiteralPath $menu -Algorithm SHA256).Hash
try {
    Invoke-Silent $setup 2 "/S /D=$lateFailureDir"
    Assert-Absent (Join-Path $lateFailureDir 'Network_Toolkit.exe')
    Assert-Absent (Join-Path $lateFailureDir 'Uninstall.exe')
    Assert-Absent $uninstallKey
    Assert-Absent $capabilities
    Assert-Absent $progKey
    Assert-Same (Read-RegistryValue $registeredApps $registeredName) $null 'Rolled back editor registration'
    Assert-LinkRegistrationAbsent
    Assert-Same (Get-FileHash -LiteralPath $menu -Algorithm SHA256).Hash $blockedMenuHash 'Unrelated Start menu item'
    Assert-Preserved
} finally { Remove-Item -LiteralPath $menu }

Write-Host 'Checking silent install, shortcuts, editor and terminal link registration.'
Invoke-Silent $setup
Assert-Installed
Assert-Preserved
Assert-Absent $desktopShortcut

Write-Host 'Checking a terminal link reaches the running Terminal window without connecting.'
# A headless Terminal owner stands in for the open window; the link launch
# exits 0 only after that window validated and queued the connection form.
# 192.0.2.10 is documentation address space; nothing connects to it.
$terminalInstance = Join-Path $dataDir 'Network_Toolkit.Terminal\Network_Toolkit.instance.json'
$owner = Start-Process -FilePath $installedExe -ArgumentList '--terminal --no-browser' -PassThru
$null = $owner.Handle
try {
    $deadline = [DateTime]::UtcNow.AddSeconds(45)
    while (-not (Test-Path -LiteralPath $terminalInstance -PathType Leaf)) {
        if ($owner.HasExited) { throw "The Terminal window stopped during startup (exit code $($owner.ExitCode))." }
        if ([DateTime]::UtcNow -gt $deadline) { throw 'The Terminal window did not register within 45 seconds.' }
        Start-Sleep -Milliseconds 200
    }
    $linkLaunch = Start-Process -FilePath $installedExe -ArgumentList '--terminal --open-url "ssh://operator:synthetic-password@192.0.2.10:2222"' -PassThru
    $null = $linkLaunch.Handle
    try {
        if (-not $linkLaunch.WaitForExit(30000)) { throw 'The link launch did not hand its connection to the running Terminal window.' }
        Assert-Same $linkLaunch.ExitCode 0 'Exit code from the terminal link launch'
    } finally {
        if (-not $linkLaunch.HasExited) { $linkLaunch.Kill() }
        $linkLaunch.Dispose()
    }
    if ($owner.HasExited) { throw 'The Terminal window stopped after receiving a link.' }
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
$programLock = [IO.File]::Open($installedExe, 'Open', 'Read', 'Read')
try {
    Invoke-Silent $setup 2
    Assert-Installed
    # NSIS normally relaunches from a temp copy, hiding the worker's exit code.
    # Run our own copy with _?= last and unquoted, including spaces in the path.
    Copy-Item -LiteralPath $uninstaller -Destination $temporaryUninstaller
    Invoke-Silent $temporaryUninstaller 2 "/S _?=$installDir"
    Assert-Installed
    Assert-Preserved
} finally {
    $programLock.Dispose()
    if (Test-Path -LiteralPath $temporaryUninstaller) { Remove-Item -LiteralPath $temporaryUninstaller }
}

Write-Host 'Checking repair replaces the executable and preserves data.'
Add-Fixture (Join-Path $installDir 'user-created.txt') 'Unrelated file in the install directory'
[IO.File]::WriteAllText($installedExe, 'Synthetic fixture for installer repair')
# Version 1.21.35 created this shortcut with --editor. Upgrade it in place.
$legacyShell = New-Object -ComObject WScript.Shell
try {
    $legacyLink = $legacyShell.CreateShortcut((Join-Path $menu 'Network Toolkit Text Editor.lnk'))
    try {
        $legacyLink.Arguments = '--editor'
        $legacyLink.Save()
    } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($legacyLink) }
} finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($legacyShell) }
Invoke-Silent $setup 0 '/S /DesktopShortcut'
Assert-Installed
Assert-Preserved
Assert-Same (Read-RegistryValue $uninstallKey 'DesktopShortcut') 1 'Selected desktop shortcut'

Write-Host 'Checking setup refuses to overwrite a newer executable.'
# An inbox PE has a real fixed version resource. It is copied only, never launched.
$newerExe = Join-Path $env:SystemRoot 'System32\cmd.exe'
if ((Get-ExecutableVersion $newerExe) -le $expectedVersion) { throw 'The downgrade fixture needs a newer executable version.' }
$newerHash = (Get-FileHash -LiteralPath $newerExe -Algorithm SHA256).Hash
$uninstallHash = (Get-FileHash -LiteralPath $uninstaller -Algorithm SHA256).Hash
Copy-Item -LiteralPath $newerExe -Destination $installedExe
Invoke-Silent $setup 2
Assert-Installed $newerHash
Assert-Same (Get-FileHash -LiteralPath $uninstaller -Algorithm SHA256).Hash $uninstallHash 'Uninstaller after refused downgrade'
Assert-Preserved
Copy-Item -LiteralPath $portableExe -Destination $installedExe

Write-Host 'Checking uninstall removes owned registrations and preserves data.'
Invoke-Uninstaller $uninstaller
Assert-UninstallFinished
foreach ($path in @($installedExe, $uninstaller, $menu, $desktopShortcut, $uninstallKey, "HKLM:\$uninstallSubkey", $capabilities, $progKey, $appKey)) {
    Assert-Absent $path
}
Assert-Same (Read-RegistryValue $registeredApps $registeredName) $null 'Removed Default Apps registration'
Assert-LinkRegistrationAbsent
foreach ($ext in $extensions) {
    Assert-Same (Read-RegistryValue "HKCU:\Software\Classes\$ext\OpenWithProgids" $progID) $null "Removed $ext Open With registration"
}
Assert-Preserved
Write-Host 'Checking custom location, optional desktop shortcut, and in-place upgrade.'
$defaultInstallDir = $installDir
$installDir = Join-Path $env:RUNNER_TEMP 'Custom Network Toolkit Install'
$installedExe = Join-Path $installDir 'Network_Toolkit.exe'
$uninstaller = Join-Path $installDir 'Uninstall.exe'
Invoke-Silent $setup 0 "/S /DesktopShortcut /D=$installDir"
Assert-Installed
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

# Re-running Setup with no location switch must reuse the registered custom folder.
Invoke-Silent $setup
Assert-Installed
Assert-Same (Read-RegistryValue $uninstallKey 'DesktopShortcut') 1 'Preserved desktop shortcut choice'

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
foreach ($path in @($installedExe, $uninstaller, $menu, $uninstallKey, $terminalKey, $linkProgKey, $appKey)) {
    Assert-Absent $path
}
Assert-Same (Read-RegistryValue $registeredApps $terminalRegisteredName) $null 'Removed terminal link registration'
if ($takenScheme) {
    Assert-Same (Get-RegistryTree "HKCU:\Software\Classes\$takenScheme") $takenTree "Another app's $takenScheme handler"
    [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKeyTree("Software\Classes\$takenScheme")
}
Assert-Preserved
Write-Host "Windows setup smoke passed for $version."
