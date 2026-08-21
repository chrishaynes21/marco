<#
.SYNOPSIS
  The minimum live acceptance for Roadmap 35B (Fast Learn). Three commands and one demonstration.

.DESCRIPTION
  Everything a Go test can check is already checked. What no test can reach is a person
  demonstrating a real route on a real desktop, so that is the only thing this asks you to do.

  Run:
      .\acceptance-35b.ps1 -Setup     builds, sandboxes, starts the Director, tells you what to do
      .\acceptance-35b.ps1 -Check     reads the store and reports PASS/FAIL per claim
      .\acceptance-35b.ps1 -Clean     stops the Director and deletes the sandbox

  YOUR REAL PLAYS AND MEMORY ARE NEVER TOUCHED. Every step runs against a throwaway
  $MARCO_HOME and $MARCO_ROUTES under $env:TEMP, and -Clean deletes them.

  Nothing here performs input on your behalf. `director learn` watches; it emits nothing.
#>
[CmdletBinding()]
param(
    [switch]$Setup,
    [switch]$Check,
    [switch]$Clean,
    # The behaviour being learned. Change it only if you are demonstrating something else.
    [string]$Name = "open mouse settings"
)

$ErrorActionPreference = "Stop"
$Root    = $PSScriptRoot
$Sandbox = Join-Path $env:TEMP "marco-35b"
$Home35  = Join-Path $Sandbox "home"
$Routes  = Join-Path $Sandbox "routes"
$Store   = Join-Path $Home35 "semantic-memory.json"
$Marco   = Join-Path $Sandbox "marco.exe"
$Dir     = Join-Path $Sandbox "director.exe"

function Use-Sandbox {
    $env:MARCO_HOME   = $Home35
    $env:MARCO_ROUTES = $Routes
}

function Say([string]$m)  { Write-Host $m }
function Step([string]$m) { Write-Host ""; Write-Host $m -ForegroundColor Cyan }
function Good([string]$m) { Write-Host "  PASS  $m" -ForegroundColor Green }
function Bad([string]$m)  { Write-Host "  FAIL  $m" -ForegroundColor Red }
function Note([string]$m) { Write-Host "        $m" -ForegroundColor DarkGray }

# ── setup ────────────────────────────────────────────────────────────────────

if ($Setup) {
    Step "Building"
    New-Item -ItemType Directory -Force $Home35, (Join-Path $Routes "global") | Out-Null
    Push-Location $Root
    try {
        & go build -o $Marco ./cmd/marco;   if ($LASTEXITCODE) { throw "building marco" }
        & go build -o $Dir   ./cmd/director; if ($LASTEXITCODE) { throw "building director" }
    } finally { Pop-Location }
    Say "  marco.exe and director.exe -> $Sandbox"

    # The Accessibility Actor. Semantic Learn cannot see anything without it, and a missing
    # binary is the one precondition worth failing loudly on rather than discovering halfway
    # through a demonstration.
    $bridge = Join-Path $Root "plugins\uia\uia.exe"
    if (-not (Test-Path $bridge)) {
        Bad "the Accessibility provider is not built: $bridge"
        Note "build it with:  powershell -File plugins\uia\build.ps1"
        Note "without it Marco cannot see the screen and this acceptance cannot run."
        exit 1
    }
    Good "Accessibility provider present"

    Use-Sandbox
    Say "  MARCO_HOME   = $Home35"
    Say "  MARCO_ROUTES = $Routes"
    Say "  (your real store and plays are untouched)"

    Step "Starting the Director"
    $running = Get-Process -Name "director" -ErrorAction SilentlyContinue
    if ($running) {
        Note "a director.exe is already running; leaving it alone."
        Note "if it is pointed at your REAL store, stop it first or this will learn into that store."
    } else {
        # The child inherits MARCO_HOME and MARCO_ROUTES from this process — Use-Sandbox set
        # them above. Windows PowerShell 5.1 has no -Environment on Start-Process, and
        # inheritance is the mechanism the rest of Marco already relies on.
        Start-Process -FilePath $Dir -ArgumentList "serve" -WindowStyle Minimized
        Start-Sleep -Seconds 3
    }

    Step "Now the only part that needs you"
    Say ""
    Say "  1. Open Windows Settings, and go to its HOME page."
    Say ""
    Say "  2. In THIS window, start the demonstration:"
    Say ""
    Write-Host "         `$env:MARCO_HOME='$Home35'; `$env:MARCO_ROUTES='$Routes'" -ForegroundColor Yellow
    Write-Host "         $Dir learn `"$Name`" --window-title Settings --actor Mouse --verb Open" -ForegroundColor Yellow
    Say ""
    Say "  3. In Settings, click 'Bluetooth & devices', then click 'Mouse'."
    Say ""
    Say "  4. In a SECOND terminal, end it:"
    Say ""
    Write-Host "         `$env:MARCO_HOME='$Home35'" -ForegroundColor Yellow
    Write-Host "         $Dir learn --finish" -ForegroundColor Yellow
    Say ""
    Say "  5. Come back here and run:  .\acceptance-35b.ps1 -Check"
    Say ""
    Step "What to watch for while you demonstrate"
    Say "  Marco should NOT ask you to name Home, Bluetooth & devices, or Mouse."
    Say "  Marco should NOT ask 'Can I try that?'."
    Say "  Marco should NOT move your mouse or press anything. It is only watching."
    Say ""
    Say "  If any of those three happen, that is the finding. Note it and stop."
    exit 0
}

# ── check ────────────────────────────────────────────────────────────────────

