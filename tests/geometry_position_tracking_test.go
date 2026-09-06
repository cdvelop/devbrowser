package devbrowser_test

import (
	"testing"

	"webtyp.com/devbrowser"
)

// TestPositionSavedOnUserMovement verifies the normal case proven by the headful
// diagnostic: CDP GetWindowForTarget reports real coordinates, so when the user
// moves the window the new position is saved to the store.
func TestPositionSavedOnUserMovement(t *testing.T) {
	store := &defaultStore{m: map[string]string{
		"browser_position": "0,0",
		"browser_size":     "900,700",
	}}
	exitChan := make(chan bool)
	b := devbrowser.New(defaultUI{}, store, exitChan)

	// First tick right after open: window is still at 0,0, same as configured —
	// nothing to save.
	b.SaveGeometry(0, 0, 900, 700)
	if pos, _ := store.Get("browser_position"); pos != "0,0" {
		t.Errorf("first tick: position should stay %q, got %q", "0,0", pos)
	}

	// User moves the window to 300,400. CDP reports the real coordinates.
	b.SaveGeometry(300, 400, 900, 700)
	if pos, _ := store.Get("browser_position"); pos != "300,400" {
		t.Errorf("after move: expected %q, got %q", "300,400", pos)
	}
}

// TestSaveGeometryWriteCount documents that one SaveGeometry call, when both
// position and size change, issues 3 Set() calls (position block + size block +
// position sync inside the size block). With kvdb's automatic debounce these
// coalesce into a single disk write per tick; this test only asserts the values
// land correctly.
func TestSaveGeometryWriteCount(t *testing.T) {
	store := &defaultStore{m: map[string]string{
		"browser_position": "0,0",
		"browser_size":     "900,700",
	}}
	exitChan := make(chan bool)
	b := devbrowser.New(defaultUI{}, store, exitChan)

	b.SaveGeometry(300, 400, 1024, 768)

	pos, _ := store.Get("browser_position")
	size, _ := store.Get("browser_size")

	if pos != "300,400" {
		t.Errorf("position: got %q, want %q", pos, "300,400")
	}
	if size != "1024,768" {
		t.Errorf("size: got %q, want %q", size, "1024,768")
	}
}
