# Encoded UTF-8 WITH A BOM, deliberately. Windows PowerShell 5.1 decodes a BOM-less file as the
# system ANSI codepage, and an em dash then arrives as three characters -- one of which is a
# double quote. Inside a string that terminates it early, and the script fails to PARSE with an
# error pointing at a brace fifty lines away. Measured on acceptance-35c.ps1: it did exactly that.
# The BOM is the fix; keeping the prose ASCII below is the belt to its braces.
<#
.SYNOPSIS
  The live acceptance for Roadmap 36C: watching becoming graph knowledge.

.DESCRIPTION
  The claim only a person can test:

      marco observe
      marco observe learn
      [walk from one screen to another, once]
      Marco knows that way between them

  Nobody typed Learn. Nobody was asked anything. Nothing was invented that they did not do, and
  nobody had to do it twice to prove that a door they had just walked through was still a door.

  What Marco learns is a GRAPH EDGE, not a recording of a workflow. So the second round of this
  acceptance is not "now it counts" -- it is "the edges you walked on two different occasions
  compose into a route you never walked as a whole".

  The Go suite proves the policy, the ledger, the licences, the bounds and the refusals against a
  real store. What it cannot supply is a real Settings window, a real accessibility tree, and a
  real person clicking -- which is where "that control had no name Marco could admit" and "the
  same screen read two ways" actually live.

      .\acceptance-36c.ps1 -Setup        build, sandbox, start a Director, watch AND learn
      .\acceptance-36c.ps1 -Round        after you have clicked: what has been noticed
      .\acceptance-36c.ps1 -Report       candidates, promotions, and what it cost
      .\acceptance-36c.ps1 -Restart      stop and restart; is the knowledge still there
      .\acceptance-36c.ps1 -Tail         LIVE: what Marco is watching, right now
      .\acceptance-36c.ps1 -Why          why the numbers are what they are
      .\acceptance-36c.ps1 -Clean        stop the Director and delete the sandbox

  ONE CLEAN CROSSING IS ENOUGH. Round 1 should already show knowledge on disk; if it does not,
  something is wrong and -Report says which condition Marco is waiting on. Round 2 walks a
  DIFFERENT way out of a screen you have already been to, which is what shows the graph
  composing rather than a workflow repeating.

  YOUR REAL PLAYS AND MEMORY ARE NEVER WRITTEN TO. -Setup COPIES your semantic memory into a
  throwaway MARCO_HOME under TEMP and everything after that runs there. -Clean deletes it.

  NOTHING HERE DRIVES YOUR DESKTOP. Observe watches, and promotion is a memory operation: no
  input, no desktop lease, no authority, no rehearsal. That is not a courtesy -- it is the
  property under test, and -Report checks it rather than asserting it.

.EXAMPLE
  .\acceptance-36c.ps1 -Setup
  # open Settings, click "Bluetooth & devices", then click "Mouse"
  .\acceptance-36c.ps1 -Round
  # go back to Home and click something ELSE -- "Network & internet", say
  .\acceptance-36c.ps1 -Round
  .\acceptance-36c.ps1 -Report
  .\acceptance-36c.ps1 -Restart
#>
[CmdletBinding()]
param(
    [switch]$Setup,
    [switch]$Round,
    [switch]$Report,
    [switch]$Restart,
    [switch]$Tail,
    [switch]$Why,
    [switch]$Where,
    [switch]$Clean,
    [int]$EveryMs = 500,
    [string]$App = ""
)

$ErrorActionPreference = "Stop"
$Root    = $PSScriptRoot
$Sandbox = Join-Path $env:TEMP "marco-36c"
$Home36  = Join-Path $Sandbox "home"
$Routes  = Join-Path $Sandbox "routes"
$Store   = Join-Path $Home36 "semantic-memory.json"
$Marco   = Join-Path $Sandbox "marco.exe"
$Dir     = Join-Path $Sandbox "director.exe"
$Rounds  = Join-Path $Sandbox "rounds.jsonl"
# THE STORE AS IT WAS BEFORE ANY OF THIS. -Setup copies a real semantic memory in, so
# what is on disk at round 1 is not what round 1 learned. Without this the harness
# would credit ambient watching with everything the person had ever taught Marco.
$Baseline = Join-Path $Sandbox "baseline.json"

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

