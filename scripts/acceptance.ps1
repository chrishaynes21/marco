<#
.SYNOPSIS
  Bring up a CLEAN Marco stack for a live Learn acceptance run.

.DESCRIPTION
  Roadmap 34 acceptance is a physical test: a person names a behaviour, demonstrates it, and
  watches what Marco made of it. That only proves anything if the run starts from nothing.

  It has repeatedly not. A failed or mis-targeted Learn attempt leaves durable subjects for the
  wrong foreground window, candidate routes nobody wants, and proposals holding the single
  interruption slot — and the next run interacts with the wreckage rather than with the code.
  One afternoon produced fifteen subjects, six candidate routes, and several near-identical
  Settings pages minted by passes that had already gone wrong.

  So this script builds the current binaries, empties a SANDBOX home, starts the stack against
  it, and refuses to hand over until the slate is provably clean.

  THE USER'S REAL STORE IS NEVER TOUCHED. Everything runs against -Home, which defaults to a
  throwaway directory, and `director reset-test-state` refuses any home that is the real one.

.PARAMETER Home
  The sandbox. Defaults to a fresh timestamped directory under the system temp folder.
  Pass the same value twice to keep a sandbox across runs.

.PARAMETER Fresh
  Use a brand-new sandbox even if -Home names an existing one.

.EXAMPLE
  .\scripts\acceptance.ps1
