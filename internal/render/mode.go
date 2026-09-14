package render

import (
	"os"

	"golang.org/x/term"
)

// Mode selects how documents are formatted.
type Mode int

const (
	// Pretty is the coloured, decorated terminal format.
	Pretty Mode = iota
	// Plain is the stable, undecorated format for pipes and scripts.
	Plain
	// JSON is the machine-readable format.
	JSON
)

// DetectMode decides the output mode from explicit flags, the NO_COLOR
// convention and whether stdout is a terminal. jsonFlag takes priority over
// plainFlag; absent either, Pretty is used only on an interactive, colour
// terminal.
func DetectMode(stdout *os.File, plainFlag, jsonFlag bool) Mode {
	switch {
	case jsonFlag:
		return JSON
	case plainFlag:
		return Plain
	case os.Getenv("NO_COLOR") != "":
		return Plain
	case stdout != nil && term.IsTerminal(int(stdout.Fd())):
		return Pretty
	default:
		return Plain
	}
}
