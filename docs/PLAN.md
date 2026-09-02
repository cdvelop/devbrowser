---
PLAN: "fix: idempotent CloseBrowser and stale Chrome profile cleanup"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 13852590319459618857
PR: https://github.com/tinywasm/devbrowser/pull/13
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — stop leaking Chrome profile directories

## 1. The defect

The vendored allocator mints one throwaway Chrome profile per browser
([chromedp/allocate.go:149-155](../chromedp/allocate.go)):

```go
dataDir, ok := a.initFlags["user-data-dir"].(string)
if !ok {
	tempDir, err := os.MkdirTemp(allocTempDir, "chromedp-runner")
	...
	args = append(args, "--user-data-dir="+tempDir)
}
```

and removes it only when the allocator context is cancelled (`os.RemoveAll(dataDir)`,
same file). If the process that owns the allocator dies without cancelling — killed,
blocked, or simply exiting without calling `CloseBrowser` — the directory stays forever.

Measured on the maintainer's machine after ~22 hours of normal development:

```
$ du -shc /tmp/chromedp-runner* | tail -1
1.9G    total          (157 directories)
```

Two independent causes, both fixed here:

1. **`CloseBrowser` refuses to be called defensively.** It returns an error when the
   browser is already closed ([CloseBrowser.go:10-12](../CloseBrowser.go)), so a shutdown
   path that just wants to guarantee "the browser is closed" has to guess whether calling
   it is legal. Consumers end up not calling it at all.
2. **Nothing ever collects the directories left by earlier runs.** A crash or a `SIGKILL`
   will always leak one; without a sweeper the leak is unbounded.

## 2. Repo constraints (read before writing code)

- **This repo is backend tooling and legitimately uses the Go standard library.**
  `devbrowser.go` already imports `errors` and `time`; the vendored `chromedp` tree uses
  `os`, `os/exec`, `syscall`. Do **NOT** replace stdlib imports with `tinywasm/*`
  equivalents — that rule applies to WASM-compiled packages, and this is not one.
- **Do not modify anything under `chromedp/`, `cdproto/`, `httphead/`, `intern/`, `ws/`,
  `pool/`, `pixelmatch/`, `humanize/`, `sysutil/`, `win/` or `screenresolution/`.** Those
  are vendored trees. All new code goes in the repo root package `devbrowser`.
- **Injection, never `init()`.** The sweeper is exposed as an API the consumer calls. Do
  not register it in an `init()`, do not call it automatically from `New` or
  `OpenBrowser`.
- Every repeated string is a named constant.

## 3. Stage 1 — failing tests first (TDD)

New file `tests/profiles_test.go` (package `devbrowser_test`), driving a `ProfileCleaner`
whose root and liveness check are injected, so no test starts a browser:

| Test | Setup | Asserts |
|---|---|---|
| `TestCleanStale_RemovesDirWithoutLock` | root contains `chromedp-runner1/` with an old modtime, no `SingletonLock` | dir removed, `removed == 1` |
| `TestCleanStale_KeepsDirWithLiveLock` | `chromedp-runner2/SingletonLock -> host-4242`, `ProcessAlive` returns true for 4242 | dir still exists, `removed == 0` |
| `TestCleanStale_RemovesDirWithDeadLock` | same, `ProcessAlive` returns false | dir removed |
| `TestCleanStale_KeepsRecentDirWithoutLock` | no lock, modtime `Now()` | dir kept (inside the grace window: a browser that is still starting has not written its lock yet) |
| `TestCleanStale_IgnoresUnrelatedEntries` | root also contains `something-else/` and a regular file `chromedp-runner-notadir` | both untouched |
| `TestCleanStale_ReportsFreedBytes` | one stale dir holding a 1 KiB file | `freed >= 1024` |
| `TestCleanStale_MissingRootIsNotAnError` | root does not exist | `removed == 0`, `err == nil` |

Update the existing `TestCloseBrowserWhenClosed` in
[tests/devbrowser_test.go](../tests/devbrowser_test.go). It currently asserts the opposite
of the behaviour this plan installs:

```go
	if err := db.CloseBrowser(); err == nil {
		t.Fatal("expected CloseBrowser to return error when already closed")
	}
```

It becomes, and the test is renamed `TestCloseBrowserIsIdempotent`:

