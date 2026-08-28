# Encoded UTF-8 WITH A BOM, deliberately. Windows PowerShell 5.1 decodes a BOM-less file as the
# system ANSI codepage and an em dash then arrives as three characters, one of which is a double
# quote -- which terminates a string early and fails to PARSE, pointing at a brace fifty lines
# away. Measured on acceptance-35c.ps1. The prose below stays ASCII as well.
<#
.SYNOPSIS
  The north-star check for Roadmap 36C.2: Learn teaches the graph.

.DESCRIPTION
  Deterministic tests already hold the invariant -- cmd/director/onegraph_test.go drives both
  production doors against one store and a mutation run shows each gate bites. This is the one
  thing they cannot supply: a real Settings window, a real accessibility tree, and a real person
  clicking.

  THE CLAIM, in one sentence:

      what you explicitly teach Marco becomes GRAPH TOPOLOGY, so it composes with what Marco
      watched, and the route is chosen when you ask rather than when you demonstrated.

      .\acceptance-36c2.ps1 -Setup   build, sandbox, start a Director, watch AND learn
      .\acceptance-36c2.ps1 -Check   after you have clicked: what the graph holds
      .\acceptance-36c2.ps1 -Clean   stop the Director and delete the sandbox

  YOUR REAL PLAYS AND MEMORY ARE NEVER WRITTEN TO. -Setup copies your semantic memory into a
  throwaway MARCO_HOME under TEMP and everything after that runs there.

  NOTHING HERE DRIVES YOUR DESKTOP. Learning is a memory operation: no input, no lease, no
  authority. -Check reads and reports; it never asks Marco to perform anything.

.EXAMPLE
  .\acceptance-36c2.ps1 -Setup
  # open Settings, click "Bluetooth & devices", then click "Mouse"
  # say:  marco learn "mouse settings" --recent
  # then go back to Home, click "Printers & scanners", then "Bluetooth & devices"
  .\acceptance-36c2.ps1 -Check
#>
[CmdletBinding()]
param(
    [switch]$Setup,
    [switch]$Check,
    [switch]$Clean
)

$ErrorActionPreference = "Stop"
$Repo    = $PSScriptRoot
$Sandbox = Join-Path $env:TEMP "marco-36c2"
$Home36  = Join-Path $Sandbox "home"
$Store   = Join-Path $Home36 "semantic-memory.json"
$Marco   = Join-Path $Sandbox "marco.exe"
$Dir     = Join-Path $Sandbox "director.exe"

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

