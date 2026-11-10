param([string]$MicrosoftTerminalManifest)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ($env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted' -or
    $env:GITHUB_REPOSITORY -ne 'TechSavage1989/network-toolkit' -or
    $env:OS -ne 'Windows_NT' -or (Split-Path $PSScriptRoot -Leaf) -notlike 'NTK-DefaultTerminal-CI-*') {
    throw 'The terminal selection experiment requires a disposable GitHub-hosted Windows fixture.'
}
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [Security.Principal.WindowsPrincipal]::new($identity)
$groupOutput = & (Join-Path $env:SystemRoot 'System32\whoami.exe') /groups /fo csv /nh
if ($principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator) -or
    $identity.Name -notmatch '\\ntkci-[0-9a-f]{8}$' -or
    $LASTEXITCODE -ne 0 -or -not ($groupOutput -match '"S-1-16-8192"')) {
    throw 'Run the handoff proof only as the dedicated medium-integrity fixture user.'
}

# Secondary Logon loads the user's hive but Start-Process passes the runner
# environment. Resolve the profile from this process token's SID, independently
# of environment-backed known folder resolution.
$accountName = ($identity.Name -split '\\')[-1]
$profileListKey = 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList\' + $identity.User.Value
$workerProfile = [Environment]::ExpandEnvironmentVariables(
    (Get-ItemPropertyValue -LiteralPath $profileListKey -Name ProfileImagePath))
if (-not [IO.Path]::IsPathRooted($workerProfile) -or (Split-Path $workerProfile -Leaf) -ine $accountName -or
    -not (Test-Path -LiteralPath $workerProfile -PathType Container)) {
    throw 'The token SID did not identify the new standard-user fixture profile.'
}
$localData = Join-Path $workerProfile 'AppData\Local'
$roamingData = Join-Path $workerProfile 'AppData\Roaming'
[void][IO.Directory]::CreateDirectory($localData)
[void][IO.Directory]::CreateDirectory($roamingData)
$env:USERNAME = $accountName
$env:USERPROFILE = $workerProfile
$env:LOCALAPPDATA = $localData
$env:APPDATA = $roamingData
$env:TEMP = Join-Path $PSScriptRoot 'Temp'
$env:TMP = $env:TEMP

Add-Type -Path (Join-Path $PSScriptRoot 'InteractiveSmoke.cs')

$packageName = 'NetworkToolkit.DefaultTerminalExperiment'
$consoleClsid = '{2EACA947-7F5F-4CFA-BA87-8F7FBEEFBE69}'
$terminalClsid = '{B60E0829-5BF3-4A4A-ACB9-C661D85B7282}'
$startupKey = 'Console\%%Startup'
$results = [ordered]@{ fixture = 'Windows 11 ARM64 with x64 Network Toolkit'; installed = $false; shells = @() }
$brokerLog = $null
$savedSelection = @{}
$createdStartupKey = $false
$restoreSelection = $false
$processes = [Collections.Generic.List[Diagnostics.Process]]::new()

function Get-BrokerDiagnostics {
    if ($brokerLog -and (Test-Path -LiteralPath $brokerLog -PathType Leaf)) {
        Get-Content -LiteralPath $brokerLog -Tail 128 |
            Where-Object { $_ -match '^[A-Za-z0-9_.-]{1,64} 0x[0-9A-Fa-f]{8}$' }
    }
}

function Get-ExtensionCatalogEvidence([string]$Name, [string]$FamilyName) {
    try {
        # FindAll is available on Windows 11 24H2 and later. An unpackaged
        # worker may lack catalog access, so retain that result separately.
        $catalog = [Windows.ApplicationModel.AppExtensions.AppExtensionCatalog, Windows.ApplicationModel, ContentType=WindowsRuntime]::Open($Name)
        $entries = @($catalog.FindAll())
        [ordered]@{
            name = $Name
            accessible = $true
            fixtureListed = [bool]($entries | Where-Object { $_.Package.Id.FamilyName -ceq $FamilyName })
        }
    } catch {
        [ordered]@{ name = $Name; accessible = $false; error = $_.Exception.Message }
    }
}

function Get-ComActivationEvidence([string]$Clsid) {
    try {
        $hresult = [DefaultTerminalSmoke]::ProbeActivation($Clsid)
        [ordered]@{ activated = $hresult -ge 0; hresult = ('0x{0:X8}' -f $hresult) }
    } catch {
        [ordered]@{ activated = $false; error = $_.Exception.Message }
    }
}

