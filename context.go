package devbrowser

import (
	"context"

	"github.com/chromedp/chromedp"
)

func (h *DevBrowser) CreateBrowserContext() error {

	// fmt.Printf("tamaño monitor: [%d] x [%d] BrowserpositionAndSize: [%v]\n", width, height, BrowserpositionAndSize)

	opts := append(

		// select all the elements after the third element
		chromedp.DefaultExecAllocatorOptions[:],
		// chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", false), // Desactivar el modo headless

		// chromedp.NoFirstRun,
		// chromedp.NoDefaultBrowserCheck,

		//quitar mensaje: Chrome is being controlled by automated test software

		// chromedp.Flag("--webview-log-js-console-messages", true),
		chromedp.WindowSize(h.width, h.height),
		chromedp.Flag("window-position", h.position),
		// chromedp.WindowSize(1530, 870),
		// chromedp.Flag("window-position", "1540,0"),
		chromedp.Flag("use-fake-ui-for-media-stream", true),
		chromedp.Flag("no-focus-on-load", true),
		// chromedp.Flag("exclude-switches", "enable-automation"),
		// chromedp.Flag("disable-blink-features", "AutomationControlled"),
		// chromedp.NoFirstRun,
		// chromedp.NoDefaultBrowserCheck,
		// chromedp.Flag("disable-infobars", true),
		// chromedp.Flag("enable-automation", true),
		// chromedp.Flag("disable-infobars", true),
		// chromedp.Flag("exclude-switches", "disable-infobars"),

		chromedp.Flag("disable-blink-features", "WebFontsInterventionV2"), //remove warning font in console [Intervention] Slow network is detected.
		chromedp.Flag("auto-open-devtools-for-tabs", true),
	)

	parentCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)

	h.Context, h.CancelFunc = chromedp.NewContext(parentCtx)

	return nil
}
