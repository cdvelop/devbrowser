---
PLAN: "desktop must mean desktop, and captures must be able to land in a directory"
TAG: v0.1.0
STATUS: running
SESSION: 17762207371060623009
---

# Plan — `devbrowser`: a real desktop viewport, and screenshots written to disk

Two independent changes to the MCP surface. Part A fixes a tool that reports success
while doing something other than what it was asked. Part B adds a tool that does not
exist yet.

---

# Part A — `browser_emulate_device` cannot produce a desktop viewport

## The defect

`mcp-management.go`, `applyDeviceEmulation()`:

```go
switch mode {
case "mobile":
	actions = append(actions,
		chromedp.EmulateViewport(375, 812, chromedp.EmulateMobile),
		emulation.SetTouchEmulationEnabled(true),
	)
case "tablet":
	actions = append(actions,
		chromedp.EmulateViewport(768, 1024, chromedp.EmulateMobile),
		emulation.SetTouchEmulationEnabled(true),
	)
case "desktop", "off", "":
	actions = append(actions,
		emulation.ClearDeviceMetricsOverride(),
		emulation.SetTouchEmulationEnabled(false),
	)
}
```

`mobile` and `tablet` **pin** a viewport. `desktop` does not — it shares a branch
with `off` and merely clears the override, handing the page back whatever the
physical Chrome window happens to be.

Three names, two behaviours, and the mismatch is on the one name that sounds like a
guarantee. `desktop` promises a desktop viewport and delivers *the current window*.

**This is not theoretical.** In a real session the window was 400 px wide, and
`browser_emulate_device{mode:"desktop"}` returned:

```
Device emulation set to desktop
```

while the page measured `innerWidth: 400` — below the 640 px mobile breakpoint. Every
desktop-only CSS rule stayed inactive. There was **no mode that could produce a
desktop viewport**, and the tool reported success for all of them. Verifying
desktop-only layout became impossible, and the failure was silent: the caller had to
measure `innerWidth` by hand to discover it.

`devbrowser` positions and sizes that window itself (`position.go`,
`monitor_geometry.go`), so a narrow window is a state this project routinely creates.

### Why it matters more than the width

Nothing in the reply says what the caller actually got. Contrast with the `capture:
true` path a few lines above, which *does* report `Viewport: %dx%d`. The plain path
returns only `"Device emulation set to %s"`. Per
`app-releases/docs/CONSTRUCTION_HARNESS.md` principle 6, the order of preference is
compile error → loud diagnostic → **never** silent failure. This is the third.

## The fix

**1. Give `desktop` its own branch that pins a viewport.**

```go
case "desktop":
	actions = append(actions,
		chromedp.EmulateViewport(1440, 900),   // NOT EmulateMobile
		emulation.SetTouchEmulationEnabled(false),
	)
case "off", "":
	actions = append(actions,
		emulation.ClearDeviceMetricsOverride(),
		emulation.SetTouchEmulationEnabled(false),
	)
```

Now each mode means exactly one thing: `mobile`/`tablet`/`desktop` pin a viewport,
`off` returns the page to the physical window. That is the "one intent, one path"
rule, and it is what makes `desktop` usable for verifying desktop breakpoints
regardless of how the window is sized.

1440×900 is the recommendation: comfortably above the 1024 px desktop breakpoint the
`tinywasm/css` scale uses, and a common laptop logical resolution. Whatever number is
chosen, put the three widths in **named constants next to each other** so the
relationship to the CSS breakpoints is visible in one place rather than buried in a
switch.

**2. Always report the resulting viewport.** The reply must name what the caller got,
not merely echo what they asked for:

```
Device emulation set to desktop (viewport 1440x900)
```

Read it back from the page rather than printing the constant — a constant that
disagrees with reality is exactly the failure being fixed. `capture: true` already
reads it back; the plain path should use the same source.

**3. Reject unknown modes at the boundary.** `default:` already returns an error
inside `applyDeviceEmulation`, but that runs **only when the browser is open** —
`mcp-management.go` writes `b.ViewportMode = args.Mode` and persists it via
`SaveConfig()` *before* any validation. A typo is stored and silently reloaded next
session (`config.go:56`). Validate the mode **before** assigning it, and prefer
typing it out of existence: the `mode` field is a bare `model.Text()` in
`EmulateDeviceArgsModel`, and an enum of four values is what the harness would use.

