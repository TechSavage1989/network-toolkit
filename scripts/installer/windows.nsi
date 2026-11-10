; APP_VERSION, APP_EXE, SETUP_EXE and APP_ICON are supplied by release packaging.
Unicode true
RequestExecutionLevel user

!include "MUI2.nsh"
!include "FileFunc.nsh"
!include "Sections.nsh"
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
InstallDirRegKey HKCU "${UNINSTALL_KEY}" "InstallLocation"
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
!define MUI_PAGE_CUSTOMFUNCTION_PRE ShowDirectoryPage
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Var StagedApp
Var RegisteredInstallDir
Var NewAppWritten

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

Function un.onInit
  SetShellVarContext current
  SetRegView 64
  !insertmacro AcquireSetupMutex
  ReadRegStr $0 HKCU "${UNINSTALL_KEY}" "InstallLocation"
  ${If} $0 == ""
  ${OrIf} $0 != $INSTDIR
    MessageBox MB_OK|MB_ICONSTOP "The uninstaller is outside the registered Network Toolkit folder." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
FunctionEnd

Function ShowDirectoryPage
  ${If} $RegisteredInstallDir != ""
    Abort
  ${EndIf}
FunctionEnd

Function ValidateInstallDirectory
  GetFullPathName $INSTDIR "$INSTDIR"
  ${GetRoot} "$INSTDIR" $0
  ${If} $0 == ""
  ${OrIf} $INSTDIR == $0
  ${OrIf} $INSTDIR == "$0\"
  ${OrIf} $INSTDIR == "$LOCALAPPDATA\NetworkToolkit"
    MessageBox MB_OK|MB_ICONSTOP "Choose a program folder separate from the drive root and Network Toolkit data folder." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
  ${If} $RegisteredInstallDir == ""
    ${If} ${FileExists} "$INSTDIR\${APP_FILENAME}"
    ${OrIf} ${FileExists} "$INSTDIR\Uninstall.exe"
      MessageBox MB_OK|MB_ICONSTOP "Choose a folder without an existing Network Toolkit executable or uninstaller." /SD IDOK
      SetErrorLevel 2
      Abort
    ${EndIf}
  ${EndIf}
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

Section "Network Toolkit"
  SectionIn RO
  Call ValidateInstallDirectory
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
  StrCpy $NewAppWritten 1

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
  ${If} $NewAppWritten == 1
  ${AndIf} $RegisteredInstallDir == ""
    Delete "$INSTDIR\${APP_FILENAME}"
    Delete "$INSTDIR\Uninstall.exe"
  ${EndIf}
  MessageBox MB_OK|MB_ICONSTOP "Setup could not finish. Close Network Toolkit and any separate Editor or Terminal windows, then run setup again." /SD IDOK
  SetErrorLevel 2
  Abort
SectionEnd

Section /o "Create a desktop shortcut" DesktopShortcut
SectionEnd

Section -ApplyDesktopShortcut
  ReadRegDWORD $0 HKCU "${UNINSTALL_KEY}" "DesktopShortcut"
  ${If} ${SectionIsSelected} ${DesktopShortcut}
    ${If} $0 == 1
    ${OrIfNot} ${FileExists} "$DESKTOP\${APP_NAME}.lnk"
      ClearErrors
      CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_FILENAME}"
      ${IfNot} ${Errors}
        WriteRegDWORD HKCU "${UNINSTALL_KEY}" "DesktopShortcut" 1
      ${EndIf}
    ${EndIf}
  ${ElseIf} $0 == 1
    Delete "$DESKTOP\${APP_NAME}.lnk"
    DeleteRegValue HKCU "${UNINSTALL_KEY}" "DesktopShortcut"
  ${EndIf}
SectionEnd

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_OK|MB_ICONSTOP "This setup requires 64-bit Windows." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
  SetShellVarContext current
  SetRegView 64
  !insertmacro AcquireSetupMutex
  ReadRegStr $RegisteredInstallDir HKCU "${UNINSTALL_KEY}" "InstallLocation"
  ${If} $RegisteredInstallDir != ""
    StrCpy $INSTDIR "$RegisteredInstallDir"
    Call RefuseDowngrade
  ${EndIf}
  ReadRegDWORD $0 HKCU "${UNINSTALL_KEY}" "DesktopShortcut"
  ${If} $0 == 1
    !insertmacro SelectSection ${DesktopShortcut}
  ${EndIf}
  ${GetParameters} $0
  ${GetOptions} $0 "/DesktopShortcut" $1
  ${IfNot} ${Errors}
    !insertmacro SelectSection ${DesktopShortcut}
  ${EndIf}
FunctionEnd

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
  ReadRegDWORD $0 HKCU "${UNINSTALL_KEY}" "DesktopShortcut"
  ${If} $0 == 1
    Delete "$DESKTOP\${APP_NAME}.lnk"
  ${EndIf}
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
