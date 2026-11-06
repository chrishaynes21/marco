@echo off
rem Build Studio WITH the Wails build tags, and the whole widget stack beside it, from ONE commit.
rem
rem   plugins\studio\build.cmd
rem
rem Close Studio and the widget first, and commit (or set aside) any change: the script refuses
rem otherwise, and says which.
rem
rem ---------------------------------------------------------------------------
rem WHAT IT MAKES, AND WHERE
rem
rem   plugins\studio\   studio.exe  studio-console.exe  director.exe  marco.exe
rem   repo root\        studio.exe  director.exe  marco.exe  marco-macros.exe
rem   plugins\overlay\  overlay.exe          (overlay.marco is source, already there)
rem   plugins\llama\    llama.exe            (the local resolver; Director finds it beside marco)
rem
rem Studio can be opened from either studio.exe. Open widget (internal/widgetstack) starts the
rem ROOT stack from either one, and refuses unless every piece carries the same build revision as
rem the Studio asking. So this script is the only supported way to make that stack: building some
rem of the pieces by hand, or copying one in from elsewhere, is exactly what the check refuses.
rem
rem ---------------------------------------------------------------------------
rem WHY THE WAILS TAGS
rem
rem `go build` in this directory produces a binary that OPENS NO WINDOW. Wails guards on its
rem own build tags: without `production`, `internal/app/app_default_windows.go` is the one that
rem compiles, and its CreateApp shows a message box reading
rem
rem   Wails applications will not build without the correct build tags.
rem
rem and exits. From a terminal that is indistinguishable from a webview that failed to come
rem up, and the binary is otherwise perfectly ordinary: it has the flags, it prints usage, and
rem `-acts` and `-proof` both work, because they return before the app is created.
rem
rem To check a binary WITHOUT opening a window on somebody's desktop, look for the guard's own
rem sentence in it; a correctly built one does not contain that code at all:
rem
rem   grep -a "will not build without the correct build tags" studio.exe
rem
rem Two Studio binaries, because they differ in one flag that cannot be changed at run time:
rem `-H windowsgui` detaches from the console, which is what a window wants and what makes a
rem terminal unable to see a single line of its output.
rem
rem ---------------------------------------------------------------------------
rem WHY THE DIRECTOR AND MARCO BESIDE EACH STUDIO
rem
rem Studio spawns `director.exe` and `marco.exe` from its OWN directory (directorBin, marcoBin),
rem so a studio.exe with a stale sibling records or performs through yesterday's build however
rem current the window is. That cost the owner several recordings. So both directories get both.
rem
rem FROM THE ROOT MODULE, which is not a detail. `go build` stamps the VCS revision into a binary
rem only when the package is in the main module, so a director built from in here could not say
rem what it was -- and saying what it is is the whole point of internal/buildid. The overlay is
rem its own module and is built from it (go -C plugins\overlay), as pack.ps1 builds it.
rem
rem ---------------------------------------------------------------------------
rem WHY ONE COMMIT, AND WHY STAGED
rem
rem A dirty tree stamps every binary `+dirty`, and two dirty builds of different contents carry
rem the same stamp, so the revision check could not tell them apart. The script refuses a tree
rem with any change git reports.
rem
rem Everything is built into a temporary directory first, each stamp is checked against HEAD,
rem and only then is anything put in place. A build that fails part-way therefore replaces
rem nothing. A file held open by a running process cannot be replaced on Windows, so anything
rem running from one of the destinations is named and the script stops before building.
rem ---------------------------------------------------------------------------
setlocal EnableExtensions
cd /d "%~dp0"
set "HERE=%CD%"
pushd "%~dp0..\.."
set "ROOT=%CD%"
popd
set "OVERLAY=%ROOT%\plugins\overlay"
set "LLAMA=%ROOT%\plugins\llama"

rem ONE COMMIT: what HEAD is, and nothing changed on top of it.
set "HEAD="
for /f "delims=" %%h in ('git -C "%ROOT%" rev-parse HEAD 2^>nul') do set "HEAD=%%h"
if not defined HEAD (
  echo studio: %ROOT% is not a git checkout, so nothing built here could say which build it is.
  exit /b 1
)
set "DIRTY="
for /f "delims=" %%l in ('git -C "%ROOT%" status --porcelain 2^>nul') do set "DIRTY=1"
if defined DIRTY (
  echo studio: the tree has changes that are not committed, so the pieces would be stamped +dirty
  echo         and the widget's revision check could not tell one such build from another:
  git -C "%ROOT%" status --short
  echo         Commit them or set them aside, then run this again. Nothing was built.
  exit /b 1
)

