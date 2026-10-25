@echo off
rem Build and run the desktop harness.
rem
rem   plugins\e2e\run.cmd              every scenario
rem   plugins\e2e\run.cmd -only play   the scenarios whose names contain "play"
rem   plugins\e2e\run.cmd -keep        leave this run's store and binaries behind
rem
rem ---------------------------------------------------------------------------
rem WHAT RUNNING THIS DOES TO YOUR MACHINE
rem
rem It opens two windows of its own — a fixture and a Studio — and lets Marco perform REAL
rem INPUT into the fixture. While a scenario is in flight, presses land on the fixture window
rem on your actual screen, so do not type during a run.
rem
rem It does not touch your library, your store, your Studio, or any binary in the tree: the
rem binaries under test are built fresh into a directory of this run's own, and the store is a
rem temporary one that is removed afterwards. It refuses to start if a fixture is already
rem running, and it will not end a process it did not start.
rem
rem An unavailable desktop is reported as NOT RUN and exits 2. That is never a pass.
rem
rem EXIT CODES
rem   0  every scenario selected ran, and passed
rem   1  a scenario failed
rem   2  something selected did not run: none of them, some of them, or the selection
rem      matched nothing. A partial run is not a pass — the scenario that did not run is
rem      the one whose question nobody asked
rem   3  the harness itself could not get started
rem
rem The harness is BUILT and then RUN, rather than `go run`: go run reports a child's exit
rem code as a sentence on stderr and exits 1 itself, which would flatten 2 and 3 into "a
rem scenario failed" for whatever is reading this.
rem ---------------------------------------------------------------------------
setlocal
cd /d "%~dp0"

rem THE FIXTURE FIRST, and only when its source is newer than the binary. It is a committed
rem binary, so rebuilding it every time would dirty the tree whose revision this run reports.
powershell -NoProfile -Command "$src = Get-Item 'Fixture.cs' -ErrorAction SilentlyContinue; exit 0" >nul 2>&1
powershell -NoProfile -Command "$s = Get-Item '..\fixture\Fixture.cs'; $b = Get-Item '..\fixture\fixture.exe' -ErrorAction SilentlyContinue; if (-not $b -or $b.LastWriteTime -lt $s.LastWriteTime) { & '..\fixture\build.ps1' } else { Write-Host 'fixture.exe is current' }"
if errorlevel 1 (
  echo e2e: the fixture could not be built.
  exit /b 3
)

go build -o e2e.exe .
if errorlevel 1 (
  echo e2e: the harness could not be built.
  exit /b 3
)
"%~dp0e2e.exe" %*
exit /b %errorlevel%
