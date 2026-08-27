# Encoded UTF-8 WITH A BOM, deliberately. Windows PowerShell 5.1 decodes a BOM-less file as the
# system ANSI codepage, and an em dash then arrives as three characters -- one of which is a
# double quote. Inside a string that terminates it early, and the script fails to PARSE with an
# error pointing at a brace fifty lines away. Measured on acceptance-35c.ps1: it did exactly that.
# The BOM is the fix; keeping the prose ASCII below is the belt to its braces.
<#
.SYNOPSIS
  The minimum live acceptance for Roadmap 36A (ambient Observe). It measures the one claim the
  Go suite cannot reach: what leaving Marco watching actually costs on a real desktop.

.DESCRIPTION
  36A's central promise is that what Marco holds from watching grows with NOVELTY and never with
  TIME. That is gated deterministically -- ten thousand sightings of one screen are one entry --
  but a deterministic test drives a fake clock past a fake desktop. What it cannot tell you is
  whether the real thing is affordable to leave running: CPU, memory, how often it actually
  reads the screen, and whether the counts stay flat when you sit still.

      .\acceptance-36a.ps1 -Setup             build, sandbox, start a Director
      .\acceptance-36a.ps1 -Watch -Quiet      leave the desktop alone for 5 minutes
      .\acceptance-36a.ps1 -Watch -Busy       use your computer normally for 5 minutes
      .\acceptance-36a.ps1 -Report            print both, side by side
      .\acceptance-36a.ps1 -Clean             stop the Director and delete the sandbox

  Run BOTH -Quiet and -Busy. Neither means anything alone: -Quiet answers "does it grow with
  time" and -Busy answers "does it notice anything at all", and a harness that only ran the
  first would call a broken observer a resounding success.

  YOUR REAL PLAYS AND MEMORY ARE NEVER WRITTEN TO. -Setup COPIES your semantic memory into a
  throwaway MARCO_HOME under the TEMP directory and everything after that runs there. -Clean
  deletes it.

  NOTHING HERE DRIVES YOUR DESKTOP. Observe watches; it does not act, and no part of this script
  performs a Play, presses a key or moves the mouse. That is not a courtesy -- it is the property
  under test. Compare acceptance-35c.ps1, which says the opposite in this same paragraph.
#>
[CmdletBinding()]
param(
    [switch]$Setup,
    [switch]$Watch,
    [switch]$Quiet,
    [switch]$Busy,
    [switch]$Report,
    [switch]$Where,
    [switch]$Clean,
    [int]$Minutes = 5,
    [int]$EverySeconds = 10
)

$ErrorActionPreference = "Stop"
$Root    = $PSScriptRoot
$Sandbox = Join-Path $env:TEMP "marco-36a"
$Home36  = Join-Path $Sandbox "home"
$Routes  = Join-Path $Sandbox "routes"
$Store   = Join-Path $Home36 "semantic-memory.json"
$Marco   = Join-Path $Sandbox "marco.exe"
$Dir     = Join-Path $Sandbox "director.exe"

function Use-Sandbox {
    $env:MARCO_HOME   = $Home36
    $env:MARCO_ROUTES = $Routes
    # MARCO_MEMORY names the memory FILE outright and beats MARCO_HOME in both the reader and
    # the writer. Leaving it set would point the sandboxed Director straight back at the real
    # store -- which this whole script exists not to touch.
    Remove-Item Env:\MARCO_MEMORY -ErrorAction SilentlyContinue
}

function Say([string]$m)  { Write-Host $m }
function Step([string]$m) { Write-Host ""; Write-Host $m -ForegroundColor Cyan }
function Good([string]$m) { Write-Host "  PASS  $m" -ForegroundColor Green }
function Bad([string]$m)  { Write-Host "  FAIL  $m" -ForegroundColor Red }
function Warn([string]$m) { Write-Host "  ??    $m" -ForegroundColor Yellow }
function Note([string]$m) { Write-Host "        $m" -ForegroundColor DarkGray }

