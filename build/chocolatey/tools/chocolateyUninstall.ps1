# Remove poke.exe / pokedex.exe from tools (shims are removed by Chocolatey automatically).
$ErrorActionPreference = 'Stop'
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
foreach ($name in @('poke.exe', 'pokedex.exe')) {
  $exe = Join-Path $toolsDir $name
  if (Test-Path $exe) {
    Remove-Item $exe -Force
  }
}
