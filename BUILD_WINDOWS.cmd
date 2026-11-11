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
rem The separate Text Editor and Terminal programs (docs\SEPARATE_EXECUTABLES.md):
rem the same code with a build tag. Each links its own resources from
rem internal\winres; the overlay leaves the main program's resource object out.
set "NTK_ROOT=%~dp0"
set "NTK_ROOT=%NTK_ROOT:\=\\%"
> build\separate-programs.overlay.json echo {"Replace":{"%NTK_ROOT%cmd\\network-toolkit\\resource_windows_amd64.syso":""}}
go build -mod=vendor -buildvcs=false -trimpath -tags ntk_editor -overlay build\separate-programs.overlay.json -ldflags="-s -w -H windowsgui" -o build\Network_Toolkit_TextEditor.exe ./cmd/network-toolkit
if errorlevel 1 exit /b 1
go build -mod=vendor -buildvcs=false -trimpath -tags ntk_terminal -overlay build\separate-programs.overlay.json -ldflags="-s -w -H windowsgui" -o build\Network_Toolkit_Terminal.exe ./cmd/network-toolkit
if errorlevel 1 exit /b 1
echo Build complete: build\Network_Toolkit.exe, build\Network_Toolkit_TextEditor.exe and build\Network_Toolkit_Terminal.exe
echo The loader is compiled into the EXE. No companion DLL is required.
endlocal