# Num turns an absent JSON field into 0. Every count in these payloads is `omitempty`, so a zero
# is ABSENT rather than 0 -- the same function acceptance-36a.ps1 needed for the same reason.
function Num ($v) { if ($null -eq $v) { 0 } else { [int]$v } }

# Stop-SandboxDirector ends the sandbox's Director and nothing else.
#
# Matched by PATH, not by name: the developer's REAL Director is very likely running from the
# repository at the same time, and killing it would be this script reaching outside its sandbox to
# do the one thing it promises not to do.
function Stop-SandboxDirector {
    Get-Process -Name director -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -eq $Dir } |
        ForEach-Object {
            try { $_.Kill() } catch {}
            try { $_.WaitForExit(5000) | Out-Null } catch {}
        }
    for ($i = 0; $i -lt 20; $i++) {
        if (-not (Test-Path $Dir)) { break }
        try { [IO.File]::Open($Dir, "Open", "Write").Close(); break } catch { Start-Sleep -Milliseconds 100 }
    }
}

# RealStore is captured ONCE, at the top of the script, before anything is sandboxed.
#
# # The bug this exists to prevent, and it silently changed what was being tested
#
# `Use-Sandbox` sets $env:MARCO_HOME at PROCESS scope, and PowerShell keeps process-scope
# environment variables for the whole terminal session. So the second invocation of this script in
# the same window inherited MARCO_HOME pointing at the sandbox — and a Real-Store that read
# MARCO_HOME then returned the SANDBOX path.
#
# Measured: a `-Clean` followed by `-Setup` in one terminal looked for the person's real memory at
# the sandbox path it had just deleted, found nothing, said "starting cold", and ran the whole
# acceptance against an empty store. Then `-Report` told them "your real store is at
# <temp>\marco-36c\..." — which is not their real store and would be alarming if they believed it.
#
# So it is resolved once, before Use-Sandbox can ever have run, and pinned in the sandbox so later
# invocations agree with the first.
$RealStorePin = Join-Path $Sandbox "real-store.txt"
function Real-Store {
    if (Test-Path $RealStorePin) { return (Get-Content $RealStorePin -Raw).Trim() }
    if ($env:MARCO_MEMORY) { return $env:MARCO_MEMORY }
    # MARCO_HOME is read only when it is NOT this script's own sandbox. A terminal that has run
    # -Setup already has it set, and following it would be following ourselves.
    if ($env:MARCO_HOME -and $env:MARCO_HOME -ne $Home36) {
        return (Join-Path $env:MARCO_HOME "semantic-memory.json")
    }
    return (Join-Path $env:APPDATA "marco\semantic-memory.json")
}

function Observe-Status {
    Use-Sandbox
    $raw = & $Marco observe status --json 2>$null
    if (-not $raw) { return $null }
    try { return ($raw | ConvertFrom-Json) } catch { return $null }
}

