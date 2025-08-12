package devbrowser

func (h *DevBrowser) Name() string {
	return "BROWSER"
}

func (h *DevBrowser) Label() string {

	state := "Open Browser"

	if h.isOpen {
		state = "Close Browser"
	}

	return state
}

func (h *DevBrowser) Execute(progress func(msgs ...any)) {

	if h.isOpen { // cerrar si esta abierto
		progress("Closing DevBrowser...")

		if err := h.CloseBrowser(); err != nil {
			progress("CloseBrowser error:", err)
		} else {
			progress("DevBrowser closed successfully.")
		}

	} else { // abrir si esta cerrado
		progress("Opening DevBrowser...")
		// Simulate opening process
		if err := h.OpenBrowser(); err != nil {
			progress("OpenBrowser error:", err)
		} else {
			progress("DevBrowser opened successfully.")
		}
	}

}
