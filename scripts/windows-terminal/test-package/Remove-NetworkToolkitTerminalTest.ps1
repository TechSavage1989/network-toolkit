# Undoes the Network Toolkit default terminal test: restores your previous
# default terminal, prints the test log, removes the package and stops
# trusting the test certificate. Run it from the test package folder:
#   powershell -ExecutionPolicy Bypass -File .\Remove-NetworkToolkitTerminalTest.ps1
$ErrorActionPreference = 'Stop'
$here = $PSScriptRoot
$terminalClass = '{B60E0829-5BF3-4A4A-ACB9-C661D85B7282}'
$startupKey = 'HKCU:\Console\%%Startup'
$stateFolder = Join-Path $env:LOCALAPPDATA 'NetworkToolkitTerminalTest'
$previousFile = Join-Path $stateFolder 'previous-default-terminal.json'

$current = Get-ItemProperty -LiteralPath $startupKey -ErrorAction SilentlyContinue
if ($current -and $current.DelegationTerminal -eq $terminalClass) {
    $previous = if (Test-Path -LiteralPath $previousFile) { Get-Content -Raw -LiteralPath $previousFile | ConvertFrom-Json } else { $null }
    foreach ($name in 'DelegationConsole', 'DelegationTerminal') {
        $value = if ($previous) { $previous.$name } else { $null }
        if ($value) { Set-ItemProperty -LiteralPath $startupKey -Name $name -Value $value }
        else { Remove-ItemProperty -LiteralPath $startupKey -Name $name -ErrorAction SilentlyContinue }
    }
    Write-Host 'Your previous default terminal is selected again.'
}

$installed = Get-AppxPackage -Name 'NetworkToolkit.DefaultTerminalExperiment'
if ($installed) {
    $log = Join-Path $env:LOCALAPPDATA "Packages\$($installed.PackageFamilyName)\LocalState\broker.log"
    if (Test-Path -LiteralPath $log) {
        Write-Host ''
        Write-Host '----- Test log (send this to Claude if the test did not work) -----'
        Get-Content -LiteralPath $log -Tail 80
        Write-Host '----- end of test log -----'
        Write-Host ''
    }
    $installed | Remove-AppxPackage
    Write-Host 'The test package is removed.'
}

$certificateFile = Join-Path $here 'NetworkToolkitTerminalTest.cer'
if (Test-Path -LiteralPath $certificateFile -PathType Leaf) {
    $thumbprint = [Security.Cryptography.X509Certificates.X509Certificate2]::new($certificateFile).Thumbprint
    $trusted = "Cert:\LocalMachine\TrustedPeople\$thumbprint"
    if (Test-Path -LiteralPath $trusted) {
        Write-Host 'Windows will ask for permission to stop trusting the test certificate.'
        $remove = Start-Process powershell.exe -Verb RunAs -Wait -PassThru -ArgumentList @('-NoProfile', '-Command', "Remove-Item -LiteralPath '$trusted'")
        if ($remove.ExitCode -eq 0 -and -not (Test-Path -LiteralPath $trusted)) { Write-Host 'The test certificate is no longer trusted.' }
        else { Write-Warning "Remove the certificate $thumbprint from Trusted People (Local Computer) with certlm.msc." }
    }
}
Remove-Item -LiteralPath $stateFolder -Recurse -Force -ErrorAction SilentlyContinue
Write-Host 'Done.'