# Explain-Round says WHY the numbers are what they are.
#
# "0 moves noticed" has at least five explanations, and they send somebody to five different
# places:
#
#     nothing ever started a session      -> the window could not be acquired
#     sessions ran and read nothing       -> the accessibility bridge
#     the page could not be read          -> perception degraded, shell only
#     the page was read and not known     -> ordinary, and not a fault
#     screens were known and never changed-> you did not move, or the pin is on one window
#
# Every one of these is already on the status payload. The first version of this harness printed
# five counts and none of the reasons, which is the same "one sentence for four problems" failure
# the Director itself was fixed for twice. See ADR-090's note on why a silence must say which.
function Explain-Round ($st) {
    $sessions = Num $st.sessions
    $samples  = Num $st.samples
    $places   = Num $st.places
    $moves    = Num $st.transitions

    if (-not $st.watching) {
        Bad "Marco is not watching at all"
        return
    }
    if ($sessions -eq 0) {
        Bad "no observation session has started"
        Note "The supervisor asks for one every second or so and something is refusing."
        Note "Either the foreground window could not be acquired, or another observation"
        Note "session owns the substrate -- ambient watching always yields to Learn, Here"
        Note "and a performance. Check: $Dir status"
        return
    }
    if ($samples -eq 0) {
        Bad "$sessions session(s) ran and took no readings"
        Note "The session started and the sampler produced nothing. That is perception,"
        Note "not the observer. Check: $Dir light"
        return
    }
    Good "$sessions session(s), $samples reading(s) of the desktop"

    if ($st.perception_degraded) {
        Bad "the last reading got no further than the window frame"
        Note "Marco can see the window and cannot read the page inside it. That is the"
        Note "accessibility bridge or a window that exposes no tree, and NOTHING here can"
        Note "be learned until it is fixed -- a degraded reading is deliberately not a"
        Note "screen, so it cannot become a place or an endpoint."
        return
    }
    if ($st.application) {
        Note "watching: $($st.application)"
    } else {
        Warn "no application on the last reading -- nothing was in front, or it could not be named"
    }

    if ($places -eq 0 -and $moves -eq 0) {
        Warn "readings happened and no screen was RECOGNISED"
        Note "That is ordinary on software Marco has not seen before, and it does not stop"
        Note "ambient learning: an unrecognised screen it can DESCRIBE is still an endpoint."
        Note "Zero moves beside it means the screen never changed -- see below."
    }
    if ($moves -eq 0) {
        Warn "no move was recorded"
        Note "A move is recorded when two consecutive readings place you somewhere"
        Note "DIFFERENT, in the same application. Zero after clicking through Settings"
        Note "means one of: the readings never resolved to distinguishable screens, the"
        Note "session is pinned to a window you have left, or the click changed the page"
        Note "faster than a reading could settle on either side of it."
    } else {
        Good "$moves move(s) recorded across $places recognised screen(s)"
    }
    if ((Num $st.noticed) -eq 0 -and $moves -gt 0) {
        Warn "moves were recorded and no relationship was noticed"
        Note "A move becomes candidate evidence only when Marco also saw WHAT you pressed,"
        Note "and could name it. A press on a control whose name the privacy allowlist"
        Note "withholds is the commonest reason -- that is the boundary working, not"
        Note "perception failing."
    }
    if ((Num $st.noticed) -gt 0 -and (Num $st.learned) -eq 0) {
        Warn "relationships were noticed and none became knowledge"
        Note "One clean traversal is meant to be enough, so this is a real result rather"
        Note "than something to wait out. Run -Report: every candidate names the one"
        Note "condition it is failing, in Marco's own words."
    }
}

# Store-Shape counts what is durable, so two rounds can be compared.
#
# It reads the JSON rather than asking the Director, deliberately: the question is what is ON DISK,
# and a Director reporting on its own memory is not independent evidence about it.
function Store-Shape {
    if (-not (Test-Path $Store)) {
        return [pscustomobject]@{ subjects = 0; relationships = 0; watched = 0; promoted = 0; goals = 0 }
    }
    $f = Get-Content $Store -Raw | ConvertFrom-Json
    $promoted = 0
    foreach ($w in @($f.watched)) { if ($w.promoted) { $promoted++ } }
    [pscustomobject]@{
        subjects      = @($f.subjects).Count
        relationships = @($f.relationships).Count
        watched       = @($f.watched).Count
        promoted      = $promoted
        goals         = @($f.goals).Count
    }
}

# ── tail ──────────────────────────────────────────────────────────────────────

