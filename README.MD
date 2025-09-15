# Gym Occupancy Dashboard

A complete gym occupancy monitoring system with data collection, web dashboard, and interactive visualization using Chart.js.

## Components

- **Data Collection**: `gym-stats-collector.sh` - Automated gym occupancy data collection
- **Web Server**: `server.go` - Interactive web dashboard with live data refresh
- **Dashboard**: `dashboard.html` - Interactive chart visualization
- **Analysis Tool**: `gym-stats-filter.sh` - Command-line data filtering and analysis

## Quick Start

First, update the `gym-config.env` file with authentication tokens:

```bash
# Build and start the web server
go build -o `gym-server` server.go
./gym-server [port]                    # Default port: 8002

# Open browser to http://localhost:8002/dashboard.html
# Use "Today" for latest CSV or date range selector for specific periods
```

## Data Collection

The `gym-stats-collector.sh` script automatically collects gym occupancy data:

### Usage
```bash
./gym-stats-collector.sh               # Collect current data
```

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


## File Structure

```
├── gym-stats-collector.sh      # Data collection script
├── gym-config.env      # Configuration file
├── server.go                   # Web server (main)
├── dashboard.html              # Interactive dashboard
├── gym-stats-filter.sh         # Analysis tool
├── gym-stats-YYYYMMDD.csv      # Data files
└── gym-data.json               # Generated chart data
```
