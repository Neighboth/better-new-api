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
#define MyAppVersion "1.9.1"
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
DisableProgramGroupPage=yes
DisableDirPage=no
OutputDir=..\binaries
OutputBaseFilename={#MyAppName} Kurulumu
SetupIconFile=..\desktop\assets\byog.ico
UninstallDisplayIcon={app}\byog.ico
UninstallDisplayName={code:GetDisplayName}
Compression=lzma2/fast
SolidCompression=no
WizardStyle=modern
WizardSmallImageFile=..\desktop\assets\wizard_small.bmp
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
CloseApplications=no
RestartApplications=no
ShowLanguageDialog=no
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
turkish.PolicyLoading=Politikalar yükleniyor...
turkish.DownloadingClient=İstemci indiriliyor...

english.PolicyPageTitle=Privacy and Terms
english.PolicyPageDescription=Read the privacy policy and the terms of service of the installation.
english.PolicyAccept=I have read and accept the privacy policy and the terms of service.
english.PolicyAcceptRequired=Accept the policies to continue the installation.
english.PolicyMissing=The policies could not be read from the server. You can find them at:
english.PolicyLoading=Loading policies...
english.DownloadingClient=Downloading client...

french.PolicyPageTitle=Confidentialité et conditions
french.PolicyPageDescription=Lisez la politique de confidentialité et les conditions d'utilisation de l'installation.
french.PolicyAccept=J'ai lu et j'accepte la politique de confidentialité et les conditions d'utilisation.
french.PolicyAcceptRequired=Acceptez les conditions pour poursuivre l'installation.
french.PolicyMissing=Les conditions n'ont pas pu être lues depuis le serveur. Vous pouvez les trouver à l'adresse :
french.PolicyLoading=Chargement des conditions...
french.DownloadingClient=Téléchargement du client...

russian.PolicyPageTitle=Конфиденциальность и условия
russian.PolicyPageDescription=Прочитайте политику конфиденциальности и условия использования установки.
russian.PolicyAccept=Я прочитал(а) и принимаю политику конфиденциальности и условия использования.
russian.PolicyAcceptRequired=Примите условия, чтобы продолжить установку.
russian.PolicyMissing=Не удалось получить условия с сервера. Их можно найти по адресу:
russian.PolicyLoading=Загрузка условий...
russian.DownloadingClient=Загрузка клиента...

japanese.PolicyPageTitle=プライバシーと利用規約
japanese.PolicyPageDescription=インストールのプライバシーポリシーと利用規約をお読みください。
japanese.PolicyAccept=プライバシーポリシーと利用規約を読み、同意します。
japanese.PolicyAcceptRequired=インストールを続けるには規約に同意してください。
japanese.PolicyMissing=サーバーから規約を取得できませんでした。次のアドレスで確認できます:
japanese.PolicyLoading=規約を読み込んでいます...
japanese.DownloadingClient=クライアントをダウンロードしています...

zhcn.PolicyPageTitle=隐私与条款
zhcn.PolicyPageDescription=请阅读本安装的隐私政策和服务条款。
zhcn.PolicyAccept=我已阅读并同意隐私政策和服务条款。
zhcn.PolicyAcceptRequired=请同意条款以继续安装。
zhcn.PolicyMissing=无法从服务器读取条款。您可以在以下地址查看：
zhcn.PolicyLoading=正在加载条款...
zhcn.DownloadingClient=正在下载客户端...

zhtw.PolicyPageTitle=隱私權與條款
zhtw.PolicyPageDescription=請閱讀本安裝的隱私權政策與服務條款。
zhtw.PolicyAccept=我已閱讀並同意隱私權政策與服務條款。
zhtw.PolicyAcceptRequired=請同意條款以繼續安裝。
zhtw.PolicyMissing=無法從伺服器讀取條款。您可以在以下網址查看：
zhtw.PolicyLoading=正在載入條款...
zhtw.DownloadingClient=正在下載用戶端...

vietnamese.PolicyPageTitle=Quyền riêng tư và điều khoản
vietnamese.PolicyPageDescription=Hãy đọc chính sách quyền riêng tư và điều khoản sử dụng của bản cài đặt.
vietnamese.PolicyAccept=Tôi đã đọc và đồng ý với chính sách quyền riêng tư và điều khoản sử dụng.
vietnamese.PolicyAcceptRequired=Hãy đồng ý với các điều khoản để tiếp tục cài đặt.
vietnamese.PolicyMissing=Không thể đọc điều khoản từ máy chủ. Bạn có thể xem tại:
vietnamese.PolicyLoading=Đang tải điều khoản...
vietnamese.DownloadingClient=Đang tải ứng dụng...
[Files]
Source: "..\binaries\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\desktop\assets\byog.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\desktop\assets\byog.ico"; Flags: dontcopy
Source: "..\desktop\assets\logo.png"; Flags: dontcopy
Source: "..\desktop\assets\wizard_small.bmp"; Flags: dontcopy

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "{code:GetDisplayName}"; ValueData: """{app}\{#MyAppExeName}"" --tray"; Flags: uninsdeletevalue

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
  BrandFaviconBase64: String;
  BrandFaviconIcoBase64: String;
  BrandFaviconBmpBase64: String;
  PolicyPage: TWizardPage;
  PolicyMemo: TNewMemo;
  PolicyAccept: TNewCheckBox;
  PoliciesLoaded: Boolean;

const
  CRYPT_STRING_BASE64_ANY = $00000006;

function CryptStringToBinary(
  pszString: String;
  cchString: DWORD;
  dwFlags: DWORD;
  pbBinary: AnsiString;
  var pcbBinary: DWORD;
  pdwSkip: DWORD;
  pdwFlags: DWORD
): BOOL;
external 'CryptStringToBinaryW@crypt32.dll stdcall';

function LoadBranding(): Boolean; forward;

procedure DecodeB64ToFile(const S: String; const OutFile: String);
var
  CleanS: String;
  CommaPos: Integer;
  OutLen: DWORD;
  Buffer: AnsiString;
begin
  CleanS := S;
  CommaPos := Pos(',', CleanS);
  if CommaPos > 0 then
    CleanS := Copy(CleanS, CommaPos + 1, Length(CleanS) - CommaPos);
  CleanS := Trim(CleanS);
  if CleanS = '' then
    Exit;

  OutLen := 0;
  if not CryptStringToBinary(CleanS, Length(CleanS), CRYPT_STRING_BASE64_ANY, '', OutLen, 0, 0) then
    Exit;
  if OutLen = 0 then
    Exit;

  SetLength(Buffer, OutLen);
  if CryptStringToBinary(CleanS, Length(CleanS), CRYPT_STRING_BASE64_ANY, Buffer, OutLen, 0, 0) then
  begin
    SaveStringToFile(OutFile, Buffer, False);
  end;
end;

function EnsureBrandIcon(): String; forward;
function EnsureBrandLogo(): String; forward;
function EnsureBrandLogoBmp(): String; forward;
function ShowCustomLanguageDialog(): Boolean; forward;

const
  WM_SETICON = $0080;
  ICON_SMALL = 0;
  ICON_BIG = 1;
  IMAGE_ICON = 1;
  LR_LOADFROMFILE = $0010;
  GW_OWNER = 4;

function LoadImageW(hInst: THandle; ImageName: String; ImageType: UINT; X, Y: Integer; Flags: UINT): THandle; external 'LoadImageW@user32.dll stdcall';
function SendMessageW(hWnd: HWND; Msg: UINT; wParam: Longint; lParam: Longint): Longint; external 'SendMessageW@user32.dll stdcall';
function GetWindow(hWnd: HWND; uCmd: UINT): HWND; external 'GetWindow@user32.dll stdcall';

procedure ApplyWindowIcon(const WindowHandle: HWND; const IcoFile: String);
var
  hIconBig, hIconSmall: THandle;
  hOwner: HWND;
begin
  if (IcoFile = '') or (not FileExists(IcoFile)) then
    Exit;
  try
    hIconBig := LoadImageW(0, IcoFile, IMAGE_ICON, ScaleX(32), ScaleY(32), LR_LOADFROMFILE);
    hIconSmall := LoadImageW(0, IcoFile, IMAGE_ICON, ScaleX(16), ScaleY(16), LR_LOADFROMFILE);
    if hIconBig <> 0 then
    begin
      SendMessageW(WindowHandle, WM_SETICON, ICON_BIG, hIconBig);
      hOwner := GetWindow(WindowHandle, GW_OWNER);
      if hOwner <> 0 then
        SendMessageW(hOwner, WM_SETICON, ICON_BIG, hIconBig);
    end;
    if hIconSmall <> 0 then
    begin
      SendMessageW(WindowHandle, WM_SETICON, ICON_SMALL, hIconSmall);
      hOwner := GetWindow(WindowHandle, GW_OWNER);
      if hOwner <> 0 then
        SendMessageW(hOwner, WM_SETICON, ICON_SMALL, hIconSmall);
    end;
  except
  end;
end;

var
  LangDialogForm: TSetupForm;
  LangDialogLogo: TBitmapImage;
  LangDialogHeader: TLabel;
  LangDialogPrompt: TLabel;
  LangDialogCombo: TNewComboBox;
  LangDialogOkBtn: TNewButton;
  LangDialogCancelBtn: TNewButton;
  LangDialogKeys: array of String;

function GetUserDefaultUILanguage(): Word; external 'GetUserDefaultUILanguage@kernel32.dll stdcall';

function DetectDefaultLanguage(): String;
var
  LangID, Primary: Word;
begin
  LangID := GetUserDefaultUILanguage();
  Primary := LangID and $3FF;
  case Primary of
    $1F: Result := 'turkish';
    $0C: Result := 'french';
    $19: Result := 'russian';
    $11: Result := 'japanese';
    $2A: Result := 'vietnamese';
    $04:
      begin
        if LangID = $0804 then
          Result := 'zhcn'
        else if (LangID = $0404) or (LangID = $0C04) then
          Result := 'zhtw'
        else
          Result := 'zhcn';
      end;
    $09: Result := 'english';
  else
    Result := 'english';
  end;
end;

procedure UpdateLanguageDialogText(const LangKey: String);
begin
  if LangKey = 'turkish' then
  begin
    LangDialogForm.Caption := 'Kurulum Dili Seçimi';
    LangDialogHeader.Caption := 'Kurulum Dilini Belirleyin';
    LangDialogPrompt.Caption := 'Lütfen kurulum sırasında kullanılacak dili seçin:';
    LangDialogOkBtn.Caption := 'Devam Et';
    LangDialogCancelBtn.Caption := 'İptal';
  end
  else if LangKey = 'french' then
  begin
    LangDialogForm.Caption := 'Sélection de la langue';
    LangDialogHeader.Caption := 'Définir la langue d''installation';
    LangDialogPrompt.Caption := 'Veuillez sélectionner la langue à utiliser :';
    LangDialogOkBtn.Caption := 'Continuer';
    LangDialogCancelBtn.Caption := 'Annuler';
  end
  else if LangKey = 'russian' then
  begin
    LangDialogForm.Caption := 'Выбор языка';
    LangDialogHeader.Caption := 'Язык установки';
    LangDialogPrompt.Caption := 'Пожалуйста, выберите язык установки:';
    LangDialogOkBtn.Caption := 'Продолжить';
    LangDialogCancelBtn.Caption := 'Отмена';
  end
  else if LangKey = 'japanese' then
  begin
    LangDialogForm.Caption := '言語の選択';
    LangDialogHeader.Caption := 'セットアップ言語の選択';
    LangDialogPrompt.Caption := 'インストール中に使用する言語を選択してください:';
    LangDialogOkBtn.Caption := '次へ';
    LangDialogCancelBtn.Caption := 'キャンセル';
  end
  else if LangKey = 'zhcn' then
  begin
    LangDialogForm.Caption := '语言选择';
    LangDialogHeader.Caption := '设置安装语言';
    LangDialogPrompt.Caption := '请选择在安装期间使用的语言：';
    LangDialogOkBtn.Caption := '继续';
    LangDialogCancelBtn.Caption := '取消';
  end
  else if LangKey = 'zhtw' then
  begin
    LangDialogForm.Caption := '語言選擇';
    LangDialogHeader.Caption := '設定安裝語言';
    LangDialogPrompt.Caption := '請選擇在安裝期間使用的語言：';
    LangDialogOkBtn.Caption := '繼續';
    LangDialogCancelBtn.Caption := '取消';
  end
  else if LangKey = 'vietnamese' then
  begin
    LangDialogForm.Caption := 'Chọn ngôn ngữ';
    LangDialogHeader.Caption := 'Cài đặt ngôn ngữ';
    LangDialogPrompt.Caption := 'Vui lòng chọn ngôn ngữ sử dụng khi cài đặt:';
    LangDialogOkBtn.Caption := 'Tiếp tục';
    LangDialogCancelBtn.Caption := 'Hủy';
  end
  else
  begin
    LangDialogForm.Caption := 'Select Setup Language';
    LangDialogHeader.Caption := 'Select Setup Language';
    LangDialogPrompt.Caption := 'Please select the language to use during installation:';
    LangDialogOkBtn.Caption := 'Continue';
    LangDialogCancelBtn.Caption := 'Cancel';
  end;
end;

procedure OnLanguageComboChange(Sender: TObject);
var
  Idx: Integer;
begin
  Idx := LangDialogCombo.ItemIndex;
  if (Idx >= 0) and (Idx < GetArrayLength(LangDialogKeys)) then
    UpdateLanguageDialogText(LangDialogKeys[Idx]);
end;

function ShowCustomLanguageDialog(): Boolean;
var
  DefaultLang, ChosenLang, IcoPath, BmpPath, P, CmdLine: String;
  I, ErrorCode: Integer;
begin
  Result := False;
  DefaultLang := DetectDefaultLanguage();

  SetArrayLength(LangDialogKeys, 8);
  LangDialogKeys[0] := 'turkish';
  LangDialogKeys[1] := 'english';
  LangDialogKeys[2] := 'zhcn';
  LangDialogKeys[3] := 'zhtw';
  LangDialogKeys[4] := 'french';
  LangDialogKeys[5] := 'russian';
  LangDialogKeys[6] := 'japanese';
  LangDialogKeys[7] := 'vietnamese';

  LangDialogForm := CreateCustomForm(ScaleX(400), ScaleY(165), False, True);
  try
    IcoPath := EnsureBrandIcon();
    ApplyWindowIcon(LangDialogForm.Handle, IcoPath);
    LangDialogForm.ClientWidth := ScaleX(400);
    LangDialogForm.ClientHeight := ScaleY(165);
    LangDialogForm.Position := poScreenCenter;
    LangDialogForm.BorderStyle := bsDialog;

    LangDialogLogo := TBitmapImage.Create(LangDialogForm);
    LangDialogLogo.Parent := LangDialogForm;
    LangDialogLogo.Left := ScaleX(18);
    LangDialogLogo.Top := ScaleY(18);
    LangDialogLogo.Width := ScaleX(48);
    LangDialogLogo.Height := ScaleY(48);
    LangDialogLogo.Stretch := True;
    BmpPath := EnsureBrandLogoBmp();
    if FileExists(BmpPath) then
    begin
      try
        LangDialogLogo.Bitmap.LoadFromFile(BmpPath);
      except
      end;
    end;

    LangDialogHeader := TLabel.Create(LangDialogForm);
    LangDialogHeader.Parent := LangDialogForm;
    LangDialogHeader.Left := ScaleX(80);
    LangDialogHeader.Top := ScaleY(16);
    LangDialogHeader.Width := ScaleX(300);
    LangDialogHeader.Font.Size := 10;
    LangDialogHeader.Font.Style := [fsBold];

    LangDialogPrompt := TLabel.Create(LangDialogForm);
    LangDialogPrompt.Parent := LangDialogForm;
    LangDialogPrompt.Left := ScaleX(80);
    LangDialogPrompt.Top := ScaleY(40);
    LangDialogPrompt.Width := ScaleX(300);
    LangDialogPrompt.Font.Size := 8;

    LangDialogCombo := TNewComboBox.Create(LangDialogForm);
    LangDialogCombo.Parent := LangDialogForm;
    LangDialogCombo.Left := ScaleX(80);
    LangDialogCombo.Top := ScaleY(66);
    LangDialogCombo.Width := ScaleX(300);
    LangDialogCombo.Height := ScaleY(24);
    LangDialogCombo.Style := csDropDownList;

    LangDialogCombo.Items.Add('Türkçe');
    LangDialogCombo.Items.Add('English');
    LangDialogCombo.Items.Add('简体中文 (Simplified Chinese)');
    LangDialogCombo.Items.Add('繁體中文 (Traditional Chinese)');
    LangDialogCombo.Items.Add('Français (French)');
    LangDialogCombo.Items.Add('Русский (Russian)');
    LangDialogCombo.Items.Add('日本語 (Japanese)');
    LangDialogCombo.Items.Add('Tiếng Việt (Vietnamese)');

    LangDialogCombo.ItemIndex := 1;
    for I := 0 to 7 do
    begin
      if LangDialogKeys[I] = DefaultLang then
      begin
        LangDialogCombo.ItemIndex := I;
        Break;
      end;
    end;

    LangDialogOkBtn := TNewButton.Create(LangDialogForm);
    LangDialogOkBtn.Parent := LangDialogForm;
    LangDialogOkBtn.Left := ScaleX(190);
    LangDialogOkBtn.Top := ScaleY(115);
    LangDialogOkBtn.Width := ScaleX(95);
    LangDialogOkBtn.Height := ScaleY(28);
    LangDialogOkBtn.ModalResult := mrOk;
    LangDialogOkBtn.Default := True;

    LangDialogCancelBtn := TNewButton.Create(LangDialogForm);
    LangDialogCancelBtn.Parent := LangDialogForm;
    LangDialogCancelBtn.Left := ScaleX(295);
    LangDialogCancelBtn.Top := ScaleY(115);
    LangDialogCancelBtn.Width := ScaleX(85);
    LangDialogCancelBtn.Height := ScaleY(28);
    LangDialogCancelBtn.ModalResult := mrCancel;

    LangDialogCombo.OnChange := @OnLanguageComboChange;
    OnLanguageComboChange(LangDialogCombo);

    if LangDialogForm.ShowModal = mrOk then
    begin
      ChosenLang := LangDialogKeys[LangDialogCombo.ItemIndex];
      // If the selected language matches the current ActiveLanguage, continue immediately without relaunch
      if CompareText(ChosenLang, ActiveLanguage) = 0 then
      begin
        Result := True;
        Exit;
      end;

      // Only relaunch if the user explicitly chose a different language
      CmdLine := '/LANG=' + ChosenLang + ' /LANGSELECTED=1';
      for I := 1 to ParamCount do
      begin
        P := ParamStr(I);
        // Exclude all Inno internal parameters (/SL5, /SPAWNWND, /RESTARTEXITCODE, etc.) and prior lang flags
        if (CompareText(Copy(P, 1, 6), '/LANG=') <> 0) and
           (CompareText(P, '/LANGSELECTED=1') <> 0) and
           (CompareText(P, '/LANGSELECTED') <> 0) and
           (CompareText(Copy(P, 1, 5), '/SL5=') <> 0) and
           (CompareText(Copy(P, 1, 10), '/SPAWNWND=') <> 0) and
           (CompareText(Copy(P, 1, 17), '/RESTARTEXITCODE=') <> 0) then
          CmdLine := CmdLine + ' ' + AddQuotes(P);
      end;

      if not ShellExec('open', ExpandConstant('{srcexe}'), CmdLine, '', SW_SHOWNORMAL, ewNoWait, ErrorCode) then
        Exec(ExpandConstant('{srcexe}'), CmdLine, '', SW_SHOWNORMAL, ewNoWait, ErrorCode);
      Result := False;
    end
    else
    begin
      Result := False;
    end;
  finally
    LangDialogForm.Free;
  end;
end;

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
      BrandFaviconBase64 := JsonText(document, 'favicon_base64');
      BrandFaviconIcoBase64 := JsonText(document, 'favicon_ico_base64');
      BrandFaviconBmpBase64 := JsonText(document, 'favicon_bmp_base64');
      Result := (BrandServerUrl <> '') or (BrandSystemName <> '');
    finally
      stream.Free;
    end;
  except
    Result := False;
  end;
end;

function EnsureBrandIcon(): String;
var
  icoPath: String;
begin
  icoPath := ExpandConstant('{tmp}\brand_icon.ico');
  if FileExists(icoPath) then
  begin
    Result := icoPath;
    Exit;
  end;
  if BrandFaviconIcoBase64 <> '' then
  begin
    DecodeB64ToFile(BrandFaviconIcoBase64, icoPath);
    if FileExists(icoPath) then
    begin
      Result := icoPath;
      Exit;
    end;
  end;
  icoPath := ExpandConstant('{tmp}\byog.ico');
  if not FileExists(icoPath) then
  begin
    try
      ExtractTemporaryFile('byog.ico');
    except
    end;
  end;
  Result := icoPath;
end;

function EnsureBrandLogo(): String;
var
  logoPath: String;
begin
  logoPath := ExpandConstant('{tmp}\brand_logo.png');
  if FileExists(logoPath) then
  begin
    Result := logoPath;
    Exit;
  end;
  if BrandFaviconBase64 <> '' then
  begin
    DecodeB64ToFile(BrandFaviconBase64, logoPath);
    if FileExists(logoPath) then
    begin
      Result := logoPath;
      Exit;
    end;
  end;
  logoPath := ExpandConstant('{tmp}\logo.png');
  if not FileExists(logoPath) then
  begin
    try
      ExtractTemporaryFile('logo.png');
    except
    end;
  end;
  Result := logoPath;
end;

function EnsureBrandLogoBmp(): String;
var
  bmpPath: String;
begin
  bmpPath := ExpandConstant('{tmp}\brand_logo.bmp');
  if FileExists(bmpPath) then
  begin
    Result := bmpPath;
    Exit;
  end;
  if BrandFaviconBmpBase64 <> '' then
  begin
    DecodeB64ToFile(BrandFaviconBmpBase64, bmpPath);
    if FileExists(bmpPath) then
    begin
      Result := bmpPath;
      Exit;
    end;
  end;
  bmpPath := ExpandConstant('{tmp}\wizard_small.bmp');
  if not FileExists(bmpPath) then
  begin
    try
      ExtractTemporaryFile('wizard_small.bmp');
    except
    end;
  end;
  Result := bmpPath;
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
    request.SetTimeouts(2000, 3000, 3000, 4000);
    request.Send;
    if request.Status = 200 then
    begin
      Result := request.ResponseText;
      StringChangeEx(Result, #13#10, #10, True);
      StringChangeEx(Result, #10, #13#10, True);
    end;
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
  icoPath, bmpPath: String;
begin
  icoPath := EnsureBrandIcon();
  ApplyWindowIcon(WizardForm.Handle, icoPath);

  bmpPath := EnsureBrandLogoBmp();
  if FileExists(bmpPath) then
  begin
    try
      WizardForm.WizardSmallBitmapImage.Bitmap.LoadFromFile(bmpPath);
      WizardForm.WizardSmallBitmapImage.Stretch := True;
    except
    end;
  end;

  PoliciesLoaded := False;

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
  PolicyMemo.Text := ExpandConstant('{cm:PolicyLoading}');

  PolicyAccept := TNewCheckBox.Create(PolicyPage);
  PolicyAccept.Parent := PolicyPage.Surface;
  PolicyAccept.Left := 0;
  PolicyAccept.Top := PolicyMemo.Height + ScaleY(10);
  PolicyAccept.Width := PolicyPage.SurfaceWidth;
  PolicyAccept.Caption := ExpandConstant('{cm:PolicyAccept}');
end;

// The policies are read when the page is shown. Reading them while the wizard is
// still being prepared keeps the wizard hidden on a slow or unreachable server.
procedure CurPageChanged(CurPageID: Integer);
var
  policies, fallbackUrl: String;
begin
  if (not PoliciesLoaded) and (PolicyPage <> nil) and (CurPageID = PolicyPage.ID) then
  begin
    PoliciesLoaded := True;
    policies := ReadPolicies();
    if policies <> '' then
      PolicyMemo.Text := policies
    else
    begin
      fallbackUrl := BrandServerUrl;
      while (Length(fallbackUrl) > 0) and (fallbackUrl[Length(fallbackUrl)] = '/') do
        SetLength(fallbackUrl, Length(fallbackUrl) - 1);
      if fallbackUrl <> '' then
        fallbackUrl := fallbackUrl + '/privacy';
      PolicyMemo.Text := ExpandConstant('{cm:PolicyMissing}') + #13#10#13#10 + fallbackUrl;
    end;
  end;
end;

function InitializeSetup(): Boolean;
var
  I: Integer;
  AlreadySelected: Boolean;
begin
  BrandingLoaded := LoadBranding();
  Log('BYOG branding: ' + IntToStr(Ord(BrandingLoaded)) + ' | ' +
    BrandSystemName + ' | ' + BrandServerUrl);

  AlreadySelected := False;
  for I := 1 to ParamCount do
  begin
    if (CompareText(Copy(ParamStr(I), 1, 6), '/LANG=') = 0) or
       (CompareText(ParamStr(I), '/LANGSELECTED=1') = 0) or
       (CompareText(ParamStr(I), '/LANGSELECTED') = 0) then
    begin
      AlreadySelected := True;
      Break;
    end;
  end;

  if WizardSilent() then
    AlreadySelected := True;

  if not AlreadySelected then
  begin
    Result := ShowCustomLanguageDialog();
    Exit;
  end;

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
      Exit;
    end;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    writeInstallerLanguage();
    if FileExists(ExpandConstant('{tmp}\brand_icon.ico')) then
      CopyFile(ExpandConstant('{tmp}\brand_icon.ico'), ExpandConstant('{app}\byog.ico'), False);
    if BrandingLoaded then
      AppendBrandingToClient();
    WriteClientConfig();
  end;
end;





