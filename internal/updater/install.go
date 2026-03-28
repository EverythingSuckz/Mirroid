package updater

// InstallType describes how the app was installed.
type InstallType int

const (
	InstallPortable  InstallType = iota // standalone binary
	InstallInstaller                    // Windows Inno Setup (Program Files)
	InstallAppImage                     // Linux AppImage
	InstallSystem                       // Linux .deb (/usr/bin) — browser only
)
