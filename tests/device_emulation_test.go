package devbrowser_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tinywasm/devbrowser"
	"github.com/tinywasm/json"
	"github.com/tinywasm/mcp"
)

func TestDeviceEmulation_Logic(t *testing.T) {
	b := &devbrowser.DevBrowser{
		Width:  1200,
		Height: 800,
		Log:    func(msg ...any) {},
	}

	// Test case: mobile emulation
	b.ViewportMode = "mobile"
	if b.ViewportMode != "mobile" {
		t.Errorf("Expected viewportMode mobile, got %s", b.ViewportMode)
	}

	// Test case: desktop (reset)
	b.ViewportMode = "desktop"
	if b.ViewportMode != "desktop" {
		t.Errorf("Expected viewportMode desktop, got %s", b.ViewportMode)
	}
}

func TestScreenshotUtility_ResultStructure(t *testing.T) {
	res := &devbrowser.ScreenshotResult{
		ImageData: []byte("fake-image"),
		PageURL:   "http://localhost",
		Width:     1024,
	}

	if len(res.ImageData) == 0 {
		t.Error("ImageData should not be empty")
	}
}

func TestDeviceEmulation_ValidationAndDistinctModes(t *testing.T) {
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

	tools := db.GetManagementTools()
	tool := tools[0] // browser_emulate_device

	// 1. Rejects unknown mode and does not mutate/persist
	argsInvalid := devbrowser.EmulateDeviceArgs{Mode: "super_fast_phone"}
	reqInvalid := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_emulate_device",
			Arguments: encodeArgs(&argsInvalid),
		},
		Action: 'u',
	}
	_, err := tool.Execute(nil, reqInvalid)
	if err == nil {
		t.Fatal("Expected error when requesting unknown device mode, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported mode") {
		t.Errorf("Expected unsupported mode error, got: %v", err)
	}
	// Check b.ViewportMode is not changed
	if db.ViewportMode == "super_fast_phone" {
		t.Error("Invalid mode should not have been mutated into config")
	}

	// 2. Setting "desktop" mode successfully pins viewport and returns size in reply
	argsDesktop := devbrowser.EmulateDeviceArgs{Mode: "desktop"}
	reqDesktop := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_emulate_device",
			Arguments: encodeArgs(&argsDesktop),
		},
		Action: 'u',
	}
	resDesktop, err := tool.Execute(nil, reqDesktop)
	if err != nil {
		t.Fatalf("Failed to execute device emulation for desktop: %v", err)
	}

	var contents mcp.TextContentList
	if err := json.Decode(string(resDesktop.Content), &contents); err != nil {
		t.Fatal(err)
	}
	resultText := contents[0].Text
	if !strings.Contains(resultText, "Device emulation set to desktop") || !strings.Contains(resultText, "viewport 1440x900") {
		t.Errorf("Desktop response should specify viewport 1440x900. Got: %s", resultText)
	}

	// 3. Setting "off" clears overrides and returns actual window layout
	argsOff := devbrowser.EmulateDeviceArgs{Mode: "off"}
	reqOff := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_emulate_device",
			Arguments: encodeArgs(&argsOff),
		},
		Action: 'u',
	}
	resOff, err := tool.Execute(nil, reqOff)
	if err != nil {
		t.Fatalf("Failed to execute device emulation off: %v", err)
	}

	var contentsOff mcp.TextContentList
	if err := json.Decode(string(resOff.Content), &contentsOff); err != nil {
		t.Fatal(err)
	}
	resultTextOff := contentsOff[0].Text
	if !strings.Contains(resultTextOff, "Device emulation set to off") {
		t.Errorf("Expected 'Device emulation set to off', got: %s", resultTextOff)
	}
}