rem NOTHING RUNNING FROM A DESTINATION, or its file cannot be replaced and the stack ends up mixed.
powershell -NoProfile -Command "$t = @('%HERE%\studio.exe','%HERE%\studio-console.exe','%HERE%\director.exe','%HERE%\marco.exe','%ROOT%\studio.exe','%ROOT%\director.exe','%ROOT%\marco.exe','%ROOT%\marco-macros.exe','%OVERLAY%\overlay.exe','%LLAMA%\llama.exe'); $r = @(Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.Path -and ($t -contains $_.Path) }); foreach ($p in $r) { Write-Output ('  process ' + $p.Id + '  ' + $p.Path) }; if ($r.Count -gt 0) { exit 1 }"
if errorlevel 1 (
  echo studio: the processes above are running from files this script replaces. Close Studio and
  echo         the widget, or end them, then run this again. Nothing was built.
  exit /b 1
)

set "STAGE=%TEMP%\marco-stack-%HEAD:~0,7%-%RANDOM%"
mkdir "%STAGE%" || exit /b 1
echo studio: building %HEAD:~0,7% into %STAGE%

set TAGS=desktop,production
go build -tags %TAGS% -ldflags "-w -s -H windowsgui" -o "%STAGE%\studio.exe" . || (set "WHAT=studio" & goto :failed)
go build -tags %TAGS% -o "%STAGE%\studio-console.exe" . || (set "WHAT=the console studio" & goto :failed)
pushd "%ROOT%"
go build -o "%STAGE%\director.exe" .\cmd\director || (popd & set "WHAT=director" & goto :failed)
go build -o "%STAGE%\marco.exe" .\cmd\marco || (popd & set "WHAT=marco" & goto :failed)
go build -o "%STAGE%\marco-macros.exe" .\cmd\marco-macros || (popd & set "WHAT=marco-macros" & goto :failed)
go -C plugins\overlay build -o "%STAGE%\overlay.exe" . || (popd & set "WHAT=the overlay" & goto :failed)
go -C plugins\llama build -o "%STAGE%\llama.exe" . || (popd & set "WHAT=the llama resolver" & goto :failed)
popd

rem EVERY STAMP IS HEAD, UNMODIFIED, before anything is put in place.
for %%f in (studio.exe studio-console.exe director.exe marco.exe marco-macros.exe overlay.exe llama.exe) do (
  go version -m "%STAGE%\%%f" | findstr /c:"vcs.revision=%HEAD%" >nul || (set "WHAT=%%f is not stamped %HEAD%" & goto :failed)
  go version -m "%STAGE%\%%f" | findstr /c:"vcs.modified=false" >nul || (set "WHAT=%%f is stamped as modified" & goto :failed)
)

call :put studio.exe "%HERE%" || goto :mixed
call :put studio-console.exe "%HERE%" || goto :mixed
call :put director.exe "%HERE%" || goto :mixed
call :put marco.exe "%HERE%" || goto :mixed
call :put studio.exe "%ROOT%" || goto :mixed
call :put director.exe "%ROOT%" || goto :mixed
call :put marco.exe "%ROOT%" || goto :mixed
call :put marco-macros.exe "%ROOT%" || goto :mixed
call :put overlay.exe "%OVERLAY%" || goto :mixed
call :put llama.exe "%LLAMA%" || goto :mixed
rd /s /q "%STAGE%"

echo studio: built %HEAD:~0,7%, every piece from that one commit:
echo         %HERE%\  studio.exe (windowed), studio-console.exe (keeps stdout), director.exe, marco.exe
echo         %ROOT%\  studio.exe, director.exe, marco.exe, marco-macros.exe
echo         %OVERLAY%\  overlay.exe
echo         %LLAMA%\  llama.exe
exit /b 0

:put
copy /y "%STAGE%\%~1" "%~2\%~1" >nul
if errorlevel 1 (
  set "WHAT=%~2\%~1"
  exit /b 1
)
exit /b 0

:failed
echo studio: %WHAT% failed. Nothing was put in place; the staged builds are removed.
rd /s /q "%STAGE%" 2>nul
exit /b 1

:mixed
echo studio: %WHAT% could not be replaced, and the pieces before it already were. The stack is
echo         now MIXED and Open widget will refuse it. Close whatever holds that file and run this
echo         again. The staged builds are kept in %STAGE%.
exit /b 1
