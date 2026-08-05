---
PLAN: "feat: emulación de dispositivo fiel + auditoría móvil"
TAG: v0.6.0
---

> Plan autocontenido. Todo lo necesario para ejecutarlo está en este documento.

# Plan — Emulación de dispositivo fiel y auditoría móvil en `devbrowser`

## 1. Objetivo

Cerrar la brecha entre lo que muestra el navegador emulado y lo que se ve en un
iPhone real, **sin añadir dependencias y sin salir de Chromium/CDP**.

El alcance es exclusivamente lo que le corresponde a esta librería: *controlar y
medir el navegador*. Lo que hay que corregir en el HTML servido o en el CSS por
defecto **no se toca aquí** (ver §7, Fuera de alcance).

## 2. Diagnóstico

### 2.1 La emulación actual pierde tres de las cuatro métricas que importan

`mcp-management.go:104-141` implementa `applyDeviceEmulation` así:

```go
// mcp-management.go:12-19
MobileWidth   = 375
MobileHeight  = 812
TabletWidth   = 768
TabletHeight  = 1024
DesktopWidth  = 1440
DesktopHeight = 900

// mcp-management.go:115
chromedp.EmulateViewport(MobileWidth, MobileHeight, chromedp.EmulateMobile)
```

Problemas, todos verificables en el árbol:

| # | Defecto | Evidencia | Consecuencia |
|---|---|---|---|
| 1 | `deviceScaleFactor` se queda en **1.0** | `chromedp.EmulateViewport` construye `emulation.SetDeviceMetricsOverride(w, h, 1.0, false)` en `chromedp/emulate.go:24`, y ninguna de las opciones que recibe (`EmulateMobile`) toca la escala | Un iPhone usa **3.0**. Todo lo que dependa del DPR se prueba mal: `srcset`, `@media (-webkit-min-device-pixel-ratio: 2)`, bordes hairline, y la nitidez de los screenshots |
| 2 | No se aplica `SetUserAgentOverride` | no aparece en `mcp-management.go` | La página recibe UA de Chrome desktop en modo "mobile". Cualquier rama que mire el UA se comporta distinto que en producción |
| 3 | Medidas inventadas | `375x812` es un iPhone X (2017) | Los dispositivos actuales son otros: iPhone 15 Pro Max mide `430x739` de viewport visible |
| 4 | Se ignora un catálogo ya vendorizado | `chromedp/device/device.go` contiene **131 dispositivos** | Se mantiene a mano lo que ya está resuelto |

El catálogo vendorizado ya tiene los cuatro valores correctos por dispositivo.
Ejemplo literal, `chromedp/device/device.go:552`:

```go
{"iPhone 15 Pro Max", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", 430, 739, 3.000000, false, true, true}
//  nombre                UA real                                                                                                                            w    h   DPR      landscape mobile touch
```

Y `chromedp.Emulate` (`chromedp/emulate.go:88-110`) ya aplica los tres overrides
juntos —UA, métricas con DPR y orientación, y touch— en una sola acción. El
código sólo tiene que llamarlo.

> La altura `739` (no `932`, la altura física en puntos) es intencional en el
> catálogo de Puppeteer: es el viewport **visible** con la barra de Safari
> desplegada. Es justamente la medida que hoy no se está usando.

### 2.2 Bloque muerto en el arranque

`OpenBrowser.go:62-71` lee `ViewportMode`, entra en el `if` y **no ejecuta
nada**: sólo hay comentarios. La restauración real ocurre 30 líneas más abajo
(`OpenBrowser.go:100-108`). El primer bloque es ruido que sugiere que ahí pasa
algo.

### 2.3 Lo que CDP no puede emular (y hay que dejar por escrito)

Revisado contra el `cdproto/` vendorizado: **no existe** ningún comando de
override de `safe-area-inset`. Los `env(safe-area-inset-*)` del notch y la barra
de gestos no son emulables desde esta librería con esta versión del protocolo.

Tampoco son emulables: la fuente San Francisco (no está instalada en el sistema),
los controles nativos de iOS, ni la contracción de la barra de URL de Safari que
hace que `100vh` desborde.

Consecuencia de diseño: **este plan no promete "verse igual que en el iPhone"**.
Promete métricas correctas y una herramienta que *detecta y nombra* los casos que
la emulación no puede reproducir. Prometer lo primero sería el defecto que este
plan viene a corregir, con otro nombre.

---

## 3. Decisión

Tres cambios, ninguno con dependencias nuevas:

