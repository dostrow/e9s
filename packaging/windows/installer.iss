#ifndef AppVersion
  #error AppVersion must be provided by build-installer.ps1
#endif
#ifndef ArtifactVersion
  #error ArtifactVersion must be provided by build-installer.ps1
#endif
#ifndef BundleDirectory
  #error BundleDirectory must be provided by build-installer.ps1
#endif
#ifndef OutputDirectory
  #error OutputDirectory must be provided by build-installer.ps1
#endif
#ifndef IconFile
  #error IconFile must be provided by build-installer.ps1
#endif

[Setup]
AppId={{DB218972-A6B5-492A-A358-41B2D57437F7}
AppName=e9s
AppVersion={#AppVersion}
AppVerName=e9s {#AppVersion}
AppPublisher=Daniel Ostrow
AppPublisherURL=https://github.com/dostrow/e9s
AppSupportURL=https://github.com/dostrow/e9s/issues
AppUpdatesURL=https://github.com/dostrow/e9s/releases
DefaultDirName={localappdata}\Programs\e9s
DefaultGroupName=e9s
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.17763
SetupIconFile={#IconFile}
UninstallDisplayIcon={app}\e9s-gui.exe
UninstallDisplayName=e9s
OutputDir={#OutputDirectory}
OutputBaseFilename=e9s-gui-{#ArtifactVersion}-windows-amd64-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
RestartApplications=no

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked

[Files]
Source: "{#BundleDirectory}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autoprograms}\e9s"; Filename: "{app}\e9s-gui.exe"; WorkingDir: "{app}"; IconFilename: "{app}\e9s.ico"; AppUserModelID: "io.github.dostrow.e9s"
Name: "{autodesktop}\e9s"; Filename: "{app}\e9s-gui.exe"; WorkingDir: "{app}"; IconFilename: "{app}\e9s.ico"; AppUserModelID: "io.github.dostrow.e9s"; Tasks: desktopicon

[Run]
Filename: "{app}\e9s-gui.exe"; Description: "Launch e9s"; Flags: nowait postinstall skipifsilent