# Num tells a counter that is present and zero apart from one that is absent.
#
# Every count in AmbientView is omitempty, so a genuine zero is dropped from the JSON and
# arrives here as a null -- which formats as an EMPTY COLUMN. "screens noticed:" followed by
# nothing is the single most important number in a -Quiet run, and it is exactly the one that
# would print blank. Measured on the 35C harness, where it did.
function Num($v) { if ($null -eq $v) { return 0 } return [int]$v }

# Stop-SandboxDirector ends ONLY the Director this script built, and waits for Windows to let go
# of the file.
#
# Matched by PATH, never by name. Killing every director.exe would take down one serving the real
# store, which nothing here started and nothing here is entitled to stop.
#
# It runs before BUILDING, not only on -Clean: a running executable is locked on Windows, so
# "go build -o" into it fails with "the process cannot access the file". The wait matters too --
# Kill() asks, and the handle survives the ask by a moment.
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
        try { $h = [System.IO.File]::Open($Dir, 'Open', 'ReadWrite', 'None'); $h.Close(); return }
        catch { Start-Sleep -Milliseconds 100 }
    }
    Note "the sandbox director.exe is still locked; the build below may fail."
}

# Real-Store is your actual semantic memory file, which this only ever READS.
#
# The same precedence the Go side uses, in the same order, because a harness that guesses
# differently from the product finds nothing and blames the person for not having learned
# anything. See semanticMemoryPath and defaultHome in cmd/director/graph.go. NOT ~\.marco.
function Real-Store {
    if ($env:MARCO_MEMORY) { return $env:MARCO_MEMORY }
    if ($env:MARCO_HOME -and ($env:MARCO_HOME -ne $Home36)) {
        return (Join-Path $env:MARCO_HOME "semantic-memory.json")
    }
    return (Join-Path $env:APPDATA "marco\semantic-memory.json")
}

# Sandbox-Cost is CPU and memory for the Director this script started, and nothing else.
#
# Summed over the process TREE, not the one process. The accessibility provider is a separate
# executable the Director drives, and it is where the real work of reading a screen happens --
# reporting only the Director's own CPU would report a number near zero and call the expensive
# half free.
function Sandbox-Cost {
    $cpu = 0.0
    $mem = 0
    $plugins = Join-Path $Root "plugins"
    $ours = Get-Process -ErrorAction SilentlyContinue | Where-Object {
        try { $_.Path -and ($_.Path -eq $Dir -or $_.Path.StartsWith($plugins)) } catch { $false }
    }
    foreach ($p in $ours) {
        try { $cpu += $p.TotalProcessorTime.TotalSeconds; $mem += $p.WorkingSet64 } catch {}
    }
    return @{ cpu_s = [math]::Round($cpu, 2); rss_mb = [math]::Round($mem / 1MB, 1) }
}

function Observe-Status {
    $raw = & $Marco observe status --json 2>&1 | Out-String
    try { return ($raw | ConvertFrom-Json) } catch { return $null }
}

# --- setup -------------------------------------------------------------------

