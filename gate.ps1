# gate.ps1 -- the alpha's full regression gate, in the order it has to run in.
#
#   1. the suites, with what declined and what outlived them (cmd\gate)
#   2. the bundle, packed from this tree (pack.ps1)
#   3. the bundle INSTALLED from an isolated directory, reporting what it is made of
#   4. the complete desktop harness against those installed binaries (plugins\e2e)
#
# -- WHY THE ORDER IS THE POINT ------------------------------------------------
#
# Each stage is only worth anything if the one before it passed. A harness run against binaries
# whose own suite is failing tells you about the harness; a bundle validated from the tree it was
# built in cannot tell you whether an installation works; and the desktop half must be the LAST
# thing, because it is the only part that takes somebody's screen.
#
# -- WHAT RUNNING THIS DOES TO YOUR MACHINE ------------------------------------
#
# Stage 1 opens nothing. Stage 3 installs into a temporary directory with LOCALAPPDATA and APPDATA
# pointed away from your own, so your library and your installed Marco are not touched. Stage 4
# opens two windows of its own and lets Marco perform REAL INPUT into its own fixture window on
# your actual screen -- so do not type during it.
#
# It ends processes only when they are its own: a child of this run, or an orphan whose parent is
# gone. Something you started yourself is reported and left alone.
#
#   .\gate.ps1                 all four stages
#   .\gate.ps1 -SuitesOnly     stage 1, which needs no screen
#   .\gate.ps1 -Installed <bin>  stages 1 and 4 against an installation you already have
#   .\gate.ps1 -Only play      stage 4 limited to one family (NOT the gate: a partial run)
#
# See [[ADR-264-the-gate-answers-for-what-did-not-run]].

param(
    [switch]$SuitesOnly,
    [string]$Installed = "",
    [string]$Only = "",
    [switch]$Keep
)

$ErrorActionPreference = "Continue"
$root = $PSScriptRoot
$started = Get-Date
$stages = @()

function Stage($name, $code, $says) {
    $script:stages += [pscustomobject]@{ Stage = $name; Code = $code; Says = $says }
}

Write-Host ""
Write-Host "marco gate" -ForegroundColor Cyan
Write-Host "  this tree: $root"
if (-not $SuitesOnly) {
    Write-Host "  stage 4 takes your screen and performs real input into its own fixture window." -ForegroundColor Yellow
}
Write-Host ""

# -- 1. THE SUITES -------------------------------------------------------------
Write-Host "== 1/4  the suites, and what declined or outlived them" -ForegroundColor Cyan
& go run ./cmd/gate
$suites = $LASTEXITCODE
Stage "suites" $suites "cmd\gate"
if ($suites -ne 0) {
    Write-Host ""
    Write-Host "gate: the suites did not pass, so nothing after them was run." -ForegroundColor Red
    exit 1
}

if ($SuitesOnly) {
    Write-Host ""
    Write-Host "gate: the suites passed. The desktop half was not run (-SuitesOnly)." -ForegroundColor Yellow
    exit 0
}

