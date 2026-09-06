package devbrowser

import (
	"fmt"

	"webtyp.com/context"
	"webtyp.com/mcp"
)

func (b *DevBrowser) GetConsoleTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "browser_get_console",
			Description: "Get browser JavaScript console logs to debug WASM runtime errors, console.log outputs, or frontend issues.",
			Args: new(GetConsoleArgs),
			Resource:    "browser",
			Action:      'r',
			Execute: func(Ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
				if !b.IsOpenFlag {
					return nil, ErrBrowserNotOpen
				}

				var args GetConsoleArgs
				if err := req.Bind(&args); err != nil {
					return nil, err
				}

				logs, err := b.GetConsoleLogs()
				if err != nil {
					return nil, fmt.Errorf("Failed to get console logs: %v", err)
				}

				if len(logs) == 0 {
					return mcp.Text("No console logs available"), nil
				}

				maxLines := args.Lines
				if maxLines == 0 {
					maxLines = 50
				}

				if len(logs) > int(maxLines) {
					logs = logs[len(logs)-int(maxLines):]
				}

				var result string
				for i, log := range logs {
					if i > 0 {
						result += "\n"
					}
					result += log
				}

				return mcp.Text(result), nil
			},
		},
	}
}
