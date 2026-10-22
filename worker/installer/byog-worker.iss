; The installation of the client of the local computing network.
;
; The installer is built with Inno Setup, the standard tool of Windows. The user
; chooses the language, the directory of the installation and decides whether
; the client gets a shortcut on the desktop. The client is removed with the
; applications of the control panel, the removal is done by the uninstaller of
; this installation.
;
; The installer writes the language of the user into a small file of the user
; folder. The page of the first start of the client uses the same language and
; shows the policies of the installation.

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
ShowLanguageDialog=yes
ChangesAssociations=no
DisableWelcomePage=no
UsePreviousAppDir=yes
UsePreviousLanguage=yes
UsePreviousTasks=yes

[Languages]
Name: "turkish"; MessagesFile: "compiler:Languages\Turkish.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "french"; MessagesFile: "compiler:Languages\French.isl"
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"
Name: "japanese"; MessagesFile: "compiler:Languages\Japanese.isl"
Name: "zhcn"; MessagesFile: "languages\ChineseSimplified.isl"
Name: "vietnamese"; MessagesFile: "languages\Vietnamese.isl"

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

[Code]
// installerLanguage returns the language of the client that belongs to the
// language of the installation.
function installerLanguage(): String;
begin
  if ActiveLanguage = 'turkish' then
    Result := 'tr'
  else if ActiveLanguage = 'french' then
    Result := 'fr'
  else if ActiveLanguage = 'russian' then
    Result := 'ru'
  else if ActiveLanguage = 'japanese' then
    Result := 'ja'
  else if ActiveLanguage = 'zhcn' then
    Result := 'zh_CN'
  else if ActiveLanguage = 'zhtw' then
    Result := 'zh_TW'
  else if ActiveLanguage = 'vietnamese' then
    Result := 'vi'
  else
    Result := 'en';
end;

// writeInstallerLanguage keeps the language of the user for the page of the
// first start of the client.
procedure writeInstallerLanguage();
var
  directory: String;
  content: String;
begin
  directory := ExpandConstant('{userprofile}\.byog');
  if not DirExists(directory) then
    ForceDirectories(directory);
  content := '{"language":"' + installerLanguage() + '"}';
  SaveStringToFile(directory + '\installer.json', content, False);
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
    writeInstallerLanguage();
end;
