;TcNo Account Switcher
;Wesley Pyburn (TechNobo)
;https://github.com/TcNobo/TcNo-Acc-Switcher

;--------------------------------
;Include Modern UI

  !include "MUI2.nsh"

;--------------------------------
;Variables

!define APP_NAME "TcNo Account Switcher"
!define LNK_NAME "TcNo Account Switcher.lnk"
!define COMP_NAME "TechNobo (Wesley Pyburn)"
!define WEB_SITE "https://tcno.co"
!define VERSION "4.0.0.0"
!define COPYRIGHT "TechNobo (Wesley Pyburn) (C) 2021"
!define DESCRIPTION "TcNo Account Switcher"
!define LICENSE_TXT "..\..\LICENSE"
!define MAIN_APP_EXE "TcNo-Acc-Switcher.exe"
!define INSTALL_TYPE "SetShellVarContext current"
!define REG_ROOT "HKCU"
!define REG_APP_PATH "Software\Microsoft\Windows\CurrentVersion\App Paths\${MAIN_APP_EXE}"
!define UNINSTALL_EXE "Uninstall TcNo Account Switcher.exe"
!define UNINSTALL_LNK_NAME "Uninstall TcNo Account Switcher.lnk"
!define UNINSTALL_PATH "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}"
!define FIRST_RUN_EXE "_First_Run_Installer.exe"

!define REG_START_MENU "Start Menu Folder"
!define SM_Folder "TcNo Account Switcher"

VIProductVersion  "${VERSION}"
VIAddVersionKey "ProductName"  "${APP_NAME}"
VIAddVersionKey "CompanyName"  "${COMP_NAME}"
VIAddVersionKey "LegalCopyright"  "${COPYRIGHT}"
VIAddVersionKey "FileDescription"  "${DESCRIPTION}"
VIAddVersionKey "FileVersion"  "${VERSION}"

;--------------------------------
;Version specific variables

!define INSTALLER_NAME "TcNo Account Switcher - Installer.exe"

!define INSTALLER_7Z "..\..\TcNo-Acc-Switcher-Client\bin\x64\Release\TcNo-Acc-Switcher.7z"

!define INSTALL_DIR "$PROGRAMFILES64\TcNo Account Switcher"
;;;;!define INSTALL_DIR "$PROGRAMFILES\TcNo Account Switcher"

;--------------------------------
;Build options
Unicode True
SetCompress off
Name "${APP_NAME}"
Caption "${APP_NAME}"
OutFile "${INSTALLER_NAME}"
BrandingText "${APP_NAME}"
XPStyle on
InstallDirRegKey "${REG_ROOT}" "${REG_APP_PATH}" ""
InstallDir "${INSTALL_DIR}"

;--------------------------------
;Interface Configuration

  !include MUI.nsh
  !define MUI_ICON "img\icon.ico"
  !define MUI_UNICON "img\icon.ico"
  !define MUI_HEADERIMAGE
  !define MUI_HEADERIMAGE_BITMAP "img\HeaderImage.bmp"
  !define MUI_HEADERIMAGE_UNBITMAP "img\HeaderImage.bmp"
  !define MUI_HEADERIMAGE_BITMAP_STRETCH AspectFitHeight
  !define MUI_HEADERIMAGE_UNBITMAP_STRETCH AspectFitHeight
  !define MUI_ABORTWARNING

  !define MUI_WELCOMEFINISHPAGE_BITMAP "img\SideBanner.bmp"
  !define MUI_WELCOMEFINISHPAGE_BITMAP_NOSTRETCH
  !define MUI_UNWELCOMEFINISHPAGE_BITMAP "img\SideBanner.bmp"
  !define MUI_UNWELCOMEFINISHPAGE_BITMAP_NOSTRETCH

  !define MUI_BGCOLOR 1F212D
  !define MUI_TEXTCOLOR FFFFFF
  !define MUI_INSTFILESPAGE_COLORS "FFFFFF 1F212D"
  !define MUI_FINISHPAGE_LINK_COLOR FFAA00
  !define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\TcNo-Acc-Switcher"

;--------------------------------
;Pages

  !insertmacro MUI_PAGE_WELCOME
  !insertmacro MUI_PAGE_LICENSE "${LICENSE_TXT}"
  !insertmacro MUI_PAGE_COMPONENTS
  !insertmacro MUI_PAGE_DIRECTORY

   ;Start Menu Folder Page Configuration
  !define MUI_STARTMENUPAGE_REGISTRY_ROOT "HKCU" 
  !define MUI_STARTMENUPAGE_REGISTRY_KEY "Software\${APP_NAME}" 
  !define MUI_STARTMENUPAGE_REGISTRY_VALUENAME "${APP_NAME}"
  
  ;!insertmacro MUI_PAGE_STARTMENU Application $SM_Folder

  !insertmacro MUI_PAGE_INSTFILES
  
  !insertmacro MUI_UNPAGE_CONFIRM
  !insertmacro MUI_UNPAGE_INSTFILES

  ; Finish page
  !define MUI_FINISHPAGE_NOAUTOCLOSE
  !define MUI_UNFINISHPAGE_NOAUTOCLOSE
  !define MUI_FINISHPAGE_LINK "https://github.com/TcNobo/TcNo-Acc-Switcher"
  !define MUI_FINISHPAGE_LINK_LOCATION "https://github.com/TcNobo/TcNo-Acc-Switcher"

  !insertmacro MUI_PAGE_FINISH



