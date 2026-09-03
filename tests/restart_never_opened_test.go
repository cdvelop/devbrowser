package devbrowser_test

import "testing"

// RestartBrowser reopens whatever was open. On a browser that was never opened
// there is no port to go back to, and opening on an empty one navigates to
// "http://localhost:/" — which fails with ERR_CONNECTION_REFUSED and leaves the
// window closed.
//
// This used to be masked: CloseBrowser returned an "already closed" error on a
// closed browser, so RestartBrowser bailed out before reaching OpenBrowser.
// Making CloseBrowser idempotent (v0.5.9) removed that accidental guard, and
// the empty-port open became reachable — every cold start of the tinywasm
// daemon hit it, because the daemon asks for the browser before the dev server
// has reported which port it listens on.
func TestRestartBrowser_NeverOpened_DoesNotOpenOnAnEmptyPort(t *testing.T) {
	db, _ := DefaultTestBrowser()
	db.SetTestMode(true)

	if db.IsOpenFlag {
		t.Fatal("expected a closed browser to start with")
	}

	if err := db.RestartBrowser(); err != nil {
		t.Fatalf("restarting a never-opened browser must be a no-op, got %v", err)
	}

	if db.OpenedOnce {
		t.Error("RestartBrowser opened a window for a browser that was never opened")
	}
	if db.LastPort != "" {
		t.Errorf("LastPort = %q, want empty: nothing has told the browser which port to use yet", db.LastPort)
	}
}

// Once the dev server has reported its port, a restart is a real restart.
func TestRestartBrowser_AfterOpen_ReusesTheKnownPort(t *testing.T) {
	db, _ := DefaultTestBrowser()
	db.SetTestMode(true)

	db.OpenBrowser("8080", false)
	if db.LastPort != "8080" {
		t.Fatalf("LastPort = %q, want 8080", db.LastPort)
	}

	if err := db.RestartBrowser(); err != nil {
		t.Fatalf("RestartBrowser: %v", err)
	}

	if db.LastPort != "8080" {
		t.Errorf("LastPort = %q after restart, want 8080", db.LastPort)
	}
}
