---
PLAN: "fix: la ventana física crece para caber el viewport emulado + DevTools"
EXECUTOR: jules
REVIEWER: none
---

> Plan autocontenido. Todo lo necesario para ejecutarlo está en este documento.
> Se despacha con el flujo CodeJob. Ver skill: agents-workflow.

# Plan — la ventana física crece para caber lo que el agente MCP está probando

## 1. El defecto

`browser_emulate_device` (`mcp-management.go:23-24`) documenta su propio
comportamiento así:

> *"Emulate a mobile device or tablet viewport **without resizing the
> physical window**."*

Eso es exacto y es justamente el problema. `chromedp.Emulate` /
`chromedp.EmulateViewport` sólo llaman a `Emulation.setDeviceMetricsOverride`
(`chromedp/emulate.go`): cambian lo que la página **cree** que mide su
viewport, pero la ventana física de Chrome —la que abrió `CreateBrowserContext`
con `chromedp.WindowSize(h.Width, h.Height)` (`context.go:27`) y la que el
humano reposiciona/redimensiona a mano (persistida por
`monitor_geometry.go:checkAndSaveGeometry` cada 2s)— se queda exactamente
donde estaba.

Consecuencia real, reportada por el usuario:

1. El humano deja la ventana en el tamaño/posición que le conviene (se guarda
   en `StoreKeyBrowserPosition`/`StoreKeyBrowserSize`, `config.go:10-11`).
2. Un agente, vía MCP, llama `browser_emulate_device` pidiendo un viewport que
   no cabe en esa ventana (p.ej. `mode: "desktop"` → 1440×900 dentro de una
   ventana de 800×600). El contenido se sale del espacio visible.
3. Humano y agente **ven cosas distintas**: el agente razona sobre un DOM
   renderizado a 1440px de ancho; el humano ve una ventana de 800px con el
   contenido recortado.
4. El humano tiene que redimensionar la ventana a mano para volver a ver algo
   coherente — y ese redimensionado manual dispara `monitorBrowserGeometry`,
   que lo persiste, pero no hay nada que avise al agente de que el tamaño
   cambió.

Un segundo factor agrava el punto 2: `context.go:31-33` abre DevTools
automáticamente cuando la ventana es ancha:

```go
// Conditionally add devtools flag
if h.Width > 1200 {
	opts = append(opts, chromedp.Flag("auto-open-devtools-for-tabs", true))
}
```

DevTools se aloja **dentro de la misma ventana física**, acoplado
(típicamente a la derecha cuando la ventana es ancha, que es justo el caso
que dispara esta condición). Le resta ancho útil al contenido, y ese ancho
**no se descuenta en ningún cálculo existente**: ni en
`CalculateConstrainedSize` (`geometry_utils.go`), ni en `GetPresetSize`, ni en
`applyDeviceEmulation`. El resultado que describe el usuario —el contenido
"queda detrás" de DevTools— es exactamente esto: se le pide al viewport un
ancho que no considera que una porción de la ventana ya está ocupada.

## 2. Lo que CDP puede y no puede resolver aquí

`Browser.setWindowBounds` (vendorizado en `cdproto/browser/browser.go:500-523`,
ya usado en modo lectura por `checkAndSaveGeometry` vía
`Browser.getWindowForTarget`) permite **redimensionar la ventana en vivo**, sin
reiniciar el navegador ni el contexto de chromedp. Es la pieza que falta.

Lo que CDP **no** expone es el tamaño real del panel de DevTools acoplado —no
hay ningún comando en `cdproto/` que lo reporte. No se puede leer su ancho
exacto; sólo se puede reservarle un margen conservador y documentarlo como
estimación. Ver §7.

## 3. Decisión

Auto-fit de crecimiento único: cuando la ventana física no alcanza para
mostrar el viewport que se le pide (viewport + margen reservado de DevTools si
aplica), la ventana **crece en vivo** hasta caber, usando
`Browser.setWindowBounds` — nunca reinicia el navegador. **Nunca encoge sola**:
el tamaño que el humano dejó (manual o auto-crecido antes) es un piso, no un
techo. La posición (`Position`, `x,y`) no se toca — sólo el tamaño, igual que
ya hace `CalculateConstrainedSize` (clampa contra el monitor sin reposicionar).