if ($Setup) {
    $realStore = Real-Store

    Step "Building"
    New-Item -ItemType Directory -Force $Home36, $Routes | Out-Null
    Stop-SandboxDirector   # a running exe is locked, and -Setup is usually a RE-run
    Push-Location $Root
    try {
        & go build -o $Marco ./cmd/marco
        if ($LASTEXITCODE) { throw "building marco" }
        & go build -o $Dir ./cmd/director
        if ($LASTEXITCODE) { throw "building director" }
    } finally { Pop-Location }
    Say "  marco.exe and director.exe -> $Sandbox"

    $bridge = Join-Path $Root "plugins\uia\uia.exe"
    if (-not (Test-Path $bridge)) {
        Bad "the Accessibility provider is not built: $bridge"
        Note "build it with:  powershell -File plugins\uia\build.ps1"
        Note "without it Marco cannot see the screen and there is nothing to measure."
        exit 1
    }
    Good "Accessibility provider present"

    Step "Copying your learned screens into the sandbox (read-only on your side)"
    # WATCHING IS MORE INTERESTING WITH SOMETHING LEARNED, and works without it. A Director with
    # an empty store still observes -- every screen simply reads as one it has not learned, which
    # is an honest -Busy run that measures cost and cannot show recognition. Copying your store
    # in lets one run show both. It is a nicety, so a missing store is a note and not an exit.
    if (Test-Path $realStore) {
        Copy-Item $realStore $Store -Force
        Good "semantic memory copied from $realStore"
        $known = @((Get-Content $Store -Raw | ConvertFrom-Json).places).Count
        Note "$known learned screens; watching will recognise those and no others."
    } else {
        Warn "no semantic memory at $realStore -- watching still works"
        Note "every screen will read as one Marco has not learned. Cost is still measured;"
        Note "recognition is not. Learn a screen first if you want to see that half."
    }
    Note "your originals were read and not written. -Clean deletes the copies."

    Use-Sandbox
    Say "  MARCO_HOME   = $Home36"
    Say "  MARCO_ROUTES = $Routes"

    Step "Starting the Director"
    # A Director publishes its endpoint under its own MARCO_HOME, so "is one running" is the
    # wrong question -- one serving your real store would answer nothing a sandboxed client asks.
    # The question is whether one is serving THIS sandbox, and EXACTLY one.
    $endpoint = Join-Path $Home36 "director-service.json"
    Remove-Item $endpoint -ErrorAction SilentlyContinue
    Start-Process -FilePath $Dir -ArgumentList "serve" -WindowStyle Minimized
    for ($i = 0; $i -lt 20 -and -not (Test-Path $endpoint); $i++) { Start-Sleep -Milliseconds 500 }
    if (-not (Test-Path $endpoint)) {
        Bad "no Director came up for the sandbox"
        Note "expected its endpoint at $endpoint"
        Note "start it by hand to see why: set MARCO_HOME to $Home36 and run: $Dir serve"
        exit 1
    }
    Good "a Director is serving the sandbox"

    Step "It is not watching yet, and it will say so"
    $before = Observe-Status
    if ($null -ne $before -and -not $before.watching) {
        Good "'marco observe status' reports not watching, without starting anything"
    } else {
        Bad "a freshly started Director already thinks it is watching"
        Note "ambient observation is off until asked for. See ADR-093."
        exit 1
    }

    Step "Now the two runs"
    Say ""
    Say "  1. .\acceptance-36a.ps1 -Watch -Quiet   then LEAVE THE DESKTOP ALONE for $Minutes min."
    Say "     Go and make a coffee. Do not touch the mouse. This measures whether what Marco"
    Say "     holds grows with TIME, and any window you switch to spoils it."
    Say ""
    Say "  2. .\acceptance-36a.ps1 -Watch -Busy    then USE YOUR COMPUTER for $Minutes min."
    Say "     Move between applications and pages, especially ones you have learned."
    Say ""
    Say "  3. .\acceptance-36a.ps1 -Report"
    Say ""
    Say "  Neither run touches your desktop. Ctrl-C stops either one; the Director keeps serving."
    exit 0
}

# --- watch -------------------------------------------------------------------

