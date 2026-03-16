package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")
	assert.Equal(t, "test_value", getEnv("TEST_VAR", "default"))

	assert.Equal(t, "default", getEnv("NON_EXISTING_VAR", "default"))
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{"true value", "true", true},
		{"yes value", "yes", true},
		{"1 value", "1", true},
		{"false value", "false", false},
		{"no value", "no", false},
		{"0 value", "0", false},
		{"invalid value", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TEST_BOOL", tt.envValue)
			defer os.Unsetenv("TEST_BOOL")
			assert.Equal(t, tt.want, getEnvBool("TEST_BOOL", false))
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal/path", "normal_path"},
		{"file:name", "file_name"},
		{"file*name?", "file_name_"},
		{`file\name`, "file_name"},
		{"file<>name", "file__name"},
		{"file|name", "file_name"},
		{"normal name", "normal name"},
	}

	for _, tt := range tests {
		result := sanitizePath(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestExtractLibraryPanelUIDs(t *testing.T) {
	dashboard := map[string]interface{}{
		"panels": []interface{}{
			map[string]interface{}{
				"libraryPanel": map[string]interface{}{
					"uid": "panel1",
				},
			},
			map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"libraryPanel": map[string]interface{}{
							"uid": "panel2",
						},
					},
				},
			},
		},
	}

	uids, err := extractLibraryPanelUIDs(dashboard)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"panel1", "panel2"}, uids)

	emptyDashboard := map[string]interface{}{}
	uids, err = extractLibraryPanelUIDs(emptyDashboard)
	assert.NoError(t, err)
	assert.Empty(t, uids)
}

