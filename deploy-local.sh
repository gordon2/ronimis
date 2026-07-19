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
mkdir -p "$RT" "$AGENTS" "$LOGS"

echo "Building gym-server..."
go build -o gym-server server.go

echo "Copying code + config to runtime ($RT)..."
cp gym-server gym-stats-collector.sh gym-config.env dashboard.html busyness.html manifest.json icon.svg icon-192.png icon-512.png "$RT"/
chmod +x "$RT/gym-stats-collector.sh"

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
    <key>ProgramArguments</key><array><string>$RT/gym-server</string><string>8002</string></array>
    <key>WorkingDirectory</key><string>$RT</string>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>StandardOutPath</key><string>$LOGS/ronimis-gym-server.log</string>
    <key>StandardErrorPath</key><string>$LOGS/ronimis-gym-server.log</string>
</dict>
</plist>
EOF

echo "(Re)loading services..."
for L in com.ronimis.gym-stats-collector com.ronimis.gym-server; do
  launchctl bootout "gui/$(id -u)/$L" 2>/dev/null || true
  launchctl bootstrap "gui/$(id -u)" "$AGENTS/$L.plist"
done

echo "Done. Dashboard: http://localhost:8002/dashboard.html"
