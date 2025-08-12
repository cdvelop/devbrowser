package devbrowser

import (
	"context"
	"errors"

	"github.com/chromedp/chromedp"
)

type DevBrowser struct {
	config   serverConfig
	ui       userInterface
	width    int    // ej "800" default "1024"
	height   int    //ej: "600" default "768"
	position string //ej: "1930,0" (when you have second monitor) default: "0,0"

	isOpen bool // Indica si el navegador está abierto

	context.Context    // Este campo no se codificará en YAML
	context.CancelFunc // Este campo no se codificará en YAML

	readyChan chan bool
	errChan   chan error
	exitChan  chan bool
}

type serverConfig interface {
	GetServerPort() string
}

type userInterface interface {
	ReturnFocus() error
}

/*
devbrowser.New creates a new DevBrowser instance.

	type serverConfig interface {
		GetServerPort() string
	}

	type userInterface interface {
		ReturnFocus() error
	}

	example :  New(serverConfig, userInterface, exitChan)
*/
func New(sc serverConfig, ui userInterface, exitChan chan bool) *DevBrowser {

	browser := &DevBrowser{
		config:    sc,
		ui:        ui,
		width:     1024,  // Default width
		height:    768,   // Default height
		position:  "0,0", // Default position
		readyChan: make(chan bool),
		errChan:   make(chan error),
		exitChan:  exitChan,
	}
	return browser
}

func (h *DevBrowser) BrowserStartUrlChanged(fieldName string, oldValue, newValue string) error {

	if !h.isOpen {
		return nil
	}

	return h.RestartBrowser()
}

func (h *DevBrowser) RestartBrowser() error {

	this := errors.New("RestartBrowser")

	err := h.CloseBrowser()
	if err != nil {
		return errors.Join(this, err)
	}

	return h.OpenBrowser()
}

func (b DevBrowser) sendkeys(host string) chromedp.Tasks {

	return chromedp.Tasks{
		chromedp.Navigate(host),
	}
}

func (b *DevBrowser) Reload() (err error) {
	if b.Context != nil && b.isOpen {
		// fmt.Println("Recargando Navegador")
		err = chromedp.Run(b.Context, chromedp.Reload())
		if err != nil {
			return errors.New("Reload DevBrowser " + err.Error())
		}
	}
	return
}
