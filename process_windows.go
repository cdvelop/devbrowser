//go:build windows

package devbrowser

import (
	"os"
)

func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	return err == nil
}
