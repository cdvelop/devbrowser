---
PLAN: "fix: recarga pendiente durante apertura + ventana que crece para caber la emulación de dispositivo"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 16436170586368041481
PR: https://github.com/tinywasm/devbrowser/pull/12
---

# PLAN — cola de ejecución para `devbrowser`

> Si te dijeron "ejecuta el plan descrito en docs/PLAN.md", ejecuta **TODOS
> los planes de abajo, en orden (de arriba hacia abajo)**. Cada plan es
> autocontenido; termina uno (sus criterios de aceptación en verde) antes de
> empezar el siguiente. Nunca mezcles cambios de un plan en el otro — ambos
> tocan `OpenBrowser.go`, pero en bloques distintos (ver cada plan).

| Orden | Plan | Tema |
|-------|------|------|
| 1 | [PLAN_RELOAD_PENDIENTE.md](PLAN_RELOAD_PENDIENTE.md) | Una recarga pedida mientras el navegador abre ya no se pierde: se encola y se aplica al quedar listo |
| 2 | [PLAN_WINDOW_AUTOFIT.md](PLAN_WINDOW_AUTOFIT.md) | La ventana física crece en vivo para caber el viewport que pide `browser_emulate_device` + el espacio de DevTools, para que el dev humano y el agente MCP vean lo mismo |

Después de completar ambos planes, corre `gotest ./...` una vez más: todo en
verde.
