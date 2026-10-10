package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

// API carries /v1 requests from the window to the HTTP handler.
// WebKitGTK before 2.40 drops POST bodies on the Wails asset server, so the
// window cannot use fetch for /v1.
type API struct {
	handler http.Handler
}

// Reply is the HTTP status and body the handler wrote.
type Reply struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// Call runs one /v1 request.
func (a *API) Call(method, path, body string) (Reply, error) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return Reply{Status: rec.Code, Body: rec.Body.String()}, nil
}