## Acceptance criteria (Part A)

1. `gotest` green.
2. A test asserts the branches are **distinct** — that `desktop` and `off` do not
   produce the same action list. The current test (`tests/device_emulation_test.go`)
   only checks that the field round-trips, which is why this shipped.
3. A test asserts an unknown mode is rejected **without** being persisted.
4. Live: with the Chrome window resized narrower than 640 px,
   `browser_emulate_device{mode:"desktop"}` followed by `browser_evaluate_js` for
   `innerWidth` returns ≥ 1024. This is the exact scenario that failed; a test on a
   normally-sized window does not cover it.
5. Live: `mode:"off"` returns the page to the physical window size.
6. The reply text contains the resulting viewport in every path.

## Noted, not fixed (Part A)

Observed while reproducing, not diagnosed — record so it is not mistaken for new:

- `mobile` requests `chromedp.EmulateMobile` and `SetTouchEmulationEnabled(true)`,
  yet the page reports `"ontouchstart" in window === false`. Either touch emulation
  is not landing, or `ontouchstart` is the wrong probe. Worth a look; out of scope
  here.
- Emulation sets `devicePixelRatio` to 1.0 for mobile and restores 1.25 on clear.
  Deliberate or incidental is unclear. It matters for Part B, where DPR decides the
  pixel dimensions of a saved image.

---

# Part B — a tool that writes the capture to a directory

## Why a new tool rather than a flag on `browser_screenshot`

`browser_screenshot` exists to show an agent **what is on screen right now**: it
returns an `ImageBlock` plus an HTML-structure report, and drops the PNG on the
clipboard. It is ephemeral by design.

Writing a file is a different intent — producing a **durable artifact**, typically a
widget or component image referenced from documentation. Different lifetime,
different failure modes (path, permissions, overwrite), different success value (a
path, not an image). Folding it into the existing tool as a `path` flag would give
one tool two meanings and force every caller to read the docs to find out which they
get. One intent, one path.

`docs/img/` in this repo is already the shape being served: `README.md` references
`docs/img/badges.svg`. This tool is what fills such directories for the widget and
component repositories.

## Who is allowed to write, and how

Worth stating plainly, because the layers get conflated and only one of them is real
enforcement.

**MCP does not write the file.** The protocol is JSON-RPC between the client and this
process: the client sends `tools/call` with the arguments, this process runs
`Execute`, and an ordinary `os.WriteFile` inside it touches the disk. The protocol
carries the request and the reply; it never sees the filesystem. **`tinywasm/mcp`
needs no change to support this** — it is not in the path.

Four layers, outermost first:

| layer | what it decides | how good it is |
|---|---|---|
| Host (Claude Code) | whether the tool is called at all | coarse: it knows the tool's *name*, not that it writes files |
| `tinywasm/mcp` | identity + permission on `Resource`/`Action` | already correct — see below |
| `devbrowser` | which resource this tool declares | **the gap** |
| OS | what the process can touch | none: it runs as the user, unsandboxed |

`tinywasm/mcp` already has the vocabulary and the right default. `server.go`:
`AccessGuarded` is the **zero value** and demands identity *and* permission, and
`model.Allowed` denies when `Authorize` is nil — *"the absence of an answer is not
permission"*. Deny by default, exactly as the harness requires.

**The gap is in this repository's use of it.** Every tool in `mcp-*.go` declares
`Resource: "browser"`. Reading the DOM and writing a PNG onto the operator's disk are
therefore *the same permission*: granting `browser` to inspect a page also grants
file writes. That is a permission that cannot be expressed, let alone withheld.

So this tool must declare its own:

```go
Resource: "browser_file",   // NOT "browser"
Action:   model.Create,     // it creates something that outlives the session
```

The name is a decision to record, not to improvise — pick it once and use it for any
future tool in this package that touches the filesystem, so `grep -rn browser_file`
lists every one of them.

