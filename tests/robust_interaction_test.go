package devbrowser_test

import (
	"webtyp.com/devbrowser"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"webtyp.com/devbrowser/chromedp"
	"webtyp.com/mcp"
)

func TestRobustInteraction(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `
			<!DOCTYPE html>
			<html>
			<body>
				<button id="btn" onclick="this.innerText='Clicked'">Original</button>
				<div id="cover" style="position:fixed; top:0; left:0; Width:100%; Height:100%; background:rgba(0,0,0,0.1); z-index:100;"></div>
				<script>
					// Remove cover after 200ms
					setTimeout(() => {
						document.getElementById('cover').remove();
					}, 200);
				</script>
			</body>
			</html>
		`)
	}))
	defer ts.Close()

	db, _ := DefaultTestBrowser(func(msg ...any) {
		t.Log(msg...)
	})
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatalf("failed to create browser context: %v", err)
	}
	db.IsOpenFlag = true
	defer db.CloseBrowser()

	err := chromedp.Run(db.Ctx, chromedp.Navigate(ts.URL))
	if err != nil {
		t.Fatalf("failed to navigate: %v", err)
	}

	interactionTools := db.GetInteractionTools()
	var clickTool *mcp.Tool
	for i := range interactionTools {
		if interactionTools[i].Name == "browser_click_element" {
			clickTool = &interactionTools[i]
			break
		}
	}

	if clickTool == nil {
		t.Fatal("browser_click_element tool not found")
	}

	// Wait for cover to be gone
	time.Sleep(300 * time.Millisecond)

	// Use WaitAfter to allow for removal of cover and transition
	args := devbrowser.ClickElementArgs{Selector: "#btn", Timeout: 2000, WaitAfter: 500}
	req := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_click_element",
			Arguments: encodeArgs(&args),
		},
		Action: 'u',
	}

	_, err = clickTool.Execute(nil, req)
	if err != nil {
		t.Fatalf("Robust click failed: %v", err)
	}

	// Give a little more time for the UI to update
	time.Sleep(200 * time.Millisecond)

	var btnText string
	err = chromedp.Run(db.Ctx, chromedp.Text("#btn", &btnText))
	if err != nil {
		t.Fatalf("failed to get button text: %v", err)
	}

	if btnText != "Clicked" {
		t.Errorf("Expected button text 'Clicked', got '%s'", btnText)
	}
}