if ($Watch) {
    Use-Sandbox
    if (-not (Test-Path $Dir)) { Bad "run -Setup first"; exit 1 }
    if ($Quiet -and $Busy)     { Bad "-Quiet and -Busy are two different runs"; exit 1 }
    if (-not $Quiet -and -not $Busy) {
        Bad "say which run this is: -Quiet or -Busy"
        Note "-Quiet: leave the desktop alone. -Busy: use it normally. Both are needed."
        exit 1
    }
    $label = "busy"
    if ($Quiet) { $label = "quiet" }
    $samples = Join-Path $Sandbox "$label.jsonl"

    Step "Asking Marco to watch"
    $raw = & $Marco observe --json 2>&1 | Out-String
    try { $on = $raw | ConvertFrom-Json } catch {
        Bad "the Director's reply was not JSON"
        Note $raw.Trim()
        exit 1
    }
    if (-not $on.watching) { Bad "it did not start watching"; Note $raw.Trim(); exit 1 }
    Good "watching"

    if ($Quiet) {
        Say ""
        Say "  HANDS OFF for $Minutes minutes. Do not switch windows. Do not move the mouse."
        Say "  Anything you do makes this run measure the other thing."
    } else {
        Say ""
        Say "  Use your computer normally for $Minutes minutes. Move around applications and"
        Say "  pages. Marco is watching and is not acting."
    }
    Say ""

    # The BASELINE cost is taken AFTER watching starts and before much of it is charged, so what
    # the report prints is the cost OF WATCHING and not the cost of a Director existing.
    Start-Sleep -Seconds 2
    $base = Sandbox-Cost
    $start = Get-Date
    Remove-Item $samples -ErrorAction SilentlyContinue
    $deadline = $start.AddMinutes($Minutes)

    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Seconds $EverySeconds
        $v = Observe-Status
        if ($null -eq $v) { Note "no reply from the Director; still waiting"; continue }
        $c = Sandbox-Cost
        $secs = [int]((Get-Date) - $start).TotalSeconds
        $row = [ordered]@{
            at_s         = $secs
            watching     = [bool]$v.watching
            places       = Num $v.places
            transitions  = Num $v.transitions
            recent       = Num $v.recent
            samples      = Num $v.samples
            sessions     = Num $v.sessions
            attention_ms = Num $v.attention_ms
            application  = "$($v.application)"
            known_place  = ("$($v.place)" -ne "")
            degraded     = [bool]$v.perception_degraded
            cpu_s        = [math]::Round($c.cpu_s - $base.cpu_s, 2)
            rss_mb       = $c.rss_mb
        }
        ($row | ConvertTo-Json -Compress) | Out-File -Append -Encoding utf8 $samples
        $left = [int]($deadline - (Get-Date)).TotalSeconds
        Say ("  {0,4}s  screens {1,3}  moves {2,3}  reads {3,4}  cpu {4,6}s  rss {5,6} MB  ({6}s left)" -f $secs, $row.places, $row.transitions, $row.samples, $row.cpu_s, $row.rss_mb, $left)
    }

    Step "Stopping"
    # STOPPING MUST FORGET. The reply is not sent until the loop has actually stopped, so the
    # counts are read BEFORE the stop and the emptiness AFTER it -- both halves are the claim.
    $last = Observe-Status
    $raw = & $Marco observe stop --json 2>&1 | Out-String
    try { $off = $raw | ConvertFrom-Json } catch { $off = $null }
    if ($null -ne $off -and -not $off.watching) { Good "stopped" }
    else { Bad "it did not report stopping"; Note $raw.Trim() }

    $after = Observe-Status
    if ($null -ne $after -and (Num $after.places) -eq 0 -and (Num $after.transitions) -eq 0) {
        Good "and forgot what it had seen ($(Num $last.places) screens, $(Num $last.transitions) moves)"
    } else {
        Bad "it stopped watching but kept what it saw"
        Note "ambient evidence is transient. See ADR-093."
    }

    Step "Nothing reached disk"
    # THE PROPERTY MOST WORTH CHECKING LIVE. Everything else here is a number; this is the
    # promise. A watching session holds no licence and so cannot write, which means the sandbox
    # store must be byte-for-byte what -Setup copied in -- and if it is not, no amount of good
    # CPU matters.
    if (Test-Path $Store) {
        $now = (Get-FileHash $Store -Algorithm MD5).Hash
        $was = Join-Path $Sandbox "store.md5"
        if (Test-Path $was) {
            if ((Get-Content $was -Raw).Trim() -eq $now) {
                Good "the semantic memory is unchanged"
            } else {
                Bad "WATCHING WROTE TO THE SEMANTIC MEMORY"
                Note "that is a licence bug, not a measurement. See ADR-093."
            }
        } else {
            $now | Out-File -Encoding ascii $was
            Note "recorded the store's checksum; the next run compares against it."
        }
    }
    Say ""
    Say "  .\acceptance-36a.ps1 -Report"
    exit 0
}

# --- report ------------------------------------------------------------------

