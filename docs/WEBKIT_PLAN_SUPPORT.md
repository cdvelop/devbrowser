---
PLAN: "docs: análisis y recomendación de soporte WebKit/Safari"
TAG: none
EXECUTOR: none
REVIEWER: none
---

# Soporte WebKit / Safari en `devbrowser` — análisis y recomendación

## Resumen ejecutivo (TL;DR)

1. **No, no es posible "emular WebKit" con chromedp.** No es una limitación de esta
   librería: CDP (Chrome DevTools Protocol) sólo lo habla Chromium/Blink. Safari
   habla otro protocolo distinto (WebKit Inspector Protocol / W3C WebDriver). Son
   dos motores y dos canales de control incompatibles.
2. **Cambiar el User-Agent a Safari es peor que no hacer nada**: seguirías
   renderizando con Blink, pero perderías la señal de que estás en Blink. Da
   confianza falsa.
3. **La mayor parte de lo que ves distinto en tu iPhone probablemente NO se debe al
   motor**, sino al *dispositivo*: safe areas del notch, viewport dinámico
   (`100vh` con la barra de Safari), `devicePixelRatio` 3, y fuentes del sistema.
   Eso se reproduce **hoy, sin dependencias nuevas**, con lo que ya está
   vendorizado en el repo — y ahora mismo el código no lo está aprovechando.
4. **Recomendación**: plan en 3 fases. Fase 0 arregla la emulación Chromium actual
   (días, cero dependencias, resuelve ~70% del problema real). Fase 1 añade WebKit
   *de verdad* vía **W3C WebDriver** (WebKitGTK en Linux) detrás de una interfaz
   `Engine`, manteniendo la promesa "sin dependencias" (sólo `net/http` de la
   stdlib). Fase 2 reutiliza **el mismo cliente WebDriver** para apuntar a
   `safaridriver` en macOS: Safari real, con cero código adicional.
5. **Se rechaza Playwright** (aunque sea el camino más rápido) porque rompe el
   principio arquitectónico del repo: exige runtime de Node, un proceso driver
   externo y ~1 GB de navegadores descargados.

---

## 1. Por qué chromedp **no puede** emular WebKit

### 1.1 El protocolo es el muro, no el navegador

Todo este repo está construido sobre CDP. La evidencia está en el propio árbol:
`cdproto/` implementa 54 dominios (`emulation`, `fetch`, `network`, `runtime`,
`input`, `dom`…) y las 19 herramientas MCP se apoyan directamente en ellos:

| Herramienta MCP | Dominio CDP del que depende |
|---|---|
| `browser_get_console`, `browser_get_errors` | `Runtime` (eventos `consoleAPICalled`, `exceptionThrown`) — ver `console_logs.go:24` |
| `browser_get_network_logs` | `Network` (eventos de request/response) — `mcp-network.go:94` |
| `browser_intercept_request` | `Fetch` — `mcp-intercept.go` |
| `browser_emulate_device` | `Emulation` — `mcp-management.go:104` |
| `browser_click_element`, `browser_fill_element`, `browser_swipe_element` | `Input` — `mcp-interaction.go` |
| `browser_evaluate_js`, `browser_get_asset` | `Runtime` |

Safari **no expone CDP**. Expone el *WebKit Remote Inspector Protocol*, cuyos
dominios tienen otros nombres, otros parámetros y otra semántica de eventos.
Portar `cdproto/` a ese protocolo no es "añadir un backend", es reescribir la
capa de transporte y las 19 herramientas.

### 1.2 Los motores son diferentes en el punto exacto que te duele

| Capa | Chrome / Chromium | Safari / iOS |
|---|---|---|
| Motor de layout | Blink (fork de WebCore de 2013) | WebCore |
| Motor JS | V8 | JavaScriptCore |
| Protocolo de automatización | CDP | WebKit Inspector / WebDriver |
| Ciclo de release | ~4 semanas | atado a las versiones de iOS/macOS |

Doce años de divergencia desde el fork es justamente lo que produce las
diferencias de layout que estás viendo. Ninguna opción de `Emulation.*` cambia el
motor de layout: `Emulation.setDeviceMetricsOverride` cambia el *tamaño del
lienzo*, no *quién pinta dentro*.

### 1.3 El anti-patrón a evitar explícitamente

```go
// NO HACER: esto no es "modo Safari"
emulation.SetUserAgentOverride("Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 ...) Safari/604.1")
```