# -- 2. THE BUNDLE -------------------------------------------------------------
$bin = $Installed
if ($bin -eq "") {
    Write-Host ""
    Write-Host "== 2/4  packing the bundle" -ForegroundColor Cyan
    & (Join-Path $root "pack.ps1")
    if ($LASTEXITCODE -ne 0) {
        Stage "pack" $LASTEXITCODE "pack.ps1"
        Write-Host "gate: the bundle could not be packed." -ForegroundColor Red
        exit 2
    }
    Stage "pack" 0 "dist\Marco.exe"

    # -- 3. INSTALLED, FROM SOMEWHERE THAT IS NOT THIS TREE --------------------
    #
    # With LOCALAPPDATA and APPDATA pointed away from the owner's, no repository beside it and
    # PATH cut to the system directories: the three things a developer's machine supplies by
    # accident and an installation cannot rely on. The first packed alpha reported a missing
    # file and an unreadable manifest here, both correct bundles.
    Write-Host ""
    Write-Host "== 3/4  installing it from an isolated directory" -ForegroundColor Cyan
    $iso = Join-Path $env:TEMP ("marco-gate-" + (Get-Date -Format "yyyyMMdd-HHmmss"))
    New-Item -ItemType Directory -Force -Path $iso | Out-Null
    Copy-Item (Join-Path $root "dist\Marco.exe") (Join-Path $iso "Marco.exe")
    $local = Join-Path $iso "Local"
    $roaming = Join-Path $iso "Roaming"
    $store = Join-Path $iso "store"
    New-Item -ItemType Directory -Force -Path $local, $roaming, $store | Out-Null

    $script = @"
`$env:LOCALAPPDATA = '$local'
`$env:APPDATA = '$roaming'
`$env:PATH = 'C:\Windows\system32;C:\Windows'
`$env:MARCO_HOME = ''
`$env:MARCO_ROUTES = ''
Set-Location '$iso'
& '$iso\Marco.exe' -acts '$store'
exit `$LASTEXITCODE
"@
    $file = Join-Path $iso "install.ps1"
    Set-Content -Path $file -Value $script -Encoding utf8
    $said = & powershell -NoProfile -ExecutionPolicy Bypass -File $file 2>&1
    $code = $LASTEXITCODE
    $said | Select-String -Pattern "Marco |build |made of|PROBLEM" | ForEach-Object { Write-Host "  $_" }
    $problems = @($said | Select-String -Pattern "PROBLEM").Count
    if ($code -ne 0 -or $problems -gt 0) {
        Stage "install" 1 "$problems problem(s) reported by the installed bundle"
        Write-Host ""
        Write-Host "gate: the packaged bundle did not install cleanly, so the desktop half was not run." -ForegroundColor Red
        exit 2
    }
    Stage "install" 0 $iso
    $bin = Join-Path $local "Marco\bin"
} else {
    Write-Host ""
    Write-Host "== 2-3/4  skipped: testing the installation at $bin" -ForegroundColor Yellow
    Stage "pack" 0 "skipped (-Installed)"
    Stage "install" 0 $bin
}

# -- 4. THE COMPLETE DESKTOP HARNESS, AGAINST THOSE BINARIES -------------------
#
# EVERY REGISTERED SCENARIO. `-only` is not the gate: a selection that matched some of them is a
# partial run, and the scenario that did not run is the one whose question nobody asked.
Write-Host ""
Write-Host "== 4/4  the complete desktop harness against the installed binaries" -ForegroundColor Cyan
$harnessArgs = @("-installed", $bin)
if ($Only -ne "") {
    Write-Host "  NOT THE GATE: -Only $Only runs some of the scenarios, which is a partial run." -ForegroundColor Yellow
    $harnessArgs += @("-only", $Only)
}
if ($Keep) { $harnessArgs += "-keep" }
& (Join-Path $root "plugins\e2e\run.cmd") @harnessArgs
$harness = $LASTEXITCODE
Stage "harness" $harness "plugins\e2e"

# -- THE VERDICT ---------------------------------------------------------------
Write-Host ""
Write-Host ("-" * 78)
$stages | ForEach-Object {
    $mark = if ($_.Code -eq 0) { "passed " } else { "FAILED " }
    Write-Host ("  {0} {1,-9} {2}" -f $mark, $_.Stage, $_.Says)
}
$worst = ($stages | Where-Object { $_.Code -ne 0 } | Select-Object -First 1)
Write-Host ""
Write-Host ("took {0}" -f ((Get-Date) - $started).ToString("hh\:mm\:ss"))
if ($null -eq $worst) {
    Write-Host "gate: passed -- the suites, the packaged bundle, its installation, and every registered scenario." -ForegroundColor Green
    Write-Host "physical recording capture: NOT EXERCISED. A person demonstrating in Studio is still the only evidence of that."
    exit 0
}
Write-Host ("gate: FAILED at {0} ({1}). Nothing after it was run." -f $worst.Stage, $worst.Code) -ForegroundColor Red
exit 1
