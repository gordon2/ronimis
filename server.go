package main

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DataPoint struct {
	X string  `json:"x"`
	Y float64 `json:"y"`
}

type Dataset struct {
	Label string      `json:"label"`
	Data  []DataPoint `json:"data"`
}

type GenerateResponse struct {
	Success  bool      `json:"success"`
	Message  string    `json:"message"`
	Output   string    `json:"output,omitempty"`
	Error    string    `json:"error,omitempty"`
	Datasets []Dataset `json:"datasets,omitempty"`
}

type DateRangeRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type busyCell struct {
	sum   float64
	count int
}

type BusynessLocation struct {
	Name    string      `json:"name"`
	Avg     [][]float64 `json:"avg"`
	Samples [][]int     `json:"samples"`
	Max     float64     `json:"max"`
}

type BusynessResponse struct {
	Days        []string           `json:"days"`
	Hours       []int              `json:"hours"`
	Locations   []BusynessLocation `json:"locations"`
	GlobalMax   float64            `json:"globalMax"`
	GeneratedAt string             `json:"generatedAt"`
	DataStart   string             `json:"dataStart"`
	DataEnd     string             `json:"dataEnd"`
	Months      []string           `json:"months"`
	From        string             `json:"from"`
	To          string             `json:"to"`
	Readings    int                `json:"readings"`
}

func findLatestCSV() (string, error) {
	files, err := filepath.Glob("gym-stats-*.csv")
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no CSV files found matching gym-stats-*.csv")
	}

	// Find the most recently modified file
	var latestFile string
	var latestTime time.Time

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = file
		}
	}

	return latestFile, nil
}

func findCSVFilesInRange(fromDate, toDate string) ([]string, error) {
	files, err := filepath.Glob("gym-stats-*.csv")
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no CSV files found matching gym-stats-*.csv")
	}

	// Parse date range
	from, err := time.Parse("2006-01-02", fromDate)
	if err != nil {
		return nil, fmt.Errorf("invalid from date format: %v", err)
	}
	to, err := time.Parse("2006-01-02", toDate)
	if err != nil {
		return nil, fmt.Errorf("invalid to date format: %v", err)
	}

	var filteredFiles []string
	for _, file := range files {
		// Extract date from filename (gym-stats-YYYYMMDD.csv)
		basename := filepath.Base(file)
		if len(basename) < 20 { // gym-stats-YYYYMMDD.csv = 20 chars minimum
			continue
		}

		dateStr := basename[10:18] // Extract YYYYMMDD
		fileDate, err := time.Parse("20060102", dateStr)
		if err != nil {
			continue
		}

		// Check if file date is within range (inclusive)
		if (fileDate.Equal(from) || fileDate.After(from)) && (fileDate.Equal(to) || fileDate.Before(to.AddDate(0, 0, 1))) {
			filteredFiles = append(filteredFiles, file)
		}
	}

	return filteredFiles, nil
}

func convertCSVFilesToJSON(csvFiles []string) ([]Dataset, error) {
	dataByLocation := make(map[string][]DataPoint)

	for _, csvFile := range csvFiles {
		err := processCSVFile(csvFile, dataByLocation)
		if err != nil {
			return nil, fmt.Errorf("failed to process %s: %v", csvFile, err)
		}
	}

	// Convert to datasets
	var datasets []Dataset
	for locationName, dataPoints := range dataByLocation {
		// Sort by timestamp
		sort.Slice(dataPoints, func(i, j int) bool {
			return dataPoints[i].X < dataPoints[j].X
		})

		datasets = append(datasets, Dataset{
			Label: locationName,
			Data:  dataPoints,
		})
	}

	// Sort datasets by location name for consistent ordering
	sort.Slice(datasets, func(i, j int) bool {
		return datasets[i].Label < datasets[j].Label
	})

	return datasets, nil
}

func convertCSVToJSON(csvFile string) ([]Dataset, error) {
	return convertCSVFilesToJSON([]string{csvFile})
}