# -Tail is a live view of what Marco is watching.
#
# # Why this exists, and why it should have existed first
#
# Every other mode here is a snapshot taken afterwards. If something goes wrong while you are
# clicking, a snapshot gives you one number at the end and no way to know WHEN it went wrong or
# what Marco was looking at while it did. The first live run of ambient watching failed for a
# reason that would have been obvious in one line of this -- the application column would have read
# `powershell` the entire time -- and instead it took a round trip and a guess.
#
# Run it in a second terminal and leave it there while you do the route.
#
# It prints ONLY when something changes, so a quiet desktop is a quiet screen and a change is
# something you can see happen. Nothing here writes anything or asks the Director to do anything:
# it is the same status read `marco observe status` makes, on a loop.
if ($Tail) {
    Use-Sandbox
    Say "Watching what Marco is watching. Ctrl-C to stop."
    Say ""
    Say ("{0,-8}  {1,-22} {2,-11} {3,-9}  {4}" -f "time", "application", "page", "screen", "counts")
    $last = ""
    while ($true) {
        $st = Observe-Status
        if (-not $st) {
            $line = "no Director is running"
        } elseif (-not $st.watching) {
            $line = "not watching"
        } else {
            # THE APPLICATION IS THE FIRST COLUMN, deliberately. "Which window is Marco
            # actually reading" is the question that was unanswerable, and it is the one
            # that turned out to matter.
            $page = if ($st.perception_degraded) { "unreadable" } else { "readable" }
            $screen = if ($st.place) { "known" } else { "new" }
            $counts = "{0} screens, {1} moves, {2} seen, {3} learned" -f
                      (Num $st.places), (Num $st.transitions), (Num $st.noticed), (Num $st.learned)
            $app = if ($st.application) { $st.application } else { "(nothing in front)" }
            $line = "{0,-22} {1,-11} {2,-9}  {3}" -f $app, $page, $screen, $counts
        }
        if ($line -ne $last) {
            $colour = "Gray"
            if ($line -match "unreadable")  { $colour = "Yellow" }
            if ($line -match "not watching|no Director") { $colour = "Red" }
            Write-Host ("{0,-8}  {1}" -f (Get-Date -Format "HH:mm:ss"), $line) -ForegroundColor $colour
            $last = $line
        }
        Start-Sleep -Milliseconds $EveryMs
    }
}

# ── why ───────────────────────────────────────────────────────────────────────

# -Why prints everything the Director already knows, unsummarised.
#
# The counts in -Round are a product answer. This is the diagnostic one: the whole ambient status
# payload, the Director's own account of itself, and what perception last managed. Nothing here is
# computed by the harness, because a harness that derived its own answer would be a second opinion
# about a system that already has one.
if ($Why) {
    Use-Sandbox
    Step "Is anything watching"
    $raw = & $Marco observe status --json 2>&1
    Say ($raw | Out-String)

    Step "What the Director says it is doing"
    $ds = & $Dir status 2>&1
    Say ($ds | Out-String)
    Note "Watching and Learning are two lines and they are answered separately. An active"
    Note "command here means something else owns the substrate and ambient watching is"
    Note "yielding to it, which is correct behaviour and would explain zero sessions."

    Step "What perception can currently read"
    # `director light` is the read-only account of the window in front: whether it could be
    # acquired, how far into it the reading got, and what it resolved to. It is the one
    # command that separates "no window" from "a window nothing can read".
    $light = & $Dir light 2>&1
    Say ($light | Out-String)

    Step "What Marco is waiting for"
    $seen = & $Marco observe status --evidence 2>&1
    Say ($seen | Out-String)

    Step "The durable record"
    if (Test-Path $Store) {
        $f = Get-Content $Store -Raw | ConvertFrom-Json
        Say ("subjects {0}, relationships {1}, candidates {2}, goals {3}" -f
             @($f.subjects).Count, @($f.relationships).Count,
             @($f.watched).Count, @($f.goals).Count)
        # A STORE THAT CONTRADICTS ITSELF SAYS SO.
        #
        # An edge or a goal needs its endpoints to exist. The loader drops orphans, so a
        # file holding relationships with no subjects will silently lose them the next time
        # it is opened — and the counts above would read as though the knowledge were there.
        if (@($f.subjects).Count -eq 0 -and
            (@($f.relationships).Count -gt 0 -or @($f.goals).Count -gt 0)) {
            Bad "this store has relationships or goals and NO subjects"
            Note "Both need endpoints that exist. The loader drops orphans, so these are"
            Note "already gone as far as any running Director is concerned -- the file is"
            Note "reporting knowledge that no longer resolves to anything."
        }
        foreach ($w in @($f.watched)) {
            Say ("  candidate {0}  traversed {1} / in {2} session(s) / contradicted {3}  {4} -> {5}  [{6}]" -f
                 $w.id, (Num $w.seen), (Num $w.sessions), (Num $w.contradicted),
                 $(if ($w.from.subject) { $w.from.subject } else { "(unrecognised)" }),
                 $(if ($w.to.subject) { $w.to.subject } else { "(unrecognised)" }),
                 $(if ($w.promoted) { "promoted" } else { "pending" }))
        }
    } else {
        Warn "no store at $Store"
    }
    return
}

# ── where ─────────────────────────────────────────────────────────────────────

