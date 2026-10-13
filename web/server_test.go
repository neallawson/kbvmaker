package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"kbvmaker/v1/media"
	"kbvmaker/web"
)

func TestStaticFallsBackToIndex(t *testing.T) {
	files := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<p>kbvmaker</p>")},
	}
	h := web.Server{Probe: &media.Fake{}, Static: files}.Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/edit", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "<p>kbvmaker</p>" {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/version", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("version status %d body %s", rec.Code, rec.Body.String())
	}
}
