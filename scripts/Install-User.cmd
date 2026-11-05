@echo off
setlocal
REM Running this installer enables view-only portal sessions for the current user.
REM Sharing activity remains visible via a tray icon; Pause or Exit stops it.
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0install-user.ps1"
if errorlevel 1 (
  echo Installation failed. Please ensure you extracted the complete ZIP.
  pause
  exit /b 1
)
echo Agent installed. Look for the Hi5Central icon in the Windows notification area.
exit /b 0
