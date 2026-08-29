<#
.SYNOPSIS
  Roadmap 37G — does one real Settings page stay ONE durable Place across reflow,
  restart, and optional sensor richness, while a different page stays different?

.DESCRIPTION
  A live acceptance against the REAL production perception path and the REAL semantic
  store, in an ISOLATED Marco home. Three pieces of acceptance debt converge here:

    35D  does a wide and a narrow rendering of one page resolve to one Place?
    37D  the capture-to-totals adapter cannot build a StructureSignature, so this
         cannot be answered offline.
    37F  a Place learned with visual evidence admitted has a richer signature than
         the same page read by accessibility alone. Does that change its identity?

  WHAT IT DOES NOT DO
  It emits no desktop input. Navigation is `ms-settings:` shell activation and resizing
  is SetWindowPos — window management, not keystrokes or clicks. Marco acquires no
  authority and no actuation lease at any point; every reading is a passive observation
  session, and the only durable writes are the two Places this deliberately establishes.

  THE REAL STORE IS NEVER TOUCHED. It is hashed before and after, and the check is a
  test outcome rather than a comment.

.PARAMETER Home
  The isolated acceptance home. Defaults to a fresh directory under TEMP.

.PARAMETER Vision
  Configure the visual detector for this run, so the sensor-richness half can be
  measured. Needs plugins/vision built.

.PARAMETER KeepHome
  Leave the acceptance home in place afterwards for inspection.
#>
[CmdletBinding()]
param(
    [string]$AcceptanceHome = (Join-Path $env:TEMP ("marco-37g-" + (Get-Date -Format 'HHmmss'))),
    [string]$Repo    = '',
    [switch]$Vision,
    [switch]$KeepHome
)

$ErrorActionPreference = 'Stop'

# $PSScriptRoot is not bound while param() defaults are evaluated, so the repository is
# resolved here instead. An empty $Repo reached Join-Path and failed the run before it
# had recorded anything.
if (-not $Repo) { $Repo = Split-Path -Parent $MyInvocation.MyCommand.Path }

# ── the real store, recorded before anything runs ─────────────────────────────
#
# Named and hashed FIRST, before this process sets MARCO_HOME to anything, because a
# guard computed afterwards is guarding whatever the script already pointed at. 36C lost
# a session to exactly that: an acceptance run left MARCO_HOME redirected and the next
# command wrote somewhere nobody expected.

function Get-RealMarcoHome {
    # The same location the product computes when MARCO_HOME says nothing:
    # os.UserConfigDir()/marco. Not read from the environment — the environment is the
    # thing being overridden.
    Join-Path $env:APPDATA 'marco'
}

function Get-TreeHash([string]$path) {
    if (-not (Test-Path $path)) { return @{} }
    $out = @{}
    Get-ChildItem -Path $path -File -Recurse | ForEach-Object {
        $out[$_.FullName] = (Get-FileHash $_.FullName -Algorithm MD5).Hash
    }
    $out
}

function Compare-TreeHash($before, $after) {
    $changed = @()
    foreach ($k in $before.Keys) {
        if (-not $after.ContainsKey($k))      { $changed += "REMOVED  $k" }
        elseif ($after[$k] -ne $before[$k])   { $changed += "MODIFIED $k" }
    }
    foreach ($k in $after.Keys) {
        if (-not $before.ContainsKey($k))     { $changed += "ADDED    $k" }
    }
    $changed
}

$script:RealHome       = Get-RealMarcoHome
$script:RealHomeBefore = Get-TreeHash $script:RealHome
$script:PriorMarcoHome = $env:MARCO_HOME

if ($script:PriorMarcoHome -and
    ((Resolve-Path -LiteralPath $script:PriorMarcoHome -ErrorAction SilentlyContinue).Path -eq
     (Resolve-Path -LiteralPath $script:RealHome -ErrorAction SilentlyContinue).Path)) {
    throw "MARCO_HOME already points at the real store; refusing to run acceptance over it."
}
if ((Resolve-Path -LiteralPath $AcceptanceHome -ErrorAction SilentlyContinue).Path -eq
    (Resolve-Path -LiteralPath $script:RealHome -ErrorAction SilentlyContinue).Path) {
    throw "-Home is the real store. The whole point of this run is that it is not."
}

# ── window management (no input) ──────────────────────────────────────────────

Add-Type @"
using System;
using System.Runtime.InteropServices;
public class Marco37G {
  [DllImport("user32.dll", SetLastError=true)]
  public static extern bool SetWindowPos(IntPtr h, IntPtr after, int x, int y, int cx, int cy, uint flags);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int n);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
}
"@

$SettingsApplication = 'applicationframehost'

