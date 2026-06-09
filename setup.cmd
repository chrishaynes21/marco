@echo off
rem Wrapper so setup.ps1 runs regardless of PowerShell's execution policy.
rem   Usage:  setup.cmd            (core only)
rem           setup.cmd -Voice     (+ offline voice)
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0setup.ps1" %*
