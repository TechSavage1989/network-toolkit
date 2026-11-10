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
!define EDITOR_ARGS "--editor-standalone"
!define APP_FILENAME "Network_Toolkit.exe"
!define INSTALL_DIR "$LOCALAPPDATA\Programs\${APP_NAME}"
!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\NetworkToolkit"
!define CAPABILITIES "Software\${APP_NAME}\Capabilities"
!define PROG_ID "NetworkToolkit.TextDocument"
!define PROG_KEY "Software\Classes\${PROG_ID}"
; Terminal links (ssh://, telnet://, serial:). Windows lists the Terminal under
; Default apps; the user chooses it there. Setup never writes a UserChoice and
; never changes a scheme key that another app or Windows already registered.
!define TERMINAL_NAME "${APP_NAME} Terminal"
!define TERMINAL_KEY "Software\${APP_NAME}\Terminal"
!define TERMINAL_CAPABILITIES "${TERMINAL_KEY}\Capabilities"
!define LINK_PROG_ID "NetworkToolkit.TerminalLink"
!define LINK_PROG_KEY "Software\Classes\${LINK_PROG_ID}"

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
!define MUI_PAGE_CUSTOMFUNCTION_LEAVE ValidateDirectoryPage
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Var StagedApp
Var StagedUninstaller
Var RegisteredInstallDir
Var NewAppWritten
Var NewUninstallerWritten
Var ShellRegistrationStarted
Var NewMainShortcut
Var NewEditorShortcut
Var NewTerminalShortcut
Var DeleteAttempts
Var FailedStep
Var NewSshScheme
Var NewTelnetScheme
Var NewSerialScheme

; Silent setup and uninstall show no message. Leave the reason they stopped in
; a small log, replaced on every run, so the hosted setup check and support can
; read it. It holds the message only: no folder, file or user name.
!define SETUP_LOG "$TEMP\Network_Toolkit_Setup.log"
!macro LogSetupStop TEXT
  ClearErrors
  FileOpen $9 "${SETUP_LOG}" w
  ${IfNot} ${Errors}
    FileWrite $9 "${TEXT}$\r$\n"
    FileClose $9
  ${EndIf}
  ClearErrors
!macroend

!macro StopSetup TEXT
  !insertmacro LogSetupStop "${TEXT}"
  MessageBox MB_OK|MB_ICONSTOP "${TEXT}" /SD IDOK
  SetErrorLevel 2
  Abort
!macroend

!macro AcquireSetupMutex
  ; Keep the handle alive until this setup or uninstall process exits.
  System::Call 'kernel32::CreateMutexW(p 0, i 0, w "Local\NetworkToolkitSetup") p .r0 ?e'
  Pop $1
  ${If} $0 == 0
  ${OrIf} $1 == 183
    !insertmacro StopSetup "Another Network Toolkit setup or uninstall is already running."
  ${EndIf}
  Delete "${SETUP_LOG}"
!macroend

; Move a staged file into place ($R7 holds the MoveFileEx flags). A scanner can
; hold a just-written file for a moment, so a sharing or access failure is
; retried for about five seconds. Any other failure, such as an existing file
; on a fresh setup, is final at once. $0 is 0 on failure and $1 the Windows error.
!macro PlaceStagedFile SOURCE TARGET
  StrCpy $DeleteAttempts 0
  ${Do}
    System::Call 'kernel32::MoveFileExW(w "${SOURCE}", w "${TARGET}", i $R7) i .r0 ?e'
    Pop $1
    ${If} $0 != 0
      ${ExitDo}
    ${EndIf}
    ${If} $1 != 5
    ${AndIf} $1 != 32
      ${ExitDo}
    ${EndIf}
    IntOp $DeleteAttempts $DeleteAttempts + 1
    ${If} $DeleteAttempts >= 20
      ${ExitDo}
    ${EndIf}
    Sleep 250
  ${Loop}
!macroend

; Compare the filesystem's resolved directory to its ordinary long path.
; A junction, symbolic link, or substituted drive can redirect the deletes.
!macro IsPhysicalDirectory PATH
  StrCpy $R5 0
  System::Call 'kernel32::GetLongPathNameW(w "${PATH}", w .r0, i ${NSIS_MAX_STRLEN}) i .r1'
  ${If} $1 > 0
  ${AndIf} $1 < ${NSIS_MAX_STRLEN}
    System::Call 'kernel32::CreateFileW(w "${PATH}", i 0, i 7, p 0, i 3, i 0x02000000, p 0) p .r1'
    ${If} $1 != -1
      System::Call 'kernel32::GetFinalPathNameByHandleW(p r1, w .r2, i ${NSIS_MAX_STRLEN}, i 0) i .r3'
      System::Call 'kernel32::CloseHandle(p r1)'
      ${If} $3 > 0
      ${AndIf} $3 < ${NSIS_MAX_STRLEN}
        StrCpy $4 $2 8
        ${If} $4 == "\\?\UNC\"
          StrCpy $2 $2 "" 8
          StrCpy $2 "\\$2"
        ${Else}
          StrCpy $4 $2 4
          ${If} $4 == "\\?\"
            StrCpy $2 $2 "" 4
          ${EndIf}
        ${EndIf}
        ${If} $0 == $2
          StrCpy $R5 1
        ${EndIf}
      ${EndIf}
    ${EndIf}
  ${EndIf}
!macroend

; Read the stored target and arguments without resolving or following the link.
; NSIS uses IShellLinkW to create these links. IPersistFile::Load opens a copy
; for inspection, while SLGP_RAWPATH avoids looking up a moved executable.
!macro IsOwnedShortcut LINK ARGS LEGACY
  StrCpy $R5 0
  ${If} ${FileExists} "${LINK}"
    System::Call 'ole32::CoInitialize(p 0) i .r9'
    System::Call 'ole32::CoCreateInstance(g "{00021401-0000-0000-C000-000000000046}", p 0, i 1, g "{000214F9-0000-0000-C000-000000000046}", *p .r0) i .r1'
    ${If} $1 == 0
      System::Call '$0->0(g "{0000010B-0000-0000-C000-000000000046}", *p .r2) i .r1'
      ${If} $1 == 0
        System::Call '$2->5(w "${LINK}", i 0) i .r1'
        ${If} $1 == 0
          System::Call '$0->3(w .r3, i ${NSIS_MAX_STRLEN}, p 0, i 4) i .r1'
          ${If} $1 == 0
          ${AndIf} $3 == "$INSTDIR\${APP_FILENAME}"
            System::Call '$0->10(w .r4, i ${NSIS_MAX_STRLEN}) i .r1'
            ${If} $1 == 0
              ${If} $4 == "${ARGS}"
              ${OrIf} $4 == "${LEGACY}"
                StrCpy $R5 1
              ${EndIf}
            ${EndIf}
          ${EndIf}
        ${EndIf}
        System::Call '$2->2()'
      ${EndIf}
      System::Call '$0->2()'
    ${EndIf}
    ${If} $9 == 0
    ${OrIf} $9 == 1
      System::Call 'ole32::CoUninitialize()'
    ${EndIf}
  ${EndIf}
!macroend

Function un.onInit
  SetShellVarContext current
  SetRegView 64
  !insertmacro AcquireSetupMutex
  ReadRegStr $0 HKCU "${UNINSTALL_KEY}" "InstallLocation"
  ${If} $0 == ""
  ${OrIf} $0 != $INSTDIR
    !insertmacro StopSetup "The uninstaller is outside the registered Network Toolkit folder."
  ${EndIf}
  !insertmacro IsPhysicalDirectory "$INSTDIR"
  ${If} $R5 != 1
    !insertmacro StopSetup "The registered Network Toolkit folder no longer points to its original location."
  ${EndIf}
FunctionEnd

Function ShowDirectoryPage
  ${If} $RegisteredInstallDir != ""
    Abort
  ${EndIf}
FunctionEnd

Function ValidateInstallDirectory
  StrCpy $R5 ""
  ; GetFullPathNameW is lexical and works before the folder has been created.
  System::Call 'kernel32::GetFullPathNameW(w "$INSTDIR", i ${NSIS_MAX_STRLEN}, w .r0, p 0) i .r1'
  ${If} $1 == 0
  ${OrIf} $1 >= ${NSIS_MAX_STRLEN}
    StrCpy $R5 "Choose a valid program folder."
    Return
  ${EndIf}
  StrCpy $1 $0 1 -1
  ${If} $1 == "\"
    StrCpy $0 $0 -1
  ${EndIf}
  StrCpy $INSTDIR $0
  ${GetRoot} "$INSTDIR" $0
  ${If} $0 == ""
  ${OrIf} $INSTDIR == $0
  ${OrIf} $INSTDIR == "$0\"
  ${OrIf} $INSTDIR == "$LOCALAPPDATA\NetworkToolkit"
    StrCpy $R5 "Choose a program folder separate from the drive root and Network Toolkit data folder."
    Return
  ${EndIf}
  StrLen $0 "$LOCALAPPDATA\NetworkToolkit\"
  StrCpy $1 $INSTDIR $0
  ${If} $1 == "$LOCALAPPDATA\NetworkToolkit\"
    StrCpy $R5 "Choose a program folder outside the Network Toolkit data folder."
    Return
  ${EndIf}
  StrCpy $R6 $INSTDIR
  ${DoUntil} ${FileExists} "$R6\*.*"
    ${GetParent} "$R6" $0
    ${If} $0 == ""
    ${OrIf} $0 == $R6
      ${Break}
    ${EndIf}
    StrCpy $R6 $0
  ${Loop}
  ; A parent reduced to a drive letter needs its root separator.
  StrLen $0 $R6
  StrCpy $1 $R6 1 1
  ${If} $0 == 2
  ${AndIf} $1 == ":"
    StrCpy $R6 "$R6\"
  ${EndIf}
  !insertmacro IsPhysicalDirectory "$R6"
  ${If} $R5 != 1
    StrCpy $R5 "Choose a program folder that does not redirect to another location."
    Return
  ${EndIf}
  StrCpy $R5 ""
  ${If} $RegisteredInstallDir == ""
    ${If} ${FileExists} "$INSTDIR\${APP_FILENAME}"
    ${OrIf} ${FileExists} "$INSTDIR\Uninstall.exe"
      StrCpy $R5 "Choose a folder without an existing Network Toolkit executable or uninstaller."
    ${EndIf}
  ${EndIf}
FunctionEnd

Function ValidateDirectoryPage
  Call ValidateInstallDirectory
  ${If} $R5 != ""
    MessageBox MB_OK|MB_ICONSTOP "$R5"
    Abort
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
    !insertmacro StopSetup "A newer version of Network Toolkit ($0) is already installed. Use setup for that version or a later version."
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

; A scheme key is described only when neither this user nor the machine has
; one, so no other app's handler is replaced. Setup records each key it
; created; only that key, still exactly as written, is removed again.
!macro ProbeLinkScheme SCHEME RESULT
  StrCpy ${RESULT} 0
  ClearErrors
  EnumRegKey $0 HKCU "Software\Classes\${SCHEME}" 0
  ${If} ${Errors}
    ClearErrors
    EnumRegKey $0 HKLM "Software\Classes\${SCHEME}" 0
    ${If} ${Errors}
      StrCpy ${RESULT} 1
    ${EndIf}
  ${EndIf}
  ClearErrors
!macroend

!macro RegisterLinkScheme SCHEME DESCRIPTION CREATE
  WriteRegStr HKCU "${TERMINAL_CAPABILITIES}\URLAssociations" "${SCHEME}" "${LINK_PROG_ID}"
  ${If} ${CREATE} == 1
    WriteRegStr HKCU "Software\Classes\${SCHEME}" "" "URL:${DESCRIPTION}"
    WriteRegStr HKCU "Software\Classes\${SCHEME}" "URL Protocol" ""
    WriteRegDWORD HKCU "${TERMINAL_KEY}\CreatedSchemes" "${SCHEME}" 1
  ${EndIf}
!macroend

!macro RemoveLinkScheme SCHEME DESCRIPTION
  ReadRegDWORD $0 HKCU "${TERMINAL_KEY}\CreatedSchemes" "${SCHEME}"
  ${If} $0 == 1
    ClearErrors
    EnumRegKey $1 HKCU "Software\Classes\${SCHEME}" 0
    ReadRegStr $2 HKCU "Software\Classes\${SCHEME}" ""
    EnumRegValue $3 HKCU "Software\Classes\${SCHEME}" 2
    ; Leave the key if another app has since added a command or other data.
    ${If} $1 == ""
    ${AndIf} $2 == "URL:${DESCRIPTION}"
    ${AndIf} $3 == ""
      DeleteRegKey HKCU "Software\Classes\${SCHEME}"
    ${EndIf}
    ClearErrors
  ${EndIf}
!macroend

!macro UnregisterShell
  !insertmacro TextExtensions RemoveExtension
  DeleteRegValue HKCU "Software\RegisteredApplications" "${EDITOR_NAME}"
  DeleteRegKey HKCU "${CAPABILITIES}"
  DeleteRegKey HKCU "${PROG_KEY}"
  DeleteRegValue HKCU "Software\RegisteredApplications" "${TERMINAL_NAME}"
  !insertmacro RemoveLinkScheme "ssh" "SSH Protocol"
  !insertmacro RemoveLinkScheme "telnet" "Telnet Protocol"
  !insertmacro RemoveLinkScheme "serial" "Serial Port Protocol"
  DeleteRegKey HKCU "${TERMINAL_KEY}"
  DeleteRegKey HKCU "${LINK_PROG_KEY}"
  DeleteRegKey /ifempty HKCU "Software\${APP_NAME}"
  DeleteRegKey HKCU "${UNINSTALL_KEY}"
  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'
!macroend

!macro CreateStartShortcut LINK ARGS LEGACY ICON CREATED
  ${If} ${FileExists} "${LINK}"
    ${If} $RegisteredInstallDir != ""
      !insertmacro IsOwnedShortcut "${LINK}" "${ARGS}" "${LEGACY}"
      ${If} $R5 == 1
        ClearErrors
        CreateShortcut "${LINK}" "$INSTDIR\${APP_FILENAME}" "${ARGS}" "$INSTDIR\${APP_FILENAME}" ${ICON}
        IfErrors install_error
      ${EndIf}
    ${EndIf}
  ${Else}
    ClearErrors
    CreateShortcut "${LINK}" "$INSTDIR\${APP_FILENAME}" "${ARGS}" "$INSTDIR\${APP_FILENAME}" ${ICON}
    IfErrors install_error
    StrCpy ${CREATED} 1
  ${EndIf}
!macroend

Section "Network Toolkit"
  SectionIn RO
  Call ValidateInstallDirectory
  ${If} $R5 != ""
    !insertmacro StopSetup "$R5"
  ${EndIf}
  ; $FailedStep names the step under way, for the log written at install_error.
  StrCpy $FailedStep "creating the program folder"
  ClearErrors
  SetOutPath "$INSTDIR"
  IfErrors install_error
  !insertmacro IsPhysicalDirectory "$INSTDIR"
  ${If} $R5 != 1
    Goto install_error
  ${EndIf}
  StrCpy $FailedStep "extracting the program"
  GetTempFileName $StagedApp "$INSTDIR"
  IfErrors install_error
  SetOverwrite try
  File /oname=$StagedApp "${APP_EXE}"
  IfErrors install_error

  ; Replace the program on the same volume only after extraction succeeds.
  ; Windows rejects the move if the running program is still using the file.
  Call RefuseDowngrade
  StrCpy $FailedStep "writing the uninstaller"
  ClearErrors
  GetTempFileName $StagedUninstaller "$INSTDIR"
  IfErrors install_error
  WriteUninstaller "$StagedUninstaller"
  IfErrors install_error
  ; Fresh setup must never replace a competing program or uninstaller.
  StrCpy $R7 0x8
  ${If} $RegisteredInstallDir != ""
    StrCpy $R7 0x9
  ${EndIf}
  !insertmacro PlaceStagedFile "$StagedApp" "$INSTDIR\${APP_FILENAME}"
  ${If} $0 == 0
    StrCpy $FailedStep "placing the program file (Windows error $1)"
    Goto install_error
  ${EndIf}
  StrCpy $StagedApp ""
  StrCpy $NewAppWritten 1

  !insertmacro PlaceStagedFile "$StagedUninstaller" "$INSTDIR\Uninstall.exe"
  ${If} $0 == 0
    StrCpy $FailedStep "placing the uninstaller (Windows error $1)"
    Goto install_error
  ${EndIf}
  StrCpy $StagedUninstaller ""
  StrCpy $NewUninstallerWritten 1
  StrCpy $FailedStep "registering the Text Editor"
  ClearErrors
  StrCpy $ShellRegistrationStarted 1
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "${APP_NAME}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\${APP_FILENAME}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKCU "${UNINSTALL_KEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair" 1
  WriteRegStr HKCU "${PROG_KEY}" "" "${APP_NAME} text document"
  WriteRegNone HKCU "${PROG_KEY}" "AllowSilentDefaultTakeOver"
  WriteRegStr HKCU "${PROG_KEY}\DefaultIcon" "" '"$INSTDIR\${APP_FILENAME}",1'
  WriteRegStr HKCU "${PROG_KEY}\shell\open\command" "" '"$INSTDIR\${APP_FILENAME}" ${EDITOR_ARGS} --open-file "%1"'
  WriteRegStr HKCU "${CAPABILITIES}" "ApplicationName" "${EDITOR_NAME}"
  WriteRegStr HKCU "${CAPABILITIES}" "ApplicationDescription" "Open text, log and configuration files in Network Toolkit Text Editor."
  WriteRegStr HKCU "${CAPABILITIES}" "ApplicationIcon" '"$INSTDIR\${APP_FILENAME}",1'
  !insertmacro TextExtensions RegisterExtension
  WriteRegStr HKCU "Software\RegisteredApplications" "${EDITOR_NAME}" "${CAPABILITIES}"
  IfErrors install_error
  StrCpy $FailedStep "registering terminal links"
  ; Probing reports a missing key through the error flag; probe every scheme
  ; first, then write and check the registration as one step.
  !insertmacro ProbeLinkScheme "ssh" $NewSshScheme
  !insertmacro ProbeLinkScheme "telnet" $NewTelnetScheme
  !insertmacro ProbeLinkScheme "serial" $NewSerialScheme
  WriteRegStr HKCU "${LINK_PROG_KEY}" "" "${TERMINAL_NAME} link"
  WriteRegStr HKCU "${LINK_PROG_KEY}" "URL Protocol" ""
  WriteRegStr HKCU "${LINK_PROG_KEY}\DefaultIcon" "" '"$INSTDIR\${APP_FILENAME}",2'
  WriteRegStr HKCU "${LINK_PROG_KEY}\shell\open\command" "" '"$INSTDIR\${APP_FILENAME}" --terminal --open-url "%1"'
  WriteRegStr HKCU "${TERMINAL_CAPABILITIES}" "ApplicationName" "${TERMINAL_NAME}"
  WriteRegStr HKCU "${TERMINAL_CAPABILITIES}" "ApplicationDescription" "Open SSH, Telnet and serial links in Network Toolkit Terminal. Every link opens the connection form for you to confirm."
  WriteRegStr HKCU "${TERMINAL_CAPABILITIES}" "ApplicationIcon" '"$INSTDIR\${APP_FILENAME}",2'
  !insertmacro RegisterLinkScheme "ssh" "SSH Protocol" $NewSshScheme
  !insertmacro RegisterLinkScheme "telnet" "Telnet Protocol" $NewTelnetScheme
  !insertmacro RegisterLinkScheme "serial" "Serial Port Protocol" $NewSerialScheme
  WriteRegStr HKCU "Software\RegisteredApplications" "${TERMINAL_NAME}" "${TERMINAL_CAPABILITIES}"
  IfErrors install_error
  StrCpy $FailedStep "creating the Start menu folder"
  CreateDirectory "$SMPROGRAMS\${APP_NAME}"
  IfErrors install_error
  StrCpy $FailedStep "creating the Start menu shortcuts"
  !insertmacro CreateStartShortcut "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk" "" "" 0 $NewMainShortcut
  !insertmacro CreateStartShortcut "$SMPROGRAMS\${APP_NAME}\${EDITOR_NAME}.lnk" "${EDITOR_ARGS}" "--editor" 1 $NewEditorShortcut
  !insertmacro CreateStartShortcut "$SMPROGRAMS\${APP_NAME}\${APP_NAME} Terminal.lnk" "--terminal" "--terminal" 2 $NewTerminalShortcut
  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'
  Return

install_error:
  !insertmacro LogSetupStop "Setup could not finish while $FailedStep."
  ${If} $StagedApp != ""
    Delete "$StagedApp"
  ${EndIf}
  ${If} $StagedUninstaller != ""
    Delete "$StagedUninstaller"
  ${EndIf}
  ${If} $RegisteredInstallDir == ""
    ${If} $NewMainShortcut == 1
      Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk"
    ${EndIf}
    ${If} $NewEditorShortcut == 1
      Delete "$SMPROGRAMS\${APP_NAME}\${EDITOR_NAME}.lnk"
    ${EndIf}
    ${If} $NewTerminalShortcut == 1
      Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME} Terminal.lnk"
    ${EndIf}
    ${If} $ShellRegistrationStarted == 1
      !insertmacro UnregisterShell
    ${EndIf}
    ${If} $NewUninstallerWritten == 1
      Delete "$INSTDIR\Uninstall.exe"
    ${EndIf}
    ${If} $NewAppWritten == 1
      Delete "$INSTDIR\${APP_FILENAME}"
    ${EndIf}
  ${EndIf}
  MessageBox MB_OK|MB_ICONSTOP "Setup could not finish. Close Network Toolkit and any separate Editor or Terminal windows, then run setup again." /SD IDOK
  SetErrorLevel 2
  Abort
SectionEnd

Section /o "Create a desktop shortcut" DesktopShortcut
SectionEnd

Section -ApplyDesktopShortcut
  ReadRegDWORD $R8 HKCU "${UNINSTALL_KEY}" "DesktopShortcut"
  ${If} ${SectionIsSelected} ${DesktopShortcut}
    ${IfNot} ${FileExists} "$DESKTOP\${APP_NAME}.lnk"
      ClearErrors
      CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_FILENAME}"
      ${IfNot} ${Errors}
        WriteRegDWORD HKCU "${UNINSTALL_KEY}" "DesktopShortcut" 1
      ${EndIf}
    ${EndIf}
  ${ElseIf} $R8 == 1
    !insertmacro IsOwnedShortcut "$DESKTOP\${APP_NAME}.lnk" "" ""
    ${If} $R5 == 1
      ClearErrors
      Delete "$DESKTOP\${APP_NAME}.lnk"
      ${If} ${Errors}
        Return
      ${EndIf}
    ${EndIf}
    DeleteRegValue HKCU "${UNINSTALL_KEY}" "DesktopShortcut"
  ${EndIf}
