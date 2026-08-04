# Soporte WebKit / Safari — análisis y estrategia

> Documento de referencia permanente. Explica **por qué** `devbrowser` no puede
> emular WebKit con chromedp, **cómo** se separan las causas de que una app se vea
> distinta en un iPhone, y **qué librería del ecosistema es dueña de cada arreglo**.
> Consúltalo antes de reabrir cualquiera de estos puntos.

---

## Resumen ejecutivo

1. **No es posible emular WebKit con chromedp.** No es una limitación de esta
   librería: CDP (Chrome DevTools Protocol) sólo lo habla Chromium/Blink. Safari
   habla otro protocolo (WebKit Inspector Protocol / W3C WebDriver). Dos motores,
   dos canales de control, incompatibles.
2. **Falsear el User-Agent a Safari es peor que no hacer nada.** Seguirías
   renderizando con Blink pero perderías la señal de que estás en Blink: pruebas
   una combinación que no existe en producción.
3. **La mayoría de las diferencias que se ven en un iPhone no vienen del motor**,
   sino del *dispositivo* y del *documento*: safe areas del notch, viewport
   dinámico, `devicePixelRatio` 3 y fuentes del sistema. Separar ambas causas es
   la decisión más importante de este documento (§2).
4. **El arreglo está repartido entre cuatro librerías**, no en una (§3). Meterlo
   todo en `devbrowser` sería duplicar responsabilidades ajenas.
5. **Si hace falta WebKit real**, el camino es **W3C WebDriver** (§5): un cliente
   HTTP con la stdlib sirve para WebKitGTK en Linux **y** para `safaridriver` en
   macOS, sin añadir una sola dependencia a `go.mod`.

---

## 1. Por qué chromedp no puede emular WebKit

### 1.1 El protocolo es el muro, no el navegador

Todo este repo está construido sobre CDP: `cdproto/` implementa 54 dominios y las
herramientas MCP se apoyan directamente en ellos.

| Herramienta MCP | Dominio CDP |
|---|---|
| `browser_get_console`, `browser_get_errors` | `Runtime` (eventos `consoleAPICalled`, `exceptionThrown`) — `console_logs.go:24` |
| `browser_get_network_logs` | `Network` — `mcp-network.go:94` |
| `browser_intercept_request` | `Fetch` — `mcp-intercept.go` |
| `browser_emulate_device` | `Emulation` — `mcp-management.go:104` |
| `browser_click_element`, `browser_fill_element`, `browser_swipe_element` | `Input` |
| `browser_evaluate_js`, `browser_get_asset` | `Runtime` |

Safari **no expone CDP**. Expone el *WebKit Remote Inspector Protocol*, con otros
dominios, otros parámetros y otra semántica de eventos. Portar `cdproto/` no es
"añadir un backend": es reescribir el transporte y todas las herramientas.

### 1.2 Los motores divergen justo donde duele

| Capa | Chrome / Chromium | Safari / iOS |
|---|---|---|
| Layout | Blink (fork de WebCore, 2013) | WebCore |
| JavaScript | V8 | JavaScriptCore |
| Automatización | CDP | WebKit Inspector / WebDriver |
| Releases | ~4 semanas | atado a iOS/macOS |

Doce años de divergencia desde el fork es lo que produce las diferencias de
layout. Ninguna opción de `Emulation.*` cambia el motor:
`Emulation.setDeviceMetricsOverride` cambia el *tamaño del lienzo*, no *quién
pinta dentro*.

### 1.3 El anti-patrón, explícito

```go
// NO HACER: esto no es "modo Safari"
emulation.SetUserAgentOverride("Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 ...) Safari/604.1")
```

La app *cree* que está en Safari y puede activar sus propios workarounds, pero
Blink sigue haciendo el layout. La única razón legítima para ese UA es probar tu
propia lógica de detección de UA, y entonces hay que documentarlo como tal.

---

## 2. Motor vs. dispositivo: la separación que hay que hacer primero

Antes de invertir en un segundo motor conviene separar las causas, porque están
mezcladas y sus costes de solución son radicalmente distintos.

