# Builds plugins/uia/uia.exe.
#
# Deliberately uses the .NET Framework compiler that ships with Windows rather than
# the .NET SDK: no install, no NuGet restore, no SDK version to keep in step, and the
# result is a single self-contained exe that runs on any Windows machine - which is
# how Marco's other plugin binaries are distributed.
#
# The UI Automation assemblies (UIAutomationClient, UIAutomationTypes) are part of
# the .NET Framework and are already in the GAC on every Windows install.

$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path

$csc = Join-Path $env:WINDIR 'Microsoft.NET\Framework64\v4.0.30319\csc.exe'
if (-not (Test-Path $csc)) {
    $csc = Join-Path $env:WINDIR 'Microsoft.NET\Framework\v4.0.30319\csc.exe'
}
if (-not (Test-Path $csc)) {
    throw "csc.exe not found. Expected the .NET Framework 4 compiler under $env:WINDIR\Microsoft.NET."
}

$sources = @('Program.cs', 'Uia.cs', 'Shell.cs', 'Value.cs', 'Json.cs') | ForEach-Object { Join-Path $here $_ }
$out = Join-Path $here 'uia.exe'

# The WPF/UIA assemblies are not next to csc.exe and there is no targeting pack on a
# machine without Visual Studio, so resolve them out of the GAC by full path. Every
# Windows install has them; only their version-stamped folder name varies.
function Resolve-GacAssembly([string]$name) {
    $gac = Join-Path $env:WINDIR 'Microsoft.NET\assembly'
    $hit = Get-ChildItem -Path $gac -Recurse -Filter "$name.dll" -ErrorAction SilentlyContinue |
           Where-Object { $_.DirectoryName -match 'v4\.0_' } |
           Sort-Object FullName -Descending |
           Select-Object -First 1
    if (-not $hit) { throw "could not find $name.dll in the GAC under $gac" }
    return $hit.FullName
}

$refs = @('System.dll', 'System.Core.dll') | ForEach-Object { "/reference:$_" }
$refs += @('UIAutomationClient', 'UIAutomationTypes', 'WindowsBase') |
         ForEach-Object { "/reference:$(Resolve-GacAssembly $_)" }

Write-Host "building $out"
& $csc /nologo /target:exe /platform:x64 /optimize+ /langversion:5 `
    "/out:$out" $refs $sources
if ($LASTEXITCODE -ne 0) { throw "csc failed with exit code $LASTEXITCODE" }

Write-Host "built $out"
