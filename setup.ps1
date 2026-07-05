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
               and tells you how to get gcc. Once built, voice is ON BY DEFAULT
               in overlay.cmd on every re-run (toggle it live with `m voice
               on|off, or bake it off with -NoVoice).
    -NoVoice   leave voice out of overlay.cmd even if voice.exe is built.
    -WebUI     the web control panel (plugins/web-ui)
    -Resolver  the Claude NL resolver plugin (plugins/claude-resolver)
    -OCR       the text/OCR anchor resolver (plugins/ocr). Builds ocr.exe AND
               installs the tesseract OCR engine it needs (via winget, or a
               -TesseractUrl silent installer), then pins MARCO_TESSERACT.
    -Vision    the SEMANTIC UI-element resolver (plugins/vision). KITCHEN SINK:
               auto-provisions the REAL detector — downloads onnxruntime, downloads
               + exports an OmniParser ONNX model (needs Python+ultralytics; or pass
               -VisionModel <model.onnx>), builds with -tags onnxvision (needs gcc),
               and pins MARCO_VISION/MARCO_ONNXRUNTIME/MARCO_VISION_MODEL/_LABELS.
               Any missing piece → the dependency-free null detector (re-run later).
               Override: -OnnxRuntimeVersion, -VisionModelUrl, -VisionLabels.
    -CV        KITCHEN SINK: pin MARCO_CV=max so every demonstrated click becomes a
               multi-signal CV anchor (image+edge+colour+OCR, +Vision if a model is
               wired), every button, even non-distinctive patches. Implies -OCR. The
               one switch to A/B test the whole CV stack.
    -NoCV      the other side of the A/B: pin MARCO_CV=off (record plain coordinates,
               no anchors, no resolvers).

  Re-run any time; downloads are skipped if present (use -Force to refetch).

.EXAMPLE
  .\setup.ps1                 # core only
  .\setup.ps1 -Voice          # core + offline voice (downloads libvosk + model)
  .\setup.ps1 -Voice -WebUI   # core + voice + web UI
  .\setup.ps1 -OCR            # core + text/OCR resolver (installs tesseract)
  .\setup.ps1 -CV             # kitchen-sink CV: every click a multi-signal anchor
  .\setup.ps1 -CV -Vision     # ...plus auto-provision the semantic detector (downloads)
  .\setup.ps1 -NoCV           # plain coordinates (the A/B baseline)
#>
[CmdletBinding()]
param(
    [switch]$Voice,
    [switch]$NoVoice,
    [switch]$WebUI,
    [switch]$Resolver,
    [switch]$OCR,
    [switch]$Vision,
    [switch]$CV,
    [switch]$NoCV,
    [switch]$Force,
    [string]$Model = "vosk-model-small-en-us-0.15",
    [string]$LibVoskVersion = "0.3.45",
    [string]$Wake = "marco",
    [string]$ApiKey = "",
    [string]$TesseractUrl = "",
    [string]$OnnxRuntime = "",
    [string]$VisionModel = "",
    [string]$OnnxRuntimeVersion = "1.26.0",
    [string]$VisionModelUrl = "https://huggingface.co/microsoft/OmniParser-v2.0/resolve/main/icon_detect/model.pt",
    [string]$VisionLabels = "icon"
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

# Find-Tesseract returns the path to tesseract.exe — on PATH, or in the standard
# install locations — or "" if it isn't installed.
function Find-Tesseract {
    $cmd = Get-Command tesseract -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $dirs = @($env:ProgramFiles, [Environment]::GetEnvironmentVariable("ProgramFiles(x86)"))
    foreach ($d in $dirs) {
        if ($d) {
            $p = Join-Path $d "Tesseract-OCR\tesseract.exe"
            if (Test-Path $p) { return $p }
        }
    }
    return ""
}

# Install-Tesseract ensures the OCR engine is present and returns its path ("" if it
# couldn't install it). Prefers an existing install; then a -TesseractUrl silent
# installer (UB-Mannheim NSIS, supports /S); then winget. Either install path may
# prompt for elevation (it installs into Program Files).
function Install-Tesseract {
    $found = Find-Tesseract
    if ($found -ne "") { Info "have tesseract ($found)"; return $found }

    if ($TesseractUrl -ne "") {
        $exe = Join-Path $env:TEMP "tesseract-setup.exe"
        Download $TesseractUrl $exe
        Info "installing tesseract (silent)..."
        Start-Process -FilePath $exe -ArgumentList "/S" -Wait
    } elseif (Get-Command winget -ErrorAction SilentlyContinue) {
        Info "installing tesseract via winget (UB-Mannheim.TesseractOCR)..."
        # winget returns non-zero if it's already installed; that's not an error here.
        & winget install -e --id UB-Mannheim.TesseractOCR `
            --accept-package-agreements --accept-source-agreements --disable-interactivity
    } else {
        Warn "can't auto-install tesseract: no winget and no -TesseractUrl. Install it with"
        Warn "  winget install UB-Mannheim.TesseractOCR"
        Warn "  (or pass -TesseractUrl <installer.exe>), then re-run .\setup.ps1 -OCR."
        return ""
    }
    return Find-Tesseract
}