;--------------------------------
;Languages
 
  !insertmacro MUI_LANGUAGE "English"

;--------------------------------
;Installer Sections

!include "FileFunc.nsh"

Section "Main files" InstSec
  SectionIn RO

  SetOutPath "$INSTDIR"
  DetailPrint "Extracting package..."
  SetDetailsPrint listonly
  File "${INSTALLER_7Z}"
  SetCompress auto
  Nsis7z::ExtractWithDetails "$INSTDIR\TcNo-Acc-Switcher.7z" "Decompressing %s..."
  Delete "$OUTDIR\TcNo-Acc-Switcher.7z"  
  
  ;Store installation folder
  WriteRegStr "${REG_ROOT}" "${REG_APP_PATH}" "" $INSTDIR
  
  ;Create uninstaller
  WriteUninstaller "$INSTDIR\${UNINSTALL_EXE}"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayName" "TcNo Account Switcher"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayVersion" "4"
  WriteRegStr HKLM "${UNINST_KEY}" "UninstallString" "$INSTDIR\${UNINSTALL_EXE}"
  WriteRegStr HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${UNINST_KEY}" "Publisher" "${COMP_NAME}"
  WriteRegStr HKLM "${UNINST_KEY}" "HelpLink" "${WEB_SITE}"
  WriteRegStr HKLM "${UNINST_KEY}" "URLInfoAbout" "${WEB_SITE}"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${UNINSTALL_EXE}"
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoModify" "1"
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoRepair" "1"

  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKLM "${UNINST_KEY}" "EstimatedSize" "$0"

  ;Create protocol
  WriteRegStr HKCR "tcno" "URL Protocol" ""
  WriteRegStr HKCR "tcno\Shell\Open\Command\" "" `"$INSTDIR\${MAIN_APP_EXE}" "%1"`
  
  ExecShell "" "$INSTDIR\${FIRST_RUN_EXE}"
SectionEnd

Section "Start Menu shortcuts" Shortcuts_StartMenu
  CreateDirectory "$SMPROGRAMS\${SM_Folder}"
  CreateShortcut "$SMPROGRAMS\${SM_Folder}\${LNK_NAME}" "$INSTDIR\${MAIN_APP_EXE}"
  CreateShortcut "$SMPROGRAMS\${SM_Folder}\${UNINSTALL_LNK_NAME}" "$INSTDIR\${UNINSTALL_EXE}"
SectionEnd

Section "Desktop shortcuts" Shortcuts_Desktop
  CreateShortCut "$DESKTOP\${LNK_NAME}" "$INSTDIR\${MAIN_APP_EXE}"
SectionEnd
;--------------------------------
;Descriptions

  ;Language strings
  LangString DESC_InstSec ${LANG_ENGLISH} "All the program files"
  LangString DESC_Shortcuts_StartMenu ${LANG_ENGLISH} "Launch & Uninstall shortcuts, placed into your Start Menu."
  LangString DESC_Shortcuts_Desktop ${LANG_ENGLISH} "Shortcut to launch the program, placed onto your Desktop."

  ;Assign language strings to sections
  !insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
    !insertmacro MUI_DESCRIPTION_TEXT ${InstSec} $(DESC_InstSec)
    !insertmacro MUI_DESCRIPTION_TEXT ${Shortcuts_StartMenu} $(DESC_Shortcuts_StartMenu)
    !insertmacro MUI_DESCRIPTION_TEXT ${Shortcuts_Desktop} $(DESC_Shortcuts_Desktop)
  !insertmacro MUI_FUNCTION_DESCRIPTION_END
 
;--------------------------------
;Uninstaller Section

Section "Uninstall"

  RMDir /r "$INSTDIR"
  ;Remove start shortcuts
  RMDIR /r "$SMPROGRAMS\${SM_Folder}"
  ;Remove desktop shortcuts
  Delete "$DESKTOP\${LNK_NAME}"
  ;Remove uninstaller entry
  DeleteRegKey /ifempty "${REG_ROOT}" "${REG_APP_PATH}"
  DeleteRegKey HKLM "${UNINST_KEY}"
  ;Remove Protocol entry
  DeleteRegKey HKCR "tcno"
SectionEnd