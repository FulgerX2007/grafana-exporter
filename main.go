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

type Config struct {
	GrafanaURL      string
	GrafanaAPIKey   string
	ExportDirectory string
	ServerHost      string
	ServerPort      string
	SkipTLSVerify   bool
	GrafanaVersion  float64 // Add this field
}

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

type DashboardResponse struct {
	Dashboards []Dashboard `json:"dashboards"`
}

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

type FolderResponse []Folder

type LibraryElement struct {
	ID        int    `json:"id"`
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Kind      int    `json:"kind"`
	FolderID  int    `json:"folderId"`
	FolderUID string `json:"folderUid"`
}

type LibraryElementsResponse struct {
	Result []LibraryElement `json:"result"`
}

type DashboardWithMeta struct {
	Dashboard map[string]interface{} `json:"dashboard"`
	Meta      struct {
		FolderID    int    `json:"folderId"`
		FolderUID   string `json:"folderUid"`
		FolderTitle string `json:"folderTitle"`
	} `json:"meta"`
}

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

type Alert struct {
	ID          int    `json:"id"`
	UID         string `json:"uid"`
	Title       string `json:"title"`
	FolderID    int    `json:"folderId"`
	FolderUID   string `json:"folderUid,omitempty"`
	FolderTitle string `json:"folderTitle,omitempty"`
}

type AlertResponse struct {
	Alerts []Alert `json:"alerts"`
}

var config Config
var folderCache map[string]string

func main() {
	initializationError := initialize()

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.GET("/api/folders", getFolders)
	e.GET("/api/dashboards", getDashboards)
	e.GET("/api/libraries", getLibraries)
	e.GET("/api/alerts", getAlerts)
	e.POST("/api/export", exportDashboards)

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

	setupStaticFiles(e)

	log.Printf("Server started on http://%s:%s", config.ServerHost, config.ServerPort)
	e.Logger.Fatal(e.Start(config.ServerHost + ":" + config.ServerPort))
}

