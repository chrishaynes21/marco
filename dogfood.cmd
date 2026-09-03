@echo off
rem Marco dogfood — the control centre, against an ISOLATED semantic memory.
rem
rem One command, one window. MARCO_HOME points at a throwaway store so the graph can be
rem inspected, reset and destroyed as often as an experiment needs, without touching the real
rem one at %APPDATA%\marco. Everything inside it is ordinary production behaviour: there is no
rem sandbox-only semantic path, and `director reset-test-state` refuses any home that is the
rem default one.
rem
rem   dogfood.cmd          open the control centre, watching and learning
rem   dogfood.cmd reset    empty the dogfood store first, then open it
rem   dogfood.cmd --check  print the preconditions and stop, opening nothing
rem
rem ---------------------------------------------------------------------------
rem IT VERIFIES ITS OWN PRECONDITIONS AND REFUSES TO OPEN IF THEY ARE FALSE.
rem
rem Four dogfood runs in a row were walked against a Director that was not watching. A reset
rem shuts the Director down and the replacement starts idle, so every reset silently disarmed
rem the one thing the experiment was about. The control centre said `not watching` the whole
rem time, and nobody was reading the strip — they were reading the screen they were navigating.
rem
rem Losing an afternoon to a run that was not exercising the configuration we thought it was is
rem worse than refusing to start. So this prints what is actually true and exits non-zero if the
rem answer is not the one the experiment needs.
rem ---------------------------------------------------------------------------
setlocal
cd /d "%~dp0"
set "MARCO_HOME=%LOCALAPPDATA%\marco-dogfood"
set "MARCO_BIN=%CD%\marco.exe"
if not exist "%MARCO_HOME%" mkdir "%MARCO_HOME%"

set "VIEW=%*"
set "DIDRESET=no"
set "CHECKONLY=no"

if /i "%~1"=="--check" set "CHECKONLY=yes"
if /i "%~1"=="--check" set "VIEW="

if /i "%~1"=="reset" goto :doreset
goto :arm

:doreset
director.exe shutdown >nul 2>&1
director.exe reset-test-state
if errorlevel 1 exit /b 1
rem `shift` does NOT rewrite %*, so the view has to be cleared by hand. Without this,
rem `dogfood.cmd reset` opened the control centre on a view called "reset", which is not a
rem view — it fell back to the default one and looked like it had worked.
set "VIEW="
set "DIDRESET=yes"

:arm
rem WATCHING IS ON BEFORE THE WINDOW OPENS.
rem
rem The harness making its declared precondition true, not a product default: `marco observe
rem learn` is the same command a person would type, and a Director started any other way still
rem comes up idle. Whether the mode should survive a restart is a separate product decision.
marco.exe observe learn >nul 2>&1

set "STATUS=%TEMP%\marco-dogfood-status.txt"
director.exe status > "%STATUS%" 2>&1

set "READY=yes"
set "V_DIRECTOR=NOT RUNNING"
set "V_WATCHING=no"
set "V_LEARNING=no"

findstr /c:"Uptime:" "%STATUS%" >nul 2>&1
if not errorlevel 1 set "V_DIRECTOR=running"
if "%V_DIRECTOR%"=="NOT RUNNING" set "READY=no"

findstr /c:"Watching: yes" "%STATUS%" >nul 2>&1
if not errorlevel 1 set "V_WATCHING=yes"
if "%V_WATCHING%"=="no" set "READY=no"

findstr /c:"Learning: yes" "%STATUS%" >nul 2>&1
if not errorlevel 1 set "V_LEARNING=yes"
if "%V_LEARNING%"=="no" set "READY=no"

set "V_COMMIT=unknown"
for /f "delims=" %%c in ('git rev-parse --short HEAD 2^>nul') do set "V_COMMIT=%%c"
for /f "delims=" %%d in ('git status --porcelain 2^>nul') do set "V_COMMIT=%V_COMMIT%+dirty"

set "V_STORE=empty"
if exist "%MARCO_HOME%\semantic-memory.json" set "V_STORE=holds earlier evidence"
if "%DIDRESET%"=="yes" set "V_STORE=reset"

echo.
if "%READY%"=="no" goto :notready
echo DOGFOOD READY
goto :report

:notready
echo DOGFOOD NOT READY
goto :report

:report
echo.
echo   Director: %V_DIRECTOR%
echo   Watching: %V_WATCHING%
echo   Learning: %V_LEARNING%
echo   Home:     %MARCO_HOME%
echo   Binary:   %V_COMMIT%
echo   Store:    %V_STORE%
echo.

if "%READY%"=="no" goto :fail
if "%CHECKONLY%"=="yes" exit /b 0
marco.exe ui %VIEW%
exit /b 0

:fail
echo   Refusing to open. A run against a Director that is not watching is a run that
echo   proves nothing, and it looks exactly like a run that does.
echo.
echo   Full status is in %STATUS%
echo.
exit /b 1
