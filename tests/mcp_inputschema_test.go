package devbrowser_test

import (
	"strings"
	"testing"

	"webtyp.com/context"
	"webtyp.com/json"
	"webtyp.com/mcp"
	"webtyp.com/model"
)

// encodeMCPMessage serializes an mcp.JSONRPCMessage to its wire JSON using
// webtyp/json (never stdlib), exactly as the transport emits it.
func encodeMCPMessage(resp mcp.JSONRPCMessage) string {
	var b []byte
	if f, ok := resp.(model.Encodable); ok {
		_ = json.Encode(f, &b)
	}
	return string(b)
}

// toolsListBody runs tools/list against a server holding all devbrowser tools and
// returns the raw wire JSON of the response.
func toolsListBody(t *testing.T) (string, int) {
	t.Helper()
	db, _ := DefaultTestBrowser()
	defer db.CloseBrowser()

	srv, err := mcp.NewServer(mcp.Config{Name: "test", Version: "1.0.0", Authorize: mcp.AllowAll}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	tools := db.GetMCPTools()
	for _, tool := range tools {
		if e := srv.AddTool(tool); e != nil {
			t.Fatalf("AddTool %s: %v", tool.Name, e)
		}
	}

	var ctx context.Context
	ctx.Set(mcp.CtxKeyUserID, "u1")
	resp := srv.HandleMessage(&ctx, []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/list","params":{}}`))
	if resp == nil {
		t.Fatal("tools/list devolvió nil")
	}
	return encodeMCPMessage(resp), len(tools)
}

// TestMCP_ToolsList_InputSchemaIsValidJSONSchema garantiza que la respuesta
// tools/list exponga, para CADA tool de devbrowser, un inputSchema que sea un
// JSON Schema "object" VÁLIDO.
//
// Contrato del estándar MCP: inputSchema DEBE ser un objeto JSON Schema con
// "type":"object". Clientes como Claude Code validan tools/list con JSON Schema
// (Zod) y descartan el ARRAY COMPLETO de tools si uno solo es inválido — dejando
// al servidor "Connected" pero SIN ninguna tool visible.
//
// Este test FALLA mientras el inputSchema se genere mal (p.ej. serializando el
// struct: {"fullpage":false}, o `null`) y PASA cuando la responsabilidad de
// generar el inputSchema desde Schema() []model.Field viva en webtyp/mcp.
// NO debe existir lógica de JSON Schema en devbrowser: models.go solo declara los
// campos; ormc genera Schema(); mcp genera el inputSchema del protocolo.
func TestMCP_ToolsList_InputSchemaIsValidJSONSchema(t *testing.T) {
	body, toolCount := toolsListBody(t)

	// Ninguna tool con inputSchema null.
	if strings.Contains(body, `"inputSchema":null`) {
		t.Errorf("hay tools con inputSchema:null (inválido)\nbody: %s", body)
	}

	// Cada tool debe tener un inputSchema que empiece por {"type":"object".
	got := strings.Count(body, `"inputSchema":{"type":"object"`)
	if got != toolCount {
		t.Errorf("inputSchema \"object\" válidos = %d, se esperaban %d (uno por tool)\nbody: %s",
			got, toolCount, body)
	}

	// No debe filtrarse ningún struct serializado como schema (síntoma del bug).
	for _, leak := range []string{
		`"inputSchema":{"fullpage"`,
		`"inputSchema":{"selector"`,
		`"inputSchema":{"url"`,
		`"inputSchema":{"reserved"`,
	} {
		if strings.Contains(body, leak) {
			t.Errorf("inputSchema es el struct serializado, no un JSON Schema: %s\nbody: %s", leak, body)
		}
	}
}

// TestMCP_ToolsList_ScreenshotSchema fija la SALIDA ESPERADA concreta para
// browser_screenshot: el campo `fullpage` como boolean dentro de properties.
// Documenta el formato exacto que mcp debe generar desde ScreenshotArgs.Schema()
// (que ormc genera como {Name:"fullpage", Type: model.FieldBool}).
func TestMCP_ToolsList_ScreenshotSchema(t *testing.T) {
	body, _ := toolsListBody(t)

	if !strings.Contains(body, `"fullpage":{"type":"boolean"}`) {
		t.Errorf("browser_screenshot: se esperaba properties.fullpage como boolean\nbody: %s", body)
	}
	// El struct crudo NO debe aparecer.
	if strings.Contains(body, `"fullpage":false`) {
		t.Errorf("browser_screenshot: inputSchema serializa el struct (\"fullpage\":false)\nbody: %s", body)
	}
}
