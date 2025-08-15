package devbrowser

import (
	"fmt"
	"time"
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

		// Navegar a la URL
		_, err = h.page.Goto(url)
		if err != nil {
			h.errChan <- fmt.Errorf("error navigating to %s: %v", url, err)
			return
		}

		// Verificar carga completa usando Playwright
		err = h.page.WaitForLoadState()
		if err != nil {
			h.errChan <- err
			return
		}

		// Esperar un momento adicional para asegurar que todo esté cargado
		time.Sleep(100 * time.Millisecond)
		h.readyChan <- true
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
