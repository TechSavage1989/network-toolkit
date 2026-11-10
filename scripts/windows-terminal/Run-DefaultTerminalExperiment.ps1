$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ($env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted' -or
    $env:GITHUB_REPOSITORY -ne 'TechSavage1989/network-toolkit' -or -not $IsWindows -or
    [Runtime.InteropServices.RuntimeInformation]::OSArchitecture -ne 'Arm64' -or
    [Environment]::OSVersion.Version.Build -lt 22000 -or
    (Get-CimInstance Win32_OperatingSystem).ProductType -ne 1) {
    throw 'Build and test this package only on the disposable GitHub-hosted Windows 11 ARM64 job.'
}

function Get-SdkTool([string]$Name) {
    $sdkRoot = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10\bin'
    foreach ($sdk in @(Get-ChildItem -LiteralPath $sdkRoot -Directory |
                       Where-Object Name -Match '^\d+\.\d+\.\d+\.\d+$' |
                       Sort-Object { [version]$_.Name } -Descending)) {
        foreach ($arch in 'arm64', 'x64') {
            $tool = Join-Path $sdk.FullName "$arch\$Name"
            if (Test-Path -LiteralPath $tool -PathType Leaf) { return $tool }
        }
    }
    throw "No Windows SDK copy of $Name is available."
}

function Invoke-Checked([string]$Program, [string[]]$Arguments) {
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$(Split-Path $Program -Leaf) failed with exit code $LASTEXITCODE." }
}

function Write-PackageLogos([string]$Source, [string]$Assets) {
    Add-Type -AssemblyName System.Drawing.Common
    $icon = [Drawing.Image]::FromFile($Source)
    try {
        foreach ($logo in @(
            @{ Name = 'StoreLogo'; Size = 50 },
            @{ Name = 'Square44x44Logo'; Size = 44 },
            @{ Name = 'Square150x150Logo'; Size = 150 }
        )) {
            $image = [Drawing.Bitmap]::new($logo.Size, $logo.Size)
            try {
                $drawing = [Drawing.Graphics]::FromImage($image)
                try { $drawing.DrawImage($icon, 0, 0, $logo.Size, $logo.Size) }
                finally { $drawing.Dispose() }
                $image.Save((Join-Path $Assets "$($logo.Name).png"), [Drawing.Imaging.ImageFormat]::Png)
            } finally { $image.Dispose() }
        }
    } finally { $icon.Dispose() }
}

