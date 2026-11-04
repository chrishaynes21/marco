#!/bin/sh
# Re-runs the S5 control-centre check (ADR-300). EVIDENCE, NOT A REGRESSION TEST: nothing runs
# this automatically and nothing fails if it is not run.
#
# It takes the Config view's script VERBATIM from cmd/marco/edit.go (from `const BT=` to the line
# before `// ---- edit view ----`), loads it beside harness.html with `fetch` stubbed, and prints
# what the page shows and what it posts. Windows, headless Edge. Run from the repository root:
#   sh docs/experiments/S5-control-centre-headless-check/run.sh
set -e
here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
awk '/^const BT=String.fromCharCode\(96\)/{on=1} /^\/\/ ---- edit view ----/{on=0} on' cmd/marco/edit.go > "$work/page.js"
cp "$here/harness.html" "$work/harness.html"
edge="/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe"
[ -x "$edge" ] || edge="/c/Program Files/Microsoft/Edge/Application/msedge.exe"
"$edge" --headless=new --disable-gpu --allow-file-access-from-files --virtual-time-budget=3000 \
  --dump-dom "file:///$(cygpath -m "$work" 2>/dev/null || echo "$work")/harness.html" 2>/dev/null |
  sed -n 's/.*<pre id="out">\([^<]*\).*/\1/p'
rm -rf "$work"
