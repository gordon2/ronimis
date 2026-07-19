# Gym Occupancy Dashboard

A complete gym occupancy monitoring system with data collection, web dashboard, and interactive visualization using Chart.js.

## Quick Start

1. Create `gym-config.env` with authentication tokens:
   - `API_TOKEN` — bearer token for the primary API (`/api/v01/openair/climbers_in_all`, all locations in one request)
   - `PHPSESSID` / `XSRF_TOKEN` / `LARAVEL_SESSION` — cookies for the legacy per-location API, used as fallback when the primary API fails (the fallback keeps the pass-type breakdown in the `response` column)

2. Collect gym occupancy data (loops, appending to daily CSVs):

```bash
./gym-stats-collector.sh
```

3. Build and start the web server:
```bash
go build -o gym-server server.go
./gym-server 8002
```

### Linux server (systemd)
Place the files from directory `services` to `/etc/systemd/system/`

```
sudo systemctl daemon-reload
sudo systemctl restart gym.service gym-stats-collector.service
```

Quick deploy — uploads relevant source files and restarts services:
```
SERVER_IP=<00.00.000.000> SERVER_USER=<username> ./deploy.sh
```

### Always-on on macOS (launchd)
Runs the collector and dashboard as background services that start at login and
auto-restart on crash. macOS blocks launchd from `~/Documents` (TCC protection),
so the running copy lives in a runtime folder outside Documents
(`~/Library/Application Support/ronimis`); this repo stays the source of truth.

Install — or update after editing the repo:
```bash
./deploy-local.sh
```
It builds the server, copies code + config into the runtime folder (data CSVs
there are left untouched), writes the LaunchAgents (collector, dashboard server,
daily backup), and (re)loads them.
Dashboard: http://localhost:8002/dashboard.html

Manage:
```bash
# stop (KeepAlive respawns a plain kill — use bootout)
launchctl bootout gui/$(id -u)/com.ronimis.gym-stats-collector
launchctl bootout gui/$(id -u)/com.ronimis.gym-server
launchctl bootout gui/$(id -u)/com.ronimis.gym-backup
# start / reload
./deploy-local.sh
```
Logs: `~/Library/Logs/ronimis-gym-collector.log`, `ronimis-gym-server.log`, `ronimis-gym-backup.log`

**Keeping it awake:** the agents run while you're logged in and survive screen lock
and app crashes, but full system sleep — idle sleep, or **closing a laptop lid** —
pauses collection (the process is frozen and resumes on wake, leaving a gap in the
data for that window). A logout also stops the agents until the next login. To
collect gap-free on a laptop, keep the Mac awake with something like **Amphetamine**.

### Backups
CSV history is backed up to the repo's **`data` branch** (kept separate from `main`
so code history stays clean). `backup.sh` commits the runtime CSVs and pushes to
`origin/data`; the `com.ronimis.gym-backup` LaunchAgent (installed by
`deploy-local.sh`) runs it at load and daily at 03:30. Restore with `git checkout data`.

## Components

- **Data Collection**: `gym-stats-collector.sh` - Polls the four gym locations every 2 minutes (primary `climbers_in_all` API, with a legacy per-location fallback) into daily CSVs
- **Web Server**: `server.go` - Serves the pages and the JSON/data endpoints
- **Dashboard**: `dashboard.html` (`/dashboard.html`) - Occupancy-over-time chart with:
  - a month/year switcher plus manual From/To, Today, and CSV download
  - adaptive downsampling so wide ranges stay readable and fast
  - a **data-freshness badge** (Live / Delayed / Collection stalled, by age) and a **"Right now"** strip comparing each gym's live count to its typical level
  - an **Insights panel** per gym (busiest/quietest day, busiest hour, best time to go, typical peak)
  - shareable URLs (`?month=YYYY-MM`, `?year=YYYY`, `?from=YYYY-MM-DD&to=YYYY-MM-DD`, `?period=all`) with browser Back/Forward support
- **Busyness**: `busyness.html` (`/busyness.html`) - Typical-busyness heatmap by weekday × hour, per location, with an All-data / year / month switcher. Averages readings in the selected period into one "typical week" (data from `GET /busyness-data`)
- **Linux deploy**: `deploy.sh` - Uploads source and restarts the systemd services on the remote server
- **macOS always-on**: `deploy-local.sh` - Installs/updates the collector + dashboard as launchd services running from a runtime folder outside `~/Documents`
- **Backup**: `backup.sh` - Commits the collected CSVs to the repo's `data` branch and pushes (run daily by a launchd agent)

Both pages are **theme-aware** (a 🌙/☀️ toggle, remembered across pages and following the OS by default) and **responsive + installable as a PWA** (`manifest.json` + `icon.svg`/`icon-192.png`/`icon-512.png`).

Generates CSV files in format: `gym-stats-YYYYMMDD.csv`

## CSV Format

Expected CSV columns:
- `timestamp` - Date and time (YYYY-MM-DD HH:MM:SS)
- `location_id` - Gym location ID
- `location_name` - Human readable gym name
- `user_count` - Current occupancy count
- `status` - API response status (success/error)
- `response` - Raw JSON response

## Analysis

The **Insights panel** on the dashboard summarises the selected period per gym:
busiest and quietest weekday, busiest hour, the best time to go (the quietest hour
that is still within the gym's active window), and the typical peak. The **"Right
now"** strip compares each gym's live count to its typical count for the current
weekday+hour. Both are computed from `GET /busyness-data` (weekday × hour
averages), which also backs the busyness heatmap.

## Endpoints

- `GET /busyness-data[?month=YYYY-MM | ?from=YYYY-MM-DD&to=YYYY-MM-DD]` - per-gym
  weekday × hour averages, samples, per-location peak, available months, data span.
- `GET /status` - most recent reading, its age in seconds, and current per-gym
  counts (backs the freshness badge and the "Right now" strip; reads only the
  latest CSV so it is cheap to poll).
- `POST /generate-data-range {from,to}` - builds the time-series chart data; wide
  ranges are averaged into time buckets (adaptive, ~1200 points/series) and the
  result is cached per range + newest-CSV mtime.
- `POST /generate-data` - same for today's file.
- `GET /download-csvs` - all `gym-stats-*.csv` as a zip.

## Tests

`go test` covers the fiddly logic: timezone conversion (UTC ↔ Europe/Tallinn),
adaptive bucket selection, and downsampling.