| # | Síntoma en el iPhone | Causa raíz | ¿Necesita WebKit? |
|---|---|---|---|
| 1 | Contenido cortado abajo, footer invisible, `100vh` que desborda | La barra de URL de Safari iOS se contrae al hacer scroll; `100vh` mide el viewport *largo* | ❌ Es UI del navegador. Se corrige con `dvh`/`svh`/`lvh` |
| 2 | Header bajo el notch / Dynamic Island; botones bajo la barra de gestos | Falta `viewport-fit=cover` + `env(safe-area-inset-*)` | ❌ Es hardware/OS |
| 3 | Todo más grande o más chico, imágenes borrosas | `devicePixelRatio` 3 en iPhone | ❌ Es métrica de emulación |
| 4 | La app entera se ve reducida, como una web de escritorio | Falta `<meta name="viewport">`: iOS renderiza a 980px | ❌ Es el documento |
| 5 | Los textos cortan distinto, se desborda un botón, cambia el alto de una fila | Fuente del sistema: iOS usa **San Francisco**; otro SO resuelve `-apple-system` a otra cosa con otras métricas | ❌ No (ni con WebKitGTK: la fuente no está instalada) |
| 6 | Inputs, `select`, date pickers con otro aspecto | Controles nativos de iOS + `-webkit-appearance` | ⚠️ Parcialmente |
| 7 | Zoom automático al enfocar un input | iOS hace zoom si `font-size < 16px` | ❌ Comportamiento de iOS |
| 8 | Scroll con rebote; `position: fixed` que salta durante el scroll | Composición y momentum scroll de iOS | ⚠️ Parcialmente |
| 9 | Layout entero roto: `:has()`, subgrid, container queries que no aplican | **Diferencia real de motor** (WebCore) | ✅ Sí |
| 10 | JS que falla sólo en iPhone: `new Date('2024-01-01 10:00')` → `Invalid Date`, regex, `Intl`, IndexedDB | **Diferencia real de motor** (JavaScriptCore) | ✅ Sí |

**Conclusión:** de los diez síntomas más frecuentes, sólo dos requieren un motor
WebKit real. Los casos 1–5 y 7 —los más comunes en apps propias— se atacan sin
salir de Chromium, y **no todos pertenecen a esta librería**.

---

## 3. Reparto de responsabilidades en el ecosistema

Este es el resultado más reutilizable del análisis. Cada arreglo tiene un dueño
único; duplicarlo en otra librería es un fork con nombre amable.

| Arreglo | Dueño | Razón |
|---|---|---|
| Métricas de emulación fieles (viewport, **DPR 3**, UA, touch, orientación) | **`tinywasm/devbrowser`** | Es quien controla el navegador. El catálogo `chromedp/device` ya está vendorizado aquí con 131 dispositivos y sus cuatro métricas correctas (`chromedp/device/device.go`); `chromedp.Emulate` (`chromedp/emulate.go:88`) los aplica de una vez |
| **Detectar y reportar** los problemas que la emulación no puede reproducir | **`tinywasm/devbrowser`** | Es la pieza que puede *medir* la página cargada. Detecta; no arregla |
| `<meta name="viewport" content="…, viewport-fit=cover">` | quien **emite el documento HTML** (`tinywasm/html` para el shell de `Document()`; `tinywasm/assetmin` para el `index.html` del bundle) | `devbrowser` no inyecta etiquetas en la app del usuario: si lo hiciera, el emulador mostraría un HTML que el servidor real no sirve. Y sin `viewport-fit=cover` los `env(safe-area-inset-*)` valen **0**, con lo cual cualquier CSS de safe areas es código muerto |
| Tokens de safe area, unidades de viewport dinámico, guarda anti-zoom de iOS, reset de controles nativos | **`tinywasm/css`** | Son decisiones del sistema de diseño, aplicables a toda app del ecosistema. El reset de `css.reset.go` ya cubre `-webkit-text-size-adjust`, `-webkit-tap-highlight-color` y el `appearance` de botones e inputs: la familia del arreglo ya vive ahí |

Regla general derivada: **`devbrowser` mide, no corrige.** Cuando una diferencia
de render se pueda arreglar en el HTML o en el CSS por defecto, la herramienta
correcta a construir aquí es la que la *detecta y la nombra*, no la que la parchea
en caliente.

### 3.1 Condición previa dentro de esta librería

La capa de emulación Chromium tiene que entregar las cuatro métricas
(viewport, DPR, UA, touch) tomadas del catálogo vendorizado, no a mano. En el
análisis inicial (agosto 2026) faltaban tres de las cuatro: el DPR quedaba en
`1.0` porque `chromedp.EmulateViewport` lo fija así (`chromedp/emulate.go:24`),
no se aplicaba `SetUserAgentOverride`, y las medidas estaban escritas a mano
(`375x812`, un iPhone X de 2017) en vez de leerse del catálogo.

