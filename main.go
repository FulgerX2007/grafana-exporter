package main

import (
	"bytes"
	"crypto/tls"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

//go:embed public/*
var publicFS embed.FS

// Configuration holds application settings from .env
type Config struct {
	GrafanaURL      string
	GrafanaAPIKey   string
	ExportDirectory string
	ServerPort      string
	SkipTLSVerify   bool
	GrafanaVersion  float64 // Add this field
}

// Dashboard represents a Grafana dashboard
type Dashboard struct {
	ID         int      `json:"id"`
	UID        string   `json:"uid"`
	Title      string   `json:"title"`
	FolderID   int      `json:"folderId"`
	FolderUID  string   `json:"folderUid,omitempty"`
	FolderName *string  `json:"folderTitle,omitempty"`
	URL        string   `json:"url,omitempty"`
	Type       string   `json:"type,omitempty"` // Make type optional
	Tags       []string `json:"tags,omitempty"`
}

// DashboardResponse is the API response structure for dashboards
type DashboardResponse struct {
	Dashboards []Dashboard `json:"dashboards"`
}

// Folder represents a Grafana folder
type Folder struct {
	ID             int    `json:"id"`
	UID            string `json:"uid"`
	Title          string `json:"title"`
	URL            string `json:"url,omitempty"`
	HasACL         bool   `json:"hasAcl,omitempty"`
	CanSave        bool   `json:"canSave,omitempty"`
	CanEdit        bool   `json:"canEdit,omitempty"`
	CanAdmin       bool   `json:"canAdmin,omitempty"`
	CreatedBy      string `json:"createdBy,omitempty"`
	Created        string `json:"created,omitempty"`
	UpdatedBy      string `json:"updatedBy,omitempty"`
	Updated        string `json:"updated,omitempty"`
	Version        int    `json:"version,omitempty"`
	ParentUID      string `json:"parentUid,omitempty"` // Add this field
	Nested         bool   `json:"nested,omitempty"`
	DashboardCount int    `json:"dashboardCount,omitempty"`
}

// FolderResponse is the API response for folders
type FolderResponse []Folder

// LibraryElement represents a Grafana library element
type LibraryElement struct {
	ID        int    `json:"id"`
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Kind      int    `json:"kind"`
	FolderID  int    `json:"folderId"`
	FolderUID string `json:"folderUid"`
}

// LibraryElementsResponse is the API response for library elements
type LibraryElementsResponse struct {
	Result []LibraryElement `json:"result"`
}

// DashboardWithMeta represents a dashboard with its metadata
type DashboardWithMeta struct {
	Dashboard map[string]interface{} `json:"dashboard"`
	Meta      struct {
		FolderID    int    `json:"folderId"`
		FolderUID   string `json:"folderUid"`
		FolderTitle string `json:"folderTitle"`
	} `json:"meta"`
}

// LibraryElementWithMeta represents a library element with its metadata
type LibraryElementWithMeta struct {
	Result struct {
		ID        int                    `json:"id"`
		UID       string                 `json:"uid"`
		Name      string                 `json:"name"`
		Kind      int                    `json:"kind"`
		Model     map[string]interface{} `json:"model"`
		FolderID  int                    `json:"folderId"`
		FolderUID string                 `json:"folderUid"`
	} `json:"result"`
}

var config Config
var folderCache map[string]string

func main() {
	// Initialize the application
	initializationError := initialize()

	// Setup Echo
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// API endpoints
	e.GET("/api/folders", getFolders)
	e.GET("/api/dashboards", getDashboards)
	e.GET("/api/libraries", getLibraries)
	e.POST("/api/export", exportDashboards)

	// Add endpoint to check if .env exists
	e.GET(
		"/api/config-status", func(c echo.Context) error {
			return c.JSON(
				http.StatusOK, map[string]interface{}{
					"hasEnvFile":   initializationError == nil,
					"errorMessage": getInitErrorMessage(initializationError),
				},
			)
		},
	)

	// Serve static files from embedded filesystem
	setupStaticFiles(e)

	// Start server
	log.Printf("Server started on http://localhost:%s", config.ServerPort)
	e.Logger.Fatal(e.Start(":" + config.ServerPort))
}

func initialize() error {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
		// Check if file doesn't exist specifically
		if os.IsNotExist(err) {
			return fmt.Errorf("missing .env file: %v", err)
		}
	}

	// Initialize configuration
	config = Config{
		GrafanaURL:      getEnv("GRAFANA_URL", "http://localhost:3000"),
		GrafanaAPIKey:   getEnv("GRAFANA_API_KEY", ""),
		ExportDirectory: getEnv("EXPORT_DIRECTORY", "./exported"),
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		SkipTLSVerify:   getEnvBool("SKIP_TLS_VERIFY", false),
		GrafanaVersion:  getEnvFloat("GRAFANA_VERSION", 11.1),
	}

	// Initialize folder cache
	folderCache = make(map[string]string)

	// Ensure export directory exists
	if err := os.MkdirAll(config.ExportDirectory, os.ModePerm); err != nil {
		log.Fatalf("Failed to create export directory: %v", err)
	}

	log.Printf("Initialized with Grafana URL: %s", config.GrafanaURL)
	log.Printf("Export directory: %s", config.ExportDirectory)
	log.Printf("Server running on port: %s", config.ServerPort)
	log.Printf("Grafana version: %.1f", config.GrafanaVersion)

	// Check Grafana connection
	checkGrafanaConnection()

	return nil
}

func getInitErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	if strings.Contains(err.Error(), "missing .env file") {
		return "Configuration file (.env) not found. Please create one based on the .env.example template."
	}

	return err.Error()
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if value == "true" || value == "1" || value == "yes" {
			return true
		}
		if value == "false" || value == "0" || value == "no" {
			return false
		}
	}
	return fallback
}

func checkGrafanaConnection() {
	url := fmt.Sprintf("%s/api/health", config.GrafanaURL)

	// Create custom HTTP client with optional TLS verification skipping
	client := &http.Client{}
	if config.SkipTLSVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		log.Println("TLS certificate verification is disabled")
	}

	req, _ := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)

	if err != nil {
		log.Printf("Warning: Could not connect to Grafana: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Warning: Grafana returned status code %d", resp.StatusCode)
		return
	}

	log.Println("Successfully connected to Grafana")
}

func getFolders(c echo.Context) error {
	// First, get all top-level folders (where parentUid is not specified)
	url := fmt.Sprintf("%s/api/folders?limit=1000", config.GrafanaURL)

	var topLevelFolders []Folder
	err := fetchAPIRaw(url, &topLevelFolders)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	log.Printf("Retrieved %d top-level folders from API", len(topLevelFolders))

	// Create a slice to store all folders (top-level and nested)
	allFolders := make([]Folder, len(topLevelFolders))
	copy(allFolders, topLevelFolders)

	// Add top-level folders to the folder cache
	for _, folder := range topLevelFolders {
		folderCache[folder.UID] = folder.Title
	}

	// Recursively get all nested folders
	processedFolders := make(map[string]bool)
	for _, folder := range topLevelFolders {
		processedFolders[folder.UID] = true
	}

	// Start with top-level folders
	foldersToProcess := make([]Folder, len(topLevelFolders))
	copy(foldersToProcess, topLevelFolders)

	// Process folders in waves until we've gone through all levels
	for len(foldersToProcess) > 0 {
		var nextWave []Folder

		for _, parentFolder := range foldersToProcess {
			// Get children of this folder
			nestedURL := fmt.Sprintf(
				"%s/api/folders?limit=1000&withParents=true&parentUid=%s",
				config.GrafanaURL, parentFolder.UID,
			)

			var childFolders []Folder
			childErr := fetchAPIRaw(nestedURL, &childFolders)

			if childErr == nil && len(childFolders) > 0 {
				log.Printf(
					"Found %d child folders for folder %s (%s)",
					len(childFolders), parentFolder.Title, parentFolder.UID,
				)

				// Process each child folder
				for i := range childFolders {
					// Ensure ParentUID is set
					childFolders[i].ParentUID = parentFolder.UID
					folderCache[childFolders[i].UID] = childFolders[i].Title

					// Add to all folders if we haven't processed it before
					if !processedFolders[childFolders[i].UID] {
						allFolders = append(allFolders, childFolders[i])
						processedFolders[childFolders[i].UID] = true

						// Add to next wave for processing its children
						nextWave = append(nextWave, childFolders[i])
					}
				}
			} else if childErr != nil {
				log.Printf(
					"Note: Error getting child folders for %s (%s): %v",
					parentFolder.Title, parentFolder.UID, childErr,
				)
			}
		}

		// Set up next wave of folders to process
		foldersToProcess = nextWave
		log.Printf("Next wave: %d folders to process", len(foldersToProcess))
	}

	// Count how many folders have parent-child relationships
	nestedCount := 0
	for _, folder := range allFolders {
		if folder.ParentUID != "" {
			nestedCount++
		}
	}

	log.Printf(
		"Total folders: %d (top-level: %d, nested: %d)",
		len(allFolders), len(topLevelFolders), nestedCount,
	)

	// Get dashboard count for each folder
	dashboardsUrl := fmt.Sprintf("%s/api/search?type=dash-db&limit=5000", config.GrafanaURL)
	var searchResult []Dashboard
	err = fetchAPIRaw(dashboardsUrl, &searchResult)
	if err != nil {
		log.Printf("Warning: Could not get dashboard counts: %v", err)
	} else {
		// Count dashboards per folder
		folderDashboardCounts := make(map[int]int)
		for _, dash := range searchResult {
			if dash.Type == "" || dash.Type == "dash-db" {
				folderDashboardCounts[dash.FolderID]++
			}
		}

		// Add dashboard counts to folder response
		for i := range allFolders {
			count := folderDashboardCounts[allFolders[i].ID]
			allFolders[i].DashboardCount = count
		}
	}

	return c.JSON(http.StatusOK, allFolders)
}

