@echo off
rem Marco dogfood — the control centre, against an ISOLATED semantic memory.
rem
rem One command, one window. MARCO_HOME points at a throwaway store so the graph can be
rem inspected, reset and destroyed as often as an experiment needs, without touching the real
rem one at %APPDATA%\marco. Everything inside it is ordinary production behaviour: there is no
rem sandbox-only semantic path, and `director reset-test-state` refuses any home that is the
rem default one.
rem
rem   dogfood.cmd          open the control centre
rem   dogfood.cmd reset    empty the dogfood store first, then open it
setlocal
cd /d "%~dp0"
set "MARCO_HOME=%LOCALAPPDATA%\marco-dogfood"
set "MARCO_BIN=%CD%\marco.exe"
if not exist "%MARCO_HOME%" mkdir "%MARCO_HOME%"
if /i "%~1"=="reset" (
  director.exe shutdown >nul 2>&1
  director.exe reset-test-state || exit /b 1
  shift
)
marco.exe ui %*