if ($Check) {
    Use-Sandbox
    if (-not (Test-Path $Store)) {
        Bad "no semantic memory at $Store"
        Note "either the demonstration has not run yet, or the Director was pointed elsewhere."
        exit 1
    }

    $mem = Get-Content $Store -Raw | ConvertFrom-Json
    $subjects      = @($mem.subjects)
    $relationships = @($mem.relationships)
    $rehearsals    = @($mem.rehearsals)
    $goals         = @($mem.goals)
    $fails = 0

    Step "What Marco understood"

    # 1. THREE PLACES. Home, Bluetooth & devices, Mouse — recognised and made durable.
    if ($subjects.Count -ge 3) {
        Good "$($subjects.Count) durable Places"
    } else {
        Bad "$($subjects.Count) durable Place(s); the demonstration crossed three screens"
        $fails++
    }
    foreach ($s in $subjects) {
        $called = if ($s.semantic) { $s.semantic } elseif ($s.called) { $s.called } else { "(unnamed)" }
        Note "$called"
    }

    # 2. TWO EDGES. The multi-edge capture, which a previous roadmap had to fix once already.
    if ($relationships.Count -ge 2) {
        Good "$($relationships.Count) durable edges"
    } else {
        Bad "$($relationships.Count) edge(s); Home->Bluetooth and Bluetooth->Mouse are two"
        $fails++
    }

    # 3. NAMES CAME FROM THE SCREEN, not from you. The claim automatic naming makes.
    $named = @($subjects | Where-Object { $_.semantic })
    if ($named.Count -ge 1) {
        Good "$($named.Count) Place(s) named automatically from what was on screen"
    } else {
        Bad "no Place carries an inferred name; you would have had to name them yourself"
        $fails++
    }

    # 4. THE NAV-RAIL REGRESSION. Mouse must not have inherited 'Bluetooth & devices' from the
    #    navigation rail, which stays selected on the sub-page. Two Places under one name makes
    #    both unresolvable, and it has happened before.
    $byName = $named | Group-Object -Property semantic | Where-Object { $_.Count -gt 1 }
    if ($byName) {
        Bad "two Places share the name '$($byName[0].Name)'"
        Note "this is the navigation-rail regression: a sub-page inheriting its section's name."
        $fails++
    } else {
        Good "no two Places share a name"
    }

    Step "What Marco did NOT do"

    # 5. THE CLAIM THE WHOLE ROADMAP TURNS ON. Marco watched; it performed nothing. A rehearsal
    #    record here would mean it replayed the route on your desktop to learn it.
    if ($rehearsals.Count -eq 0) {
        Good "no rehearsal records — Marco performed nothing to learn this"
    } else {
        Bad "$($rehearsals.Count) rehearsal record(s): Marco replayed the route to learn it"
        Note "Fast Learn is meant to learn by watching. This is the ceremony returning."
        $fails++
    }

    Step "What it produced"

    if ($goals.Count -ge 1) { Good "goal recorded: $($goals[0].name)" }
    else { Bad "no goal recorded under your words"; $fails++ }

    $plays = & $Marco plays 2>&1 | Out-String
    $hit = @($plays -split "`n" | Where-Object { $_ -match [regex]::Escape($Name) })
    if ($hit.Count -eq 1) {
        Good "exactly one Play: $($hit[0].Trim())"
    } elseif ($hit.Count -eq 0) {
        Bad "no Play called '$Name'"
        Note ($plays.Trim())
        $fails++
    } else {
        Bad "$($hit.Count) Plays match '$Name' — a repeat demonstration duplicated it"
        $fails++
    }

    # 6. AND IT IS PLANNABLE. An observed edge must be eligible to attempt when you ask for it,
    #    or Fast Learn would have saved something that cannot run.
    $reach = & $Dir reach "$Name" 2>&1 | Out-String
    if ($reach -match "know a way|already there") {
        Good "Marco can plan a way there"
        Note ($reach.Trim() -split "`n" | Select-Object -First 1)
    } else {
        Bad "Marco cannot plan a way to what it just learned"
        Note ($reach.Trim())
        $fails++
    }

    Write-Host ""
    if ($fails -eq 0) {
        Write-Host "  35B acceptance PASSED." -ForegroundColor Green
        Write-Host "  Optional, and worth doing once: run it for real." -ForegroundColor DarkGray
        Write-Host "    $Marco do `"$Name`"" -ForegroundColor DarkGray
        Write-Host "  That is the first time Marco performs it. Success there is execution proof;" -ForegroundColor DarkGray
        Write-Host "  an honest refusal is data, not a failed acceptance." -ForegroundColor DarkGray
    } else {
        Write-Host "  $fails check(s) failed. Each one above says what it means." -ForegroundColor Red
    }
    exit ([int]($fails -gt 0))
}

# ── clean ────────────────────────────────────────────────────────────────────

if ($Clean) {
    Step "Cleaning up"
    Get-Process -Name "director" -ErrorAction SilentlyContinue | ForEach-Object {
        try { $_.Kill(); Note "stopped director (pid $($_.Id))" } catch {}
    }
    if (Test-Path $Sandbox) {
        Remove-Item -Recurse -Force $Sandbox
        Good "removed $Sandbox"
    }
    Remove-Item Env:\MARCO_HOME, Env:\MARCO_ROUTES -ErrorAction SilentlyContinue
    Say "  your real store and plays were never touched."
    exit 0
}

Say "usage: .\acceptance-35b.ps1 -Setup | -Check | -Clean"
Say ""
Say "  -Setup   build, sandbox, start the Director, and print the one thing you do"
Say "  -Check   read the store and report PASS/FAIL per claim"
Say "  -Clean   stop the Director and delete the sandbox"
exit 2
