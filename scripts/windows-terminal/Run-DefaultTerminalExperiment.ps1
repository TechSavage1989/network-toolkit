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
$username = "ntkci-$nonce"
$certificate = $null
$account = $null
$worker = $null
try {
    [void](New-Item -ItemType Directory -Path $scratch, $stage, (Join-Path $stage 'Assets'), (Join-Path $stage 'Public'), $workerDirectory)
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
        ('cl.exe /nologo /W4 /WX /EHsc /std:c++17 /O2 /MT /DUNICODE /D_UNICODE /DWIN32_LEAN_AND_MEAN /DNOMINMAX "{0}" /link /SUBSYSTEM:WINDOWS /OUT:"{1}" ole32.lib advapi32.lib' -f $cpp, $broker),
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

    $official = Get-AppxPackage -AllUsers -Name Microsoft.WindowsTerminal |
        Where-Object { $_.PackageFamilyName -ceq 'Microsoft.WindowsTerminal_8wekyb3d8bbwe' -and $_.Architecture -eq 'Arm64' } |
        Sort-Object { [version]$_.Version } -Descending | Select-Object -First 1
    if (-not $official) { throw 'The runner does not include the native Microsoft Windows Terminal Stable package.' }
    $officialManifest = Join-Path $official.InstallLocation 'AppxManifest.xml'

    foreach ($file in 'Test-DefaultTerminalExperiment.ps1', 'InteractiveSmoke.cs') {
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot $file) -Destination $workerDirectory
    }
    $password = ConvertTo-SecureString ("NtK!" + [Guid]::NewGuid().ToString('N')) -AsPlainText -Force
    $account = New-LocalUser -Name $username -Password $password -Description 'Disposable Network Toolkit console fixture'
    Add-LocalGroupMember -SID 'S-1-5-32-545' -Member $account
    $acl = Get-Acl -LiteralPath $workerDirectory
    $acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($account.SID, 'Modify',
        'ContainerInherit,ObjectInherit', 'None', 'Allow'))
    Set-Acl -LiteralPath $workerDirectory -AclObject $acl
    $workerFile = Join-Path $workerDirectory 'Test-DefaultTerminalExperiment.ps1'
    $stdout = Join-Path $scratch 'worker-output.txt'
    $stderr = Join-Path $scratch 'worker-error.txt'
    $startArgs = @{
        FilePath = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
        ArgumentList = '-NoLogo -NoProfile -NonInteractive -File "{0}" -MicrosoftTerminalManifest "{1}"' -f $workerFile, $officialManifest
        Credential = [PSCredential]::new(".\$username", $password)
        LoadUserProfile = $true
        WorkingDirectory = $workerDirectory
        PassThru = $true
        Environment = @{ GITHUB_ACTIONS = 'true'; RUNNER_ENVIRONMENT = 'github-hosted'; GITHUB_REPOSITORY = $env:GITHUB_REPOSITORY }
        RedirectStandardOutput = $stdout
        RedirectStandardError = $stderr
    }
    $worker = Start-Process @startArgs
    if (-not $worker.WaitForExit(180000)) { throw 'The standard-user console handoff fixture timed out.' }
    Get-Content -LiteralPath $stdout
    Get-Content -LiteralPath $stderr
    if ($worker.ExitCode -ne 0) { throw "The standard-user console handoff fixture failed with exit code $($worker.ExitCode)." }
} finally {
    if ($worker) {
        try {
            if (-not $worker.HasExited) { $worker.Kill(); [void]$worker.WaitForExit(5000) }
        } finally { $worker.Dispose() }
    }
    $evidence = Join-Path $workerDirectory 'evidence.json'
    if (Test-Path -LiteralPath $evidence -PathType Leaf) {
        Copy-Item -LiteralPath $evidence -Destination $resultPath -Force
    }
    try {
        if ($account) { Remove-LocalUser -SID $account.SID }
    } finally {
        if ($certificate) {
            foreach ($store in 'Cert:\LocalMachine\TrustedPeople', 'Cert:\CurrentUser\My') {
                $leaf = Join-Path $store $certificate.Thumbprint
                if (Test-Path -LiteralPath $leaf) { Remove-Item -LiteralPath $leaf }
            }
        }
    }
}
