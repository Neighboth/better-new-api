; The installation of the client of the local computing network.
;
; The installer is built with Inno Setup, the standard tool of Windows. The
; dashboard downloads the installer with the name, the icon and the address of
; the installation written into it, the installation shows the name of the
; installation and it keeps the language of the user.
;
; The program of the client is installed with the same settings at the end of
; its file, the client reads them when it starts and it does not ask the user
; for them. The policies of the installation are shown before the client is
; installed, the same policy of the page of the first start.

#define MyAppName "BYOG Worker"
#define MyAppVersion "1.9.0"
#define MyAppPublisher "BYOG"
#define MyAppExeName "BYOG Worker.exe"

[Setup]
AppId={{5B4C0A1E-6C1B-4C0E-9E4F-7A3D2B5C8E11}
AppName={code:GetDisplayName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
VersionInfoVersion={#MyAppVersion}
VersionInfoDescription={#MyAppName}
VersionInfoProductName={#MyAppName}
VersionInfoCompany={#MyAppPublisher}
DefaultDirName={code:GetDefaultDir}
DefaultGroupName={#MyAppName}
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
DisableProgramGroupPage=yes
DisableDirPage=no
OutputDir=..\binaries
OutputBaseFilename={#MyAppName} Kurulumu
SetupIconFile=..\desktop\assets\byog.ico
UninstallDisplayIcon={app}\byog.ico
UninstallDisplayName={code:GetDisplayName}
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

[Languages]
Name: "turkish"; MessagesFile: "compiler:Languages\Turkish.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "french"; MessagesFile: "compiler:Languages\French.isl"
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"
Name: "japanese"; MessagesFile: "compiler:Languages\Japanese.isl"
Name: "zhcn"; MessagesFile: "languages\ChineseSimplified.isl"
Name: "zhtw"; MessagesFile: "languages\ChineseTraditional.isl"
Name: "vietnamese"; MessagesFile: "languages\Vietnamese.isl"

[CustomMessages]
turkish.PolicyPageTitle=Gizlilik ve Kullanım
turkish.PolicyPageDescription=Kurulumun gizlilik politikasını ve kullanım koşullarını okuyun.
turkish.PolicyAccept=Gizlilik politikasını ve kullanım koşullarını okudum, kabul ediyorum.
turkish.PolicyAcceptRequired=Kuruluma devam etmek için politikaları kabul edin.
turkish.PolicyMissing=Politikalar sunucudan alınamadı. Politikaları şu adreste bulabilirsiniz:

english.PolicyPageTitle=Privacy and Terms
english.PolicyPageDescription=Read the privacy policy and the terms of service of the installation.
english.PolicyAccept=I have read and accept the privacy policy and the terms of service.
english.PolicyAcceptRequired=Accept the policies to continue the installation.
english.PolicyMissing=The policies could not be read from the server. You can find them at:

french.PolicyPageTitle=Confidentialité et conditions
french.PolicyPageDescription=Lisez la politique de confidentialité et les conditions d'utilisation de l'installation.
french.PolicyAccept=J'ai lu et j'accepte la politique de confidentialité et les conditions d'utilisation.
french.PolicyAcceptRequired=Acceptez les conditions pour poursuivre l'installation.
french.PolicyMissing=Les conditions n'ont pas pu être lues depuis le serveur. Vous pouvez les trouver à l'adresse :

russian.PolicyPageTitle=Конфиденциальность и условия
russian.PolicyPageDescription=Прочитайте политику конфиденциальности и условия использования установки.
russian.PolicyAccept=Я прочитал(а) и принимаю политику конфиденциальности и условия использования.
russian.PolicyAcceptRequired=Примите условия, чтобы продолжить установку.
russian.PolicyMissing=Не удалось получить условия с сервера. Их можно найти по адресу:

japanese.PolicyPageTitle=プライバシーと利用規約
japanese.PolicyPageDescription=インストールのプライバシーポリシーと利用規約をお読みください。
japanese.PolicyAccept=プライバシーポリシーと利用規約を読み、同意します。
japanese.PolicyAcceptRequired=インストールを続けるには規約に同意してください。
japanese.PolicyMissing=サーバーから規約を取得できませんでした。次のアドレスで確認できます:

zhcn.PolicyPageTitle=隐私与条款
zhcn.PolicyPageDescription=请阅读本安装的隐私政策和服务条款。
zhcn.PolicyAccept=我已阅读并同意隐私政策和服务条款。
zhcn.PolicyAcceptRequired=请同意条款以继续安装。
zhcn.PolicyMissing=无法从服务器读取条款。您可以在以下地址查看：

zhtw.PolicyPageTitle=隱私權與條款
zhtw.PolicyPageDescription=請閱讀本安裝的隱私權政策與服務條款。
zhtw.PolicyAccept=我已閱讀並同意隱私權政策與服務條款。
zhtw.PolicyAcceptRequired=請同意條款以繼續安裝。
zhtw.PolicyMissing=無法從伺服器讀取條款。您可以在以下網址查看：

vietnamese.PolicyPageTitle=Quyền riêng tư và điều khoản
vietnamese.PolicyPageDescription=Hãy đọc chính sách quyền riêng tư và điều khoản sử dụng của bản cài đặt.
vietnamese.PolicyAccept=Tôi đã đọc và đồng ý với chính sách quyền riêng tư và điều khoản sử dụng.
vietnamese.PolicyAcceptRequired=Hãy đồng ý với các điều khoản để tiếp tục cài đặt.
vietnamese.PolicyMissing=Không thể đọc điều khoản từ máy chủ. Bạn có thể xem tại:
[Files]
Source: "..\binaries\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\desktop\assets\byog.ico"; DestDir: "{app}"; Flags: ignoreversion

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "{code:GetDisplayName}"; ValueData: """{app}\{#MyAppExeName}"""; Flags: uninsdeletevalue

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,BYOG Worker}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
Type: filesandordirs; Name: "{app}"

[Code]
var
  BrandingLoaded: Boolean;
  BrandServerUrl: String;
  BrandSystemName: String;
  BrandAppName: String;
  BrandFavicon: String;
  PolicyPage: TWizardPage;
  PolicyMemo: TNewMemo;
  PolicyAccept: TNewCheckBox;

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

// JsonUnescape reads a text of JSON with its escapes.
function JsonUnescape(const value: String): String;
var
  index, code, low: Integer;
begin
  Result := '';
  index := 1;
  while index <= Length(value) do
  begin
    if (value[index] <> '\') or (index = Length(value)) then
    begin
      Result := Result + value[index];
      Inc(index);
      Continue;
    end;
    Inc(index);
    case value[index] of
      '"', '\', '/':
        Result := Result + value[index];
      'b':
        Result := Result + #8;
      'f':
        Result := Result + #12;
      'n':
        Result := Result + #10;
      'r':
        Result := Result + #13;
      't':
        Result := Result + #9;
      'u':
        begin
          code := StrToIntDef('$' + Copy(value, index + 1, 4), 0);
          index := index + 4;
          if (code >= $D800) and (code <= $DBFF) and (Copy(value, index + 1, 2) = '\u') then
          begin
            low := StrToIntDef('$' + Copy(value, index + 3, 4), 0);
            index := index + 6;
            Result := Result + Chr(code) + Chr(low);
          end
          else
            Result := Result + Chr(code);
        end;
    else
      Result := Result + value[index];
    end;
    Inc(index);
  end;
end;

// JsonText returns the value of a text of a JSON document.
function JsonText(const document: String; const key: String): String;
var
  pattern: String;
  start, finish: Integer;
begin
  Result := '';
  pattern := '"' + key + '":"';
  start := Pos(pattern, document);
  if start = 0 then
    Exit;
  start := start + Length(pattern);
  finish := start;
  while finish <= Length(document) do
  begin
    if document[finish] = '\' then
    begin
      finish := finish + 2;
      Continue;
    end;
    if document[finish] = '"' then
      Break;
    Inc(finish);
  end;
  if finish > Length(document) then
    Exit;
  Result := JsonUnescape(Copy(document, start, finish - start));
end;

// LoadBranding reads the settings of the installation from the end of the
// program of the installation.
function LoadBranding(): Boolean;
var
  stream: TFileStream;
  total, markerPosition: Int64;
  settingsSize: Int64;
  lengthText: AnsiString;
  mark: AnsiString;
  settings: AnsiString;
  document, markText: String;
  index: Integer;
begin
  Result := False;
  BrandServerUrl := '';
  BrandSystemName := '';
  BrandAppName := '';
  BrandFavicon := '';
  try
    stream := TFileStream.Create(ExpandConstant('{srcexe}'), fmOpenRead or fmShareDenyNone);
    try
      total := stream.Size;
      if total < 32 then
        Exit;
      stream.Seek(total - 4, soFromBeginning);
      SetLength(lengthText, 4);
      stream.ReadBuffer(lengthText, 4);
      settingsSize := Ord(lengthText[1]) + Ord(lengthText[2]) * 256 +
        Ord(lengthText[3]) * 65536 + Ord(lengthText[4]) * 16777216;
      if (settingsSize = 0) or (settingsSize > 1048576) then
        Exit;
      markerPosition := total - 4 - settingsSize - 11;
      if markerPosition < 0 then
        Exit;
      stream.Seek(markerPosition, soFromBeginning);
      SetLength(mark, 11);
      stream.ReadBuffer(mark, 11);
      markText := '';
      for index := 1 to 11 do
        markText := markText + Chr(Ord(mark[index]));
      if markText <> #0#0'BYOG-CFG'#0 then
        Exit;
      stream.Seek(markerPosition + 11, soFromBeginning);
      SetLength(settings, Longint(settingsSize));
      stream.ReadBuffer(settings, Longint(settingsSize));
      document := '';
      for index := 1 to Length(settings) do
        document := document + Chr(Ord(settings[index]));
      BrandServerUrl := JsonText(document, 'server_url');
      BrandSystemName := JsonText(document, 'system_name');
      BrandAppName := JsonText(document, 'app_name');
      BrandFavicon := JsonText(document, 'favicon');
      Result := (BrandServerUrl <> '') or (BrandSystemName <> '');
    finally
      stream.Free;
    end;
  except
    Result := False;
  end;
end;

// GetDisplayName returns the name of the installation in the applications of
// the control panel.
function GetDisplayName(Param: String): String;
var
  name: String;
begin
  name := Trim(BrandSystemName);
  if name = '' then
    name := Trim(BrandAppName);
  if name = '' then
    name := 'BYOG Worker';
  if (name = 'BYOG Worker') or
     (CompareText(Copy(name, Length(name) - 10, 11), 'BYOG Worker') = 0) then
    Result := name
  else
    Result := name + ' BYOG Worker';
end;

// GetDefaultDir returns the directory of the installation.
function GetDefaultDir(Param: String): String;
begin
  Result := ExpandConstant('{localappdata}\Programs\') + GetDisplayName('');
  Log('BYOG installation directory: ' + Result);
end;

// JsonEscape writes a text as a text of JSON.
function JsonEscape(const value: String): String;
var
  index: Integer;
  character: Char;
begin
  Result := '';
  for index := 1 to Length(value) do
  begin
    character := value[index];
    if character = '"' then
      Result := Result + '\"'
    else if character = '\' then
      Result := Result + '\\'
    else if Ord(character) = 10 then
      Result := Result + '\n'
    else if Ord(character) = 13 then
      Result := Result + '\r'
    else if Ord(character) = 9 then
      Result := Result + '\t'
    else
      Result := Result + character;
  end;
end;

// ReadPolicies reads the policies of the installation as a text.
function ReadPolicies(): String;
var
  request: Variant;
  address: String;
begin
  Result := '';
  if BrandServerUrl = '' then
    Exit;
  address := BrandServerUrl + '/api/v1/worker/setup/policies?lang=' + installerLanguage();
  try
    request := CreateOleObject('WinHttp.WinHttpRequest.5.1');
    request.Open('GET', address, False);
    request.SetTimeouts(2000, 2000, 4000, 4000);
    request.Send;
    if request.Status = 200 then
      Result := request.ResponseText;
  except
    Result := '';
  end;
end;

// AppendBrandingToClient writes the settings of the installation at the end of
// the program of the client.
procedure AppendBrandingToClient();
var
  source, target: TFileStream;
  total, markerPosition, settingsSize: Int64;
  lengthText: AnsiString;
  marker: AnsiString;
begin
  source := TFileStream.Create(ExpandConstant('{srcexe}'), fmOpenRead or fmShareDenyNone);
  try
    total := source.Size;
    if total < 32 then
      Exit;
    source.Seek(total - 4, soFromBeginning);
    SetLength(lengthText, 4);
    source.ReadBuffer(lengthText, 4);
    settingsSize := Ord(lengthText[1]) + Ord(lengthText[2]) * 256 +
      Ord(lengthText[3]) * 65536 + Ord(lengthText[4]) * 16777216;
    if (settingsSize = 0) or (settingsSize > 1048576) then
      Exit;
    markerPosition := total - 4 - settingsSize - 11;
    if markerPosition < 0 then
      Exit;
    // The last bytes of a program that was not branded by the installation
    // can look like the settings, the mark tells the settings from them.
    source.Seek(markerPosition, soFromBeginning);
    SetLength(marker, 11);
    source.ReadBuffer(marker, 11);
    if marker <> #0#0'BYOG-CFG'#0 then
      Exit;
    target := TFileStream.Create(ExpandConstant('{app}\{#MyAppExeName}'), fmOpenReadWrite);
    try
      target.Seek(0, soFromEnd);
      source.Seek(markerPosition, soFromBeginning);
      target.CopyFrom(source, total - markerPosition, 65536);
    finally
      target.Free;
    end;
  finally
    source.Free;
  end;
end;

// writeInstallerLanguage keeps the language of the user for the page of the
// first start of the client.
procedure writeInstallerLanguage();
var
  directory: String;
  content: String;
begin
  directory := ExpandConstant('{%USERPROFILE}\.byog');
  if not DirExists(directory) then
    ForceDirectories(directory);
  content := '{"language":"' + installerLanguage() + '"}';
  SaveStringToFile(directory + '\installer.json', content, False);
end;

// WriteClientConfig writes the language and the acceptance of the policies of
// the installation before the client starts. The settings of an existing
// installation are kept.
procedure WriteClientConfig();
var
  directory, path, content: String;
begin
  directory := ExpandConstant('{%USERPROFILE}\.byog');
  if not DirExists(directory) then
    ForceDirectories(directory);
  path := directory + '\config.json';
  if FileExists(path) then
    Exit;
  content := '{"server_url":"' + JsonEscape(BrandServerUrl) + '"' +
    ',"language":"' + installerLanguage() + '"' +
    ',"legal_accepted":true}';
  SaveStringToFile(path, content, False);
end;

procedure InitializeWizard();
var
  policies: String;
begin
  policies := ReadPolicies();

  PolicyPage := CreateCustomPage(wpSelectDir,
    ExpandConstant('{cm:PolicyPageTitle}'),
    ExpandConstant('{cm:PolicyPageDescription}'));

  PolicyMemo := TNewMemo.Create(PolicyPage);
  PolicyMemo.Parent := PolicyPage.Surface;
  PolicyMemo.Left := 0;
  PolicyMemo.Top := 0;
  PolicyMemo.Width := PolicyPage.SurfaceWidth;
  PolicyMemo.Height := PolicyPage.SurfaceHeight - ScaleY(40);
  PolicyMemo.ScrollBars := ssVertical;
  PolicyMemo.ReadOnly := True;
  if policies <> '' then
    PolicyMemo.Text := policies
  else
    PolicyMemo.Text := ExpandConstant('{cm:PolicyMissing}') + #13#10#13#10 + BrandServerUrl;

  PolicyAccept := TNewCheckBox.Create(PolicyPage);
  PolicyAccept.Parent := PolicyPage.Surface;
  PolicyAccept.Left := 0;
  PolicyAccept.Top := PolicyMemo.Height + ScaleY(10);
  PolicyAccept.Width := PolicyPage.SurfaceWidth;
  PolicyAccept.Caption := ExpandConstant('{cm:PolicyAccept}');
end;

function InitializeSetup(): Boolean;
begin
  BrandingLoaded := LoadBranding();
  Log('BYOG branding: ' + IntToStr(Ord(BrandingLoaded)) + ' | ' +
    BrandSystemName + ' | ' + BrandServerUrl);
  Result := True;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if (not WizardSilent) and (PolicyPage <> nil) and (CurPageID = PolicyPage.ID) then
  begin
    if not PolicyAccept.Checked then
    begin
      MsgBox(ExpandConstant('{cm:PolicyAcceptRequired}'), mbError, MB_OK);
      Result := False;
    end;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    writeInstallerLanguage();
    if BrandingLoaded then
      AppendBrandingToClient();
    WriteClientConfig();
  end;
end;




