package devbrowser_test

import (
	"strings"
	"testing"

	"webtyp.com/devbrowser"
)

// optimizedSiteMetrics simulates a well-optimized TinyGo WASM site:
// small heap, fast load, few DOM nodes, small WASM binary
func optimizedSiteMetrics() map[string]any {
	return map[string]any{
		"heapUsed":       4.2 * 1048576, // 4.2 MB
		"heapTotal":      8.0 * 1048576, // 8 MB
		"heapLimit":      2048.0 * 1048576,
		"domInteractive": 23.0,
		"domLoaded":      45.0,
		"fullLoad":       120.0,
		"fp":             30.0,
		"fcp":            50.0,
		"domNodes":       15.0,
		"domDepth":       4.0,
		"wasmFiles": []any{
			map[string]any{"name": "client.wasm", "size": 180.0, "duration": 40.0},
		},
		"resourceCount":   3.0,
		"totalTransferKB": 210.0,
	}
}

// heavySiteMetrics simulates a bloated Go stdlib WASM site:
// large heap, slow load, many DOM nodes, large WASM binary
func heavySiteMetrics() map[string]any {
	return map[string]any{
		"heapUsed":       85.0 * 1048576,  // 85 MB
		"heapTotal":      128.0 * 1048576, // 128 MB
		"heapLimit":      2048.0 * 1048576,
		"domInteractive": 1200.0,
		"domLoaded":      2500.0,
		"fullLoad":       4800.0,
		"fp":             800.0,
		"fcp":            1500.0,
		"domNodes":       850.0,
		"domDepth":       18.0,
		"wasmFiles": []any{
			map[string]any{"name": "client.wasm", "size": 12800.0, "duration": 3200.0},
		},
		"resourceCount":   25.0,
		"totalTransferKB": 14000.0,
	}
}

func TestFormatPerformanceReport_OptimizedSite(t *testing.T) {
	report := devbrowser.FormatPerformanceReport("http://localhost:8080/", optimizedSiteMetrics())

	checks := []struct {
		label    string
		contains string
	}{
		{"URL", "Performance: http://localhost:8080/"},
		{"heap used", "4.2"},
		{"heap total", "8.0 MB"},
		{"interactive", "Interactive 23ms"},
		{"dom loaded", "DOM Loaded 45ms"},
		{"full load", "Full Load 120ms"},
		{"first paint", "FP 30ms"},
		{"FCP", "FCP 50ms"},
		{"dom nodes", "15 nodes"},
		{"dom depth", "max depth 4"},
		{"wasm file", "client.wasm 180 KB"},
		{"wasm load", "loaded in 40ms"},
		{"resources", "3 total"},
		{"transfer", "210 KB transferred"},
	}

	for _, c := range checks {
		if !strings.Contains(report, c.contains) {
			t.Errorf("[%s] expected report to contain %q\nGot:\n%s", c.label, c.contains, report)
		}
	}
}

func TestFormatPerformanceReport_HeavySite(t *testing.T) {
	report := devbrowser.FormatPerformanceReport("http://localhost:8080/", heavySiteMetrics())

	checks := []struct {
		label    string
		contains string
	}{
		{"high heap", "85.0"},
		{"high heap total", "128.0 MB"},
		{"slow interactive", "Interactive 1200ms"},
		{"slow load", "Full Load 4800ms"},
		{"slow FCP", "FCP 1500ms"},
		{"many nodes", "850 nodes"},
		{"deep DOM", "max depth 18"},
		{"large wasm", "client.wasm 12800 KB"},
		{"slow wasm load", "loaded in 3200ms"},
		{"many resources", "25 total"},
		{"large transfer", "14000 KB transferred"},
	}

	for _, c := range checks {
		if !strings.Contains(report, c.contains) {
			t.Errorf("[%s] expected report to contain %q\nGot:\n%s", c.label, c.contains, report)
		}
	}
}

func TestFormatPerformanceReport_EmptyMetrics(t *testing.T) {
	report := devbrowser.FormatPerformanceReport("about:blank", map[string]any{})

	if !strings.Contains(report, "Performance: about:blank") {
		t.Errorf("Should always have URL header, got:\n%s", report)
	}

	// No memory, timing, paint, DOM, WASM, or resources lines
	for _, absent := range []string{"Memory:", "Timing:", "Paint:", "DOM:", "WASM:", "Resources:"} {
		if strings.Contains(report, absent) {
			t.Errorf("Empty metrics should NOT contain %q, got:\n%s", absent, report)
		}
	}
}

func TestFormatPerformanceReport_PartialMetrics(t *testing.T) {
	// Only DOM and resources, no memory/timing/paint/wasm
	metrics := map[string]any{
		"domNodes":        5.0,
		"domDepth":        2.0,
		"resourceCount":   1.0,
		"totalTransferKB": 50.0,
		"wasmFiles":       []any{},
	}

	report := devbrowser.FormatPerformanceReport("http://localhost:8080/", metrics)

	if !strings.Contains(report, "DOM:       5 nodes | max depth 2") {
		t.Errorf("Should contain DOM line, got:\n%s", report)
	}
	if !strings.Contains(report, "Resources: 1 total | 50 KB transferred") {
		t.Errorf("Should contain Resources line, got:\n%s", report)
	}

	for _, absent := range []string{"Memory:", "Timing:", "Paint:", "WASM:"} {
		if strings.Contains(report, absent) {
			t.Errorf("Partial metrics should NOT contain %q, got:\n%s", absent, report)
		}
	}
}

func TestFormatPerformanceReport_MultipleWasmFiles(t *testing.T) {
	metrics := map[string]any{
		"wasmFiles": []any{
			map[string]any{"name": "client.wasm", "size": 200.0, "duration": 50.0},
			map[string]any{"name": "worker.wasm", "size": 80.0, "duration": 20.0},
		},
	}

	report := devbrowser.FormatPerformanceReport("http://localhost:8080/", metrics)

	if !strings.Contains(report, "client.wasm 200 KB (loaded in 50ms)") {
		t.Errorf("Should list client.wasm, got:\n%s", report)
	}
	if !strings.Contains(report, "worker.wasm 80 KB (loaded in 20ms)") {
		t.Errorf("Should list worker.wasm, got:\n%s", report)
	}
}

func TestFormatPerformanceReport_CompactOutput(t *testing.T) {
	report := devbrowser.FormatPerformanceReport("http://localhost:8080/", optimizedSiteMetrics())

	lines := strings.Split(strings.TrimSpace(report), "\n")
	if len(lines) > 10 {
		t.Errorf("Report should be compact (<=10 lines), got %d lines:\n%s", len(lines), report)
	}
}