if ($Report) {
    $any = $false
    foreach ($label in @("quiet", "busy")) {
        $path = Join-Path $Sandbox "$label.jsonl"
        Step "$label run"
        if (-not (Test-Path $path)) {
            Warn "UNMEASURED -- .\acceptance-36a.ps1 -Watch -$label has not been run"
            continue
        }
        $rows = @(Get-Content $path | ForEach-Object { $_ | ConvertFrom-Json })
        if ($rows.Count -eq 0) { Warn "no samples"; continue }
        $any = $true
        $first = $rows[0]
        $last = $rows[-1]
        $mins = [math]::Max($last.at_s / 60.0, 0.001)

        Say ("  watched for      {0:n1} min, {1} status reads" -f $mins, $rows.Count)
        Say ("  screens noticed  {0} -> {1}" -f (Num $first.places), (Num $last.places))
        Say ("  moves noticed    {0} -> {1}" -f (Num $first.transitions), (Num $last.transitions))
        Say ("  screen reads     {0}  ({1:n1} per minute)" -f (Num $last.samples), ((Num $last.samples) / $mins))
        Say ("  attention        {0} ms between reads at the end" -f (Num $last.attention_ms))
        Say ("  CPU              {0:n2} s  ({1:n1}% of one core)" -f $last.cpu_s, (100.0 * $last.cpu_s / [math]::Max($last.at_s, 1)))
        Say ("  memory           {0} MB" -f $last.rss_mb)

        if ($label -eq "quiet") {
            # THE 36A CLAIM. Reads keep climbing while the desktop is unchanged; what is HELD
            # does not. A quiet run whose screen count grew is either a desktop somebody
            # touched or a buffer that is a log, and the two are told apart by asking the
            # person, not by this script.
            $grew = (Num $last.places) - (Num $first.places)
            if ($grew -le 1) {
                Good "what Marco holds did not grow with time ($(Num $last.samples) reads, $grew new screens)"
            } else {
                Bad "it grew by $grew screens while nothing was happening"
                Note "either the desktop was touched during the run, or growth is tracking time."
            }
            # Attention should have backed off toward the ceiling. If it has not, either the
            # desktop kept changing or the backoff is not working -- and those are told apart by
            # the count above, not by this line.
            if ((Num $last.attention_ms) -ge 4000) {
                Good "attention backed off to $(Num $last.attention_ms) ms between reads"
            } else {
                Warn "attention was still $(Num $last.attention_ms) ms; something kept changing"
            }
        } else {
            # A BUSY RUN THAT NOTICED NOTHING IS A FAILED RUN, not a cheap one. This is the
            # check that stops a broken observer reading as a wonderfully efficient one.
            if ((Num $last.places) -ge 2 -or (Num $last.transitions) -ge 1) {
                Good "it noticed you moving around"
            } else {
                Bad "it noticed nothing while you used the computer"
                Note "check the Accessibility provider, or whether the run was actually busy."
            }
            $apps = @($rows | ForEach-Object { $_.application } | Where-Object { $_ } | Select-Object -Unique)
            if ($apps.Count -gt 0) { Note ("saw: " + ($apps -join ", ")) }
            $known = @($rows | Where-Object { $_.known_place }).Count
            Note "$known of $($rows.Count) reads were on a screen it had learned"
            $deg = @($rows | Where-Object { $_.degraded }).Count
            if ($deg -gt 0) { Note "$deg reads could see the window and not the page (degraded)" }
        }
    }
    if (-not $any) { Say ""; Note "nothing measured yet. Start with -Setup." }
    Say ""
    Say "  UNMEASURED by this harness: whether ambient watching changes how a Play performs."
    Say "  36A gates that deterministically -- a Play takes the ordinary door whether or not"
    Say "  Marco is watching -- and this script deliberately performs nothing."
    exit 0
}

# --- where / clean -----------------------------------------------------------

if ($Where) {
    Say "  sandbox   $Sandbox"
    Say "  home      $Home36"
    Say "  store     $Store"
    Say "  samples   $(Join-Path $Sandbox 'quiet.jsonl'), $(Join-Path $Sandbox 'busy.jsonl')"
    Say "  real      $(Real-Store)   (read only, never written)"
    exit 0
}

if ($Clean) {
    Stop-SandboxDirector
    if (Test-Path $Sandbox) { Remove-Item $Sandbox -Recurse -Force; Good "deleted $Sandbox" }
    else { Note "nothing to delete" }
    exit 0
}

Say "  .\acceptance-36a.ps1 -Setup | -Watch -Quiet | -Watch -Busy | -Report | -Where | -Clean"
Say ""
Say "  Run -Watch -Quiet AND -Watch -Busy. Neither means anything alone."
exit 0