Con esto tu app *cree* que está en Safari (y puede activar sus propios
polyfills/workarounds), pero Blink sigue haciendo el layout. Es decir: pruebas
una combinación que **no existe en el mundo real**. La única razón legítima para
poner ese UA es probar tu propia lógica de detección de UA, y en ese caso hay que
documentarlo como tal.

---

## 2. Diagnóstico: ¿por qué se ve "muy diferente" en el iPhone?

Este es el punto más importante del documento. Antes de invertir semanas en un
segundo motor, conviene separar las causas, porque **están mezcladas** y tienen
costes de solución radicalmente distintos.

| # | Síntoma típico | Causa raíz | ¿Lo arregla tener WebKit? |
|---|---|---|---|
| 1 | Contenido cortado abajo / footer que no se ve / `100vh` que desborda | La barra de URL de Safari iOS se contrae al hacer scroll; `100vh` mide el viewport *largo*, no el visible | ❌ No. Es UI del navegador, no motor. Se arregla con `dvh`/`svh`/`lvh` |
| 2 | Header debajo del notch / Dynamic Island; botones bajo la barra de gestos | Falta `viewport-fit=cover` + `env(safe-area-inset-*)` | ❌ No. Es hardware/OS |
| 3 | Todo se ve "más grande" o "más chico", imágenes borrosas | `devicePixelRatio` = 3 en iPhone; el emulador actual usa **1.0** | ❌ No. Es métrica de emulación (**bug actual del repo**, ver §3) |
| 4 | Los textos cortan distinto, se desborda un botón, cambia el alto de una fila | Fuente del sistema: iOS usa **San Francisco**; tu Linux/Windows resuelve `-apple-system` a otra cosa con métricas distintas | ❌ No (ni siquiera con WebKitGTK: la fuente no está en el sistema) |
| 5 | Inputs, `select`, date pickers, checkboxes con otro aspecto | Controles nativos de iOS + `-webkit-appearance` | ⚠️ Parcialmente |
| 6 | Zoom automático al enfocar un input | iOS hace zoom si `font-size < 16px` | ❌ No, comportamiento de iOS |
| 7 | Scroll con rebote, `position: fixed` que "salta" durante el scroll | Composición y momentum scroll de iOS | ⚠️ Parcialmente |
| 8 | Un layout entero roto, `:has()`/subgrid/container queries que no aplican | **Diferencia real de motor** (soporte y bugs de WebCore) | ✅ Sí |
| 9 | JS que falla sólo en iPhone: `new Date('2024-01-01 10:00')` → `Invalid Date`, regex, `Intl`, IndexedDB | **Diferencia real de motor** (JavaScriptCore) | ✅ Sí |
| 10 | Colores distintos en fotos/degradados | Pantalla P3 vs sRGB | ❌ No |

**Conclusión del diagnóstico:** de los 10 síntomas más frecuentes, sólo 2 (los
casos 8 y 9) requieren un motor WebKit real. Los casos 1–4, que son con
diferencia los más comunes en apps propias, se atacan **hoy** mejorando la
emulación que ya tienes. Por eso la Fase 0 va primero: es la que tiene mejor
relación coste/beneficio, y además te dice cuánto problema queda realmente para
justificar la Fase 1.

---

## 3. Estado actual del repo: hay fidelidad gratis sin usar

La emulación actual está en `mcp-management.go`:

```go
// mcp-management.go:12-19
MobileWidth   = 375
MobileHeight  = 812
...
// mcp-management.go:115
chromedp.EmulateViewport(MobileWidth, MobileHeight, chromedp.EmulateMobile)
```

Tres problemas concretos, todos verificables:

1. **`deviceScaleFactor` queda en 1.0.** `chromedp.EmulateViewport` construye
   `emulation.SetDeviceMetricsOverride(width, height, 1.0, false)`
   (`chromedp/emulate.go:24`) y ninguna de las opciones que se le pasan cambia la
   escala. Un iPhone real usa **3.0**. Todo lo que dependa de DPR (imágenes
   `srcset`, `@media (-webkit-min-device-pixel-ratio: 2)`, bordes hairline,
   nitidez de los screenshots) se está probando mal.
2. **No hay `SetUserAgentOverride`.** La app recibe UA de Chrome desktop en modo
   "mobile", con lo cual cualquier rama de código server-side o client-side que
   mire el UA se comporta distinto que en producción.