1. `applyDeviceEmulation` pasa a delegar en `chromedp.Emulate` + catálogo `chromedp/device`.
2. `browser_emulate_device` acepta un dispositivo concreto además de los modos actuales.
3. Nueva herramienta `browser_audit_mobile`, 100% JS inyectado, que reporta los
   problemas que la emulación no puede mostrar.

### Alternativas rechazadas

| Alternativa | Por qué no |
|---|---|
| Corregir sólo las constantes (`375x812` → `430x739`) | Arregla 1 de 4 defectos: seguirían faltando DPR y UA. Y deja el catálogo sin usar |
| Añadir constantes `MobileScale`, `MobileUA`, … | Duplica a mano lo que `chromedp/device` ya tiene por los 131 dispositivos. Es mantener un fork del catálogo |
| Poner UA de Safari para "probar WebKit" | Renderiza Blink con UA de Safari: una combinación que no existe en producción. Da confianza falsa |
| Emular safe areas inyectando CSS `env()` | `env()` no es escribible desde JS. Lo que sí se puede es *detectar su ausencia*, que es lo que hace `browser_audit_mobile` |

---

## 4. Cambios

### 4.1 `mcp-management.go` — emulación delegada al catálogo

`applyDeviceEmulation` resuelve el modo a un `chromedp.Device` y ejecuta
`chromedp.Emulate`:

| Modo | Dispositivo del catálogo | Resultado |
|---|---|---|
| `mobile` | `device.IPhone15ProMax` | 430×739, DPR 3, UA de iOS 17.5, touch |
| `tablet` | `device.IPadPro` (verificar nombre exacto en `device.go`) | métricas y UA de iPad |
| `desktop` | *(sin dispositivo)* | se mantiene `EmulateViewport(1440, 900)` + touch off |
| `off` / `""` | — | `EmulateReset()` (`chromedp/emulate.go:118`), que además limpia el UA override; hoy `ClearDeviceMetricsOverride` no lo hace |

Las constantes `MobileWidth/Height` y `TabletWidth/Height` (`mcp-management.go:12-17`)
**se eliminan**: su valor pasa a venir del catálogo. `DesktopWidth/Height` se
quedan, porque "desktop" no es un dispositivo del catálogo sino una medida
nuestra.

> `desktop` es el único modo que no debe activar `mobile`/`touch`. Es la razón de
> que no se mapee a un dispositivo del catálogo, y así está ya en el código
> actual (`mcp-management.go:123-127`).

### 4.2 `browser_emulate_device` — dispositivo explícito

`EmulateDeviceArgs` gana un campo opcional `device`. Reglas:

- `device` vacío → comportamiento actual por `mode` (compatibilidad hacia atrás intacta).
- `device` con valor → gana sobre `mode`, y se resuelve contra el catálogo por
  nombre normalizado (minúsculas, sin espacios: `"iphone15promax"`, `"ipadpro"`,
  `"pixel7"`).
- Nombre desconocido → error explícito que **liste los dispositivos disponibles**,
  no un `unsupported device: x` a secas. El consumidor es un LLM: el error es su
  única forma de descubrir el catálogo.

Persistencia: el dispositivo elegido se guarda junto al modo. En `config.go` se
añade `StoreKeyViewportDevice = "viewport_device"`, cargado en `LoadConfig` y
guardado en `SaveConfig` con el mismo patrón que `StoreKeyViewportMode`
(`config.go:11,56-58,84-86`), y se restaura en `OpenBrowser.go:100-108`.

### 4.3 `browser_audit_mobile` — nueva herramienta

Archivo nuevo `mcp-audit.go`, siguiendo el patrón de `mcp-performance.go`:
una constante con el JS, una función que formatea el reporte en texto compacto, y
el registro de la `mcp.Tool`. Sin dominios CDP: sólo `chromedp.Evaluate`. Eso la
hace además portable a un futuro segundo motor.

Checks, todos con `selector` + `línea de por qué importa`:

| Check | Qué detecta | Por qué |
|---|---|---|
| `viewport-meta` | ausencia de `<meta name="viewport">`, o presencia sin `viewport-fit=cover` | Sin el meta, iOS renderiza a 980px y toda la app se ve reducida. Sin `viewport-fit=cover`, los `env(safe-area-inset-*)` valen **0** y el CSS de safe areas es código muerto |
| `vh-units` | elementos con `height`/`min-height` calculados desde `vh` | `100vh` no encoge cuando la barra de Safari se contrae: el contenido se corta abajo. Se corrige con `dvh`/`svh` |
| `safe-area` | elementos `position: fixed` pegados a `top: 0` o `bottom: 0` sin padding derivado de `env()` | Quedan bajo el notch o bajo la barra de gestos |
| `input-zoom` | `input`/`select`/`textarea` con `font-size` computado `< 16px` | iOS hace zoom automático al enfocarlos y descoloca el layout |
| `tap-target` | elementos interactivos con caja `< 44×44 px` | Mínimo táctil de las HIG de Apple |
| `fixed-vh` | `position: fixed` **y** altura en `vh` a la vez | El caso que más rompe: header/footer fijo que salta durante el scroll en iOS |

