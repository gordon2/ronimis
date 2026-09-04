#!/bin/bash
# Install or update the always-on macOS services (collector + dashboard).
#
# macOS blocks launchd agents from reading/running anything in ~/Documents (TCC),
# so the running copy lives in a runtime folder OUTSIDE Documents. This repo stays
# the source of truth; re-run this script any time after editing it to push the
# changes to the running copy. Data CSVs in the runtime folder are left untouched.
set -e
cd "$(dirname "$0")"

RT="$HOME/Library/Application Support/ronimis"
AGENTS="$HOME/Library/LaunchAgents"
LOGS="$HOME/Library/Logs"
PORT="8822"   # single source of truth: the server plist below and the smoke test both use it
mkdir -p "$RT" "$AGENTS" "$LOGS"

echo "Building gym-server..."
go build -o gym-server server.go

echo "Copying code + config to runtime ($RT)..."
# Keep the currently-deployed binary so a failed smoke test can roll back to it.
if [ -e "$RT/gym-server" ]; then cp "$RT/gym-server" "$RT/gym-server.prev"; fi
cp gym-server gym-stats-collector.sh gym-config.env dashboard.html busyness.html manifest.json icon.svg icon-192.png icon-512.png backup.sh "$RT"/
chmod +x "$RT/gym-stats-collector.sh" "$RT/backup.sh"

# Seed existing CSVs on first install; never clobber live data on later runs.
for f in gym-stats-*.csv; do [ -e "$RT/$f" ] || cp "$f" "$RT/" 2>/dev/null || true; done

echo "Writing LaunchAgents..."
cat > "$AGENTS/com.ronimis.gym-stats-collector.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>com.ronimis.gym-stats-collector</string>
    <key>ProgramArguments</key><array><string>$RT/gym-stats-collector.sh</string></array>
    <key>WorkingDirectory</key><string>$RT</string>
    <key>EnvironmentVariables</key><dict><key>PATH</key><string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string></dict>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>StandardOutPath</key><string>$LOGS/ronimis-gym-collector.log</string>
    <key>StandardErrorPath</key><string>$LOGS/ronimis-gym-collector.log</string>
</dict>
</plist>
EOF

cat > "$AGENTS/com.ronimis.gym-server.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>com.ronimis.gym-server</string>
    <key>ProgramArguments</key><array><string>$RT/gym-server</string><string>$PORT</string></array>
    <key>WorkingDirectory</key><string>$RT</string>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>StandardOutPath</key><string>$LOGS/ronimis-gym-server.log</string>
    <key>StandardErrorPath</key><string>$LOGS/ronimis-gym-server.log</string>
</dict>
</plist>
EOF

cat > "$AGENTS/com.ronimis.gym-backup.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>com.ronimis.gym-backup</string>
    <key>ProgramArguments</key><array><string>$RT/backup.sh</string></array>
    <key>WorkingDirectory</key><string>$RT</string>
    <key>EnvironmentVariables</key><dict><key>PATH</key><string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string></dict>
    <key>RunAtLoad</key><true/>
    <key>StartCalendarInterval</key><dict><key>Hour</key><integer>3</integer><key>Minute</key><integer>30</integer></dict>
    <key>StandardOutPath</key><string>$LOGS/ronimis-gym-backup.log</string>
    <key>StandardErrorPath</key><string>$LOGS/ronimis-gym-backup.log</string>
</dict>
</plist>
EOF

echo "(Re)loading services..."
for L in com.ronimis.gym-stats-collector com.ronimis.gym-server com.ronimis.gym-backup; do
  launchctl bootout "gui/$(id -u)/$L" 2>/dev/null || true
  launchctl bootstrap "gui/$(id -u)" "$AGENTS/$L.plist"
done

# Smoke test: a wrong binary (e.g. one lacking these routes) or a crash-loop is
# caught here at deploy time instead of surfacing later as a dead dashboard.
echo "Smoke-testing http://localhost:$PORT ..."
sc=""; bc=""
for _ in $(seq 1 15); do
  sc=$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 "http://localhost:$PORT/status" || true)
  bc=$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 "http://localhost:$PORT/busyness-data" || true)
  if [ "$sc" = "200" ] && [ "$bc" = "200" ]; then break; fi
  sleep 1
done

if [ "$sc" = "200" ] && [ "$bc" = "200" ]; then
  rm -f "$RT/gym-server.prev"
  echo "Smoke test passed (/status and /busyness-data → 200)."
  echo "Done. Dashboard: http://localhost:$PORT/dashboard.html"
else
  echo "!! SMOKE TEST FAILED (/status=$sc /busyness-data=$bc)" >&2
  if [ -e "$RT/gym-server.prev" ]; then
    cp "$RT/gym-server.prev" "$RT/gym-server"
    launchctl kickstart -k "gui/$(id -u)/com.ronimis.gym-server"
    echo "Rolled back to the previous binary and restarted the service." >&2
  else
    echo "No previous binary to roll back to (first install?) — check $LOGS/ronimis-gym-server.log" >&2
  fi
  exit 1
fi