3. **Se ignora un catálogo de 131 dispositivos que ya está vendorizado.**
   `chromedp/device/device.go:552` ya contiene, por ejemplo:

   ```
   {"iPhone 15 Pro Max", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 ...)", 430, 739, 3.0, false, true, true}
   ```

   Es decir: viewport correcto (430×739, ya descontando la UI de Safari), DPR 3,
   `mobile` y `touch` activos, y el UA que corresponde. Y `chromedp.Emulate`
   (`chromedp/emulate.go:88`) ya aplica los tres overrides juntos. El código sólo
   tiene que llamarlo.

Además hay un bloque muerto en `OpenBrowser.go:62-71`: lee `ViewportMode`, entra
en el `if` y no hace nada (sólo comentarios). La restauración real ocurre después,
en las líneas 100-108. Ese bloque debe eliminarse.

**Nota honesta sobre safe areas:** revisé el `cdproto/` vendorizado y **no existe**
un comando de override de `safe-area-inset`. Es decir, los `env(safe-area-inset-*)`
del caso 2 no se pueden emular desde CDP con esta versión. La mitigación práctica
es que la app lea los insets a variables CSS
(`--sat: env(safe-area-inset-top, 0px)`) y que `devbrowser` inyecte valores de
prueba sobre esas variables; eso sí es posible con `browser_evaluate_js`.

---

## 4. Opciones evaluadas

| Opción | Motor real | Plataformas | Dependencias nuevas | Coste | ¿Da layout de iPhone? |
|---|---|---|---|---|---|
| **A. Mejorar emulación Chromium** | ❌ Blink | todas | **ninguna** | bajo | Métricas sí, motor no |
| **B. WebKitGTK + WebDriver** | ✅ WebCore/JSC | Linux (y macOS/BSD vía paquete) | binario del sistema; en Go sólo stdlib | medio | Motor sí, métricas de iPhone no |
| **C. `safaridriver`** | ✅ Safari real | **sólo macOS** | ninguna (viene con macOS) | bajo *si ya existe B* | Safari desktop, no iPhone |
| **D. Playwright WebKit** | ✅ WebKit (build propio) | todas | Node + driver + ~1 GB | medio | Sí (tiene emulación de dispositivo) |
| **E. iOS Simulator / iPhone real** | ✅ MobileSafari | **sólo macOS** (o el teléfono) | Xcode / cable USB | alto (o cero, si es manual) | **Es la verdad de terreno** |
| **F. Cloud (BrowserStack, LambdaTest…)** | ✅ real | todas | cuenta de pago, red | recurrente | Sí |

### Por qué se descarta D (Playwright), aunque sea lo más rápido

Playwright trae un build propio de WebKit que corre en Linux y **sí** tiene
emulación de dispositivo (viewport + DPR + UA + touch en un motor WebKit real).
Técnicamente es la opción más completa. Se descarta porque:

- Rompe el principio del repo. `go.mod` no tiene una sola dependencia externa a
  `tinywasm/*`; todo Chrome DevTools Protocol está vendorizado precisamente para
  no depender de terceros.
- `playwright-go` **no** es una implementación en Go: es un cliente que lanza el
  driver de **Node.js** de Playwright. Pasarías de "un binario de Chrome" a "Node
  + driver + navegadores", con `playwright install` obligatorio en cada máquina y
  cada CI.
- Acopla las versiones: cada upgrade de Playwright puede romper el driver.

Si en algún momento se decide que "sin dependencias" ya no es un requisito duro,
**D es la respuesta correcta** y este documento debería revisarse. Mientras el
requisito siga en pie, B+C es el equivalente más cercano.

### Por qué se descarta F (cloud)

Coste recurrente, latencia, y exige exponer la app en desarrollo hacia fuera
(túnel). Para una herramienta de desarrollo local es desproporcionado. Sigue
siendo válido para una verificación puntual antes de un release.

---

## 5. Recomendación

> **Fase 0 ahora. Fase 1 sólo si la Fase 0 no basta. Fase 2 es casi gratis una vez
> hecha la 1. Y en paralelo, siempre, el iPhone real como verdad de terreno.**

### Fase 0 — Exprimir la emulación Chromium (recomendado empezar aquí)

*Objetivo: eliminar del tablero los síntomas 1–4 y 6, que no son culpa del motor.*

1. Migrar `applyDeviceEmulation` a `chromedp.Emulate(device.X)` usando el catálogo
   ya vendorizado. Con eso llegan gratis UA correcto, DPR 3, orientación y touch.
