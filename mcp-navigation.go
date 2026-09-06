package devbrowser

import (
	"fmt"
	"net/url"
	"strings"

	"webtyp.com/context"
	"webtyp.com/mcp"
)

func (b *DevBrowser) GetNavigationTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "browser_navigate",
			Description: "Navigate the browser to a specific Url or relative path (e.g. /login)",
			Args: new(NavigateArgs),
			Resource:    "browser",
			Action:      'u',
			Execute: func(Ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
				if !b.IsOpenFlag {
					return nil, ErrBrowserNotOpen
				}

				var args NavigateArgs
				if err := req.Bind(&args); err != nil {
					return nil, err
				}

				targetURL := args.Url
				if !strings.Contains(targetURL, "://") {
					if b.LastPort == "" {
						return nil, fmt.Errorf("browser has no active app port; open the app first")
					}

					scheme := "http"
					if b.LastHttps {
						scheme = "https"
					}

					base, err := url.Parse(fmt.Sprintf("%s://localhost:%s", scheme, b.LastPort))
					if err != nil {
						return nil, fmt.Errorf("failed to parse base Url: %v", err)
					}

					rel, err := url.Parse(targetURL)
					if err != nil {
						return nil, fmt.Errorf("failed to parse target path: %v", err)
					}

					targetURL = base.ResolveReference(rel).String()
				}

				err := b.NavigateToURL(targetURL)
				if err != nil {
					return nil, fmt.Errorf("Error navigating to %s: %v", targetURL, err)
				}

				current, err := b.CurrentURL()
				if err != nil {
					return mcp.Text(fmt.Sprintf("Navigated to %s (could not get current Url: %v)", targetURL, err)), nil
				}

				return mcp.Text(fmt.Sprintf("Navigated to %s (current: %s)", targetURL, current)), nil
			},
		},
	}
}
