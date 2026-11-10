# Installs the Network Toolkit default terminal test package for the signed-in
# user. Run it from the folder that holds the package, in your own (not an
# administrator) PowerShell window:
#   powershell -ExecutionPolicy Bypass -File .\Install-NetworkToolkitTerminalTest.ps1
$ErrorActionPreference = 'Stop'
$here = $PSScriptRoot
$package = Join-Path $here 'NetworkToolkitTerminalTest.msix'
$certificateFile = Join-Path $here 'NetworkToolkitTerminalTest.cer'
foreach ($file in $package, $certificateFile) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing $(Split-Path $file -Leaf). Extract the whole test package first." }
}
if ([Environment]::OSVersion.Version.Build -lt 22621) {
    throw 'This test needs Windows 11 version 22H2 or later.'
}
if (-not (Get-AppxPackage -Name Microsoft.WindowsTerminal)) {
    throw 'Install Windows Terminal from the Microsoft Store first. The test uses its console host.'
}

# Remember the current default terminal once, so the remove script can put it back.
$stateFolder = Join-Path $env:LOCALAPPDATA 'NetworkToolkitTerminalTest'
$previousFile = Join-Path $stateFolder 'previous-default-terminal.json'
[void](New-Item -ItemType Directory -Force -Path $stateFolder)
if (-not (Test-Path -LiteralPath $previousFile)) {
    $startup = Get-ItemProperty -LiteralPath 'HKCU:\Console\%%Startup' -ErrorAction SilentlyContinue
    [ordered]@{
        DelegationConsole = if ($startup) { $startup.DelegationConsole } else { $null }
        DelegationTerminal = if ($startup) { $startup.DelegationTerminal } else { $null }
    } | ConvertTo-Json | Set-Content -Encoding UTF8 -LiteralPath $previousFile
}

# Trust the test certificate on this PC only (Trusted People). Its signing key
# was created on a disposable build machine and deleted after signing.
$thumbprint = [Security.Cryptography.X509Certificates.X509Certificate2]::new($certificateFile).Thumbprint
$trusted = "Cert:\LocalMachine\TrustedPeople\$thumbprint"
if (-not (Test-Path -LiteralPath $trusted)) {
    Write-Host 'Windows will ask for permission to trust the test certificate on this PC.'
    $command = "Import-Certificate -FilePath '{0}' -CertStoreLocation Cert:\LocalMachine\TrustedPeople | Out-Null" -f $certificateFile.Replace("'", "''")
    $import = Start-Process powershell.exe -Verb RunAs -Wait -PassThru -ArgumentList @('-NoProfile', '-Command', $command)
    if ($import.ExitCode -ne 0 -or -not (Test-Path -LiteralPath $trusted)) { throw 'The test certificate was not trusted, so the package cannot be installed.' }
}

Add-AppxPackage -Path $package
Write-Host ''
Write-Host 'Installed. Next: open Settings > System > For developers > Terminal and choose'
Write-Host '"Network Toolkit Default Terminal Experiment". Then open Command Prompt and Windows PowerShell from Start.'
