// Package aivotetracker exists at the module root so that the static frontend
// in web/ can be embedded into the server binary.
package aivotetracker

import (
	"embed"
	"io/fs"
)

//go:embed all:web
var webFiles embed.FS

// Web returns the static site rooted at web/.
func Web() (fs.FS, error) {
	return fs.Sub(webFiles, "web")
}
