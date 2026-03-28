#ifndef MyAppVersion
  #define MyAppVersion "0.0.1"
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
InfoAfterFile=info_after.txt
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

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "..\..\Mirroid.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[Code]
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