if ($Where) {
    Say "sandbox : $Sandbox"
    Say "home    : $Home36"
    Say "routes  : $Routes"
    Say "store   : $Store"
    Say "rounds  : $Rounds"
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
    New-Item -ItemType Directory -Force $Home36 | Out-Null
    New-Item -ItemType Directory -Force $Routes | Out-Null
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
        Note "Without it Marco reads no accessibility tree, so it can see neither the screens"
        Note "you move between nor the control you press. This run will notice nothing and"
        Note "tell you nothing about 36C."
    }

    Step "Sandboxing your memory"
    $real = Real-Store
    Set-Content -Path $RealStorePin -Value $real -Encoding utf8
    if (Test-Path $real) {
        Copy-Item $real $Store -Force
        Good "copied $real"
    } else {
        Warn "no semantic memory found at $real -- starting cold"
        Note "A fine test and a harder one: every screen you walk through will be one Marco"
        Note "has never seen, which is exactly the case ambient learning had to make work."
    }
    Remove-Item $Rounds -ErrorAction SilentlyContinue
    (Store-Shape | ConvertTo-Json -Compress) |
        Set-Content -Path $Baseline -Encoding utf8

    Step "Starting a Director, watching AND learning"
    Use-Sandbox
    Stop-SandboxDirector
    # TWO SWITCHES, and this is the one place the script uses both. `marco observe` alone
    # watches and remembers nothing; the second command is the one that makes memory durable.
    & $Marco observe | Out-Null
    Start-Sleep -Seconds 2
    $before = Observe-Status
    if ($before -and $before.learning) {
        Bad "watching turned learning on with it -- the two lifecycles are not separate"
        return
    }
    Good "watching, and not yet learning"
    & $Marco observe learn | Out-Null
    Start-Sleep -Seconds 1
    $st = Observe-Status
    if (-not $st -or -not $st.watching -or -not $st.learning) {
        Bad "watching and learning did not both start"
        return
    }
    Good "Marco is watching, and may remember what recurs"

    Step "First: open a second terminal and leave this running in it"
    Say  ""
    Say  "      .\acceptance-36c.ps1 -Tail"
    Say  ""
    Note "It shows which window Marco is reading, whether it can read the page, and the"
    Note "counts, live. Without it the only evidence you get is a number afterwards, and"
    Note "a number afterwards cannot tell you WHEN something went wrong or what Marco was"
    Note "looking at while it did."

    Step "Then use your computer, normally"
    Say  "  1. Open Settings."
    Say  "  2. Click 'Bluetooth & devices'."
    Say  "  3. Click 'Mouse'."
    Say  ""
    Say  "  Then run:  .\acceptance-36c.ps1 -Round"
    Say  ""
    Note "NORMALLY means normally. Do not pause after switching windows to let Marco catch"
    Note "up -- if it needs that, it is not ambient and the run should say so. Watching"
    Note "follows the window in front within about a tenth of a second."
    Note ""
    Note ""
    Note "One clean crossing is enough. You are not proving a habit -- you are showing Marco"
    Note "what a control does, and it only has to see that once."
    Note ""
    Note "After -Round, go back to Home and click something ELSE. Two ways out of one screen,"
    Note "learned on two occasions, is what a graph turns into a route and a recording cannot."
    return
}

# ── round ─────────────────────────────────────────────────────────────────────

