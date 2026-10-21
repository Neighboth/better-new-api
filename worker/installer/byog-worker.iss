; The installation of the client of the local computing network.
;
; The installer is built with Inno Setup, the standard tool of Windows. The user
; chooses the directory of the installation and decides whether the client gets a
; shortcut on the desktop. The client is removed with the applications of the
; control panel, the removal is done by the uninstaller of this installation.

#define MyAppName "BYOG Worker"
#define MyAppVersion "1.9.0"
#define MyAppPublisher "BYOG"
#define MyAppExeName "BYOG Worker.exe"
#define MyAppId "BYOGWorker"

[Setup]
AppId={{5B4C0A1E-6C1B-4C0E-9E4F-7A3D2B5C8E11}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
VersionInfoVersion={#MyAppVersion}
VersionInfoDescription={#MyAppName}
VersionInfoCompany={#MyAppPublisher}
DefaultDirName={localappdata}\Programs\{#MyAppName}
DefaultGroupName={#MyAppName}
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
DisableProgramGroupPage=yes
DisableDirPage=no
AllowNoIcons=no
OutputDir=..\binaries
OutputBaseFilename={#MyAppName} Kurulumu
SetupIconFile=..\desktop\assets\byog.ico
UninstallDisplayIcon={app}\byog.ico
UninstallDisplayName={#MyAppName}
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
CloseApplications=yes
RestartApplications=no
ShowLanguageDialog=no
ChangesAssociations=no
DisableWelcomePage=no

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "..\binaries\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\desktop\assets\byog.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\byog.ico"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\byog.ico"; Tasks: desktopicon
Name: "{userstartup}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\byog.ico";

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
Type: filesandordirs; Name: "{app}"
