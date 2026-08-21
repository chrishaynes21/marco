# Encoded UTF-8 WITH A BOM, deliberately. Windows PowerShell 5.1 decodes a BOM-less file as the
# system ANSI codepage, and an em dash then arrives as three characters -- one of which is a
# double quote. Inside a string that terminates it early, and the script fails to PARSE with an
# error pointing at a brace fifty lines away. Measured here: this file did exactly that. The BOM
# is the fix; keeping the prose ASCII below is the belt to its braces.
<#
.SYNOPSIS
  The minimum live acceptance for Roadmap 35C (fast verified execution). Two commands and one
  window.

.DESCRIPTION
  35C removes redundant epistemic work from carrying out a learned Play. Every claim it makes is
  gated deterministically in the Go suite; the one thing no test can reach is what the saving is
  worth on a real desktop, against a real accessibility provider, on your machine.

  So that is all this asks for:

      .\acceptance-35c.ps1 -Setup     build, sandbox, copy your learned route in, start Director
      .\acceptance-35c.ps1 -Run       open Settings on Home first -- then this performs it once
      .\acceptance-35c.ps1 -Report    print what it cost
      .\acceptance-35c.ps1 -Clean     stop the Director and delete the sandbox

  YOUR REAL PLAYS AND MEMORY ARE NEVER WRITTEN TO. Setup COPIES your semantic memory and routes
  into a throwaway $MARCO_HOME under $env:TEMP and everything after that runs there. -Clean
  deletes it. Nothing writes back.

  -Run performs REAL INPUT: it navigates Settings, because that is the thing being measured.
  Nothing else on your desktop is touched, and you can stop it at any point with 'marco stop'.
#>
[CmdletBinding()]
param(
    [switch]$Setup,
    [switch]$Run,
    [switch]$Report,
    [switch]$Where,
    [switch]$Clean,
    # The learned outcome to carry out. Change it only if you are measuring a different route.
    [string]$Name = "open mouse settings"
)

$ErrorActionPreference = "Stop"
$Root    = $PSScriptRoot
$Sandbox = Join-Path $env:TEMP "marco-35c"
$Home35  = Join-Path $Sandbox "home"
$Routes  = Join-Path $Sandbox "routes"
$Store   = Join-Path $Home35 "semantic-memory.json"
$Marco   = Join-Path $Sandbox "marco.exe"
$Dir     = Join-Path $Sandbox "director.exe"
$Result  = Join-Path $Sandbox "perform.json"

function Use-Sandbox {
    $env:MARCO_HOME   = $Home35
    $env:MARCO_ROUTES = $Routes
    # MARCO_MEMORY names the memory FILE outright and is honoured by both the reader and the
    # writer, so it wins over MARCO_HOME. Leaving it set would point the sandboxed Director
    # straight back at the real store -- which this whole script exists not to touch.
    Remove-Item Env:\MARCO_MEMORY -ErrorAction SilentlyContinue
}

function Say([string]$m)  { Write-Host $m }
function Step([string]$m) { Write-Host ""; Write-Host $m -ForegroundColor Cyan }
function Good([string]$m) { Write-Host "  PASS  $m" -ForegroundColor Green }
function Bad([string]$m)  { Write-Host "  FAIL  $m" -ForegroundColor Red }
function Note([string]$m) { Write-Host "        $m" -ForegroundColor DarkGray }

# Num is a counter that is present and zero, told apart from one that is absent.
#
# The cost fields are `omitempty`, so a genuine zero is dropped from the JSON and arrives here as
# $null -- which formats as an EMPTY COLUMN. "full establishments:" followed by nothing is the one
# number in this report that most needs to read as a hard zero, and it was the one printing blank.
#
# Absent and zero mean the same thing INSIDE a cost object, and only there: whether the object
# itself is missing is a different question, asked once, before any of this.
function Num($v) { if ($null -eq $v) { return 0 } return [int]$v }