function Stop-FixtureProcesses {
    foreach ($process in $processes) {
        try {
            if (-not $process.HasExited) { $process.Kill(); [void]$process.WaitForExit(5000) }
        } finally { $process.Dispose() }
    }
    $processes.Clear()
}

function Wait-ForFile([string]$File, [string]$Expected) {
    $until = [DateTime]::UtcNow.AddSeconds(20)
    do {
        if ((Test-Path -LiteralPath $File -PathType Leaf) -and (Get-Content -Raw -LiteralPath $File).Trim() -ceq $Expected) {
            return
        }
        Start-Sleep -Milliseconds 250
    } while ([DateTime]::UtcNow -lt $until)
    throw "The original shell did not write the expected fixture marker in $(Split-Path $File -Leaf)."
}

function Test-Shell([string]$Name, [string]$Executable, [string]$Arguments,
                   [string]$StartupMarker, [string]$InputCommand, [string]$ExpectedInput) {
    $before = @(Get-Process -Name Network_Toolkit -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Id)
    $fixtureShellPid = [DefaultTerminalSmoke]::LaunchNewConsole($Executable, $Arguments, $PSScriptRoot)
    $shell = [Diagnostics.Process]::GetProcessById($fixtureShellPid)
    $processes.Add($shell)
    $result = [ordered]@{ shell = $Name; shellPid = $fixtureShellPid; window = $false; input = $false }
    $results.shells += $result

    Wait-ForFile (Join-Path $PSScriptRoot "$Name-startup.txt") $StartupMarker
    $result.startupMarker = $true
    $until = [DateTime]::UtcNow.AddSeconds(50)
    $window = [IntPtr]::Zero
    do {
        foreach ($candidate in @(Get-Process -Name Network_Toolkit -ErrorAction SilentlyContinue)) {
            if ($candidate.Id -in $before -or $candidate.Path -ine $installedExecutable) { continue }
            $window = [DefaultTerminalSmoke]::FindWindow($candidate.Id)
            if ($window -ne [IntPtr]::Zero) {
                $hostProcess = $candidate
                break
            }
        }
        if ($window -ne [IntPtr]::Zero) { break }
        Start-Sleep -Milliseconds 250
    } while ([DateTime]::UtcNow -lt $until)
    if ($window -eq [IntPtr]::Zero) {
        throw "Windows did not route $Name to a visible Network Toolkit experiment window."
    }
    $processes.Add($hostProcess)
    $result.hostPid = $hostProcess.Id
    $result.window = $true
    [DefaultTerminalSmoke]::SendCommand($window, $hostProcess.Id, $InputCommand)
    Wait-ForFile (Join-Path $PSScriptRoot "$Name-input.txt") ($ExpectedInput.Replace('%PID%', [string]$fixtureShellPid))
    $result.input = $true
    [DefaultTerminalSmoke]::SendCommand($window, $hostProcess.Id, 'exit')
    if (-not $shell.WaitForExit(15000) -or -not $hostProcess.WaitForExit(15000)) {
        throw "The delegated $Name shell or its Network Toolkit host did not close."
    }
    $result.closed = $true
    Write-Host "Windows selected Network Toolkit for $Name, and the original shell accepted input."
}

