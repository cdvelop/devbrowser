package devbrowser_test

import "testing"

func TestNewDefaults(t *testing.T) {
	db, _ := DefaultTestBrowser()
	if db == nil {
		t.Fatal("New returned nil")
	}
	if db.Width != 1024 {
		t.Fatalf("expected default width 1024, got %d", db.Width)
	}
	if db.Height != 768 {
		t.Fatalf("expected default height 768, got %d", db.Height)
	}
	if db.Position != "0,0" {
		t.Fatalf("expected default position '0,0', got '%s'", db.Position)
	}
}

func TestCloseBrowserIsIdempotent(t *testing.T) {
	db, _ := DefaultTestBrowser()
	// Ensure isOpen is false by default
	if db.IsOpenFlag {
		t.Fatal("expected isOpen false by default")
	}
	if err := db.CloseBrowser(); err != nil {
		t.Fatalf("CloseBrowser on a closed browser must be a no-op, got %v", err)
	}
	if err := db.CloseBrowser(); err != nil {
		t.Fatalf("second CloseBrowser must also be a no-op, got %v", err)
	}
}
