package devbrowser_test

import (
	"testing"

	"webtyp.com/devbrowser"
)

// TestPositionSavedMirrorsSizePath verifies that position is tracked exactly
// like size: whenever CDP reports a different value, it is saved to the store.
// This mirrors the size block — no extra guards, same conditions.
func TestPositionSavedMirrorsSizePath(t *testing.T) {
	store := &defaultStore{m: map[string]string{
		"browser_position": "300,400",
		"browser_size":     "800,600",
	}}
	exitChan := make(chan bool)
	b := devbrowser.New(defaultUI{}, store, exitChan)

	// CDP reports a new position AND new size (user moved + resized the window).
	b.SaveGeometry(150, 75, 1024, 768)

	savedPos, _ := store.Get("browser_position")
	savedSize, _ := store.Get("browser_size")

	if savedSize != "1024,768" {
		t.Errorf("size not saved: got %q, want %q", savedSize, "1024,768")
	}
	if savedPos != "150,75" {
		t.Errorf("position not saved: got %q, want %q", savedPos, "150,75")
	}
}

// TestPositionSavedWhenSizeChanges verifies that when only size changes,
// the current position is also written to the store in the same operation.
func TestPositionSavedWhenSizeChanges(t *testing.T) {
	store := &defaultStore{m: map[string]string{}}
	exitChan := make(chan bool)
	b := devbrowser.New(defaultUI{}, store, exitChan)
	b.Position = "300,400"

	// CDP reports the same position but a new size.
	b.SaveGeometry(300, 400, 1200, 700)

	savedSize, _ := store.Get("browser_size")
	if savedSize != "1200,700" {
		t.Errorf("size not saved: got %q, want %q", savedSize, "1200,700")
	}

	savedPos, err := store.Get("browser_position")
	if err != nil || savedPos != "300,400" {
		t.Errorf("position not saved when size changed (err: %v): got %q, want %q",
			err, savedPos, "300,400")
	}
}
