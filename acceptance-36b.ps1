# Encoded UTF-8 WITH A BOM, deliberately. Windows PowerShell 5.1 decodes a BOM-less file as the
# system ANSI codepage, and an em dash then arrives as three characters -- one of which is a
# double quote. Inside a string that terminates it early, and the script fails to PARSE with an
# error pointing at a brace fifty lines away. Measured on acceptance-35c.ps1: it did exactly that.
# The BOM is the fix; keeping the prose ASCII below is the belt to its braces.
<#
.SYNOPSIS
  The live acceptance for Roadmap 36B: "learn what I just did".

.DESCRIPTION
  The claim is one sentence long and only a person can test it:

      marco observe
      [use your computer normally]
      marco learn "open mouse settings" --recent
      done

  No repeat demonstration. No naming any screen. No naming any button. No rehearsal. No input
  from Marco at any point during the learn.

  The Go suite proves the promotion, the licences, the selection rules and the refusals against
  a real store. What it cannot supply is a real Settings window, a real accessibility tree, and
  a real person clicking -- which is exactly where "the control had no name Marco could admit"
  and "the screen never settled" live.

      .\acceptance-36b.ps1 -Setup      build, sandbox, start a Director, start watching
      .\acceptance-36b.ps1 -Learn      after you have clicked: learn what you just did
      .\acceptance-36b.ps1 -Report     what was promoted, what was asked, what it cost
      .\acceptance-36b.ps1 -Restart    stop and restart the Director, then find the play
      .\acceptance-36b.ps1 -Clean      stop the Director and delete the sandbox

  YOUR REAL PLAYS AND MEMORY ARE NEVER WRITTEN TO. -Setup COPIES your semantic memory into a
  throwaway MARCO_HOME under TEMP and everything after that runs there. -Clean deletes it.

  NOTHING HERE DRIVES YOUR DESKTOP. Observe watches and Learn promotes what it watched; neither
  acts, and no part of this script performs a Play, presses a key or moves the mouse. That is
  not a courtesy -- it is the property under test, and -Report checks it rather than asserting
  it. The one step that WOULD drive the desktop is running the learned play, and this script
  deliberately stops before it: -Restart proves the play is discoverable and hands you the
  command to run yourself, when you are ready and watching.

.EXAMPLE
  .\acceptance-36b.ps1 -Setup
  # open Settings, click "Bluetooth & devices", then click "Mouse"
  .\acceptance-36b.ps1 -Learn
  .\acceptance-36b.ps1 -Report
  .\acceptance-36b.ps1 -Restart
#>
[CmdletBinding()]
param(
    [switch]$Setup,
    [switch]$Learn,
    [switch]$Report,
    [switch]$Restart,
    [switch]$Where,
    [switch]$Clean,
    [string]$Name = "open mouse settings",
    [string]$App = ""
)

$ErrorActionPreference = "Stop"
$Root    = $PSScriptRoot
$Sandbox = Join-Path $env:TEMP "marco-36b"
$Home36  = Join-Path $Sandbox "home"
$Routes  = Join-Path $Sandbox "routes"
$Store   = Join-Path $Home36 "semantic-memory.json"
$Marco   = Join-Path $Sandbox "marco.exe"
$Dir     = Join-Path $Sandbox "director.exe"
$Result  = Join-Path $Sandbox "learned.json"

function Use-Sandbox {
    $env:MARCO_HOME   = $Home36
    $env:MARCO_ROUTES = $Routes
    $env:DIRECTOR_BIN = $Dir
    $env:MARCO_BIN    = $Marco
}

function Say  ($m) { Write-Host $m }
function Step ($m) { Write-Host ""; Write-Host "== $m" -ForegroundColor Cyan }
function Good ($m) { Write-Host "   OK   $m" -ForegroundColor Green }
function Bad  ($m) { Write-Host "   BAD  $m" -ForegroundColor Red }
function Warn ($m) { Write-Host "   ??   $m" -ForegroundColor Yellow }
function Note ($m) { Write-Host "        $m" -ForegroundColor DarkGray }

# Num turns an absent JSON field into 0.
#
# Every count in these payloads is `omitempty`, so a zero is ABSENT rather than 0 and a bare
# arithmetic comparison against $null quietly reads as zero in one direction and errors in the
# other. Measured on acceptance-36a.ps1, which needed the same function for the same reason.
function Num ($v) { if ($null -eq $v) { 0 } else { [int]$v } }

# Stop-SandboxDirector ends the sandbox's Director and nothing else.
#
# Matched by PATH, not by name: the developer's REAL Director is very likely running from the
# repository at the same time, and killing it would be this script reaching outside its sandbox
# to do the one thing it promises not to do.
function Stop-SandboxDirector {
    Get-Process -Name director -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -eq $Dir } |
        ForEach-Object {
            try { $_.Kill() } catch {}
            try { $_.WaitForExit(5000) | Out-Null } catch {}
        }
    # The file handle outlives the process by a moment. Waiting for it means a following
    # -Setup can overwrite the binary instead of failing with a sharing violation.
    for ($i = 0; $i -lt 20; $i++) {
        if (-not (Test-Path $Dir)) { break }
        try { [IO.File]::Open($Dir, "Open", "Write").Close(); break } catch { Start-Sleep -Milliseconds 100 }
    }
}

