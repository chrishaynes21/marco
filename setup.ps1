<#
.SYNOPSIS
  Alpha installer for the Marco overlay stack. Builds the layers you choose and
  writes a one-click launcher (overlay.cmd).

.DESCRIPTION
  Always builds the pure-Go core (cgo-free, needs only Go):
    - marco.exe         the headless engine
    - marco-macros.exe  the macros layer (OS effects)
    - overlay.exe       the native gamer-HUD UI layer

  Optional components (flags), so you install only what you want:
    -Voice     offline voice (Vosk). Sets up the external deps: downloads
               libvosk + a model and builds voice.exe with cgo. Needs a C
               compiler (gcc) on PATH; if missing it builds the demo-only voice
               and tells you how to get gcc.
    -WebUI     the web control panel (plugins/web-ui)
    -Resolver  the Claude NL resolver plugin (plugins/claude-resolver)

  Re-run any time; downloads are skipped if present (use -Force to refetch).

.EXAMPLE
  .\setup.ps1                 # core only
  .\setup.ps1 -Voice          # core + offline voice (downloads libvosk + model)
  .\setup.ps1 -Voice -WebUI   # core + voice + web UI
#>
[CmdletBinding()]
param(
    [switch]$Voice,
    [switch]$WebUI,
    [switch]$Resolver,
    [switch]$Force,
    [string]$Model = "vosk-model-small-en-us-0.15",
    [string]$LibVoskVersion = "0.3.45",
    [string]$Wake = "marco",
    [string]$ApiKey = ""
)

$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
$root = $PSScriptRoot
Set-Location $root

function Info($m) { Write-Host "  $m" -ForegroundColor Cyan }
function Ok($m)   { Write-Host "  $m" -ForegroundColor Green }
function Warn($m) { Write-Host "  $m" -ForegroundColor Yellow }
function Step($m) { Write-Host "`n== $m ==" -ForegroundColor White }

function Need($exe, $hint) {
    if (-not (Get-Command $exe -ErrorAction SilentlyContinue)) {
        throw "$exe not found on PATH. $hint"
    }
}

# Build a package in the ROOT module (marco engine + its cmds; cgo-free).
function Build($out, $pkg) {
    $env:CGO_ENABLED = "0"
    & go build -o $out $pkg
    if ($LASTEXITCODE -ne 0) { throw "build failed: $pkg" }
    Ok "built $out"
}

# Build a separate-module plugin (its own go.mod) via `go -C`.
function BuildMod($dir, $out) {
    $env:CGO_ENABLED = "0"
    & go -C $dir build -o $out .
    if ($LASTEXITCODE -ne 0) { throw "build failed: $dir" }
    Ok "built $dir\$out"
}

function Download($url, $dest) {
    if ((Test-Path $dest) -and -not $Force) { Info "have $(Split-Path $dest -Leaf)"; return }
    Info "downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $dest
}

# --- core (always) ----------------------------------------------------------
Step "Core layers"
Need go "Install Go from https://go.dev/dl and reopen the terminal."
Build "marco.exe"         "./cmd/marco"
Build "marco-macros.exe"  "./cmd/marco-macros"
BuildMod "plugins\overlay" "overlay.exe"

if ($WebUI) { Step "Web UI"; BuildMod "plugins\web-ui" "web-ui.exe" }

$resolverReady = $false
if ($Resolver) {
    Step "Resolver (Anthropic NL -> route)"
    BuildMod "plugins\claude-resolver" "claude-resolver.exe"
    $resolverReady = $true
    # The Claude resolver needs ANTHROPIC_API_KEY. Persist it to the user
    # environment (setx) rather than baking the secret into overlay.cmd.
    if ($ApiKey -ne "") {
        & setx ANTHROPIC_API_KEY $ApiKey | Out-Null
        Ok "stored ANTHROPIC_API_KEY (user env)"
    } elseif (-not $env:ANTHROPIC_API_KEY) {
        Warn "no API key: set one with  setx ANTHROPIC_API_KEY sk-...  (or re-run with -ApiKey sk-...)"
    }
}

