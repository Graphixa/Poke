# Measure subsequent-launch browse startup (plan section 15A).
# Run in Windows Terminal with sixel active. Requires built poke.exe and an
# existing active dataset/cache (not first-run bootstrap).
#
# Usage:
#   .\scripts\measure-browse-startup.ps1
#   .\scripts\measure-browse-startup.ps1 -Binary bin\poke.exe -Runs 10

param(
    [string]$Binary = "bin\poke.exe",
    [int]$Runs = 10,
    [int]$HoldSeconds = 1
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path $Binary)) {
    Write-Error "Binary not found: $Binary (run: go build -o bin\poke.exe ./cmd/poke)"
}

$env:POKEDEX_DEBUG_STARTUP = "1"
$firstViewMs = New-Object System.Collections.Generic.List[double]

function Parse-FirstViewMs {
    param([string[]]$Lines)
    $total = 0.0
    $found = $false
    foreach ($line in $Lines) {
        if ($line -notmatch '^startup:\s+') { continue }
        $body = $line -replace '^startup:\s+', ''
        if ($body -match '^(\S+)\s+(\d+(?:\.\d+)?)(ms|s|m|h)') {
            $dur = [double]$Matches[2]
            switch ($Matches[3]) {
                'ms' { }
                's'  { $dur *= 1000 }
                'm'  { $dur *= 60000 }
                'h'  { $dur *= 3600000 }
            }
            $total += $dur
            if ($Matches[1] -eq 'startup.first_view_or_first_render') {
                $found = $true
                break
            }
        }
    }
    if (-not $found) { return $null }
    return $total
}

Write-Host "Measuring $Runs browse launches (hold ${HoldSeconds}s, Ctrl+C quit)..."
Write-Host "Binary: $Binary"
Write-Host ""

for ($i = 1; $i -le $Runs; $i++) {
    $stderrFile = [System.IO.Path]::GetTempFileName()
    $stdoutFile = [System.IO.Path]::GetTempFileName()
    try {
        $p = Start-Process -FilePath $Binary -ArgumentList "browse" `
            -NoNewWindow -PassThru `
            -RedirectStandardError $stderrFile -RedirectStandardOutput $stdoutFile
        Start-Sleep -Seconds $HoldSeconds
        if (-not $p.HasExited) {
            $p.CloseMainWindow() | Out-Null
            Start-Sleep -Milliseconds 200
            if (-not $p.HasExited) { $p.Kill() }
            $p.WaitForExit(5000) | Out-Null
        }
        $lines = @(Get-Content $stderrFile -ErrorAction SilentlyContinue)
        $ms = Parse-FirstViewMs $lines
        if ($null -eq $ms) {
            Write-Warning "Run $i`: no startup.first_view_or_first_render in stderr; raw lines:"
            $lines | ForEach-Object { Write-Host "  $_" }
        } else {
            $firstViewMs.Add($ms)
            Write-Host ("Run {0,2}: process -> first view = {1,7:N0} ms" -f $i, $ms)
        }
    } finally {
        Remove-Item $stderrFile, $stdoutFile -ErrorAction SilentlyContinue
    }
}

if ($firstViewMs.Count -eq 0) {
    Write-Error "No timing samples collected. Run from an interactive Windows Terminal session."
}

$sorted = $firstViewMs | Sort-Object
$median = $sorted[[int][math]::Floor(($sorted.Count - 1) / 2)]
$p95Idx = [int][math]::Ceiling(0.95 * $sorted.Count) - 1
if ($p95Idx -ge $sorted.Count) { $p95Idx = $sorted.Count - 1 }
$p95 = $sorted[$p95Idx]

Write-Host ""
Write-Host ("Samples: {0}" -f $sorted.Count)
Write-Host ("Median:  {0:N0} ms (target <150 ms)" -f $median)
Write-Host ("P95:     {0:N0} ms (target <250 ms)" -f $p95)
Write-Host ""
Write-Host "Full per-mark breakdown prints on quit when POKEDEX_DEBUG_STARTUP=1."
