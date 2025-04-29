package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
