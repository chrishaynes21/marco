# gate.ps1 -- the alpha's full regression gate, in the order it has to run in.
#
#   1. the suites, with what declined and what outlived them (cmd\gate)
#   2. a CLEAN CHECKOUT of this commit, built and gated from the committed source (cmd\cleanroom)
#   3. the bundle, packed from this tree (pack.ps1)
#   4. the bundle INSTALLED from an isolated directory, reporting what it is made of
#   5. the tests whose preconditions are facts about the screen (-tags desktop)
#   6. the complete desktop harness against those installed binaries (plugins\e2e)
#
# -- WHY THE ORDER IS THE POINT ------------------------------------------------
#
# Each stage is only worth anything if the one before it passed. A harness run against binaries
# whose own suite is failing tells you about the harness; a bundle validated from the tree it was
# built in cannot tell you whether an installation works; and the desktop half must be the LAST
# thing, because it is the only part that takes somebody's screen.
#
# -- AND STAGE 1 IS NOT EVIDENCE ABOUT A CLONE ---------------------------------
#
# Stage 2 exists because stage 1 reads the working tree, and the working tree is not what anybody
# else gets. `go build ./...` and the whole suite passed here for the entire life of
# cmd\perceptionreplay\coverage.go -- 420 lines of production source that was never committed,
# because .gitignore carried `coverage.*` and that matches a .go file as readily as a .out file.
# `git status` does not list an ignored file, so nothing could have said so: the check and the
# thing it checks shared a blind spot.
#
# So packing is now gated on a build of the COMMITTED source, in a checkout that holds nothing
# else, and the result names the commit it tested. A stale result from another commit cannot
# stand in for this one.
#
# -- AND A RUN THAT WAS NARROWED IS NOT THE GATE -------------------------------
#
# -SuitesOnly, -Installed and -Only each leave a question nobody asked, so each one adds a line
# to $narrowed and the verdict reports what the pass is a pass OF. A partial run used to end with
# the same sentence as a whole one, which made the sentence worth nothing.
#
# -- WHAT RUNNING THIS DOES TO YOUR MACHINE ------------------------------------
#
# Stages 1 and 2 open nothing. Stage 4 installs into a temporary directory with LOCALAPPDATA and
# APPDATA pointed away from your own, so your library and your installed Marco are not touched.
# Stage 6
# opens two windows of its own and lets Marco perform REAL INPUT into its own fixture window on
# your actual screen -- so do not type during it.
#
# It ends processes only when it can SHOW it started them. Something you started yourself is
# reported, kept and LEFT RUNNING -- including when you have closed the shell it came from, which
# used to be read as permission.
#
#   .\gate.ps1                 all six stages
#   .\gate.ps1 -SuitesOnly     stage 1, which needs no screen
#   .\gate.ps1 -Installed <bin>  stages 1, 5 and 6 against an installation you already have
#   .\gate.ps1 -Only play      stage 6 limited to one family (NOT the gate: a partial run)
#
# See [[ADR-264-the-gate-answers-for-what-did-not-run]].

param(
    [switch]$SuitesOnly,
    [string]$Installed = "",
    [string]$Only = "",
    [switch]$Keep
)

$ErrorActionPreference = "Continue"
$root = $PSScriptRoot
$started = Get-Date
$stages = @()
$narrowed = @()

function Stage($name, $code, $says) {
    $script:stages += [pscustomobject]@{ Stage = $name; Code = $code; Says = $says }
}

# Narrowed records a question THIS RUN DID NOT ASK, so that the verdict can say so.
function Narrowed($says) { $script:narrowed += $says }

# Verdict is the one place this script ends, so that no exit can skip the stage table.
#
# A failing stage used to exit from where it stood, printing one red line and no table -- so a run
# that died in packing left nothing behind saying which stage had been reached. And a pass now
# says what it is a pass OF: with any of the narrowing switches, the full sentence would be a
# claim about scenarios, a bundle or an installation this run never touched.
function Verdict {
    Write-Host ""
    Write-Host ("-" * 78)
    $script:stages | ForEach-Object {
        $mark = if ($_.Code -eq 0) { "passed " } else { "FAILED " }
        Write-Host ("  {0} {1,-9} {2}" -f $mark, $_.Stage, $_.Says)
    }
    Write-Host ""
    Write-Host ("took {0}" -f ((Get-Date) - $script:started).ToString("hh\:mm\:ss"))
    $worst = ($script:stages | Where-Object { $_.Code -ne 0 } | Select-Object -First 1)
    if ($null -ne $worst) {
        Write-Host ("gate: FAILED at {0}: {1}. Nothing after it was run." -f $worst.Stage, $worst.Says) -ForegroundColor Red
        # WITH THE FAILING STAGE'S OWN CODE: 1 is a suite or a scenario that failed,
        # 2 is a bundle that could not be packed or installed.
        exit $worst.Code
    }
    if ($script:narrowed.Count -gt 0) {
        Write-Host "gate: PASSED WHAT IT RAN, WHICH WAS NOT THE GATE." -ForegroundColor Yellow
        $script:narrowed | ForEach-Object {
            Write-Host ("  not asked: {0}" -f $_) -ForegroundColor Yellow
        }
        exit 0
    }
    Write-Host "gate: passed -- the suites, a clean checkout of this commit, the packaged bundle, its installation, and every registered scenario." -ForegroundColor Green
    Write-Host "physical recording capture: NOT EXERCISED. A person demonstrating in Studio is still the only evidence of that."
    exit 0
}

