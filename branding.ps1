<#
  branding.ps1 — swap Marco's icon in one step.

  Drop a square PNG (ideally >= 256x256) at plugins\marco-app\icon.png, or pass its
  path with -Png. This regenerates everything derived from it:
    - plugins\marco-app\icon.ico              (multi-size Windows icon)
    - cmd\marco\favicon.png                   (128px UI favicon + header logo)
    - plugins\marco-app\rsrc_windows_amd64.syso   (Marco.exe icon resource)
    - plugins\overlay\rsrc_windows_amd64.syso     (overlay.exe icon resource)

  Then rebuild the bundle with pack.ps1.

    powershell -ExecutionPolicy Bypass -File .\branding.ps1 -Png C:\path\to\logo.png
#>
param([string]$Png)
$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
$app = Join-Path $root "plugins\marco-app"
$src = Join-Path $app "icon.png"
$ico = Join-Path $app "icon.ico"
$fav = Join-Path $root "cmd\marco\favicon.png"

if ($Png) {
    if (-not (Test-Path $Png)) { throw "no such file: $Png" }
    Copy-Item $Png $src -Force
    Write-Host "using $Png as the icon" -ForegroundColor Cyan
}
if (-not (Test-Path $src)) { throw "no icon at $src -- pass -Png <path>" }

Write-Host "==> icon.ico + favicon.png (stdlib resize, offline)" -ForegroundColor Cyan
& go -C (Join-Path $root "tools\branding") run . $src $ico $fav
if ($LASTEXITCODE) { throw "icon generation failed" }

Write-Host "==> Windows resources (.syso, needs network for rsrc)" -ForegroundColor Cyan
foreach ($d in @($app, (Join-Path $root "plugins\overlay"))) {
    & go run github.com/akavel/rsrc@latest -ico $ico -arch amd64 -o (Join-Path $d "rsrc_windows_amd64.syso")
    if ($LASTEXITCODE) { throw "rsrc failed for $d" }
}
Write-Host "OK  icon swapped. Rebuild the bundle with:  .\pack.ps1" -ForegroundColor Green