$nonce = [Guid]::NewGuid().ToString('N').Substring(0, 8)
$scratch = Join-Path $env:RUNNER_TEMP "ntk-default-terminal-$nonce"
$stage = Join-Path $scratch 'stage'
$workerDirectory = Join-Path $env:ProgramData "NTK-DefaultTerminal-CI-$nonce"
$resultPath = Join-Path $env:RUNNER_TEMP 'ntk-default-terminal-evidence.json'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$certificate = $null
$taskName = $null
$seen = $null
$experimentStart = $null
try {
    $workerTemp = Join-Path $workerDirectory 'Temp'
    [void](New-Item -ItemType Directory -Path $scratch, $stage, (Join-Path $stage 'Assets'), (Join-Path $stage 'Public'), $workerDirectory, $workerTemp)
    $manifestPath = Join-Path $stage 'AppxManifest.xml'
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'DefaultTerminalExperiment.appxmanifest') -Destination $manifestPath
    Write-PackageLogos (Join-Path $root 'assets\Network_Toolkit_Icon.png') (Join-Path $stage 'Assets')
    'Disposable console handoff fixture.' | Set-Content -Encoding ascii -LiteralPath (Join-Path $stage 'Public\fixture.txt')

    $vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio\Installer\vswhere.exe'
    $visualStudio = & $vswhere -latest -products '*' -requires Microsoft.VisualStudio.Component.VC.Tools.ARM64 -property installationPath
    if ($LASTEXITCODE -ne 0 -or -not $visualStudio) { throw 'Visual Studio ARM64 C++ tools are required.' }
    $devCmd = Join-Path $visualStudio 'Common7\Tools\VsDevCmd.bat'
    $broker = Join-Path $stage 'NetworkToolkitTerminalHandoff.exe'
    $cpp = Join-Path $PSScriptRoot 'ConsoleHandoff.cpp'
    $buildCmd = Join-Path $scratch 'build-broker.cmd'
    @(
        '@echo off',
        ('call "{0}" -arch=arm64 -host_arch=arm64' -f $devCmd),
        'if errorlevel 1 exit /b %errorlevel%',
        ('cd /d "{0}"' -f $scratch),
        ('cl.exe /nologo /W4 /WX /EHsc /std:c++17 /O2 /MT /DUNICODE /D_UNICODE /DWIN32_LEAN_AND_MEAN /DNOMINMAX /DNTK_HANDOFF_EXPERIMENT_DIAGNOSTICS "{0}" /link /SUBSYSTEM:WINDOWS /OUT:"{1}" ole32.lib advapi32.lib' -f $cpp, $broker),
        'exit /b %errorlevel%'
    ) | Set-Content -Encoding ascii -LiteralPath $buildCmd
    Invoke-Checked $env:ComSpec @('/d', '/c', $buildCmd)

    Push-Location $root
    try {
        $env:GOOS = 'windows'
        $env:GOARCH = 'amd64'
        $env:CGO_ENABLED = '0'
        Invoke-Checked 'go' @('build', '-trimpath', '-mod=vendor', '-tags', 'toolkit_terminal_handoff',
            '-ldflags', '-H=windowsgui', '-o', (Join-Path $stage 'Network_Toolkit.exe'), './cmd/network-toolkit')
    } finally { Pop-Location }

    $package = Join-Path $workerDirectory 'DefaultTerminalExperiment.msix'
    Invoke-Checked (Get-SdkTool 'makeappx.exe') @('pack', '/d', $stage, '/p', $package, '/o')
    [xml]$manifest = Get-Content -Raw -LiteralPath $manifestPath
    $certificateArgs = @{
        Type = 'CodeSigningCert'
        Subject = $manifest.Package.Identity.Publisher
        CertStoreLocation = 'Cert:\CurrentUser\My'
        KeyAlgorithm = 'RSA'
        KeyLength = 2048
        KeyExportPolicy = 'NonExportable'
        NotAfter = [DateTime]::Now.AddDays(1)
    }
    $certificate = New-SelfSignedCertificate @certificateArgs
    $signtool = Get-SdkTool 'signtool.exe'
    Invoke-Checked $signtool @('sign', '/fd', 'SHA256', '/sha1', $certificate.Thumbprint, $package)
    $certPath = Join-Path $scratch 'fixture.cer'
    Export-Certificate -Cert $certificate -FilePath $certPath | Out-Null
    # Only the fixture's short-lived leaf is trusted, on the disposable runner.
    Import-Certificate -FilePath $certPath -CertStoreLocation 'Cert:\LocalMachine\TrustedPeople' | Out-Null
    Invoke-Checked $signtool @('verify', '/pa', $package)

    Import-Module Appx -UseWindowsPowerShell -WarningAction SilentlyContinue
    $official = Get-AppxPackage -AllUsers -Name Microsoft.WindowsTerminal |
        Where-Object { $_.PackageFamilyName -ceq 'Microsoft.WindowsTerminal_8wekyb3d8bbwe' -and $_.Architecture -eq 'Arm64' } |
        Sort-Object { [version]$_.Version } -Descending | Select-Object -First 1
    if (-not $official) { throw 'The runner does not include the native Microsoft Windows Terminal Stable package.' }
    $officialManifest = Join-Path $official.InstallLocation 'AppxManifest.xml'

    foreach ($file in 'Test-DefaultTerminalExperiment.ps1', 'InteractiveSmoke.cs') {
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot $file) -Destination $workerDirectory
    }
    $workerFile = Join-Path $workerDirectory 'Test-DefaultTerminalExperiment.ps1'
    $stdout = Join-Path $workerDirectory 'worker-output.txt'
    $stderr = Join-Path $workerDirectory 'worker-error.txt'
    $exitFile = Join-Path $workerDirectory 'worker-exit.txt'
    # Windows does not hand an elevated console to another terminal, and the
    # job runs elevated (run 42746869734: integrity S-1-16-12288, no
    # OpenConsole). Run the proof as the session owner's limited token through
    # an interactive scheduled task, as an ordinary unelevated user does.
    $me = [Security.Principal.WindowsIdentity]::GetCurrent()
    $acl = Get-Acl -LiteralPath $workerDirectory
    $acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($me.User, 'Modify',
        'ContainerInherit,ObjectInherit', 'None', 'Allow'))
    Set-Acl -LiteralPath $workerDirectory -AclObject $acl
    $windowsPowerShell = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
    $wrapper = Join-Path $workerDirectory 'run-worker.cmd'
    @(
        '@echo off',
        'set GITHUB_ACTIONS=true',
        'set RUNNER_ENVIRONMENT=github-hosted',
        ('set GITHUB_REPOSITORY={0}' -f $env:GITHUB_REPOSITORY),
        ('set TEMP={0}' -f $workerTemp),
        ('set TMP={0}' -f $workerTemp),
        ('cd /d "{0}"' -f $workerDirectory),
        ('"{0}" -NoLogo -NoProfile -NonInteractive -ExecutionPolicy RemoteSigned -File "{1}" -MicrosoftTerminalManifest "{2}" > "{3}" 2> "{4}"' -f
            $windowsPowerShell, $workerFile, $officialManifest, $stdout, $stderr),
        ('echo %errorlevel%> "{0}"' -f $exitFile)
    ) | Set-Content -Encoding ascii -LiteralPath $wrapper
    $taskName = "NTK-DefaultTerminal-$nonce"
    $taskPrincipal = New-ScheduledTaskPrincipal -UserId $me.Name -LogonType Interactive -RunLevel Limited
    $taskAction = New-ScheduledTaskAction -Execute $env:ComSpec -Argument ('/d /c "{0}"' -f $wrapper) -WorkingDirectory $workerDirectory
    $taskSettings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit (New-TimeSpan -Minutes 6) -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
    Register-ScheduledTask -TaskName $taskName -Action $taskAction -Principal $taskPrincipal -Settings $taskSettings | Out-Null
    $experimentStart = [DateTime]::Now
    Start-ScheduledTask -TaskName $taskName
    # Sample console and broker processes while the worker runs. The artifact
    # store is not reachable from every reviewer, so this goes to the job log.
    $watched = 'conhost', 'OpenConsole', 'WindowsTerminal', 'NetworkToolkitTerminalHandoff', 'Network_Toolkit', 'cmd', 'powershell'
    $seen = [ordered]@{}
    $deadline = [DateTime]::UtcNow.AddMilliseconds(300000)
    while (-not (Test-Path -LiteralPath $exitFile -PathType Leaf)) {
        Start-Sleep -Milliseconds 500
        if ([DateTime]::UtcNow -ge $deadline) { throw 'The console handoff fixture timed out.' }
        try { $samples = @(Get-Process -Name $watched -IncludeUserName -ErrorAction SilentlyContinue) }
        catch { $samples = @(Get-Process -Name $watched -ErrorAction SilentlyContinue) }
        foreach ($sample in $samples) {
            $key = '{0}:{1}' -f $sample.ProcessName, $sample.Id
            if ($seen.Contains($key)) { continue }
            $commandLine = (Get-CimInstance Win32_Process -Filter "ProcessId=$($sample.Id)" -ErrorAction SilentlyContinue).CommandLine
            $parent = (Get-CimInstance Win32_Process -Filter "ProcessId=$($sample.Id)" -ErrorAction SilentlyContinue).ParentProcessId
            $user = if ($sample.PSObject.Properties['UserName']) { $sample.UserName } else { '?' }
            $seen[$key] = '{0:HH:mm:ss.fff} {1} pid={2} parent={3} session={4} user={5} cmd={6}' -f [DateTime]::Now,
                $sample.ProcessName, $sample.Id, $parent, $sample.SessionId, $user, $commandLine
        }
    }
    Start-Sleep -Seconds 1
    if (Test-Path -LiteralPath $stdout) { Get-Content -LiteralPath $stdout }
    if (Test-Path -LiteralPath $stderr) { Get-Content -LiteralPath $stderr }
    $workerExit = (Get-Content -Raw -LiteralPath $exitFile).Trim()
    if ($workerExit -ne '0') { throw "The console handoff fixture failed with exit code $workerExit." }
} finally {
    if ($taskName) {
        Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
    }
    $evidence = Join-Path $workerDirectory 'evidence.json'
    if (Test-Path -LiteralPath $evidence -PathType Leaf) {
        Copy-Item -LiteralPath $evidence -Destination $resultPath -Force
        Write-Host '----- evidence.json -----'
        Get-Content -Raw -LiteralPath $evidence | Write-Host
    }
    if ($seen) {
        Write-Host '----- processes started while the fixture ran -----'
        foreach ($line in $seen.Values) { Write-Host $line }
    }
    if ($experimentStart) {
        Write-Host '----- Windows events while the fixture ran (warnings and errors) -----'
        foreach ($log in 'System', 'Application', 'Microsoft-Windows-AppModel-Runtime/Admin',
                         'Microsoft-Windows-AppXDeploymentServer/Operational', 'Microsoft-Windows-TWinUI/Operational') {
            try {
                Get-WinEvent -FilterHashtable @{ LogName = $log; StartTime = $experimentStart; Level = 1, 2, 3 } -MaxEvents 40 -ErrorAction Stop |
                    Sort-Object TimeCreated | ForEach-Object {
                        Write-Host ('{0:HH:mm:ss} [{1}] {2} {3}: {4}' -f $_.TimeCreated, $log, $_.ProviderName, $_.Id,
                            (($_.Message -replace '\s+', ' ') -replace '^(.{0,400}).*$', '$1'))
                    }
            } catch { Write-Host "[$log] $($_.Exception.Message)" }
        }
    }
    try {
        # The experiment runs as the session owner; nothing else to remove here.
    } finally {
        if ($certificate) {
            foreach ($store in 'Cert:\LocalMachine\TrustedPeople', 'Cert:\CurrentUser\My') {
                $leaf = Join-Path $store $certificate.Thumbprint
                if (Test-Path -LiteralPath $leaf) { Remove-Item -LiteralPath $leaf }
            }
        }
    }
}
