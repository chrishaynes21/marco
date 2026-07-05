<#
  pack.ps1 — build the self-extracting single-file Marco.exe for Windows.

  Stages the engine + overlay binaries and the starter routes into
  plugins\marco-app\assets, then builds that module (which go:embeds them) into
  dist\Marco.exe. Ship that one file; on launch it unpacks to %LOCALAPPDATA%\Marco
  and runs the overlay stack.

    powershell -ExecutionPolicy Bypass -File .\pack.ps1

  To change the app icon: run branding.ps1 (optionally -Png <path>), then re-run this.
#>
$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
$app = Join-Path $root "plugins\marco-app"
$assets = Join-Path $app "assets"
$dist = Join-Path $root "dist"
New-Item -ItemType Directory -Force -Path $assets, $dist | Out-Null

Write-Host "==> building engine + overlay" -ForegroundColor Cyan
& go build -o (Join-Path $assets "marco.exe") ./cmd/marco
if ($LASTEXITCODE) { throw "marco build failed" }
& go build -o (Join-Path $assets "marco-macros.exe") ./cmd/marco-macros
if ($LASTEXITCODE) { throw "marco-macros build failed" }
& go -C plugins/overlay build -o (Join-Path $assets "overlay.exe") .
if ($LASTEXITCODE) { throw "overlay build failed" }
Copy-Item (Join-Path $root "plugins\overlay\overlay.marco") (Join-Path $assets "overlay.marco") -Force

# No starter routes are shipped — the launcher seeds a single "hello" route on first run
# (see plugins\marco-app\main.go). Users teach the rest.

$sha = (& git -C $root rev-parse --short HEAD 2>$null)
if (-not $sha) { $sha = "dev" }
$ver = "$sha-$(Get-Date -Format yyyyMMddHHmmss)"
Set-Content -Path (Join-Path $assets "version.txt") -Value $ver -NoNewline -Encoding ascii

Write-Host "==> building dist\Marco.exe (embedding the stack)" -ForegroundColor Cyan
& go -C $app build -o (Join-Path $dist "Marco.exe") .
if ($LASTEXITCODE) { throw "Marco.exe build failed" }

$mb = [math]::Round((Get-Item (Join-Path $dist "Marco.exe")).Length / 1MB, 1)
Write-Host "OK  dist\Marco.exe  ($mb MB)  version $ver" -ForegroundColor Green
Write-Host "    Ship that one file. It unpacks to %LOCALAPPDATA%\Marco and launches the overlay." -ForegroundColor DarkGray