SectionEnd

Function .onInit
  ${IfNot} ${RunningX64}
    !insertmacro StopSetup "This setup requires 64-bit Windows."
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
  ReadRegDWORD $R8 HKCU "${UNINSTALL_KEY}" "DesktopShortcut"
  StrCpy $R7 0
  ${If} $R8 == 1
    !insertmacro IsOwnedShortcut "$DESKTOP\${APP_NAME}.lnk" "" ""
    StrCpy $R7 $R5
  ${EndIf}
  !insertmacro IsOwnedShortcut "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk" "" ""
  StrCpy $NewMainShortcut $R5
  !insertmacro IsOwnedShortcut "$SMPROGRAMS\${APP_NAME}\${EDITOR_NAME}.lnk" "${EDITOR_ARGS}" "--editor"
  StrCpy $NewEditorShortcut $R5
  !insertmacro IsOwnedShortcut "$SMPROGRAMS\${APP_NAME}\${APP_NAME} Terminal.lnk" "--terminal" "--terminal"
  StrCpy $NewTerminalShortcut $R5
  ; An open program must be closed before removing shortcuts or registrations.
  ; Retry for about five seconds first: antivirus or the search indexer can
  ; hold a just-written executable briefly, which is not an open program.
  IfFileExists "$INSTDIR\${APP_FILENAME}" 0 remove_registration
  StrCpy $DeleteAttempts 0
  delete_program:
  ClearErrors
  Delete "$INSTDIR\${APP_FILENAME}"
  ${If} ${Errors}
    IntOp $DeleteAttempts $DeleteAttempts + 1
    ${If} $DeleteAttempts < 20
      Sleep 250
      Goto delete_program
    ${EndIf}
    !insertmacro StopSetup "Close Network Toolkit and any separate Editor or Terminal windows before uninstalling."
  ${EndIf}
  ; Windows reports a delete as done while another program (a scanner) still
  ; has the file open, and removes the name only when that program closes it.
  ; Wait for the name to go, so the program folder can be removed below.
  StrCpy $DeleteAttempts 0
  wait_program_gone:
  IfFileExists "$INSTDIR\${APP_FILENAME}" 0 remove_registration
  IntOp $DeleteAttempts $DeleteAttempts + 1
  ${If} $DeleteAttempts < 20
    Sleep 250
    Goto wait_program_gone
  ${EndIf}

remove_registration:
  ${If} $R7 == 1
    Delete "$DESKTOP\${APP_NAME}.lnk"
  ${EndIf}
  !insertmacro UnregisterShell

  ${If} $NewMainShortcut == 1
    Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk"
  ${EndIf}
  ${If} $NewEditorShortcut == 1
    Delete "$SMPROGRAMS\${APP_NAME}\${EDITOR_NAME}.lnk"
  ${EndIf}
  ${If} $NewTerminalShortcut == 1
    Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME} Terminal.lnk"
  ${EndIf}
  RMDir "$SMPROGRAMS\${APP_NAME}"

  ; App data and any unrelated files in the program directory are retained.
  SetOutPath "$TEMP"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
SectionEnd