// pickBucketMinutes chooses an aggregation interval so a wide range stays readable
// (~1200 points per series) while short ranges keep raw 2-minute detail.
func pickBucketMinutes(from, to time.Time) int {
	spanMinutes := to.Sub(from).Minutes()
	if spanMinutes <= 0 {
		return 2
	}
	target := spanMinutes / 1200.0
	ladder := []int{2, 5, 10, 15, 30, 60, 120, 180, 360, 720, 1440}
	for _, step := range ladder {
		if float64(step) >= target {
			return step
		}
	}
	return ladder[len(ladder)-1]
}

// downsampleDatasets averages each series into fixed buckets aligned to local
// midnight. Empty buckets are dropped so gaps are preserved. bucketMinutes <= 2
// returns the data unchanged (raw 2-minute readings).
func downsampleDatasets(datasets []Dataset, bucketMinutes int) []Dataset {
	if bucketMinutes <= 2 {
		return datasets
	}

	out := make([]Dataset, 0, len(datasets))
	for _, ds := range datasets {
		type agg struct {
			sum   float64
			count int
			order int
		}
		buckets := make(map[string]*agg)
		var keys []string

		for _, p := range ds.Data {
			t, err := time.Parse("2006-01-02T15:04:05Z07:00", p.X)
			if err != nil {
				continue
			}
			minuteOfDay := t.Hour()*60 + t.Minute()
			floored := (minuteOfDay / bucketMinutes) * bucketMinutes
			bucketStart := time.Date(t.Year(), t.Month(), t.Day(), floored/60, floored%60, 0, 0, t.Location())
			key := bucketStart.Format("2006-01-02T15:04:05Z07:00")

			b := buckets[key]
			if b == nil {
				b = &agg{order: len(keys)}
				buckets[key] = b
				keys = append(keys, key)
			}
			b.sum += p.Y
			b.count++
		}

		points := make([]DataPoint, 0, len(keys))
		for _, key := range keys {
			b := buckets[key]
			points = append(points, DataPoint{
				X: key,
				Y: math.Round((b.sum/float64(b.count))*10) / 10,
			})
		}
		out = append(out, Dataset{Label: ds.Label, Data: points})
	}
	return out
}

func processCSVFile(csvFile string, dataByLocation map[string][]DataPoint) error {
	file, err := os.Open(csvFile)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true    // Handle malformed quotes more gracefully
	reader.FieldsPerRecord = -1 // Variable number of fields per record

	// Read header
	headers, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV headers: %v", err)
	}

	// Find column indices
	var timestampIdx, timezoneIdx, locationNameIdx, userCountIdx, statusIdx int = -1, -1, -1, -1, -1
	for i, header := range headers {
		switch header {
		case "timestamp":
			timestampIdx = i
		case "timezone":
			timezoneIdx = i
		case "location_name":
			locationNameIdx = i
		case "user_count":
			userCountIdx = i
		case "status":
			statusIdx = i
		}
	}

	if timestampIdx == -1 || timezoneIdx == -1 || locationNameIdx == -1 || userCountIdx == -1 || statusIdx == -1 {
		return fmt.Errorf("missing required columns in CSV")
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		maxIdx := max2(max2(max2(timestampIdx, timezoneIdx), max2(locationNameIdx, userCountIdx)), statusIdx)
		if len(record) <= maxIdx {
			continue
		}

		// Skip non-success records
		if record[statusIdx] != "success" {
			continue
		}

		// Parse timestamp from CSV
		timestamp := record[timestampIdx]
		t, err := time.Parse("2006-01-02 15:04:05", timestamp)
		if err != nil {
			continue
		}

		// Load Estonia/Tallinn timezone for display (gyms are in Tallinn)
		tallinnTZ, err := time.LoadLocation("Europe/Tallinn")
		if err != nil {
			// Fallback to fixed offset if timezone loading fails
			tallinnTZ = time.FixedZone("EET", 2*3600) // UTC+2 as fallback
		}

		var sourceTime time.Time

		// Handle timezone field if available, otherwise assume UTC (for legacy files)
		if timezoneIdx != -1 && len(record) > timezoneIdx {
			timezoneStr := record[timezoneIdx]

			// Parse timestamp with its original timezone
			timestampWithTZ := timestamp + " " + timezoneStr
			sourceTime, err = time.Parse("2006-01-02 15:04:05 MST", timestampWithTZ)
			if err != nil {
				// Fallback: treat as UTC if timezone parsing fails
				sourceTime = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
			}
		} else {
			// Legacy files without timezone field - assume UTC
			sourceTime = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
		}

		// Always convert to Tallinn timezone for display (since gyms are in Tallinn)
		tallinnTime := sourceTime.In(tallinnTZ)

		// Round to a nearest 2-minute interval
		minute := tallinnTime.Minute()
		roundedMinute := (minute / 2) * 2
		tallinnTime = time.Date(tallinnTime.Year(), tallinnTime.Month(), tallinnTime.Day(),
			tallinnTime.Hour(), roundedMinute, 0, 0, tallinnTZ)

		// Format as ISO timestamp with timezone for proper JavaScript parsing
		isoTimestamp := tallinnTime.Format("2006-01-02T15:04:05Z07:00")

		// Parse user count
		userCount, err := strconv.Atoi(record[userCountIdx])
		if err != nil {
			continue
		}

		locationName := record[locationNameIdx]

		dataByLocation[locationName] = append(dataByLocation[locationName], DataPoint{
			X: isoTimestamp,
			Y: float64(userCount),
		})
	}

	return nil
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// busynessLocalTime converts a logged (timestamp, timezone) pair to Tallinn local
// time. Historic rows are logged in UTC; recent ones carry EEST/EET, which are
// Tallinn's own summer/winter zones, so their wall-clock is already local.
func busynessLocalTime(tsStr, tzStr string, tallinn *time.Location) (time.Time, bool) {
	t, err := time.Parse("2006-01-02 15:04:05", tsStr)
	if err != nil {
		return time.Time{}, false
	}
	switch strings.ToUpper(strings.TrimSpace(tzStr)) {
	case "", "UTC", "GMT", "Z":
		src := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
		return src.In(tallinn), true
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, tallinn), true
	}
}

