package devbrowser_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywasm/devbrowser"
	"github.com/tinywasm/json"
	"github.com/tinywasm/mcp"
)

func TestSaveScreenshot_MetadataAndIsolation(t *testing.T) {
	db, _ := DefaultTestBrowser()
	defer db.CloseBrowser()

	tools := db.GetMCPTools()
	var saveTool *mcp.Tool
	for i := range tools {
		if tools[i].Name == "browser_save_screenshot" {
			saveTool = &tools[i]
			break
		}
	}

	if saveTool == nil {
		t.Fatal("browser_save_screenshot tool is not registered")
	}

	if saveTool.Resource != "browser_file" {
		t.Errorf("Expected Resource 'browser_file', got '%s'", saveTool.Resource)
	}

	if saveTool.Action != 'c' {
		t.Errorf("Expected Action 'c' (model.Create), got '%c'", saveTool.Action)
	}
}

func TestSaveScreenshot_MutualExclusivity(t *testing.T) {
	db, _ := DefaultTestBrowser()
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	db.IsOpenFlag = true
	defer db.CloseBrowser()

	tools := db.GetScreenshotTools()
	var saveTool *mcp.Tool
	for i := range tools {
		if tools[i].Name == "browser_save_screenshot" {
			saveTool = &tools[i]
			break
		}
	}
	if saveTool == nil {
		t.Fatal("browser_save_screenshot tool is not found")
	}

	args := devbrowser.SaveScreenshotArgs{
		Dir:      "./tmp",
		Name:     "widget",
		Fullpage: true,
		Selector: ".some-widget",
	}

	req := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_save_screenshot",
			Arguments: encodeArgs(&args),
		},
		Action: 'c',
	}

	_, err := saveTool.Execute(nil, req)
	if err == nil {
		t.Error("Expected error when both selector and fullpage are true, got nil")
	} else if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("Expected mutually exclusive error, got: %v", err)
	}
}

func TestSaveScreenshot_SecurityConstraints(t *testing.T) {
	db, _ := DefaultTestBrowser()
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	db.IsOpenFlag = true
	defer db.CloseBrowser()

	tools := db.GetScreenshotTools()
	var saveTool *mcp.Tool
	for i := range tools {
		if tools[i].Name == "browser_save_screenshot" {
			saveTool = &tools[i]
			break
		}
	}

	// 1. Path traversal in Dir
	argsTraversal := devbrowser.SaveScreenshotArgs{
		Dir:  "../../etc",
		Name: "passwd",
	}
	reqTraversal := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_save_screenshot",
			Arguments: encodeArgs(&argsTraversal),
		},
		Action: 'c',
	}
	_, err := saveTool.Execute(nil, reqTraversal)
	if err == nil {
		t.Error("Expected error for path traversal (..), got nil")
	} else if !strings.Contains(err.Error(), "traversal") {
		t.Errorf("Expected traversal error, got: %v", err)
	}

	// 2. Separators inside Name
	argsSeparators := devbrowser.SaveScreenshotArgs{
		Dir:  "./tmp",
		Name: "subdir/widget",
	}
	reqSeparators := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_save_screenshot",
			Arguments: encodeArgs(&argsSeparators),
		},
		Action: 'c',
	}
	_, err = saveTool.Execute(nil, reqSeparators)
	if err == nil {
		t.Error("Expected error for separator in name, got nil")
	} else if !strings.Contains(err.Error(), "separators") {
		t.Errorf("Expected separators error, got: %v", err)
	}
}

func TestSaveScreenshot_OverwritePrevention(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><h1>Test</h1></body></html>`)
	}))
	defer ts.Close()

	db, _ := DefaultTestBrowser()
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	db.IsOpenFlag = true
	defer db.CloseBrowser()

	if err := db.NavigateToURL(ts.URL); err != nil {
		t.Fatal(err)
	}

	tools := db.GetScreenshotTools()
	var saveTool *mcp.Tool
	for i := range tools {
		if tools[i].Name == "browser_save_screenshot" {
			saveTool = &tools[i]
			break
		}
	}

	tmpDir, err := os.MkdirTemp("", "save_screenshot_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create pre-existing file with specific content
	existingFilePath := filepath.Join(tmpDir, "pre_existing.png")
	originalBytes := []byte("original content - must stay identical")
	if err := os.WriteFile(existingFilePath, originalBytes, 0644); err != nil {
		t.Fatal(err)
	}

	args := devbrowser.SaveScreenshotArgs{
		Dir:       tmpDir,
		Name:      "pre_existing",
		Overwrite: false,
	}

	req := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_save_screenshot",
			Arguments: encodeArgs(&args),
		},
		Action: 'c',
	}

	_, err = saveTool.Execute(nil, req)
	if err == nil {
		t.Error("Expected error when file already exists and overwrite is false, got nil")
	}

	// Verify the file was untouched
	fileBytes, err := os.ReadFile(existingFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(fileBytes) != string(originalBytes) {
		t.Error("Pre-existing file was modified when overwrite=false")
	}
}

func TestSaveScreenshot_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><div id="target" style="width: 100px; height: 100px; background: red;">Widget</div></body></html>`)
	}))
	defer ts.Close()

	db, _ := DefaultTestBrowser()
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	db.IsOpenFlag = true
	defer db.CloseBrowser()

	if err := db.NavigateToURL(ts.URL); err != nil {
		t.Fatal(err)
	}

	tools := db.GetScreenshotTools()
	var saveTool *mcp.Tool
	for i := range tools {
		if tools[i].Name == "browser_save_screenshot" {
			saveTool = &tools[i]
			break
		}
	}

	tmpDir, err := os.MkdirTemp("", "save_screenshot_success_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	args := devbrowser.SaveScreenshotArgs{
		Dir:       tmpDir,
		Name:      "successful_capture",
		Overwrite: true,
	}

	req := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_save_screenshot",
			Arguments: encodeArgs(&args),
		},
		Action: 'c',
	}

	res, err := saveTool.Execute(nil, req)
	if err != nil {
		t.Fatalf("Failed to execute save screenshot: %v", err)
	}

	var contents mcp.TextContentList
	if err := json.Decode(string(res.Content), &contents); err != nil {
		t.Fatal(err)
	}
	resultText := contents[0].Text

	expectedFile := filepath.Join(tmpDir, "successful_capture.png")
	if !strings.Contains(resultText, expectedFile) {
		t.Errorf("Expected status message to contain absolute path '%s'. Got: %s", expectedFile, resultText)
	}

	// Check file exists and has size > 0
	info, err := os.Stat(expectedFile)
	if err != nil {
		t.Fatalf("Expected file to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Saved screenshot is empty (0 bytes)")
	}

	// Validate PNG header
	fileBytes, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(fileBytes) < 8 || string(fileBytes[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Error("Saved file is not a valid PNG")
	}
}
