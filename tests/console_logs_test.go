package devbrowser_test

import (
	"testing"
	"time"

	"webtyp.com/devbrowser/chromedp"
)

func TestGetConsoleLogs(t *testing.T) {
	// Create a new devbrowser.DevBrowser instance using the shared test helper
	logMessages := []string{}
	logger := func(message ...any) {
		for _, msg := range message {
			logMessages = append(logMessages, msg.(string))
		}
	}

	db, _ := DefaultTestBrowser(logger)

	// Initialize browser context
	err := db.CreateBrowserContext()
	if err != nil {
		t.Fatalf("failed to create browser context: %v", err)
	}
	defer db.Cancel()

	db.IsOpenFlag = true

	// Navigate to a blank page
	err = chromedp.Run(db.Ctx,
		chromedp.Navigate("about:blank"),
	)
	if err != nil {
		t.Fatalf("failed to navigate: %v", err)
	}

	// Wait a moment for the page to load
	time.Sleep(100 * time.Millisecond)

	// Initialize the console capture system
	err = db.InitializeConsoleCapture()
	if err != nil {
		t.Fatalf("failed to initialize console capture: %v", err)
	}

	// Execute some console commands
	script := `
		console.log('Test message');
		console.error('Test error');
		console.warn('Test warning');
		console.info('Test info');
	`
	err = chromedp.Run(db.Ctx,
		chromedp.Evaluate(script, nil),
	)
	if err != nil {
		t.Fatalf("failed to execute console commands: %v", err)
	}

	// Wait a moment for logs to be captured
	time.Sleep(100 * time.Millisecond)

	// Get the logs
	logs, err := db.GetConsoleLogs()
	if err != nil {
		t.Fatalf("failed to get console logs after commands: %v", err)
	}

	// Verify we captured the console messages
	if len(logs) < 4 {
		t.Fatalf("expected at least 4 log entries, got %d", len(logs))
	}

	// Check for expected messages (without type prefix)
	expectedMessages := []string{
		"Test message",
		"Test error",
		"Test warning",
		"Test info",
	}

	for i, expected := range expectedMessages {
		found := false
		for _, log := range logs {
			if log == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected message %d '%s' not found in logs: %v", i, expected, logs)
		}
	}
}

func TestClearConsoleLogs(t *testing.T) {
	// Create a new devbrowser.DevBrowser instance using the shared test helper
	logger := func(message ...any) {}
	db, _ := DefaultTestBrowser(logger)

	// Initialize browser context
	err := db.CreateBrowserContext()
	if err != nil {
		t.Fatalf("failed to create browser context: %v", err)
	}
	defer db.Cancel()

	db.IsOpenFlag = true

	// Navigate to a blank page
	err = chromedp.Run(db.Ctx,
		chromedp.Navigate("about:blank"),
	)
	if err != nil {
		t.Fatalf("failed to navigate: %v", err)
	}

	// Wait a moment for the page to load
	time.Sleep(100 * time.Millisecond)

	// Initialize the console capture system
	err = db.InitializeConsoleCapture()
	if err != nil {
		t.Fatalf("failed to initialize console capture: %v", err)
	}

	// Add some console messages
	script := `
		console.log('Message 1');
		console.log('Message 2');
	`
	err = chromedp.Run(db.Ctx,
		chromedp.Evaluate(script, nil),
	)
	if err != nil {
		t.Fatalf("failed to execute console commands: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify we have logs
	logs, err := db.GetConsoleLogs()
	if err != nil {
		t.Fatalf("failed to get console logs: %v", err)
	}

	if len(logs) < 2 {
		t.Fatalf("expected at least 2 log entries before clear, got %d", len(logs))
	}

	// Clear the logs
	err = db.ClearConsoleLogs()
	if err != nil {
		t.Fatalf("failed to clear console logs: %v", err)
	}

	// Verify logs are cleared
	logs, err = db.GetConsoleLogs()
	if err != nil {
		t.Fatalf("failed to get console logs after clear: %v", err)
	}

	if len(logs) != 0 {
		t.Fatalf("expected 0 log entries after clear, got %d: %v", len(logs), logs)
	}
}

func TestGetConsoleLogsWithoutContext(t *testing.T) {
	db, _ := DefaultTestBrowser()

	// Try to get logs without initializing context
	_, err := db.GetConsoleLogs()
	if err == nil {
		t.Fatal("expected error when getting logs without context, got nil")
	}

	expectedError := "browser context not initialized"
	if err.Error() != expectedError {
		t.Errorf("expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestClearConsoleLogsWithoutContext(t *testing.T) {
	db, _ := DefaultTestBrowser()

	// Try to clear logs without initializing context
	err := db.ClearConsoleLogs()
	if err == nil {
		t.Fatal("expected error when clearing logs without context, got nil")
	}

	expectedError := "browser context not initialized"
	if err.Error() != expectedError {
		t.Errorf("expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestConsoleLogsAutoCleanOnReload(t *testing.T) {
	// Create a new devbrowser.DevBrowser instance using the shared test helper
	logger := func(message ...any) {}
	db, _ := DefaultTestBrowser(logger)

	// Initialize browser context
	err := db.CreateBrowserContext()
	if err != nil {
		t.Fatalf("failed to create browser context: %v", err)
	}
	defer db.Cancel()

	db.IsOpenFlag = true

	// Navigate to a blank page
	err = chromedp.Run(db.Ctx,
		chromedp.Navigate("about:blank"),
	)
	if err != nil {
		t.Fatalf("failed to navigate: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Initialize the console capture system
	err = db.InitializeConsoleCapture()
	if err != nil {
		t.Fatalf("failed to initialize console capture: %v", err)
	}

	// Add some console messages
	script := `
		console.log('First message');
		console.log('Second message');
	`
	err = chromedp.Run(db.Ctx,
		chromedp.Evaluate(script, nil),
	)
	if err != nil {
		t.Fatalf("failed to execute console commands: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify we have logs
	logs, err := db.GetConsoleLogs()
	if err != nil {
		t.Fatalf("failed to get console logs: %v", err)
	}

	if len(logs) != 2 {
		t.Fatalf("expected 2 log entries before reload, got %d: %v", len(logs), logs)
	}

	// Reload the page - this should clear the console logs automatically
	err = chromedp.Run(db.Ctx,
		chromedp.Reload(),
	)
	if err != nil {
		t.Fatalf("failed to reload page: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify logs are cleared after reload (mimicking browser behavior)
	logs, err = db.GetConsoleLogs()
	if err != nil {
		t.Fatalf("failed to get console logs after reload: %v", err)
	}

	if len(logs) != 0 {
		t.Fatalf("expected 0 log entries after reload (console should be cleared), got %d: %v", len(logs), logs)
	}

	// Add new messages after reload
	script = `
		console.log('After reload message');
	`
	err = chromedp.Run(db.Ctx,
		chromedp.Evaluate(script, nil),
	)
	if err != nil {
		t.Fatalf("failed to execute console commands after reload: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify only new logs are present
	logs, err = db.GetConsoleLogs()
	if err != nil {
		t.Fatalf("failed to get console logs after new messages: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 log entry after reload with new messages, got %d: %v", len(logs), logs)
	}

	if logs[0] != "After reload message" {
		t.Errorf("expected 'After reload message', got '%s'", logs[0])
	}
}
