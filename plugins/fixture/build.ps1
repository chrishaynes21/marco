# Builds plugins/fixture/fixture.exe — the deterministic desktop Marco is tested against.
#
# The same compiler plugins/uia uses, for the same reasons: no SDK, no NuGet restore, no SDK
# version to keep in step, and a single self-contained exe that runs on any Windows machine.
# WinForms and Drawing are part of the .NET Framework and are already present on every Windows
# install, so unlike the UIA assemblies they sit next to csc.exe and need no GAC lookup.

$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path

$csc = Join-Path $env:WINDIR 'Microsoft.NET\Framework64\v4.0.30319\csc.exe'
if (-not (Test-Path $csc)) {
    $csc = Join-Path $env:WINDIR 'Microsoft.NET\Framework\v4.0.30319\csc.exe'
}
if (-not (Test-Path $csc)) {
    throw "csc.exe not found. Expected the .NET Framework 4 compiler under $env:WINDIR\Microsoft.NET."
}

$out = Join-Path $here 'fixture.exe'
$refs = @('System.dll', 'System.Drawing.dll', 'System.Windows.Forms.dll') |
        ForEach-Object { "/reference:$_" }

Write-Host "building $out"
# /target:winexe, not exe: a console window appearing beside the fixture would be a second window
# in the foreground, and the acceptance asserts which window leads.
& $csc /nologo /target:winexe /platform:x64 /optimize+ /langversion:5 `
    "/out:$out" $refs (Join-Path $here 'Fixture.cs')
if ($LASTEXITCODE -ne 0) { throw "csc failed with exit code $LASTEXITCODE" }

Write-Host "built $out"
