//go:build gui

package main

import (
	"os"

	"github.com/dostrow/e9s/internal/gui"
)

func main() {
	os.Exit(gui.Run(os.Args))
}
