#!/bin/bash

# Climbing gym user stats collector
# Collects user count data from multiple locations

# Primary API: all locations in one authenticated request
API_URL="https://ministeerium.codeventions.com/api/v01/openair/climbers_in_all"

# Legacy API (fallback): per-location, includes pass-type breakdown
BASE_URL="https://ministeerium.codeventions.com/t/coupling/show_climbers_in/?json=true&location="

# Location definitions (location_id:name pairs)
LOCATIONS="1:Hipodroom 3:T1 9:Mustika 10:Suur-Paala"

get_location_name() {
    local id=$1
    for location in $LOCATIONS; do
        if [[ "$location" == "$id:"* ]]; then
            echo "${location#*:}"
            return
        fi
    done
    echo "Unknown"
}

get_log_file() {
    echo "gym-stats-$(date +%Y%m%d).csv"
}

ensure_csv_header() {
    local log_file="$1"
    if [ ! -f "$log_file" ]; then
        echo "timestamp,timezone,location_id,location_name,user_count,status,response" > "$log_file"
    fi
}

# Load configuration from external file
CONFIG_FILE="${CONFIG_FILE:-gym-config.env}"

if [ -f "$CONFIG_FILE" ]; then
    source "$CONFIG_FILE"
else
    echo "Error: Configuration file '$CONFIG_FILE' not found!"
    echo "Please create $CONFIG_FILE with your authentication tokens:"
    echo "  API_TOKEN=<bearer token for the primary API>"
    echo "  PHPSESSID= / XSRF_TOKEN= / LARAVEL_SESSION= (legacy fallback cookies, optional)"
    exit 1
fi

# Build cookies from config
COOKIES="PHPSESSID=${PHPSESSID}; XSRF-TOKEN=${XSRF_TOKEN}; laravel_session=${LARAVEL_SESSION}"

# Primary: one request for all locations via the new API
collect_data_api() {
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    timezone=$(date '+%Z')
    log_file="$(get_log_file)"

    # Ensure CSV header exists for today's file
    ensure_csv_header "$log_file"

    echo "[$timestamp] Collecting data for all locations (primary API)"

    response=$(curl -s -w "HTTPSTATUS:%{http_code}" "$API_URL" \
        -H "Authorization: Bearer $API_TOKEN" \
        -H 'Accept: application/json' \
        --max-time 30)

    http_code=$(echo "$response" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
    body=$(echo "$response" | sed 's/HTTPSTATUS:[0-9]*$//')

    if [ "$http_code" != "200" ]; then
        echo "  -> ERROR: HTTP $http_code"
        if [ "$http_code" = "401" ] || [ "$http_code" = "403" ]; then
            echo "  -> Auth failed - update API_TOKEN in $CONFIG_FILE"
        fi
        return 1
    fi

    # One line per location: id <TAB> api_name <TAB> total <TAB> compact_json
    rows=$(echo "$body" | python3 -c "import sys, json; data=json.load(sys.stdin); [print('\t'.join([str(l['location_id']), str(l['location_name']), str(l['total']), json.dumps(l, separators=(',',':'))])) for l in data]" 2>/dev/null)

    if [ -z "$rows" ]; then
        echo "  -> ERROR: could not parse API response"
        return 1
    fi

    while IFS=$'\t' read -r location_id api_name user_count loc_json; do
        # Prefer local short names (keeps series continuity, e.g. "T1"),
        # fall back to API name for locations not in LOCATIONS yet
        location_name="$(get_location_name $location_id)"
        if [ "$location_name" = "Unknown" ]; then
            location_name="$api_name"
        fi
        echo "$timestamp,$timezone,$location_id,$location_name,$user_count,success,\"$loc_json\"" >> "$log_file"
        echo "  -> $location_name: $user_count users"
    done <<< "$rows"
}

# Fallback: legacy per-location API (keeps pass-type breakdown in response column)
collect_data_legacy() {
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    timezone=$(date '+%Z')
    log_file="$(get_log_file)"

    # Ensure CSV header exists for today's file
    ensure_csv_header "$log_file"

    # Loop through all locations
    for location_pair in $LOCATIONS; do
        location_id="${location_pair%:*}"
        location_name="$(get_location_name $location_id)"
        url="${BASE_URL}${location_id}"

        echo "[$timestamp] Collecting data for $location_name (ID: $location_id)"

        # Make request with error handling
        response=$(curl -s -w "HTTPSTATUS:%{http_code}" "$url" \
            -H 'User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:142.0) Gecko/20100101 Firefox/142.0' \
            -H 'Accept: application/json, text/plain, */*' \
            -H 'Accept-Language: en-US,en;q=0.5' \
            -H "X-XSRF-TOKEN: $XSRF_TOKEN" \
            -H 'DNT: 1' \
            -H 'Connection: keep-alive' \
            -H "Referer: https://ministeerium.codeventions.com/t/doorserver/openair/ministeerium/${API_KEY}" \
            -H "Cookie: $COOKIES" \
            -H 'Sec-Fetch-Mode: cors' \
            -H 'Sec-Fetch-Site: same-origin' \
            --max-time 30)

        # Extract HTTP status and body
        http_code=$(echo "$response" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
        body=$(echo "$response" | sed 's/HTTPSTATUS:[0-9]*$//')

        if [ "$http_code" = "200" ]; then
            # Try to extract user count from JSON
            user_count=$(echo "$body" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('total', 'unknown'))" 2>/dev/null || echo "parse_error")
            echo "$timestamp,$timezone,$location_id,$location_name,$user_count,success,\"$body\"" >> "$log_file"
            echo "  -> Users: $user_count"
        else
            echo "$timestamp,$timezone,$location_id,$location_name,error,$http_code,\"$body\"" >> "$log_file"
            echo "  -> ERROR: HTTP $http_code"

            # If auth error, remind user to update tokens
            if [ "$http_code" = "401" ] || [ "$http_code" = "403" ]; then
                echo "  -> Auth failed - update cookie tokens in $CONFIG_FILE"
            fi
        fi

    done
}

collect_data() {
    if [ -n "$API_TOKEN" ] && collect_data_api; then
        return
    fi
    echo "  -> Falling back to legacy API"
    collect_data_legacy
}

# Main loop
echo "Starting gym stats collection (Ctrl+C to stop)"
echo "Data will be logged to daily files: gym-stats-YYYYMMDD.csv"

while true; do
    current_log="$(get_log_file)"
    echo "Current log file: $current_log"

    collect_data

    # 2 minute delay before next collection cycle
    sleep $((60*2))
done