Esto fue confirmado con el usuario antes de escribir este plan: la alternativa
de "nunca tocar la ventana física y en cambio reportar al agente un tamaño
efectivo escalado" fue descartada porque el objetivo explícito es que **humano
y agente vean lo mismo**, no una versión reducida/reportada.

## 4. Cambios

### 4.1 `devbrowser.go` — nuevo campo en `DevBrowser`

Añadir junto a los demás campos de estado de ventana (cerca de
`SizeConfigured`, línea ~31):

```go
// DevToolsReserved is true when auto-open-devtools-for-tabs was launched
// for this session (context.go decides this once, at CreateBrowserContext
// time, based on the window width at launch). Later window growth does not
// retroactively open or close DevTools, so this flag is a session-long
// snapshot, not a live query — CDP exposes no way to read DevTools' actual
// panel bounds.
DevToolsReserved bool
```

### 4.2 `context.go` — fijar el flag donde ya se decide

`CreateBrowserContext` ya decide si DevTools se abre (líneas 30-33). Cambiar
ese bloque para que también deje constancia en el struct:

```go
// Conditionally add devtools flag
h.DevToolsReserved = h.Width > 1200
if h.DevToolsReserved {
	opts = append(opts, chromedp.Flag("auto-open-devtools-for-tabs", true))
}
```

No se agrega ningún `b.Mu.Lock()` aquí: el resto de `CreateBrowserContext` ya
lee `h.Width`/`h.Height`/`h.Position` sin lock porque se ejecuta antes de que
existan lectores concurrentes (dentro de la goroutine de apertura, antes de
`h.ReadyChan <- true`). Mantener el mismo estilo.

### 4.3 `geometry_utils.go` — tamaño requerido, puro

Añadir, junto a `CalculateConstrainedSize`:

```go
// DevToolsReservedWidth is a conservative, documented ESTIMATE of the
// horizontal space Chrome's docked DevTools panel occupies when auto-opened
// via auto-open-devtools-for-tabs (see context.go). CDP exposes no command to
// read the panel's actual bounds, so this value is fixed and deliberately
// generous rather than pixel-exact — it assumes DevTools docks to the right,
// which only happens for the wide windows that trigger the auto-open in the
// first place (see §7 of docs/PLAN_WINDOW_AUTOFIT.md for the bottom-dock
// case this does not cover).
const DevToolsReservedWidth = 420

// RequiredWindowSize returns the physical window size needed to show a
// requested viewport of (reqW, reqH) without DevTools covering any of it,
// clamped to the detected monitor. It never returns less than the window's
// current size: growth is one-directional, the existing window size is a
// floor. If the monitor size has not been detected yet, it is detected now
// (mirrors the lazy-detect in GetPresetSize).
func (b *DevBrowser) RequiredWindowSize(reqW, reqH int) (int, int) {
	b.Mu.Lock()
	monW, monH := b.MonitorWidth, b.MonitorHeight
	b.Mu.Unlock()

	if monW == 0 || monH == 0 {
		b.DetectMonitorSize()
		b.Mu.Lock()
		monW, monH = b.MonitorWidth, b.MonitorHeight
		b.Mu.Unlock()
	}

	b.Mu.Lock()
	curW, curH := b.Width, b.Height
	reserved := b.DevToolsReserved
	b.Mu.Unlock()

	neededW := reqW
	if reserved {
		neededW += DevToolsReservedWidth
	}
	neededH := reqH

	if neededW < curW {
		neededW = curW
	}
	if neededH < curH {
		neededH = curH
	}

	return b.CalculateConstrainedSize(neededW, neededH, monW, monH)
}
```

### 4.4 `window_autofit.go` (archivo nuevo) — resize en vivo

Nuevo archivo, mismo patrón que `monitor_geometry.go` (que ya usa
`Browser.getWindowForTarget` para leer bounds):

