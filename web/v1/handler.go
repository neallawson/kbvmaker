// Package v1 is the HTTP API for Montage 1.
// A later Montage is a sibling package. This one imports only neal/kbvmaker/v1.
package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	engine "neal/kbvmaker/v1"
	"neal/kbvmaker/v1/media"
	"neal/kbvmaker/v1/rule"
)

// Handler serves /v1 for one probe. The request names the document.
// There is no session and no global project.
type Handler struct {
	Probe media.Probe
}

// Register mounts the Montage 1 routes on mux.
func (h Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/version", h.version)
	mux.HandleFunc("POST /v1/plan", h.plan)
}

func (h Handler) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]int{"montage": engine.Version})
}

type planRequest struct {
	Document string `json:"document"`
}

func (h Handler) plan(w http.ResponseWriter, r *http.Request) {
	var req planRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeRule(w, http.StatusBadRequest, rule.E(0, "bad-number", "request body"))
		return
	}
	if req.Document == "" {
		writeRule(w, http.StatusBadRequest, rule.E(0, "missing-field", "document"))
		return
	}
	d, err := engine.Load(req.Document)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeRule(w, http.StatusUnprocessableEntity, rule.E(0, "missing-media", "document %s", req.Document))
			return
		}
		writeRule(w, http.StatusUnprocessableEntity, err)
		return
	}
	p, err := engine.New(h.Probe).Resolve(r.Context(), d)
	if err != nil {
		writeRule(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
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
