@echo off
rem Build the vision plugin WITH the ONNX backend.
rem
rem   plugins\vision\build.cmd
rem
rem ---------------------------------------------------------------------------
rem WHY THIS EXISTS
rem
rem `go build` in this directory produces a working binary that can detect nothing.
rem `backend_null.go` is the default build and `backend_onnx.go` is behind `-tags onnxvision`,
rem so the ordinary command silently yields a detector whose every answer is "no model loaded".
rem That is what was sitting in the tree: a vision.exe, present and current and inert, while
rem `director vision` reported "available no" and read as a missing subsystem.
rem
rem The tag needs cgo and therefore a C compiler. The binding loads the ONNX Runtime shared
rem library DYNAMICALLY, so no onnxruntime headers are needed here — only gcc.
rem
rem If this refuses for want of a compiler, install one:
rem   winget install BrechtSanders.WinLibs.POSIX.UCRT
rem ---------------------------------------------------------------------------
setlocal
cd /d "%~dp0"

where gcc >nul 2>&1
if errorlevel 1 (
  echo vision: no C compiler on PATH.
  echo         The ONNX backend needs cgo. Install one with:
  echo           winget install BrechtSanders.WinLibs.POSIX.UCRT
  echo         then reopen your shell so gcc is on PATH.
  exit /b 1
)

set CGO_ENABLED=1
go build -tags onnxvision -o vision.exe .
if errorlevel 1 (
  echo vision: build failed.
  exit /b 1
)
echo vision: built vision.exe with the ONNX backend