# Install-OnnxRuntime downloads the ONNX Runtime Windows shared library and returns the
# path to onnxruntime.dll (cached under _dl; skipped if present). "" on failure.
function Install-OnnxRuntime {
    $dl = Join-Path $root "_dl"
    New-Item -ItemType Directory -Force -Path $dl | Out-Null
    $name = "onnxruntime-win-x64-$OnnxRuntimeVersion"
    $dir = Join-Path $dl $name
    $dll = Join-Path $dir "lib\onnxruntime.dll"
    if ((Test-Path $dll) -and -not $Force) { Info "have onnxruntime ($OnnxRuntimeVersion)"; return $dll }
    $zip = Join-Path $dl "$name.zip"
    $url = "https://github.com/microsoft/onnxruntime/releases/download/v$OnnxRuntimeVersion/$name.zip"
    try {
        Info "downloading onnxruntime $OnnxRuntimeVersion..."
        Download $url $zip
        Expand-Archive -Path $zip -DestinationPath $dl -Force
    } catch { Warn "onnxruntime download failed: $($_.Exception.Message)"; return "" }
    if (Test-Path $dll) { Ok "onnxruntime -> $dll"; return $dll }
    Warn "onnxruntime.dll not found after extract (zip layout changed?)"
    return ""
}

# Find-Python returns a real CPython interpreter that has ultralytics importable (for the
# .pt -> .onnx export), preferring a standard install over the Microsoft Store execution
# alias (WindowsApps\python.exe) — that alias shadows `python` on PATH but can't run
# ultralytics. "" if none usable.
function Find-Python {
    $cands = New-Object System.Collections.ArrayList
    $base = Join-Path $env:LOCALAPPDATA "Programs\Python"
    if (Test-Path $base) {
        Get-ChildItem $base -Directory -Filter "Python3*" -ErrorAction SilentlyContinue |
            Sort-Object Name -Descending |
            ForEach-Object { [void]$cands.Add((Join-Path $_.FullName "python.exe")) }
    }
    foreach ($d in @("C:\Python314", "C:\Python313", "C:\Python312")) {
        [void]$cands.Add((Join-Path $d "python.exe"))
    }
    $cmd = Get-Command python -ErrorAction SilentlyContinue
    if ($cmd -and $cmd.Source -notlike "*WindowsApps*") { [void]$cands.Add($cmd.Source) }
    foreach ($p in $cands) {
        if (Test-Path $p) {
            try { & $p -c "import ultralytics" 2>$null } catch {}
            if ($LASTEXITCODE -eq 0) { return $p }
        }
    }
    return ""
}

