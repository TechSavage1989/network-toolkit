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
    'Network Toolkit Text Editor.lnk' = '--editor'
    'Network Toolkit Terminal.lnk' = '--terminal'
}

foreach ($path in @($installDir, $dataDir, $menu, $uninstallKey, "HKLM:\$uninstallSubkey", $appKey, $progKey)) {
    Assert-Absent $path
}
Assert-Same (Read-RegistryValue $registeredApps $registeredName) $null 'Preexisting app registration'

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
    if (-not (Read-RegistryValue $capabilities 'ApplicationDescription')) { throw 'The Default Apps description is missing.' }
    Assert-Same (Read-RegistryValue "$progKey\shell\open\command" '') ('"{0}" --open-file "%1"' -f $installedExe) 'Editor open command'
    $editorClass = Get-Item -LiteralPath $progKey
    try {
        Assert-Same ($editorClass.GetValueKind('AllowSilentDefaultTakeOver')) ([Microsoft.Win32.RegistryValueKind]::None) 'Default takeover opt-out'
    } finally { $editorClass.Dispose() }
    foreach ($ext in $extensions) {
        Assert-Same (Read-RegistryValue "$capabilities\FileAssociations" $ext) $progID "$ext Default Apps registration"
        Assert-Same (Read-RegistryValue "HKCU:\Software\Classes\$ext\OpenWithProgids" $progID) '' "$ext Open With registration"
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
            } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($link) }
        }
    } finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($shell) }
}

New-Item -Path $dataDir -ItemType Directory | Out-Null
Add-Fixture (Join-Path $dataDir 'Network_Toolkit.settings.json') '{"theme":"dark"}'
Add-Fixture (Join-Path $dataDir 'Network_Toolkit.macros.json') '{"syntheticFixture":true}'
Add-Fixture (Join-Path $dataDir 'Network_Toolkit.credentials.v2.vault') 'Synthetic installer preservation fixture'

Write-Host 'Checking silent install, shortcuts, and editor registration.'
Invoke-Silent $setup
Assert-Installed
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
Invoke-Silent $setup
Assert-Installed
Assert-Preserved

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
Invoke-Silent $uninstaller
foreach ($path in @($installedExe, $uninstaller, $menu, $uninstallKey, "HKLM:\$uninstallSubkey", $capabilities, $progKey)) {
    Assert-Absent $path
}
Assert-Same (Read-RegistryValue $registeredApps $registeredName) $null 'Removed Default Apps registration'
foreach ($ext in $extensions) {
    Assert-Same (Read-RegistryValue "HKCU:\Software\Classes\$ext\OpenWithProgids" $progID) $null "Removed $ext Open With registration"
}
Assert-Preserved
Write-Host "Windows setup smoke passed for $version."
