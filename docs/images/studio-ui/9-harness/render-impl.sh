#!/bin/sh
# render-impl.sh <outdir> <theme> <states...>
out=$1; theme=$2; shift 2; mkdir -p $out
for s in "$@"; do n=$(basename $s .js); echo "$s $n normal"; echo "$s $n narrow"; done | xargs -P 6 -L 1 sh -c '
  s=$0; n=$1; w=$2
  if [ "$w" = normal ]; then W=1180; H=1000; else W=640; H=1100; fi
  ./h.exe -dir . -theme '"$theme"' -state $s -out '"$out"'/$n-$w.png -audit '"$out"'/$n-$w.json -w $W -h $H || echo FAIL $n $w'
