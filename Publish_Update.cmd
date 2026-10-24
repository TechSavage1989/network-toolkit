@echo off
setlocal DisableDelayedExpansion
cd /d "%~dp0"
where python >nul 2>nul
if errorlevel 1 (
  echo Python 3 is required for this optional local publisher.
  pause
  exit /b 1
)
python scripts\setup_github.py publish
set "toolkit_result=%errorlevel%"
pause
exit /b %toolkit_result%
