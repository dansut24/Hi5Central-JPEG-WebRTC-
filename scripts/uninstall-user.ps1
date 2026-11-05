# Remove Hi5Central JPEG WebRTC from the current Windows account.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$root = Join-Path $env:LOCALAPPDATA 'Hi5Central\JPEGWebRTC'
$run = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
Remove-ItemProperty -Path $run -Name 'Hi5JPEGWebRTCTray' -ErrorAction SilentlyContinue
# Stop only the current user's installed agent at its exact executable path.
$exe = Join-Path $root 'Hi5-JPEG-ScreenShare-GUI.exe'
Get-CimInstance Win32_Process -Filter "Name='Hi5-JPEG-ScreenShare-GUI.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.ExecutablePath -eq $exe } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
Write-Host 'Automatic startup removed. Exit the tray application to finish uninstalling.'
Write-Host ('Then delete folder: ' + $root)
