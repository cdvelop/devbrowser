package devbrowser_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tinywasm/devbrowser"
)

func TestCleanStale_RemovesDirWithoutLock(t *testing.T) {
	tempDir := t.TempDir()
	runnerDir := filepath.Join(tempDir, "chromedp-runner1")
	if err := os.Mkdir(runnerDir, 0755); err != nil {
		t.Fatal(err)
	}

	oldTime := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(runnerDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	cleaner := devbrowser.NewProfileCleaner()
	cleaner.Root = tempDir
	cleaner.Grace = 5 * time.Minute

	removed, _, err := cleaner.CleanStale()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed dir, got %d", removed)
	}
	if _, err := os.Stat(runnerDir); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed", runnerDir)
	}
}

func TestCleanStale_KeepsDirWithLiveLock(t *testing.T) {
	tempDir := t.TempDir()
	runnerDir := filepath.Join(tempDir, "chromedp-runner2")
	if err := os.Mkdir(runnerDir, 0755); err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(runnerDir, "SingletonLock")
	if err := os.Symlink("host-4242", lockPath); err != nil {
		t.Skipf("symlink creation failed (might be Windows without privileges): %v", err)
	}

	cleaner := devbrowser.NewProfileCleaner()
	cleaner.Root = tempDir
	cleaner.ProcessAlive = func(pid int) bool {
		return pid == 4242
	}

	removed, _, err := cleaner.CleanStale()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed dirs, got %d", removed)
	}
	if _, err := os.Stat(runnerDir); err != nil {
		t.Fatalf("expected %s to still exist, got err %v", runnerDir, err)
	}
}

func TestCleanStale_RemovesDirWithDeadLock(t *testing.T) {
	tempDir := t.TempDir()
	runnerDir := filepath.Join(tempDir, "chromedp-runner3")
	if err := os.Mkdir(runnerDir, 0755); err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(runnerDir, "SingletonLock")
	if err := os.Symlink("host-4242", lockPath); err != nil {
		t.Skipf("symlink creation failed: %v", err)
	}

	cleaner := devbrowser.NewProfileCleaner()
	cleaner.Root = tempDir
	cleaner.ProcessAlive = func(pid int) bool {
		return false
	}

	removed, _, err := cleaner.CleanStale()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed dir, got %d", removed)
	}
	if _, err := os.Stat(runnerDir); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed", runnerDir)
	}
}

func TestCleanStale_KeepsRecentDirWithoutLock(t *testing.T) {
	tempDir := t.TempDir()
	runnerDir := filepath.Join(tempDir, "chromedp-runner4")
	if err := os.Mkdir(runnerDir, 0755); err != nil {
		t.Fatal(err)
	}

	cleaner := devbrowser.NewProfileCleaner()
	cleaner.Root = tempDir
	cleaner.Grace = 5 * time.Minute
	cleaner.Now = time.Now

	removed, _, err := cleaner.CleanStale()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed dirs, got %d", removed)
	}
	if _, err := os.Stat(runnerDir); err != nil {
		t.Fatalf("expected %s to exist, got err %v", runnerDir, err)
	}
}

func TestCleanStale_IgnoresUnrelatedEntries(t *testing.T) {
	tempDir := t.TempDir()

	unrelatedDir := filepath.Join(tempDir, "something-else")
	if err := os.Mkdir(unrelatedDir, 0755); err != nil {
		t.Fatal(err)
	}

	regularFile := filepath.Join(tempDir, "chromedp-runner-notadir")
	if err := os.WriteFile(regularFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	cleaner := devbrowser.NewProfileCleaner()
	cleaner.Root = tempDir
	cleaner.Grace = 0

	removed, _, err := cleaner.CleanStale()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed dirs, got %d", removed)
	}
	if _, err := os.Stat(unrelatedDir); err != nil {
		t.Fatalf("expected %s to exist", unrelatedDir)
	}
	if _, err := os.Stat(regularFile); err != nil {
		t.Fatalf("expected %s to exist", regularFile)
	}
}

func TestCleanStale_ReportsFreedBytes(t *testing.T) {
	tempDir := t.TempDir()
	runnerDir := filepath.Join(tempDir, "chromedp-runner5")
	if err := os.Mkdir(runnerDir, 0755); err != nil {
		t.Fatal(err)
	}

	payload := make([]byte, 1024)
	if err := os.WriteFile(filepath.Join(runnerDir, "data.bin"), payload, 0644); err != nil {
		t.Fatal(err)
	}

	oldTime := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(runnerDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	cleaner := devbrowser.NewProfileCleaner()
	cleaner.Root = tempDir
	cleaner.Grace = 5 * time.Minute

	removed, freed, err := cleaner.CleanStale()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed dir, got %d", removed)
	}
	if freed < 1024 {
		t.Fatalf("expected freed >= 1024, got %d", freed)
	}
}

func TestCleanStale_MissingRootIsNotAnError(t *testing.T) {
	cleaner := devbrowser.NewProfileCleaner()
	cleaner.Root = filepath.Join(t.TempDir(), "nonexistent")

	removed, freed, err := cleaner.CleanStale()
	if err != nil {
		t.Fatalf("expected no error for missing root, got %v", err)
	}
	if removed != 0 || freed != 0 {
		t.Fatalf("expected 0 removed and 0 freed, got removed=%d, freed=%d", removed, freed)
	}
}

func TestCleanStale_ZeroGraceFallsBackToDefault(t *testing.T) {
	tempDir := t.TempDir()
	runnerDir := filepath.Join(tempDir, "chromedp-runner7")
	if err := os.Mkdir(runnerDir, 0755); err != nil {
		t.Fatal(err)
	}

	// A cleaner built as a struct literal leaves Grace at zero. A brand-new
	// directory with no lock yet is a browser that is still starting: it must
	// survive the sweep.
	cleaner := &devbrowser.ProfileCleaner{Root: tempDir}

	removed, _, err := cleaner.CleanStale()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed dirs with a zero Grace, got %d", removed)
	}
	if _, err := os.Stat(runnerDir); err != nil {
		t.Fatalf("expected %s to exist, got err %v", runnerDir, err)
	}
}
