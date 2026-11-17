# Download poke Windows zip from GitHub release and extract.
# Checksum is injected at pack time by the release workflow so the zip download is validated (Chocolatey requirement).
$ErrorActionPreference = 'Stop'
$version = $env:ChocolateyPackageVersion
$tag = "v$version"
$zipName = "poke_${version}_windows_amd64.zip"
$zipUrl = "https://github.com/Graphixa/Pokedex/releases/download/$tag/$zipName"
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$unzipLocation = Join-Path $env:TEMP "poke-choco-$version"
if (Test-Path $unzipLocation) { Remove-Item $unzipLocation -Recurse -Force }
New-Item -ItemType Directory -Path $unzipLocation -Force | Out-Null

$packageArgs = @{
  PackageName    = 'poke'
  Url64bit       = $zipUrl
  UnzipLocation  = $unzipLocation
  Checksum64     = 'CHECKSUM_PLACEHOLDER'
  ChecksumType64 = 'sha256'
}
Install-ChocolateyZipPackage @packageArgs

# Zip contains poke.exe and pokedex.exe; copy both to tools for shims
$pokeExe = Get-ChildItem -Path $unzipLocation -Filter "poke.exe" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
$aliasExe = Get-ChildItem -Path $unzipLocation -Filter "pokedex.exe" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
if ($pokeExe) {
  Copy-Item $pokeExe.FullName -Destination (Join-Path $toolsDir "poke.exe") -Force
}
if ($aliasExe) {
  Copy-Item $aliasExe.FullName -Destination (Join-Path $toolsDir "pokedex.exe") -Force
}
Remove-Item $unzipLocation -Recurse -Force -ErrorAction SilentlyContinue
