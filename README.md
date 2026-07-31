# devbrowser
<img src="docs/img/badges.svg">
<img src="docs/img/badges.svg">

A lightweight Go library for launching and controlling web browsers programmatically, designed for automation and development tools.

## Usage

The main entry point is the `New` function, which creates a new browser controller:

```go
import "github.com/tinywasm/devbrowser"

type myServerConfig struct{}
func (myServerConfig) GetServerPort() string { return "8080" }

type myUI struct{}
func (myUI) ReturnFocus() {}

func main() {
	exitChan := make(chan bool)
	browser := devbrowser.New(myServerConfig{}, myUI{}, exitChan)
	err := browser.OpenBrowser()
	if err != nil {
		// handle error
	}
	// ... use browser ...
	browser.CloseBrowser()
}
```

## Public API

- `New(sc serverConfig, ui userInterface, exitChan chan bool) *DevBrowser`: Create a new DevBrowser instance.
- `(*DevBrowser) OpenBrowser() error`: Launch a new browser window.
- `(*DevBrowser) CloseBrowser() error`: Close the browser and clean up resources.
- `(*DevBrowser) Reload() error`: Reload the current page in the browser.
- `(*DevBrowser) RestartBrowser() error`: Restart the browser (close and reopen).
- `(*DevBrowser) BrowserStartUrlChanged(fieldName, oldValue, newValue string) error`: Handle changes to the start URL and restart the browser if open.
- `(*DevBrowser) BrowserPositionAndSizeChanged(fieldName, oldValue, newValue string) error`: Change the browser window's position and size, and restart the browser.
- `(*DevBrowser) Name() string` and `(*DevBrowser) Label() string`: For UI integration, returns the component name and label.
- `(*DevBrowser) Execute(progress func(msgs ...any))`: For UI integration, toggles browser open/close and reports progress.

- `(*DevBrowser) SetHeadless(headless bool)`: Configure whether the browser runs in headless mode (without a visible UI).
	- Signature: `func (b *DevBrowser) SetHeadless(headless bool)`
	- Default: `false` (shows the browser window). This is convenient for local development and debugging.
	- Tests: the test helper `DefaultTestBrowser()` configures the returned `DevBrowser` with `headless = true` so unit tests run without requiring a graphical display.
	- Notes: Call this before `OpenBrowser()` (or before the browser context is created) to ensure the headless flag is applied when launching Chrome/Chromium.
	- Example:

```go
db := devbrowser.New(myServerConfig{}, myUI{}, exitChan)
// run with no UI (useful in CI/tests)
db.SetHeadless(true)
err := db.OpenBrowser()
if err != nil {
		// handle error
}
```

## MCP Tools

The following Model Context Protocol (MCP) tools are available for browser automation:

| Tool | Description |
|---|---|
| `browser_get_console` | Capture console messages from the loaded page |
| `browser_emulate_device` | Emulate a mobile or tablet device |
| `browser_screenshot` | Take a screenshot of the current page |
| `browser_save_screenshot` | Capture a screenshot and write it as a durable PNG file on disk (with path validation, overwrite prevention, and mutual exclusivity) |
| `browser_get_content` | Get simplified semantic HTML of the page |
| `browser_click_element` | Click on an element specified by a selector |
| `browser_fill_element` | Fill an input field with a value |
| `browser_navigate` | Navigate to a specific URL or relative path |
| `browser_swipe_element` | Perform a swipe gesture on an element |
| `browser_inspect_element` | Get detailed information about a DOM element |
| `browser_get_performance` | Get page performance metrics |
| `browser_get_network_logs` | Get network requests and responses metadata |
| `browser_evaluate_js` | Execute JavaScript in the browser context |
| `browser_get_errors` | Get captured JavaScript errors |
| `browser_get_source` | Get the raw HTML (outerHTML) of the entire page or a specific element by selector |
| `browser_get_styles` | Extract CSS rules from loaded stylesheets, with an optional selector filter |
| `browser_get_storage` | Read localStorage, sessionStorage, or cookies from the current domain |
| `browser_get_asset` | Download the content of a JS or CSS file by URL using the active session |
| `browser_intercept_request` | Capture bodies of requests and responses XHR/fetch calls (CDP Fetch domain) |

- `(*DevBrowser) GetConsoleLogs() ([]string, error)`: Capture console messages from the loaded page.
	- Signature: `func (b *DevBrowser) GetConsoleLogs() ([]string, error)`
	- Behavior: injects a small script into the page that maintains `window.__consoleLogs` and returns its contents as a slice of strings. Captures `console.log`, `console.error`, `console.warn`, and `console.info` messages.
	- Requirements: the browser context must be initialized (`OpenBrowser()` called and context created). Returns an error if the context is not ready or the evaluation fails.
	- Example:

```go
logs, err := db.GetConsoleLogs()
if err != nil {
		// handle error
}
for _, l := range logs {
		fmt.Println(l)
}
```

- `(*DevBrowser) ClearConsoleLogs() error`: Clear the in-page captured console log buffer.
	- Signature: `func (b *DevBrowser) ClearConsoleLogs() error`
	- Behavior: executes a small script that resets `window.__consoleLogs = []` if present.
	- Requirements: the browser context must be initialized. Returns an error if the evaluation fails.
	- Example:

```go
err := db.ClearConsoleLogs()
if err != nil {
		// handle error
}
```

### Saving screenshots directly to disk

The `browser_save_screenshot` tool is designed to produce durable documentation artifacts (such as widget and component reference images) directly on the local filesystem. This tool is isolated to the separate `browser_file` resource type for maximum security and access control.

#### Worked Example: Documenting a Widget Component

1. **Emulate the desired viewport (e.g., desktop):**
   ```json
   {
     "mode": "desktop"
   }
   ```
   *Result:* Sets the emulated device viewport to exactly `1440x900`.

2. **Capture and save a specific component by CSS selector:**
   ```json
   {
     "dir": "./docs/img",
     "name": "badges",
     "selector": ".badge-container",
     "overwrite": true
   }
   ```
   *Result:* Captures only the `.badge-container` element, cleans and resolves the destination directory to an absolute path, creates `docs/img/` if missing, and safely writes the PNG file to `docs/img/badges.png`.

3. **Reference the file in your documentation:**
   ```markdown
   <img src="docs/img/badges.png">
   ```