```go
package devbrowser

import (
	"context"
	"fmt"

	"github.com/tinywasm/devbrowser/cdproto/browser"
	"github.com/tinywasm/devbrowser/chromedp"
)

// GrowWindowToFit resizes the live physical browser window, in place, so it
// is at least (reqW, reqH) plus any DevTools reservation — see
// RequiredWindowSize — and never shrinks it. It is the single place that
// keeps what a human sees in the window in sync with what an MCP-driven
// device emulation call renders, so both are looking at the same content
// instead of the emulated viewport overflowing a window sized for something
// else. Returns false, nil when no resize was necessary.
func (b *DevBrowser) GrowWindowToFit(reqW, reqH int) (bool, error) {
	b.Mu.Lock()
	ctx := b.Ctx
	curW, curH := b.Width, b.Height
	b.Mu.Unlock()

	if ctx == nil {
		return false, nil
	}

	newW, newH := b.RequiredWindowSize(reqW, reqH)
	if newW <= curW && newH <= curH {
		return false, nil
	}

	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			t := chromedp.FromContext(ctx).Target
			windowID, _, err := browser.GetWindowForTarget().WithTargetID(t.TargetID).Do(ctx)
			if err != nil {
				return err
			}
			bounds := &browser.Bounds{
				Width:  int64(newW),
				Height: int64(newH),
			}
			return browser.SetWindowBounds(windowID, bounds).Do(ctx)
		}),
	)
	if err != nil {
		return false, fmt.Errorf("GrowWindowToFit: %w", err)
	}

	b.Mu.Lock()
	b.Width = newW
	b.Height = newH
	b.SizeConfigured = true
	b.DB.Set(StoreKeyBrowserSize, fmt.Sprintf("%d,%d", newW, newH))
	b.Mu.Unlock()

	return true, nil
}
```

`bounds.Left`/`bounds.Top` se dejan en cero a propósito: el campo lleva
`json:"left,omitempty"` (`cdproto/browser/types.go:52-53`), así que se omiten
del request y `Browser.setWindowBounds` conserva la posición actual — sólo se
pide el resize. No tocar la posición está alineado con §3.

### 4.5 `mcp-management.go` — resolver de dimensiones + enganche

Añadir, junto a `resolveDevice` (después de la línea 195):

```go
// EmulationViewportSize returns the CSS pixel viewport size that mode/devName
// will render at once applied by applyDeviceEmulation, so the physical
// window can be grown to fit BEFORE the CDP emulation override is issued.
// Keep this in sync with the switch in applyDeviceEmulation — same modes,
// same device shortcuts.
func EmulationViewportSize(mode, devName string) (int, int, error) {
	if devName != "" {
		d, _, err := resolveDevice(devName)
		if err != nil {
			return 0, 0, err
		}
		info := d.Device()
		return int(info.Width), int(info.Height), nil
	}

	switch mode {
	case "mobile":
		info := device.IPhone15ProMax.Device()
		return int(info.Width), int(info.Height), nil
	case "tablet":
		info := device.IPadPro.Device()
		return int(info.Width), int(info.Height), nil
	case "desktop":
		return DesktopWidth, DesktopHeight, nil
	case "off", "":
		return 0, 0, nil
	default:
		return 0, 0, fmt.Errorf("unsupported mode: %s", mode)
	}
}
```

En el handler de `browser_emulate_device` (líneas 62-76), antes de llamar
`b.applyDeviceEmulation()`:

```go
var actualW, actualH int
if b.IsOpen() && b.Ctx != nil {
	// args.Mode/args.Device already passed validation above (lines 38-51),
	// so the error here cannot occur — it is only re-derived to get the
	// dimensions, not to re-validate.
	if reqW, reqH, _ := EmulationViewportSize(args.Mode, args.Device); reqW > 0 && reqH > 0 {
		if _, err := b.GrowWindowToFit(reqW, reqH); err != nil {
			b.Logger(fmt.Sprintf("Failed to grow window for emulation: %v", err))
		}
	}

	if err := b.applyDeviceEmulation(); err != nil {
		return nil, err
	}
	b.UI.RefreshUI()

	// Read back dimensions dynamically
	if err := chromedp.Run(b.Ctx,
		chromedp.Evaluate(`window.innerWidth`, &actualW),
		chromedp.Evaluate(`window.innerHeight`, &actualH),
	); err != nil {
		b.Logger(fmt.Sprintf("Failed to read back viewport: %v", err))
	}
}
```

