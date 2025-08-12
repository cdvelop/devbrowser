package devbrowser

import (
	"context"
	"errors"

	"github.com/chromedp/chromedp"
)

func (h *DevBrowser) CloseBrowser() error {
	if !h.isOpen {
		return errors.New("DevBrowser is already closed")
	}

	// Primero cerrar todas las pestañas/contextos
	if err := chromedp.Run(h.Context, chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			return nil
		}),
	}); err != nil {
		return err
	}

	// Luego cancelar el contexto principal
	if h.CancelFunc != nil {
		h.CancelFunc()
		h.isOpen = false
	}

	// Limpiar recursos
	h.Context = nil
	h.CancelFunc = nil

	return nil
}
