; APP_VERSION, APP_EXE, SETUP_EXE and APP_ICON are supplied by release packaging.
Unicode true
RequestExecutionLevel user

!include "MUI2.nsh"
!include "FileFunc.nsh"
!include "WordFunc.nsh"
!include "x64.nsh"

!define APP_NAME "Network Toolkit"
!define EDITOR_NAME "${APP_NAME} Text Editor"
!define APP_FILENAME "Network_Toolkit.exe"
!define INSTALL_DIR "$LOCALAPPDATA\Programs\${APP_NAME}"
!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\NetworkToolkit"
!define CAPABILITIES "Software\${APP_NAME}\Capabilities"
!define PROG_ID "NetworkToolkit.TextDocument"
!define PROG_KEY "Software\Classes\${PROG_ID}"

Name "${APP_NAME} ${APP_VERSION}"
OutFile "${SETUP_EXE}"
InstallDir "${INSTALL_DIR}"
SetCompressor /SOLID lzma
VIProductVersion "${APP_VERSION}.0"
VIAddVersionKey "ProductName" "${APP_NAME}"
VIAddVersionKey "ProductVersion" "${APP_VERSION}"
VIAddVersionKey "FileDescription" "${APP_NAME} Windows Setup"
VIAddVersionKey "FileVersion" "${APP_VERSION}"
VIAddVersionKey "LegalCopyright" "Network Toolkit contributors"

!define MUI_ICON "${APP_ICON}"
!define MUI_UNICON "${APP_ICON}"
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_TEXT "Open Windows Default Apps to choose a text editor"
!define MUI_FINISHPAGE_RUN_NOTCHECKED
!define MUI_FINISHPAGE_RUN_FUNCTION OpenDefaultApps
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Var StagedApp

!macro AcquireSetupMutex
  ; Keep the handle alive until this setup or uninstall process exits.
  System::Call 'kernel32::CreateMutexW(p 0, i 0, w "Local\NetworkToolkitSetup") p .r0 ?e'
  Pop $1
  ${If} $0 == 0
  ${OrIf} $1 == 183
    MessageBox MB_OK|MB_ICONSTOP "Another Network Toolkit setup or uninstall is already running." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
!macroend

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_OK|MB_ICONSTOP "This setup requires 64-bit Windows." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
  SetShellVarContext current
  SetRegView 64
  StrCpy $INSTDIR "${INSTALL_DIR}"
  !insertmacro AcquireSetupMutex
  Call RefuseDowngrade
FunctionEnd

Function un.onInit
  SetShellVarContext current
  SetRegView 64
  StrCpy $INSTDIR "${INSTALL_DIR}"
  !insertmacro AcquireSetupMutex
FunctionEnd

Function OpenDefaultApps
  ClearErrors
  ExecShell "open" "ms-settings:defaultapps?registeredAppUser=Network%20Toolkit%20Text%20Editor"
  ${If} ${Errors}
    ExecShell "open" "ms-settings:defaultapps"
  ${EndIf}
FunctionEnd

Function RefuseDowngrade
  ; The in-app updater replaces the binary without rewriting DisplayVersion.
  ; An executable with no readable version can still be repaired.
  ${GetFileVersion} "$INSTDIR\${APP_FILENAME}" $0
  ${If} ${Errors}
    Return
  ${EndIf}
  ${VersionCompare} $0 "${APP_VERSION}.0" $1
  ${If} $1 == 1
    ${If} $StagedApp != ""
      Delete "$StagedApp"
    ${EndIf}
    MessageBox MB_OK|MB_ICONSTOP "A newer version of Network Toolkit ($0) is already installed. Use setup for that version or a later version." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
FunctionEnd

!macro RegisterExtension EXTENSION
  WriteRegStr HKCU "${CAPABILITIES}\FileAssociations" "${EXTENSION}" "${PROG_ID}"
  WriteRegStr HKCU "Software\Classes\${EXTENSION}\OpenWithProgids" "${PROG_ID}" ""
!macroend

!macro RemoveExtension EXTENSION
  DeleteRegValue HKCU "Software\Classes\${EXTENSION}\OpenWithProgids" "${PROG_ID}"
!macroend

