# Records perception fixtures for the Director's tests.
#
# Fixtures are the point at which Director development stops needing a live desktop.
# Everything downstream of perception -- fusion, identity, target ranking, planning,
# verification -- is developed and tested against these JSON files, which means the
# test suite is deterministic, runs in CI, and never clicks anything on the author's
# machine. See ANALYSIS.md step 4 of the vertical-slice build order.
#
# The dialogs here are BUILT rather than borrowed, for two reasons: a real
# application's tree changes with its version and theme (so a fixture recorded today
# fails next month for no useful reason), and the cases that matter most -- two
# controls with the same label, a disabled button, an offscreen element -- are
# awkward to produce on demand in a real app. These are real UI Automation trees
# walked out of real windows; only the window's content is under our control.
#
# Real-application captures are complementary, not replaced. Record one with:
#   .\uia.exe snapshot ..\..\fixtures\<name>\accessibility.json --delay 5000
# then bring the window you want to the front.

$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$uia = Join-Path $here 'uia.exe'
$fixtures = Join-Path $here '..\..\fixtures'

if (-not (Test-Path $uia)) { throw "uia.exe not built -- run build.ps1 first" }

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

# Capture writes the fixture for one built form.
#
# The retry loop is not defensive padding. A form's UI Automation provider publishes
# its child controls ASYNCHRONOUSLY after the window appears, and the first capture
# in a session additionally pays UIA's COM initialisation and the CLR's JIT. Without
# waiting for the tree to stop growing, the first fixture records the window chrome
# and none of the controls -- a fixture that looks plausible, is silently wrong, and
# would send every downstream test chasing a bug that isn't there.
function Capture([string]$name, [System.Windows.Forms.Form]$form) {
    $dir = Join-Path $fixtures $name
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $out = Join-Path $dir 'accessibility.json'

    $form.StartPosition = 'Manual'
    $form.Location = New-Object System.Drawing.Point(400, 300)
    $form.Show()
    $form.Refresh()
    [System.Windows.Forms.Application]::DoEvents()

    # Capture until the element count repeats -- the tree has settled.
    $last = -1
    $stable = 0
    for ($i = 0; $i -lt 12 -and $stable -lt 2; $i++) {
        Start-Sleep -Milliseconds 250
        [System.Windows.Forms.Application]::DoEvents()
        & $uia snapshot $out --window $form.Handle.ToInt64() --max-nodes 400 --quiet
        $n = ((Get-Content $out -Raw | ConvertFrom-Json).Elements).Count
        if ($n -eq $last -and $n -gt 0) { $stable++ } else { $stable = 0 }
        $last = $n
    }

    Write-Host ("  {0}: {1} elements" -f $name, $last)
    $form.Close()
    $form.Dispose()
}

# Warm-up: absorb UIA COM initialisation and JIT on a throwaway window so the first
# real fixture isn't the one that pays for them.
$warm = New-Object System.Windows.Forms.Form
$warm.Text = 'warmup'
$warm.Show(); [System.Windows.Forms.Application]::DoEvents()
Start-Sleep -Milliseconds 300
& $uia snapshot ([System.IO.Path]::GetTempFileName()) --window $warm.Handle.ToInt64() --quiet
$warm.Close(); $warm.Dispose()

