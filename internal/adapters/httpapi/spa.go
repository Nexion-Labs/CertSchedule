package httpapi

import (
	"io/fs"
	"net/http"
)

// WithSPA wraps the API handler so that any request not matched by an API
// route falls through to serving the built React app (with SPA fallback to
// index.html for client-side routes), from the given filesystem — typically
// an embed.FS rooted at web/dist populated by go:embed in cmd/server.
func WithSPA(api http.Handler, staticFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(staticFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIRoute(r.URL.Path) {
			api.ServeHTTP(w, r)
			return
		}

		if _, err := fs.Stat(staticFS, cleanPath(r.URL.Path)); err != nil {
			// Not a real static asset: fall back to index.html so client-side
			// routing (React Router) can take over.
			r2 := new(http.Request)
			*r2 = *r
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func isAPIRoute(path string) bool {
	return len(path) >= 8 && path[:8] == "/api/v1/" || path == "/healthz"
}

func cleanPath(p string) string {
	if p == "" || p == "/" {
		return "."
	}
	if p[0] == '/' {
		return p[1:]
	}
	return p
}