# Stop-Sandbox-Director ends ONLY the director this script built, and waits for Windows to let go
# of the file.
#
# Matched by PATH, never by name. Killing every director.exe would take down one serving the real
# store -- which nothing here started and nothing here is entitled to stop.
#
# It has to run before BUILDING, not only on -Clean. A running executable is locked on Windows, so
# `go build -o` into it fails with "the process cannot access the file", and -Setup died at its
# first step with a message about file handles rather than about the Director it left running.
#
# The wait matters too: Kill() asks, and the handle survives the ask by a moment. Building into a
# path Windows has not released yet fails exactly as if nothing had been stopped.
function Stop-SandboxDirector {
    $ours = Get-Process -Name "director" -ErrorAction SilentlyContinue | Where-Object {
        try { $_.Path -eq $Dir } catch { $false }
    }
    foreach ($p in $ours) {
        try { $p.Kill(); $p.WaitForExit(5000); Note "stopped the sandbox director (pid $($p.Id))" }
        catch {}
    }
    if (-not (Test-Path $Dir)) { return }
    for ($i = 0; $i -lt 40; $i++) {
        try {
            $h = [System.IO.File]::Open($Dir, 'Open', 'ReadWrite', 'None'); $h.Close(); return
        } catch { Start-Sleep -Milliseconds 100 }
    }
    Note "the sandbox director.exe is still locked; the build below may fail."
}

# Real-Store is your actual semantic memory file, which this only ever READS.
#
# The same precedence the Go side uses, in the same order, because a harness that guesses
# differently from the product finds nothing and blames the person for not having learned
# anything. See semanticMemoryPath and defaultHome in cmd/director/graph.go:
#
#   $MARCO_MEMORY names the file outright and beats everything
#   $MARCO_HOME  names the directory it sits in
#   otherwise    os.UserConfigDir() + "\marco", which on Windows is %APPDATA%\marco
#
# NOT ~\.marco. That was this script's first guess, it is not where anything lives, and the
# only symptom was "there is nothing learned to measure" about a store that was full.
function Real-Store {
    if ($env:MARCO_MEMORY) { return $env:MARCO_MEMORY }
    if ($env:MARCO_HOME -and ($env:MARCO_HOME -ne $Home35)) {
        return (Join-Path $env:MARCO_HOME "semantic-memory.json")
    }
    return (Join-Path $env:APPDATA "marco\semantic-memory.json")
}

# ── setup ────────────────────────────────────────────────────────────────────

