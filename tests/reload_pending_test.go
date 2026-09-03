package devbrowser_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestRecargaPedidaMientrasAbreSeAplicaAlQuedarListo(t *testing.T) {
	var logs []string
	var mu sync.Mutex
	logger := func(msg ...any) {
		mu.Lock()
		defer mu.Unlock()
		var parts []string
		for _, m := range msg {
			parts = append(parts, fmt.Sprint(m))
		}
		logs = append(logs, strings.Join(parts, " "))
	}

	db, _ := DefaultTestBrowser(logger)
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	defer db.CloseBrowser()

	// Simulate browser is opening (IsOpenFlag is true, but ready is false)
	db.IsOpenFlag = true

	// 1. Reload called while opening -> enqueued
	if err := db.Reload(); err != nil {
		t.Fatalf("Reload returned unexpected error: %v", err)
	}

	if !db.IsPendingReload() {
		t.Error("Expected pendingReload to be true after Reload() while opening")
	}

	mu.Lock()
	foundPendingLog := false
	for _, l := range logs {
		if strings.Contains(l, "Reload pending: the browser is still opening") {
			foundPendingLog = true
			break
		}
	}
	mu.Unlock()

	if !foundPendingLog {
		t.Errorf("Expected 'Reload pending' log message, got logs: %v", logs)
	}

	// 2. Transition to ready and process pending reload
	db.SetReadyForTest(true)
	db.ProcessPendingReload()

	if db.IsPendingReload() {
		t.Error("Expected pendingReload to be false after processing pending reload")
	}

	mu.Lock()
	foundApplyingLog := false
	for _, l := range logs {
		if strings.Contains(l, "Applying the pending reload") {
			foundApplyingLog = true
			break
		}
	}
	mu.Unlock()

	if !foundApplyingLog {
		t.Errorf("Expected 'Applying the pending reload' log message, got logs: %v", logs)
	}
}

func TestVariasRecargasPendientesSonUnaSola(t *testing.T) {
	var logs []string
	var mu sync.Mutex
	logger := func(msg ...any) {
		mu.Lock()
		defer mu.Unlock()
		var parts []string
		for _, m := range msg {
			parts = append(parts, fmt.Sprint(m))
		}
		logs = append(logs, strings.Join(parts, " "))
	}

	db, _ := DefaultTestBrowser(logger)
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	defer db.CloseBrowser()

	db.IsOpenFlag = true

	// Call Reload 3 times before ready
	if err := db.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := db.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := db.Reload(); err != nil {
		t.Fatal(err)
	}

	if !db.IsPendingReload() {
		t.Error("Expected pendingReload to be true")
	}

	// Process pending reload
	db.SetReadyForTest(true)
	db.ProcessPendingReload()

	mu.Lock()
	applyingCount := 0
	for _, l := range logs {
		if strings.Contains(l, "Applying the pending reload") {
			applyingCount++
		}
	}
	mu.Unlock()

	if applyingCount != 1 {
		t.Errorf("Expected exactly 1 'Applying the pending reload' log, got %d", applyingCount)
	}
}

func TestSinNavegadorAbiertoNoSeEncolaNada(t *testing.T) {
	db, _ := DefaultTestBrowser()
	db.IsOpenFlag = false

	if err := db.Reload(); err != nil {
		t.Fatal(err)
	}

	if db.IsPendingReload() {
		t.Error("Expected pendingReload to remain false when browser is closed")
	}
}

func TestCerrarLimpiaLaRecargaPendiente(t *testing.T) {
	db, _ := DefaultTestBrowser()
	db.IsOpenFlag = true

	if err := db.Reload(); err != nil {
		t.Fatal(err)
	}

	if !db.IsPendingReload() {
		t.Fatal("Expected pendingReload to be true before close")
	}

	if err := db.CloseBrowser(); err != nil {
		t.Fatal(err)
	}

	if db.IsPendingReload() {
		t.Error("Expected CloseBrowser to clear pendingReload to false")
	}
}
