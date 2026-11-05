# Install the JPEG screen-sharing agent for the CURRENT USER without elevation.
# Run after extracting the successful GitHub Actions ZIP containing the EXE and this script.
# By installing, you opt in to a visible background agent that starts on sign-in.
[CmdletBinding()]
param([switch]$AcceptUnattendedSharing)
$ErrorActionPreference = 'Stop'
$root = Join-Path $env:LOCALAPPDATA 'Hi5Central\JPEGWebRTC'
$source = Join-Path (Split-Path -Parent $PSScriptRoot) 'Hi5-JPEG-ScreenShare-GUI.exe'
$traySource = Join-Path $PSScriptRoot 'tray.ps1'
if (!(Test-Path -LiteralPath $source)) { throw 'Missing Hi5-JPEG-ScreenShare-GUI.exe. Extract the complete release ZIP before installing.' }
if (!(Test-Path -LiteralPath $traySource)) { throw 'Missing scripts/tray.ps1. Extract the complete release ZIP before installing.' }

Write-Host ''
Write-Host 'Hi5Central JPEG user-mode screen sharing' -ForegroundColor Cyan
Write-Host '-----------------------------------------'
Write-Host 'Installs only for your Windows account; no administrator rights required.'
Write-Host 'Starts at each sign-in and stays in the notification area.'
Write-Host 'An authenticated portal operator may view the signed-in desktop'
Write-Host 'without an additional per-session approval prompt.'
Write-Host 'The local user can pause or exit it from the tray at any time.'
Write-Host ''
if (!$AcceptUnattendedSharing) {
    $answer = Read-Host 'Type INSTALL to confirm informed consent'
    if ($answer -cne 'INSTALL') { throw 'Installation cancelled.' }
}

New-Item -Path $root -ItemType Directory -Force | Out-Null
Copy-Item -LiteralPath $source -Destination (Join-Path $root 'Hi5-JPEG-ScreenShare-GUI.exe') -Force
Copy-Item -LiteralPath $traySource -Destination (Join-Path $root 'tray.ps1') -Force

$run = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
New-Item -Path $run -Force | Out-Null
$startup = 'powershell.exe -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "' + (Join-Path $root 'tray.ps1') + '"'
New-ItemProperty -Path $run -Name 'Hi5JPEGWebRTCTray' -Value $startup -PropertyType String -Force | Out-Null

Start-Process -FilePath 'powershell.exe' -WindowStyle Hidden -ArgumentList @(
    '-NoProfile','-WindowStyle','Hidden','-ExecutionPolicy','Bypass','-File',
    ('"' + (Join-Path $root 'tray.ps1') + '"')
)
Write-Host ''
Write-Host ('Installed to: ' + $root) -ForegroundColor Green
Write-Host 'Starting tray icon. Look in the notification area (^ icon).'
Write-Host 'Portal: https://jpeg.hi5central.com/portal'
Write-Host 'Remove this application using scripts\uninstall-user.ps1.'
