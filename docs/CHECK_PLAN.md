---
PLAN: "fix: hide Chrome automated-test infobar on headed launch"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — Hide Chrome "controlled by automated test software" infobar

## Goal

When `devbrowser` launches Chrome in headed mode, Chrome shows a top infobar:

> Chrome is being controlled by automated test software

That bar steals vertical space and distracts during local development (especially with DevTools + device toolbar open). Remove it with the smallest correct change.

## Diagnosis (why it appears)

1. `CreateBrowserContext` in `context.go` builds allocator options from `chromedp.DefaultExecAllocatorOptions`.
2. That default slice **includes** `Flag("enable-automation", true)` in vendored `chromedp/allocate.go` (around the `DefaultExecAllocatorOptions` block).
3. Chrome shows the infobar **if and only if** the process was started with `--enable-automation`.
4. CDP control does **not** require that flag. chromedp already forces `--remote-debugging-port=0` in `ExecAllocator.Allocate`. Disabling `enable-automation` keeps automation working; it only hides the banner and avoids setting the automation-oriented UI chrome.

### How chromedp flag override works (critical)

`chromedp.Flag` writes into a `map[string]interface{}` (`initFlags`). Later options with the same name **overwrite** earlier ones:

```go
// chromedp/allocate.go — Flag
a.initFlags[name] = value
```

When building argv, a **bool false** means the switch is **omitted** (not passed as `--enable-automation=false`):

```go
case bool:
    if value {
        args = append(args, fmt.Sprintf("--%s", name))
    }
```

Therefore appending `chromedp.Flag("enable-automation", false)` after `DefaultExecAllocatorOptions` is enough: the default `true` is replaced, and Chrome never receives `--enable-automation`.

## Rejected alternatives

| Approach | Why not |
|---|---|
| `--disable-infobars` | Deprecated; Chrome ignores it for this banner. |
| Selenium-style `excludeSwitches: ["enable-automation"]` | Prefs/capabilities path; not how chromedp builds argv. Overriding the flag is the native equivalent. |
| Patch vendored `chromedp.DefaultExecAllocatorOptions` | Unnecessarily invasive; harder to re-vendor; default is correct for generic automation, wrong only for our headed dev UX. |
| New `DevBrowser` field / store key / `With*` option | YAGNI. This library is a **dev** browser controller; the infobar is never useful here. Always-off is the product default. Revisit only if a consumer needs the banner for demos. |
| `disable-blink-features=AutomationControlled` | Targets `navigator.webdriver` anti-bot signals, **not** the infobar. Also: a second `Flag("disable-blink-features", ...)` would **replace** the existing `WebFontsInterventionV2` value (map overwrite), so any merge must be a single comma-separated string. Out of scope. |

## Decision

**One flag override in `context.go`.** No new public API, no config store key, no chromedp fork edit.

## Development Rules

- This repo is **backend tooling** (launches a host Chrome process). stdlib is legitimate — do NOT strip stdlib imports.
- Flat package layout at module root for library code; do not invent subpackages for this change.
- No hardcoded repeated strings in new logic — if a flag name is referenced from more than one place, use a named constant. For a single-site override next to sibling `chromedp.Flag(...)` calls, an inline flag name matching the existing style in `context.go` is acceptable and preferred for consistency with neighboring lines.
- Do not modify vendored `chromedp/` or `cdproto/` for this feature.
- Do not add comments unless they document a non-obvious Chrome/chromedp contract (the ozone-platform comment style in `context.go` is the bar).
- English only in code and docs touched by this plan.
- Do not run `gopush` or `codejob`.

## Scope

### In scope

- Override `enable-automation` to `false` in `CreateBrowserContext` (`context.go`).
- Brief README note under the headed/headless section so consumers know the infobar is suppressed on purpose.

### Out of scope

- Changing headless defaults, window geometry, DevTools auto-open, cache flags.
- Anti-detection / `navigator.webdriver` spoofing.
- Public toggle to re-enable the infobar.
- Edits under `chromedp/` or `cdproto/`.