# Real-Store finds the semantic memory this machine actually uses, so -Setup can copy it.
function Real-Store {
    if ($env:MARCO_MEMORY) { return $env:MARCO_MEMORY }
    if ($env:MARCO_HOME)   { return (Join-Path $env:MARCO_HOME "semantic-memory.json") }
    return (Join-Path $env:APPDATA "marco\semantic-memory.json")
}

function Observe-Status {
    Use-Sandbox
    $raw = & $Marco observe status --json 2>$null
    if (-not $raw) { return $null }
    try { return ($raw | ConvertFrom-Json) } catch { return $null }
}

# ── where ─────────────────────────────────────────────────────────────────────

if ($Where) {
    Say "sandbox : $Sandbox"
    Say "home    : $Home36"
    Say "routes  : $Routes"
    Say "store   : $Store"
    Say "result  : $Result"
    Say "real    : $(Real-Store)   (copied in by -Setup, never written to)"
    return
}

# ── clean ─────────────────────────────────────────────────────────────────────

if ($Clean) {
    Step "Cleaning up"
    Stop-SandboxDirector
    if (Test-Path $Sandbox) { Remove-Item -Recurse -Force $Sandbox }
    Good "sandbox removed"
    return
}

# ── setup ─────────────────────────────────────────────────────────────────────

if ($Setup) {
    Step "Building"
    Stop-SandboxDirector
    New-Item -ItemType Directory -Force $Home36  | Out-Null
    New-Item -ItemType Directory -Force $Routes  | Out-Null
    Push-Location $Root
    try {
        & go build -o $Marco ./cmd/marco
        if ($LASTEXITCODE -ne 0) { throw "building marco.exe failed" }
        & go build -o $Dir ./cmd/director
        if ($LASTEXITCODE -ne 0) { throw "building director.exe failed" }
    } finally { Pop-Location }
    Good "marco.exe and director.exe built into the sandbox"

    $uia = Join-Path $Root "plugins\uia\uia.exe"
    if (Test-Path $uia) {
        Good "accessibility bridge present"
    } else {
        Warn "plugins\uia\uia.exe is missing"
        Note "Without it Marco reads no accessibility tree, so it can see neither the"
        Note "screens you move between nor the control you press. This run will refuse"
        Note "honestly and tell you nothing about 36B."
    }

    Step "Sandboxing your memory"
    $real = Real-Store
    if (Test-Path $real) {
        Copy-Item $real $Store -Force
        $md5 = (Get-FileHash $Store -Algorithm MD5).Hash
        Good "copied $real"
        Note "sandbox copy md5 $md5"
        Set-Content -Path (Join-Path $Sandbox "store.md5") -Value $md5 -Encoding utf8
    } else {
        Warn "no semantic memory found at $real -- starting cold"
        Note "That is a fine test and a harder one: every screen you walk through will be"
        Note "one Marco has never seen, which is exactly the case 36B had to make work."
    }

    Step "Starting a Director in the sandbox"
    Use-Sandbox
    Stop-SandboxDirector
    & $Marco observe | Out-Null
    Start-Sleep -Seconds 2
    $st = Observe-Status
    if (-not $st -or -not $st.watching) {
        Bad "watching did not start"
        return
    }
    Good "Marco is watching, in the sandbox"

    Step "Now use your computer"
    Say  "  1. Open Settings."
    Say  "  2. Click 'Bluetooth & devices'."
    Say  "  3. Click 'Mouse'."
    Say  ""
    Say  "  Then run:  .\acceptance-36b.ps1 -Learn"
    Say  ""
    Note "Take your time. Nothing here is on a timer: the trail is bounded by SIZE and"
    Note "not by the clock, so a slow page or a pause costs nothing."
    return
}

# ── learn ─────────────────────────────────────────────────────────────────────

if ($Learn) {
    Use-Sandbox
    $before = Observe-Status
    if (-not $before -or -not $before.watching) {
        Bad "Marco is not watching -- run -Setup first"
        return
    }
    Step "What watching has noticed"
    Say  "  screens : $(Num $before.places)"
    Say  "  moves   : $(Num $before.transitions)"
    if ((Num $before.transitions) -eq 0) {
        Warn "no moves noticed, so there is nothing to learn"
        Note "Marco records a move only between two readings it could place. If you"
        Note "clicked and this is zero, the accessibility reading is the thing to look at."
    }

    Step "Learning what you just did"
    $args = @("learn", $Name, "--recent")
    if ($App) { $args += @("--application", $App) }
    $out = & $Dir @args --json 2>&1
    $out | Out-String | Set-Content -Path $Result -Encoding utf8
    try { $v = ($out | ConvertFrom-Json) } catch { $v = $null }
    if (-not $v) {
        Bad "the Director's reply was unreadable"
        Say ($out | Out-String)
        return
    }
    Say ""
    Say "  $($v.saying)"
    Say ""
    if ($v.learned) {
        Good "learned as $($v.play)"
    } else {
        Warn "nothing was learned: $($v.refused)"
    }
    Say ""
    Say "  Then run:  .\acceptance-36b.ps1 -Report"
    return
}