# -- save-dialog: the vertical slice's target case ----------------------------
# "Marco, click Save." A Save button, plus the near-misses a naive matcher trips
# over: a "Save As..." menu item that also contains the word Save, and a "Save"
# LABEL that is text rather than a control.
function New-SaveDialog {
    $f = New-Object System.Windows.Forms.Form
    $f.Text = 'Save As'
    $f.Size = New-Object System.Drawing.Size(520, 260)

    $lbl = New-Object System.Windows.Forms.Label
    $lbl.Text = 'Save'                      # decoy: same text, inert role
    $lbl.Location = New-Object System.Drawing.Point(16, 20)
    $lbl.AutoSize = $true
    $f.Controls.Add($lbl)

    $name = New-Object System.Windows.Forms.TextBox
    $name.Text = 'untitled.txt'
    $name.Location = New-Object System.Drawing.Point(16, 48)
    $name.Size = New-Object System.Drawing.Size(360, 24)
    $name.AccessibleName = 'File name'
    $f.Controls.Add($name)

    $save = New-Object System.Windows.Forms.Button
    $save.Text = 'Save'                     # the real target
    $save.Location = New-Object System.Drawing.Point(280, 150)
    $save.Size = New-Object System.Drawing.Size(90, 30)
    $f.Controls.Add($save)

    $cancel = New-Object System.Windows.Forms.Button
    $cancel.Text = 'Cancel'
    $cancel.Location = New-Object System.Drawing.Point(380, 150)
    $cancel.Size = New-Object System.Drawing.Size(90, 30)
    $f.Controls.Add($cancel)

    $menu = New-Object System.Windows.Forms.MenuStrip
    $file = New-Object System.Windows.Forms.ToolStripMenuItem
    $file.Text = 'File'
    [void]$file.DropDownItems.Add('Save As...')   # decoy: contains "Save"
    [void]$file.DropDownItems.Add('Exit')
    [void]$menu.Items.Add($file)
    $f.Controls.Add($menu)

    return $f
}

# -- duplicate-labels: two identical buttons, one in each group ---------------
# Label alone cannot decide this. Ranking has to fall back on structure, and if it
# cannot decide, the Director must ASK rather than pick one.
function New-DuplicateLabels {
    $f = New-Object System.Windows.Forms.Form
    $f.Text = 'Settings'
    $f.Size = New-Object System.Drawing.Size(520, 300)

    $left = New-Object System.Windows.Forms.GroupBox
    $left.Text = 'Audio'
    $left.Location = New-Object System.Drawing.Point(16, 16)
    $left.Size = New-Object System.Drawing.Size(220, 200)
    $b1 = New-Object System.Windows.Forms.Button
    $b1.Text = 'Apply'
    $b1.Location = New-Object System.Drawing.Point(60, 140)
    $b1.Size = New-Object System.Drawing.Size(90, 30)
    $left.Controls.Add($b1)
    $f.Controls.Add($left)

    $right = New-Object System.Windows.Forms.GroupBox
    $right.Text = 'Video'
    $right.Location = New-Object System.Drawing.Point(260, 16)
    $right.Size = New-Object System.Drawing.Size(220, 200)
    $b2 = New-Object System.Windows.Forms.Button
    $b2.Text = 'Apply'                       # identical label, different group
    $b2.Location = New-Object System.Drawing.Point(60, 140)
    $b2.Size = New-Object System.Drawing.Size(90, 30)
    $right.Controls.Add($b2)
    $f.Controls.Add($right)

    return $f
}

# -- disabled-button: the target exists but cannot be used -------------------
# The correct answer is "Save is greyed out", NOT a click that reports success and
# does nothing, and NOT "there is no Save button".
function New-DisabledButton {
    $f = New-Object System.Windows.Forms.Form
    $f.Text = 'Document'
    $f.Size = New-Object System.Drawing.Size(460, 220)

    $save = New-Object System.Windows.Forms.Button
    $save.Text = 'Save'
    $save.Enabled = $false                   # nothing to save yet
    $save.Location = New-Object System.Drawing.Point(40, 100)
    $save.Size = New-Object System.Drawing.Size(90, 30)
    $f.Controls.Add($save)

    $discard = New-Object System.Windows.Forms.Button
    $discard.Text = 'Discard'
    $discard.Location = New-Object System.Drawing.Point(150, 100)
    $discard.Size = New-Object System.Drawing.Size(90, 30)
    $f.Controls.Add($discard)

    $chk = New-Object System.Windows.Forms.CheckBox
    $chk.Text = 'Autosave'
    $chk.Checked = $true
    $chk.Location = New-Object System.Drawing.Point(40, 50)
    $chk.AutoSize = $true
    $f.Controls.Add($chk)

    return $f
}

Capture 'save-dialog'      (New-SaveDialog)
Capture 'duplicate-labels' (New-DuplicateLabels)
Capture 'disabled-button'  (New-DisabledButton)

Write-Host "fixtures written under $fixtures"