function Get-SettingsWindow {
    $w = Get-Process ApplicationFrameHost -ErrorAction SilentlyContinue |
         Where-Object { $_.MainWindowTitle -eq 'Settings' } | Select-Object -First 1
    if (-not $w) { throw "Settings is not open." }
    $w.MainWindowHandle
}

function Get-SettingsSize {
    $r = New-Object Marco37G+RECT
    [void][Marco37G]::GetWindowRect((Get-SettingsWindow), [ref]$r)
    [pscustomobject]@{ Width = $r.Right - $r.Left; Height = $r.Bottom - $r.Top }
}

function Set-SettingsSize([int]$width, [int]$height) {
    $h = Get-SettingsWindow
    [void][Marco37G]::ShowWindow($h, 9)          # SW_RESTORE — never maximised, or the size is ignored
    [void][Marco37G]::SetWindowPos($h, [IntPtr]::Zero, 60, 60, $width, $height, 0x0014)
    Start-Sleep -Seconds 3                        # let the page reflow on its own
    Get-SettingsSize
}

function Open-SettingsPage([string]$uri) {
    Start-Process $uri
    Start-Sleep -Seconds 4
}

# ── the Director, in the acceptance home ──────────────────────────────────────

$Director = Join-Path $Repo 'director.exe'
if (-not (Test-Path $Director)) { throw "$Director is missing — go build -o director.exe ./cmd/director" }

function Start-AcceptanceDirector {
    param([switch]$Quiet)
    Stop-AcceptanceDirector
    New-Item -ItemType Directory -Force -Path $AcceptanceHome | Out-Null
    $env:MARCO_HOME = $AcceptanceHome
    if ($Vision) {
        $env:MARCO_VISION_MODEL = Join-Path $Repo 'plugins\vision\models\icon_detect.onnx'
        # THE 1.28 RUNTIME, not the copy beside the plugin. plugins\vision\onnxruntime.dll is
        # 1.26 and the plugin is built against onnxruntime_go v1.32, which asks for API 28:
        # "The requested API version [28] is not available, only API versions [1, 26] are
        # supported in this build". The detector then reports itself unavailable, which reads
        # in a result table exactly like a detector that ran and found nothing.
        $env:MARCO_ONNXRUNTIME = Join-Path $Repo 'tools\onnxruntime\onnxruntime-win-x64-1.28.0\lib\onnxruntime.dll'
        if (-not (Test-Path $env:MARCO_ONNXRUNTIME)) {
            throw "-Vision needs $($env:MARCO_ONNXRUNTIME); without it the detector is silently absent and the run would report an unmeasured thing as measured."
        }
    } else {
        Remove-Item Env:\MARCO_VISION_MODEL -ErrorAction SilentlyContinue
        Remove-Item Env:\MARCO_ONNXRUNTIME  -ErrorAction SilentlyContinue
    }
    Start-Process -FilePath $Director -ArgumentList 'serve' -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $AcceptanceHome 'serve.out') `
        -RedirectStandardError  (Join-Path $AcceptanceHome 'serve.err') | Out-Null
    Start-Sleep -Seconds 3
    if (-not $Quiet) { Write-Host "  director serving $AcceptanceHome" -ForegroundColor DarkGray }
}

function Stop-AcceptanceDirector {
    Get-Process director -ErrorAction SilentlyContinue | Stop-Process -Force
    Start-Sleep -Seconds 1
}

function Invoke-Director { & $Director @args 2>&1 }

# ── one reading ───────────────────────────────────────────────────────────────
#
# Two facts come from two places on purpose, and the report says which is which.
#
#   IDENTITY comes from the live Director: a passive session, then `director showing`,
#   which is ObserveShowing — the one "where am I standing" door, resolved by
#   observe.PlaceNow against the real store. Load-bearing.
#
#   ELEMENTS / SUFFICIENCY / PROVENANCE come from the session's own account. They are
#   DIAGNOSTICS. Element count is explicitly not identity (37C, 37D) and is printed so a
#   reader can see the presentations really did differ.

function Get-Reading([string]$label) {
    # THE DIAGNOSTIC HALF, taken first and taken alone. `walk-audit` runs one reading
    # through the production collector and fusion engine and reports what each source
    # did and whether the reading sufficed. It resolves no Place and touches no store —
    # 37D established that its capture-to-totals adapter is enough for the sufficiency
    # judgement and NOT enough for identity, so nothing here may be read as identity.
    Push-Location $Repo
    $audit = (Invoke-Director walk-audit --application $SettingsApplication --repeat 1) | Out-String
    Pop-Location
    $elements = 0; $sufficiency = 'unknown'
    foreach ($line in ($audit -split "`n")) {
        if ($line -match '^\s*1\s+(\d+)\s+\d+x\s+\S+\s+\S+\s+(\S+)') {
            $elements = [int]$Matches[1]; $sufficiency = $Matches[2]
        }
    }
    # THE IDENTITY HALF, and the load-bearing one. A passive observation session — the
    # production `StartObservation`, whose Episode is the zero value, so it recognises
    # and cannot establish — then `director showing`, which is ObserveShowing: the one
    # "where am I standing" door, resolved by observe.PlaceNow against the real store.
    $started   = (Invoke-Director observe-game --application $SettingsApplication `
                    --duration 30s --interval 900ms --json) | Out-String
    $sessionID = ($started | ConvertFrom-Json).id
    Start-Sleep -Seconds 9
    $place = ((Invoke-Director showing --application $SettingsApplication --json) |
                Out-String | ConvertFrom-Json)

    # WHICH SENSORS THE DIRECTOR ITSELF USED, from the session's own record.
    #
    # NOT from walk-audit. That is a separate reading taken by a separate process, and it
    # never asks for the visual pass — so reporting its providers here printed `vision:False`
    # for every row of a run where the Director had the detector configured and was using it.
    # A harness that mislabels which sensors ran is a harness that can claim a sensor-richness
    # result it did not measure.
    $account = ((Invoke-Director observation-session $sessionID --json) | Out-String)
    $ran = @()
    try {
        $ran = @(($account | ConvertFrom-Json).stats.proven_providers.PSObject.Properties |
                    ForEach-Object { $_.Name })
    } catch { $ran = @() }
    $visionRan = ($ran -contains 'vision')

    $null = Invoke-Director cancel-observation $sessionID
    Start-Sleep -Seconds 2

    $size = Get-SettingsSize
    [pscustomobject]@{
        Label       = $label
        Outcome     = $place.outcome
        Subject     = $place.subject
        Why         = $place.why
        Sensors     = $(if ($ran.Count) { $ran -join '+' } else { 'none contributed' })
        VisionRan   = $visionRan
        Elements    = $elements
        Sufficiency = $sufficiency
        Width       = $size.Width
        Height      = $size.Height
        Session     = $sessionID
    }
}

# Establish-Place is the one operation here that WRITES. It is `director learn`, the
# normal permitted acquisition path: Episode.EstablishPlaces is set by Learn and by
# nothing else, so the place the session is standing on becomes durable. The session is
# then cancelled — nothing is demonstrated and no play is kept. See ADR-047.
function Establish-Place([string]$name) {
    $null = Invoke-Director learn $name --application $SettingsApplication `
                --follow=false --no-highlight
    Start-Sleep -Seconds 14
    $null = Invoke-Director learn --cancel
    Start-Sleep -Seconds 2
}

