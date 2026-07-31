package devbrowser

import (
	"fmt"

	"github.com/tinywasm/context"
	"github.com/tinywasm/devbrowser/cdproto/emulation"
	"github.com/tinywasm/devbrowser/chromedp"
	"github.com/tinywasm/mcp"
)

const (
	MobileWidth   = 375
	MobileHeight  = 812
	TabletWidth   = 768
	TabletHeight  = 1024
	DesktopWidth  = 1440
	DesktopHeight = 900
)

func (b *DevBrowser) GetManagementTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "browser_emulate_device",
			Description: "Emulate a mobile device or tablet viewport without resizing the physical window. This toggle affects rendering and touch events. This change is persisted.",
			Args:        new(EmulateDeviceArgs),
			Resource:    "browser",
			Action:      'u',
			Execute: func(Ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
				var args EmulateDeviceArgs
				if err := req.Bind(&args); err != nil {
					return nil, err
				}

				// Validate the mode *before* assigning and saving
				switch args.Mode {
				case "mobile", "tablet", "desktop", "off", "":
					// valid modes
				default:
					return nil, fmt.Errorf("unsupported mode: %s", args.Mode)
				}

				b.Mu.Lock()
				b.ViewportMode = args.Mode
				b.Mu.Unlock()

				if err := b.SaveConfig(); err != nil {
					b.Logger(fmt.Sprintf("Error saving emulation config: %v", err))
				}

				var actualW, actualH int
				if b.IsOpen() && b.Ctx != nil {
					if err := b.applyDeviceEmulation(); err != nil {
						return nil, err
					}
					b.UI.RefreshUI()

					// Read back dimensions dynamically
					if err := chromedp.Run(b.Ctx,
						chromedp.Evaluate(`window.innerWidth`, &actualW),
						chromedp.Evaluate(`window.innerHeight`, &actualH),
					); err != nil {
						b.Logger(fmt.Sprintf("Failed to read back viewport: %v", err))
					}
				}

				statusMsg := fmt.Sprintf("Device emulation set to %s", args.Mode)
				if b.IsOpen() && b.Ctx != nil && actualW > 0 && actualH > 0 {
					statusMsg = fmt.Sprintf("Device emulation set to %s (viewport %dx%d)", args.Mode, actualW, actualH)
				}

				if args.Capture {
					var res *ScreenshotResult
					var err error
					if args.Selector != "" {
						res, err = b.CaptureElementScreenshot(args.Selector)
					} else {
						res, err = b.CaptureScreenshot(false)
					}

					if err != nil {
						return nil, fmt.Errorf("%s. Failed to capture screenshot: %v", statusMsg, err)
					}

					// Build visual context report
					contextReport := fmt.Sprintf(
						"%s\nURL: %s | Title: %s | Viewport: %dx%d\n\n%s",
						statusMsg,
						res.PageURL,
						res.PageTitle,
						res.Width, res.Height,
						res.HTMLStructure,
					)

					return mcp.Text(contextReport), nil
				} else {
					return mcp.Text(statusMsg), nil
				}
			},
		},
	}
}

// applyDeviceEmulation applies the current b.ViewportMode using CDP emulation commands.
func (b *DevBrowser) applyDeviceEmulation() error {
	b.Mu.Lock()
	mode := b.ViewportMode
	b.Mu.Unlock()

	var actions []chromedp.Action

	switch mode {
	case "mobile":
		actions = append(actions,
			chromedp.EmulateViewport(MobileWidth, MobileHeight, chromedp.EmulateMobile),
			emulation.SetTouchEmulationEnabled(true),
		)
	case "tablet":
		actions = append(actions,
			chromedp.EmulateViewport(TabletWidth, TabletHeight, chromedp.EmulateMobile),
			emulation.SetTouchEmulationEnabled(true),
		)
	case "desktop":
		actions = append(actions,
			chromedp.EmulateViewport(DesktopWidth, DesktopHeight), // NOT EmulateMobile
			emulation.SetTouchEmulationEnabled(false),
		)
	case "off", "":
		// Clear overrides by emulating a standard desktop viewport
		// We use ClearDeviceMetricsOverride to reset to the window size,
		// allowing the browser layout to adjust naturally to DevTools.
		actions = append(actions,
			emulation.ClearDeviceMetricsOverride(),
			emulation.SetTouchEmulationEnabled(false),
		)
	default:
		return fmt.Errorf("unsupported mode: %s", mode)
	}

	return chromedp.Run(b.Ctx, actions...)
}