```go
	if err := db.CloseBrowser(); err != nil {
		t.Fatalf("CloseBrowser on a closed browser must be a no-op, got %v", err)
	}
	if err := db.CloseBrowser(); err != nil {
		t.Fatalf("second CloseBrowser must also be a no-op, got %v", err)
	}
```

## 4. Stage 2 — `CloseBrowser` becomes idempotent

In [CloseBrowser.go](../CloseBrowser.go), replace:

```go
	if !h.IsOpenFlag {
		return errors.New("DevBrowser is already closed")
	}
```

with an early `return nil`. Drop the `errors` import from that file if it becomes unused.

Callers that relied on the error: [OpenBrowser.go:45](../OpenBrowser.go) and
[OpenBrowser.go:126](../OpenBrowser.go) already ignore the return value;
[tui.go:58](../tui.go) logs it. Verify with `grep -rn "CloseBrowser()" --include=*.go .`
that no caller branches on the error being non-nil after the change.

## 5. Stage 3 — `profiles.go` (new file, root package)

```go
package devbrowser

// ProfilePrefix is the name prefix the vendored allocator gives every throwaway
// Chrome profile directory it creates.
const ProfilePrefix = "chromedp-runner"

// singletonLockName is the symlink Chrome maintains in a profile root while it
// is running. Its target is "<hostname>-<pid>".
const singletonLockName = "SingletonLock"

// DefaultStaleGrace is how long a profile directory with no lock is left alone,
// so a browser that is still starting up is never swept from under itself.
const DefaultStaleGrace = 5 * time.Minute

// ProfileCleaner removes Chrome profile directories left behind by browsers
// whose owning process is gone. Zero value is not usable; build it with
// NewProfileCleaner and override the fields in tests.
type ProfileCleaner struct {
	Root         string                 // where profiles live; default os.TempDir()
	Grace        time.Duration          // default DefaultStaleGrace
	Now          func() time.Time       // default time.Now
	ProcessAlive func(pid int) bool     // default processAlive
}

func NewProfileCleaner() *ProfileCleaner

// CleanStale removes every stale profile directory under Root and reports how
// many went and how many bytes that freed. A directory is stale when it has no
// SingletonLock and is older than Grace, or when the pid in its SingletonLock
// is not a live process. Errors on individual directories are skipped, not
// returned: a sweep must never fail a caller's startup.
func (c *ProfileCleaner) CleanStale() (removed int, freed int64, err error)
```

Implementation notes the executor must follow:

- Only consider entries that `IsDir()` **and** whose name starts with `ProfilePrefix`.
- Liveness: `os.Readlink(filepath.Join(dir, singletonLockName))`. On success the target is
  `"<hostname>-<pid>"`; take everything after the **last** `-` and `strconv.Atoi` it
  (hostnames contain `-`, as in `cs-laptop-500068`). A parse failure means "cannot tell" →
  treat the directory as **in use** and skip it; never delete on a value you could not read.
- On `os.Readlink` failure (no lock, or a platform without the symlink), fall back to the
  age rule: stale only when `c.Now().Sub(info.ModTime()) > c.Grace`.
- `freed` is computed by walking the directory before removing it; a walk error contributes 0.

Liveness helper, split by platform so Windows still builds:

`process_unix.go` (`//go:build !windows`):

```go
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
```

`process_windows.go` (`//go:build windows`): `os.FindProcess` returns an error for a dead
pid on Windows, so `err == nil` is the answer.

## 6. Acceptance criteria

- `gotest ./...` green.
- `grep -rn "already closed" --include=*.go .` → empty.
- `grep -rn "chromedp-runner" --include=*.go . | grep -v "^./chromedp/"` → only
  `profiles.go` (the constant) and the tests.
- No file under `chromedp/`, `cdproto/` or any other vendored tree is modified:
  `git diff --name-only` lists only root-package files, `docs/`, and `tests/`.
- `README.md` gains a short section: what `ProfileCleaner` is for, that the consumer calls
  `CleanStale()` at startup, and that `CloseBrowser` is safe to call unconditionally.

## 7. Stages

| # | Stage | Files |
|---|-------|-------|
| 1 | Failing tests | `tests/profiles_test.go` (new), `tests/devbrowser_test.go` |
| 2 | Idempotent close | `CloseBrowser.go` |
| 3 | Sweeper | `profiles.go`, `process_unix.go`, `process_windows.go` (all new) |
| 4 | Docs | `README.md` |
