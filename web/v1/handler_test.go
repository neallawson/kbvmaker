package v1_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neal/kbvmaker/v1/media"
	webv1 "neal/kbvmaker/web/v1"
)

func TestVersion(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	mux := http.NewServeMux()
	webv1.Handler{Probe: &media.Fake{}}.Register(mux)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"montage":1`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestPlan(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "piece.montage.json")
	body := `{
	  "montage": 1,
	  "audio": "song.mp3",
	  "slots": [{"src": "a.jpg", "mode": "hold"}]
	}`
	if err := os.WriteFile(docPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "song.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	probe := &media.Fake{
		Durations: map[string]float64{filepath.Join(dir, "song.mp3"): 4},
		Images:    map[string]media.Info{filepath.Join(dir, "a.jpg"): {Width: 3000, Height: 4000}},
	}
	mux := http.NewServeMux()
	webv1.Handler{Probe: probe}.Register(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/plan", strings.NewReader(`{"document":"`+docPath+`"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"duration":4`) || !strings.Contains(rec.Body.String(), `"src":"a.jpg"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestAudioDuration(t *testing.T) {
	mux := http.NewServeMux()
	webv1.Handler{Probe: &media.Fake{Durations: map[string]float64{"/music/song.mp3": 222}}}.Register(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/audio", strings.NewReader(`{"path":"/music/song.mp3"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"seconds":222`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestDocumentRoundTripAndPlanBody(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "piece.montage.json")
	probe := &media.Fake{
		Durations: map[string]float64{filepath.Join(dir, "song.mp3"): 20},
		Images: map[string]media.Info{
			filepath.Join(dir, "a.jpg"): {Width: 3000, Height: 4000},
			filepath.Join(dir, "b.jpg"): {Width: 3000, Height: 4000},
		},
	}
	for _, name := range []string{"song.mp3", "a.jpg", "b.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	webv1.Handler{Probe: probe}.Register(mux)

	body := `{
	  "montage": 1,
	  "audio": "song.mp3",
	  "slots": [
	    {"src": "a.jpg", "mode": "hold", "dur": 8},
	    {"src": "b.jpg", "mode": "in-center"}
	  ]
	}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/document", strings.NewReader(`{"document":"`+docPath+`","body":`+body+`}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/document", strings.NewReader(`{"document":"`+docPath+`"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"dur": 8`) {
		t.Fatalf("read %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/plan", strings.NewReader(`{"dir":"`+dir+`","body":`+body+`}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"clipDur":8`) {
		t.Fatalf("plan %d %s", rec.Code, rec.Body.String())
	}
}

func TestThumbnailReturnsTheRequestedJPEG(t *testing.T) {
	mux := http.NewServeMux()
	webv1.Handler{
		Probe: &media.Fake{},
		Thumb: func(_ context.Context, path string, w, h int) ([]byte, error) {
			if path != "/pics/a.jpg" || w != 32 || h != 18 {
				t.Fatalf("thumb %s %dx%d", path, w, h)
			}
			return []byte("jpeg-bytes"), nil
		},
	}.Register(mux)
	rec := post(mux, "/v1/thumbnail", `{"path":"/pics/a.jpg","width":32,"height":18}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Type   string `json:"type"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
		Data   string `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(got.Data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "image/jpeg" || got.Width != 32 || got.Height != 18 || string(raw) != "jpeg-bytes" {
		t.Fatalf("reply %+v decoded %q", got, raw)
	}
}

func TestThumbnailRejectsABadSize(t *testing.T) {
	mux := http.NewServeMux()
	webv1.Handler{Probe: &media.Fake{}, Thumb: func(context.Context, string, int, int) ([]byte, error) {
		t.Fatal("thumbnailer should not run")
		return nil, nil
	}}.Register(mux)
	for _, body := range []string{
		`{"path":"/pics/a.jpg","width":0,"height":18}`,
		`{"path":"/pics/a.jpg","width":8000,"height":18}`,
		`{"width":32,"height":18}`,
	} {
		rec := post(mux, "/v1/thumbnail", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status %d %s", body, rec.Code, rec.Body.String())
		}
	}
}

func TestThumbnailReportsAMissingPicture(t *testing.T) {
	mux := http.NewServeMux()
	webv1.Handler{Probe: &media.Fake{}, Thumb: func(context.Context, string, int, int) ([]byte, error) {
		return nil, errors.New("open a.jpg: no such file")
	}}.Register(mux)
	rec := post(mux, "/v1/thumbnail", `{"path":"/pics/a.jpg","width":32,"height":18}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"code":"missing-media"`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func post(mux *http.ServeMux, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	mux.ServeHTTP(rec, req)
	return rec
}

func TestPlanRejectsAMissingDocument(t *testing.T) {
	mux := http.NewServeMux()
	webv1.Handler{Probe: &media.Fake{}}.Register(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/plan", strings.NewReader(`{"document":"/no/such.montage.json"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"missing-media"`) || !strings.Contains(rec.Body.String(), `"slot":0`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}