# Get-VisionModel resolves an .onnx detector: an explicit -VisionModel wins; otherwise it
# downloads the OmniParser icon_detect weights (.pt) and EXPORTS them to ONNX with
# ultralytics (Python) when available. Returns the .onnx path, or "" if it can't (then the
# null detector is built and the user supplies a model later). Model is the operator's
# artifact (BYO) — its license is theirs; setup just fetches what's pointed at.
function Get-VisionModel {
    if ($VisionModel -ne "") {
        if (-not (Test-Path $VisionModel)) { throw "VisionModel not found: $VisionModel" }
        return (Resolve-Path $VisionModel).Path
    }
    $models = Join-Path $root "plugins\vision\models"
    New-Item -ItemType Directory -Force -Path $models | Out-Null
    $onnx = Join-Path $models "icon_detect.onnx"
    if ((Test-Path $onnx) -and -not $Force) { Info "have vision model (icon_detect.onnx)"; return $onnx }
    $pt = Join-Path $models "icon_detect.pt"
    try { Info "downloading OmniParser icon_detect weights..."; Download $VisionModelUrl $pt }
    catch { Warn "model download failed: $($_.Exception.Message)"; return "" }
    # Export .pt -> .onnx via a real ultralytics-equipped Python (NOT the Store alias).
    $pyExe = Find-Python
    if ($pyExe -eq "") {
        Warn "downloaded $pt but found no Python with ultralytics to export it (the Store"
        Warn "  'python' alias won't work). Install: winget install Python.Python.3.13;"
        Warn "  <python> -m pip install ultralytics. Then export:"
        Warn "  yolo export model=`"$pt`" format=onnx imgsz=640 opset=12  (or pass -VisionModel <onnx>)"
        return ""
    }
    Info "exporting model to ONNX (ultralytics: $pyExe)..."
    & $pyExe -c "from ultralytics import YOLO; YOLO(r'$pt').export(format='onnx', imgsz=640, opset=12)" 2>&1 | Out-Host
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $onnx)) {
        Warn "ONNX export failed. Export manually:"
        Warn "  & `"$pyExe`" -m ultralytics  # or: yolo export model=`"$pt`" format=onnx imgsz=640 opset=12"
        return ""
    }
    Ok "vision model -> $onnx"
    return $onnx
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

# --- OCR text resolver (optional) -------------------------------------------
$ocrReady = $false
$tesseractExe = ""
if ($OCR -or $CV) {
    Step "OCR text resolver"
    BuildMod "plugins\ocr" "ocr.exe"
    $ocrReady = $true
    # The resolver shells out to the tesseract CLI (cross-platform). Install it now so
    # text anchors work out of the box; if we can't, they degrade to the recorded
    # coordinate until it's installed.
    $tesseractExe = Install-Tesseract
    if ($tesseractExe -ne "") {
        Ok "tesseract: $tesseractExe"
    } else {
        Warn "ocr.exe built, but tesseract isn't installed - text anchors fall back for now."
    }
}

# --- vision resolver (optional) ---------------------------------------------
$visionReady = $false     # vision.exe exists (at least the null detector)
$visionModel = ""         # path pinned only when the real detector is wired
$visionLabels = ""        # class names for the wired model
if ($Vision) {
    Step "Vision (semantic UI-element resolver)"
    $visionDir = Join-Path $root "plugins\vision"
    # Kitchen sink: try to provision the WHOLE real detector automatically — the ONNX
    # runtime lib, a model (download + export, or BYO via -VisionModel), and the cgo build.
    # Any missing piece falls back to the dependency-free null detector (declines), so this
    # never hard-fails; you can supply what's missing and re-run.
    $ortPath = if ($OnnxRuntime -ne "") { (Resolve-Path $OnnxRuntime).Path } else { Install-OnnxRuntime }
    $onnxModel = try { Get-VisionModel } catch { Warn $_.Exception.Message; "" }
    $haveGcc = [bool](Get-Command gcc -ErrorAction SilentlyContinue)

    if (($ortPath -ne "") -and ($onnxModel -ne "") -and $haveGcc) {
        Info "fetching onnxruntime_go binding..."
        & go -C $visionDir get github.com/yalue/onnxruntime_go
        if ($LASTEXITCODE -ne 0) { throw "go get onnxruntime_go failed" }
        $env:CGO_ENABLED = "1"
        & go -C $visionDir build -tags onnxvision -o vision.exe .
        $rc = $LASTEXITCODE
        $env:CGO_ENABLED = ""
        if ($rc -ne 0) { throw "vision (onnxvision) build failed" }
        $visionModel = $onnxModel
        $visionLabels = $VisionLabels
        $script:OnnxRuntimePath = $ortPath
        Ok "built vision.exe (REAL detector) - model $visionModel"
        Ok "spike it:  .\marco.exe vision detect <screenshot.png>"
    } else {
        # Couldn't assemble the full stack — build the null detector and say what's missing.
        BuildMod "plugins\vision" "vision.exe"
        if (-not $haveGcc) { Warn "no gcc on PATH - the real detector needs cgo (e.g. 'scoop install mingw')." }
        if ($ortPath -eq "") { Warn "no onnxruntime.dll (download failed or none supplied)." }
        if ($onnxModel -eq "") { Warn "no .onnx model (supply -VisionModel, or install Python+ultralytics to auto-export)." }
        Warn "built vision.exe with the NULL detector (declines until the above are resolved); re-run -Vision."
    }
    $visionReady = $true
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
# Voice is ON BY DEFAULT once built: emit the prefix whenever voice.exe exists (whether just
# built by -Voice or from a prior run), unless -NoVoice opts out. Mute it live with `m voice off.
$voiceExe = Join-Path $root "plugins\voice\voice.exe"
if ((Test-Path $voiceExe) -and -not $NoVoice) {
    $voicePrefix = "plugins\voice\voice.exe --model plugins\voice\model --wake `"$Wake`" | "
}
$lines = [System.Collections.ArrayList]@(
    '@echo off',
    'rem Generated by setup.ps1 - launches the Marco overlay stack.',
    'setlocal',
    'cd /d "%~dp0"',
    'set "MARCO_BIN=%CD%\marco.exe"'
)
# The CV master switch (A/B the whole anchor stack). -CV = kitchen sink, -NoCV = off.
if ($CV) {
    [void]$lines.Add('set "MARCO_CV=max"')
} elseif ($NoCV) {
    [void]$lines.Add('set "MARCO_CV=off"')
}
if ($resolverReady) {
    # Claude resolves loose phrasing the local matcher misses (needs ANTHROPIC_API_KEY).
    [void]$lines.Add('set "MARCO_RESOLVER=%CD%\plugins\claude-resolver\claude-resolver.exe"')
}
if ($ocrReady) {
    # The OCR resolver fulfils a route's text anchor (do Text's Find). Spawned `marco
    # do` reads MARCO_OCR and launches it lazily, so it costs nothing until a text
    # anchor runs.
    [void]$lines.Add('set "MARCO_OCR=%CD%\plugins\ocr\ocr.exe"')
    # Pin the tesseract path we found/installed, so it works even before a PATH refresh
    # (a fresh install isn't on this session's PATH yet). The plugin prefers this.
    if ($tesseractExe -ne "") {
        [void]$lines.Add("set `"MARCO_TESSERACT=$tesseractExe`"")
    }
}
if ($visionReady) {
    # The vision resolver fulfils a route's Vision's Locate/Detect. Spawned `marco do`
    # reads MARCO_VISION and launches it lazily. With no model pinned it's the null
    # detector (declines), so this is safe to set either way.
    [void]$lines.Add('set "MARCO_VISION=%CD%\plugins\vision\vision.exe"')
    if ($visionModel -ne "") {
        [void]$lines.Add("set `"MARCO_VISION_MODEL=$visionModel`"")
        [void]$lines.Add("set `"MARCO_ONNXRUNTIME=$($script:OnnxRuntimePath)`"")
        if ($visionLabels -ne "") {
            [void]$lines.Add("set `"MARCO_VISION_LABELS=$visionLabels`"")
        }
    }
}
[void]$lines.Add($voicePrefix + 'marco.exe serve ^')
[void]$lines.Add('  --host "OS=bridge:%CD%\marco-macros.exe" ^')
if ($visionReady) {
    [void]$lines.Add('  --host "Vision=bridge:%CD%\plugins\vision\vision.exe" ^')
}
[void]$lines.Add('  --host "Overlay=bridge:%CD%\plugins\overlay\overlay.exe" ^')
[void]$lines.Add('  plugins\overlay\overlay.marco %*')
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