2. Ampliar `browser_emulate_device` para aceptar un nombre de dispositivo
   (`"iphone15promax"`, `"ipadpro"`, …) además de los modos actuales
   `mobile|tablet|desktop|off`, manteniendo compatibilidad hacia atrás: los modos
   actuales pasan a ser alias de dispositivos concretos.
3. Nueva herramienta de auditoría, p. ej. `browser_audit_mobile`, implementada
   **100% con JS inyectado** (mismo patrón que `mcp-performance.go`), que detecte
   los sospechosos habituales:
   - uso de `100vh`/`vh` en contenedores de layout sin variante `dvh`/`svh`
   - `<meta name="viewport">` sin `viewport-fit=cover`
   - ausencia de `env(safe-area-inset-*)` cuando hay elementos `fixed` pegados a
     los bordes
   - inputs con `font-size < 16px` (zoom automático de iOS)
   - `position: fixed` combinado con alturas en `vh`
4. Inyección de safe areas simuladas sobre variables CSS, documentando la
   convención (`--sat/--sar/--sab/--sal`).
5. Eliminar el bloque muerto de `OpenBrowser.go:62-71`.

**Criterio de salida:** si tras la Fase 0 sigues encontrando diferencias en el
iPhone que el emulador no reproduce, ya tienes casos concretos, y esos casos son
la justificación (y el set de pruebas) de la Fase 1.

### Fase 1 — WebKit real vía W3C WebDriver, sin dependencias Go

*Objetivo: cubrir los síntomas 8 y 9 (bugs reales de WebCore/JavaScriptCore).*

La clave arquitectónica: **W3C WebDriver es HTTP + JSON**. Un cliente mínimo se
escribe con `net/http` + el `tinywasm/json` que ya usa el repo. Cero dependencias
nuevas en `go.mod`, coherente con la filosofía de vendorizar.

- Motor: `WebKitWebDriver` de WebKitGTK (paquete `webkit2gtk-driver` en Debian/
  Ubuntu). Es WebCore + JavaScriptCore reales.
- Descubrimiento del binario: extender el patrón que ya existe en `execpath.go`
  (`ResolveChromeExecPath` con validación por `--version` y override por variable
  de entorno). Añadir `ResolveWebKitDriverPath` con `WEBKIT_DRIVER_PATH`.
- Arquitectura: extraer una interfaz `Engine` con las operaciones que ambos
  backends comparten, y dos implementaciones: `chromeEngine` (la actual, sin
  cambios de comportamiento) y `webkitEngine`.

**Matriz de compatibilidad realista de las 19 herramientas MCP sobre WebDriver:**

| Herramienta | WebDriver | Cómo |
|---|---|---|
| `browser_navigate` | ✅ | `POST /session/{id}/url` |
| `browser_screenshot` / `browser_save_screenshot` | ✅ | `GET /screenshot`, `GET /element/{id}/screenshot` |
| `browser_click_element` | ✅ | `POST /element/{id}/click` |
| `browser_fill_element` | ✅ | `POST /element/{id}/value` |
| `browser_swipe_element` | ✅ | Actions API (pointer type `touch`) |
| `browser_evaluate_js` | ✅ | `POST /execute/sync` |
| `browser_get_source` | ✅ | `GET /source` o `outerHTML` vía JS |
| `browser_get_content` / `browser_get_structure` | ✅ | ya se resuelven por JS |
| `browser_get_performance` | ✅ | ya es JS puro (`mcp-performance.go`) |
| `browser_get_styles` | ✅ | reescribir sobre `document.styleSheets` |
| `browser_get_storage` | ✅ | vía JS |
| `browser_inspect_element` | ⚠️ | reimplementar por JS (hoy no usa CDP, verificar) |
| `browser_get_asset` | ⚠️ | `fetch()` desde la página en vez de `Runtime` |
| `browser_get_console` / `browser_get_errors` | ⚠️ | requiere **shim JS** (parchear `console.*` y `window.onerror` en cada carga) en vez de eventos `Runtime` |
| `browser_emulate_device` | ❌ | WebDriver **no tiene emulación de dispositivo**; sólo `Set Window Rect` |
| `browser_get_network_logs` | ❌ | no hay dominio `Network`; requeriría proxy propio |
| `browser_intercept_request` | ❌ | no hay dominio `Fetch` |

Es decir: **~13 de 19 herramientas** son portables, 3 con trabajo extra y 3 no.
Eso es aceptable si el propósito del motor WebKit está bien acotado: *verificar
layout y semántica JS*, no *depurar red*. Las herramientas no soportadas deben
devolver un error explícito del tipo `"browser_get_network_logs no está disponible
en el motor webkit; usa el motor chrome"` — nunca fallar de forma silenciosa o
ambigua.