**On MCP `roots`.** The protocol does define a way for a client to declare which
directories a server may operate in, and `tinywasm/mcp` does not implement it
(verified: no occurrence in the package). Adopting it is **not** recommended as part
of this work, for one reason: roots are *advisory*. They are a hint the server is
expected to honour, not a constraint the protocol enforces — a server that ignores
them still writes wherever it likes. It would document intent without adding safety.

Which means **the path validation below is the only real confinement**, and it should
be treated that way rather than as input hygiene.

## The tool

```
browser_save_screenshot
```

Arguments — a new `SaveScreenshotArgsModel` in `models.go`, with the struct generated
into `models_orm.go` by `ormc` (**do not hand-edit** — the file header says
`DO NOT EDIT`):

| field | type | meaning |
|---|---|---|
| `dir` | `Text`, NotNull | destination directory; created if missing |
| `name` | `Text`, NotNull | file name without extension; `.png` is appended |
| `selector` | `Text`, optional | capture just this element — the widget case |
| `fullpage` | `Bool`, optional | whole scrollable page; mutually exclusive with `selector` |
| `overwrite` | `Bool`, optional | default **false**: refuse to clobber |

Return the **absolute path written** and the pixel dimensions. A tool whose whole
purpose is to produce a file must say where the file is; returning "saved" makes the
caller guess.

`overwrite` defaults to false because the harness says access-shaped defaults deny
(principle 8) and because these files are checked into documentation — silently
replacing one is a change nobody asked for. Refusing costs one explicit argument.

### Reuse, do not re-implement

`CaptureScreenshot(fullpage)` and `CaptureElementScreenshot(selector)` already exist
in `screenshot_utils.go` and already return `ScreenshotResult{ImageData, Width,
Height, ...}`. This tool selects between them and writes bytes. **No new capture
path** — a second one would drift from the first.

### Path handling is the risk

`dir` comes from a client and reaches the filesystem. This is the one genuinely
dangerous argument in the package, and it needs the same explicit treatment
`permittedSelector` and `permittedURL` already get in `models.go`:

- Add a `permittedPath` whitelist. It must **not** include characters that make
  traversal or command construction expressible.
- Reject `..` segments outright after cleaning, not before.
- Resolve to an absolute path and report that.
- `name` is a file name, not a path: reject any separator in it.

Decide and record whether writes are confined to a configured root. Unconfined, this
tool writes anywhere the process can — acceptable for a local dev tool, but it is a
decision, not a default to arrive at by omission.

### DPR and reproducibility

An image captured under `mobile` emulation (DPR 1.0) and the same capture under `off`
(DPR 1.25) differ in pixel dimensions for the same CSS layout. Documentation images
must be reproducible, so the returned dimensions and the emulation mode belong in the
reply, and `docs/` should say that a capture intended for docs sets the mode first.

## Acceptance criteria (Part B)

1. `gotest` green; the new args model is **generated**, not hand-written.
1b. The tool declares a filesystem-specific `Resource` and `Action: model.Create`,
   and a test asserts it is **not** `"browser"` — otherwise the distinction silently
   regresses the first time someone copies a neighbouring tool as a template.
2. A test writes to a temp dir and asserts: the file exists, is a valid PNG, and the
   returned path is absolute and matches where it landed.
3. A test asserts `overwrite: false` on an existing file returns an error and leaves
   the original **byte-identical**.
4. A test asserts traversal (`dir: "../../etc"`, a separator inside `name`) is
   rejected — the security-relevant case, so it must be a test and not a review note.
5. A test asserts `selector` and `fullpage` together are rejected rather than one
   silently winning.
6. Live: capture a single widget by selector into a docs directory and confirm the
   image is that widget, cropped, not the page.
7. `README.md` documents the tool with one worked example: emulate a viewport,
   capture a component by selector, reference the file from a doc.

## Out of scope

- **Image post-processing** — cropping beyond the element box, padding, background
  substitution, format conversion. `pixelmatch` is vendored here for comparison, not
  editing.
- **A docs-generation pipeline** that walks a component catalogue and captures each
  one. That belongs in the repository that owns the catalogue; this is the primitive
  it would call.
- **Changing `browser_screenshot`.** It keeps returning an image block and touching
  the clipboard. The two tools stay separate on purpose.
