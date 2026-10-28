<#
  pack.ps1 — build the self-extracting single-file Marco.exe for Windows.

  Stages Studio, the engine, the Director, the accessibility provider and the legacy
  overlay into plugins\marco-app\assets, writes a manifest of what each piece is, then
  builds that module (which go:embeds them) into dist\Marco.exe. Ship that one file; on
  launch it unpacks to %LOCALAPPDATA%\Marco and opens STUDIO. The overlay is still in
  there and starts only with `Marco.exe --overlay`.

    powershell -ExecutionPolicy Bypass -File .\pack.ps1

  To change the app icon: run branding.ps1 (optionally -Png <path>), then re-run this.
#>
$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
$app = Join-Path $root "plugins\marco-app"
$assets = Join-Path $app "assets"
$dist = Join-Path $root "dist"
New-Item -ItemType Directory -Force -Path $assets, $dist | Out-Null

Write-Host "==> building engine + director + overlay" -ForegroundColor Cyan
& go build -o (Join-Path $assets "marco.exe") ./cmd/marco
if ($LASTEXITCODE) { throw "marco build failed" }
& go build -o (Join-Path $assets "marco-macros.exe") ./cmd/marco-macros
if ($LASTEXITCODE) { throw "marco-macros build failed" }
# The Director service. It lands beside marco.exe in bin\, which is exactly where
# cmd/marco's directorBin() looks — so `marco director ...` auto-starts it with no env var.
# An installed Marco without it cannot perform a learned play at all.
& go build -o (Join-Path $assets "director.exe") ./cmd/director
if ($LASTEXITCODE) { throw "director build failed" }
& go -C plugins/overlay build -o (Join-Path $assets "overlay.exe") .
if ($LASTEXITCODE) { throw "overlay build failed" }
Copy-Item (Join-Path $root "plugins\overlay\overlay.marco") (Join-Path $assets "overlay.marco") -Force

# The accessibility provider — the Theater's only actor. Built with the .NET Framework
# csc.exe that ships with Windows (see plugins\uia\build.ps1): no SDK, no restore.
# It is extracted to bin\plugins\uia\uia.exe, which is the second tier of the discovery
# cmd/marco already does, so nothing has to be told where it is.
Write-Host "==> building accessibility provider (plugins\uia)" -ForegroundColor Cyan
& (Join-Path $root "plugins\uia\build.ps1") | Out-Host
$uia = Join-Path $root "plugins\uia\uia.exe"
if (-not (Test-Path $uia)) { throw "uia build failed: no plugins\uia\uia.exe" }
Copy-Item $uia (Join-Path $assets "uia.exe") -Force

