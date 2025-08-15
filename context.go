package devbrowser

import (
	"errors"

	"github.com/playwright-community/playwright-go"
)

func (h *DevBrowser) CreateBrowserContext() error {
	// Instalar playwright si no está instalado
	err := playwright.Install()
	if err != nil {
		return errors.New("failed to install Playwright: " + err.Error())
	}

	// Crear instancia de playwright
	h.playwright, err = playwright.Run()
	if err != nil {
		return errors.New("failed to run Playwright: " + err.Error())
	}

	// Configurar argumentos del navegador
	args := []string{
		"--disable-blink-features=WebFontsInterventionV2", // Remove font warning
		"--use-fake-ui-for-media-stream",
		"--no-focus-on-load",
		"--auto-open-devtools-for-tabs",
		"--window-position=" + h.position,
	}

	// Configurar opciones del navegador
	h.browser, err = h.playwright.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false), // Desactivar modo headless
		Args:     args,
	})
	if err != nil {
		return errors.New("failed to launch browser: " + err.Error())
	}

	// Crear contexto del navegador con viewport personalizado
	h.context, err = h.browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  h.width,
			Height: h.height,
		},
	})
	if err != nil {
		return errors.New("failed to create browser context: " + err.Error())
	}

	// Crear nueva página
	h.page, err = h.context.NewPage()
	if err != nil {
		return errors.New("failed to create new page: " + err.Error())
	}

	// Función de cancelación personalizada
	h.cancelFunc = func() {
		if h.page != nil {
			h.page.Close()
		}
		if h.context != nil {
			h.context.Close()
		}
		if h.browser != nil {
			h.browser.Close()
		}
		if h.playwright != nil {
			h.playwright.Stop()
		}
	}

	return nil
}
