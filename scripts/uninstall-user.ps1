# Remove Hi5Central JPEG WebRTC from the current Windows account only.
$ErrorActionPreference = 'Stop'
$root = Join-Path $env:LOCALAPPDATA 'Hi5Central\JPEGWebRTC'
$run = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
Remove-ItemProperty -Path $run -Name 'Hi5JPEGWebRTCTray' -ErrorAction SilentlyContinue

$exe = Join-Path $root 'Hi5-JPEG-ScreenShare-GUI.exe'
$tray = Join-Path $root 'tray.ps1'
# Limit termination to our exact installed paths, not every PowerShell or WebRTC process.
Get-CimInstance Win32_Process -Filter "Name='Hi5-JPEG-ScreenShare-GUI.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.ExecutablePath -eq $exe } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
Get-CimInstance Win32_Process -Filter "Name='powershell.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.CommandLine -and $_.CommandLine.Contains($tray) } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }

Start-Sleep -Milliseconds 750
if (Test-Path -LiteralPath $root) {
    Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction Stop
}
Write-Host 'Hi5Central JPEG WebRTC removed from this user account.' -ForegroundColor Green
