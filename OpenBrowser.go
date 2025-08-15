package devbrowser

import (
	"context"
	"fmt"

	"github.com/chromedp/chromedp"
)

func (h *DevBrowser) OpenBrowser() {
	if h.isOpen {
		return
	}

	// Add listener for exit signal
	go func() {
		<-h.exitChan
		h.CloseBrowser()
	}()
	// fmt.Println("*** START DEV BROWSER ***")
	go func() {
		err := h.CreateBrowserContext()
		if err != nil {
			h.errChan <- err
			return
		}

		h.isOpen = true
		var protocol = "http"
		url := protocol + `://localhost:` + h.config.GetServerPort() + "/"

		err = chromedp.Run(h.Context, h.sendkeys(url))
		if err != nil {
			h.errChan <- fmt.Errorf("error navigating to %s: %v", url, err)
			return
		}

		// Verificar carga completa
		err = chromedp.Run(h.Context, chromedp.ActionFunc(func(ctx context.Context) error {
			for {
				var readyState string
				select {

				case <-ctx.Done():
					return ctx.Err()
				default:
					err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(`document.readyState`, &readyState))
					if err != nil {
						return err
					}

					if readyState == "complete" {
						h.readyChan <- true
						return nil
					}
				}
			}
		}))

		if err != nil {
			h.errChan <- err
		}
	}()

	// Esperar señal de inicio o error
	select {
	case err := <-h.errChan:
		h.isOpen = false
		h.logger.Write([]byte("Error opening DevBrowser: " + err.Error()))
		return
	case <-h.readyChan:
		// Tomar el foco de la UI después de abrir el navegador
		/* 	err := h.ui.ReturnFocus()
		if err != nil {
			h.logger.Write([]byte("Error returning focus to UI: " + err.Error()))
		} */
		return
	}
}