func getDashboards(c echo.Context) error {
	// Use the search API with expanded response to get all dashboards
	url := fmt.Sprintf("%s/api/search?type=dash-db&limit=5000", config.GrafanaURL)

	// The API returns an array, not an object with a dashboards field
	var searchResult []Dashboard
	err := fetchAPIRaw(url, &searchResult)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	log.Printf("Retrieved %d dashboards from API", len(searchResult))
	for i, dash := range searchResult {
		if i < 3 { // Log just a few for debugging
			log.Printf(
				"Dashboard %d: ID=%d, UID=%s, Title=%s, FolderID=%d, FolderUID=%s, Type=%s",
				i, dash.ID, dash.UID, dash.Title, dash.FolderID, dash.FolderUID, dash.Type,
			)
		}
	}

	// Filter our results to only include actual dashboards
	var dashboardsOnly []Dashboard
	for _, item := range searchResult {
		// Include if it's explicitly a dashboard or if Type is not specified
		if item.Type == "" || item.Type == "dash-db" {
			// If the dashboard has a Title and UID, consider it valid
			if item.Title != "" && item.UID != "" {
				dashboardsOnly = append(dashboardsOnly, item)
			}
		}
	}

	log.Printf("Filtered to %d actual dashboards", len(dashboardsOnly))

	// Create a response structure that matches what the frontend expects
	response := DashboardResponse{
		Dashboards: dashboardsOnly,
	}

	// Load folder information for dashboards that don't have folder names
	for i, dash := range response.Dashboards {
		// For dashboards in the General folder
		if dash.FolderID == 0 {
			generalStr := "General"
			response.Dashboards[i].FolderName = &generalStr
			continue
		}

		// For dashboards with missing folder name
		if dash.FolderName == nil || *dash.FolderName == "" {
			if dash.FolderUID != "" {
				folderName, ok := folderCache[dash.FolderUID]
				if ok {
					response.Dashboards[i].FolderName = &folderName
				} else {
					// If folder not in cache, try to fetch it
					folderURL := fmt.Sprintf("%s/api/folders/%s", config.GrafanaURL, dash.FolderUID)
					var folder Folder
					if err := fetchAPIRaw(folderURL, &folder); err == nil {
						folderCache[folder.UID] = folder.Title
						response.Dashboards[i].FolderName = &folder.Title
					} else {
						// If we can't find the folder, use a placeholder
						unknown := fmt.Sprintf("Folder ID %d", dash.FolderID)
						response.Dashboards[i].FolderName = &unknown
					}
				}
			} else {
				// If no folderUID, use a placeholder based on ID
				unknown := fmt.Sprintf("Folder ID %d", dash.FolderID)
				response.Dashboards[i].FolderName = &unknown
			}
		}
	}

	return c.JSON(http.StatusOK, response)
}