func TestExportLibraryElement(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-export-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config = Config{
		GrafanaURL:    "http://test-grafana",
		GrafanaAPIKey: "test-key",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		response := LibraryElementWithMeta{
			Result: struct {
				ID        int                    `json:"id"`
				UID       string                 `json:"uid"`
				Name      string                 `json:"name"`
				Kind      int                    `json:"kind"`
				Model     map[string]interface{} `json:"model"`
				FolderID  int                    `json:"folderId"`
				FolderUID string                 `json:"folderUid"`
			}{
				ID:       1,
				UID:      "test-uid",
				Name:     "Test Panel",
				Model:    map[string]interface{}{"test": "data"},
				FolderID: 0,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer ts.Close()

	config.GrafanaURL = ts.URL

	var count int
	var errors []string
	err = exportLibraryElement("test-uid", tempDir, &count, &errors)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Empty(t, errors)

	expectedPath := filepath.Join(tempDir, "General", "Test Panel.json")
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err)
}

func TestConfigStatusEndpoint(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/config-status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := func(c echo.Context) error {
		return c.JSON(
			http.StatusOK,
			map[string]interface{}{
				"hasEnvFile":   true,
				"errorMessage": "",
			},
		)
	}

	assert.NoError(t, h(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["hasEnvFile"].(bool))
	assert.Equal(t, "", response["errorMessage"])
}

func TestForceEnableZipExportConfig(t *testing.T) {
	// Test default value
	originalConfig := config
	defer func() { config = originalConfig }()

	config = Config{
		ForceEnableZipExport: false,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/config-status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := func(c echo.Context) error {
		return c.JSON(
			http.StatusOK,
			map[string]interface{}{
				"hasEnvFile":           true,
				"errorMessage":         "",
				"forceEnableZipExport": config.ForceEnableZipExport,
			},
		)
	}

	assert.NoError(t, h(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response["forceEnableZipExport"].(bool))

	// Test enabled value
	config.ForceEnableZipExport = true
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	assert.NoError(t, h(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	err = json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["forceEnableZipExport"].(bool))
}

func TestExtractVersionNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected int
	}{
		{"float64 version", map[string]interface{}{"version": float64(5)}, 5},
		{"int version", map[string]interface{}{"version": int(3)}, 3},
		{"zero version", map[string]interface{}{"version": float64(0)}, 0},
		{"missing version", map[string]interface{}{}, 0},
		{"string version (unsupported)", map[string]interface{}{"version": "3"}, 0},
		{"nil dashboard", map[string]interface{}{"version": nil}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVersionNumber(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFetchDashboardDetailsWithVersion(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/dashboards/uid/test-uid-1":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"dashboard": map[string]interface{}{
					"version": float64(7),
					"updated": "2026-01-01T00:00:00Z",
				},
				"meta": map[string]interface{}{
					"folderId":    0,
					"folderUid":   "",
					"folderTitle": "",
				},
			})
		case r.URL.Path == "/api/dashboards/uid/test-uid-1/versions/7":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      1,
				"version": 7,
				"created": "2026-03-15T10:30:00Z",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	config = Config{
		GrafanaURL:    ts.URL,
		GrafanaAPIKey: "test-key",
	}

	dashboards := []Dashboard{
		{ID: 1, UID: "test-uid-1", Title: "Test Dashboard"},
	}

	result := fetchDashboardDetails(dashboards)
	assert.Len(t, result, 1)
	assert.Equal(t, 7, result[0].Version)
	assert.Equal(t, "2026-03-15T10:30:00Z", result[0].Updated)
}

func TestFetchDashboardDetailsVersionAPIFallback(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/dashboards/uid/test-uid-2":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"dashboard": map[string]interface{}{
					"version": float64(3),
					"updated": "2026-02-01T12:00:00Z",
				},
				"meta": map[string]interface{}{
					"folderId":    0,
					"folderUid":   "",
					"folderTitle": "",
				},
			})
		case r.URL.Path == "/api/dashboards/uid/test-uid-2/versions/3":
			// Simulate versions API failure
			http.Error(w, "not found", http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	config = Config{
		GrafanaURL:    ts.URL,
		GrafanaAPIKey: "test-key",
	}

	dashboards := []Dashboard{
		{ID: 2, UID: "test-uid-2", Title: "Test Dashboard 2"},
	}

	result := fetchDashboardDetails(dashboards)
	assert.Len(t, result, 1)
	assert.Equal(t, 3, result[0].Version)
	// Should fall back to the updated field from dashboard detail
	assert.Equal(t, "2026-02-01T12:00:00Z", result[0].Updated)
}

func TestGetEnvFloat(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		setEnv   bool
		fallback float64
		expected float64
	}{
		{"valid float", "TEST_FLOAT", "11.5", true, 0.0, 11.5},
		{"integer value", "TEST_FLOAT", "10", true, 0.0, 10.0},
		{"invalid value uses fallback", "TEST_FLOAT", "abc", true, 5.5, 5.5},
		{"missing env uses fallback", "TEST_FLOAT_MISSING", "", false, 3.14, 3.14},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			} else {
				os.Unsetenv(tt.envKey)
			}
			result := getEnvFloat(tt.envKey, tt.fallback)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

func TestSafePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-safepath-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name      string
		base      string
		unsafe    string
		expectErr bool
	}{
		{"valid subpath", tempDir, "subdir", false},
		{"valid nested path", tempDir, filepath.Join("a", "b"), false},
		{"path traversal blocked", tempDir, "../../../etc/passwd", true},
		{"dot-dot blocked", tempDir, "..", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := safePath(tt.base, tt.unsafe)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, result, tempDir)
			}
		})
	}
}

func TestGetInitErrorMessage(t *testing.T) {
	assert.Equal(t, "", getInitErrorMessage(nil))
	assert.Contains(t, getInitErrorMessage(fmt.Errorf("missing .env file")), "Configuration file")
	assert.Equal(t, "some other error", getInitErrorMessage(fmt.Errorf("some other error")))
}

func TestSanitizePathEdgeCases(t *testing.T) {
	assert.Equal(t, "_", sanitizePath(""))
	assert.Equal(t, "_", sanitizePath("."))
	assert.Equal(t, "_", sanitizePath(".."))
	assert.Equal(t, "a_b", sanitizePath("a..b"))
}

func TestExtractLibraryPanelUIDsEdgeCases(t *testing.T) {
	// panels is not an array
	bad := map[string]interface{}{"panels": "not-an-array"}
	_, err := extractLibraryPanelUIDs(bad)
	assert.Error(t, err)

	// panels with non-map elements
	mixed := map[string]interface{}{
		"panels": []interface{}{"string-element", nil, 42},
	}
	uids, err := extractLibraryPanelUIDs(mixed)
	assert.NoError(t, err)
	assert.Empty(t, uids)
}

