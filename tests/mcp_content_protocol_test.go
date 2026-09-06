package devbrowser_test

import (
	"fmt"
	"strings"
	"testing"

	"webtyp.com/context"
	"webtyp.com/mcp"
)

// helpers

func findTool(tools []mcp.Tool, name string) *mcp.Tool {
	for i := range tools {
		if tools[i].Name == name {
			return &tools[i]
		}
	}
	return nil
}

func emptyReq(name string) mcp.Request {
	return mcp.Request{
		Params: mcp.CallToolParams{Name: name, Arguments: "{}"},
		Action: 'r',
	}
}

func contentIsArray(result *mcp.Result) bool {
	return strings.HasPrefix(strings.TrimSpace(result.Content), "[")
}

// Bug C: mcp.Text() devuelve Content como objeto en lugar de array.
// Estos tests fallan con mcp < fix y pasan automáticamente al actualizar la dependencia.

// TestMCPContent_GetErrors_Empty_IsArray verifica que browser_get_errors devuelva content como array
// incluso en el caso vacío "No JavaScript errors captured" — sin necesitar browser con contexto real.
// Este test cubre el path más simple de mcp.Text() en devbrowser.
func TestMCPContent_GetErrors_Empty_IsArray(t *testing.T) {
	db, _ := DefaultTestBrowser()
	db.IsOpenFlag = true
	tool := findTool(db.GetErrorTools(), "browser_get_errors")
	if tool == nil {
		t.Fatal("browser_get_errors tool not found")
	}

	var ctx context.Context
	result, err := tool.Execute(&ctx, emptyReq("browser_get_errors"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if !contentIsArray(result) {
		t.Fatalf("Bug C: content is not a JSON array — mcp.Text() fix not applied.\nContent: %s", result.Content)
	}
}

// TestMCPContent_GetNetworkLogs_IsArray verifica que browser_get_network_logs devuelva content como array.
// Con IsOpenFlag=true y sin logs reales, la tool devuelve un mensaje vacío via mcp.Text().
func TestMCPContent_GetNetworkLogs_IsArray(t *testing.T) {
	db, _ := DefaultTestBrowser()
	db.IsOpenFlag = true
	tool := findTool(db.GetNetworkTools(), "browser_get_network_logs")
	if tool == nil {
		t.Fatal("browser_get_network_logs tool not found")
	}

	var ctx context.Context
	result, err := tool.Execute(&ctx, emptyReq("browser_get_network_logs"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if !contentIsArray(result) {
		t.Fatalf("Bug C: content is not a JSON array — mcp.Text() fix not applied.\nContent: %s", result.Content)
	}
}

// --- Bug TUI pollution: b.Logger duplica el resultado de tools MCP hacia la TUI ---
// Estos tests fallan antes del fix y pasan una vez eliminados los b.Logger duplicados.

// logCapture retorna un logger que acumula mensajes y una función para leer lo acumulado.
func logCapture() (func(...any), func() []string) {
	var logs []string
	return func(msgs ...any) {
		for _, m := range msgs {
			logs = append(logs, fmt.Sprint(m))
		}
	}, func() []string { return logs }
}

// resultText extrae el texto del primer item de content.
func resultText(r *mcp.Result) string {
	text, _ := mcp.GetText(r)
	return text
}

// TestMCPTool_EmulateDevice_NoTUIPollution verifica que browser_emulate_device
// no envíe el resultado al Logger (TUI). Solo debe ir al cliente MCP via return.
// No requiere browser real — funciona con IsOpenFlag=false.
func TestMCPTool_EmulateDevice_NoTUIPollution(t *testing.T) {
	logger, getLogs := logCapture()
	db, _ := DefaultTestBrowser(logger)

	tool := findTool(db.GetManagementTools(), "browser_emulate_device")
	if tool == nil {
		t.Fatal("browser_emulate_device tool not found")
	}

	var ctx context.Context
	req := mcp.Request{
		Params: mcp.CallToolParams{Name: "browser_emulate_device", Arguments: `{"mode":"mobile"}`},
		Action: 'u',
	}
	result, err := tool.Execute(&ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(result)
	for _, entry := range getLogs() {
		if entry == text {
			t.Fatalf("Bug: tool result leaked to TUI Logger.\nLogger received: %q\nExpected: only in mcp.Text() return", entry)
		}
	}
}

// TestMCPTool_GetErrors_NoTUIPollution verifica que browser_get_errors
// no envíe su resultado al Logger (TUI).
func TestMCPTool_GetErrors_NoTUIPollution(t *testing.T) {
	logger, getLogs := logCapture()
	db, _ := DefaultTestBrowser(logger)
	db.IsOpenFlag = true

	tool := findTool(db.GetErrorTools(), "browser_get_errors")
	if tool == nil {
		t.Fatal("browser_get_errors tool not found")
	}

	var ctx context.Context
	result, err := tool.Execute(&ctx, emptyReq("browser_get_errors"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(result)
	for _, entry := range getLogs() {
		if entry == text {
			t.Fatalf("Bug: tool result leaked to TUI Logger.\nLogger received: %q", entry)
		}
	}
}

// TestMCPTool_GetNetworkLogs_NoTUIPollution verifica que browser_get_network_logs
// no envíe su resultado al Logger (TUI).
func TestMCPTool_GetNetworkLogs_NoTUIPollution(t *testing.T) {
	logger, getLogs := logCapture()
	db, _ := DefaultTestBrowser(logger)
	db.IsOpenFlag = true

	tool := findTool(db.GetNetworkTools(), "browser_get_network_logs")
	if tool == nil {
		t.Fatal("browser_get_network_logs tool not found")
	}

	var ctx context.Context
	result, err := tool.Execute(&ctx, emptyReq("browser_get_network_logs"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(result)
	for _, entry := range getLogs() {
		if entry == text {
			t.Fatalf("Bug: tool result leaked to TUI Logger.\nLogger received: %q", entry)
		}
	}
}
