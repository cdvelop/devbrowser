package devbrowser_test

import (
	"webtyp.com/devbrowser"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"webtyp.com/devbrowser/chromedp"
	"webtyp.com/mcp"
)

func TestBrowserNavigation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/page2" {
			fmt.Fprint(w, "<h1>Page 2</h1>")
			return
		}
		fmt.Fprint(w, "<h1>Home</h1>")
	}))
	defer ts.Close()

	db, _ := DefaultTestBrowser()
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	db.IsOpenFlag = true
	defer db.CloseBrowser()

	navTools := db.GetNavigationTools()
	var navigateTool *mcp.Tool
	for i := range navTools {
		if navTools[i].Name == "browser_navigate" {
			navigateTool = &navTools[i]
			break
		}
	}

	if navigateTool == nil {
		t.Fatal("browser_navigate tool not found")
	}

	// Test navigation
	args := devbrowser.NavigateArgs{Url: ts.URL + "/page2"}
	req := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_navigate",
			Arguments: encodeArgs(&args),
		},
		Action: 'u',
	}

	_, err := navigateTool.Execute(nil, req)
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	var title string
	err = chromedp.Run(db.Ctx, chromedp.Title(&title))
	if err != nil {
		t.Fatalf("Failed to get title: %v", err)
	}
}