Write-Host ""
Write-Host "marco gate" -ForegroundColor Cyan
Write-Host "  this tree: $root"
if (-not $SuitesOnly) {
    Write-Host "  stages 5 and 6 take your screen; stage 6 performs real input into its own fixture window." -ForegroundColor Yellow
}
Write-Host ""

# -- 1. THE SUITES -------------------------------------------------------------
#
# TOLD WHICH TREE IT IS FOR, rather than left to read whichever directory it was started from:
# `.\gate.ps1` from the root and `D:\Macros\marco\gate.ps1` from anywhere else would otherwise be
# two different runs, and the second would gate a tree nobody named.
Write-Host "== 1/6  the suites, and what declined or outlived them" -ForegroundColor Cyan
& go -C $root run ./cmd/gate -root $root
$suites = $LASTEXITCODE
Stage "suites" $suites "cmd\gate"
if ($suites -ne 0) { Verdict }

if ($SuitesOnly) {
    Narrowed "a clean checkout of this commit, the bundle, its installation, the tests that need the screen and every desktop scenario (-SuitesOnly)"
    Verdict
}

$bin = $Installed
if ($bin -eq "") {
    # -- 2. A CLEAN CHECKOUT OF THIS COMMIT ------------------------------------
    #
    # A PREREQUISITE TO PACKAGING, because a bundle packed from a commit nobody can build is a
    # bundle only this machine could have made. The result names the commit, and this stage
    # compares that name with the commit it is about to pack -- a passing record of some other
    # commit is not a record of this one.
    #
    # AND THE TREE HAS TO BE CLEAN, or the two halves are about different things: cleanroom
    # builds the COMMIT and pack.ps1 packs the WORKING TREE, so an uncommitted change would be
    # packed having been checked nowhere. That is the same shape of gap as the ignored file this
    # stage exists for, one step along.
    Write-Host ""
    Write-Host "== 2/6  a clean checkout of this commit, built and gated" -ForegroundColor Cyan
    $head = (& git -C $root rev-parse HEAD 2>$null)
    if ($LASTEXITCODE -ne 0 -or $null -eq $head) {
        Stage "clean checkout" 2 "this tree is not a git checkout, so the committed source cannot be built from it"
        Verdict
    }
    $head = $head.Trim()
    $dirty = @(& git -C $root status --porcelain) | Where-Object { $_ -ne "" }
    if ($dirty.Count -gt 0) {
        Stage "clean checkout" 2 "$($dirty.Count) uncommitted change(s): the bundle would be packed from the working tree while only $($head.Substring(0,7)) was checked"
        Write-Host ""
        Write-Host "gate: the working tree has uncommitted changes, so what would be packed is not what a clean checkout can build." -ForegroundColor Red
        Write-Host "  Commit or stash them, then run the gate again. Changed:" -ForegroundColor Red
        $dirty | Select-Object -First 10 | ForEach-Object { Write-Host "    $_" -ForegroundColor Red }
        if ($dirty.Count -gt 10) { Write-Host "    ... and $($dirty.Count - 10) more" -ForegroundColor Red }
        Verdict
    }

    $cleanResult = Join-Path $env:TEMP ("marco-cleanroom-" + (Get-Date -Format "yyyyMMdd-HHmmss") + ".txt")
    & go -C $root run ./cmd/cleanroom -root $root -commit $head -result $cleanResult
    $cleanCode = $LASTEXITCODE

    # ASKED OF THE RECORD AND NOT ONLY OF THE EXIT CODE, for the same reason packing is asked
    # three ways: an exit code says a program finished, and what a later step reads is the file.
    $recorded = ""
    $verdict = ""
    if (Test-Path $cleanResult) {
        foreach ($line in (Get-Content $cleanResult)) {
            if ($line -match '^commit:\s*(\S+)') { $recorded = $Matches[1] }
            if ($line -match '^verdict:\s*(\S+)') { $verdict = $Matches[1] }
        }
    }
    $why = ""
    if ($cleanCode -ne 0) {
        $why = "cmd\cleanroom exited $cleanCode : this commit does not build from the committed source alone"
    } elseif ($recorded -eq "") {
        $why = "cmd\cleanroom reported success and wrote no result naming a commit, so there is nothing to gate packing on"
    } elseif ($verdict -ne "PASSED") {
        $why = "the recorded verdict is '$verdict', not PASSED"
    } elseif ($recorded -ne $head) {
        $why = "the result names commit $($recorded.Substring(0,7)) and this run would pack $($head.Substring(0,7))"
    }
    if ($why -ne "") {
        Stage "clean checkout" 2 $why
        Write-Host ""
        Write-Host "gate: no clean checkout of this commit has been built, so nothing was packed." -ForegroundColor Red
        Write-Host "  $why" -ForegroundColor Red
        Verdict
    }
    Stage "clean checkout" 0 "$($head.Substring(0,7)) built and gated from a clean checkout; $cleanResult"

    # -- 3. THE BUNDLE ---------------------------------------------------------
    Write-Host ""
    Write-Host "== 3/6  packing the bundle" -ForegroundColor Cyan
    $bundle = Join-Path $root "dist\Marco.exe"
    # WHAT IS THERE BEFORE, so that what is there after can be compared with it.
    $was = $null
    if (Test-Path $bundle) {
        $f = Get-Item $bundle
        $was = [pscustomobject]@{ When = $f.LastWriteTimeUtc; Size = $f.Length }
    }
    $packing = (Get-Date).ToUniversalTime()
    $why = ""

    # -- THE PACKER FAILS BY THROWING, AND A THROW IS NOT AN EXIT CODE ---------
    #
    # pack.ps1 validates what it staged and reports a bad bundle with `throw`: a missing
    # director.exe, a Studio built without the Wails tags, a binary carrying no VCS revision.
    # $LASTEXITCODE is then whatever the last successful native command left behind, which is
    # zero -- and an exit code checked on its own would have let this script install and gate
    # whatever dist\Marco.exe WAS ALREADY THERE.
    #
    # Asked three ways, because each catches a different shape of the same failure: the
    # exception, the exit code, and -- the only one of the three that is actually evidence --
    # whether the bundle on disk is the one this run made. A pack.ps1 that is missing, or that
    # swallows a failure of its own with -ErrorAction, produces neither an exception that
    # reaches here nor an exit code of its own, and passes the first two.
    try {
        $ErrorActionPreference = "Stop"
        & (Join-Path $root "pack.ps1")
        if ($LASTEXITCODE -ne 0) { $why = "pack.ps1 exited $LASTEXITCODE" }
    } catch {
        $why = "pack.ps1 failed: " + $_.Exception.Message
    } finally {
        $ErrorActionPreference = "Continue"
    }
    if ($why -eq "" -and -not (Test-Path $bundle)) {
        $why = "pack.ps1 reported no failure and there is no dist\Marco.exe"
    }
    if ($why -eq "") {
        # WRITTEN DURING THIS RUN, OR AT LEAST NOT THE FILE THAT WAS THERE BEFORE IT.
        #
        # Two arms, because neither covers the other. `go build -o` writes the output and so
        # dates it now, which is the ordinary case and is what the first arm establishes; but
        # Copy-Item PRESERVES a write time, so a packer that assembles the bundle from
        # something built a moment earlier would be called stale by the first arm alone. What
        # a packer that did nothing cannot produce is a CHANGE, which is the second.
        $made = Get-Item $bundle
        $fresh = ($made.LastWriteTimeUtc -ge $packing)
        $changed = ($null -eq $was) -or ($made.LastWriteTimeUtc -ne $was.When) -or
            ($made.Length -ne $was.Size)
        if (-not $fresh -and -not $changed) {
            $why = ("dist\Marco.exe is unchanged from the file that was there before this " +
                "run began -- last written {0:yyyy-MM-dd HH:mm:ss}Z, the same {1} bytes " +
                "-- so pack.ps1 reported success and produced nothing, and this is not " +
                "the bundle this run packed") -f $made.LastWriteTimeUtc, $made.Length
        }
    }
    if ($why -ne "") {
        Stage "pack" 2 $why
        Write-Host ""
        Write-Host "gate: the bundle could not be packed, so nothing was installed or run against it." -ForegroundColor Red
        Write-Host "  $why" -ForegroundColor Red
        Verdict
    }
    Stage "pack" 0 "dist\Marco.exe"

    # -- 4. INSTALLED, FROM SOMEWHERE THAT IS NOT THIS TREE --------------------
    #
    # With LOCALAPPDATA and APPDATA pointed away from the owner's, no repository beside it and
    # PATH cut to the system directories: the three things a developer's machine supplies by
    # accident and an installation cannot rely on. The first packed alpha reported a missing
    # file and an unreadable manifest here, both correct bundles.
    Write-Host ""
    Write-Host "== 4/6  installing it from an isolated directory" -ForegroundColor Cyan
    $iso = Join-Path $env:TEMP ("marco-gate-" + (Get-Date -Format "yyyyMMdd-HHmmss"))
    New-Item -ItemType Directory -Force -Path $iso | Out-Null
    Copy-Item $bundle (Join-Path $iso "Marco.exe")
    $local = Join-Path $iso "Local"
    $roaming = Join-Path $iso "Roaming"
    $store = Join-Path $iso "store"
    New-Item -ItemType Directory -Force -Path $local, $roaming, $store | Out-Null

    $script = @"
`$env:LOCALAPPDATA = '$local'
`$env:APPDATA = '$roaming'
`$env:PATH = 'C:\Windows\system32;C:\Windows'
`$env:MARCO_HOME = ''
`$env:MARCO_ROUTES = ''
Set-Location '$iso'
& '$iso\Marco.exe' -acts '$store'
exit `$LASTEXITCODE
"@
    $file = Join-Path $iso "install.ps1"
    Set-Content -Path $file -Value $script -Encoding utf8
    $said = & powershell -NoProfile -ExecutionPolicy Bypass -File $file 2>&1
    $code = $LASTEXITCODE
    $said | Select-String -Pattern "Marco |build |made of|PROBLEM" | ForEach-Object { Write-Host "  $_" }
    $problems = @($said | Select-String -Pattern "PROBLEM").Count
    if ($code -ne 0 -or $problems -gt 0) {
        Stage "install" 2 "$problems problem(s) reported by the installed bundle (exit $code)"
        Write-Host ""
        Write-Host "gate: the packaged bundle did not install cleanly, so the desktop half was not run." -ForegroundColor Red
        Verdict
    }
    Stage "install" 0 $iso
    $bin = Join-Path $local "Marco\bin"
} else {
    Write-Host ""
    Write-Host "== 3-4/6  skipped: testing the installation at $bin" -ForegroundColor Yellow
    Stage "pack" 0 "skipped (-Installed)"
    Stage "install" 0 $bin
    Narrowed "whether a clean checkout of this commit builds, and whether this tree packs a bundle that installs cleanly from nowhere (-Installed), so stages 2, 3 and 4 did not run"
}