# --- voice (optional) -------------------------------------------------------
$voiceReady = $false
if ($Voice) {
    Step "Voice (Vosk, offline)"
    $voiceDir = Join-Path $root "plugins\voice"
    $dl = Join-Path $root "_dl"
    New-Item -ItemType Directory -Force -Path $dl | Out-Null

    # model
    $modelDir = Join-Path $voiceDir "model"
    if ((Test-Path $modelDir) -and -not $Force) {
        Info "have model"
    } else {
        $mzip = Join-Path $dl "$Model.zip"
        Download "https://alphacephei.com/vosk/models/$Model.zip" $mzip
        if (Test-Path $modelDir) { Remove-Item -Recurse -Force $modelDir }
        Expand-Archive -Path $mzip -DestinationPath $dl -Force
        Move-Item (Join-Path $dl $Model) $modelDir
        Ok "model -> plugins\voice\model"
    }

    # libvosk (native lib + header)
    if (((Test-Path (Join-Path $voiceDir "libvosk.dll")) -and (Test-Path (Join-Path $voiceDir "vosk_api.h"))) -and -not $Force) {
        Info "have libvosk"
    } else {
        $lname = "vosk-win64-$LibVoskVersion"
        $lzip = Join-Path $dl "$lname.zip"
        Download "https://github.com/alphacep/vosk-api/releases/download/v$LibVoskVersion/$lname.zip" $lzip
        Expand-Archive -Path $lzip -DestinationPath $dl -Force
        $ldir = Join-Path $dl $lname
        Copy-Item (Join-Path $ldir "libvosk.dll") $voiceDir -Force
        Copy-Item (Join-Path $ldir "vosk_api.h") $voiceDir -Force
        # mingw runtime DLLs ship alongside on some releases - copy if present.
        Get-ChildItem $ldir -Filter "*.dll" | ForEach-Object { Copy-Item $_.FullName $voiceDir -Force }
        Ok "libvosk -> plugins\voice"
    }

    # build voice with cgo if a compiler is available; else demo-only fallback.
    if (Get-Command gcc -ErrorAction SilentlyContinue) {
        $env:CGO_ENABLED = "1"   # critical: prior cgo-free builds left this at 0
        $env:CGO_CFLAGS = "-I$voiceDir"
        $env:CGO_LDFLAGS = "-L$voiceDir -lvosk"
        & go -C $voiceDir build -o voice.exe .
        $rc = $LASTEXITCODE
        Remove-Item Env:CGO_CFLAGS, Env:CGO_LDFLAGS -ErrorAction SilentlyContinue
        if ($rc -ne 0) { throw "voice cgo build failed (see errors above)" }
        $voiceReady = $true
        Remove-Item -Recurse -Force $dl -ErrorAction SilentlyContinue # extracted; drop the zips
        Ok "built plugins\voice\voice.exe (real mic)"
    } else {
        $env:CGO_ENABLED = "0"
        & go -C $voiceDir build -o voice.exe .
        Warn "no gcc on PATH -> built DEMO-only voice.exe (use --demo)."
        Warn "for real mic: install mingw-w64 (e.g. 'scoop install mingw') and re-run .\setup.ps1 -Voice"
    }
}

# --- launcher ---------------------------------------------------------------
Step "Launcher"
$env:CGO_ENABLED = ""
$voicePrefix = ""
if ($voiceReady) {
    $voicePrefix = "plugins\voice\voice.exe --model plugins\voice\model --wake `"$Wake`" | "
}
$lines = [System.Collections.ArrayList]@(
    '@echo off',
    'rem Generated by setup.ps1 - launches the Marco overlay stack.',
    'setlocal',
    'cd /d "%~dp0"',
    'set "MARCO_BIN=%CD%\marco.exe"'
)
if ($resolverReady) {
    # Claude resolves loose phrasing the local matcher misses (needs ANTHROPIC_API_KEY).
    [void]$lines.Add('set "MARCO_RESOLVER=%CD%\plugins\claude-resolver\claude-resolver.exe"')
}
[void]$lines.Add($voicePrefix + 'marco.exe serve ^')
[void]$lines.Add('  --host "OS=bridge:%CD%\marco-macros.exe" ^')
[void]$lines.Add('  --host "Overlay=bridge:%CD%\plugins\overlay\overlay.exe" ^')
[void]$lines.Add('  programs\overlay.marco %*')
Set-Content -Path (Join-Path $root "overlay.cmd") -Value $lines -Encoding ascii
Ok "wrote overlay.cmd"

Step "Done"
Write-Host "  Run it:  .\overlay.cmd" -ForegroundColor Green
if ($Voice -and -not $voiceReady) {
    Write-Host "  Voice is demo-only until you install gcc and re-run with -Voice." -ForegroundColor Yellow
} elseif ($voiceReady) {
    Write-Host "  Voice is on - speak after launch; a finished phrase runs as a command." -ForegroundColor Green
} else {
    Write-Host "  Add voice later:  .\setup.ps1 -Voice" -ForegroundColor Cyan
}
