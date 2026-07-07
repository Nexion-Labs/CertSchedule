// Package webui embeds the built React frontend so the Go binary can serve
// it directly (single binary, single container, single k8s Deployment).
//
// The dist/ directory here is a build artifact: `make web-build` (or the
// Docker build stage) compiles web/ with Vite and copies its output into
// internal/webui/dist before `go build`. A placeholder index.html is
// committed so the package still compiles (go:embed requires at least one
// matching file) before the frontend has ever been built.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS serves the built frontend's static files, rooted so that paths match
// exactly what the browser requests (e.g. "index.html", "assets/app.js").
var FS fs.FS

func init() {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	FS = sub
}
