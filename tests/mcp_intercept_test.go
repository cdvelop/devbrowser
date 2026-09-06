package devbrowser_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"webtyp.com/devbrowser"
	"webtyp.com/devbrowser/chromedp"
	"webtyp.com/json"
	"webtyp.com/mcp"
)

func TestInterceptRequest_CapturesBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"status":"ok","received":true}`)
			return
		}
		fmt.Fprint(w, `<html><body><script>
            async function callApi() {
                await fetch('/api', {
                    method: 'POST',
                    body: JSON.stringify({x: 1})
                });
            }
        </script></body></html>`)
	}))
	defer ts.Close()

	db, _ := DefaultTestBrowser()
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	db.IsOpenFlag = true
	db.InitializeInterceptCapture()
	defer db.CloseBrowser()

	if err := db.NavigateToURL(ts.URL); err != nil {
		t.Fatal(err)
	}

	tools := db.GetInterceptTools()
	tool := tools[0]

	// Start interception
	startArgs := devbrowser.InterceptRequestArgs{Action: "start"}
	startReq := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_intercept_request",
			Arguments: encodeArgs(&startArgs),
		},
		Action: 'u',
	}
	_, err := tool.Execute(nil, startReq)
	if err != nil {
		t.Fatal(err)
	}

	// Trigger fetch
	var evalResult interface{}
	err = chromedp.Run(db.Ctx, chromedp.Evaluate(`callApi()`, &evalResult))
	if err != nil {
		t.Fatal(err)
	}

	// Give it a moment to capture
	time.Sleep(500 * time.Millisecond)

	// Get results
	getArgs := devbrowser.InterceptRequestArgs{Action: "get"}
	getReq := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_intercept_request",
			Arguments: encodeArgs(&getArgs),
		},
		Action: 'r',
	}
	res, err := tool.Execute(nil, getReq)
	if err != nil {
		t.Fatal(err)
	}

	var contents mcp.TextContentList
	if err := json.Decode(string(res.Content), &contents); err != nil {
		t.Fatal(err)
	}
	resText := contents[0].Text

	if !strings.Contains(resText, "/api") {
		t.Errorf("Expected intercepted request to /api not found. Got: %s", resText)
	}
	if !strings.Contains(resText, `{"x":1}`) {
		t.Errorf("Expected request body not found. Got: %s", resText)
	}
	if !strings.Contains(resText, `"received":true`) {
		t.Errorf("Expected response body not found. Got: %s", resText)
	}

	// Stop interception
	stopArgs := devbrowser.InterceptRequestArgs{Action: "stop"}
	stopReq := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_intercept_request",
			Arguments: encodeArgs(&stopArgs),
		},
		Action: 'u',
	}
	_, err = tool.Execute(nil, stopReq)
	if err != nil {
		t.Fatal(err)
	}
}

func TestInterceptRequest_LimitsMemory(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body></body></html>`)
	}))
	defer ts.Close()

	db, _ := DefaultTestBrowser()
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	db.IsOpenFlag = true
	db.InitializeInterceptCapture()
	defer db.CloseBrowser()

	if err := db.NavigateToURL(ts.URL); err != nil {
		t.Fatal(err)
	}

	tools := db.GetInterceptTools()
	tool := tools[0]

	// Start interception
	startArgs := devbrowser.InterceptRequestArgs{Action: "start"}
	startReq := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_intercept_request",
			Arguments: encodeArgs(&startArgs),
		},
		Action: 'u',
	}
	tool.Execute(nil, startReq)

	// Trigger many fetches
	script := `for(let i=0; i<110; i++) fetch("/")`
	chromedp.Run(db.Ctx, chromedp.Evaluate(script, nil))

	// Give it a moment to capture
	time.Sleep(1 * time.Second)

	// Verify limit
	db.InterceptMutex.Lock()
	count := len(db.InterceptedReqs)
	db.InterceptMutex.Unlock()
	if count > 100 {
		t.Errorf("InterceptedReqs exceeded limit: %d", count)
	}
}