try {
    $officialTerminal = Get-AppxPackage -Name Microsoft.WindowsTerminal
    if (-not $officialTerminal) {
        if (-not (Test-Path -LiteralPath $MicrosoftTerminalManifest -PathType Leaf)) {
            throw 'The hosted image does not have Microsoft Windows Terminal Stable available for the fixture.'
        }
        Add-AppxPackage -Register -DisableDevelopmentMode -Path $MicrosoftTerminalManifest
        $officialTerminal = Get-AppxPackage -Name Microsoft.WindowsTerminal
    }
    if ($officialTerminal.PackageFamilyName -cne 'Microsoft.WindowsTerminal_8wekyb3d8bbwe') {
        throw 'The registered console host does not have the expected Microsoft package identity.'
    }
    $results.microsoftTerminalVersion = [string]$officialTerminal.Version

    if (Get-AppxPackage -Name $packageName) { throw 'The fixture package already exists in this test profile.' }
    Add-AppxPackage -Path (Join-Path $PSScriptRoot 'DefaultTerminalExperiment.msix')
    $installed = Get-AppxPackage -Name $packageName
    if (-not $installed -or $installed.Architecture -ne 'Arm64') { throw 'The native ARM64 fixture MSIX did not register.' }
    $results.installed = $true
    $results.fixturePackageVersion = [string]$installed.Version
    $results.sessionId = [Diagnostics.Process]::GetCurrentProcess().SessionId
    try { $results.desktopHasInput = [DefaultTerminalSmoke]::DesktopHasInput() }
    catch { $results.desktopInputProbeError = $_.Exception.Message }
    $installedExecutable = Join-Path $installed.InstallLocation 'Network_Toolkit.exe'
    $brokerLog = Join-Path $localData "Packages\$($installed.PackageFamilyName)\LocalState\broker.log"
    $manifest = Get-AppxPackageManifest -Package $installed.PackageFullName
    foreach ($contract in @(
        @{ Name = 'com.microsoft.windows.console.host'; Clsid = $consoleClsid },
        @{ Name = 'com.microsoft.windows.terminal.host'; Clsid = $terminalClsid }
    )) {
        $node = $manifest.SelectSingleNode("//*[local-name()='AppExtension' and @Name='$($contract.Name)']/*[local-name()='Properties']/*[local-name()='Clsid']")
        if (-not $node -or $node.InnerText -cne $contract.Clsid) { throw "Registered fixture lost the $($contract.Name) extension." }
    }
    $results.extensionCatalogs = @(
        Get-ExtensionCatalogEvidence 'com.microsoft.windows.console.host' $installed.PackageFamilyName
        Get-ExtensionCatalogEvidence 'com.microsoft.windows.terminal.host' $installed.PackageFamilyName
    )

    # Model the choice written by Windows Settings, in this fixture profile only.
    # Native CreateProcess then exercises the real OS console handoff.
    $current = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey($startupKey)
    $createdStartupKey = $null -eq $current
    if ($current) {
        try {
            foreach ($name in 'DelegationConsole', 'DelegationTerminal') {
                if ($current.GetValueNames() -contains $name) {
                    $savedSelection[$name] = @($current.GetValue($name), $current.GetValueKind($name))
                }
            }
        } finally { $current.Dispose() }
    }
    $restoreSelection = $true
    $current = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($startupKey)
    try {
        $current.SetValue('DelegationConsole', $consoleClsid)
        $current.SetValue('DelegationTerminal', $terminalClsid)
    } finally { $current.Dispose() }

    $nonce = [Guid]::NewGuid().ToString('N')
    $errors = @()
    $cmdStart = '/D /Q /K "set NTK_HANDOFF_NONCE={0}&&echo {0}>cmd-startup.txt"' -f $nonce
    try {
        Test-Shell 'cmd' (Join-Path $env:SystemRoot 'System32\cmd.exe') $cmdStart $nonce 'echo %NTK_HANDOFF_NONCE%>cmd-input.txt' $nonce
    } catch {
        $errors += $_.Exception.Message
    } finally { Stop-FixtureProcesses }

    $ps = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
    $psStart = '-NoLogo -NoProfile -NoExit -Command "$env:NTK_HANDOFF_NONCE = ''{0}''; Set-Content -Encoding Ascii ps-startup.txt ''{0}''"' -f $nonce
    $psInput = 'Set-Content -Encoding Ascii ps-input.txt "$PID|$env:NTK_HANDOFF_NONCE"'
    try {
        Test-Shell 'ps' $ps $psStart $nonce $psInput "%PID%|$nonce"
    } catch {
        $errors += $_.Exception.Message
    } finally { Stop-FixtureProcesses }

    # Probe COM independently after native launches so this activation cannot
    # make the real shell test appear to have started the broker.
    $results.brokerBeforeDirectProbe = @(Get-BrokerDiagnostics)
    $results.comActivation = Get-ComActivationEvidence $terminalClsid
    $results.brokerAfterDirectProbe = @(Get-BrokerDiagnostics)

    if ($errors.Count -ne 0) {
        $results.errors = $errors
        throw ($errors -join [Environment]::NewLine)
    }
} finally {
    try {
        if ($restoreSelection) {
            $current = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey($startupKey, $true)
            try {
                foreach ($name in 'DelegationConsole', 'DelegationTerminal') {
                    if ($savedSelection.ContainsKey($name)) {
                        $current.SetValue($name, $savedSelection[$name][0], $savedSelection[$name][1])
                    } else {
                        $current.DeleteValue($name, $false)
                    }
                }
                $removeEmpty = $createdStartupKey -and $current.ValueCount -eq 0 -and $current.SubKeyCount -eq 0
            } finally { $current.Dispose() }
            if ($removeEmpty) { [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKey($startupKey, $false) }
            $results.selectionRestored = $true
        }
    } finally {
        try {
            if ($results.installed) {
                Get-AppxPackage -Name $packageName | Remove-AppxPackage
                $results.packageRemoved = $true
            }
        } finally {
            $results | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $PSScriptRoot 'evidence.json')
        }
    }
}
