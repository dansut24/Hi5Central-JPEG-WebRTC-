# Hi5Central JPEG WebRTC - no-elevation current-user installer.
# Running this script enables view-only screen sharing from the authorised
# web portal while this Windows user is signed in. An active stream is visibly
# indicated in the system tray, where the user can Pause or Exit at any time.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$root = Join-Path $env:LOCALAPPDATA 'Hi5Central\JPEGWebRTC'
$sourceRoot = Split-Path -Parent $PSScriptRoot
$sourceExe = Join-Path $sourceRoot 'Hi5-JPEG-ScreenShare-GUI.exe'
$sourceTray = Join-Path $PSScriptRoot 'tray.ps1'
$sourceUninstall = Join-Path $PSScriptRoot 'uninstall-user.ps1'
$stage = Join-Path $env:TEMP ('Hi5JPEG-' + [guid]::NewGuid().ToString('N'))
$exeName = 'Hi5-JPEG-ScreenShare-GUI.exe'
$trayName = 'tray.ps1'
$uninstallName = 'uninstall-user.ps1'

if (!(Test-Path -LiteralPath $sourceExe)) {
    throw 'Missing agent EXE. Extract the complete GitHub Actions ZIP before running Install-User.cmd.'
}
if (!(Test-Path -LiteralPath $sourceTray) -or !(Test-Path -LiteralPath $sourceUninstall)) {
    throw 'Missing installer support scripts. Extract the complete ZIP.'
}
try {
    New-Item -Path $stage -ItemType Directory -Force | Out-Null
    Copy-Item -LiteralPath $sourceExe -Destination (Join-Path $stage $exeName) -Force
    Copy-Item -LiteralPath $sourceTray -Destination (Join-Path $stage $trayName) -Force
    Copy-Item -LiteralPath $sourceUninstall -Destination (Join-Path $stage $uninstallName) -Force

    # Safe upgrade: stop only this user's exact installed agent and tray paths.
    $oldExe = Join-Path $root $exeName
    $oldTray = Join-Path $root $trayName
    Get-CimInstance Win32_Process -Filter "Name='Hi5-JPEG-ScreenShare-GUI.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.ExecutablePath -eq $oldExe } |
        ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
    Get-CimInstance Win32_Process -Filter "Name='powershell.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -and $_.CommandLine.Contains($oldTray) } |
        ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
    Start-Sleep -Milliseconds 500

    New-Item -Path $root -ItemType Directory -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $stage $exeName) -Destination $oldExe -Force
    Copy-Item -LiteralPath (Join-Path $stage $trayName) -Destination $oldTray -Force
    Copy-Item -LiteralPath (Join-Path $stage $uninstallName) -Destination (Join-Path $root $uninstallName) -Force

    # Current-user sign-in startup only. No service, admin prompt, or scheduled task.
    $run = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
    New-Item -Path $run -Force | Out-Null
    $startCommand = 'powershell.exe -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "' + $oldTray + '"'
    New-ItemProperty -Path $run -Name 'Hi5JPEGWebRTCTray' -PropertyType String -Value $startCommand -Force | Out-Null

    # Launch the visible notification-area controller, not a console.
    Start-Process -FilePath 'powershell.exe' -WindowStyle Hidden -ArgumentList @(
       '-NoProfile','-WindowStyle','Hidden','-ExecutionPolicy','Bypass','-File',
       ('"' + $oldTray + '"')
    )
    Write-Host 'Hi5Central JPEG installed for the signed-in Windows user.' -ForegroundColor Green
    Write-Host 'The notification-area icon shows sharing status, and Pause/Exit stops sharing.'
    Write-Host 'Device portal: https://jpeg.hi5central.com/portal'
} finally {
    if (Test-Path -LiteralPath $stage) {
        Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue
    }
}
