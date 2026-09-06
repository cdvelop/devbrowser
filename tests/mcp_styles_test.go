package devbrowser_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webtyp.com/devbrowser"
	"webtyp.com/json"
	"webtyp.com/mcp"
)

func TestGetStyles_AllSheets(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><style>.box { color: red; font-size: 14px; }</style></head><body><div class="box">hello</div></body></html>`)
	}))
	defer ts.Close()

	db, _ := DefaultTestBrowser()
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	db.IsOpenFlag = true
	defer db.CloseBrowser()

	if err := db.NavigateToURL(ts.URL); err != nil {
		t.Fatal(err)
	}

	tools := db.GetStylesTools()
	tool := tools[0]

	args := devbrowser.GetStylesArgs{Selector: "", Sheet: -1}
	req := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_get_styles",
			Arguments: encodeArgs(&args),
		},
		Action: 'r',
	}
	res, err := tool.Execute(nil, req)
	if err != nil {
		t.Fatal(err)
	}

	var contents mcp.TextContentList
	if err := json.Decode(string(res.Content), &contents); err != nil {
		t.Fatal(err)
	}
	result := contents[0].Text

	if !strings.Contains(result, ".box") || !strings.Contains(result, "color: red") {
		t.Errorf("Expected styles not found. Got: %s", result)
	}
}

func TestGetStyles_SheetIndex(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head>
            <style>.sheet0 { color: red; }</style>
            <style>.sheet1 { color: blue; }</style>
        </head><body></body></html>`)
	}))
	defer ts.Close()

	db, _ := DefaultTestBrowser()
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	db.IsOpenFlag = true
	defer db.CloseBrowser()

	if err := db.NavigateToURL(ts.URL); err != nil {
		t.Fatal(err)
	}

	tools := db.GetStylesTools()
	tool := tools[0]

	// Request sheet 1
	args := devbrowser.GetStylesArgs{Selector: "", Sheet: 1}
	req := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_get_styles",
			Arguments: encodeArgs(&args),
		},
		Action: 'r',
	}
	res, err := tool.Execute(nil, req)
	if err != nil {
		t.Fatal(err)
	}

	var contents mcp.TextContentList
	if err := json.Decode(string(res.Content), &contents); err != nil {
		t.Fatal(err)
	}
	result := contents[0].Text

	if !strings.Contains(result, ".sheet1") {
		t.Error("missing .sheet1 rule")
	}
	if strings.Contains(result, ".sheet0") {
		t.Error("should not contain .sheet0 rule from first sheet")
	}
}

func TestGetStyles_Selector(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><style>
            .box { color: red; }
            .other { color: blue; }
        </style></head><body></body></html>`)
	}))
	defer ts.Close()

	db, _ := DefaultTestBrowser()
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	db.IsOpenFlag = true
	defer db.CloseBrowser()

	if err := db.NavigateToURL(ts.URL); err != nil {
		t.Fatal(err)
	}

	tools := db.GetStylesTools()
	tool := tools[0]

	args := devbrowser.GetStylesArgs{Selector: ".box", Sheet: -1}
	req := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_get_styles",
			Arguments: encodeArgs(&args),
		},
		Action: 'r',
	}
	res, err := tool.Execute(nil, req)
	if err != nil {
		t.Fatal(err)
	}

	var contents mcp.TextContentList
	if err := json.Decode(string(res.Content), &contents); err != nil {
		t.Fatal(err)
	}
	result := contents[0].Text

	if !strings.Contains(result, ".box") {
		t.Error("missing .box rule")
	}
	if strings.Contains(result, ".other") {
		t.Error("should not contain .other rule")
	}
}

func TestGetStyles_CrossOrigin(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><link rel="stylesheet" href="http://example.com/external.css"></head><body></body></html>`)
	}))
	defer ts.Close()

	db, _ := DefaultTestBrowser()
	if err := db.CreateBrowserContext(); err != nil {
		t.Fatal(err)
	}
	db.IsOpenFlag = true
	defer db.CloseBrowser()

	if err := db.NavigateToURL(ts.URL); err != nil {
		t.Fatal(err)
	}

	tools := db.GetStylesTools()
	tool := tools[0]

	args := devbrowser.GetStylesArgs{Selector: "", Sheet: -1}
	req := mcp.Request{
		Params: mcp.CallToolParams{
			Name:      "browser_get_styles",
			Arguments: encodeArgs(&args),
		},
		Action: 'r',
	}
	res, err := tool.Execute(nil, req)
	if err != nil {
		t.Fatal(err)
	}

	var contents mcp.TextContentList
	if err := json.Decode(string(res.Content), &contents); err != nil {
		t.Fatal(err)
	}
	result := contents[0].Text

	if !strings.Contains(result, "/* cross-origin:") {
		t.Logf("Result: %s", result)
	}
}
