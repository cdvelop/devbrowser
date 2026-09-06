package devbrowser

import (
	"fmt"

	"webtyp.com/screenshot"
)

// QueryMonitorSize is a function variable to allow mocking in tests.
// It returns width, height of the primary display.
var QueryMonitorSize = func() (int, int) {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return 0, 0
	}
	// Primary display (index 0)
	bounds := screenshot.GetDisplayBounds(0)
	return bounds.Dx(), bounds.Dy()
}

// detectMonitorSize attempts to retrieve the primary monitor's dimensions.
// It updates the DevBrowser's monitorWidth and monitorHeight fields.
func (b *DevBrowser) DetectMonitorSize() {
	w, h := QueryMonitorSize()

	if w == 0 || h == 0 {
		b.Log("No active displays found or extraction failed")
		return
	}

	b.Mu.Lock()
	b.MonitorWidth = w
	b.MonitorHeight = h
	b.Mu.Unlock()

	b.Log(fmt.Sprintf("Detected monitor size: %dx%d", w, h))
}