if ($Round) {
    Use-Sandbox
    $st = Observe-Status
    if (-not $st -or -not $st.watching) {
        Bad "Marco is not watching -- run -Setup first"
        return
    }
    $shape = Store-Shape
    $n = 1
    if (Test-Path $Rounds) { $n = (@(Get-Content $Rounds)).Count + 1 }

    Step "Round $n"
    Say  "  screens noticed   : $(Num $st.places)"
    Say  "  moves noticed     : $(Num $st.transitions)"
    Say  "  relationships seen: $(Num $st.noticed)"
    Say  "  candidates held   : $(Num $st.candidates)"
    Say  "  remembered so far : $(Num $st.learned)"
    Say  ""
    Say  ("  on disk: {0} subjects, {1} relationships, {2} candidates ({3} promoted), {4} goals" -f
          $shape.subjects, $shape.relationships, $shape.watched, $shape.promoted, $shape.goals)

    # WHY, not just how much.
    #
    # "0 moves" has five explanations that send somebody to five different places, and the first
    # version of this printed one sentence for all of them -- the exact shape of failure this
    # repository has paid for repeatedly, reproduced in the harness meant to catch it. Every
    # field below is already on the status payload; it was simply not being read.
    Explain-Round $st
    if ((Num $st.transitions) -eq 0) {
        Note "For the full picture: .\acceptance-36c.ps1 -Why"
    }
    # ONE CLEAN CROSSING IS ENOUGH, so round 1 producing knowledge is the pass and round 1
    # producing none is the thing that needs explaining. This assertion used to be the other
    # way up -- it WARNED when something became durable after one round, because the policy
    # then wanted two. That was the workflow model, and it is what 36C.1 corrected.
    if ($n -eq 1) {
        $base = if (Test-Path $Baseline) {
            Get-Content $Baseline -Raw | ConvertFrom-Json
        } else { $shape }
        if ($shape.relationships -gt $base.relationships) {
            Good ("{0} relationship(s) became knowledge on the FIRST clean crossing" -f
                  ($shape.relationships - $base.relationships))
        } elseif ((Num $st.transitions) -gt 0) {
            Warn "you moved between screens and nothing became knowledge"
            Note "One clean traversal is meant to be enough, so this is a real result and"
            Note "not patience. Run -Report: Marco says which condition it is waiting on."
        }
    }

    $row = [pscustomobject]@{
        round = $n; at = (Get-Date).ToString("o")
        places = (Num $st.places); transitions = (Num $st.transitions)
        noticed = (Num $st.noticed); learned = (Num $st.learned)
        candidates = (Num $st.candidates)
        subjects = $shape.subjects; relationships = $shape.relationships
        watched = $shape.watched; promoted = $shape.promoted; goals = $shape.goals
    }
    Add-Content -Path $Rounds -Value ($row | ConvertTo-Json -Compress) -Encoding utf8
    Say ""
    if ($n -lt 2) {
        Say "  Now go back and take a DIFFERENT way out of a screen you have already seen,"
        Say "  then run:  .\acceptance-36c.ps1 -Round"
    } else {
        Say "  Then run:  .\acceptance-36c.ps1 -Report"
    }
    return
}

# ── report ────────────────────────────────────────────────────────────────────

if ($Report) {
    Use-Sandbox
    if (-not (Test-Path $Rounds)) {
        Bad "no rounds recorded -- do -Setup, click, then -Round"
        return
    }
    $rows = @(Get-Content $Rounds | ForEach-Object { $_ | ConvertFrom-Json })
    Step "What each round saw"
    $rows | Format-Table round, transitions, noticed, candidates, learned,
        subjects, relationships, promoted, goals -AutoSize | Out-String | Write-Host

    if ($rows.Count -lt 2) {
        Note "one round -- enough to judge the headline claim, which is that a single clean"
        Note "crossing becomes knowledge. A second round is what shows edges COMPOSING, so"
        Note "take a different way out of a screen you have seen and run -Round again."
    }
    $first = $rows[0]
    $last  = $rows[-1]

    Step "Did watching become knowledge"
    if ($last.promoted -gt 0) {
        Good "$($last.promoted) relationship(s) Marco learned by watching, with nobody teaching it"
    } elseif ($last.candidates -gt 0) {
        Warn "evidence accumulated and nothing was promoted"
        Note "Every candidate names its own reason. The next section asks Marco for it."
    } else {
        Bad "no candidate evidence at all -- nothing was noticed to learn from"
    }
    if ($rows.Count -ge 2 -and $last.relationships -gt $first.relationships) {
        Good ("the graph grew by {0} edge(s) across the rounds" -f
              ($last.relationships - $first.relationships))
        Note "Edges learned on separate occasions, and a route through them is one nobody"
        Note "demonstrated as a whole. That is the difference between a graph and a recording."
    }

    # AND MARCO SAYS WHY, in its own words.
    #
    # The first version of this said "the policy will say why ... read the candidate rows in
    # <file>" -- which is the harness telling somebody to go and be the diagnostic itself, the
    # same cop-out one layer along. The policy has the sentences; this asks for them.
    Step "What Marco is waiting for"
    $seen = & $Marco observe status --evidence 2>&1
    Say ($seen | Out-String)

    Step "What it did NOT do"
    if ($last.goals -eq $first.goals) {
        Good "no goal was invented from anonymous repeated behaviour"
    } else {
        Bad "$($last.goals - $first.goals) goal(s) appeared; ambient promotion must name nothing"
    }
    $raw = & $Dir status --json 2>$null
    try { $ds = ($raw | ConvertFrom-Json) } catch { $ds = $null }
    if ($ds -and $ds.active) {
        Bad "a command is running after promotion, which performs nothing:"
        Say ($ds.active | Out-String)
    } else {
        Good "no command ran -- nothing was performed and nothing drove your desktop"
    }

    Step "What the durable record holds"
    # THE PRIVACY CHECK, over the file rather than over a report about it. Candidate evidence
    # is the first thing in the ambient path that survives a restart, so this is where a
    # boundary would be easiest to lose.
    if (Test-Path $Store) {
        $text = Get-Content $Store -Raw
        $bad = @()
        foreach ($word in @("screenshot", "png", "base64", "clipboard", "keystroke", "password")) {
            if ($text -match $word) { $bad += $word }
        }
        if ($bad.Count -gt 0) {
            Bad "the store mentions: $($bad -join ', ')"
        } else {
            Good "no screenshot, no transcript, no clipboard, no keystroke, no secret"
        }
        Note "Candidate evidence is a summary: counts, times, structure, and the two words"
        Note "the interface put on things. Read $Store yourself -- it is meant to be readable."
    }

    Step "Your real memory"
    $real = Real-Store
    if (Test-Path $real) {
        Note "your real store is at $real and this script never opens it for writing"
    }
    Say ""
    Say "  Then run:  .\acceptance-36c.ps1 -Restart"
    return
}

