package devbrowser

import (
	"fmt"
)

// calculateConstrainedSize determines the best window size based on available monitor space
// and previous configuration.
//
// logic:
// 1. If monitor size is unknown (0x0), return requested size
// 2. If valid monitor size:
//   - If !sizeConfigured (first run or auto):
//     Accept requested size BUT ensure it fits within monitor (clamping)
//   - If sizeConfigured (user manually set size):
//     Check if it fits. If not, scale down or clamp. To be safe, we clamp to available area.
//
// In all cases, we ensure the window is not larger than the available screen area.
func (b *DevBrowser) CalculateConstrainedSize(reqW, reqH, monW, monH int) (int, int) {
	if monW <= 0 || monH <= 0 {
		return reqW, reqH
	}

	finalW, finalH := reqW, reqH

	// Constrain width
	if finalW > monW {
		finalW = monW
	}

	// Constrain height
	if finalH > monH {
		finalH = monH
	}

	// If we modified dimensions, we might want to log it (caller handles logging)
	return finalW, finalH
}

// startWithDetectedSize updates the browser size if it hasn't been configured yet
// and we have detected a monitor size.
func (b *DevBrowser) StartWithDetectedSize() {
	b.Mu.Lock()
	defer b.Mu.Unlock()

	// If user already has a saved config, respect it (but apply constraints later)
	if b.SizeConfigured {
		return
	}

	// If no monitor detected, keep defaults
	if b.MonitorWidth == 0 || b.MonitorHeight == 0 {
		return
	}

	// Logic for default size when nothing is configured:
	// We want a reasonable default that fits.
	// Default is 1024x768 in New().
	// If monitor is smaller (e.g. mobile 375x...), default might be too big.

	// So strictly speaking, we just run the constraint logic on the default.
	// If the default 1024x768 fits, we keep it. Use cases:
	// - Desktop (1920x1080): 1024x768 fits -> use default.
	// - Tablet/Phone (e.g. 800x600 screen?): 1024x768 -> becomes 800x600.

	// However, the user request says:
	// "se necestia que se sepa de forma automatizada las medidas del monitor para tener
	// como base el alto y ancho cuando este no ha sido configurado [...] asi arrancar con esa medida"
	// This implies we might want to start MAXIMIZED or close to full screen if not configured?
	// Or maybe just ensure it fits.

	// "tomar las medidas del monitor como base":
	// Let's interpret this as: define a sensible default based on monitor size.
	// If it's a large screen, maybe we stick to a sensible "desktop" default (like 1280x800 or 1440x900).
	// If it's small, we take the max available.

	// Let's stick to the safer "ensure it fits" approach for now, which satisfies "adjust to them".
	// But let's verify if "arrancar con esa medida" means full screen.
	// Usually devs don't want full screen 4k browser on startup.
	// Let's assume they want the Configured Default (1024x768) BUT guaranteed to fit.

	newW, newH := b.CalculateConstrainedSize(b.Width, b.Height, b.MonitorWidth, b.MonitorHeight)

	// If monitor is really big, maybe we can be bolder than 1024x768?
	// The prompt says: "las medidas desktop,mobile, tablet no puedes superar ninguna de estas deben ser proporcional o ajaustarce a ellas"
	// This confirms the constraint logic is the priority.

	// Special case: If the "default" hasn't been touched, maybe we upgrade it to a better desktop size
	// if the screen allows? 1024x768 is a bit old school.
	// But let's safely just constrain for now.

	b.Width = newW
	b.Height = newH
	b.Log(fmt.Sprintf("Browser size auto-adjusted to monitor: %dx%d", newW, newH))
}

// DevToolsReservedWidth is a conservative, documented ESTIMATE of the
// horizontal space Chrome's docked DevTools panel occupies when auto-opened
// via auto-open-devtools-for-tabs (see context.go). CDP exposes no command to
// read the panel's actual bounds, so this value is fixed and deliberately
// generous rather than pixel-exact — it assumes DevTools docks to the right,
// which only happens for the wide windows that trigger the auto-open in the
// first place (see §7 of docs/PLAN_WINDOW_AUTOFIT.md for the bottom-dock
// case this does not cover).
const DevToolsReservedWidth = 420

// RequiredWindowSize returns the physical window size needed to show a
// requested viewport of (reqW, reqH) without DevTools covering any of it,
// clamped to the detected monitor. It never returns less than the window's
// current size: growth is one-directional, the existing window size is a
// floor. If the monitor size has not been detected yet, it is detected now
// (mirrors the lazy-detect in GetPresetSize).
func (b *DevBrowser) RequiredWindowSize(reqW, reqH int) (int, int) {
	b.Mu.Lock()
	monW, monH := b.MonitorWidth, b.MonitorHeight
	b.Mu.Unlock()

	if monW == 0 || monH == 0 {
		b.DetectMonitorSize()
		b.Mu.Lock()
		monW, monH = b.MonitorWidth, b.MonitorHeight
		b.Mu.Unlock()
	}

	b.Mu.Lock()
	curW, curH := b.Width, b.Height
	reserved := b.DevToolsReserved
	b.Mu.Unlock()

	neededW := reqW
	if reserved {
		neededW += DevToolsReservedWidth
	}
	neededH := reqH

	if neededW < curW {
		neededW = curW
	}
	if neededH < curH {
		neededH = curH
	}

	return b.CalculateConstrainedSize(neededW, neededH, monW, monH)
}

// getPresetSize calculates the optimal dimensions for a requested preset mode.
// It uses predefined base sizes but ensures they fit within the current monitor constraints.
func (b *DevBrowser) GetPresetSize(mode string) (int, int, error) {
	var baseW, baseH int

	// Base definitions
	switch mode {
	case "desktop":
		baseW, baseH = 1440, 900
	case "mobile":
		baseW, baseH = 375, 812
	case "tablet":
		baseW, baseH = 768, 1024
	default:
		return 0, 0, fmt.Errorf("unknown mode: %s", mode)
	}

	b.Mu.Lock()
	monW := b.MonitorWidth
	monH := b.MonitorHeight
	b.Mu.Unlock()

	// If monitor size not detected yet (lazy load), try to detect it now
	if monW == 0 || monH == 0 {
		b.DetectMonitorSize()
		// Re-read
		b.Mu.Lock()
		monW = b.MonitorWidth
		monH = b.MonitorHeight
		b.Mu.Unlock()
	}

	// If still not detected, return base size
	if monW == 0 || monH == 0 {
		return baseW, baseH, nil
	}

	// Calculate constrained size
	// We currently use clamping ("adjust to them") which is standard for maximizing space
	// while ensuring it fits.
	finalW, finalH := b.CalculateConstrainedSize(baseW, baseH, monW, monH)

	return finalW, finalH, nil
}