if ($Setup) {
    $realStore  = Real-Store
    $realRoutes = if ($env:MARCO_ROUTES -and ($env:MARCO_ROUTES -ne $Routes)) {
        $env:MARCO_ROUTES
    } else { Join-Path $Root "routes" }

    Step "Building"
    New-Item -ItemType Directory -Force $Home35, (Join-Path $Routes "global") | Out-Null
    Stop-SandboxDirector   # a running exe is locked, and -Setup is usually a RE-run
    Push-Location $Root
    try {
        & go build -o $Marco ./cmd/marco;    if ($LASTEXITCODE) { throw "building marco" }
        & go build -o $Dir   ./cmd/director; if ($LASTEXITCODE) { throw "building director" }
    } finally { Pop-Location }
    Say "  marco.exe and director.exe -> $Sandbox"

    $bridge = Join-Path $Root "plugins\uia\uia.exe"
    if (-not (Test-Path $bridge)) {
        Bad "the Accessibility provider is not built: $bridge"
        Note "build it with:  powershell -File plugins\uia\build.ps1"
        Note "without it Marco cannot see the screen and nothing here can run."
        exit 1
    }
    Good "Accessibility provider present"

    Step "Copying your learned route into the sandbox (read-only on your side)"
    if (Test-Path $realStore) {
        Copy-Item $realStore $Store -Force
        Good "semantic memory copied from $realStore"
        $goals = @((Get-Content $Store -Raw | ConvertFrom-Json).goals)
        if ($goals.Count -gt 0) {
            Note ("it knows: " + (($goals | ForEach-Object { "'" + $_.name + "'" }) -join ", "))
        }
    } else {
        Bad "no semantic memory at $realStore"
        Note "looked there because that is where the Director itself looks. In order:"
        Note ("  MARCO_MEMORY = " + $(if ($env:MARCO_MEMORY) { $env:MARCO_MEMORY } else { "(unset)" }))
        Note ("  MARCO_HOME   = " + $(if ($env:MARCO_HOME)   { $env:MARCO_HOME }   else { "(unset)" }))
        Note ("  default      = " + (Join-Path $env:APPDATA "marco\semantic-memory.json"))
        Note "there is nothing learned to measure. Run .\acceptance-35b.ps1 -Setup first and"
        Note "demonstrate the route once; then come back here."
        exit 1
    }
    if (Test-Path $realRoutes) {
        Copy-Item (Join-Path $realRoutes "*") $Routes -Recurse -Force -ErrorAction SilentlyContinue
        Good "routes copied from $realRoutes"
    }
    Note "your originals were read and not written. -Clean deletes the copies."

    Use-Sandbox
    Say "  MARCO_HOME   = $Home35"
    Say "  MARCO_ROUTES = $Routes"

    Step "Starting the Director"
    # A Director publishes its endpoint under its own MARCO_HOME, so "is one running" is the
    # wrong question -- one serving your real store would answer nothing a sandboxed client
    # asks. The question is whether one is serving THIS sandbox.
    #
    # And EXACTLY one. The first version of this always started another, so every -Setup left
    # a second Director behind: two processes, one MARCO_HOME, one endpoint file, and one
    # semantic memory between them. Measured -- three were running by the time anybody looked.
    # Stop-SandboxDirector above has already ended ours, so this starts the only one.
    $endpoint = Join-Path $Home35 "director-service.json"
    Remove-Item $endpoint -ErrorAction SilentlyContinue
    Start-Process -FilePath $Dir -ArgumentList "serve" -WindowStyle Minimized
    for ($i = 0; $i -lt 20 -and -not (Test-Path $endpoint); $i++) { Start-Sleep -Milliseconds 500 }
    if (Test-Path $endpoint) {
        Good "a Director is serving the sandbox"
    } else {
        Bad "no Director came up for the sandbox"
        Note "expected its endpoint at $endpoint"
        Note "start it by hand to see why:  `$env:MARCO_HOME='$Home35'; $Dir serve"
        exit 1
    }

    Step "Does the sandbox know the route?"
    # ASKED OF THE STORE, not of `director reach`. Reach takes the application from a running
    # or finished observation session, and a Director that has just started has neither -- so
    # it would answer "I don't know where you are" about a route it knows perfectly well.
    # That is the cold-start gap `PerformGoal` exists to close, and -Run is what exercises it.
    $goal = @($goals | Where-Object { $_.name -eq $Name }) | Select-Object -First 1
    if ($goal) {
        Good "'$($goal.name)' is learned, in $($goal.application)"
        Note "whether it can be planned from where you are is what -Run finds out."
    } else {
        Bad "nothing learned under the name '$Name'"
        Note "the store holds: $(($goals | ForEach-Object { $_.name }) -join ', ')"
        Note "re-run with -Name '<one of those>'."
        exit 1
    }

    Step "Now the only part that needs you"
    Say ""
    Say "  1. Open Windows Settings and go to its HOME page."
    Say "  2. Leave it open. Put any other window in front of it if you like -- the play"
    Say "     brings Settings forward itself, and that is part of what is measured."
    Say "  3. Run:  .\acceptance-35c.ps1 -Run"
    Say ""
    Say "  It will navigate Settings for real. 'marco stop' ends it at any point."
    exit 0
}

# ── run ──────────────────────────────────────────────────────────────────────

