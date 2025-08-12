package devbrowser

import (
	"errors"
	"strconv"
	"strings"
)

func (h *DevBrowser) BrowserPositionAndSizeChanged(fieldName string, oldValue, newValue string) error {

	if !h.isOpen {
		return nil
	}

	err := h.setBrowserPositionAndSize(newValue)
	if err != nil {
		return err
	}

	return h.RestartBrowser()
}

func (b *DevBrowser) setBrowserPositionAndSize(newConfig string) (err error) {

	this := errors.New("setBrowserPositionAndSize")

	position, width, height, err := getBrowserPositionAndSize(newConfig)

	if err != nil {
		return errors.Join(this, err)
	}
	b.position = position

	b.width, err = strconv.Atoi(width)
	if err != nil {
		return errors.Join(this, err)
	}
	b.height, err = strconv.Atoi(height)
	if err != nil {
		return errors.Join(this, err)
	}

	return
}

func getBrowserPositionAndSize(config string) (position, width, height string, err error) {
	current := strings.Split(config, ":")

	if len(current) != 2 {
		err = errors.New("browse config must be in the format: 1930,0:800,600")
		return
	}

	positions := strings.Split(current[0], ",")
	if len(positions) != 2 {
		err = errors.New("position must be with commas e.g.: 1930,0:800,600")
		return
	}
	position = current[0]

	sizes := strings.Split(current[1], ",")
	if len(sizes) != 2 {
		err = errors.New("width and height must be with commas e.g.: 1930,0:800,600")
		return
	}

	widthInt, err := strconv.Atoi(sizes[0])
	if err != nil {
		err = errors.New("width must be an integer number")
		return
	}
	width = strconv.Itoa(widthInt)

	heightInt, err := strconv.Atoi(sizes[1])
	if err != nil {
		err = errors.New("height must be an integer number")
		return
	}
	height = strconv.Itoa(heightInt)

	return
}

func verifyBrowserposition(newConfig string) (err error) {
	_, _, _, err = getBrowserPositionAndSize(newConfig)
	return
}
