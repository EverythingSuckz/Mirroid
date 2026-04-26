#ifndef MyAppVersion
  #define MyAppVersion "0.0.5"
#endif

#define MyAppName "Mirroid"
#define MyAppPublisher "EverythingSuckz"
#define MyAppURL "https://github.com/EverythingSuckz/Mirroid"
#define MyAppExeName "Mirroid.exe"

[Setup]
AppId={{B8F3A2D1-7E4C-4A9B-8D6F-1C2E5A3B9D7F}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}/issues
AppUpdatesURL={#MyAppURL}/releases
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
DisableWelcomePage=no
OutputDir=..\..\
OutputBaseFilename=mirroid-windows-amd64-setup
SetupIconFile=icon.ico
WizardStyle=modern
WizardImageFile=..\..\assets\wizard_img.png
WizardSmallImageFile=..\..\assets\wizard_small_img.png
Compression=lzma2
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayIcon={app}\{#MyAppExeName}
PrivilegesRequired=admin
MinVersion=10.0
CloseApplications=force
RestartApplications=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[InstallDelete]
; Clean previous installation so stale files (e.g. bundled adb/scrcpy from a
; different version) don't persist across updates. The uninstaller is recreated
; by the installer after this step.
Type: filesandordirs; Name: "{app}\*"

[Files]
Source: "..\..\Mirroid.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\_bundled\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
; Normal install: show "Launch Mirroid" checkbox on final page
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[Code]
function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  AdbPath: String;
  ResultCode: Integer;
begin
  Result := '';
  AdbPath := ExpandConstant('{app}\adb.exe');
  if FileExists(AdbPath) then begin
    // graceful shutdown of the adb daemon so connected devices get cleaned up
    Exec(AdbPath, 'kill-server', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  end;
  // any leftover adb/scrcpy with the exe still mapped will block InstallDelete
  // and trigger a rollback — RestartManager misses background daemons
  Exec('taskkill.exe', '/F /IM adb.exe /T', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Exec('taskkill.exe', '/F /IM scrcpy.exe /T', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Sleep(500);
end;

procedure ArtCreditClick(Sender: TObject);
var
  ErrorCode: Integer;
begin
  ShellExec('open', 'https://www.figma.com/@NathanCovert', '', '', SW_SHOWNORMAL, ewNoWait, ErrorCode);
end;

procedure InitializeWizard();
var
  PrefixLabel: TNewStaticText;
  LinkLabel: TNewStaticText;
begin
  PrefixLabel := TNewStaticText.Create(WizardForm);
  PrefixLabel.Parent := WizardForm.WelcomePage;
  PrefixLabel.Caption := 'Art By:';
  PrefixLabel.Font.Size := 7;
  PrefixLabel.Font.Color := clGray;
  PrefixLabel.AutoSize := True;
  PrefixLabel.Left := WizardForm.WelcomeLabel2.Left;
  PrefixLabel.Top := WizardForm.WelcomePage.ClientHeight - ScaleY(20);

  LinkLabel := TNewStaticText.Create(WizardForm);
  LinkLabel.Parent := WizardForm.WelcomePage;
  LinkLabel.Caption := '@NathanCovert';
  LinkLabel.Font.Size := 7;
  LinkLabel.Font.Color := clBlue;
  LinkLabel.Font.Style := [fsUnderline];
  LinkLabel.Cursor := crHand;
  LinkLabel.AutoSize := True;
  LinkLabel.OnClick := @ArtCreditClick;
  LinkLabel.Left := PrefixLabel.Left + PrefixLabel.Width + ScaleX(3);
  LinkLabel.Top := PrefixLabel.Top;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  AppDataDir: String;
begin
  if CurUninstallStep = usPostUninstall then
  begin
    AppDataDir := ExpandConstant('{userappdata}\Mirroid');
    if DirExists(AppDataDir) then
    begin
      if MsgBox('Do you want to remove your Mirroid settings and presets?',
        mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES then
      begin
        DelTree(AppDataDir, True, True, True);
      end;
    end;
  end;
end;
