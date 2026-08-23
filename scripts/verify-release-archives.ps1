# Verify GoReleaser dist archives contain both poke and pokedex executables.
# Used on Windows hosts without bash; CI uses scripts/verify-release-archives.sh.
param(
    [string]$DistDir = ""
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
if (-not $DistDir) {
    $DistDir = Join-Path $Root "dist"
}

if (-not (Test-Path $DistDir)) {
    Write-Error "dist directory not found at $DistDir; run: goreleaser release --snapshot --clean --skip=publish"
}

$archives = @()
$archives += Get-ChildItem -Path $DistDir -Filter "*.tar.gz" -File -ErrorAction SilentlyContinue
$archives += Get-ChildItem -Path $DistDir -Filter "*.zip" -File -ErrorAction SilentlyContinue

if ($archives.Count -eq 0) {
    Write-Error "no .tar.gz or .zip archives under $DistDir"
}

Add-Type -AssemblyName System.IO.Compression.FileSystem
$tmpdir = Join-Path ([System.IO.Path]::GetTempPath()) ("poke-verify-" + [guid]::NewGuid().ToString("n"))
New-Item -ItemType Directory -Path $tmpdir | Out-Null

$fail = $false
$checked = 0

try {
    foreach ($archive in $archives) {
        $extract = Join-Path $tmpdir $checked.ToString()
        New-Item -ItemType Directory -Path $extract | Out-Null
        $checked++

        Write-Host "==> $($archive.Name)"

        if ($archive.Name -like "*.tar.gz") {
            tar -xzf $archive.FullName -C $extract
            $wantPoke = "poke"
            $wantAlias = "pokedex"
        } elseif ($archive.Name -like "*.zip") {
            [System.IO.Compression.ZipFile]::ExtractToDirectory($archive.FullName, $extract)
            $wantPoke = "poke.exe"
            $wantAlias = "pokedex.exe"
        } else {
            continue
        }

        $foundPoke = Get-ChildItem -Path $extract -Recurse -File -Filter $wantPoke -ErrorAction SilentlyContinue
        $foundAlias = Get-ChildItem -Path $extract -Recurse -File -Filter $wantAlias -ErrorAction SilentlyContinue
        if (-not $foundAlias) {
            $foundAlias = Get-ChildItem -Path $extract -Recurse -File -Filter "pokedex" -ErrorAction SilentlyContinue
        }

        if ($foundPoke) {
            Write-Host "  found $($foundPoke[0].Name)"
        } else {
            Write-Host "  missing $wantPoke" -ForegroundColor Red
            $fail = $true
        }
        if ($foundAlias) {
            Write-Host "  found $($foundAlias[0].Name)"
        } else {
            Write-Host "  missing $wantAlias" -ForegroundColor Red
            $fail = $true
        }
    }
} finally {
    Remove-Item -Recurse -Force $tmpdir -ErrorAction SilentlyContinue
}

if ($checked -eq 0) {
    Write-Error "no archives were checked under $DistDir"
}

if ($fail) {
    Write-Error "one or more archives are missing poke/pokedex"
}

Write-Host "ok: verified $checked archive(s) contain poke and pokedex"