func accumulateBusyness(csvFile string, acc map[string]*[7][24]busyCell, tallinn *time.Location, from, to *time.Time, span *[2]time.Time, months map[string]bool) {
	file, err := os.Open(csvFile)
	if err != nil {
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	headers, err := reader.Read()
	if err != nil {
		return
	}

	tsIdx, tzIdx, locIdx, cntIdx, stIdx := -1, -1, -1, -1, -1
	for i, header := range headers {
		switch header {
		case "timestamp":
			tsIdx = i
		case "timezone":
			tzIdx = i
		case "location_name":
			locIdx = i
		case "user_count":
			cntIdx = i
		case "status":
			stIdx = i
		}
	}

	if tsIdx == -1 || locIdx == -1 || cntIdx == -1 || stIdx == -1 {
		return
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		maxIdx := max2(max2(max2(tsIdx, tzIdx), max2(locIdx, cntIdx)), stIdx)
		if len(record) <= maxIdx {
			continue
		}

		if record[stIdx] != "success" {
			continue
		}

		count, err := strconv.Atoi(record[cntIdx])
		if err != nil {
			continue
		}

		tzVal := ""
		if tzIdx != -1 {
			tzVal = record[tzIdx]
		}

		local, ok := busynessLocalTime(record[tsIdx], tzVal, tallinn)
		if !ok {
			continue
		}

		if span[0].IsZero() || local.Before(span[0]) {
			span[0] = local
		}
		if span[1].IsZero() || local.After(span[1]) {
			span[1] = local
		}
		months[local.Format("2006-01")] = true

		if from != nil && local.Before(*from) {
			continue
		}
		if to != nil && !local.Before(*to) {
			continue
		}

		dayIdx := (int(local.Weekday()) + 6) % 7 // Mon=0 ... Sun=6
		hour := local.Hour()

		name := record[locIdx]
		grid := acc[name]
		if grid == nil {
			grid = &[7][24]busyCell{}
			acc[name] = grid
		}
		grid[dayIdx][hour].sum += float64(count)
		grid[dayIdx][hour].count++
	}
}

func busynessDataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	tallinn, err := time.LoadLocation("Europe/Tallinn")
	if err != nil {
		tallinn = time.FixedZone("EET", 2*3600)
	}

	q := r.URL.Query()
	monthStr := strings.TrimSpace(q.Get("month"))
	fromStr := strings.TrimSpace(q.Get("from"))
	toStr := strings.TrimSpace(q.Get("to"))

	var fromPtr, toPtr *time.Time
	if t, err := time.ParseInLocation("2006-01", monthStr, tallinn); err == nil {
		start := t
		end := t.AddDate(0, 1, 0) // exclusive upper bound: the whole selected month
		fromPtr = &start
		toPtr = &end
	} else {
		if t, err := time.ParseInLocation("2006-01-02", fromStr, tallinn); err == nil {
			fromPtr = &t
		}
		if t, err := time.ParseInLocation("2006-01-02", toStr, tallinn); err == nil {
			end := t.AddDate(0, 0, 1) // exclusive upper bound: include the whole 'to' day
			toPtr = &end
		}
	}

	files, err := filepath.Glob("gym-stats-*.csv")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	sort.Strings(files)

	acc := make(map[string]*[7][24]busyCell)
	months := make(map[string]bool)
	var span [2]time.Time
	for _, f := range files {
		accumulateBusyness(f, acc, tallinn, fromPtr, toPtr, &span, months)
	}

	names := make([]string, 0, len(acc))
	for n := range acc {
		names = append(names, n)
	}
	sort.Strings(names)

	globalMax := 0.0
	readings := 0
	locations := make([]BusynessLocation, 0, len(names))
	for _, name := range names {
		grid := acc[name]
		avg := make([][]float64, 7)
		samples := make([][]int, 7)
		locMax := 0.0
		for d := 0; d < 7; d++ {
			avg[d] = make([]float64, 24)
			samples[d] = make([]int, 24)
			for h := 0; h < 24; h++ {
				c := grid[d][h]
				samples[d][h] = c.count
				readings += c.count
				if c.count > 0 {
					v := math.Round((c.sum/float64(c.count))*10) / 10
					avg[d][h] = v
					if v > locMax {
						locMax = v
					}
				} else {
					avg[d][h] = -1
				}
			}
		}
		if locMax > globalMax {
			globalMax = locMax
		}
		locations = append(locations, BusynessLocation{Name: name, Avg: avg, Samples: samples, Max: locMax})
	}

	hours := make([]int, 24)
	for i := range hours {
		hours[i] = i
	}

	fmtDate := func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02")
	}
	dataStart := fmtDate(span[0])
	dataEnd := fmtDate(span[1])

	monthList := make([]string, 0, len(months))
	for m := range months {
		monthList = append(monthList, m)
	}
	sort.Strings(monthList)

	effFrom := dataStart
	if fromPtr != nil {
		effFrom = fromPtr.Format("2006-01-02")
	}
	effTo := dataEnd
	if toPtr != nil {
		effTo = toPtr.AddDate(0, 0, -1).Format("2006-01-02")
	}

	json.NewEncoder(w).Encode(BusynessResponse{
		Days:        []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
		Hours:       hours,
		Locations:   locations,
		GlobalMax:   globalMax,
		GeneratedAt: time.Now().In(tallinn).Format("2006-01-02 15:04:05 MST"),
		DataStart:   dataStart,
		DataEnd:     dataEnd,
		Months:      monthList,
		From:        effFrom,
		To:          effTo,
		Readings:    readings,
	})
}

