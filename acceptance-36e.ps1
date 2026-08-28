# Encoded UTF-8 WITH A BOM, deliberately. Windows PowerShell 5.1 decodes a BOM-less file as the
# system ANSI codepage and an em dash then arrives as three characters, one of which is a double
# quote -- which terminates a string early and fails to PARSE, pointing at a brace fifty lines
# away. Measured on acceptance-35c.ps1. The prose below stays ASCII as well.
<#
.SYNOPSIS
  The north-star check for Roadmap 36E: Marco chooses the better known way, and says why.

.DESCRIPTION
  Deterministic tests hold the policy and the wiring. This is the one thing they cannot supply: a
  real Settings tree, where two ways to one page genuinely exist and the evidence about them is
  whatever your afternoon happened to produce.

  THE CLAIM:

      when Marco knows two ways to the same place, it picks using what it knows about them --
      and it can tell you why in words, before it does anything.

      .\acceptance-36e.ps1 -Setup   build, sandbox, start a Director, watch AND learn
      .\acceptance-36e.ps1 -Why     what route would Marco take, and on what grounds
      .\acceptance-36e.ps1 -Clean   stop the Director and delete the sandbox

  -Why DRIVES NOTHING. It asks the planner the same question the performer asks, and prints the
  route with the planner's own reasons. Run it, change the evidence, run it again.

  YOUR REAL PLAYS AND MEMORY ARE NEVER WRITTEN TO.

.EXAMPLE
  .\acceptance-36e.ps1 -Setup
  # walk Home -> Bluetooth & devices -> Mouse
  # say:  marco learn "mouse settings" --recent
  # walk back Home, then click "Mouse" directly if your Settings offers it
  .\acceptance-36e.ps1 -Why
#>
[CmdletBinding()]
param(
    [switch]$Setup,
    [switch]$Why,
    [switch]$Clean
)

$ErrorActionPreference = "Stop"
$Repo    = $PSScriptRoot
$Sandbox = Join-Path $env:TEMP "marco-36e"
$Home36  = Join-Path $Sandbox "home"
$Store   = Join-Path $Home36 "semantic-memory.json"
$Marco   = Join-Path $Sandbox "marco.exe"
$Dir     = Join-Path $Sandbox "director.exe"
$Phrase  = "mouse settings"

function Say  ($m) { Write-Host $m }
function Step ($m) { Write-Host ""; Write-Host "== $m" -ForegroundColor Cyan }
function Good ($m) { Write-Host "   OK   $m" -ForegroundColor Green }
function Warn ($m) { Write-Host "   ??   $m" -ForegroundColor Yellow }
function Bad  ($m) { Write-Host "   XX   $m" -ForegroundColor Red }
function Note ($m) { Write-Host "        $m" -ForegroundColor DarkGray }

function Use-Sandbox {
    $env:MARCO_HOME   = $Home36
    $env:MARCO_ROUTES = Join-Path $Home36 "routes"
    $env:MARCO_BIN    = $Marco
}

# Resolved BEFORE anything is sandboxed and pinned, so a second invocation in the same terminal
# agrees with the first. Reading it afterwards finds the throwaway home and reports it back as
# "your real store", which acceptance-36c.ps1 did until it was measured.
$RealStorePin = Join-Path $Sandbox "real-store.txt"
function Real-Store {
    if (Test-Path $RealStorePin) { return (Get-Content $RealStorePin -Raw).Trim() }
    $h = $env:MARCO_HOME
    if (-not $h) { $h = Join-Path $env:LOCALAPPDATA "marco" }
    return (Join-Path $h "semantic-memory.json")
}

if ($Clean) {
    Get-Process director -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -eq $Dir } | Stop-Process -Force
    Remove-Item $Sandbox -Recurse -Force -ErrorAction SilentlyContinue
    Good "sandbox gone"
    return
}