func initialize() error {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
		if os.IsNotExist(err) {
			return fmt.Errorf("missing .env file: %v", err)
		}
	}

	config = Config{
		GrafanaURL:      getEnv("GRAFANA_URL", "http://localhost:3000"),
		GrafanaAPIKey:   getEnv("GRAFANA_API_KEY", ""),
		ExportDirectory: getEnv("EXPORT_DIRECTORY", "./exported"),
		ServerHost:      getEnv("SERVER_HOST", "127.0.0.1"),
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		SkipTLSVerify:   getEnvBool("SKIP_TLS_VERIFY", false),
		GrafanaVersion:  getEnvFloat("GRAFANA_VERSION", 11.1),
	}

	folderCache = make(map[string]string)

	if err := os.MkdirAll(config.ExportDirectory, os.ModePerm); err != nil {
		log.Fatalf("Failed to create export directory: %v", err)
	}

	log.Printf("Initialized with Grafana URL: %s", config.GrafanaURL)
	log.Printf("Export directory: %s", config.ExportDirectory)
	log.Printf("Server running on host and port: %s:%s", config.ServerHost, config.ServerPort)
	log.Printf("Grafana version: %.1f", config.GrafanaVersion)

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
	url := fmt.Sprintf("%s/api/folders?limit=1000", config.GrafanaURL)

	var topLevelFolders []Folder
	err := fetchAPIRaw(url, &topLevelFolders)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	log.Printf("Retrieved %d top-level folders from API", len(topLevelFolders))

	allFolders := make([]Folder, len(topLevelFolders))
	copy(allFolders, topLevelFolders)

	for _, folder := range topLevelFolders {
		folderCache[folder.UID] = folder.Title
	}

	processedFolders := make(map[string]bool)
	for _, folder := range topLevelFolders {
		processedFolders[folder.UID] = true
	}

	foldersToProcess := make([]Folder, len(topLevelFolders))
	copy(foldersToProcess, topLevelFolders)

	for len(foldersToProcess) > 0 {
		var nextWave []Folder

		for _, parentFolder := range foldersToProcess {
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

				for i := range childFolders {
					childFolders[i].ParentUID = parentFolder.UID
					folderCache[childFolders[i].UID] = childFolders[i].Title

					if !processedFolders[childFolders[i].UID] {
						allFolders = append(allFolders, childFolders[i])
						processedFolders[childFolders[i].UID] = true

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

		foldersToProcess = nextWave
		log.Printf("Next wave: %d folders to process", len(foldersToProcess))
	}

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

	dashboardsUrl := fmt.Sprintf("%s/api/search?type=dash-db&limit=5000", config.GrafanaURL)
	var searchResult []Dashboard
	err = fetchAPIRaw(dashboardsUrl, &searchResult)
	if err != nil {
		log.Printf("Warning: Could not get dashboard counts: %v", err)
	} else {
		folderDashboardCounts := make(map[int]int)
		for _, dash := range searchResult {
			if dash.Type == "" || dash.Type == "dash-db" {
				folderDashboardCounts[dash.FolderID]++
			}
		}

		for i := range allFolders {
			count := folderDashboardCounts[allFolders[i].ID]
			allFolders[i].DashboardCount = count
		}
	}

	return c.JSON(http.StatusOK, allFolders)
}

func getDashboards(c echo.Context) error {
	url := fmt.Sprintf("%s/api/search?type=dash-db&limit=5000", config.GrafanaURL)

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

	var dashboardsOnly []Dashboard
	for _, item := range searchResult {
		if item.Type == "" || item.Type == "dash-db" {
			if item.Title != "" && item.UID != "" {
				dashboardsOnly = append(dashboardsOnly, item)
			}
		}
	}

	log.Printf("Filtered to %d actual dashboards", len(dashboardsOnly))

	response := DashboardResponse{
		Dashboards: dashboardsOnly,
	}

	for i, dash := range response.Dashboards {
		if dash.FolderID == 0 {
			generalStr := "General"
			response.Dashboards[i].FolderName = &generalStr
			continue
		}

		if dash.FolderName == nil || *dash.FolderName == "" {
			if dash.FolderUID != "" {
				folderName, ok := folderCache[dash.FolderUID]
				if ok {
					response.Dashboards[i].FolderName = &folderName
				} else {
					folderURL := fmt.Sprintf("%s/api/folders/%s", config.GrafanaURL, dash.FolderUID)
					var folder Folder
					if err := fetchAPIRaw(folderURL, &folder); err == nil {
						folderCache[folder.UID] = folder.Title
						response.Dashboards[i].FolderName = &folder.Title
					} else {
						unknown := fmt.Sprintf("Folder ID %d", dash.FolderID)
						response.Dashboards[i].FolderName = &unknown
					}
				}
			} else {
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

func getAlerts(c echo.Context) error {
	var alertRules []Alert
	var err error

	url := fmt.Sprintf("%s/api/v1/provisioning/alert-rules", config.GrafanaURL)
	err = fetchAPIRaw(url, &alertRules)

	if err != nil {
		legacyURL := fmt.Sprintf("%s/api/alerts", config.GrafanaURL)
		err = fetchAPIRaw(legacyURL, &alertRules)

		if err != nil {
			log.Printf("Warning: Could not fetch alerts: %v", err)
			return c.JSON(http.StatusOK, AlertResponse{Alerts: []Alert{}})
		}
	}

	log.Printf("Retrieved %d alert rules from API", len(alertRules))

	for i := range alertRules {
		if alertRules[i].FolderID == 0 {
			alertRules[i].FolderTitle = "General"
		} else if alertRules[i].FolderUID != "" {
			folderName, ok := folderCache[alertRules[i].FolderUID]
			if ok {
				alertRules[i].FolderTitle = folderName
			} else {
				folderURL := fmt.Sprintf("%s/api/folders/%s", config.GrafanaURL, alertRules[i].FolderUID)
				var folder Folder
				if err := fetchAPIRaw(folderURL, &folder); err == nil {
					folderCache[folder.UID] = folder.Title
					alertRules[i].FolderTitle = folder.Title
				} else {
					alertRules[i].FolderTitle = fmt.Sprintf("Folder ID %d", alertRules[i].FolderID)
				}
			}
		}
	}

	return c.JSON(http.StatusOK, AlertResponse{Alerts: alertRules})
}

func exportDashboards(c echo.Context) error {
	var req struct {
		DashboardUIDs []string `json:"dashboardUIDs"`
		AlertUIDs     []string `json:"alertUIDs"`
		IncludeAlerts bool     `json:"includeAlerts"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request format"})
	}

	if len(req.DashboardUIDs) == 0 && len(req.AlertUIDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No dashboards or alerts selected"})
	}

	timestamp := time.Now().Format("20060102_150405")
	exportPath := filepath.Join(config.ExportDirectory, timestamp)

	if err := os.MkdirAll(exportPath, os.ModePerm); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create export directory"})
	}

	exportedLibraries := make(map[string]bool)
	exportResult := struct {
		ExportedDashboards int      `json:"exportedDashboards"`
		ExportedLibraries  int      `json:"exportedLibraries"`
		ExportedAlerts     int      `json:"exportedAlerts"`
		Errors             []string `json:"errors"`
		ExportPath         string   `json:"exportPath"`
	}{
		Errors:     []string{},
		ExportPath: exportPath,
	}

	for _, uid := range req.DashboardUIDs {
		dashURL := fmt.Sprintf("%s/api/dashboards/uid/%s", config.GrafanaURL, uid)
		dashboard, err := fetchAPI[DashboardWithMeta](dashURL)

		if err != nil {
			exportResult.Errors = append(exportResult.Errors, fmt.Sprintf("Failed to fetch dashboard %s: %v", uid, err))
			continue
		}

		var folderPath string
		if dashboard.Meta.FolderID == 0 {
			folderPath = filepath.Join(exportPath, "General")
		} else {
			folderName := dashboard.Meta.FolderTitle
			folderPath = filepath.Join(exportPath, sanitizePath(folderName))
		}

		if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
			exportResult.Errors = append(
				exportResult.Errors,
				fmt.Sprintf("Failed to create folder structure for %s: %v", uid, err),
			)
			continue
		}

		dashboardTitle, ok := dashboard.Dashboard["title"].(string)
		if !ok {
			dashboardTitle = uid
		}

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

		libraryPanels, err := extractLibraryPanelUIDs(dashboard.Dashboard)
		if err != nil {
			exportResult.Errors = append(
				exportResult.Errors,
				fmt.Sprintf("Failed to extract library panels from %s: %v", uid, err),
			)
		}

		for _, libraryUID := range libraryPanels {
			if exportedLibraries[libraryUID] {
				continue
			}

			if err := exportLibraryElement(
				libraryUID,
				folderPath, // Use the same folder as the dashboard
				&exportResult.ExportedLibraries,
				&exportResult.Errors,
			); err != nil {
				exportResult.Errors = append(exportResult.Errors, err.Error())
				continue
			}

			exportedLibraries[libraryUID] = true
		}
	}

	if req.IncludeAlerts {
		for _, uid := range req.AlertUIDs {
			alertURL := fmt.Sprintf("%s/api/v1/provisioning/alert-rules/%s", config.GrafanaURL, uid)
			var alert map[string]interface{}
			err := fetchAPIRaw(alertURL, &alert)

			if err != nil {
				legacyURL := fmt.Sprintf("%s/api/alerts/%s", config.GrafanaURL, uid)
				err = fetchAPIRaw(legacyURL, &alert)
			}

			if err != nil {
				exportResult.Errors = append(exportResult.Errors, fmt.Sprintf("Failed to fetch alert %s: %v", uid, err))
				continue
			}

			alertsPath := filepath.Join(exportPath, "Alerts")
			if err := os.MkdirAll(alertsPath, os.ModePerm); err != nil {
				exportResult.Errors = append(exportResult.Errors, fmt.Sprintf("Failed to create alerts folder: %v", err))
				continue
			}

			var alertTitle string
			if title, ok := alert["title"].(string); ok {
				alertTitle = title
			} else {
				alertTitle = uid
			}

			filename := filepath.Join(alertsPath, sanitizePath(alertTitle)+".json")
			alertJSON, err := json.MarshalIndent(alert, "", "  ")
			if err != nil {
				exportResult.Errors = append(exportResult.Errors, fmt.Sprintf("Failed to marshal alert %s: %v", uid, err))
				continue
			}

			if err := os.WriteFile(filename, alertJSON, 0644); err != nil {
				exportResult.Errors = append(exportResult.Errors, fmt.Sprintf("Failed to write alert %s: %v", uid, err))
				continue
			}

			exportResult.ExportedAlerts++
		}
	}

	return c.JSON(http.StatusOK, exportResult)
}

func extractLibraryPanelUIDs(dashboard map[string]interface{}) ([]string, error) {
	libraryUIDs := make([]string, 0)

	panelsInterface, ok := dashboard["panels"]
	if !ok {
		return libraryUIDs, nil
	}

	panels, ok := panelsInterface.([]interface{})
	if !ok {
		return libraryUIDs, fmt.Errorf("panels is not an array")
	}

	for _, panelInterface := range panels {
		panel, ok := panelInterface.(map[string]interface{})
		if !ok {
			continue
		}

		if libraryPanel, ok := panel["libraryPanel"].(map[string]interface{}); ok {
			if uid, ok := libraryPanel["uid"].(string); ok {
				libraryUIDs = append(libraryUIDs, uid)
			}
		}

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

	var folderPath string
	if library.Result.FolderID == 0 {
		folderPath = filepath.Join(basePath, "General")
	} else {
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

	libraryElementExport := map[string]interface{}{
		"folderUid": library.Result.FolderUID,
		"name":      library.Result.Name,
		"model":     library.Result.Model,
		"kind":      library.Result.Kind,
		"uid":       library.Result.UID,
	}

	filename := filepath.Join(folderPath, sanitizePath(library.Result.Name)+".json")
	libraryJSON, err := json.MarshalIndent(libraryElementExport, "", "  ")
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

func fetchAPI[T any](url string) (T, error) {
	var result T

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

func fetchAPIRaw(url string, target interface{}) error {
	client := &http.Client{}
	if config.SkipTLSVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	maxRetries := 1
	if strings.Contains(url, "/children") {
		maxRetries = 3
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			log.Printf("Retrying API call (%d of %d): %s", i+1, maxRetries, url)
			time.Sleep(500 * time.Millisecond)
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

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))

			// Special case: If we're fetching subfolders and get a 404,
			// this might mean the API endpoint is different or not supported
			if strings.Contains(url, "/children") && resp.StatusCode == http.StatusNotFound {
				if sliceTarget, ok := target.(*[]Folder); ok {
					*sliceTarget = []Folder{}
					return nil
				}
			}

			continue
		}

		if len(bodyBytes) == 0 || string(bodyBytes) == "[]" {
			if sliceTarget, ok := target.(*[]Folder); ok {
				*sliceTarget = []Folder{}
				return nil
			}
		}

		if strings.Contains(url, "/children") {
			preview := string(bodyBytes)
			if len(preview) > 500 {
				preview = preview[:500] + "..."
			}
			log.Printf("Subfolder response: %s", preview)
		}

		err = json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(target)
		if err != nil {
			lastErr = fmt.Errorf("JSON decode error: %v (body: %s)", err, string(bodyBytes))
			continue
		}

		return nil
	}

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

func setupStaticFiles(e *echo.Echo) {
	fsys, err := fs.Sub(publicFS, "public")
	if err != nil {
		log.Fatalf("Failed to get public subdirectory: %v", err)
	}

	fileServer := http.FileServer(http.FS(fsys))
	e.GET("/*", echo.WrapHandler(fileServer))
}