#>
[CmdletBinding()]
param(
    [string]$Home_ = '',
    [switch]$Fresh
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
Set-Location $repo

function Step($n, $text) { Write-Host "`n[$n] $text" -ForegroundColor Cyan }
function Ok($text)       { Write-Host "    OK  $text" -ForegroundColor Green }
function Warn($text)     { Write-Host "    !!  $text" -ForegroundColor Yellow }
function Die($text)      { Write-Host "    XX  $text" -ForegroundColor Red; exit 1 }

# ── 0. the binaries under test ────────────────────────────────────────────────
#
# Built FIRST, so the run cannot accidentally exercise yesterday's director.exe. Windows locks a
# running executable, so this must happen before anything starts — and after the stop below it
# would be too late.
Step 0 'Building the current binaries'
$null = & go build -o director.exe ./cmd/director 2>&1
if ($LASTEXITCODE -ne 0) { Die 'director did not build' }
$null = & go build -o marco.exe ./cmd/marco 2>&1
if ($LASTEXITCODE -ne 0) { Die 'marco did not build' }
Ok 'director.exe and marco.exe are current'

# ── 1. stop the Director gracefully ───────────────────────────────────────────
Step 1 'Stopping any running Director'
& .\director.exe shutdown 2>&1 | Out-Null
Start-Sleep -Milliseconds 600
Ok 'shutdown requested'

# ── 2. no stale processes ─────────────────────────────────────────────────────
#
# A survivor holds the old semantic memory in memory and writes it back over a cleared sandbox,
# which looks exactly like the reset not having worked.
Step 2 'Verifying nothing stale is alive'
foreach ($name in 'director', 'marco', 'overlay') {
    $procs = Get-Process $name -ErrorAction SilentlyContinue
    if ($procs) {
        Warn "$name still running (pid $($procs.Id -join ', ')) - stopping"
        $procs | Stop-Process -Force
        Start-Sleep -Milliseconds 400
    }
}
foreach ($name in 'director', 'marco', 'overlay') {
    if (Get-Process $name -ErrorAction SilentlyContinue) { Die "$name would not stop" }
}
Ok 'no director / marco / overlay processes'

# ── 3. a clean sandbox ────────────────────────────────────────────────────────
Step 3 'Preparing the sandbox'
if ([string]::IsNullOrWhiteSpace($Home_) -or $Fresh) {
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $Home_ = Join-Path $env:TEMP "marco-acceptance-$stamp"
}
if (-not (Test-Path $Home_)) { New-Item -ItemType Directory -Force $Home_ | Out-Null }
$env:MARCO_HOME = $Home_

# THE PLAY STORE IS A SECOND SANDBOX, and it does not follow MARCO_HOME.
#
# $MARCO_ROUTES and $MARCO_HOME are independent: neither implies the other. Isolating only the
# home left the routes tree resolving to "routes" relative to the working directory, which for a
# Director started from the repo is the USER'S OWN tree — so an acceptance Learn wrote its play in
# beside their real ones, and a rerun could collide with the name it left behind. A test that
# writes into the thing it is testing is not a clean run.
$env:MARCO_ROUTES = Join-Path $Home_ 'routes'
if (-not (Test-Path $env:MARCO_ROUTES)) {
    New-Item -ItemType Directory -Force $env:MARCO_ROUTES | Out-Null
}
$repoRoutes = Join-Path $repo 'routes'
if ([IO.Path]::GetFullPath($env:MARCO_ROUTES).TrimEnd('\') -ieq
    [IO.Path]::GetFullPath($repoRoutes).TrimEnd('\')) {
    Die "the sandbox routes tree resolved to the real one ($repoRoutes)"
}
$strayPlays = @(Get-ChildItem $env:MARCO_ROUTES -Recurse -Filter *.marco -ErrorAction SilentlyContinue)
if ($strayPlays.Count -ne 0) {
    Die "$($strayPlays.Count) play(s) already in the sandbox routes tree - pass -Fresh"
}

# The reset refuses a home that is the real store, so this is safe even if someone passes one.
$reset = & .\director.exe reset-test-state 2>&1
if ($LASTEXITCODE -ne 0) { Die "reset refused: $reset" }
$reset | ForEach-Object { Write-Host "    $_" -ForegroundColor DarkGray }
Ok "sandbox is $Home_"
Ok "plays go to $env:MARCO_ROUTES"

# ── 4. start the stack on the current binaries ────────────────────────────────
Step 4 'Starting Director and the control centre'
$dirLog = Join-Path $Home_ 'director.log'
$uiLog  = Join-Path $Home_ 'ui.log'
Start-Process -FilePath (Join-Path $repo 'director.exe') -ArgumentList 'serve' `
    -RedirectStandardOutput $dirLog -RedirectStandardError (Join-Path $Home_ 'director.err') `
    -WindowStyle Hidden
$up = $false
foreach ($i in 1..40) {
    Start-Sleep -Milliseconds 250
    & .\director.exe status 2>&1 | Out-Null
    if ($LASTEXITCODE -eq 0) { $up = $true; break }
}
if (-not $up) { Die "the Director did not come up; see $dirLog" }
Ok 'Director is listening'

Start-Process -FilePath (Join-Path $repo 'marco.exe') -ArgumentList 'ui', 'learn' `
    -RedirectStandardOutput $uiLog -RedirectStandardError (Join-Path $Home_ 'ui.err') `
    -WindowStyle Hidden
$url = $null
foreach ($i in 1..40) {
    Start-Sleep -Milliseconds 250
    if (Test-Path $uiLog) {
        $line = Get-Content $uiLog -TotalCount 1
        if ($line -match '(http://\S+)') { $url = $Matches[1].TrimEnd('/'); break }
    }
}
if (-not $url) { Die "the control centre did not come up; see $uiLog" }
Ok "control centre at $url"

# ── 5-7. prove the slate is clean ─────────────────────────────────────────────
#
# Read through the PRODUCTION surface — the same endpoint the panel polls — rather than by
# looking at files. A sandbox that is empty on disk and a Director that already believes
# something are different situations, and only one of them is visible here.
Step 5 'Verifying the slate is clean'
$learn = Invoke-RestMethod -Uri "$url/api/learn" -TimeoutSec 10

if (-not $learn.available) { Die 'the panel cannot reach the Director' }

$q = [int]$learn.questions_open
if ($q -ne 0) { Die "$q question(s) already open - a stale question holds the interruption slot" }
Ok 'questions open: 0'

if ($learn.running) { Die 'a learning session is already running' }
Ok 'no session in flight'

# omitempty means an absent array, and @($null).Count is 1 in PowerShell — the nulls
# have to go before counting or an empty sandbox reports one place.
$places = @($learn.places | Where-Object { $_ }).Count
if ($places -ne 0) { Die "$places durable place(s) already known - the sandbox is not clean" }
Ok 'durable subjects: 0'

if ([int]$learn.captured -ne 0) { Die "captured actions is $($learn.captured), want 0" }
Ok 'captured actions: 0'

# Grants and candidates are in-memory and per-Director; a Director that just started has
# neither. Asserted through status rather than assumed.
$status = & .\director.exe status 2>&1 | Out-String
# Asserted POSITIVELY. A negative lookahead after \s* matches with zero spaces consumed, so
# "Active command: none" satisfied it and a freshly started Director looked busy.
if ($status -notmatch 'Active command:\s+none') { Die "a command is already active:`n$status" }
Ok 'no active command, no grant'

# ---- 8. hand over --------------------------------------------------------------
#
# Plain ASCII from here down. This banner is the last thing a person reads before doing the
# physical test, and a mangled box-drawing character in a terminal that is not UTF-8 makes the
# whole instruction look broken.
Write-Host ""
Write-Host "=============================================" -ForegroundColor DarkGray
Write-Host " Clean stack ready" -ForegroundColor Green
Write-Host "   sandbox : $Home_"
Write-Host "   plays   : $env:MARCO_ROUTES"
Write-Host "   panel   : $url"
Write-Host ""
Write-Host " Before you demonstrate, the panel must show:" -ForegroundColor Cyan
Write-Host "   Watching: (your application)"
Write-Host "   Target locked: YES"
Write-Host "   Captured actions: 0"
Write-Host "   Questions open: 0"
Write-Host ""
Write-Host " Name it, press Start, go to the application and WAIT for" -ForegroundColor Cyan
Write-Host " 'Target locked: YES' before clicking anything."
Write-Host "=============================================" -ForegroundColor DarkGray