!macro TextExtensions ACTION
  !insertmacro ${ACTION} ".txt"
  !insertmacro ${ACTION} ".log"
  !insertmacro ${ACTION} ".ini"
  !insertmacro ${ACTION} ".cfg"
  !insertmacro ${ACTION} ".conf"
!macroend

Section "Install"
  ClearErrors
  SetOutPath "$INSTDIR"
  IfErrors install_error
  GetTempFileName $StagedApp "$INSTDIR"
  IfErrors install_error
  SetOverwrite try
  File /oname=$StagedApp "${APP_EXE}"
  IfErrors install_error

  ; Replace the program on the same volume only after extraction succeeds.
  ; Windows rejects the move if the running program is still using the file.
  Call RefuseDowngrade
  System::Call 'kernel32::MoveFileExW(w "$StagedApp", w "$INSTDIR\${APP_FILENAME}", i 0x9) i .r0'
  ${If} $0 == 0
    Goto install_error
  ${EndIf}
  StrCpy $StagedApp ""

  ClearErrors
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  IfErrors install_error
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "${APP_NAME}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\${APP_FILENAME}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKCU "${UNINSTALL_KEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair" 1

  CreateDirectory "$SMPROGRAMS\${APP_NAME}"
  CreateShortcut "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk" "$INSTDIR\${APP_FILENAME}"
  CreateShortcut "$SMPROGRAMS\${APP_NAME}\${EDITOR_NAME}.lnk" "$INSTDIR\${APP_FILENAME}" "--editor"
  CreateShortcut "$SMPROGRAMS\${APP_NAME}\${APP_NAME} Terminal.lnk" "$INSTDIR\${APP_FILENAME}" "--terminal"

  WriteRegStr HKCU "${PROG_KEY}" "" "${APP_NAME} text document"
  WriteRegNone HKCU "${PROG_KEY}" "AllowSilentDefaultTakeOver"
  WriteRegStr HKCU "${PROG_KEY}\DefaultIcon" "" '"$INSTDIR\${APP_FILENAME}",0'
  WriteRegStr HKCU "${PROG_KEY}\shell\open\command" "" '"$INSTDIR\${APP_FILENAME}" --open-file "%1"'
  WriteRegStr HKCU "${CAPABILITIES}" "ApplicationName" "${EDITOR_NAME}"
  WriteRegStr HKCU "${CAPABILITIES}" "ApplicationDescription" "Open text, log and configuration files in Network Toolkit Text Editor."
  WriteRegStr HKCU "${CAPABILITIES}" "ApplicationIcon" '"$INSTDIR\${APP_FILENAME}",0'
  !insertmacro TextExtensions RegisterExtension
  WriteRegStr HKCU "Software\RegisteredApplications" "${EDITOR_NAME}" "${CAPABILITIES}"
  IfErrors install_error
  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'
  Return

install_error:
  ${If} $StagedApp != ""
    Delete "$StagedApp"
  ${EndIf}
  MessageBox MB_OK|MB_ICONSTOP "Setup could not finish. Close Network Toolkit and any separate Editor or Terminal windows, then run setup again." /SD IDOK
  SetErrorLevel 2
  Abort
SectionEnd

Section "Uninstall"
  ; An open program must be closed before removing shortcuts or registrations.
  IfFileExists "$INSTDIR\${APP_FILENAME}" 0 remove_registration
  ClearErrors
  Delete "$INSTDIR\${APP_FILENAME}"
  ${If} ${Errors}
    MessageBox MB_OK|MB_ICONSTOP "Close Network Toolkit and any separate Editor or Terminal windows before uninstalling." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}

remove_registration:
  !insertmacro TextExtensions RemoveExtension
  DeleteRegValue HKCU "Software\RegisteredApplications" "${EDITOR_NAME}"
  DeleteRegKey HKCU "${CAPABILITIES}"
  DeleteRegKey HKCU "${PROG_KEY}"
  DeleteRegKey HKCU "${UNINSTALL_KEY}"
  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'

  Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\${APP_NAME}\${EDITOR_NAME}.lnk"
  Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME} Terminal.lnk"
  RMDir "$SMPROGRAMS\${APP_NAME}"

  ; App data and any unrelated files in the program directory are retained.
  SetOutPath "$TEMP"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
SectionEnd