func generateDataHandler(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Find latest CSV file
	csvFile, err := findLatestCSV()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Convert CSV to JSON
	datasets, err := convertCSVToJSON(csvFile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to convert CSV: %v", err),
		})
		return
	}

	// Write to gym-data.json
	jsonFile, err := os.Create("gym-data.json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create JSON file: %v", err),
		})
		return
	}
	defer jsonFile.Close()

	encoder := json.NewEncoder(jsonFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(datasets); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to write JSON: %v", err),
		})
		return
	}

	// Success response
	output := fmt.Sprintf("Successfully generated gym-data.json from %s\nFound %d locations with data", csvFile, len(datasets))

	json.NewEncoder(w).Encode(GenerateResponse{
		Success:  true,
		Message:  "Data generated successfully",
		Output:   output,
		Datasets: datasets,
	})
}

func generateDataRangeHandler(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Parse request body
	var dateRange DateRangeRequest
	if err := json.NewDecoder(r.Body).Decode(&dateRange); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Find CSV files in date range
	csvFiles, err := findCSVFilesInRange(dateRange.From, dateRange.To)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if len(csvFiles) == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success: false,
			Error:   fmt.Sprintf("No CSV files found for date range %s to %s", dateRange.From, dateRange.To),
		})
		return
	}

	// Convert CSV files to JSON
	datasets, err := convertCSVFilesToJSON(csvFiles)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to convert CSV files: %v", err),
		})
		return
	}

	// Downsample wide ranges so the chart stays readable and fast
	bucketMinutes := 2
	fromDate, fromErr := time.Parse("2006-01-02", dateRange.From)
	toDate, toErr := time.Parse("2006-01-02", dateRange.To)
	if fromErr == nil && toErr == nil {
		bucketMinutes = pickBucketMinutes(fromDate, toDate.AddDate(0, 0, 1))
		datasets = downsampleDatasets(datasets, bucketMinutes)
	}

	// Write to gym-data.json
	jsonFile, err := os.Create("gym-data.json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create JSON file: %v", err),
		})
		return
	}
	defer jsonFile.Close()

	encoder := json.NewEncoder(jsonFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(datasets); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenerateResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to write JSON: %v", err),
		})
		return
	}

	// Success response
	output := fmt.Sprintf("Successfully generated gym-data.json from %d files (%s to %s)\nFound %d locations with data (bucket: %d min)",
		len(csvFiles), dateRange.From, dateRange.To, len(datasets), bucketMinutes)

	json.NewEncoder(w).Encode(GenerateResponse{
		Success:  true,
		Message:  "Date range data generated successfully",
		Output:   output,
		Datasets: datasets,
	})
}

