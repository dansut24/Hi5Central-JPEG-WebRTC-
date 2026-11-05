@echo off
setlocal
echo Installing Hi5Central JPEG screen sharing for this Windows user...
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0install-user.ps1"
if errorlevel 1 (
  echo Installation was not completed.
  pause
) else (
  echo Installation complete.
  timeout /t 5 >nul
)