if ($Setup) {
    $real = Real-Store
    Step "Building"
    Push-Location $Repo
    try {
        New-Item -ItemType Directory -Force $Sandbox | Out-Null
        New-Item -ItemType Directory -Force $Home36  | Out-Null
        Set-Content -Path $RealStorePin -Value $real -Encoding utf8
        & go build -o $Marco ./cmd/marco
        if (-not $?) { Bad "marco did not build"; return }
        & go build -o $Dir ./cmd/director
        if (-not $?) { Bad "director did not build"; return }
        Good "marco.exe and director.exe"
    } finally { Pop-Location }

    Step "Sandboxing your memory"
    if (Test-Path $real) {
        Copy-Item $real $Store -Force
        Good "copied $real"
    } else {
        Warn "no semantic memory at $real -- starting cold, which is a harder test"
    }

    Step "Watching, and learning from what it sees"
    Use-Sandbox
    Get-Process director -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -eq $Dir } | Stop-Process -Force
    & $Marco observe | Out-Null
    Start-Sleep -Seconds 2
    & $Marco observe learn | Out-Null
    Start-Sleep -Seconds 1
    $st = (& $Marco observe status --json 2>$null) | ConvertFrom-Json
    if (-not $st.watching -or -not $st.learning) { Bad "watching and learning did not both start"; return }
    Good "Marco is watching and may remember what it sees"

    Step "Give it two ways to one page"
    Say  "  1. Open Settings. Walk: Home -> 'Bluetooth & devices' -> 'Mouse'."
    Say  "  2. Say what that was:"
    Say  ""
    Say  "         $Marco learn `"$Phrase`" --recent"
    Say  ""
    Say  "  3. Go back to Home. If your Settings shows 'Mouse' on the Home page, click it."
    Say  "     If it does not, walk a DIFFERENT two-step way to Mouse instead."
    Say  ""
    Say  "  Then run:  .\acceptance-36e.ps1 -Why"
    Note ""
    Note "Two known ways is the whole setup. Which one Marco picks, and the grounds it gives,"
    Note "is the roadmap."
    return
}

if ($Why) {
    Use-Sandbox
    if (-not (Test-Path $Store)) { Bad "no store at $Store -- run -Setup first"; return }
    $f = Get-Content $Store -Raw | ConvertFrom-Json
    $named = @{}
    foreach ($s in @($f.subjects)) { $named[$s.id] = $s.name }

    Step "What Marco knows"
    Say ("  {0} screen(s), {1} edge(s), {2} goal(s)" -f
         @(@($f.subjects) | Where-Object { $_.structure.subject -ne "target" }).Count,
         @($f.relationships).Count, @($f.goals).Count)
    foreach ($e in @($f.relationships)) {
        $from = $named[$e.from]; if (-not $from) { $from = $e.from }
        $to   = $named[$e.to];   if (-not $to)   { $to   = $e.to }
        Say ("    {0}  ->  {1}   (watched {2})" -f $from, $to, $e.observations)
    }
    # WHAT MARCO DOES NOT UNDERSTAND, read from the same ledger the planner reads.
    $confused = @(@($f.watched) | Where-Object { $_.contradicted -gt 0 })
    if ($confused.Count -gt 0) {
        Warn "$($confused.Count) way(s) Marco has seen lead somewhere else too"
        Note "Those stay usable and the planner would rather not: see the reasons below."
    }

    Step "Which way would it take, and why"
    # THE DIAGNOSTIC, and it drives nothing. Same planner, same grade, same reasons the
    # performer would act on.
    $out = & $Dir reach $Phrase 2>&1
    Say ($out | Out-String)

    Step "Ask again from somewhere else"
    Note "The route depends on where you are standing, so the interesting question is usually"
    Note "about a place you are not. Pick a screen id from the list above and try:"
    Say  ""
    foreach ($s in @(@($f.subjects) | Where-Object { $_.structure.subject -ne "target" } | Select-Object -First 3)) {
        $n = $s.name; if (-not $n) { $n = "(unnamed)" }
        Say ("    $Dir reach `"$Phrase`" --from {0}     # {1}" -f $s.id, $n)
    }
    Say  ""
    Note "Then change the evidence -- walk one of the ways again, or let Marco perform it --"
    Note "and run -Why once more. The goal does not change. The route may."
    Note "Your real store is at $(Real-Store) and this script never opened it for writing."
    Say  "  Finished? .\acceptance-36e.ps1 -Clean"
    return
}

Say "acceptance-36e.ps1 -- the north-star check for choosing the better known way"
Say ""
Say "  -Setup   build, sandbox, start a Director, watch AND learn"
Say "  -Why     what route would Marco take, and on what grounds (drives nothing)"
Say "  -Clean   stop the Director and delete the sandbox"
Say ""
Say "  Two known ways to one page. Marco picks with what it knows, and says why."
Say ""
Say "  Get-Help .\acceptance-36e.ps1 -Detailed   for what this is actually testing"
