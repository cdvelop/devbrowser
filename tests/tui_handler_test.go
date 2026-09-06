package devbrowser_test

import (
	"fmt"
	"webtyp.com/devbrowser"
	"testing"
)

type mockUI struct {
	refreshed bool
}

func (m *mockUI) RefreshUI() {
	m.refreshed = true
}

func (m *mockUI) ReturnFocus() error {
	return nil
}

func TestTUI_HandlerSelection(t *testing.T) {
	var loggedMsg string
	b := &devbrowser.DevBrowser{
		AutoStart: true,
		Log: func(msg ...any) {
			loggedMsg = ""
			for _, m := range msg {
				loggedMsg += fmt.Sprint(m)
			}
		},
		UI: &mockUI{},
		DB: &mockStore{data: make(map[string]string)},
	}

	// 1. Label
	if b.Label() != devbrowser.LabelBrowserAutoRun {
		t.Errorf("expected Label '%s', got %q", devbrowser.LabelBrowserAutoRun, b.Label())
	}

	// 2. Options
	opts := b.Options()
	if len(opts) != 2 {
		t.Fatalf("expected 2 options, got %d", len(opts))
	}
	if _, ok := opts[0]["on"]; !ok {
		t.Errorf("expected first option to be 'on'")
	}
	if _, ok := opts[1]["off"]; !ok {
		t.Errorf("expected second option to be 'off'")
	}

	// 3. Initial Value
	if b.Value() != "on" {
		t.Errorf("expected initial value 'on', got %q", b.Value())
	}

	// 4. Change to off
	ui := b.UI.(*mockUI)
	ui.refreshed = false
	b.Change("off")
	if b.AutoStart {
		t.Error("expected AutoStart to be false")
	}
	if b.Value() != "off" {
		t.Errorf("expected value 'off', got %q", b.Value())
	}
	if !ui.refreshed {
		t.Error("expected UI refresh")
	}

	// 5. Change to on
	ui.refreshed = false
	b.Change("on")
	if !b.AutoStart {
		t.Error("expected AutoStart to be true")
	}
	if b.Value() != "on" {
		t.Errorf("expected value 'on', got %q", b.Value())
	}

	// 6. Unknown key
	loggedMsg = ""
	b.Change("bogus")
	if loggedMsg == "" {
		t.Error("expected error log for unknown key")
	}
	// State should not change
	if !b.AutoStart {
		t.Error("AutoStart changed on unknown key")
	}

	// 7. StatusMessage
	msg := b.StatusMessage()
	// Format: "Closed | Auto-Start: on | Shortcut B" (Closed by default in struct)
	expected := "Closed | Auto-Start: on | Shortcut B"
	if msg != expected {
		t.Errorf("expected status message %q, got %q", expected, msg)
	}
}
