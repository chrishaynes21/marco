@echo off
rem Watch what the Director believes it is looking at, live, while you navigate.
rem
rem   watch-interface.cmd                     trackmania, 2 minutes
rem   watch-interface.cmd rocketleague 180    another application, another length
rem
rem ---------------------------------------------------------------------------
rem WHAT THIS IS FOR
rem
rem A passive observation session with the experimental detector running, printing the
rem session's own answer to "what interface am I looking at, what is selected, and what
rem changed since the last observation" every two seconds. It injects NOTHING: no key, no
rem click, no controller. You navigate; it watches.
rem
rem WHY IT IS A SCRIPT AND NOT THREE COMMANDS
rem
rem Because the three commands have to agree about four pieces of configuration, and a run
rem whose configuration nobody recorded is a measurement of an unknown system. In particular:
rem
rem   MARCO_SHADOW_UNGATED  the purchase policy declines an inference whenever the primary
rem                         reading claims to be sufficient. On a game it always claims that
rem                         — accessibility exposes a window and a pane — so the only sensor
rem                         that can see the menu is declined. Measured live on Trackmania:
rem                         ONE inference in 78 looks with the gate on, 130 in 322 with it
rem                         off. The session report names the policy so no number from this
rem                         script can be read as if it came from the default one.
rem
rem   MARCO_HOME            an isolated store. Nothing here should touch the real one.
rem
rem It also shuts down whatever Director is already serving that home first. `--check` style
rem readiness reporting about a process that is not the one you just configured has cost this
rem project several afternoons; see the dogfood stale-director note.
rem ---------------------------------------------------------------------------
setlocal
cd /d "%~dp0"

set "APP=%~1"
if "%APP%"=="" set "APP=trackmania"
set "SECS=%~2"
if "%SECS%"=="" set "SECS=120"

if not exist "director.exe" (
  echo watch-interface: no director.exe. Build it first:
  echo     go build -o director.exe ./cmd/director
  exit /b 1
)
if not exist "plugins\vision\vision.exe" (
  echo watch-interface: no vision plugin. The default `go build` in plugins/vision
  echo                  produces a detector that detects NOTHING. Build it with:
  echo     plugins\vision\build.cmd
  exit /b 1
)

set "MARCO_HOME=%LOCALAPPDATA%\marco-liveiface"
set "MARCO_SHADOW_VISION=screenparser"
if "%MARCO_SHADOW_CADENCE%"=="" set "MARCO_SHADOW_CADENCE=1s"
set "MARCO_SHADOW_UNGATED=1"
set "DIRECTOR_VISION=%CD%\plugins\vision\vision.exe"
if not exist "%MARCO_HOME%" mkdir "%MARCO_HOME%"

rem THE OLD ONE GOES FIRST. A Director already serving this home was started with whatever
rem configuration it was started with, and it would answer every question below.
director.exe shutdown >nul 2>&1

echo Starting a Director on %MARCO_HOME% ...
start "marco-watch-interface" /min director.exe serve
rem The endpoint file appears when it is listening. Polling for it beats sleeping for a
rem guess, and a guess that was too short reads as "the Director is not running".
set /a TRIES=0
:wait
set /a TRIES+=1
director.exe status >nul 2>&1
if not errorlevel 1 goto ready
if %TRIES% GEQ 30 (
  echo watch-interface: the Director did not come up.
  exit /b 1
)
rem `timeout` refuses when stdin is redirected, which is how a harness runs this.
ping -n 2 127.0.0.1 >nul
goto wait

:ready
echo.
director.exe observe-game --application "%APP%" --duration %SECS%s --interval 500ms
if errorlevel 1 (
  echo watch-interface: could not start a session on "%APP%".
  echo                  `director.exe windows` lists what it can see.
  director.exe shutdown >nul 2>&1
  exit /b 1
)
echo.
echo Navigate the menu. Ctrl-C stops watching; the session stops on its own.
echo.

set /a LOOPS=%SECS%/2
for /l %%i in (1,1,%LOOPS%) do (
  cls
  director.exe observation-session observe_1
  ping -n 3 127.0.0.1 >nul
)

echo.
echo The session is over. The Director is still running on this script's isolated home,
echo so run this script again to watch another one, or stop it with:
echo     set MARCO_HOME=%MARCO_HOME%  ^&^&  director.exe shutdown
endlocal