Salida: texto plano compacto, agrupado por check, con selector CSS del elemento y
conteo. Nada de JSON — el consumidor es un LLM y `mcp-performance.go:74` ya fijó
esa convención en este repo.

Argumentos: `selector` opcional para acotar el análisis a un subárbol. La
herramienta es de sólo lectura: `Resource: "browser"`, `Action: 'r'`.

### 4.4 `OpenBrowser.go` — eliminar el bloque muerto

Borrar `OpenBrowser.go:62-71`. La restauración de emulación ya vive en las líneas
100-108, después de `Navigate`, que es donde tiene que estar.

### 4.5 README

En la tabla de herramientas MCP, añadir `browser_audit_mobile`. Y una sección
corta de **qué reproduce y qué no** la emulación, con la tabla de §2.3. Es la
parte del cambio que evita que alguien confíe de más en el emulador.

---

## 5. Reglas de desarrollo

- Este repo es **tooling de backend** (lanza un proceso Chrome del host): la
  stdlib es legítima, no la quites.
- Paquete plano en la raíz para el código de librería. No inventes subpaquetes.
- **No modifiques `chromedp/` ni `cdproto/`.** Son vendorizados; la migración
  consiste en *usar* `chromedp/device`, no en editarlo.
- El JS de auditoría va en una constante de Go, como `GetPerformanceJS`
  (`mcp-performance.go:13`). Nada de construir JS por concatenación de datos del
  usuario: el `selector` entra por `JSON.stringify` en la plantilla o como
  argumento de la función, nunca interpolado crudo.
- Sin strings repetidos: si un nombre de check aparece más de una vez, es una
  constante con nombre.
- Comentarios sólo donde documenten un contrato no obvio de Chrome/CDP (el nivel
  del comentario de `ozone-platform` en `context.go:19-23` es la vara).
- Código y comentarios en inglés. Este documento en español.
- No ejecutes `gopush` ni `codejob`.

## 6. Verificación

- `tests/device_emulation_test.go` ya existe: extenderlo para afirmar que tras
  `mode: "mobile"` el DPR emulado es 3 y el UA contiene `iPhone`, leídos desde la
  página con `window.devicePixelRatio` y `navigator.userAgent`.
- Test nuevo del resolutor de nombres de dispositivo: alias conocido → `device.Info`
  esperado; desconocido → error que menciona al menos un nombre válido.
- Test de `browser_audit_mobile` contra un HTML fijo en `tests/` con un caso
  positivo de cada check.
- Los tests existentes de `tests/` deben pasar **sin modificarse**: el
  comportamiento por defecto (`mode` sin `device`) no cambia de contrato.

## 7. Alcance

### Dentro

- `mcp-management.go`, `config.go`, `OpenBrowser.go`, `mcp-audit.go` (nuevo),
  `mcp-tools.go` (registro), `README.md`, tests.

### Fuera — pertenece a otras librerías

Los dos arreglos que más impacto visual tienen **no son de este repo**, y meterlos
aquí sería duplicar responsabilidades:

| Arreglo | Dueño | Por qué no aquí |
|---|---|---|
| `viewport-fit=cover` en el `<meta name="viewport">` | quien emite el HTML servido | `devbrowser` observa el navegador; no inyecta etiquetas en la app del usuario. Si lo hiciera, el emulador mostraría un HTML que el servidor real no sirve |
| Tokens de safe area, unidades de viewport dinámico y guarda anti-zoom de iOS | la librería de CSS por defecto del framework | Son decisiones de diseño del sistema, aplicables a **toda** app del ecosistema, no sólo a las que se depuran con esta herramienta |

`browser_audit_mobile` es exactamente la pieza que conecta ambos mundos: **detecta
y reporta** esos dos problemas, y deja el arreglo en manos de quien es dueño de
la etiqueta y del CSS.

### Fuera, sin dueño en este plan

- Añadir un segundo motor de render (WebKit). Es otra decisión, con su propio
  análisis; ver `docs/WEBKIT_PLAN_SUPPORT.md`.
- Automatización de Safari o de iOS.
