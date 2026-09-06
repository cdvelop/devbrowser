package devbrowser_test

import (
	"strings"
	"testing"

	"webtyp.com/mcp"
)

func TestRelativeNavigation(t *testing.T) {
	db, _ := DefaultTestBrowser()
	db.IsOpenFlag = true
	db.LastPort = "8080"
	db.LastHttps = false

	navTools := db.GetNavigationTools()
	var navigateTool *mcp.Tool
	for i := range navTools {
		if navTools[i].Name == "browser_navigate" {
			navigateTool = &navTools[i]
			break
		}
	}

	if navigateTool == nil {
		t.Fatal("browser_navigate tool not found")
	}

	tests := []struct {
		name        string
		url         string
		lastPort    string
		lastHttps   bool
		expectedErr string
		resolvedURL string
	}{
		{
			name:        "absolute url",
			url:         "http://example.com",
			lastPort:    "8080",
			resolvedURL: "http://example.com",
		},
		{
			name:        "relative path",
			url:         "/login",
			lastPort:    "8080",
			lastHttps:   false,
			resolvedURL: "http://localhost:8080/login",
		},
		{
			name:        "relative path https",
			url:         "/secure",
			lastPort:    "443",
			lastHttps:   true,
			resolvedURL: "https://localhost:443/secure",
		},
		{
			name:        "missing port",
			url:         "/login",
			lastPort:    "",
			expectedErr: "browser has no active app port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db.LastPort = tt.lastPort
			db.LastHttps = tt.lastHttps

			req := mcp.Request{
				Params: mcp.CallToolParams{
					Name:      "browser_navigate",
					Arguments: `{"url":"` + tt.url + `"}`,
				},
			}

			_, err := navigateTool.Execute(nil, req)

			if tt.expectedErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("expected error containing %q, got %v", tt.expectedErr, err)
				}
				return
			}

			if err != nil {
				// We expect error because Ctx is nil, but we check if URL was resolved in the error message
				expectedContains := "Error navigating to " + tt.resolvedURL
				if !strings.Contains(err.Error(), expectedContains) {
					t.Errorf("expected resolution to contain %q, got error %v", expectedContains, err)
				}
			}
		})
	}
}
