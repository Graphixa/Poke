# poke Installer Script for Windows
# Installs poke (+ pokedex alias) on Windows (PowerShell)
#
# Usage:
#   irm https://raw.githubusercontent.com/Graphixa/Pokedex/main/scripts/install.ps1 | iex
#   # Or with a specific version:
#   $env:POKE_VERSION="1.0.0"; irm https://raw.githubusercontent.com/Graphixa/Pokedex/main/scripts/install.ps1 | iex
#
# Environment:
#   POKE_VERSION          Version or latest (default: latest)
#   POKE_NONINTERACTIVE=1 Skip "Continue?" (non-interactive install)
#   CI                       When non-empty, prompt is skipped (common on CI runners)
#   NO_COLOR=1               Disable ANSI colors / subdued styling where supported
#

$ErrorActionPreference = "Stop"
$ProgressPreference = 'SilentlyContinue'

function Write-InstallOsLine {
    param([Parameter(Mandatory)][string]$OsName, [Parameter(Mandatory)][string]$ArchName)
    if ($env:NO_COLOR -eq '1') {
        Write-Host "OS: $OsName | ARCH: $ArchName"
        return
    }
    Write-Host -NoNewline -ForegroundColor Blue "OS: "
    Write-Host -NoNewline "$OsName "
    Write-Host -NoNewline -ForegroundColor Blue "| "
    Write-Host -NoNewline -ForegroundColor Blue "ARCH: "
    Write-Host $ArchName
}

$Repo = "Graphixa/Pokedex"
$RepoUrl = "https://github.com/$Repo"

# Detect architecture (Windows arm64 builds are not published yet)
$IsArm64 = $env:PROCESSOR_ARCHITECTURE -eq "ARM64" -or $env:PROCESSOR_ARCHITEW6432 -eq "ARM64"
if ($IsArm64) {
    Write-Error "Windows arm64 builds are not published yet. Use amd64 Windows or install from source."
    exit 1
} elseif ([Environment]::Is64BitOperatingSystem) {
    $Arch = "amd64"
} else {
    Write-Error "32-bit Windows is not supported. poke requires 64-bit Windows."
    exit 1
}

$Version = $env:POKE_VERSION
if (-not $Version -or $Version -eq "") {
    $Version = "latest"
}

if ($Version -eq "latest") {
    $BaseUrl = "$RepoUrl/releases/latest/download"
} else {
    $Version = $Version -replace '^v', ''
    $BaseUrl = "$RepoUrl/releases/download/v$Version"
}

$DisplayVersion = $Version

# GoReleaser archive: poke_<version>_windows_<arch>.zip (see .goreleaser.yaml)
if ($Version -eq "latest") {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $ArchiveName = ($release.assets | Where-Object { $_.name -match "^poke_.+_windows_$Arch\.zip$" } | Select-Object -First 1).name
    if (-not $ArchiveName) {
        Write-Error "Could not find release archive for windows/$Arch"
        exit 1
    }
} else {
    $ArchiveName = "poke_${Version}_windows_${Arch}.zip"
}

$DownloadUrl = "$BaseUrl/$ArchiveName"

# Installation directory (user-local, no admin required)
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\Poke"
$InstalledBin = Join-Path $InstallDir "poke.exe"
$InstalledAlias = Join-Path $InstallDir "pokedex.exe"

Write-InstallOsLine -OsName "windows" -ArchName $Arch
Write-Host ""

Write-Host "This will install poke $DisplayVersion to $InstalledBin"
Write-Host "  (and alias pokedex at $InstalledAlias)"
Write-Host ""

if (Test-Path $InstalledBin) {
    try {
        $CurrentVersion = & $InstalledBin version 2>$null | Select-Object -First 1
        if ($env:NO_COLOR -eq '1') {
            Write-Host "poke is already installed at: $InstalledBin"
            Write-Host "Current version: $CurrentVersion"
            Write-Host "This will be overwritten."
        } else {
            Write-Host "poke is already installed at: $InstalledBin" -ForegroundColor Yellow
            Write-Host "Current version: $CurrentVersion" -ForegroundColor Yellow
            Write-Host "This will be overwritten." -ForegroundColor Yellow
        }
        Write-Host ""
    } catch {
        # Ignore errors when checking version
    }
}

$ShouldPrompt = $true
if ($env:POKE_NONINTERACTIVE -eq '1') {
    $ShouldPrompt = $false
} elseif ($env:CI) {
    $ShouldPrompt = $false
} else {
    try {
        if ([Console]::IsInputRedirected -or [Console]::IsOutputRedirected) {
            $ShouldPrompt = $false
        }
    } catch {
        $ShouldPrompt = $false
    }
}

if ($ShouldPrompt) {
    Write-Host -NoNewline "Continue? [y/N] "
    try {
        $reply = [Console]::ReadLine()
    } catch {
        $reply = "n"
    }
    if ([string]::IsNullOrWhiteSpace($reply)) {
        $reply = "n"
    }
    $replyNorm = $reply.Trim().ToLowerInvariant()
    if ($replyNorm -ne "y" -and $replyNorm -ne "yes") {
        if ($env:NO_COLOR -eq '1') {
            Write-Host "Cancelled."
        } else {
            Write-Host "Cancelled." -ForegroundColor Yellow
        }
        exit 0
    }
    Write-Host ""
}

