package devbrowser_test

import (
	"context"
	"testing"
)

func TestRequiredWindowSize_GrowsForDevToolsReservation(t *testing.T) {
	db, _ := DefaultTestBrowser()
	db.Width = 800
	db.Height = 600
	db.MonitorWidth = 1920
	db.MonitorHeight = 1080
	db.DevToolsReserved = true

	reqW, reqH := db.RequiredWindowSize(1440, 900)

	expectedW := 1440 + 420 // DevToolsReservedWidth
	if expectedW > 1920 {
		expectedW = 1920
	}

	if reqW != expectedW {
		t.Errorf("Expected width %d, got %d", expectedW, reqW)
	}
	if reqH != 900 {
		t.Errorf("Expected height 900, got %d", reqH)
	}
}

func TestRequiredWindowSize_NeverShrinks(t *testing.T) {
	db, _ := DefaultTestBrowser()
	db.Width = 1600
	db.Height = 1000
	db.MonitorWidth = 1920
	db.MonitorHeight = 1080
	db.DevToolsReserved = false

	reqW, reqH := db.RequiredWindowSize(375, 812)

	if reqW != 1600 || reqH != 1000 {
		t.Errorf("Expected window size to remain 1600x1000, got %dx%d", reqW, reqH)
	}
}

func TestRequiredWindowSize_ClampsToMonitor(t *testing.T) {
	db, _ := DefaultTestBrowser()
	db.Width = 800
	db.Height = 600
	db.MonitorWidth = 1200
	db.MonitorHeight = 800
	db.DevToolsReserved = true

	reqW, reqH := db.RequiredWindowSize(1440, 900)

	if reqW > 1200 {
		t.Errorf("Expected width to be clamped to monitor width 1200, got %d", reqW)
	}
	if reqH > 800 {
		t.Errorf("Expected height to be clamped to monitor height 800, got %d", reqH)
	}
}

func TestGrowWindowToFit_NoOpWhenContextNil(t *testing.T) {
	db, _ := DefaultTestBrowser()
	db.Ctx = nil

	resized, err := db.GrowWindowToFit(1440, 900)
	if err != nil {
		t.Fatalf("GrowWindowToFit returned unexpected error: %v", err)
	}
	if resized {
		t.Error("Expected resized to be false when context is nil")
	}
}

func TestGrowWindowToFit_NoOpWhenAlreadyBigEnough(t *testing.T) {
	db, _ := DefaultTestBrowser()
	db.Width = 2000
	db.Height = 1200
	db.MonitorWidth = 2560
	db.MonitorHeight = 1440
	db.Ctx = context.Background()

	resized, err := db.GrowWindowToFit(375, 812)
	if err != nil {
		t.Fatalf("GrowWindowToFit returned unexpected error: %v", err)
	}
	if resized {
		t.Error("Expected resized to be false when window is already big enough")
	}
}
