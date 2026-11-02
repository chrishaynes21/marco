#!/bin/sh
# usage: render.sh <page> <suffix>  — renders every state at normal and narrow width
page=$1; sfx=$2
for s in states/*.js; do n=$(basename $s .js)
  echo "$n normal"; echo "$n narrow"
done | xargs -P 6 -L 1 sh -c '
  n=$0; w=$1
  if [ "$w" = normal ]; then W=1180; H=1000; else W=640; H=1100; fi
  ./h.exe -page "'"$page"'" -dir . -state states/$n.js -out out/$n-$w'"$sfx"'.png -audit out/$n-$w'"$sfx"'.json -w $W -h $H || echo FAIL $n $w'