Write-Host "Creating installation directory..." -ForegroundColor Blue
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$TempZip = Join-Path $env:TEMP "poke-install-$Arch.zip"
$ExtractDir = Join-Path $env:TEMP "poke-install-extract-$Arch"
$TempFile = Join-Path $ExtractDir "poke.exe"
$TempAlias = Join-Path $ExtractDir "pokedex.exe"

Write-Host "Downloading poke..." -ForegroundColor Blue
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TempZip -UseBasicParsing
} catch {
    Write-Error "Failed to download poke"
    if ($Version -ne "latest") {
        Write-Host "Version v$Version may not exist. Check available versions at:" -ForegroundColor Yellow
        Write-Host "$RepoUrl/releases" -ForegroundColor Blue
    }
    exit 1
}

if (-not (Test-Path $TempZip)) {
    Write-Error "Downloaded archive not found"
    exit 1
}

$ChecksumsUrl = "$BaseUrl/checksums.txt"
$ChecksumsTemp = Join-Path $env:TEMP "poke-checksums-$Arch.txt"
Write-Host "Downloading checksums..." -ForegroundColor Blue
try {
    Invoke-WebRequest -Uri $ChecksumsUrl -OutFile $ChecksumsTemp -UseBasicParsing
} catch {
    Write-Error "Failed to download checksums.txt"
    Remove-Item $TempZip -Force -ErrorAction SilentlyContinue
    exit 1
}

$expectedHash = $null
Get-Content -LiteralPath $ChecksumsTemp -ErrorAction SilentlyContinue | ForEach-Object {
    $line = $_.TrimEnd("`r")
    if ($line -match '^\s*([A-Fa-f0-9]{64})\s+[\*]?(.+)$') {
        $name = $Matches[2].Trim()
        if ($name -eq $ArchiveName) {
            $expectedHash = $Matches[1].ToLowerInvariant()
        }
    }
}
Remove-Item $ChecksumsTemp -Force -ErrorAction SilentlyContinue

if (-not $expectedHash) {
    Write-Error "No checksum line for $ArchiveName in checksums.txt"
    Remove-Item $TempZip -Force -ErrorAction SilentlyContinue
    exit 1
}

$actualHash = (Get-FileHash -LiteralPath $TempZip -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actualHash -ne $expectedHash) {
    Write-Error "Checksum mismatch for $ArchiveName"
    Remove-Item $TempZip -Force -ErrorAction SilentlyContinue
    exit 1
}

Write-Host "✓ Checksum verified" -ForegroundColor Green

if (Test-Path $ExtractDir) {
    Remove-Item $ExtractDir -Recurse -Force -ErrorAction SilentlyContinue
}
Expand-Archive -Path $TempZip -DestinationPath $ExtractDir -Force
Remove-Item $TempZip -Force -ErrorAction SilentlyContinue

if (-not (Test-Path $TempFile)) {
    Write-Error "poke.exe not found in $ArchiveName"
    Remove-Item $ExtractDir -Recurse -Force -ErrorAction SilentlyContinue
    exit 1
}
if (-not (Test-Path $TempAlias)) {
    Write-Error "pokedex.exe not found in $ArchiveName"
    Remove-Item $ExtractDir -Recurse -Force -ErrorAction SilentlyContinue
    exit 1
}

try {
    $null = & $TempFile version 2>&1
} catch {
    Write-Error "Downloaded binary appears to be invalid"
    Remove-Item $ExtractDir -Recurse -Force -ErrorAction SilentlyContinue
    exit 1
}

Write-Host "Installing to $InstallDir..." -ForegroundColor Blue
Move-Item -Path $TempFile -Destination $InstalledBin -Force
Move-Item -Path $TempAlias -Destination $InstalledAlias -Force
Remove-Item $ExtractDir -Recurse -Force -ErrorAction SilentlyContinue

try {
    $InstalledVersion = & $InstalledBin version 2>$null | Select-Object -First 1
} catch {
    $InstalledVersion = "poke"
}

Write-Host ""
Write-Host "✓ poke installed successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "  Location: $InstalledBin"
Write-Host "  Alias:    $InstalledAlias"
Write-Host "  Version:  $InstalledVersion"
Write-Host ""

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$PathEntries = if ($UserPath) { $UserPath -split ';' } else { @() }

if ($PathEntries -contains $InstallDir) {
    Write-Host "✓ $InstallDir is already in your PATH" -ForegroundColor Green
} else {
    Write-Host "⚠ $InstallDir is not in your PATH" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Adding to PATH..." -ForegroundColor Blue

    $NewPath = if ($UserPath) { "$UserPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    $env:Path = "$env:Path;$InstallDir"

    Write-Host "✓ Added to PATH" -ForegroundColor Green
    Write-Host ""
    Write-Host "Note: You may need to restart your terminal for 'poke' to be available." -ForegroundColor Yellow
    Write-Host "Or run poke directly:" -ForegroundColor Yellow
    Write-Host "  $InstalledBin --help" -ForegroundColor Blue
}

Write-Host ""
Write-Host "You can now use 'poke' (or 'pokedex'):" -ForegroundColor Green
Write-Host "  poke --help" -ForegroundColor Blue
Write-Host "  poke refresh" -ForegroundColor Blue
Write-Host "  poke update" -ForegroundColor Blue
Write-Host ""