Mientras esa base no sea fiel, cualquier conclusión sacada del emulador sobre
"cómo se ve en el iPhone" es ruido, y no tiene sentido evaluar la §5.

### 3.2 Lo que CDP no puede emular

Revisado contra el `cdproto/` vendorizado: **no existe** comando de override de
`safe-area-inset`. Tampoco son emulables la fuente San Francisco, los controles
nativos de iOS ni la contracción de la barra de URL de Safari.

Es decir: la emulación Chromium, por buena que sea, nunca responderá "¿se ve
igual que en mi iPhone?". Responde "¿el layout aguanta estas métricas?".

---

## 4. Opciones evaluadas para tener WebKit real

| Opción | Motor real | Plataformas | Dependencias nuevas | ¿Da layout de iPhone? |
|---|---|---|---|---|
| **A. WebKitGTK + WebDriver** | ✅ WebCore/JSC | Linux (paquete `webkit2gtk-driver`) | binario del sistema; en Go sólo stdlib | Motor sí, métricas de iPhone no |
| **B. `safaridriver`** | ✅ Safari real | **sólo macOS** | ninguna (viene con macOS) | Safari de escritorio, no iPhone |
| **C. Playwright WebKit** | ✅ WebKit (build propio) | todas | Node + driver + ~1 GB | Sí (trae emulación de dispositivo) |
| **D. Simulador de iOS / iPhone real** | ✅ MobileSafari | **sólo macOS** (o el teléfono) | Xcode / cable USB | **Es la verdad de terreno** |
| **E. Cloud (BrowserStack, LambdaTest…)** | ✅ real | todas | cuenta de pago, red | Sí |

### Por qué se descarta C (Playwright), aunque sea lo más completo

Playwright trae un build propio de WebKit que corre en Linux y sí tiene emulación
de dispositivo sobre motor WebKit real. Técnicamente es la opción más completa.
Se descarta porque:

- Rompe el principio del repo: `go.mod` no tiene una sola dependencia fuera de
  `tinywasm/*`, y todo CDP está vendorizado precisamente para no depender de terceros.
- `playwright-go` no es una implementación en Go: es un cliente que lanza el
  driver de **Node.js**. Pasarías de "un binario de Chrome" a "Node + driver +
  navegadores", con `playwright install` obligatorio en cada máquina y cada CI.
- Acopla versiones: cada upgrade puede romper el driver.

Si algún día "sin dependencias" deja de ser un requisito duro, **C es la respuesta
correcta** y este documento debe revisarse.

### Por qué se descarta E (cloud)

Coste recurrente, latencia y exige exponer la app en desarrollo hacia fuera
(túnel). Desproporcionado para una herramienta local. Sigue siendo válido para
una verificación puntual antes de un release.

---

## 5. Estrategia recomendada para WebKit real

> **Sólo tiene sentido abordarla cuando §3.1 esté cerrado y sigan apareciendo
> diferencias.** Esas diferencias son a la vez la justificación y el set de
> pruebas de este trabajo.

### 5.1 La decisión clave: W3C WebDriver, no un protocolo propietario

**WebDriver es HTTP + JSON.** Un cliente mínimo se escribe con `net/http` y el
`tinywasm/json` que el repo ya usa: **cero dependencias nuevas**, coherente con la
filosofía de vendorizar.

Y el mismo cliente sirve para dos motores:

- **WebKitGTK** (`WebKitWebDriver`, paquete `webkit2gtk-driver`) → WebCore +
  JavaScriptCore reales en Linux.
- **`safaridriver`** (incluido en macOS, se activa con `safaridriver --enable`) →
  **Safari real**, sin escribir código adicional.

Ése es el argumento decisivo frente a implementar el WebKit Inspector Protocol:
un solo cliente, dos motores, ninguna dependencia.

Para el descubrimiento del binario ya hay patrón en el repo: `ResolveChromeExecPath`
(`execpath.go:15`) valida candidatos con `--version` y admite override por
variable de entorno.

### 5.2 Qué se puede portar y qué no

Arquitectura: una interfaz `Engine` que abstraiga **sólo el transporte** (navegar,
evaluar, capturar, interactuar), con dos implementaciones. Toda la lógica basada
en JS inyectado se comparte entre motores.

