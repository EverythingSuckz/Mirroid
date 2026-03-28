#Requires -Version 5.1
<#
.SYNOPSIS
    Downloads pinned adb and scrcpy dependencies for local builds.

.DESCRIPTION
    Fetches the same pinned versions used by CI into _bundled/ at the project root.
    After running this, you can:
      - Build the installer:  & "C:\Program Files (x86)\Inno Setup 6\ISCC.exe" packaging\windows\mirroid.iss
      - Create a portable zip: Compress-Archive -Path Mirroid.exe, _bundled\* -DestinationPath mirroid-portable.zip
      - Just run the app with bundled deps alongside the exe

.PARAMETER Clean
    Remove _bundled/ directory before downloading (fresh fetch).

.EXAMPLE
    .\packaging\fetch-deps.ps1
    .\packaging\fetch-deps.ps1 -Clean
#>

param(
    [switch]$Clean
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# ── Pinned versions (keep in sync with .github/workflows/release.yml) ──
$ScrcpyVersion        = "3.3.4"
$PlatformToolsVersion = "37.0.0"

# ── Paths ──
# Script lives at packaging/fetch-deps.ps1, so one level up = project root
$ProjectRoot = Split-Path -Parent $PSScriptRoot
if (-not $ProjectRoot) {
    $ProjectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
}
$BundledDir = Join-Path $ProjectRoot "_bundled"
$TempDir    = Join-Path $env:TEMP "mirroid-deps"

# ── URLs ──
$AdbUrl    = "https://dl.google.com/android/repository/platform-tools_r${PlatformToolsVersion}-win.zip"
$ScrcpyUrl = "https://github.com/Genymobile/scrcpy/releases/download/v${ScrcpyVersion}/scrcpy-win64-v${ScrcpyVersion}.zip"

# ── Helpers ──
function Write-Step($msg) { Write-Host "  -> $msg" -ForegroundColor Cyan }
function Write-Done($msg) { Write-Host "  OK $msg" -ForegroundColor Green }

# ── Main ──
Write-Host ""
Write-Host "Mirroid dependency fetcher" -ForegroundColor Yellow
Write-Host "  scrcpy:         v$ScrcpyVersion"
Write-Host "  platform-tools: r$PlatformToolsVersion"
Write-Host ""

if ($Clean -and (Test-Path $BundledDir)) {
    Write-Step "Cleaning $BundledDir"
    Remove-Item -Recurse -Force $BundledDir
}

if (Test-Path $BundledDir) {
    $existing = Get-ChildItem $BundledDir -File | Measure-Object
    if ($existing.Count -gt 0) {
        Write-Host "  _bundled/ already exists with $($existing.Count) files. Use -Clean to re-fetch." -ForegroundColor DarkYellow
        Write-Host ""
        exit 0
    }
}

New-Item -ItemType Directory -Force -Path $BundledDir | Out-Null
New-Item -ItemType Directory -Force -Path $TempDir | Out-Null

# ── Download and extract adb ──
Write-Step "Downloading adb platform-tools r$PlatformToolsVersion..."
$adbZip = Join-Path $TempDir "platform-tools.zip"
Invoke-WebRequest -Uri $AdbUrl -OutFile $adbZip -UseBasicParsing

Write-Step "Extracting adb files..."
$adbExtract = Join-Path $TempDir "pt"
Expand-Archive -Path $adbZip -DestinationPath $adbExtract -Force

Copy-Item (Join-Path $adbExtract "platform-tools\adb.exe")          $BundledDir
Copy-Item (Join-Path $adbExtract "platform-tools\AdbWinApi.dll")     $BundledDir
Copy-Item (Join-Path $adbExtract "platform-tools\AdbWinUsbApi.dll")  $BundledDir
Write-Done "adb.exe + DLLs"

# ── Download and extract scrcpy ──
Write-Step "Downloading scrcpy v$ScrcpyVersion..."
$scrcpyZip = Join-Path $TempDir "scrcpy.zip"
Invoke-WebRequest -Uri $ScrcpyUrl -OutFile $scrcpyZip -UseBasicParsing

Write-Step "Extracting scrcpy files..."
$scrcpyExtract = Join-Path $TempDir "scrcpy"
Expand-Archive -Path $scrcpyZip -DestinationPath $scrcpyExtract -Force

# The zip contains a top-level directory like scrcpy-win64-v3.3.4/ — copy its contents flat
$scrcpyInner = Get-ChildItem $scrcpyExtract -Directory | Select-Object -First 1
if ($scrcpyInner) {
    Copy-Item (Join-Path $scrcpyInner.FullName "*") $BundledDir -Recurse -Force
} else {
    Copy-Item (Join-Path $scrcpyExtract "*") $BundledDir -Recurse -Force
}
Write-Done "scrcpy + DLLs"

# ── Cleanup temp ──
Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue

# ── Summary ──
$files = Get-ChildItem $BundledDir -File
Write-Host ""
Write-Host "Done! $($files.Count) files in _bundled/:" -ForegroundColor Green
foreach ($f in $files) {
    $sizeMB = [math]::Round($f.Length / 1MB, 1)
    Write-Host "  $($f.Name) ($sizeMB MB)"
}
$totalMB = [math]::Round(($files | Measure-Object -Property Length -Sum).Sum / 1MB, 1)
Write-Host "  ────────────"
Write-Host "  Total: $totalMB MB"
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "  1. Build:     go build -o Mirroid.exe ."
Write-Host "  2. Installer: & 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe' packaging\windows\mirroid.iss"
Write-Host "  3. Portable:  (copy Mirroid.exe into _bundled/ and zip)"
Write-Host ""