function Get-StoreSubjects {
    $f = Join-Path $AcceptanceHome 'semantic-memory.json'
    if (-not (Test-Path $f)) { return @() }
    @((Get-Content $f -Raw | ConvertFrom-Json).subjects)
}

function Show-Reading($r, [string]$expect) {
    $same = if ($expect) { if ($r.Subject -eq $expect) { 'yes' } else { 'NO' } } else { '-' }
    $colour = if ($same -eq 'NO') { 'Red' } elseif ($same -eq 'yes') { 'Green' } else { 'Gray' }
    Write-Host ("  {0,-28} {1,-13} {2,-22} {3,5}px {4,4} el  {5,-11} vision:{6,-5} same:{7}" -f `
        $r.Label, $r.Outcome, $(if ($r.Subject) { $r.Subject } else { "-" }),
        $r.Width, $r.Elements, $r.Sufficiency, $r.VisionRan, $same) -ForegroundColor $colour
    if ($r.Why) { Write-Host ("      why  " + $r.Why) -ForegroundColor DarkGray }
    $script:Results += $r
}

# ── the run ───────────────────────────────────────────────────────────────────

$script:Results = @()
try {
    Write-Host ""
    Write-Host "ROADMAP 37G — live Place identity convergence" -ForegroundColor Cyan
    Write-Host "  real home        $script:RealHome (recorded, never written)"
    Write-Host "  acceptance home  $AcceptanceHome"
    Write-Host "  detector         $(if ($Vision) { 'configured' } else { 'not configured' })"
    Write-Host ""

    Start-AcceptanceDirector
    Open-SettingsPage 'ms-settings:mousetouchpad'
    $wide = Set-SettingsSize 1500 1000
    Write-Host "BASELINE — establish Mouse at $($wide.Width)x$($wide.Height)" -ForegroundColor Yellow
    Establish-Place 'mouse settings'
    $baseline = Get-Reading 'MOUSE baseline (wide)'
    $MOUSE_A = $baseline.Subject
    Show-Reading $baseline
    if (-not $MOUSE_A) { throw "no baseline Place was established; the rest of the run would be meaningless" }
    Write-Host "  MOUSE_A = $MOUSE_A" -ForegroundColor Cyan

    Write-Host ""
    Write-Host "TEST 1 — same presentation (control)" -ForegroundColor Yellow
    Show-Reading (Get-Reading 'same presentation') $MOUSE_A

    Write-Host ""
    Write-Host "TEST 2 — wide -> narrow reflow" -ForegroundColor Yellow
    $narrow = Set-SettingsSize 700 1000
    Write-Host "  resized to $($narrow.Width)x$($narrow.Height)"
    Show-Reading (Get-Reading 'narrow') $MOUSE_A

    Write-Host ""
    Write-Host "TEST 3 — narrow -> wide" -ForegroundColor Yellow
    $back = Set-SettingsSize 1500 1000
    Write-Host "  resized to $($back.Width)x$($back.Height)"
    Show-Reading (Get-Reading 'wide again') $MOUSE_A

    Write-Host ""
    Write-Host "TEST 4 — presentation variant (narrow enough to collapse the nav)" -ForegroundColor Yellow
    $tiny = Set-SettingsSize 560 900
    Write-Host "  resized to $($tiny.Width)x$($tiny.Height)"
    Show-Reading (Get-Reading 'nav collapsed') $MOUSE_A
    [void](Set-SettingsSize 1500 1000)

    Write-Host ""
    Write-Host "TEST 5 — Director restart" -ForegroundColor Yellow
    Start-AcceptanceDirector
    Show-Reading (Get-Reading 'after restart') $MOUSE_A

    Write-Host ""
    Write-Host "TEST 6 — a genuinely different destination" -ForegroundColor Yellow
    Open-SettingsPage 'ms-settings:bluetooth'
    Establish-Place 'bluetooth settings'
    $bt = Get-Reading 'BLUETOOTH'
    Show-Reading $bt
    $BLUETOOTH_B = $bt.Subject
    Write-Host "  BLUETOOTH_B = $BLUETOOTH_B" -ForegroundColor Cyan
    if ($BLUETOOTH_B -eq $MOUSE_A) {
        Write-Host "  FALSE MERGE — Bluetooth and Mouse are the same Place" -ForegroundColor Red
    }
    Open-SettingsPage 'ms-settings:mousetouchpad'
    Show-Reading (Get-Reading 'back to Mouse') $MOUSE_A

    Write-Host ""
    Write-Host "TEST 7 — restart with both Places" -ForegroundColor Yellow
    Start-AcceptanceDirector
    Show-Reading (Get-Reading 'Mouse after 2nd restart') $MOUSE_A
    Open-SettingsPage 'ms-settings:bluetooth'
    Show-Reading (Get-Reading 'Bluetooth after 2nd restart') $BLUETOOTH_B

    Write-Host ""
    Write-Host "STORE" -ForegroundColor Yellow
    $subjects = Get-StoreSubjects
    Write-Host "  $($subjects.Count) durable subject(s):"
    foreach ($s in $subjects) {
        Write-Host ("    {0}  {1,-12} roles={2}" -f $s.id, $s.semantic,
            (($s.structure.roles.PSObject.Properties | ForEach-Object { "$($_.Name):$($_.Value)" }) -join ' '))
    }

    Write-Host ""
    Write-Host "SUMMARY" -ForegroundColor Cyan
    $script:Results | Format-Table Label, Outcome, Subject, Elements, Sufficiency, Width, Sensors -AutoSize |
        Out-String | Write-Host
}
finally {
    Stop-AcceptanceDirector
    # THE ENVIRONMENT GOES BACK, and the real store is checked rather than trusted.
    if ($null -eq $script:PriorMarcoHome) {
        Remove-Item Env:\MARCO_HOME -ErrorAction SilentlyContinue
    } else {
        $env:MARCO_HOME = $script:PriorMarcoHome
    }
    Remove-Item Env:\MARCO_VISION_MODEL -ErrorAction SilentlyContinue
    Remove-Item Env:\MARCO_ONNXRUNTIME  -ErrorAction SilentlyContinue

    $changed = Compare-TreeHash $script:RealHomeBefore (Get-TreeHash $script:RealHome)
    if ($changed.Count -eq 0) {
        Write-Host "REAL_USER_STORE_MODIFIED: NO" -ForegroundColor Green
    } else {
        Write-Host "REAL_USER_STORE_MODIFIED: YES" -ForegroundColor Red
        $changed | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    }
    if (-not $KeepHome) { Remove-Item -Recurse -Force $AcceptanceHome -ErrorAction SilentlyContinue }
    else { Write-Host "acceptance home kept at $AcceptanceHome" -ForegroundColor DarkGray }
}