(Sustituye el bloque `if b.IsOpen() && b.Ctx != nil { ... }` existente; el
resto del handler —`statusMsg`, captura de screenshot— no cambia.)

Actualizar también la `Description` de la tool (línea 24), porque hoy afirma
justo lo contrario de lo que este plan implementa — un agente MCP lee esa
descripción para saber qué esperar:

```go
Description: "Emulate a mobile device, tablet, or named device viewport. Also grows the physical window live (never shrinks) so the requested viewport is fully visible instead of being covered by DevTools — window position and any pre-existing size are preserved as a floor. This toggle affects rendering and touch events. This change is persisted.",
```

### 4.6 `OpenBrowser.go` — aplicar el mismo auto-fit al restaurar

El bloque que restaura la emulación al reabrir (líneas 89-97) hoy sólo llama
`applyDeviceEmulation`. Debe crecer la ventana primero, igual que el handler
del tool:

```go
// Restore device emulation if set
h.Mu.Lock()
vMode := h.ViewportMode
vDevice := h.ViewportDevice
h.Mu.Unlock()
if vMode != "" && vMode != "off" {
	if reqW, reqH, err := EmulationViewportSize(vMode, vDevice); err == nil && reqW > 0 && reqH > 0 {
		if _, err := h.GrowWindowToFit(reqW, reqH); err != nil {
			h.Logger(fmt.Sprintf("Failed to grow window for restored emulation: %v", err))
		}
	}
	if err := h.applyDeviceEmulation(); err != nil {
		h.Logger(fmt.Sprintf("Failed to restore emulation: %v", err))
	}
}
```

Este bloque corre después de `CreateBrowserContext` (línea 56, ya fijó
`h.DevToolsReserved`) y antes de `h.ready = true` (línea 104) — el orden
correcto para que `RequiredWindowSize` vea el flag ya establecido.

## 5. Lo que NO se toca

- **Posición de la ventana** (`Position`, `x,y`): sólo crece el tamaño, nunca
  se reposiciona. Igual que el comportamiento actual de
  `CalculateConstrainedSize`.
- **`browser_audit_mobile`** (`mcp-audit.go`): ese plan anterior audita CSS de
  la página, no geometría de ventana; no hay solape.
- **La condición `h.Width > 1200`** que decide si DevTools se auto-abre: sigue
  siendo una decisión de una sola vez, al lanzar Chrome. Si la ventana crece
  después de ese punto (p.ej. por este mismo mecanismo), DevTools no se
  abre/cierra retroactivamente — no hay comando CDP fiable para eso, y no es
  el problema que este plan resuelve.
- **`monitorBrowserGeometry`** (polling cada 2s): sigue igual. Puede observar
  el tamaño que `GrowWindowToFit` acaba de fijar y volver a "guardarlo" — es
  idempotente, no hay conflicto.

## 6. Verificación

- `gotest ./...` en `devbrowser` — verde, sin tocar tests existentes salvo los
  nuevos de este plan.
- Tests nuevos en `tests/window_autofit_test.go` (paquete `devbrowser_test`,
  mismo patrón que `tests/geometry_constraints_test.go`):

  | Test | Qué prueba |
  |---|---|
  | `TestRequiredWindowSize_GrowsForDevToolsReservation` | `DevToolsReserved: true`, ventana 800×600, monitor 1920×1080 → `RequiredWindowSize(1440, 900)` da ancho ≥ `1440+DevToolsReservedWidth`, clampado ≤ 1920 |
  | `TestRequiredWindowSize_NeverShrinks` | ventana ya 1600×1000 → `RequiredWindowSize(375, 812)` devuelve 1600×1000 sin cambios |
  | `TestRequiredWindowSize_ClampsToMonitor` | tamaño requerido mayor que el monitor → resultado no supera `MonitorWidth`/`MonitorHeight` |
  | `TestGrowWindowToFit_NoOpWhenContextNil` | `b.Ctx == nil` → devuelve `false, nil` sin panic |
  | `TestGrowWindowToFit_NoOpWhenAlreadyBigEnough` | `b.Ctx` no nil (p.ej. `context.Background()`) pero el tamaño ya alcanza → devuelve `false, nil` **sin** intentar `chromedp.Run` (verificable porque no hay browser real detrás y el test no debe fallar ni colgar) |

