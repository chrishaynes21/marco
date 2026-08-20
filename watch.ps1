# Watch what Marco currently sees and believes, live.
#
# Read-only. This polls state the Director service already holds — it starts no observation,
# takes no sample, runs no vision pass and forms no interpretation, so leaving it open beside a
# live session cannot change what that session sees.
#
#   .\watch.ps1              what Marco sees and believes
#   .\watch.ps1 -Deep        the evidence underneath it
#   .\watch.ps1 -Every 2     seconds between refreshes (default 1)
#
# Ctrl-C to stop.

param(
    [switch]$Deep,
    [int]$Every = 1
)

$marco = Join-Path $PSScriptRoot "marco.exe"
if (-not (Test-Path $marco)) {
    Write-Host "marco.exe is not built yet. Run: go build -o marco.exe ./cmd/marco" -ForegroundColor Yellow
    exit 1
}

$reading = if ($Deep) { "diagnose" } else { "watch" }

# Alternate-screen so the panel does not scroll the terminal's history away.
try {
    while ($true) {
        $out = & $marco director $reading 2>&1 | Out-String
        Clear-Host
        $stamp = Get-Date -Format "HH:mm:ss"
        Write-Host "marco director $reading   $stamp   (Ctrl-C to stop)" -ForegroundColor DarkGray
        Write-Host ""
        Write-Host $out
        Start-Sleep -Seconds $Every
    }
} finally {
    Write-Host "stopped watching." -ForegroundColor DarkGray
}
