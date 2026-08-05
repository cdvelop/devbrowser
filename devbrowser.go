package devbrowser

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/tinywasm/devbrowser/chromedp"
)

type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

type UserInterface interface {
	RefreshUI()
	ReturnFocus() error
}

type DevBrowser struct {
	UI             UserInterface
	Width          int    // ej "800" default "1024"
	Height         int    //ej: "600" default "768"
	Position       string //ej: "1930,0" (when you have second monitor) default: "0,0"
	Headless       bool   // true para modo headless (sin UI), false muestra el navegador
	AutoStart      bool   // true if browser should auto-open on startup
	MonitorWidth   int    // Detected monitor availability width
	MonitorHeight  int    // Detected monitor availability height
	SizeConfigured bool   // Track if size was loaded from storage
	ViewportMode   string // Current emulation mode ("mobile", "tablet", "desktop", "off", "")
	ViewportDevice string // Current emulation device name (e.g. "iphone15promax")
	FirstCall      bool   // Internal flag to track if OpenBrowser was called for the first time
	OpenedOnce     bool   // Internal flag to track if browser was actually opened at least once

	LastPort  string
	LastHttps bool

	IsOpenFlag bool // Indica si el navegador está abierto

	// ready is true only AFTER the initial open fully completed (browser
	// allocated + navigated). IsOpenFlag is set optimistically before the async
	// open finishes, so it is NOT a safe signal for issuing chromedp actions:
	// running an action (e.g. Reload) on the context before the first allocation
	// returns makes chromedp allocate a SECOND browser -> double window.
	ready bool

	DB Store // Key-value store para configuración y estado

	// chromedp fields
	Ctx    context.Context
	Cancel context.CancelFunc
	// AllocCancel cancels the exec allocator (the Chrome OS process). Must be
	// called on close, otherwise cancelling only Ctx closes the tab/target but
	// leaves the Chrome window alive -> orphan windows on restart (double window).
	AllocCancel context.CancelFunc

	ReadyChan chan bool
	ErrChan   chan error
	ExitChan  chan bool

	Log func(message ...any) // For logging output (Loggable interface)

	// Console log capture
	ConsoleLogs []string
	LogsMutex   sync.Mutex

	// Network log capture
	NetworkLogs  []NetworkLogEntry
	NetworkMutex sync.Mutex

	// JS error capture
	JsErrors    []JSError
	ErrorsMutex sync.Mutex

	// Request interception
	InterceptActive bool
	InterceptedReqs []InterceptedRequest
	InterceptMutex  sync.Mutex

	// Operation busy flag (atomic) to prevent race conditions and UI blocking
	// 0 = idle, 1 = busy
	Busy int32

	TestMode bool // Skip opening browser in tests

	// Cache configuration
	CacheEnabled bool // Disabled by default for development
	Mu           sync.Mutex
}

// Option configures the DevBrowser
type Option func(*DevBrowser)

// WithCache configures whether the browser cache is enabled
func WithCache(enabled bool) Option {
	return func(b *DevBrowser) {
		b.CacheEnabled = enabled
	}
}

type JSError struct {
	Message      string
	Source       string // File/URL where error occurred
	LineNumber   int
	ColumnNumber int
	StackTrace   string
	Timestamp    time.Time
}

type NetworkLogEntry struct {
	URL       string
	Method    string
	Status    int
	Type      string // xhr, fetch, document, script, image, etc.
	Duration  int64  // milliseconds
	Failed    bool
	ErrorText string
}

/*
devbrowser.New creates a new DevBrowser instance.

	type serverConfig interface {
		GetServerPort() string
	}

	type userInterface interface {
		RefreshUI()
		ReturnFocus() error
	}

	example :  New(userInterface, st, exitChan, WithCache(true))
*/
func New(ui UserInterface, st Store, exitChan chan bool, opts ...Option) *DevBrowser {

	// Initialize clipboard for cross-platform support
	// err := clipboard.Init()
	// if err != nil {
	// 	// Can't log yet, no logger injected
	// }

	browser := &DevBrowser{
		UI:           ui,
		DB:           st,
		Width:        1024, // Default width
		Height:       768,  // Default height
		Position:     "0,0",
		FirstCall:    true,
		ReadyChan:    make(chan bool),
		ErrChan:      make(chan error),
		ExitChan:     exitChan,
		CacheEnabled: false, // Default: Cache disabled for development
	}

	// Apply options
	for _, opt := range opts {
		opt(browser)
	}

	// Load all configuration from store
	browser.LoadConfig()

	//id := atomic.AddInt32(&instanceCounter, 1)
	// Logger not set yet, so we can't log this via b.Logger consistently
	// But let's try just in case it's injected early or for future ref inspection
	// We'll use fmt temporarily for this one-time struct init check if needed,
	// but user dislikes fmt. Let's rely on AutoStart logs primarily.
	// If we really need New log, we might need fmt. But user said no fmt.
	// We'll skip logging in New for now unless critical, but we'll keep the counter.
	// actually, let's just use fmt for the New call since it is before logger init usually
	//fmt.Printf("DEBUG: DevBrowser New Instance #%d created\n", id)

	return browser
}