| Herramienta MCP | WebDriver | Cómo |
|---|---|---|
| `browser_navigate` | ✅ | `POST /session/{id}/url` |
| `browser_screenshot` / `browser_save_screenshot` | ✅ | `GET /screenshot`, `GET /element/{id}/screenshot` |
| `browser_click_element` | ✅ | `POST /element/{id}/click` |
| `browser_fill_element` | ✅ | `POST /element/{id}/value` |
| `browser_swipe_element` | ✅ | Actions API (pointer type `touch`) |
| `browser_evaluate_js` | ✅ | `POST /execute/sync` |
| `browser_get_source` | ✅ | `GET /source` |
| `browser_get_content`, `browser_get_structure` | ✅ | ya se resuelven por JS |
| `browser_get_performance` | ✅ | ya es JS puro (`mcp-performance.go`) |
| `browser_get_styles` | ✅ | reescribir sobre `document.styleSheets` |
| `browser_get_storage` | ✅ | vía JS |
| `browser_inspect_element` | ⚠️ | reimplementar por JS |
| `browser_get_asset` | ⚠️ | `fetch()` desde la página en vez de `Runtime` |
| `browser_get_console`, `browser_get_errors` | ⚠️ | **shim JS** (parchear `console.*` y `window.onerror` en cada carga) en vez de eventos `Runtime` |
| `browser_emulate_device` | ❌ | WebDriver **no tiene emulación de dispositivo**; sólo `Set Window Rect` |
| `browser_get_network_logs` | ❌ | no hay dominio `Network`; requeriría un proxy propio |
| `browser_intercept_request` | ❌ | no hay dominio `Fetch` |

**~13 de 19 herramientas son portables**, 3 con trabajo extra y 3 no. Es aceptable
si el propósito del motor WebKit está acotado: *verificar layout y semántica JS*,
no *depurar red*. Las no soportadas deben devolver un error explícito del tipo
`"browser_get_network_logs no está disponible en el motor webkit; usa el motor
chrome"` — nunca fallar en silencio.

### 5.3 La limitación que hay que aceptar por escrito

WebKitGTK da el **motor**, no el **dispositivo**: no hay DPR 3, ni safe areas, ni
San Francisco, ni controles de iOS. Y `safaridriver` da Safari **de escritorio**;
Safari iOS no es automatizable con él.

Confundir "mi CSS funciona en WebCore" con "se ve igual que en mi iPhone" es el
mayor riesgo de toda esta línea de trabajo.

---

## 6. La verdad de terreno

Nada de lo anterior sustituye al dispositivo real:

- **Depuración remota:** iPhone por USB a un Mac → Safari → *Desarrollo* → tu
  iPhone. Inspector web completo sobre la página real. Es la única forma de ver a
  la vez el motor real, las fuentes reales, los safe areas reales y el viewport
  dinámico real.
- Sin Mac no hay alternativa fiel: el Simulador de iOS también exige macOS. Lo más
  cercano en Linux sigue siendo WebKitGTK.
- Automatizar el iPhone exigiría `ios_webkit_debug_proxy` (expone el protocolo de
  inspección de WebKit del dispositivo) o Appium/XCUITest. Ambos quedan **fuera
  del alcance** de esta librería: son cadenas de herramientas propias de macOS y
  no encajan con un binario Go autocontenido.

---

## 7. Riesgos y no-objetivos

**Riesgos**

| Riesgo | Mitigación |
|---|---|
| Duplicar la lógica de las herramientas en dos backends | Que `Engine` abstraiga sólo transporte; toda la lógica de JS inyectado, compartida |
| Falsa seguridad ("pasó en WebKitGTK, luego pasa en iPhone") | Documentar en el README qué cubre cada motor; nunca llamarlo "modo Safari" ni "modo iPhone" |
| WebKitGTK va por detrás del WebKit de Safari | Registrar la versión del driver en los reportes; el veredicto final lo da el iPhone real |
| Regresión en el camino Chrome, que es el de uso diario | La refactorización a `Engine` no debe cambiar el comportamiento de Chrome; los tests de `tests/` deben pasar sin modificarse |

**No-objetivos explícitos**

- No se implementará el WebKit Inspector Protocol.
- No se añadirán Node, Playwright ni Selenium al árbol de dependencias.
- No se soportará automatización de Safari iOS.
- No se hará *UA spoofing* presentándolo como soporte de Safari.
- `devbrowser` no inyectará etiquetas HTML ni CSS en la app del usuario: eso es de
  las librerías listadas en §3.