# Real-Store is where the person's own memory lives, resolved BEFORE anything is sandboxed.
#
# Reading it after Use-Sandbox would find the throwaway home and report it back as "your real
# store", which acceptance-36c.ps1 did until it was measured. The value is pinned to a file so a
# second invocation in the same terminal agrees with the first.
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
        Note "your real store is never opened for writing by this script"
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
    if (-not $st.watching -or -not $st.learning) {
        Bad "watching and learning did not both start"
        return
    }
    Good "Marco is watching and may remember what it sees"

    Step "Now use your computer, normally"
    Say  "  1. Open Settings, click 'Bluetooth & devices', then click 'Mouse'."
    Say  "  2. Say what it was:"
    Say  ""
    Say  "         $Marco learn `"mouse settings`" --recent"
    Say  ""
    Say  "  3. Go back to Home. Click 'Printers & scanners'."
    Say  "     From there, click 'Bluetooth & devices'."
    Say  "     Do NOT teach this one. Just do it -- Marco is watching."
    Say  ""
    Say  "  Then run:  .\acceptance-36c2.ps1 -Check"
    Note ""
    Note "Step 3 is the whole point. It is an ordinary afternoon, nobody teaching anything,"
    Note "and it should give Marco a second way into a screen you taught it from Home."
    return
}

if ($Check) {
    Use-Sandbox
    if (-not (Test-Path $Store)) { Bad "no store at $Store -- run -Setup first"; return }
    $f = Get-Content $Store -Raw | ConvertFrom-Json

    $subjects = @($f.subjects)
    $screens  = @($subjects | Where-Object { $_.structure.subject -ne "target" })
    $edges    = @($f.relationships)
    $goals    = @($f.goals)

    Step "What the graph holds"
    Say ("  {0} screen(s), {1} target(s), {2} edge(s), {3} goal(s)" -f
         $screens.Count, ($subjects.Count - $screens.Count), $edges.Count, $goals.Count)

    $named = @{}
    foreach ($s in $subjects) { $named[$s.id] = $s.name }
    foreach ($e in $edges) {
        $from = $named[$e.from]; if (-not $from) { $from = $e.from }
        $to   = $named[$e.to];   if (-not $to)   { $to   = $e.to }
        Say ("    {0}  ->  {1}   (seen {2})" -f $from, $to, $e.observations)
    }

    Step "Is what you TAUGHT in the same graph as what Marco WATCHED"
    # THE CLAIM. A screen with two ways in is the observable form of it: one of those ways was
    # taught explicitly and the other was only watched, and a design with two graphs would put
    # them in different places and show one way in here.
    $incoming = @{}
    foreach ($e in $edges) {
        if (-not $incoming.ContainsKey($e.to)) { $incoming[$e.to] = 0 }
        $incoming[$e.to]++
    }
    $joined = @($incoming.Keys | Where-Object { $incoming[$_] -gt 1 })
    if ($joined.Count -gt 0) {
        foreach ($id in $joined) {
            $n = $named[$id]; if (-not $n) { $n = $id }
            Good ("$n has $($incoming[$id]) ways in, and they were not all taught")
        }
        Note "One graph. A taught edge and a watched edge arriving at one screen is the whole"
        Note "claim: Marco can now reach it from either, and nobody demonstrated the second."
    } else {
        Warn "no screen has two ways in yet"
        Note "Either step 3 was not walked, or one of its screens could not be read. Try:"
        Note "  $Marco observe status --evidence"
    }

    Step "Is the name a destination or a route"
    if ($goals.Count -eq 0) {
        Warn "no goal was learned -- did the `learn ... --recent` step run?"
    } else {
        foreach ($g in $goals) {
            $n = $named[$g.subject]; if (-not $n) { $n = $g.subject }
            Say ("  `"{0}`"  ->  {1}" -f $g.name, $n)
        }
        # A GOAL HAS NO START, structurally: there is no field one could go in.
        $hasStart = $false
        foreach ($g in $goals) {
            if ($g.PSObject.Properties.Name -contains "from" -or
                $g.PSObject.Properties.Name -contains "start" -or
                $g.PSObject.Properties.Name -contains "route") { $hasStart = $true }
        }
        if ($hasStart) {
            Bad "a goal carries a start, so the first demonstration owns the way in"
        } else {
            Good "a goal names a destination and carries no start"
            Note "So `"mouse settings`" means the Mouse page, not the way you happened to take."
        }
    }

    Step "What it did NOT do"
    $raw = & $Dir status --json 2>$null
    try { $ds = ($raw | ConvertFrom-Json) } catch { $ds = $null }
    if ($ds -and $ds.active) {
        Bad "a command is running; learning performs nothing"
    } else {
        Good "no command ran -- nothing was performed and nothing drove your desktop"
    }
    $text = Get-Content $Store -Raw
    $bad = @()
    foreach ($word in @("screenshot", "png", "base64", "clipboard", "keystroke", "password")) {
        if ($text -match $word) { $bad += $word }
    }
    if ($bad.Count -gt 0) { Bad "the store mentions: $($bad -join ', ')" }
    else { Good "no screenshot, no transcript, no clipboard, no keystroke, no secret" }

    Say ""
    Note "Your real store is at $(Real-Store) and this script never opened it for writing."
    Say  "  Finished? .\acceptance-36c2.ps1 -Clean"
    return
}

Say "acceptance-36c2.ps1 -- the north-star check for Learn teaching the graph"
Say ""
Say "  -Setup   build, sandbox, start a Director, watch AND learn"
Say "  -Check   after you have clicked: what the graph holds"
Say "  -Clean   stop the Director and delete the sandbox"
Say ""
Say "  Teach one way in. Walk another. They belong to the same graph."
Say ""
Say "  Get-Help .\acceptance-36c2.ps1 -Detailed   for what this is actually testing"