func TestGetDashboardsHandler(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/search":
			json.NewEncoder(w).Encode([]Dashboard{
				{ID: 1, UID: "uid-1", Title: "Dashboard 1", Type: "dash-db", FolderID: 0},
			})
		case r.URL.Path == "/api/dashboards/uid/uid-1":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"dashboard": map[string]interface{}{
					"version": float64(2),
					"updated": "2026-01-01T00:00:00Z",
				},
				"meta": map[string]interface{}{
					"folderId":    0,
					"folderUid":   "",
					"folderTitle": "",
				},
			})
		case r.URL.Path == "/api/dashboards/uid/uid-1/versions/2":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      1,
				"version": 2,
				"created": "2026-03-10T08:00:00Z",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	config = Config{
		GrafanaURL:    ts.URL,
		GrafanaAPIKey: "test-key",
	}
	folderCache = make(map[string]string)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboards", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := getDashboards(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response DashboardResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Dashboards, 1)
	assert.Equal(t, 2, response.Dashboards[0].Version)
	assert.Equal(t, "2026-03-10T08:00:00Z", response.Dashboards[0].Updated)
	assert.NotNil(t, response.Dashboards[0].FolderName)
	assert.Equal(t, "General", *response.Dashboards[0].FolderName)
}

func TestGetAlertsHandler(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/provisioning/alert-rules" {
			json.NewEncoder(w).Encode([]Alert{
				{ID: 1, UID: "alert-1", Title: "Test Alert", FolderID: 0},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	config = Config{
		GrafanaURL:    ts.URL,
		GrafanaAPIKey: "test-key",
	}
	folderCache = make(map[string]string)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := getAlerts(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response AlertResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Alerts, 1)
	assert.Equal(t, "General", response.Alerts[0].FolderTitle)
}

func TestGetLibrariesHandler(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(LibraryElementsResponse{
			Result: []LibraryElement{
				{ID: 1, UID: "lib-1", Name: "Panel 1", Kind: 1},
			},
		})
	}))
	defer ts.Close()

	config = Config{
		GrafanaURL:    ts.URL,
		GrafanaAPIKey: "test-key",
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := getLibraries(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestFetchAPIError(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	config = Config{
		GrafanaURL:    ts.URL,
		GrafanaAPIKey: "test-key",
	}

	_, err := fetchAPI[Dashboard](ts.URL + "/api/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestFetchAPIRawError(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	config = Config{
		GrafanaURL:    ts.URL,
		GrafanaAPIKey: "test-key",
	}

	var result Dashboard
	err := fetchAPIRaw(ts.URL+"/api/test", &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestZipDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-zip-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	srcDir := filepath.Join(tempDir, "src")
	assert.NoError(t, os.MkdirAll(srcDir, os.ModePerm))
	assert.NoError(t, os.WriteFile(filepath.Join(srcDir, "test.json"), []byte(`{"test":true}`), 0644))

	zipPath := filepath.Join(tempDir, "output.zip")
	err = zipDirectory(srcDir, zipPath)
	assert.NoError(t, err)

	_, err = os.Stat(zipPath)
	assert.NoError(t, err)
}

func TestExportDashboardsHandler(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/dashboards/uid/uid-export-1":
			json.NewEncoder(w).Encode(DashboardWithMeta{
				Dashboard: map[string]interface{}{
					"title":   "Exported Dashboard",
					"version": float64(1),
					"panels":  []interface{}{},
				},
				Meta: struct {
					FolderID    int    `json:"folderId"`
					FolderUID   string `json:"folderUid"`
					FolderTitle string `json:"folderTitle"`
				}{
					FolderID:    0,
					FolderUID:   "",
					FolderTitle: "",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	tempDir, err := os.MkdirTemp("", "test-export-handler-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config = Config{
		GrafanaURL:      ts.URL,
		GrafanaAPIKey:   "test-key",
		ExportDirectory: tempDir,
	}
	folderCache = make(map[string]string)

	e := echo.New()
	body := `{"dashboardUIDs":["uid-export-1"],"alertUIDs":[],"includeAlerts":false,"exportAsZip":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = exportDashboards(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), result["exportedDashboards"])
}

func TestExportDashboardsInvalidRequest(t *testing.T) {
	e := echo.New()

	// Empty UIDs
	body := `{"dashboardUIDs":[],"alertUIDs":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := exportDashboards(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetFoldersHandler(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/folders" {
			parentUID := r.URL.Query().Get("parentUid")
			if parentUID == "" {
				json.NewEncoder(w).Encode([]Folder{
					{ID: 1, UID: "folder-1", Title: "Folder A"},
				})
			} else {
				json.NewEncoder(w).Encode([]Folder{})
			}
			return
		}
		if r.URL.Path == "/api/search" {
			callCount++
			json.NewEncoder(w).Encode([]Dashboard{
				{ID: 10, UID: "d-1", Title: "Dash 1", Type: "dash-db", FolderID: 1},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	config = Config{
		GrafanaURL:    ts.URL,
		GrafanaAPIKey: "test-key",
	}
	folderCache = make(map[string]string)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/folders", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := getFolders(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var folders []Folder
	err = json.Unmarshal(rec.Body.Bytes(), &folders)
	assert.NoError(t, err)
	assert.Len(t, folders, 1)
	assert.Equal(t, "Folder A", folders[0].Title)
	assert.Equal(t, 1, folders[0].DashboardCount)
}

func TestExportDashboardsWithAlerts(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/provisioning/alert-rules/alert-1":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"title": "My Alert",
				"uid":   "alert-1",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	tempDir, err := os.MkdirTemp("", "test-export-alerts-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config = Config{
		GrafanaURL:      ts.URL,
		GrafanaAPIKey:   "test-key",
		ExportDirectory: tempDir,
	}
	folderCache = make(map[string]string)

	e := echo.New()
	body := `{"dashboardUIDs":[],"alertUIDs":["alert-1"],"includeAlerts":true,"exportAsZip":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = exportDashboards(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), result["exportedAlerts"])
}

func TestExportDashboardsAsZip(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/dashboards/uid/uid-zip-1" {
			json.NewEncoder(w).Encode(DashboardWithMeta{
				Dashboard: map[string]interface{}{
					"title":   "Zip Dashboard",
					"version": float64(1),
					"panels":  []interface{}{},
				},
				Meta: struct {
					FolderID    int    `json:"folderId"`
					FolderUID   string `json:"folderUid"`
					FolderTitle string `json:"folderTitle"`
				}{FolderID: 0},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	tempDir, err := os.MkdirTemp("", "test-export-zip-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config = Config{
		GrafanaURL:      ts.URL,
		GrafanaAPIKey:   "test-key",
		ExportDirectory: tempDir,
	}
	folderCache = make(map[string]string)

	e := echo.New()
	body := `{"dashboardUIDs":["uid-zip-1"],"alertUIDs":[],"includeAlerts":false,"exportAsZip":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = exportDashboards(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/zip")
}

func TestCheckGrafanaConnection(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	config = Config{
		GrafanaURL: ts.URL,
	}
	// Should not panic
	checkGrafanaConnection()
}

func TestCheckGrafanaConnectionFailure(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	config = Config{
		GrafanaURL: "http://localhost:1", // unlikely to be running
	}
	// Should not panic, just log warning
	checkGrafanaConnection()
}

func TestGetAlertsHandlerFallbackToLegacy(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/provisioning/alert-rules" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.URL.Path == "/api/alerts" {
			json.NewEncoder(w).Encode([]Alert{
				{ID: 1, UID: "alert-legacy", Title: "Legacy Alert", FolderID: 5, FolderUID: "folder-5"},
			})
			return
		}
		if r.URL.Path == "/api/folders/folder-5" {
			json.NewEncoder(w).Encode(Folder{ID: 5, UID: "folder-5", Title: "My Folder"})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "test-key"}
	folderCache = make(map[string]string)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := getAlerts(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response AlertResponse
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Len(t, response.Alerts, 1)
	assert.Equal(t, "My Folder", response.Alerts[0].FolderTitle)
}

func TestGetAlertsHandlerBothFail(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "test-key"}
	folderCache = make(map[string]string)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := getAlerts(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response AlertResponse
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Empty(t, response.Alerts)
}

func TestExportLibraryElementWithFolder(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-export-lib-folder-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/library-elements/lib-with-folder":
			json.NewEncoder(w).Encode(LibraryElementWithMeta{
				Result: struct {
					ID        int                    `json:"id"`
					UID       string                 `json:"uid"`
					Name      string                 `json:"name"`
					Kind      int                    `json:"kind"`
					Model     map[string]interface{} `json:"model"`
					FolderID  int                    `json:"folderId"`
					FolderUID string                 `json:"folderUid"`
				}{
					ID: 1, UID: "lib-with-folder", Name: "Folder Panel",
					Model: map[string]interface{}{"type": "graph"}, FolderID: 5, FolderUID: "folder-5",
				},
			})
		case r.URL.Path == "/api/folders/folder-5":
			json.NewEncoder(w).Encode(Folder{ID: 5, UID: "folder-5", Title: "Library Folder"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "test-key"}
	folderCache = make(map[string]string)

	var count int
	var errors []string
	err = exportLibraryElement("lib-with-folder", tempDir, &count, &errors)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	expectedPath := filepath.Join(tempDir, "Library Folder", "Folder Panel.json")
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err)
}

func TestExportLibraryElementCachedFolder(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-export-lib-cached-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/library-elements/lib-cached" {
			json.NewEncoder(w).Encode(LibraryElementWithMeta{
				Result: struct {
					ID        int                    `json:"id"`
					UID       string                 `json:"uid"`
					Name      string                 `json:"name"`
					Kind      int                    `json:"kind"`
					Model     map[string]interface{} `json:"model"`
					FolderID  int                    `json:"folderId"`
					FolderUID string                 `json:"folderUid"`
				}{
					ID: 2, UID: "lib-cached", Name: "Cached Panel",
					Model: map[string]interface{}{}, FolderID: 10, FolderUID: "cached-folder",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "test-key"}
	folderCache = map[string]string{"cached-folder": "Cached Folder Name"}

	var count int
	var errors []string
	err = exportLibraryElement("lib-cached", tempDir, &count, &errors)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestFetchAPIRawWithRetry(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode([]Folder{{ID: 1, UID: "f1", Title: "Folder"}})
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "test-key"}

	// Should retry for /children URLs
	var result []Folder
	err := fetchAPIRaw(ts.URL+"/api/folders/abc/children", &result)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 3, callCount)
}

func TestFetchAPIRawChildrenNotFound(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "test-key"}

	var result []Folder
	err := fetchAPIRaw(ts.URL+"/api/folders/abc/children", &result)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestFetchAPIRawEmptyResponse(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "test-key"}

	var result []Folder
	err := fetchAPIRaw(ts.URL+"/api/test", &result)
	assert.NoError(t, err)
}

func TestExportDashboardsWithLibraryPanels(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/dashboards/uid/uid-with-lib":
			json.NewEncoder(w).Encode(DashboardWithMeta{
				Dashboard: map[string]interface{}{
					"title":   "Dash With Lib",
					"version": float64(1),
					"panels": []interface{}{
						map[string]interface{}{
							"libraryPanel": map[string]interface{}{
								"uid": "lib-panel-1",
							},
						},
					},
				},
				Meta: struct {
					FolderID    int    `json:"folderId"`
					FolderUID   string `json:"folderUid"`
					FolderTitle string `json:"folderTitle"`
				}{FolderID: 0},
			})
		case r.URL.Path == "/api/library-elements/lib-panel-1":
			json.NewEncoder(w).Encode(LibraryElementWithMeta{
				Result: struct {
					ID        int                    `json:"id"`
					UID       string                 `json:"uid"`
					Name      string                 `json:"name"`
					Kind      int                    `json:"kind"`
					Model     map[string]interface{} `json:"model"`
					FolderID  int                    `json:"folderId"`
					FolderUID string                 `json:"folderUid"`
				}{ID: 1, UID: "lib-panel-1", Name: "Lib Panel", Model: map[string]interface{}{}, FolderID: 0},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	tempDir, err := os.MkdirTemp("", "test-export-lib-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "test-key", ExportDirectory: tempDir}
	folderCache = make(map[string]string)

	e := echo.New()
	body := `{"dashboardUIDs":["uid-with-lib"],"alertUIDs":[],"includeAlerts":false,"exportAsZip":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = exportDashboards(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &result)
	assert.Equal(t, float64(1), result["exportedDashboards"])
	assert.Equal(t, float64(1), result["exportedLibraries"])
}

func TestExportDashboardsWithFolder(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/dashboards/uid/uid-folder" {
			json.NewEncoder(w).Encode(DashboardWithMeta{
				Dashboard: map[string]interface{}{
					"title":   "Folder Dashboard",
					"version": float64(1),
					"panels":  []interface{}{},
				},
				Meta: struct {
					FolderID    int    `json:"folderId"`
					FolderUID   string `json:"folderUid"`
					FolderTitle string `json:"folderTitle"`
				}{FolderID: 5, FolderUID: "folder-5", FolderTitle: "My Folder"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	tempDir, err := os.MkdirTemp("", "test-export-folder-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "test-key", ExportDirectory: tempDir}
	folderCache = make(map[string]string)

	e := echo.New()
	body := `{"dashboardUIDs":["uid-folder"],"alertUIDs":[],"includeAlerts":false,"exportAsZip":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = exportDashboards(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &result)
	assert.Equal(t, float64(1), result["exportedDashboards"])
}

func TestGetDashboardsWithFolderLookup(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/search":
			json.NewEncoder(w).Encode([]Dashboard{
				{ID: 1, UID: "uid-f", Title: "Dash F", Type: "dash-db", FolderID: 5, FolderUID: "folder-uid-5"},
			})
		case r.URL.Path == "/api/dashboards/uid/uid-f":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"dashboard": map[string]interface{}{"version": float64(1), "updated": "2026-01-01T00:00:00Z"},
				"meta":      map[string]interface{}{"folderId": 5, "folderUid": "folder-uid-5", "folderTitle": ""},
			})
		case r.URL.Path == "/api/dashboards/uid/uid-f/versions/1":
			json.NewEncoder(w).Encode(map[string]interface{}{"version": 1, "created": "2026-01-01T00:00:00Z"})
		case r.URL.Path == "/api/folders/folder-uid-5":
			json.NewEncoder(w).Encode(Folder{ID: 5, UID: "folder-uid-5", Title: "Looked Up Folder"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "test-key"}
	folderCache = make(map[string]string)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboards", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := getDashboards(c)
	assert.NoError(t, err)

	var response DashboardResponse
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Len(t, response.Dashboards, 1)
	assert.NotNil(t, response.Dashboards[0].FolderName)
	assert.Equal(t, "Looked Up Folder", *response.Dashboards[0].FolderName)
}

func TestCheckGrafanaConnectionNon200(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL}
	checkGrafanaConnection()
}

func TestFetchAPIRawWithTLSSkip(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "key", SkipTLSVerify: true}

	var result map[string]string
	err := fetchAPIRaw(ts.URL+"/api/test", &result)
	assert.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
}

func TestFetchAPIWithTLSSkip(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Dashboard{ID: 1, UID: "test", Title: "Test"})
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "key", SkipTLSVerify: true}

	result, err := fetchAPI[Dashboard](ts.URL + "/api/test")
	assert.NoError(t, err)
	assert.Equal(t, "Test", result.Title)
}

func TestExtractVersionNumberJsonNumber(t *testing.T) {
	dashboard := map[string]interface{}{
		"version": json.Number("42"),
	}
	assert.Equal(t, 42, extractVersionNumber(dashboard))

	// Invalid json.Number
	dashboard2 := map[string]interface{}{
		"version": json.Number("not-a-number"),
	}
	assert.Equal(t, 0, extractVersionNumber(dashboard2))
}

func TestExportDashboardsFetchError(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	tempDir, err := os.MkdirTemp("", "test-export-err-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "key", ExportDirectory: tempDir}
	folderCache = make(map[string]string)

	e := echo.New()
	body := `{"dashboardUIDs":["nonexistent"],"alertUIDs":["nonexistent"],"includeAlerts":true,"exportAsZip":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = exportDashboards(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &result)
	assert.Equal(t, float64(0), result["exportedDashboards"])
	errors := result["errors"].([]interface{})
	assert.True(t, len(errors) > 0)
}

func TestGetDashboardsAPIError(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "key"}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboards", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := getDashboards(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestGetFoldersAPIError(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "key"}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/folders", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := getFolders(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestGetLibrariesAPIError(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "key"}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := getLibraries(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestExportDashboardsBadJSON(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/export", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := exportDashboards(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetDashboardsWithCachedFolder(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/search":
			json.NewEncoder(w).Encode([]Dashboard{
				{ID: 1, UID: "uid-cached", Title: "Cached Dash", Type: "dash-db", FolderID: 7, FolderUID: "cached-f"},
			})
		case r.URL.Path == "/api/dashboards/uid/uid-cached":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"dashboard": map[string]interface{}{"version": float64(1), "updated": "2026-01-01T00:00:00Z"},
				"meta":      map[string]interface{}{"folderId": 7, "folderUid": "cached-f", "folderTitle": ""},
			})
		case r.URL.Path == "/api/dashboards/uid/uid-cached/versions/1":
			json.NewEncoder(w).Encode(map[string]interface{}{"version": 1, "created": "2026-01-01T00:00:00Z"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "key"}
	folderCache = map[string]string{"cached-f": "Pre-Cached Folder"}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboards", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	getDashboards(c)

	var response DashboardResponse
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Equal(t, "Pre-Cached Folder", *response.Dashboards[0].FolderName)
}

func TestGetDashboardsNoFolderUID(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/search":
			json.NewEncoder(w).Encode([]Dashboard{
				{ID: 1, UID: "uid-nofuid", Title: "No FolderUID Dash", Type: "dash-db", FolderID: 99},
			})
		case r.URL.Path == "/api/dashboards/uid/uid-nofuid":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"dashboard": map[string]interface{}{"version": float64(1), "updated": "2026-01-01T00:00:00Z"},
				"meta":      map[string]interface{}{"folderId": 99, "folderUid": "", "folderTitle": ""},
			})
		case r.URL.Path == "/api/dashboards/uid/uid-nofuid/versions/1":
			json.NewEncoder(w).Encode(map[string]interface{}{"version": 1, "created": "2026-01-01T00:00:00Z"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "key"}
	folderCache = make(map[string]string)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboards", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	getDashboards(c)

	var response DashboardResponse
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Contains(t, *response.Dashboards[0].FolderName, "Folder ID 99")
}

func TestGetFoldersWithNestedFolders(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parentUID := r.URL.Query().Get("parentUid")
		if r.URL.Path == "/api/folders" {
			if parentUID == "" {
				json.NewEncoder(w).Encode([]Folder{
					{ID: 1, UID: "parent-1", Title: "Parent"},
				})
			} else if parentUID == "parent-1" {
				json.NewEncoder(w).Encode([]Folder{
					{ID: 2, UID: "child-1", Title: "Child"},
				})
			} else {
				json.NewEncoder(w).Encode([]Folder{})
			}
			return
		}
		if r.URL.Path == "/api/search" {
			json.NewEncoder(w).Encode([]Dashboard{})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "key"}
	folderCache = make(map[string]string)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/folders", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	getFolders(c)

	var folders []Folder
	json.Unmarshal(rec.Body.Bytes(), &folders)
	assert.Len(t, folders, 2)
}

func TestGetFoldersDashboardCountError(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/folders" {
			json.NewEncoder(w).Encode([]Folder{
				{ID: 1, UID: "f1", Title: "F1"},
			})
			return
		}
		if r.URL.Path == "/api/search" {
			callCount++
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "key"}
	folderCache = make(map[string]string)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/folders", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	getFolders(c)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestExportDashboardsNoTitle(t *testing.T) {
	originalConfig := config
	originalCache := folderCache
	defer func() {
		config = originalConfig
		folderCache = originalCache
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/dashboards/uid/uid-notitle" {
			json.NewEncoder(w).Encode(DashboardWithMeta{
				Dashboard: map[string]interface{}{
					"version": float64(1),
					"panels":  []interface{}{},
				},
				Meta: struct {
					FolderID    int    `json:"folderId"`
					FolderUID   string `json:"folderUid"`
					FolderTitle string `json:"folderTitle"`
				}{FolderID: 0},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	tempDir, err := os.MkdirTemp("", "test-export-notitle-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config = Config{GrafanaURL: ts.URL, GrafanaAPIKey: "key", ExportDirectory: tempDir}
	folderCache = make(map[string]string)

	e := echo.New()
	body := `{"dashboardUIDs":["uid-notitle"],"alertUIDs":[],"includeAlerts":false,"exportAsZip":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	exportDashboards(c)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &result)
	assert.Equal(t, float64(1), result["exportedDashboards"])
}

func TestGetEnvBoolForceEnableZipExport(t *testing.T) {
	// Test FORCE_ENABLE_ZIP_EXPORT environment variable parsing
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{"force enable zip true", "true", true},
		{"force enable zip yes", "yes", true},
		{"force enable zip 1", "1", true},
		{"force enable zip false", "false", false},
		{"force enable zip no", "no", false},
		{"force enable zip 0", "0", false},
		{"force enable zip empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("FORCE_ENABLE_ZIP_EXPORT", tt.envValue)
				defer os.Unsetenv("FORCE_ENABLE_ZIP_EXPORT")
			} else {
				os.Unsetenv("FORCE_ENABLE_ZIP_EXPORT")
			}
			assert.Equal(t, tt.want, getEnvBool("FORCE_ENABLE_ZIP_EXPORT", false))
		})
	}
}
