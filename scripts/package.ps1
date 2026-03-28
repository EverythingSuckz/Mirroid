#Requires -Version 5.1
<#
.SYNOPSIS
    Build Mirroid locally with bundled dependencies.

.DESCRIPTION
    Full local build pipeline:
      1. Fetch adb + scrcpy into _bundled/ (skipped if already present)
      2. Build the Go binary via fyne package
      3. Optionally build the Inno Setup installer
      4. Optionally create a portable zip

.PARAMETER Installer
    Build the Inno Setup installer (requires Inno Setup 6 installed).

.PARAMETER Portable
    Create a portable zip (Mirroid.exe + all deps flat).

.PARAMETER Clean
    Remove _bundled/ and build artifacts before starting.

.PARAMETER SkipDeps
    Skip dependency fetch (use existing _bundled/).

.EXAMPLE
    .\scripts\package.ps1                          # build only
    .\scripts\package.ps1 -Installer               # build + installer
    .\scripts\package.ps1 -Portable                # build + portable zip
    .\scripts\package.ps1 -Installer -Portable     # build + both
    .\scripts\package.ps1 -Clean -Installer        # fresh build + installer
#>

param(
    [switch]$Installer,
    [switch]$Portable,
    [switch]$Clean,
    [switch]$SkipDeps
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# ── Paths ──
$ScriptDir   = Split-Path -Parent $PSScriptRoot  # repo root (scripts/ is one level down)
if (-not $ScriptDir) {
    $ScriptDir = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
}
$ProjectRoot = $ScriptDir
$BundledDir  = Join-Path $ProjectRoot "_bundled"
$FetchScript = Join-Path $ProjectRoot "packaging\fetch-deps.ps1"
$IssFile     = Join-Path $ProjectRoot "packaging\windows\mirroid.iss"

# ── Helpers ──
function Write-Step($msg)  { Write-Host "`n  >> $msg" -ForegroundColor Cyan }
function Write-Done($msg)  { Write-Host "  OK $msg" -ForegroundColor Green }
function Write-Skip($msg)  { Write-Host "  -- $msg" -ForegroundColor DarkGray }
function Write-Err($msg)   { Write-Host "  !! $msg" -ForegroundColor Red }

# ── Clean ──
if ($Clean) {
    Write-Step "Cleaning build artifacts..."
    foreach ($item in @("$ProjectRoot\Mirroid.exe", "$ProjectRoot\mirroid-windows-amd64-portable.zip", "$ProjectRoot\mirroid-windows-amd64-setup.exe")) {
        if (Test-Path $item) { Remove-Item -Force $item; Write-Host "    removed $item" }
    }
    # Pass -Clean through to fetch-deps
}

# ── Step 1: Fetch dependencies ──
if ($SkipDeps) {
    Write-Skip "Skipping dependency fetch (-SkipDeps)"
} else {
    Write-Step "Fetching dependencies..."
    $fetchArgs = @()
    if ($Clean) { $fetchArgs += "-Clean" }
    & $FetchScript @fetchArgs
    if ($LASTEXITCODE -and $LASTEXITCODE -ne 0) {
        Write-Err "Dependency fetch failed"; exit 1
    }
}

# Verify _bundled/ exists
if (-not (Test-Path (Join-Path $BundledDir "adb.exe"))) {
    Write-Err "_bundled/adb.exe not found. Run without -SkipDeps first."
    exit 1
}

# ── Step 2: Build with fyne ──
Write-Step "Building Mirroid with fyne package..."
Push-Location $ProjectRoot
try {
    fyne package --target windows --release
    if ($LASTEXITCODE -ne 0) {
        Write-Err "fyne package failed"; exit 1
    }
    Write-Done "Mirroid.exe"
} finally {
    Pop-Location
}

# Verify binary exists
if (-not (Test-Path "$ProjectRoot\Mirroid.exe")) {
    Write-Err "Mirroid.exe not found after build"; exit 1
}

# ── Step 3: Installer (optional) ──
if ($Installer) {
    $iscc = "C:\Program Files (x86)\Inno Setup 6\ISCC.exe"
    if (-not (Test-Path $iscc)) {
        Write-Err "Inno Setup 6 not found at $iscc"
        Write-Host "    Install: choco install innosetup -y" -ForegroundColor DarkYellow
        exit 1
    }
    Write-Step "Building installer..."
    & $iscc "/DMyAppVersion=local" $IssFile
    if ($LASTEXITCODE -ne 0) {
        Write-Err "Inno Setup failed"; exit 1
    }
    Write-Done "mirroid-windows-amd64-setup.exe"
}

# ── Step 4: Portable zip (optional) ──
if ($Portable) {
    Write-Step "Creating portable zip..."
    $portableDir = Join-Path $ProjectRoot "_portable"
    if (Test-Path $portableDir) { Remove-Item -Recurse -Force $portableDir }
    New-Item -ItemType Directory -Force -Path $portableDir | Out-Null

    Copy-Item "$ProjectRoot\Mirroid.exe" $portableDir
    Copy-Item "$BundledDir\*" $portableDir -Recurse -Force

    $zipPath = Join-Path $ProjectRoot "mirroid-windows-amd64-portable.zip"
    if (Test-Path $zipPath) { Remove-Item -Force $zipPath }
    Compress-Archive -Path "$portableDir\*" -DestinationPath $zipPath

    Remove-Item -Recurse -Force $portableDir
    Write-Done "mirroid-windows-amd64-portable.zip"
}

# ── Summary ──
Write-Host ""
Write-Host "Build complete!" -ForegroundColor Green
Write-Host ""

$outputs = @("$ProjectRoot\Mirroid.exe")
if ($Installer) { $outputs += "$ProjectRoot\mirroid-windows-amd64-setup.exe" }
if ($Portable)  { $outputs += "$ProjectRoot\mirroid-windows-amd64-portable.zip" }

foreach ($out in $outputs) {
    if (Test-Path $out) {
        $sizeMB = [math]::Round((Get-Item $out).Length / 1MB, 1)
        Write-Host "  $([System.IO.Path]::GetFileName($out)) ($sizeMB MB)" -ForegroundColor White
    }
}
Write-Host ""