# ==> STUDIO, WHICH IS THE ALPHA'S FRONT DOOR.
#
# WITH THE WAILS TAGS, and that is not a detail: a plain `go build` here produces a binary
# that OPENS NO WINDOW. Wails guards on its own tags — without `production` the CreateApp
# that compiles shows a message box reading "Wails applications will not build without the
# correct build tags" and exits. The binary is otherwise perfectly ordinary: it has the
# flags, it prints usage, and `-acts` and `-proof` work, because they return before the app
# is created. One of those sat in the tree looking current until somebody double-clicked it.
#
# `-H windowsgui` detaches from the console, which is what a window wants.
#
# BUILT FROM INSIDE ITS OWN MODULE, so `go build` stamps the VCS revision into it. A package
# built from outside its module is not stamped at all, `internal/buildid` then answers
# "unknown", and a window that cannot name itself cannot be compared with the marco and
# director beside it — which is the whole point of carrying a revision.
Write-Host "==> building studio.exe with the Wails tags" -ForegroundColor Cyan
& go -C (Join-Path $root "plugins\studio") build -tags desktop,production `
    -ldflags "-w -s -H windowsgui" -o (Join-Path $assets "studio.exe") .
if ($LASTEXITCODE) { throw "studio build failed" }

# AND IT MUST NOT BE THE ONE THAT ONLY SHOWS AN ERROR DIALOG.
#
# Checked by looking for the guard's own sentence in the bytes: a correctly built Studio does
# not contain that code at all. Checked here rather than trusted, because the two builds are
# indistinguishable from a terminal and the difference is the entire product.
$guard = "will not build without the correct build tags"
$bytes = [System.IO.File]::ReadAllBytes((Join-Path $assets "studio.exe"))
$text = [System.Text.Encoding]::ASCII.GetString($bytes)
if ($text.Contains($guard)) {
    throw "studio.exe was built WITHOUT the Wails tags: it contains the guard's message box and would open no window"
}

# No starter routes are shipped — the launcher seeds a single "hello" route on first run
# (see plugins\marco-app\main.go). The user shows Marco the rest.

$sha = (& git -C $root rev-parse --short HEAD 2>$null)
if (-not $sha) { $sha = "dev" }
$ver = "$sha-$(Get-Date -Format yyyyMMddHHmmss)"
Set-Content -Path (Join-Path $assets "version.txt") -Value $ver -NoNewline -Encoding ascii

# The embed is a directory (see plugins\marco-app\main.go), so a forgotten asset is no longer a
# compile error. Check the list HERE, where the person who can fix it is standing — an installer
# missing director.exe or uia.exe still launches and simply cannot perform anything.
$required = @("studio.exe", "marco.exe", "marco-macros.exe", "director.exe", "uia.exe", "overlay.exe", "overlay.marco", "version.txt")
foreach ($f in $required) {
    if (-not (Test-Path (Join-Path $assets $f))) { throw "not staged: plugins\marco-app\assets\$f" }
}

# ==> AND WHAT EACH PIECE IS, WRITTEN DOWN.
#
# # Why a manifest and not just the launcher's version
#
# Because `version.txt` is a label this script chose, and a label is not an identity. It says
# what the REPOSITORY was at pack time, which is a fact about this machine at this minute — and
# it would say the same thing about a bundle whose studio.exe had been replaced by hand.
#
# So each Go binary is asked what IT says, through `go version -m`, which reads back the VCS
# stamp `go build` left in it. Two binaries that disagree are then a thing somebody can see.
# And the one piece that is not Go — the accessibility provider, built by the .NET Framework
# csc.exe that ships with Windows — has no such stamp, so it is identified by the hash of its
# bytes, which is the only identity it has. Every file gets a hash regardless: a revision says
# which SOURCE a binary came from and a hash says whether THIS is that binary.
#
# A binary that cannot say its revision is a FAILURE here, not a note. "unknown" is what a
# package built from outside its own module answers, which is the mistake `build.cmd` made with
# the director once — and a shipped bundle of them would make every run record unciteable.
Write-Host "==> writing the bundle manifest" -ForegroundColor Cyan
$goBins = @("studio.exe", "marco.exe", "marco-macros.exe", "director.exe", "overlay.exe")
$entries = @()
foreach ($f in $required) {
    $path = Join-Path $assets $f
    $hash = (Get-FileHash -Algorithm SHA256 -Path $path).Hash.ToLower()
    $rev = ""
    if ($goBins -contains $f) {
        $stamp = (& go version -m $path 2>$null | Select-String -Pattern "vcs.revision=(\S+)")
        if ($stamp) { $rev = $stamp.Matches[0].Groups[1].Value }
        if (-not $rev) {
            throw "$f carries no VCS revision: it was built from outside its own module, so nothing it writes can say which build produced it"
        }
        $dirty = (& go version -m $path 2>$null | Select-String -Pattern "vcs.modified=true")
        $short = $rev.Substring(0, [math]::Min(7, $rev.Length))
        if ($dirty) { $short = "$short+dirty" }
        $rev = $short
    }
    $entries += [pscustomobject]@{ file = $f; sha256 = $hash; revision = $rev }
}
$manifest = [pscustomobject]@{ version = $ver; packed = (Get-Date -Format "o"); files = $entries }
$manifest | ConvertTo-Json -Depth 4 | Set-Content -Path (Join-Path $assets "manifest.json") -Encoding utf8
$required += "manifest.json"
foreach ($e in $entries) {
    Write-Host ("    {0,-18} {1,-14} {2}" -f $e.file, $e.revision, $e.sha256.Substring(0, 12)) -ForegroundColor DarkGray
}

Write-Host "==> building dist\Marco.exe (embedding the stack)" -ForegroundColor Cyan
& go -C $app build -o (Join-Path $dist "Marco.exe") .
if ($LASTEXITCODE) { throw "Marco.exe build failed" }

$mb = [math]::Round((Get-Item (Join-Path $dist "Marco.exe")).Length / 1MB, 1)
Write-Host "OK  dist\Marco.exe  ($mb MB)  version $ver" -ForegroundColor Green
Write-Host "    Ship that one file. It unpacks to %LOCALAPPDATA%\Marco and opens Studio." -ForegroundColor DarkGray
Write-Host "    The legacy overlay is in there too and starts only with:  Marco.exe --overlay" -ForegroundColor DarkGray