func (h *DevBrowser) BrowserStartUrlChanged(fieldName string, oldValue, newValue string) error {

	if !h.IsOpenFlag {
		return nil
	}

	return h.RestartBrowser()
}

func (h *DevBrowser) RestartBrowser() error {

	this := errors.New("RestartBrowser")

	err := h.CloseBrowser()
	if err != nil {
		return errors.Join(this, err)
	}

	h.OpenBrowser(h.LastPort, h.LastHttps)

	return nil
}

func (b *DevBrowser) NavigateToURL(url string) error {
	if b.Ctx == nil {
		return errors.New("context not initialized")
	}

	if err := chromedp.Run(b.Ctx, chromedp.Navigate(url)); err != nil {
		return err
	}
	return nil
}

func (b *DevBrowser) CurrentURL() (string, error) {
	if b.Ctx == nil {
		return "", errors.New("context not initialized")
	}

	var url string
	if err := chromedp.Run(b.Ctx, chromedp.Location(&url)); err != nil {
		return "", err
	}
	return url, nil
}

func (b *DevBrowser) Reload() error {
	// Gate on `ready`, not IsOpenFlag: during startup the file watcher can fire
	// a reload while the initial open is still allocating the browser. Running
	// chromedp.Run on the not-yet-allocated context would spawn a SECOND Chrome
	// (the about:blank "double window"). The initial Navigate already shows the
	// current content, so a reload before ready is redundant — skip it.
	b.Mu.Lock()
	ready := b.ready && b.Ctx != nil && b.IsOpenFlag
	b.Mu.Unlock()

	if !ready {
		b.Logger("Reload skipped: browser still opening")
		return nil
	}

	b.Logger("Reload")
	if err := chromedp.Run(b.Ctx, chromedp.Reload()); err != nil {
		return errors.New("Reload " + err.Error())
	}
	return nil
}

func (b *DevBrowser) SetLog(f func(message ...any)) {
	b.Log = f
}

func (b *DevBrowser) GetLog() func(message ...any) {
	return b.Log
}

func (b *DevBrowser) Logger(messages ...any) {
	if b.Log != nil {
		b.Log(messages...)
	}
}

// SetHeadless configura si el navegador debe ejecutarse en modo headless (sin UI).
// Por defecto es false (muestra la ventana del navegador).
// Debe llamarse antes de OpenBrowser().
func (b *DevBrowser) SetHeadless(headless bool) {
	b.Headless = headless
}

func (b *DevBrowser) SetTestMode(testMode bool) {
	b.TestMode = testMode
}

// monitorBrowserClose monitors the browser context and updates state when browser is closed manually
func (b *DevBrowser) monitorBrowserClose() {
	b.Mu.Lock()
	ctx := b.Ctx
	b.Mu.Unlock()

	if ctx == nil {
		return
	}

	// Wait for context to be done (browser closed)
	<-ctx.Done()

	b.Mu.Lock()
	defer b.Mu.Unlock()

	// Only handle if browser was marked as open (manual close by user)
	if b.IsOpenFlag {
		b.Logger("Browser closed by user")
		b.IsOpenFlag = false
		b.Ctx = nil
		b.Cancel = nil
		if b.UI != nil {
			b.UI.RefreshUI()
		}
	}
}

func (b *DevBrowser) IsOpen() bool {
	return b.IsOpenFlag
}

func (b *DevBrowser) InitializeConsoleCapture() error {
	return b.initializeConsoleCapture()
}

func (b *DevBrowser) InitializeInterceptCapture() {
	b.initializeInterceptCapture()
}