if ($Run) {
    Use-Sandbox
    if (-not (Test-Path $Dir)) { Bad "run -Setup first"; exit 1 }

    # NAME THE APPLICATION. `PerformGoal` will search every application that holds goals when
    # this is empty, which works -- but Settings is `applicationframehost`, and so are XBOX and
    # Realtek Audio Console. Saying which one is meant costs nothing and removes one source of
    # ambiguity from a measurement.
    $app = ""
    if (Test-Path $Store) {
        $g = @((Get-Content $Store -Raw | ConvertFrom-Json).goals) |
             Where-Object { $_.name -eq $Name } | Select-Object -First 1
        if ($g) { $app = $g.application }
    }

    Step "Performing '$Name'$(if ($app) { " in $app" })"
    Say "  hands off the keyboard and mouse from here."
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    if ($app) { $raw = & $Dir perform "$Name" --application $app --json 2>&1 | Out-String }
    else      { $raw = & $Dir perform "$Name" --json 2>&1 | Out-String }
    $sw.Stop()

    try { $view = $raw | ConvertFrom-Json } catch {
        Bad "the Director's reply was not JSON"
        Note $raw.Trim()
        exit 1
    }
    $view | Add-Member -NotePropertyName wall_ms -NotePropertyValue $sw.ElapsedMilliseconds -Force
    $json = $view | ConvertTo-Json -Depth 8
    $json | Out-File -Encoding utf8 $Result
    # AND KEEP EVERY ATTEMPT. Runs overwrote each other, so a failure could only be described
    # from memory once the next one had happened -- and consecutive attempts are exactly what a
    # person reaches for when something goes wrong.
    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $json | Out-File -Encoding utf8 (Join-Path $Sandbox "perform-$stamp.json")
    Good "recorded to $Result (and kept as perform-$stamp.json)"
    & $PSCommandPath -Report -Name $Name
    exit 0
}

# ── report ───────────────────────────────────────────────────────────────────

