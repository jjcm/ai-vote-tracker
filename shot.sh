#!/usr/bin/env bash
# Development helper: capture full-page screenshots of every route so the
# implementation can be compared against assets/design/.
set -u
OUT=${OUT:-/tmp/shots}
BASE=${BASE:-http://127.0.0.1:8400}
W=${W:-1440}
H=${H:-2400}
mkdir -p "$OUT"
for route in "home:/" "bills:/bills" "alignment:/alignment" "about:/about"; do
  name=${route%%:*}
  path=${route#*:}
  timeout 25 google-chrome --headless=new --disable-gpu --no-sandbox --disable-dev-shm-usage \
    --no-first-run --no-default-browser-check --user-data-dir="/tmp/chrome-prof-$name" \
    --hide-scrollbars --force-device-scale-factor=1 --virtual-time-budget=5000 \
    --window-size="$W,$H" --screenshot="$OUT/$name.png" "$BASE$path" >/dev/null 2>&1
  echo "$OUT/$name.png $(stat -c%s "$OUT/$name.png" 2>/dev/null || echo missing)"
done
