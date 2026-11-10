$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
# Builds the side-loaded x64 test package the owner installs on their own
# Windows 11 PC to check Network Toolkit as the default terminal. Runs only on
# a disposable GitHub-hosted x64 runner: the signing key is created there, is
# never exported, and is deleted with the runner. Only the public certificate
# leaves the runner, so nobody can sign anything else with it.
if ($env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted' -or
    $env:GITHUB_REPOSITORY -ne 'TechSavage1989/network-toolkit' -or -not $IsWindows -or
    [Runtime.InteropServices.RuntimeInformation]::OSArchitecture -ne 'X64') {
    throw 'Build the default terminal test package only on a disposable GitHub-hosted x64 Windows runner.'
}

function Get-SdkTool([string]$Name) {
    $sdkRoot = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10\bin'
    foreach ($sdk in @(Get-ChildItem -LiteralPath $sdkRoot -Directory |
                       Where-Object Name -Match '^\d+\.\d+\.\d+\.\d+$' |
                       Sort-Object { [version]$_.Name } -Descending)) {
        $tool = Join-Path $sdk.FullName "x64\$Name"
        if (Test-Path -LiteralPath $tool -PathType Leaf) { return $tool }
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

$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$scratch = Join-Path $env:RUNNER_TEMP 'ntk-terminal-test-package'
$stage = Join-Path $scratch 'stage'
$output = Join-Path $env:RUNNER_TEMP 'NetworkToolkitTerminalTest'
$certificate = $null
try {
    [void](New-Item -ItemType Directory -Force -Path $scratch, $stage, (Join-Path $stage 'Assets'), (Join-Path $stage 'Public'), $output)
    $manifestPath = Join-Path $stage 'AppxManifest.xml'
    $manifestText = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'DefaultTerminalExperiment.appxmanifest')
    if ($manifestText -notmatch 'ProcessorArchitecture="arm64"') { throw 'The experiment manifest changed; review the x64 test package build.' }
    $manifestText = $manifestText.Replace('ProcessorArchitecture="arm64"', 'ProcessorArchitecture="x64"').
        Replace('Description="Hosted Windows console handoff fixture"', 'Description="Side-loaded test of Network Toolkit as the Windows default terminal"')
    Set-Content -Encoding utf8 -LiteralPath $manifestPath -Value $manifestText
    Write-PackageLogos (Join-Path $root 'assets\Network_Toolkit_Icon.png') (Join-Path $stage 'Assets')
    'Network Toolkit default terminal test package.' | Set-Content -Encoding ascii -LiteralPath (Join-Path $stage 'Public\fixture.txt')

    $vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio\Installer\vswhere.exe'
    $visualStudio = & $vswhere -latest -products '*' -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath
    if ($LASTEXITCODE -ne 0 -or -not $visualStudio) { throw 'Visual Studio x64 C++ tools are required.' }
    $devCmd = Join-Path $visualStudio 'Common7\Tools\VsDevCmd.bat'
    $broker = Join-Path $stage 'NetworkToolkitTerminalHandoff.exe'
    $cpp = Join-Path $PSScriptRoot 'ConsoleHandoff.cpp'
    $buildCmd = Join-Path $scratch 'build-broker.cmd'
    @(
        '@echo off',
        ('call "{0}" -arch=amd64 -host_arch=amd64' -f $devCmd),
        'if errorlevel 1 exit /b %errorlevel%',
        ('cd /d "{0}"' -f $scratch),
        ('cl.exe /nologo /W4 /WX /EHsc /std:c++17 /O2 /MT /DUNICODE /D_UNICODE /DWIN32_LEAN_AND_MEAN /DNOMINMAX /DNTK_HANDOFF_EXPERIMENT_DIAGNOSTICS /DNTK_HANDOFF_TEST_PACKAGE "{0}" /link /SUBSYSTEM:WINDOWS /OUT:"{1}" ole32.lib advapi32.lib' -f $cpp, $broker),
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

    $package = Join-Path $output 'NetworkToolkitTerminalTest.msix'
    Invoke-Checked (Get-SdkTool 'makeappx.exe') @('pack', '/d', $stage, '/p', $package, '/o')
    [xml]$manifest = Get-Content -Raw -LiteralPath $manifestPath
    # Valid for 30 days: the package must be installed while the certificate is valid.
    $certificate = New-SelfSignedCertificate -Type CodeSigningCert -Subject $manifest.Package.Identity.Publisher `
        -CertStoreLocation 'Cert:\CurrentUser\My' -KeyAlgorithm RSA -KeyLength 2048 -KeyExportPolicy NonExportable `
        -NotAfter ([DateTime]::Now.AddDays(30))
    $signtool = Get-SdkTool 'signtool.exe'
    Invoke-Checked $signtool @('sign', '/fd', 'SHA256', '/sha1', $certificate.Thumbprint, $package)
    $certPath = Join-Path $output 'NetworkToolkitTerminalTest.cer'
    Export-Certificate -Cert $certificate -FilePath $certPath | Out-Null

    foreach ($file in 'Install-NetworkToolkitTerminalTest.ps1', 'Remove-NetworkToolkitTerminalTest.ps1', 'README.txt') {
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot "test-package\$file") -Destination $output
    }
    $sums = Get-ChildItem -LiteralPath $output -File | Sort-Object Name | ForEach-Object {
        '{0}  {1}' -f (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash.ToLowerInvariant(), $_.Name
    }
    $sums | Set-Content -Encoding ascii -LiteralPath (Join-Path $output 'SHA256SUMS.txt')
    Write-Host '----- test package files -----'
    $sums | ForEach-Object { Write-Host $_ }
    Write-Host ('Certificate thumbprint: {0}; valid until {1:yyyy-MM-dd}' -f $certificate.Thumbprint, $certificate.NotAfter)
} finally {
    if ($certificate) {
        $leaf = Join-Path 'Cert:\CurrentUser\My' $certificate.Thumbprint
        if (Test-Path -LiteralPath $leaf) { Remove-Item -LiteralPath $leaf -DeleteKey }
    }
}
