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
there are left untouched), writes the two LaunchAgents, and (re)loads them.
Dashboard: http://localhost:8002/dashboard.html

Manage:
```bash
# stop (KeepAlive respawns a plain kill — use bootout)
launchctl bootout gui/$(id -u)/com.ronimis.gym-stats-collector
launchctl bootout gui/$(id -u)/com.ronimis.gym-server
# start / reload
./deploy-local.sh
```
Logs: `~/Library/Logs/ronimis-gym-collector.log` and `ronimis-gym-server.log`

## Components

- **Data Collection**: `gym-stats-collector.sh` - Automated gym occupancy data collection
- **Web Server**: `server.go` - Interactive web dashboard with live data refresh
- **Dashboard**: `dashboard.html` - Occupancy-over-time chart with a month/year switcher (plus manual From/To), an Insights panel (busiest/quietest day & time, typical peak per gym), and adaptive downsampling so wide ranges stay readable and fast. The selected period is encoded in the URL for shareable links: `?month=YYYY-MM`, `?year=YYYY`, `?from=YYYY-MM-DD&to=YYYY-MM-DD`, or `?period=all` (`/dashboard.html`)
- **Busyness**: `busyness.html` - Typical busyness heatmap by weekday × hour, per location. Averages all readings in the selected period into one "typical week" (`/busyness.html`, data from `GET /busyness-data`, optional `?month=YYYY-MM`; defaults to all data)
- **Linux deploy**: `deploy.sh` - Uploads source and restarts the systemd services on the remote server
- **macOS always-on**: `deploy-local.sh` - Installs/updates the collector + dashboard as launchd services running from a runtime folder outside `~/Documents`


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
busiest and quietest weekday, busiest and quietest hour, and the typical peak.
It is computed from `GET /busyness-data` (weekday × hour averages), which also
backs the busyness heatmap.

## Endpoints

- `GET /busyness-data[?month=YYYY-MM | ?from=YYYY-MM-DD&to=YYYY-MM-DD]` - per-gym
  weekday × hour averages, samples, per-location peak, available months, data span.
- `POST /generate-data-range {from,to}` - builds `gym-data.json` for the time-series
  chart; wide ranges are averaged into time buckets (adaptive, ~1200 points/series).
- `POST /generate-data` - same for today's file.
- `GET /download-csvs` - all `gym-stats-*.csv` as a zip.
