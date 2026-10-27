// Package v1 is the HTTP API for Montage 1.
// A later Montage is a sibling package. This one imports only kbvmaker/v1.
package v1

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	"net/http"
	"os"

	engine "kbvmaker/v1"
	"kbvmaker/v1/card"
	"kbvmaker/v1/compile"
	"kbvmaker/v1/doc"
	"kbvmaker/v1/media"
	"kbvmaker/v1/rule"
)

// Handler serves /v1 for one probe. The request names the document.
// There is no session and no global project.
type Handler struct {
	Probe media.Probe
	// Thumb builds a JPEG of exactly width by height. Nil uses ffmpeg.
	Thumb func(ctx context.Context, path string, width, height int) ([]byte, error)
	// ThumbContain fits the whole picture inside the box. Nil uses ffmpeg.
	ThumbContain func(ctx context.Context, path string, width, height int) ([]byte, error)
}

// Register mounts the Montage 1 routes on mux.
func (h Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/version", h.version)
	mux.HandleFunc("POST /v1/audio", h.audio)
	mux.HandleFunc("POST /v1/thumbnail", h.thumbnail)
	mux.HandleFunc("POST /v1/plan", h.plan)
	mux.HandleFunc("POST /v1/document", h.document)
	mux.HandleFunc("POST /v1/export-script", h.exportScript)
	mux.HandleFunc("POST /v1/render", h.render)
}

func (h Handler) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]int{"montage": engine.Version})
}

type audioRequest struct {
	Path string `json:"path"`
}

func (h Handler) audio(w http.ResponseWriter, r *http.Request) {
	var req audioRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	if req.Path == "" {
		writeRule(w, http.StatusBadRequest, rule.E(0, "missing-field", "a soundtrack file is required"))
		return
	}
	seconds, err := h.Probe.Duration(r.Context(), req.Path)
	if err != nil {
		writeRule(w, http.StatusUnprocessableEntity, rule.E(0, "missing-media", "soundtrack file %s could not be read", req.Path))
		return
	}
	if seconds <= 0 {
		writeRule(w, http.StatusUnprocessableEntity, rule.E(0, "bad-number", "the soundtrack length %v seconds cannot be used", seconds))
		return
	}
	writeJSON(w, http.StatusOK, map[string]float64{"seconds": seconds})
}

type thumbRequest struct {
	Path   string    `json:"path"`
	Card   *doc.Card `json:"card"`
	Width  int       `json:"width"`
	Height int       `json:"height"`
	// Fit is cover, the filmstrip crop, or contain, the whole picture.
	Fit string `json:"fit,omitempty"`
}

func (h Handler) thumbnail(w http.ResponseWriter, r *http.Request) {
	var req thumbRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	hasPath := req.Path != ""
	hasCard := req.Card != nil
	if hasPath && hasCard {
		writeRule(w, http.StatusBadRequest, rule.E(0, "missing-field", "a thumbnail takes a picture file or a card, not both"))
		return
	}
	if !hasPath && !hasCard {
		writeRule(w, http.StatusBadRequest, rule.E(0, "missing-field", "a thumbnail needs a picture file or a card"))
		return
	}
	if req.Width < 1 || req.Height < 1 || req.Width > media.ThumbMax || req.Height > media.ThumbMax {
		writeRule(w, http.StatusBadRequest, rule.E(0, "bad-number", "thumbnail size %dx%d must be from 1 to %d pixels on each side", req.Width, req.Height, media.ThumbMax))
		return
	}
	if req.Fit != "" && req.Fit != "cover" && req.Fit != "contain" {
		writeRule(w, http.StatusBadRequest, rule.E(0, "unknown-field", "fit %q is not supported", req.Fit))
		return
	}
	var jpeg []byte
	var err error
	if hasCard {
		if err := req.Card.Finish(0); err != nil {
			writeRule(w, http.StatusBadRequest, err)
			return
		}
		jpeg, err = card.JPEG(*req.Card, req.Width, req.Height)
		if err != nil {
			writeRule(w, http.StatusUnprocessableEntity, rule.E(0, "bad-number", "the card could not be painted"))
			return
		}
	} else {
		makeThumb := h.Thumb
		if req.Fit == "contain" {
			makeThumb = h.ThumbContain
			if makeThumb == nil {
				makeThumb = media.ThumbnailContain
			}
		} else if makeThumb == nil {
			makeThumb = media.Thumbnail
		}
		jpeg, err = makeThumb(r.Context(), req.Path, req.Width, req.Height)
		if err != nil {
			writeRule(w, http.StatusUnprocessableEntity, rule.E(0, "missing-media", "picture %s could not be read", req.Path))
			return
		}
	}
	width, height := req.Width, req.Height
	if cfg, _, cfgErr := image.DecodeConfig(bytes.NewReader(jpeg)); cfgErr == nil {
		width, height = cfg.Width, cfg.Height
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"type":   "image/jpeg",
		"width":  width,
		"height": height,
		"data":   base64.StdEncoding.EncodeToString(jpeg),
	})
}

type planRequest struct {
	Document string          `json:"document"`
	Body     json.RawMessage `json:"body"`
	Dir      string          `json:"dir"`
}

