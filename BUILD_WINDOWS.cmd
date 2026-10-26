@echo off
setlocal
cd /d "%~dp0"
where go >nul 2>nul
if errorlevel 1 (
  echo The Go compiler is needed to build from source. The supplied EXE does not need Go.
  exit /b 1
)
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
set GOTOOLCHAIN=local
if not exist build mkdir build
go build -mod=vendor -buildvcs=false -trimpath -ldflags="-s -w -H windowsgui" -o build\Network_Toolkit.exe ./cmd/network-toolkit
if errorlevel 1 exit /b 1
echo Build complete: build\Network_Toolkit.exe
echo The loader is compiled into the EXE. No companion DLL is required.
endlocal