## Stage 1 — Flag override

**File:** `context.go`  
**Function:** `(*DevBrowser) CreateBrowserContext`

In the `opts := append(chromedp.DefaultExecAllocatorOptions[:], ...)` literal, add **after** the copied defaults (order among the appended flags does not matter for distinct keys; what matters is that this key is set in the append list so it overwrites the default map entry):

```go
chromedp.Flag("enable-automation", false),
```

Place it next to the other Chrome UX flags (near `headless` / `disable-blink-features`), with a short comment explaining **why** (default chromedp enables it; false omits the switch and hides the headed infobar; CDP still works via remote-debugging-port). Comment tone should match the existing `ozone-platform` comment block.

Do **not**:

- remove `DefaultExecAllocatorOptions` usage
- set the flag only when `!h.Headless` (always false is simpler and harmless in headless)
- touch `disable-blink-features` in this stage

### Acceptance

- `grep -n 'enable-automation' context.go` shows exactly one line setting the flag to `false`.
- `grep -n 'enable-automation' chromedp/allocate.go` still shows the upstream default `true` (untouched).
- `go build ./...` succeeds.

## Stage 2 — README

**File:** `README.md`

In the `SetHeadless` / headed-mode notes section, add one short bullet:

- Headed launches suppress Chrome's "controlled by automated test software" infobar by overriding chromedp's default `--enable-automation` (CDP automation is unchanged).

Do not expand into a design essay. Do not link to `PLAN.md`.

### Acceptance

- README mentions the infobar suppression in the headed/headless area.
- No new permanent doc files required (`ARCHITECTURE.md` / `DESIGN.md` optional later only if more automation fingerprint work lands).

## Stage 3 — Verification

Automated UI assertion of the infobar is brittle (Chrome chrome UI, not page DOM). Prefer:

1. **Compile/tests (required):**
   ```bash
   go test ./...
   ```
   Existing headless tests must keep passing (flag omission must not break allocator/CDP).

2. **Manual headed smoke (required for the human closer; executor notes it in the PR):**
   - Build/run the usual consumer path that opens a headed window (`Headless=false`).
   - Confirm the top gray bar text is **absent**.
   - Confirm DevTools / navigation / one MCP or Reload path still works (proves CDP alive).

Optional lightweight regression guard (only if easy and consistent with repo test style): a test that does **not** launch Chrome but documents intent is unnecessary if it requires exporting internals. Prefer no new test over a weak one. Do **not** add a flaky headed integration test solely for the banner.

### Acceptance

- `go test ./...` passes.
- PR description includes the manual headed check result (pass/fail + Chrome version if known).

## Stages checklist

| # | Stage | Status |
|---|---|---|
| 1 | Override `enable-automation` in `context.go` | done |
| 2 | README headed-mode note | done |
| 3 | `go test ./...` + manual headed smoke note in PR | done |

## Reference — current launch options (do not rewrite wholesale)

`context.go` today (conceptually):

```go
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.Flag("headless", h.Headless),
    chromedp.Flag("disable-blink-features", "WebFontsInterventionV2"),
    chromedp.Flag("use-fake-ui-for-media-stream", true),
    chromedp.Flag("ozone-platform", "x11"),
    chromedp.Flag("window-position", h.Position),
    chromedp.WindowSize(h.Width, h.Height),
)
// + conditional auto-open-devtools, disable-cache, ExecPath
```

After Stage 1, the append list must also contain:

```go
chromedp.Flag("enable-automation", false),
```

## Risk notes

- **Low risk:** omitting `--enable-automation` is a well-known headed-Chrome practice; remote debugging remains the control channel.
- **Do not** "fix" by deleting `enable-automation` from vendored defaults — keep the override local to `devbrowser`.
- If a future Chrome version reintroduces a similar banner under another switch, revisit with a new plan; do not pile experimental flags preemptively.
)
