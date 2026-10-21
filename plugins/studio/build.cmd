@echo off
rem Build Studio WITH the Wails build tags.
rem
rem   plugins\studio\build.cmd
rem
rem ---------------------------------------------------------------------------
rem WHY THIS EXISTS
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
rem That is exactly what was sitting in the tree — a current studio.exe, built an hour before,
rem that could only show an error dialog. The owner found it by double-clicking it.
rem
rem To check a binary WITHOUT opening a window on somebody's desktop, look for the guard's own
rem sentence in it; a correctly built one does not contain that code at all:
rem
rem   grep -a "will not build without the correct build tags" studio.exe
rem
rem Two binaries, because they differ in one flag that cannot be changed at run time:
rem `-H windowsgui` detaches from the console, which is what a window wants and what makes a
rem terminal unable to see a single line of its output.
rem ---------------------------------------------------------------------------
setlocal
cd /d "%~dp0"

set TAGS=desktop,production

go build -tags %TAGS% -ldflags "-w -s -H windowsgui" -o studio.exe .
if errorlevel 1 (
  echo studio: build failed.
  exit /b 1
)

go build -tags %TAGS% -o studio-console.exe .
if errorlevel 1 (
  echo studio: the console build failed.
  exit /b 1
)

rem AND THE DIRECTOR BESIDE EACH OF THEM.
rem
rem Studio spawns `director.exe` from its OWN directory (see directorBin), so a studio.exe
rem here with a stale director.exe beside it runs yesterday's recorder however current the
rem window is. That is not hypothetical: it cost the owner several recordings, which reported
rem the old sentences and none of the day's fixes, while every binary looked freshly built --
rem because the fresh one was the OTHER copy, at the repo root.
rem
rem So both directories get both binaries, built from the same source in the same breath.
rem FROM THE ROOT MODULE, which is not a detail. `go build` stamps the VCS revision
rem into a binary only when the package is in the main module, so building the director
rem from in here produced one that could not say what it was -- and saying what it is is
rem the whole point of internal/buildid.
pushd "%~dp0..\.."
go build -o "%~dp0director.exe" .\cmd\director
if errorlevel 1 (
  popd
  echo studio: building the director failed.
  exit /b 1
)
popd

copy /y studio.exe "%~dp0..\..\studio.exe" >nul
copy /y director.exe "%~dp0..\..\director.exe" >nul

echo studio: built studio.exe (windowed), studio-console.exe (keeps stdout) and
echo         director.exe, in this directory and beside marco.exe.