**Limitación que hay que aceptar por escrito:** WebKitGTK te da el *motor*, no el
*dispositivo*. No hay DPR 3, ni safe areas, ni fuente San Francisco, ni controles
nativos de iOS. Fase 1 responde "¿mi CSS/JS funciona en WebCore?", no "¿se ve
igual que en mi iPhone?". Confundir ambas preguntas es el mayor riesgo de este
plan.

### Fase 2 — Safari real en macOS, gratis

`safaridriver` (incluido en macOS, se activa con `safaridriver --enable`) **habla
el mismo W3C WebDriver** que WebKitGTK. Por eso Fase 2 es esencialmente
configuración: el mismo cliente de la Fase 1, apuntando a otro binario. Es el
mayor argumento a favor de elegir WebDriver en vez de un protocolo propietario.

Salvedad: es Safari *de escritorio*. Safari iOS no es automatizable con
`safaridriver`.

### Fase 3 / paralelo — La verdad de terreno

Ninguna de las fases anteriores sustituye al dispositivo real, y conviene decirlo
en el README para que nadie confíe de más:

- **Depuración remota real:** iPhone conectado por USB a un Mac → Safari →
  *Desarrollo* → tu iPhone. Inspector web completo sobre la página real. Es la
  única forma de ver simultáneamente el motor real, las fuentes reales, los safe
  areas reales y el viewport dinámico real.
- Si no hay Mac: el **Simulador de iOS** requiere igualmente macOS. En Linux no
  hay alternativa fiel; lo más cercano sigue siendo WebKitGTK (Fase 1).
- Para automatizar el iPhone haría falta `ios_webkit_debug_proxy` (expone el
  protocolo de inspección de WebKit del dispositivo) o Appium/XCUITest. Ambos
  quedan **fuera del alcance** de esta librería: son cadenas de herramientas
  propias de macOS y no encajan con un binario Go autocontenido.

---

## 6. Riesgos y no-objetivos

**Riesgos**

| Riesgo | Mitigación |
|---|---|
| Duplicar la lógica de las 19 herramientas en dos backends | Mantener toda la lógica basada en JS inyectado compartida entre motores; que la interfaz `Engine` sólo abstraiga transporte (navegar, evaluar, capturar, interactuar) |
| Falsa sensación de seguridad ("pasó en WebKitGTK, luego pasa en iPhone") | Documentar en el README exactamente qué cubre y qué no cada motor; nunca llamarlo "modo Safari" ni "modo iPhone" |
| WebKitGTK va por detrás de la versión de WebKit de Safari | Registrar la versión del driver en los reportes; usar iPhone real para el veredicto final |
| Regresión en el camino Chrome, que es el que se usa a diario | La refactorización a `Engine` no debe cambiar el comportamiento de Chrome; los tests de `tests/` deben pasar sin modificarse |

**No-objetivos explícitos**

- No se va a implementar el WebKit Inspector Protocol.
- No se va a añadir Node, Playwright ni Selenium al árbol de dependencias.
- No se va a soportar automatización de Safari iOS.
- No se va a hacer *UA spoofing* presentándolo como soporte de Safari.

---

## 7. Próximos pasos concretos

1. **[Fase 0]** Migrar `applyDeviceEmulation` a `chromedp.Emulate` + catálogo
   `chromedp/device`; añadir parámetro `device` a `browser_emulate_device`;
   persistirlo junto a `StoreKeyViewportMode` en `config.go`.
2. **[Fase 0]** Eliminar el bloque muerto de `OpenBrowser.go:62-71`.
3. **[Fase 0]** Implementar `browser_audit_mobile` (JS puro) con los checks de
   `100vh`, `viewport-fit`, safe areas y `font-size` de inputs.
4. **[Fase 0]** Documentar en el README qué reproduce y qué **no** reproduce la
   emulación Chromium.
5. **[Evaluación]** Usar la app real con Fase 0 aplicada. Anotar cada diferencia
   que persista contra el iPhone. **Ese listado decide si la Fase 1 se hace.**
6. **[Fase 1, si procede]** Spike de 1 día: cliente W3C WebDriver mínimo (sesión,
   navegar, ejecutar JS, screenshot) contra `WebKitWebDriver`, verificando de paso
   si el DPR se puede forzar vía `GDK_SCALE`. Con ese resultado se planifica el
   resto.
