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

func TestDeviceResolution(t *testing.T) {
	db, _ := DefaultTestBrowser()
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	defer db.CloseBrowser()

	tools := db.GetManagementTools()
	tool := tools[0] // browser_emulate_device

	// 1. Emulate with an unknown device name -> must return an error containing a list of available devices
	argsInvalid := devbrowser.EmulateDeviceArgs{Device: "super_fast_invalid_phone"}
	reqInvalid := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_emulate_device",
			Arguments: encodeArgs(&argsInvalid),
		},
		Action: 'u',
	}
	_, err := tool.Execute(nil, reqInvalid)
	if err == nil {
		t.Fatal("Expected error when requesting unknown device name, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported device") || !strings.Contains(err.Error(), "iPhone") {
		t.Errorf("Expected error to list available devices. Got: %v", err)
	}

	// 2. Emulate with a valid device name (case-insensitive, spaces ignored) -> success
	argsValid := devbrowser.EmulateDeviceArgs{Device: "iPhone15ProMax"}
	reqValid := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_emulate_device",
			Arguments: encodeArgs(&argsValid),
		},
		Action: 'u',
	}
	resValid, err := tool.Execute(nil, reqValid)
	if err != nil {
		t.Fatalf("Failed to execute emulation for iPhone15ProMax: %v", err)
	}

	var contents mcp.TextContentList
	if err := json.Decode(string(resValid.Content), &contents); err != nil {
		t.Fatal(err)
	}
	resultText := contents[0].Text
	if !strings.Contains(resultText, "Device emulation set to iPhone15ProMax") {
		t.Errorf("Unexpected response for iPhone15ProMax: %s", resultText)
	}
}

func TestMobileAudit_Tool(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			<!DOCTYPE html>
			<html>
			<head>
				<!-- missing viewport meta completely -->
				<style>
					.vh-container {
						height: 100vh;
					}
					.safe-edge {
						position: fixed;
						bottom: 0;
						left: 0;
						background: red;
					}
				</style>
			</head>
			<body>
				<div class="vh-container">VH Element</div>
				<div class="safe-edge">No safe area margin fixed element</div>
				<input id="small-input" style="font-size: 12px;" type="text" />
				<button id="small-button" style="width: 20px; height: 20px;">X</button>
			</body>
			</html>
		`)
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

	tools := db.GetMCPTools()
	var auditTool mcp.Tool
	for _, tool := range tools {
		if tool.Name == "browser_audit_mobile" {
			auditTool = tool
			break
		}
	}

	if auditTool.Name == "" {
		t.Fatal("browser_audit_mobile tool is not registered")
	}

	args := devbrowser.AuditMobileArgs{}
	req := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_audit_mobile",
			Arguments: encodeArgs(&args),
		},
		Action: 'r',
	}

	res, err := auditTool.Execute(nil, req)
	if err != nil {
		t.Fatalf("Failed to execute mobile audit: %v", err)
	}

	var contents mcp.TextContentList
	if err := json.Decode(string(res.Content), &contents); err != nil {
		t.Fatal(err)
	}
	report := contents[0].Text

	// Verify all fail categories are correctly detected in the report
	if !strings.Contains(report, "Viewport Meta Check") || !strings.Contains(report, "missing") {
		t.Errorf("Expected missing viewport meta detection, got report: %s", report)
	}
	if !strings.Contains(report, "VH Units Check") || !strings.Contains(report, "vh-container") {
		t.Errorf("Expected vh unit detection for vh-container, got: %s", report)
	}
	if !strings.Contains(report, "Safe Area Check") || !strings.Contains(report, "safe-edge") {
		t.Errorf("Expected safe area violation for safe-edge, got: %s", report)
	}
	if !strings.Contains(report, "Input Zoom Check") || !strings.Contains(report, "#small-input") {
		t.Errorf("Expected input-zoom violation for small-input, got: %s", report)
	}
	if !strings.Contains(report, "Tap Target Check") || !strings.Contains(report, "#small-button") {
		t.Errorf("Expected tap-target violation for small-button, got: %s", report)
	}
}
