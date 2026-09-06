package devbrowser

import (
	"fmt"

	"webtyp.com/context"
	"webtyp.com/devbrowser/chromedp"
	"webtyp.com/mcp"
)

// InspectElementJS extracts detailed element information like Chrome DevTools.
// Returns a JSON-like string with box model, position, styles, and accessibility info.
const InspectElementJS = `
(selector) => {
	const el = document.querySelector(selector);
	if (!el) return JSON.stringify({ error: 'Element not found: ' + selector });

	const rect = el.getBoundingClientRect();
	const style = window.getComputedStyle(el);

	// Box Model
	const boxModel = {
		width: rect.width,
		height: rect.height,
		padding: {
			top: parseFloat(style.paddingTop),
			right: parseFloat(style.paddingRight),
			bottom: parseFloat(style.paddingBottom),
			left: parseFloat(style.paddingLeft)
		},
		margin: {
			top: parseFloat(style.marginTop),
			right: parseFloat(style.marginRight),
			bottom: parseFloat(style.marginBottom),
			left: parseFloat(style.marginLeft)
		},
		border: {
			top: parseFloat(style.borderTopWidth),
			right: parseFloat(style.borderRightWidth),
			bottom: parseFloat(style.borderBottomWidth),
			left: parseFloat(style.borderLeftWidth)
		}
	};

	// Position
	const position = {
		type: style.position,
		top: rect.top,
		left: rect.left,
		right: rect.right,
		bottom: rect.bottom,
		// Offset from document
		offsetTop: el.offsetTop,
		offsetLeft: el.offsetLeft,
		// Scroll position
		scrollTop: el.scrollTop,
		scrollLeft: el.scrollLeft
	};

	// Layout
	const layout = {
		display: style.display,
		flexDirection: style.flexDirection,
		justifyContent: style.justifyContent,
		alignItems: style.alignItems,
		gridTemplateColumns: style.gridTemplateColumns,
		gridTemplateRows: style.gridTemplateRows,
		gap: style.gap,
		overflow: style.overflow,
		zIndex: style.zIndex
	};

	// Typography
	const typography = {
		fontFamily: style.fontFamily,
		fontSize: style.fontSize,
		fontWeight: style.fontWeight,
		lineHeight: style.lineHeight,
		textAlign: style.textAlign,
		color: style.color
	};

	// Background
	const background = {
		color: style.backgroundColor,
		image: style.backgroundImage !== 'none' ? style.backgroundImage : null
	};

	// Accessibility
	const accessibility = {
		role: el.getAttribute('role'),
		ariaLabel: el.getAttribute('aria-label'),
		ariaDescribedBy: el.getAttribute('aria-describedby'),
		tabIndex: el.tabIndex,
		isKeyboardFocusable: el.tabIndex >= 0 || ['A', 'BUTTON', 'INPUT', 'SELECT', 'TEXTAREA'].includes(el.tagName)
	};

	// Element identity
	const identity = {
		tagName: el.tagName.toLowerCase(),
		id: el.id || null,
		className: el.className || null,
		name: el.getAttribute('name')
	};

	return JSON.stringify({
		identity,
		boxModel,
		position,
		layout,
		typography,
		background,
		accessibility
	}, null, 2);
}
`

func (b *DevBrowser) GetInspectTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "browser_inspect_element",
			Description: "Inspect a specific element to get detailed CSS properties like Chrome DevTools. Returns box model (width, height, padding, margin, border), position (top, left, offset), layout (display, flex, grid), typography (font, color), and accessibility info.",
			Args: new(InspectElementArgs),
			Resource:    "browser",
			Action:      'r',
			Execute: func(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
				if !b.IsOpenFlag {
					return nil, ErrBrowserNotOpen
				}

				var args InspectElementArgs
				if err := req.Bind(&args); err != nil {
					return nil, err
				}

				var result string
				js := fmt.Sprintf("(%s)(%q)", InspectElementJS, args.Selector)

				err := chromedp.Run(b.Ctx,
					chromedp.Evaluate(js, &result),
				)

				if err != nil {
					return nil, fmt.Errorf("Failed to inspect element: %v", err)
				}

				return mcp.Text(fmt.Sprintf("Inspect Element: %s\n%s", args.Selector, result)), nil
			},
		},
	}
}