func (h Handler) plan(w http.ResponseWriter, r *http.Request) {
	var req planRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	if len(req.Body) > 0 && string(req.Body) != "null" && req.Document != "" {
		writeRule(w, http.StatusBadRequest, rule.E(0, "unknown-field", "send a file path or a montage, not both"))
		return
	}
	d, ok := h.loadPlan(w, req)
	if !ok {
		return
	}
	p, err := engine.New(h.Probe).Resolve(r.Context(), d)
	if err != nil {
		writeRule(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

type documentRequest struct {
	Document string          `json:"document"`
	Body     json.RawMessage `json:"body"`
}

func (h Handler) document(w http.ResponseWriter, r *http.Request) {
	var req documentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	if req.Document == "" {
		writeRule(w, http.StatusBadRequest, rule.E(0, "missing-field", "document"))
		return
	}
	if len(req.Body) > 0 && string(req.Body) != "null" {
		h.writeDocument(w, req)
		return
	}
	b, err := os.ReadFile(req.Document)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeRule(w, http.StatusUnprocessableEntity, rule.E(0, "missing-media", "the file was not found"))
			return
		}
		writeRule(w, http.StatusUnprocessableEntity, rule.E(0, "bad-number", "%s", err.Error()))
		return
	}
	if _, err := doc.Decode(b); err != nil {
		writeRule(w, http.StatusUnprocessableEntity, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func (h Handler) writeDocument(w http.ResponseWriter, req documentRequest) {
	d, err := doc.Decode(req.Body)
	if err != nil {
		writeRule(w, http.StatusUnprocessableEntity, err)
		return
	}
	if err := d.ValidateShape(); err != nil {
		writeRule(w, http.StatusUnprocessableEntity, err)
		return
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, req.Body, "", "  "); err != nil {
		writeRule(w, http.StatusBadRequest, rule.E(0, "bad-number", "the request is not valid JSON"))
		return
	}
	buf.WriteByte('\n')
	if err := os.WriteFile(req.Document, buf.Bytes(), 0o644); err != nil {
		writeRule(w, http.StatusUnprocessableEntity, rule.E(0, "bad-number", "%s", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type fileRequest struct {
	Document string `json:"document"`
	File     string `json:"file"`
}

func (h Handler) exportScript(w http.ResponseWriter, r *http.Request) {
	var req fileRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	cmds, _, ok := h.commands(w, r, req.Document)
	if !ok {
		return
	}
	script := engine.ExportScript(cmds)
	if req.File == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(script))
		return
	}
	if err := os.WriteFile(req.File, []byte(script), 0o644); err != nil {
		writeRule(w, http.StatusUnprocessableEntity, rule.E(0, "bad-number", "%s", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h Handler) render(w http.ResponseWriter, r *http.Request) {
	var req fileRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	cmds, dir, ok := h.commands(w, r, req.Document)
	if !ok {
		return
	}
	if err := engine.RunWith(r.Context(), dir, cmds, engine.RunOptions{Atomic: true}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"slot": 0, "code": "render", "message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h Handler) loadPlan(w http.ResponseWriter, req planRequest) (*doc.Document, bool) {
	if len(req.Body) > 0 && string(req.Body) != "null" {
		d, err := doc.Decode(req.Body)
		if err != nil {
			writeRule(w, http.StatusUnprocessableEntity, err)
			return nil, false
		}
		d.Dir = req.Dir
		return d, true
	}
	if req.Document == "" {
		writeRule(w, http.StatusBadRequest, rule.E(0, "missing-field", "document"))
		return nil, false
	}
	d, err := engine.Load(req.Document)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeRule(w, http.StatusUnprocessableEntity, rule.E(0, "missing-media", "the file was not found"))
			return nil, false
		}
		writeRule(w, http.StatusUnprocessableEntity, err)
		return nil, false
	}
	return d, true
}

func (h Handler) commands(w http.ResponseWriter, r *http.Request, path string) ([]compile.Command, string, bool) {
	if path == "" {
		writeRule(w, http.StatusBadRequest, rule.E(0, "missing-field", "document"))
		return nil, "", false
	}
	d, err := engine.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeRule(w, http.StatusUnprocessableEntity, rule.E(0, "missing-media", "the file was not found"))
			return nil, "", false
		}
		writeRule(w, http.StatusUnprocessableEntity, err)
		return nil, "", false
	}
	eng := engine.New(h.Probe)
	p, err := eng.Resolve(r.Context(), d)
	if err != nil {
		writeRule(w, http.StatusUnprocessableEntity, err)
		return nil, "", false
	}
	cmds, err := eng.Compile(p)
	if err != nil {
		writeRule(w, http.StatusUnprocessableEntity, err)
		return nil, "", false
	}
	return cmds, d.Dir, true
}

func decodeRequest(w http.ResponseWriter, r *http.Request, dest any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		writeRule(w, http.StatusBadRequest, rule.E(0, "bad-number", "the request is not valid JSON"))
		return false
	}
	return true
}

func writeRule(w http.ResponseWriter, status int, err error) {
	re, ok := rule.As(err)
	if !ok {
		re = rule.E(0, "bad-number", "%s", err.Error())
	}
	writeJSON(w, status, map[string]any{
		"slot":    re.Slot,
		"code":    re.Code,
		"message": re.Msg,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(body)
}