# ── restart ───────────────────────────────────────────────────────────────────

if ($Restart) {
    Use-Sandbox
    $before = Store-Shape
    Step "Stopping the Director"
    Stop-SandboxDirector
    Good "stopped -- the transient trail is gone, deliberately"

    Step "Is the knowledge still there"
    $after = Store-Shape
    if ($after.relationships -eq $before.relationships -and $after.subjects -eq $before.subjects) {
        Good "$($after.relationships) relationship(s) and $($after.subjects) subject(s) survived"
    } else {
        Bad "the durable record changed across a restart: $($before.relationships) -> $($after.relationships) relationships"
    }
    if ($after.watched -eq $before.watched) {
        Good "$($after.watched) candidate summaries survived -- provenance for what was learned"
    } else {
        Bad "candidate evidence changed across a restart"
    }

    Step "And learning is OFF again"
    # NOT A SETTING. Learning does not survive a restart, deliberately: a durable toggle that
    # makes Marco build permanent memory from a desktop is a consent conversation, and
    # inventing one by implication is what ADR-093 refused for watching and ADR-095 refuses
    # here. Starting a Director and finding it already learning would be exactly that.
    & $Marco observe | Out-Null
    Start-Sleep -Seconds 2
    $st = Observe-Status
    if ($st -and $st.learning) {
        Bad "a fresh Director came up already learning -- that is a setting nobody agreed to"
    } else {
        Good "a fresh Director watches and does not learn until asked"
    }
    Say ""
    Note "Nothing in this file drives your desktop, and nothing here asks Marco to perform"
    Note "what it learned. Running a play is the ordinary door: its own authority, the"
    Note "foreground check, the production lease and the shared walker."
    return
}

Say "acceptance-36c.ps1 -- the live acceptance for watching becoming graph knowledge"
Say ""
Say "  -Setup     build, sandbox, start a Director, watch AND learn"
Say "  -Round     after you have clicked: what has been noticed"
Say "  -Report    candidates, promotions, and what it cost"
Say "  -Restart   stop and restart; is the knowledge still there"
Say "  -Tail      LIVE: what Marco is watching, right now"
Say "  -Why       everything the Director knows, unsummarised"
Say "  -Where     print the sandbox paths"
Say "  -Clean     stop the Director and delete the sandbox"
Say ""
Say "  One clean crossing is enough. Round 2 is for a DIFFERENT way out of a screen."
Say ""
Say "  Get-Help .\acceptance-36c.ps1 -Detailed   for what this is actually testing"
