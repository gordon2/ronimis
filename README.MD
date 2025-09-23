# Gym Occupancy Dashboard

A complete gym occupancy monitoring system with data collection, web dashboard, and interactive visualization using Chart.js.

## Quick Start

1. The `gym-stats-collector.sh` script automatically collects gym occupancy data:

```bash
./gym-stats-collector.sh
```

2. Update the `gym-config.env` file with authentication tokens:

3. Build and start the web server
```bash
go build -o gym-server server.go
./gym-server 8002
```

### Services
Place the files from directory `services` to `/etc/systemd/system/`

```
sudo systemctl daemon-reload
sudo systemctl restart gym.service gym-stats-collector.service
```

### Quick deploy
Uploads relevant source files and restarts services
```
SERVER_IP=<00.00.000.000> SERVER_USER=<username> ./deploy.sh
```

## Components

- **Data Collection**: `gym-stats-collector.sh` - Automated gym occupancy data collection
- **Web Server**: `server.go` - Interactive web dashboard with live data refresh
- **Dashboard**: `dashboard.html` - Interactive chart visualization
- **Analysis Tool**: `gym-stats-filter.sh` - Command-line data filtering and analysis


Generates CSV files in format: `gym-stats-YYYYMMDD.csv`

## CSV Format

Expected CSV columns:
- `timestamp` - Date and time (YYYY-MM-DD HH:MM:SS)
- `location_id` - Gym location ID
- `location_name` - Human readable gym name
- `user_count` - Current occupancy count
- `status` - API response status (success/error)
- `response` - Raw JSON response

## Analysis Tools

### Command Line Filtering
```bash
# Show today's data
./gym-stats-filter.sh

# Filter by date range
./gym-stats-filter.sh --from 2025-09-14 --to 2025-09-15

# Filter by location
./gym-stats-filter.sh --location "T1"

# Show summary statistics
./gym-stats-filter.sh --summary
```
