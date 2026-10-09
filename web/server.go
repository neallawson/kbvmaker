// Package web serves the Montage HTTP API and, when a build is present, the client.
package web

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"neal/kbvmaker/v1/media"
	apiv1 "neal/kbvmaker/web/v1"
)

// Server is the local browser host. Static may be nil when only the API is mounted.
type Server struct {
	Probe  media.Probe
	Static fs.FS
}

// Handler is the shared router for the browser process and the Wails window.
func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	apiv1.Handler{Probe: s.Probe}.Register(mux)
	if s.Static != nil {
		mux.Handle("/", spa(s.Static))
	}
	return mux
}

// spa serves a static client and falls back to index.html for app routes.
func spa(root fs.FS) http.Handler {
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		f, err := root.Open(name)
		if err != nil {
			serveIndex(w, root)
			return
		}
		st, err := f.Stat()
		_ = f.Close()
		if err != nil || st.IsDir() {
			serveIndex(w, root)
			return
		}
		files.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, root fs.FS) {
	f, err := root.Open("index.html")
	if err != nil {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, f)
}