- Test nuevo en `tests/device_emulation_test.go`:

  | Test | Qué prueba |
  |---|---|
  | `TestEmulationViewportSize_Modes` | tabla: `"mobile"` → `(430, 739)`, `"tablet"` → `(1024, 1366)`, `"desktop"` → `(1440, 900)`, `""`/`"off"` → `(0, 0, nil)`, modo desconocido → error |
  | `TestEmulationViewportSize_NamedDevice` | `devName` conocido (p.ej. `"iphone15promax"`) → dimensiones del catálogo; desconocido → error (mismo contrato que `resolveDevice`) |

- Prueba manual (no automatizable sin Chrome real, dejar constancia en el PR):
  ventana chica (p.ej. 800×600, sin DevTools por no superar 1200) → pedir
  `browser_emulate_device mode: "desktop"` → la ventana crece en vivo a
  ~1440×900 (sin DevTools reservado porque no se abrió a esta ventana) y el
  navegador no se reinicia (una sola ventana de Chrome, sin parpadeo).

## 7. Fuera de alcance

- **Modelar DevTools acoplado abajo** (dock-bottom): la condición actual de
  auto-apertura (`h.Width > 1200`) sólo dispara para ventanas anchas, que es
  el caso donde Chrome acopla a la derecha por defecto. No se intenta cubrir
  el caso de ventanas angostas con DevTools abajo.
- **Leer el ancho real del panel de DevTools vía CDP**: no existe ese comando
  en el protocolo vendorizado. `DevToolsReservedWidth` queda como estimación
  documentada y ajustable, no como medición.
- **Reposicionar la ventana** (mover `x,y` cuando el crecimiento se saldría
  del monitor en un setup multi-monitor): fuera de alcance: igual que
  `CalculateConstrainedSize` hoy, sólo se clampa tamaño, nunca se recalcula
  posición. Documentado como limitación conocida, no como bug.
- **Encoger la ventana automáticamente** cuando se pide un dispositivo más
  chico que el tamaño actual: decisión explícita de este plan (§3) — el
  crecimiento es de una sola dirección.

## 8. Reglas de desarrollo

- Este repo es **tooling de backend** (lanza un proceso Chrome del host): la
  stdlib es legítima, no la quites.
- **No modifiques `chromedp/` ni `cdproto/`.** Son vendorizados; este plan
  sólo *usa* `Browser.setWindowBounds`, ya presente en
  `cdproto/browser/browser.go:500-523`.
- Sin strings/números mágicos repetidos: `DevToolsReservedWidth` es la única
  fuente del margen reservado; no repitas `420` en otro lugar.
- `RequiredWindowSize` y `GrowWindowToFit` van exportados (mayúscula inicial):
  los tests viven en el paquete externo `devbrowser_test`
  (`tests/*_test.go`), igual que `CalculateConstrainedSize` y `SaveGeometry`.
- Comentarios sólo donde documenten un contrato no obvio de Chrome/CDP (nivel
  de los comentarios ya existentes en `context.go` sobre `ozone-platform`).
- Código y comentarios en inglés. Este documento en español.
- No ejecutes `gopush` ni `codejob`.

## 9. Etapas

| # | Etapa | Archivos |
|---|---|---|
| 1 | Campo `DevToolsReserved` | `devbrowser.go` |
| 2 | Fijar el flag al decidir auto-abrir DevTools | `context.go` |
| 3 | `DevToolsReservedWidth` + `RequiredWindowSize` (puro) | `geometry_utils.go` |
| 4 | `GrowWindowToFit` (resize en vivo vía CDP) | `window_autofit.go` (nuevo) |
| 5 | `EmulationViewportSize` + enganche en la tool + Description actualizada | `mcp-management.go` |
| 6 | Enganche en la restauración de arranque | `OpenBrowser.go` |
| 7 | Tests | `tests/window_autofit_test.go` (nuevo), `tests/device_emulation_test.go` |
| 8 | README: describir el auto-fit y su estimación de DevTools | `README.md` |