if ($Report) {
    if (-not (Test-Path $Result)) { Bad "nothing recorded yet; run -Run"; exit 1 }
    $view = Get-Content $Result -Raw | ConvertFrom-Json

    Step "What happened"
    # @($null) is a ONE-ELEMENT array containing nothing, so a view with no steps printed a
    # phantom row reading "step  -> : ". Filtered, because an empty row where a reader expects
    # a list is worse than no list.
    $steps = @($view.steps | Where-Object { $_ })
    if ($view.arrived) { Good "arrived: $($view.say)" }
    else {
        Bad "did not arrive: $($view.refusal) -- $($view.say)"
        Note "a refusal is data, not a failed acceptance."
    }
    foreach ($s in $steps) {
        $mark = if ($s.verified) { "verified" } else { $s.refusal }
        Note "step $($s.from) -> $($s.to): $mark"
        if ($s.detail) { Note "   $($s.detail)" }
    }

    # NOTHING WALKED IS NOT AN OLD DIRECTOR, and reporting it as one sends the reader to
    # rebuild binaries that are perfectly current. A refusal before the first edge -- the
    # application not there, the screen unrecognised, somebody else being watched -- produces
    # no steps and therefore no cost, and that is the honest answer to "what did it cost".
    if ($steps.Count -eq 0) {
        Step "What it cost"
        Say "  nothing was walked, so nothing was spent."
        Note "the refusal above happened before the first edge."
        if ($view.refusal -eq "place_unknown") {
            Note ""
            Note "place_unknown means Marco brought the application forward, looked, and did"
            Note "not recognise the screen. Before assuming it is on the wrong page, run:"
            Note ""
            Note "    .\acceptance-35c.ps1 -Where"
            Note ""
            Note "which shows which windows Marco can actually see, and the SHAPE the route's"
            Note "first screen is recognised by. Settings, XBOX and Realtek Audio Console all"
            Note "answer to 'applicationframehost', and a resized window is a different shape."
        }
        exit 0
    }

    Step "What it cost"
    $c = $view.cost
    if (-not $c) {
        Bad "steps ran and the reply carried no cost. This Director predates protocol 10."
        Note "rebuild the sandbox with -Setup."
        exit 1
    }
    $wall     = Num $view.wall_ms
    $inside   = Num $c.total_ms
    $looking  = Num $c.looking_ms
    $samples  = Num $c.samples
    $resolves = Num $c.resolutions
    $estab    = Num $c.establishments
    $confirms = Num $c.confirmations
    $reused   = Num $c.reused

    Say ("  wall clock                {0,6} ms" -f $wall)
    Say ("  inside the walk           {0,6} ms" -f $inside)
    Say ("  spent looking             {0,6} ms" -f $looking)
    Say ""
    Say ("  screen readings           {0,6}" -f $samples)
    Say ("  Place resolutions         {0,6}" -f $resolves)
    Say ("  full establishments       {0,6}" -f $estab)
    Say ("  shortened confirmations   {0,6}" -f $confirms)
    Say ("  proofs reused             {0,6}" -f $reused)

    Step "What was avoided"
    # DERIVED FROM THIS RUN, and labelled as derived throughout. Each reused proof replaced a
    # full establishment -- establishSamples readings with a sampleGap between them -- with a
    # single reading and no gap. Those three constants live in
    # internal/director/rehearse/live.go; if they change, so does this arithmetic.
    $establishSamples = 6
    $confirmSamples   = 1
    $sampleGapMs      = 120
    $perLook          = $establishSamples - $confirmSamples
    $avoided          = $reused * $perLook

    if ($reused -eq 0) {
        Note "no proof was reused. Either the route is a single edge from a screen nothing"
        Note "had established, or every proof was contradicted -- check the steps above."
        exit 0
    }

    Say ("  establishments replaced   {0,6}" -f $reused)
    Say ("  readings avoided          {0,6}   (derived: {1} x {2})" -f $avoided, $reused, $perLook)
    $wouldHave = $samples + $avoided
    $pct = [math]::Round(100.0 * $avoided / [math]::Max($wouldHave, 1), 1)
    Say ("  readings without 35C      {0,6}   (derived)" -f $wouldHave)
    Say ("  reduction                 {0,5}%   (derived)" -f $pct)

    # AND WHAT A READING COST ON THIS MACHINE, from this run rather than from a constant.
    # `looking_ms` is the time inside the shortened confirmations and any establishments; with
    # confirmations of one reading each, dividing gives the cost of one reading HERE. That is
    # the only per-reading figure anybody has, and it is this desktop's.
    if ($looking -gt 0 -and $confirms -gt 0 -and $estab -eq 0) {
        $perReading = [math]::Round($looking / $confirms, 0)
        $timeAvoided = $reused * ($perLook * $perReading + $perLook * $sampleGapMs)
        Say ""
        Say ("  one screen reading        {0,6} ms  (measured here: {1} ms / {2})" -f `
            $perReading, $looking, $confirms)
        Say ("  time avoided              {0,6} ms  (derived)" -f $timeAvoided)
        Say ("  walk without 35C          {0,6} ms  (derived)" -f ($inside + $timeAvoided))
    }
    Note "derived, not measured: the same route was not run against the old code, because"
    Note "a switch to turn the handoff off would be a second path through the one part of"
    Note "this system that must have only one. The deterministic proof of the same claim is"
    Note "TestOneRouteProvesEachScreenOnce, which calibrates against the production code."
    exit 0
}

# ── where ────────────────────────────────────────────────────────────────────

# -Where answers the question a `place_unknown` leaves behind: is Marco even looking at the right
# window, and what would it have to see to recognise the screen?
#
# It exists because "put it on the screen the route starts from" is useless advice when the
# person believes they already have. Settings, XBOX and Realtek Audio Console all answer to
# `applicationframehost`, and a route's start is a STRUCTURE -- so many buttons, so many groups --
# which shifts when a window is resized or when Windows changes what it puts on a page.
if ($Where) {
    Use-Sandbox
    if (-not (Test-Path $Store)) { Bad "run -Setup first"; exit 1 }
    $mem  = Get-Content $Store -Raw | ConvertFrom-Json
    $goal = @($mem.goals) | Where-Object { $_.name -eq $Name } | Select-Object -First 1
    if (-not $goal) { Bad "nothing learned under '$Name'"; exit 1 }
    $app = $goal.application

    Step "Which windows Marco can see in $app"
    $wins = & $Dir windows --application $app 2>&1 | Out-String
    Say ($wins.TrimEnd())
    Note "if Settings is not in that list, nothing below can match: Marco cannot look at a"
    Note "window it cannot see, and XBOX and Realtek Audio Console answer to the same name."

    Step "What the route starts from"
    # WHERE the route begins: the first edge whose From nothing else points to.
    $rels = @($mem.relationships | Where-Object { $_.application -eq $app })
    $tos  = @($rels | ForEach-Object { $_.to })
    $start = @($rels | Where-Object { $tos -notcontains $_.from } | ForEach-Object { $_.from } |
              Select-Object -Unique)
    foreach ($id in $start) {
        $s = @($mem.subjects) | Where-Object { $_.id -eq $id } | Select-Object -First 1
        if (-not $s) { continue }
        $name = if ($s.semantic) { $s.semantic } elseif ($s.called) { $s.called } else { "(unnamed)" }
        Say "  $name  [$id]"
        $roles = $s.structure.roles
        if ($roles) {
            $pairs = $roles.PSObject.Properties | Sort-Object Name |
                     ForEach-Object { "$($_.Name)=$($_.Value)" }
            Note ("it is recognised by its shape: " + ($pairs -join ", "))
        }
    }
    Step "What Marco actually read, last time it looked"
    # `sight` reports the LAST session's evidence, which -Run leaves behind. It is the only
    # cheap way to tell the two failures apart, and they need opposite things done about them:
    #
    #   read the window, recognised nothing   -> the page really is one Marco does not know
    #   barely read the window at all         -> the content tree was not there to read
    #
    # The second is the one that looks like the first and is not. Settings is a UWP app hosted
    # inside ApplicationFrameHost; when the hosted content is suspended or has not painted, the
    # accessibility tree collapses to the frame -- caption buttons, a title, and ONE box the
    # size of the content area. A dozen or so controls where there should be well over a
    # hundred, and "I don't recognise this screen" is a true but deeply misleading way to say it.
    $sight = & $Dir sight --json 2>&1 | Out-String
    try { $s = $sight | ConvertFrom-Json } catch { $s = $null }
    if ($s) {
        Say "  the place    $($s.place)"
        Say "  it read      $($s.about)"
        $read = 0
        if ($s.about -match '(\d+)') { $read = [int]$Matches[1] }
        $want = 0
        foreach ($id in $start) {
            $sub = @($mem.subjects) | Where-Object { $_.id -eq $id } | Select-Object -First 1
            if ($sub -and $sub.structure.roles) {
                $n = ($sub.structure.roles.PSObject.Properties |
                      Measure-Object -Property Value -Sum).Sum
                if ($n -gt $want) { $want = [int]$n }
            }
        }
        Say "  it needs     about $want controls to recognise the page it starts from"
        if ($read -gt 0 -and $want -gt 0 -and $read -lt ($want / 3)) {
            Bad "the window is barely being read at all"
            Note "$read controls against $want is not a resized page -- it is the content"
            Note "tree missing. Settings runs inside ApplicationFrameHost, and a suspended or"
            Note "unpainted hosted app reports its frame and one blank box where the page is."
            Note ""
            Note "CLOSE Settings completely and open it again, put it on Home, then -Run."
        }
    } else {
        Note "nothing to report yet -- `sight` reads the last look, so run -Run first."
    }

    Note ""
    Note "Marco matches a screen by its shape, not its title, so a genuinely resized page or"
    Note "different Windows cards will also fail to match -- honestly, and with a similar count"
    Note "to the numbers above rather than a tenth of it."
    exit 0
}

# ── clean ────────────────────────────────────────────────────────────────────

if ($Clean) {
    Step "Cleaning up"
    Stop-SandboxDirector
    if (Test-Path $Sandbox) {
        Remove-Item -Recurse -Force $Sandbox
        Good "removed $Sandbox"
    }
    Remove-Item Env:\MARCO_HOME, Env:\MARCO_ROUTES -ErrorAction SilentlyContinue
    Say "  your real store and plays were only ever read."
    exit 0
}

Say "usage: .\acceptance-35c.ps1 -Setup | -Run | -Report | -Where | -Clean"
Say ""
Say "  -Setup   build, sandbox, copy your learned route in, start the Director"
Say "  -Run     perform the route once and record what it cost"
Say "  -Report  print the counters from the last run"
Say "  -Where   what Marco can see, and what the route needs it to see"
Say "  -Clean   stop the Director and delete the sandbox"
exit 2