# -- 5. THE TESTS WHOSE PRECONDITIONS ARE FACTS ABOUT THE SCREEN ---------------
#
# `-tags desktop`. These are out of stage 1 on purpose: stage 1 must be usable while somebody is
# using their computer, and a test that needs a window to be there and to hold still across three
# looks is a test that reports a finding whenever a game is on screen. They are not declared
# skips, because `gate-skips.txt` can express neither state of an environment-dependent one: an
# undeclared skip fails the gate, and a declaration whose test ran fails it too.
#
# BEFORE THE HARNESS AND NOT AFTER IT. These only LOOK at the screen; stage 6 performs real input
# into its own fixture window. Running the observers first means a failure here is read against a
# desktop nothing has pressed yet.
#
# Run from the tree rather than the installation, because a build tag is a property of the source
# and there is nothing tagged in a bundle.
Write-Host ""
Write-Host "== 5/6  the tests that need the screen (-tags desktop)" -ForegroundColor Cyan
& go -C $root test -count=1 -tags desktop ./cmd/director/
$desktopTests = $LASTEXITCODE
Stage "desktop tests" $desktopTests "go test -tags desktop .\cmd\director"
if ($desktopTests -ne 0) { Verdict }

# -- 6. THE COMPLETE DESKTOP HARNESS, AGAINST THOSE BINARIES -------------------
#
# EVERY REGISTERED SCENARIO. `-only` is not the gate: a selection that matched some of them is a
# partial run, and the scenario that did not run is the one whose question nobody asked.
Write-Host ""
Write-Host "== 6/6  the complete desktop harness against the installed binaries" -ForegroundColor Cyan
$harnessArgs = @("-installed", $bin)
if ($Only -ne "") {
    Write-Host "  NOT THE GATE: -Only $Only runs some of the scenarios, which is a partial run." -ForegroundColor Yellow
    $harnessArgs += @("-only", $Only)
    Narrowed "every scenario whose name does not match '$Only' (-Only), none of which ran"
}
if ($Keep) { $harnessArgs += "-keep" }
& (Join-Path $root "plugins\e2e\run.cmd") @harnessArgs
$harness = $LASTEXITCODE
Stage "harness" $harness "plugins\e2e"

Verdict
