package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type DataPoint struct {
	X string `json:"x"`
	Y int    `json:"y"`
}

type Dataset struct {
	Label string      `json:"label"`
	Data  []DataPoint `json:"data"`
}

type GenerateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

type DateRangeRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
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
	var timestampIdx, locationNameIdx, userCountIdx, statusIdx int = -1, -1, -1, -1
	for i, header := range headers {
		switch header {
		case "timestamp":
			timestampIdx = i
		case "location_name":
			locationNameIdx = i
		case "user_count":
			userCountIdx = i
		case "status":
			statusIdx = i
		}
	}

	if timestampIdx == -1 || locationNameIdx == -1 || userCountIdx == -1 || statusIdx == -1 {
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

		maxIdx := max2(max2(timestampIdx, locationNameIdx), max2(userCountIdx, statusIdx))
		if len(record) <= maxIdx {
			continue
		}

		// Skip non-success records
		if record[statusIdx] != "success" {
			continue
		}

		// Parse timestamp from CSV (stored as UTC)
		timestamp := record[timestampIdx]
		t, err := time.Parse("2006-01-02 15:04:05", timestamp)
		if err != nil {
			continue
		}

		// Treat the timestamp as UTC (since collector script now uses `date -u`)
		utcTime := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)

		// Load Estonia/Tallinn timezone
		tallinnTZ, err := time.LoadLocation("Europe/Tallinn")
		if err != nil {
			// Fallback to fixed offset if timezone loading fails
			tallinnTZ = time.FixedZone("EET", 2*3600) // UTC+2 as fallback
		}

		// Convert UTC time to Tallinn timezone
		tallinnTime := utcTime.In(tallinnTZ)

		// Round to a nearest 2-minute interval
		minute := tallinnTime.Minute()
		roundedMinute := (minute / 2) * 2
		tallinnTime = time.Date(tallinnTime.Year(), tallinnTime.Month(), tallinnTime.Day(),
			tallinnTime.Hour(), roundedMinute, 0, 0, tallinnTZ)

		// Format as ISO timestamp with proper timezone offset
		isoTimestamp := tallinnTime.Format("2006-01-02T15:04:05-07:00")

		// Parse user count
		userCount, err := strconv.Atoi(record[userCountIdx])
		if err != nil {
			continue
		}

		locationName := record[locationNameIdx]

		dataByLocation[locationName] = append(dataByLocation[locationName], DataPoint{
			X: isoTimestamp,
			Y: userCount,
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
		Success: true,
		Message: "Data generated successfully",
		Output:  output,
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
	output := fmt.Sprintf("Successfully generated gym-data.json from %d files (%s to %s)\nFound %d locations with data",
		len(csvFiles), dateRange.From, dateRange.To, len(datasets))

	json.NewEncoder(w).Encode(GenerateResponse{
		Success: true,
		Message: "Date range data generated successfully",
		Output:  output,
	})
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

	fmt.Printf("Server running at http://localhost:%s/\n", port)
	fmt.Printf("Dashboard: http://localhost:%s/dashboard.html\n", port)
	fmt.Printf("Generate data: POST to http://localhost:%s/generate-data\n", port)
	fmt.Printf("Generate data range: POST to http://localhost:%s/generate-data-range\n", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
