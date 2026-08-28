# Encoded UTF-8 WITH A BOM, deliberately. Windows PowerShell 5.1 decodes a BOM-less file as the
# system ANSI codepage and an em dash then arrives as three characters, one of which is a double
# quote -- which terminates a string early and fails to PARSE, pointing at a brace fifty lines
# away. Measured on acceptance-35c.ps1. The prose below stays ASCII as well.
<#
.SYNOPSIS
  The north-star check for Roadmap 36D: a name means an outcome, not a route.

.DESCRIPTION
  Deterministic tests hold the layer -- one resolver shared by the performer and the diagnostic,
  ambiguity refused rather than guessed, a reused name saying what it used to mean. This is the
  one thing they cannot supply: a real Settings window and a real person clicking.

  THE CLAIM, in one sentence:

      teach Marco what "mouse settings" MEANS once, and it will find its own way there from
      wherever you happen to be standing next time.

      .\acceptance-36d.ps1 -Setup   build, sandbox, start a Director, watch AND learn
      .\acceptance-36d.ps1 -Means   what does that phrase mean, and how would Marco get there
      .\acceptance-36d.ps1 -Do      actually carry it out (DRIVES REAL INPUT)
      .\acceptance-36d.ps1 -Clean   stop the Director and delete the sandbox

  -Means DRIVES NOTHING. It asks the same question the performer asks, of the same store, and
  prints the whole chain: phrase, outcome, destination, where Marco thinks you are, and the route
  it would choose. Run it before -Do, from a different screen each time.

  -Do PERFORMS REAL INPUT. It is the only mode here that touches your desktop.

  YOUR REAL PLAYS AND MEMORY ARE NEVER WRITTEN TO. -Setup copies your semantic memory into a
  throwaway MARCO_HOME under TEMP and everything after that runs there.

.EXAMPLE
  .\acceptance-36d.ps1 -Setup
  # open Settings, click "Bluetooth & devices", then click "Mouse"
  # say:  marco learn "mouse settings" --recent
  # go back to Home, click "Printers & scanners", then "Bluetooth & devices"
  .\acceptance-36d.ps1 -Means
  # now stand somewhere ELSE in Settings and ask again
  .\acceptance-36d.ps1 -Means
  .\acceptance-36d.ps1 -Do
#>
[CmdletBinding()]
param(
    [switch]$Setup,
    [switch]$Means,
    [switch]$Do,
    [switch]$Clean
)

$ErrorActionPreference = "Stop"
$Repo    = $PSScriptRoot
$Sandbox = Join-Path $env:TEMP "marco-36d"
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

# Real-Store is resolved BEFORE anything is sandboxed and pinned, so a second invocation in the
# same terminal agrees with the first. Reading it after Use-Sandbox finds the throwaway home and
# reports it back as "your real store", which acceptance-36c.ps1 did until it was measured.
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

    Step "Teach it what the words mean, once"
    Say  "  1. Open Settings, click 'Bluetooth & devices', then click 'Mouse'."
    Say  "  2. Say what that was:"
    Say  ""
    Say  "         $Marco learn `"$Phrase`" --recent"
    Say  ""
    Say  "  3. Go back to Home. Click 'Printers & scanners'."
    Say  "     From there, click 'Bluetooth & devices'. Do NOT teach this one."
    Say  ""
    Say  "  Then run:  .\acceptance-36d.ps1 -Means"
    Note ""
    Note "Step 3 gives Marco a second way into Bluetooth that nobody taught it. The claim is"
    Note "that the MEANING of the phrase does not change and the ROUTE does."
    return
}

if ($Means) {
    Use-Sandbox
    Step "What does that phrase mean"
    # THE DIAGNOSTIC, and it drives nothing. It asks the same resolver the performer asks, so
    # what it prints is what would be acted on rather than a second opinion about it.
    $out = & $Dir reach $Phrase 2>&1
    Say ($out | Out-String)

    Step "What is on disk"
    if (-not (Test-Path $Store)) { Bad "no store at $Store"; return }
    $f = Get-Content $Store -Raw | ConvertFrom-Json
    $named = @{}
    foreach ($s in @($f.subjects)) { $named[$s.id] = $s.name }
    foreach ($g in @($f.goals)) {
        $n = $named[$g.subject]; if (-not $n) { $n = $g.subject }
        Say ("  `"{0}`"  means  {1}" -f $g.name, $n)
    }
    if (@($f.goals).Count -eq 0) { Warn "no goal was learned -- did the learn step run?" }

    Say ""
    Note "Run this again from a DIFFERENT Settings screen. The `"means`" line must not change."
    Note "The route under it should. That is the whole roadmap in two readings."
    Say  ""
    Say  "  Ready? .\acceptance-36d.ps1 -Do    (this one drives real input)"
    return
}

if ($Do) {
    Use-Sandbox
    Step "Carrying it out"
    Warn "this drives REAL INPUT into whatever Settings window is in front"
    Note "Marco will bring Settings forward, look at where you actually are, plan from there,"
    Note "and verify it arrived. If perception misses, let it fail -- do not slow down for it."
    Say  ""
    $out = & $Dir perform $Phrase 2>&1
    Say ($out | Out-String)
    Say ""
    Note "Compare what it walked with what -Means predicted from the same place. The route is"
    Note "chosen at this moment, so the two should agree and neither should mention where the"
    Note "demonstration originally began."
    Note "Your real store is at $(Real-Store) and this script never opened it for writing."
    Say  "  Finished? .\acceptance-36d.ps1 -Clean"
    return
}

Say "acceptance-36d.ps1 -- the north-star check for a name meaning an outcome"
Say ""
Say "  -Setup   build, sandbox, start a Director, watch AND learn"
Say "  -Means   what the phrase means, and how Marco would get there (drives nothing)"
Say "  -Do      carry it out (DRIVES REAL INPUT)"
Say "  -Clean   stop the Director and delete the sandbox"
Say ""
Say "  Teach the meaning once. Ask from anywhere. The route is today's problem."
Say ""
Say "  Get-Help .\acceptance-36d.ps1 -Detailed   for what this is actually testing"