func getLibraries(c echo.Context) error {
	url := fmt.Sprintf("%s/api/library-elements?perPage=100", config.GrafanaURL)

	libraries, err := fetchAPI[LibraryElementsResponse](url)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, libraries)
}

func exportDashboards(c echo.Context) error {
	// Parse request
	var req struct {
		DashboardUIDs []string `json:"dashboardUIDs"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request format"})
	}

	if len(req.DashboardUIDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No dashboards selected"})
	}

	// Create timestamped export directory
	timestamp := time.Now().Format("20060102_150405")
	exportPath := filepath.Join(config.ExportDirectory, timestamp)
	dashboardsPath := filepath.Join(exportPath, "dashboards")
	librariesPath := filepath.Join(exportPath, "libraries")

	if err := os.MkdirAll(dashboardsPath, os.ModePerm); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create export directory"})
	}
	if err := os.MkdirAll(librariesPath, os.ModePerm); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			map[string]string{"error": "Failed to create libraries directory"},
		)
	}

	// Track exported libraries to avoid duplicates
	exportedLibraries := make(map[string]bool)
	exportResult := struct {
		ExportedDashboards int      `json:"exportedDashboards"`
		ExportedLibraries  int      `json:"exportedLibraries"`
		Errors             []string `json:"errors"`
		ExportPath         string   `json:"exportPath"`
	}{
		Errors:     []string{},
		ExportPath: exportPath,
	}

	// Export each dashboard
	for _, uid := range req.DashboardUIDs {
		dashURL := fmt.Sprintf("%s/api/dashboards/uid/%s", config.GrafanaURL, uid)
		dashboard, err := fetchAPI[DashboardWithMeta](dashURL)

		if err != nil {
			exportResult.Errors = append(exportResult.Errors, fmt.Sprintf("Failed to fetch dashboard %s: %v", uid, err))
			continue
		}

		// Create folder structure if needed
		var folderPath string
		if dashboard.Meta.FolderID == 0 {
			folderPath = filepath.Join(dashboardsPath, "General")
		} else {
			folderName := dashboard.Meta.FolderTitle
			folderPath = filepath.Join(dashboardsPath, sanitizePath(folderName))
		}

		if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
			exportResult.Errors = append(
				exportResult.Errors,
				fmt.Sprintf("Failed to create folder structure for %s: %v", uid, err),
			)
			continue
		}

		// Extract dashboard title for filename
		dashboardTitle, ok := dashboard.Dashboard["title"].(string)
		if !ok {
			dashboardTitle = uid
		}

		// Save dashboard JSON
		filename := filepath.Join(folderPath, sanitizePath(dashboardTitle)+".json")
		dashboardJSON, err := json.MarshalIndent(dashboard.Dashboard, "", "  ")
		if err != nil {
			exportResult.Errors = append(
				exportResult.Errors,
				fmt.Sprintf("Failed to marshal dashboard %s: %v", uid, err),
			)
			continue
		}

		if err := os.WriteFile(filename, dashboardJSON, 0644); err != nil {
			exportResult.Errors = append(exportResult.Errors, fmt.Sprintf("Failed to write dashboard %s: %v", uid, err))
			continue
		}

		exportResult.ExportedDashboards++

		// Find and export library panels used in this dashboard
		libraryPanels, err := extractLibraryPanelUIDs(dashboard.Dashboard)
		if err != nil {
			exportResult.Errors = append(
				exportResult.Errors,
				fmt.Sprintf("Failed to extract library panels from %s: %v", uid, err),
			)
		}

		// Export each library panel
		for _, libraryUID := range libraryPanels {
			if exportedLibraries[libraryUID] {
				continue // Skip already exported libraries
			}

			if err := exportLibraryElement(
				libraryUID,
				librariesPath,
				&exportResult.ExportedLibraries,
				&exportResult.Errors,
			); err != nil {
				exportResult.Errors = append(exportResult.Errors, err.Error())
				continue
			}

			exportedLibraries[libraryUID] = true
		}
	}

	return c.JSON(http.StatusOK, exportResult)
}

func extractLibraryPanelUIDs(dashboard map[string]interface{}) ([]string, error) {
	libraryUIDs := make([]string, 0)

	// Check if dashboard has panels
	panelsInterface, ok := dashboard["panels"]
	if !ok {
		return libraryUIDs, nil
	}

	panels, ok := panelsInterface.([]interface{})
	if !ok {
		return libraryUIDs, fmt.Errorf("panels is not an array")
	}

	// Extract library panel UIDs from panels
	for _, panelInterface := range panels {
		panel, ok := panelInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if panel is a library panel
		if libraryPanel, ok := panel["libraryPanel"].(map[string]interface{}); ok {
			if uid, ok := libraryPanel["uid"].(string); ok {
				libraryUIDs = append(libraryUIDs, uid)
			}
		}

		// Check for nested panels (in rows or other container panels)
		if nestedPanelsInterface, ok := panel["panels"].([]interface{}); ok {
			for _, nestedPanelInterface := range nestedPanelsInterface {
				if nestedPanel, ok := nestedPanelInterface.(map[string]interface{}); ok {
					if libraryPanel, ok := nestedPanel["libraryPanel"].(map[string]interface{}); ok {
						if uid, ok := libraryPanel["uid"].(string); ok {
							libraryUIDs = append(libraryUIDs, uid)
						}
					}
				}
			}
		}
	}

	return libraryUIDs, nil
}

func exportLibraryElement(uid string, basePath string, count *int, errors *[]string) error {
	url := fmt.Sprintf("%s/api/library-elements/%s", config.GrafanaURL, uid)
	library, err := fetchAPI[LibraryElementWithMeta](url)

	if err != nil {
		return fmt.Errorf("failed to fetch library element %s: %v", uid, err)
	}

	// Create folder structure
	var folderPath string
	if library.Result.FolderID == 0 {
		folderPath = filepath.Join(basePath, "General")
	} else {
		// Get folder name from cache or API
		folderName, ok := folderCache[library.Result.FolderUID]
		if !ok {
			folderURL := fmt.Sprintf("%s/api/folders/%s", config.GrafanaURL, library.Result.FolderUID)
			folder, err := fetchAPI[Folder](folderURL)
			if err != nil {
				folderName = "Unknown_" + library.Result.FolderUID
			} else {
				folderName = folder.Title
				folderCache[library.Result.FolderUID] = folderName
			}
		}
		folderPath = filepath.Join(basePath, sanitizePath(folderName))
	}

	if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create folder structure for library %s: %v", uid, err)
	}

	// Save library JSON
	filename := filepath.Join(folderPath, sanitizePath(library.Result.Name)+".json")
	libraryJSON, err := json.MarshalIndent(library.Result.Model, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal library element %s: %v", uid, err)
	}

	if err := os.WriteFile(filename, libraryJSON, 0644); err != nil {
		return fmt.Errorf("failed to write library element %s: %v", uid, err)
	}

	*count++
	return nil
}

func sanitizePath(path string) string {
	// Replace characters that are problematic in filenames
	sanitized := strings.ReplaceAll(path, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, "\\", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	sanitized = strings.ReplaceAll(sanitized, "*", "_")
	sanitized = strings.ReplaceAll(sanitized, "?", "_")
	sanitized = strings.ReplaceAll(sanitized, "\"", "_")
	sanitized = strings.ReplaceAll(sanitized, "<", "_")
	sanitized = strings.ReplaceAll(sanitized, ">", "_")
	sanitized = strings.ReplaceAll(sanitized, "|", "_")
	return sanitized
}

// Generic function to fetch and parse API responses

// Generic function to fetch and parse API responses
func fetchAPI[T any](url string) (T, error) {
	var result T

	// Create custom HTTP client with optional TLS verification skipping
	client := &http.Client{}
	if config.SkipTLSVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return result, err
	}

	req.Header.Add("Authorization", "Bearer "+config.GrafanaAPIKey)

	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return result, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, err
	}

	return result, nil
}

// Fetch raw API response and decode into provided target
func fetchAPIRaw(url string, target interface{}) error {
	// Create custom HTTP client with optional TLS verification skipping
	client := &http.Client{}
	if config.SkipTLSVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	// Try up to 3 times for nested folder endpoints which might be flaky
	maxRetries := 1
	if strings.Contains(url, "/children") {
		maxRetries = 3
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			log.Printf("Retrying API call (%d of %d): %s", i+1, maxRetries, url)
			time.Sleep(500 * time.Millisecond) // Add a small delay between retries
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Add("Authorization", "Bearer "+config.GrafanaAPIKey)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Read the entire response body
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		// Check status code
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))

			// Special case: If we're fetching subfolders and get a 404,
			// this might mean the API endpoint is different or not supported
			if strings.Contains(url, "/children") && resp.StatusCode == http.StatusNotFound {
				// Return an empty array for this case instead of an error
				if sliceTarget, ok := target.(*[]Folder); ok {
					*sliceTarget = []Folder{} // Set an empty slice
					return nil
				}
			}

			continue
		}

		// If the response is empty or just "[]", return an empty result for slice targets
		if len(bodyBytes) == 0 || string(bodyBytes) == "[]" {
			if sliceTarget, ok := target.(*[]Folder); ok {
				*sliceTarget = []Folder{} // Set an empty slice
				return nil
			}
		}

		// Debug response for nested folders
		if strings.Contains(url, "/children") {
			preview := string(bodyBytes)
			if len(preview) > 500 {
				preview = preview[:500] + "..."
			}
			log.Printf("Subfolder response: %s", preview)
		}

		// Try to decode the response
		err = json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(target)
		if err != nil {
			lastErr = fmt.Errorf("JSON decode error: %v (body: %s)", err, string(bodyBytes))
			continue
		}

		// Success
		return nil
	}

	// If we got here, all retries failed
	return lastErr
}

func getEnvFloat(key string, fallback float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return fallback
}

// setupStaticFiles sets up the static file server with embedded files
func setupStaticFiles(e *echo.Echo) {
	// Get the subdirectory from the embedded filesystem
	fsys, err := fs.Sub(publicFS, "public")
	if err != nil {
		log.Fatalf("Failed to get public subdirectory: %v", err)
	}

	// Use the filesystem for static file serving
	fileServer := http.FileServer(http.FS(fsys))
	e.GET("/*", echo.WrapHandler(fileServer))
}