func downloadCSVsHandler(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Find all CSV files
	files, err := filepath.Glob("gym-stats-*.csv")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error finding CSV files"))
		return
	}

	if len(files) == 0 {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("No CSV files found"))
		return
	}

	// Generate filename with current date and time in format "1-sep-2025_00-00-00.zip"
	now := time.Now()
	monthName := strings.ToLower(now.Month().String()[:3])
	filename := fmt.Sprintf("gym-stats-%d-%s-%d_%02d-%02d-%02d.zip",
		now.Day(),
		monthName,
		now.Year(),
		now.Hour(),
		now.Minute(),
		now.Second())

	// Set headers for ZIP download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// Create ZIP writer
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Add each CSV file to the ZIP
	for _, filePath := range files {
		err := addFileToZip(zipWriter, filePath)
		if err != nil {
			log.Printf("Error adding file %s to zip: %v", filePath, err)
			continue
		}
	}
}

func addFileToZip(zipWriter *zip.Writer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get file info for the header
	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create ZIP file header
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.Base(filePath)
	header.Method = zip.Deflate

	// Create the file in the ZIP
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	// Copy file content to ZIP
	_, err = io.Copy(writer, file)
	return err
}

func corsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	port := "8002"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	// Static file server
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", corsHandler(fs))

	// Data generation endpoints
	http.HandleFunc("/generate-data", generateDataHandler)
	http.HandleFunc("/generate-data-range", generateDataRangeHandler)
	http.HandleFunc("/download-csvs", downloadCSVsHandler)
	http.HandleFunc("/busyness-data", busynessDataHandler)

	fmt.Printf("Server running at http://localhost:%s/\n", port)
	fmt.Printf("Dashboard: http://localhost:%s/dashboard.html\n", port)
	fmt.Printf("Busyness: http://localhost:%s/busyness.html\n", port)
	fmt.Printf("Generate data: POST to http://localhost:%s/generate-data\n", port)
	fmt.Printf("Generate data range: POST to http://localhost:%s/generate-data-range\n", port)
	fmt.Printf("Download CSVs: GET http://localhost:%s/download-csvs\n", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