# ── report ────────────────────────────────────────────────────────────────────

if ($Report) {
    Use-Sandbox
    if (-not (Test-Path $Result)) {
        Bad "no learn has run -- do -Setup, click, then -Learn"
        return
    }
    $v = (Get-Content $Result -Raw | ConvertFrom-Json)
    $r = $v.recent

    Step "What was selected"
    Say  "  outcome        : $($r.outcome)"
    if ($r.why)  { Say "  shortfall      : $($r.why)" }
    Say  "  application    : $($r.application)"
    Say  "  steps          : $(Num $r.steps)"
    Say  "  considered     : $(Num $r.considered)  (steps of the trail the walk looked at)"

    Step "What was promoted"
    Say  "  places made durable : $(Num $r.places_established)"
    Say  "  controls remembered : $(Num $r.targets_remembered)"
    Say  "  play                : $($v.play)"
    Say  "  registered          : $($v.registered)"

    Step "What was NOT asked"
    # The clean path asks nothing. A naming question would arrive as a pending proposal, and
    # a rehearsal offer as a phase this view would still be sitting in.
    if ($v.settled) { Good "the learn finished in one call -- nothing is waiting on you" }
    else            { Bad  "the learn is still waiting: $($v.phase)" }
    if ($v.question_id) { Bad "a question was raised: $($v.question_id)" }
    else                { Good "no question was raised" }

    Step "What it cost your desktop"
    # THE PROPERTY, checked rather than asserted. Learning is a read of evidence Marco already
    # had; if anything here drove the desktop it went through the performance slot, and the
    # Director's own status is where that shows.
    $raw = & $Dir status --json 2>$null
    try { $st = ($raw | ConvertFrom-Json) } catch { $st = $null }
    if ($st -and $st.active) {
        Bad "a command is running after a learn that should have performed nothing:"
        Say ($st.active | Out-String)
    } else {
        Good "no command ran -- nothing was performed"
    }

    Step "Watching continues"
    $now = Observe-Status
    if ($now -and $now.watching) {
        Good "still watching ($(Num $now.places) screens, $(Num $now.transitions) moves)"
    } else {
        Bad "watching stopped when it was learned from"
    }

    Step "Your real memory"
    $expected = Join-Path $Sandbox "store.md5"
    if (Test-Path $expected) {
        $want = (Get-Content $expected -Raw).Trim()
        $got  = (Get-FileHash $Store -Algorithm MD5).Hash
        if ($got -eq $want) {
            Warn "the sandbox store is UNCHANGED"
            Note "That is wrong for this run: a learn writes places, edges and a goal, so"
            Note "an unchanged store means nothing became durable."
        } else {
            Good "the sandbox store changed, which is what learning means"
        }
    }
    $real = Real-Store
    if (Test-Path $real) {
        Note "your real store is at $real and this script never opens it for writing"
    }
    Say ""
    Say "  Then run:  .\acceptance-36b.ps1 -Restart"
    return
}

# ── restart ───────────────────────────────────────────────────────────────────

if ($Restart) {
    Use-Sandbox
    Step "Stopping the Director"
    Stop-SandboxDirector
    Good "stopped -- everything watching held is gone, deliberately"

    Step "Is the play still there"
    # A COLD FIND. Nothing is running, no session exists, and the trail that produced this is
    # forgotten. What is left is the durable artifact, found by the ordinary resolver.
    $found = & $Marco routes 2>&1 | Out-String
    if ($found -match [regex]::Escape($Name)) {
        Good "the play survived the restart and is discoverable by the words you used"
    } else {
        Bad "the play is not discoverable after a restart"
        Say $found
    }

    Step "Running it is YOUR step, not this script's"
    Say  "  Nothing in this file drives your desktop. When you are ready and watching:"
    Say  ""
    Say  "      `$env:MARCO_HOME='$Home36'; `$env:MARCO_ROUTES='$Routes'"
    Say  "      $Marco do `"$Name`""
    Say  ""
    Note "That goes through the ordinary door: its own authority, the foreground check, the"
    Note "production lease and the shared walker. There is no special path for a play that"
    Note "was learned this way, which is the point."
    return
}

Say "acceptance-36b.ps1 -- the live acceptance for `"learn what I just did`""
Say ""
Say "  -Setup     build, sandbox, start a Director, start watching"
Say "  -Learn     after you have clicked: learn what you just did"
Say "  -Report    what was promoted, what was asked, what it cost"
Say "  -Restart   stop and restart, then find the play"
Say "  -Where     print the sandbox paths"
Say "  -Clean     stop the Director and delete the sandbox"
Say ""
Say "  Get-Help .\acceptance-36b.ps1 -Detailed   for what this is actually testing"
