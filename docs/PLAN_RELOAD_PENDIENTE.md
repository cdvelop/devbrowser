---
PLAN: "fix: una recarga pedida mientras el navegador abre ya no se pierde"
EXECUTOR: jules
REVIEWER: none
---

> Plan autocontenido: todo lo necesario para ejecutarlo está aquí.
> Se despacha con el flujo CodeJob. Ver skill: agents-workflow.
>
> Este plan se ejecuta como parte de la cola en `docs/PLAN.md` (orden 1 de 2),
> junto con `docs/PLAN_WINDOW_AUTOFIT.md`. Ambos son independientes entre sí:
> no mezclar cambios de uno en el otro.

# Plan — la recarga que llega temprano se encola, no se tira

## 1. El defecto

`devbrowser.go`, `func (b *DevBrowser) Reload()`:

```go
b.Mu.Lock()
ready := b.ready && b.Ctx != nil && b.IsOpenFlag
b.Mu.Unlock()

if !ready {
	b.Logger("Reload skipped: browser still opening")
	return nil                 // ← la recarga se pierde para siempre
}
```

La guarda es **correcta y necesaria**: su comentario explica por qué existe —
correr `chromedp.Run` sobre un contexto todavía sin asignar levantaba un
**segundo** Chrome (la famosa "doble ventana" en `about:blank`). Eso no se toca.

Lo que está mal es lo que hace después: descarta la petición y devuelve `nil`,
como si hubiera recargado.

El comentario asume que la recarga es redundante:

> *"El Navigate inicial ya muestra el contenido actual, así que una recarga
> antes de ready es redundante"*

**Ese supuesto es falso en el caso que importa.** El demonio construye el sitio
en dos tiempos: primero el módulo raíz (síncrono) y luego el grafo completo de
dependencias (asíncrono, hasta 30 s). Si el escaneo aterriza mientras el
navegador se está abriendo, el `Navigate` inicial mostró el sitio **incompleto**
—sin el CSS ni los iconos de ninguna dependencia— y la recarga que lo
completaría es exactamente la que se tira.

Evidencia, arranque real del demonio sobre un proyecto:

```
22:42:03  SERVER   Starting Internal Server on port: 8080
22:42:03  WATCH    SSR: escaneo completo — 4 módulos
22:42:03  BROWSER  Reload skipped: browser still opening
22:42:05  BROWSER  Open | Auto-Start: on | Shortcut B
```

El usuario lo describe como *"nunca se actualiza"*: la página se queda a medio
vestir hasta que recarga a mano.

## 2. El cambio

Encolar la recarga en vez de descartarla. Es un booleano, no una cola: N
recargas pendientes se resuelven con **una** recarga.

### 2.1 · Campo nuevo

```go
// pendingReload recuerda que alguien pidió recargar mientras el navegador
// todavía se estaba abriendo. Descartar esa petición dejaba en pantalla el
// contenido de la primera pintada —que puede ser un sitio a medio construir—
// hasta que un humano recargaba a mano.
pendingReload bool
```

Protegido por `b.Mu`, como el resto del estado.

### 2.2 · `Reload()` marca en vez de tirar

```go
b.Mu.Lock()
ready := b.ready && b.Ctx != nil && b.IsOpenFlag
if !ready {
	// Solo se encola si el navegador está EN CAMINO. Con el navegador
	// cerrado no hay nada que recargar y encolarlo provocaría una recarga
	// sorpresa la próxima vez que se abra.
	if b.IsOpenFlag {
		b.pendingReload = true
	}
	b.Mu.Unlock()
	b.Logger("Reload pendiente: el navegador aún se está abriendo")
	return nil
}
b.Mu.Unlock()
```

El mensaje **cambia de texto a propósito**: "skipped" describía una pérdida;
"pendiente" describe lo que ahora pasa de verdad.

### 2.3 · Consumirla al quedar listo

En `OpenBrowser.go`, justo donde hoy se marca `h.ready = true` (después del
`Navigate` inicial y de aplicar la emulación de dispositivo, antes de
`h.ReadyChan <- true`):

```go
h.Mu.Lock()
h.ready = true
pending := h.pendingReload
h.pendingReload = false
h.Mu.Unlock()

if pending {
	h.Logger("Aplicando la recarga pendiente")
	if err := h.Reload(); err != nil {
		h.Logger("Recarga pendiente fallida:", err)
	}
}
```

Ojo al orden: `h.ready` ya es `true` cuando se llama a `Reload()`, así que entra
por el camino normal y no se vuelve a encolar.

### 2.4 · Limpiar al cerrar

En `CloseBrowser` (y en el camino de `RestartBrowser`), poner
`b.pendingReload = false`. Una recarga pendiente de una sesión anterior no debe
dispararse en la siguiente.

## 3. Lo que NO se toca

- **La guarda de `ready`**: sigue exactamente igual. Es lo que impide la segunda
  ventana de Chrome. Solo cambia qué se hace cuando salta.
- **`NavigateToURL`, `OpenBrowser` en lo demás, `monitorBrowserClose`,
  `monitorBrowserGeometry`**: sin cambios.
- **Nada de `docs/PLAN.md`** (plan de emulación, en revisión con PR abierta).

## 4. Tests

En `tests/`, con `TestMode` activo cuando haga falta evitar levantar Chrome de
verdad.

| Test | Qué prueba |
|---|---|
| `TestRecargaPedidaMientrasAbreSeAplicaAlQuedarListo` | `Reload()` antes de `ready` → tras `ready` se ejecuta **una** recarga |
| `TestVariasRecargasPendientesSonUnaSola` | tres `Reload()` seguidos antes de `ready` → una sola recarga |
| `TestSinNavegadorAbiertoNoSeEncolaNada` | `IsOpenFlag == false` → `pendingReload` sigue `false` |
| `TestCerrarLimpiaLaRecargaPendiente` | pendiente + `CloseBrowser()` → no se dispara al reabrir |

El primero es el que hoy falla.

## 5. Criterios de aceptación

| # | Comprobación | Esperado |
|---|---|---|
| 1 | `go test ./...` | verde |
| 2 | `grep -rn "Reload skipped" .` | vacío (el texto cambió) |
| 3 | `grep -n "pendingReload" devbrowser.go OpenBrowser.go CloseBrowser.go` | declarado, marcado, consumido y limpiado |
| 4 | Arranque real con el escaneo aterrizando durante la apertura | la página termina **completa**, sin recargar a mano |
| 5 | Ventanas de Chrome tras el arranque | **una** (la guarda sigue cumpliendo su función) |

## 6. Etapas

| # | Etapa | Archivos |
|---|---|---|
| 1 | Campo `pendingReload` | `devbrowser.go` |
| 2 | `Reload()` marca en vez de descartar, y cambia el mensaje | `devbrowser.go` |
| 3 | Consumir la pendiente al quedar listo | `OpenBrowser.go` |
| 4 | Limpiar al cerrar/reiniciar | `CloseBrowser.go`, `devbrowser.go` |
| 5 | Tests | `tests/` |
