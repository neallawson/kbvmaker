package doc

import (
	"bytes"
	"encoding/json"
	"math"
	"sort"

	"kbvmaker/v1/rule"
)

func rejectDup(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := walkJSON(dec); err != nil {
		return err
	}
	return nil
}

func walkJSON(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return rule.E(0, "bad-number", "%s", err.Error())
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch d {
	case '{':
		seen := map[string]struct{}{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return rule.E(0, "bad-number", "%s", err.Error())
			}
			k, ok := kt.(string)
			if !ok {
				return rule.E(0, "bad-number", "json object key is not a string")
			}
			if _, dup := seen[k]; dup {
				return rule.E(0, "duplicate-key", "duplicate key %q", k)
			}
			seen[k] = struct{}{}
			if err := walkJSON(dec); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return rule.E(0, "bad-number", "%s", err.Error())
		}
		return nil
	case '[':
		for dec.More() {
			if err := walkJSON(dec); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return rule.E(0, "bad-number", "%s", err.Error())
		}
		return nil
	default:
		return rule.E(0, "bad-number", "unexpected JSON delimiter")
	}
}

func object(b []byte) (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, rule.E(0, "bad-number", "%s", err.Error())
	}
	return raw, nil
}

func allow(raw map[string]json.RawMessage, keys ...string) error {
	ok := map[string]struct{}{}
	for _, k := range keys {
		ok[k] = struct{}{}
	}
	for k := range raw {
		if _, yes := ok[k]; !yes {
			return unsupportedField(k)
		}
	}
	return nil
}

// UnmarshalJSON decodes one of the time-point spellings.
func (t *TimePoint) UnmarshalJSON(b []byte) error {
	raw, err := object(b)
	if err != nil {
		return err
	}
	switch {
	case has(raw, "sec"):
		if err := allow(raw, "sec", "of"); err != nil {
			return err
		}
		if _, ok := raw["of"]; !ok {
			return rule.E(0, "missing-field", "sec requires of")
		}
		t.Kind = "sec"
		t.Sec, err = num(raw["sec"])
		if err != nil {
			return err
		}
		t.Of, err = str(raw["of"])
		return err
	case has(raw, "pct"):
		if err := allow(raw, "pct", "of"); err != nil {
			return err
		}
		if _, ok := raw["of"]; !ok {
			return rule.E(0, "missing-field", "pct requires of")
		}
		t.Kind = "pct"
		t.Pct, err = num(raw["pct"])
		if err != nil {
			return err
		}
		if t.Pct < 0 || t.Pct > 100 {
			return rule.E(0, "bad-time", "percent %v", t.Pct)
		}
		t.Of, err = str(raw["of"])
		return err
	case has(raw, "timecode"):
		if err := allow(raw, "timecode"); err != nil {
			return err
		}
		t.Kind = "timecode"
		t.Timecode, err = str(raw["timecode"])
		return err
	case has(raw, "frame"):
		if err := allow(raw, "frame"); err != nil {
			return err
		}
		t.Kind = "frame"
		n, err := num(raw["frame"])
		if err != nil {
			return err
		}
		if n < 0 || n != float64(int(n)) {
			return rule.E(0, "bad-time", "frame %v", n)
		}
		t.Frame = int(n)
		return nil
	case has(raw, "anchor"):
		if err := allow(raw, "anchor", "offset"); err != nil {
			return err
		}
		t.Kind = "anchor"
		t.Anchor, err = str(raw["anchor"])
		if err != nil {
			return err
		}
		if o, ok := raw["offset"]; ok {
			t.Offset, err = num(o)
			if err != nil {
				return err
			}
			t.HasOffset = true
		}
		return nil
	default:
		return rule.E(0, "bad-time", "time point needs sec, pct, timecode, frame, or anchor")
	}
}

// MarshalJSON writes the spelling this point was parsed as.
func (t TimePoint) MarshalJSON() ([]byte, error) {
	switch t.Kind {
	case "sec":
		return json.Marshal(struct {
			Sec float64 `json:"sec"`
			Of  string  `json:"of"`
		}{t.Sec, t.Of})
	case "pct":
		return json.Marshal(struct {
			Pct float64 `json:"pct"`
			Of  string  `json:"of"`
		}{t.Pct, t.Of})
	case "timecode":
		return json.Marshal(struct {
			Timecode string `json:"timecode"`
		}{t.Timecode})
	case "frame":
		return json.Marshal(struct {
			Frame int `json:"frame"`
		}{t.Frame})
	case "anchor":
		if t.HasOffset {
			return json.Marshal(struct {
				Anchor string  `json:"anchor"`
				Offset float64 `json:"offset"`
			}{t.Anchor, t.Offset})
		}
		return json.Marshal(struct {
			Anchor string `json:"anchor"`
		}{t.Anchor})
	default:
		return nil, rule.E(0, "bad-time", "cannot encode time kind %q", t.Kind)
	}
}

// UnmarshalJSON decodes a segment. dur may be a number or "rest".
func (s *Segment) UnmarshalJSON(b []byte) error {
	raw, err := object(b)
	if err != nil {
		return err
	}
	if err := allow(raw, "from", "to", "dur", "zoom", "pan", "ease"); err != nil {
		return err
	}
	if v, ok := raw["from"]; ok {
		s.From = &TimePoint{}
		if err := s.From.UnmarshalJSON(v); err != nil {
			return err
		}
	}
	if v, ok := raw["to"]; ok {
		s.To = &TimePoint{}
		if err := s.To.UnmarshalJSON(v); err != nil {
			return err
		}
	}
	if v, ok := raw["dur"]; ok {
		s.HasDur = true
		var asString string
		if err := json.Unmarshal(v, &asString); err == nil {
			if asString != "rest" {
				return rule.E(0, "bad-time", "dur %q", asString)
			}
			s.DurRest = true
		} else {
			s.DurSeconds, err = num(v)
			if err != nil {
				return err
			}
		}
	}
	if v, ok := raw["zoom"]; ok {
		s.Zoom = &ZoomSpec{}
		if err := s.Zoom.UnmarshalJSON(v); err != nil {
			return err
		}
	}
	if v, ok := raw["pan"]; ok {
		s.Pan = &PanSpec{}
		if err := s.Pan.UnmarshalJSON(v); err != nil {
			return err
		}
	}
	if v, ok := raw["ease"]; ok {
		s.Ease, err = str(v)
		if err != nil {
			return err
		}
	}
	return nil
}

// MarshalJSON writes a segment in authoring form.
func (s Segment) MarshalJSON() ([]byte, error) {
	m := map[string]any{}
	if s.From != nil {
		m["from"] = s.From
	}
	if s.To != nil {
		m["to"] = s.To
	}
	if s.HasDur {
		if s.DurRest {
			m["dur"] = "rest"
		} else {
			m["dur"] = s.DurSeconds
		}
	}
	if s.Zoom != nil {
		m["zoom"] = s.Zoom
	}
	if s.Pan != nil {
		m["pan"] = s.Pan
	}
	if s.Ease != "" {
		m["ease"] = s.Ease
	}
	return marshalOrdered(m, "from", "to", "dur", "zoom", "pan", "ease")
}

// UnmarshalJSON decodes a constant zoom, a min, or a from/to ramp.
func (z *ZoomSpec) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return rule.E(0, "bad-number", "zoom is null")
	}
	switch b[0] {
	case '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return rule.E(0, "bad-number", "%s", err.Error())
		}
		z.End = &ZoomEnd{Atoms: []ZoomAtom{Sym(s)}}
		return nil
	case '{':
		raw, err := object(b)
		if err != nil {
			return err
		}
		if _, ok := raw["min"]; ok {
			if err := allow(raw, "min"); err != nil {
				return err
			}
			atoms, err := zoomAtoms(raw["min"])
			if err != nil {
				return err
			}
			z.End = &ZoomEnd{Min: true, Atoms: atoms}
			return nil
		}
		if err := allow(raw, "from", "to"); err != nil {
			return err
		}
		if _, ok := raw["from"]; !ok || !has(raw, "to") {
			return rule.E(0, "missing-field", "zoom ramp needs from and to")
		}
		from, err := zoomEnd(raw["from"])
		if err != nil {
			return err
		}
		to, err := zoomEnd(raw["to"])
		if err != nil {
			return err
		}
		z.From, z.To = &from, &to
		return nil
	default:
		n, err := num(b)
		if err != nil {
			return err
		}
		z.End = &ZoomEnd{Atoms: []ZoomAtom{Num(n)}}
		return nil
	}
}

// MarshalJSON writes a zoom spec.
func (z ZoomSpec) MarshalJSON() ([]byte, error) {
	if z.From != nil || z.To != nil {
		return json.Marshal(struct {
			From *ZoomEnd `json:"from"`
			To   *ZoomEnd `json:"to"`
		}{z.From, z.To})
	}
	if z.End == nil {
		return []byte("null"), nil
	}
	if z.End.Min {
		return json.Marshal(struct {
			Min []ZoomAtom `json:"min"`
		}{z.End.Atoms})
	}
	if len(z.End.Atoms) == 1 {
		return z.End.Atoms[0].MarshalJSON()
	}
	return nil, rule.E(0, "bad-number", "zoom endpoint is empty")
}

// MarshalJSON writes a number or a symbol.
func (a ZoomAtom) MarshalJSON() ([]byte, error) {
	if a.IsNum {
		return json.Marshal(a.Num)
	}
	return json.Marshal(a.Sym)
}

// UnmarshalJSON decodes one zoom atom.
func (a *ZoomAtom) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		if err := json.Unmarshal(b, &a.Sym); err != nil {
			return rule.E(0, "bad-number", "%s", err.Error())
		}
		return nil
	}
	n, err := num(b)
	if err != nil {
		return err
	}
	a.IsNum = true
	a.Num = n
	return nil
}

// MarshalJSON writes an endpoint.
func (e ZoomEnd) MarshalJSON() ([]byte, error) {
	if e.Min {
		return json.Marshal(struct {
			Min []ZoomAtom `json:"min"`
		}{e.Atoms})
	}
	if len(e.Atoms) != 1 {
		return nil, rule.E(0, "bad-number", "zoom endpoint is empty")
	}
	return e.Atoms[0].MarshalJSON()
}

func zoomEnd(b []byte) (ZoomEnd, error) {
	var spec ZoomSpec
	if err := spec.UnmarshalJSON(b); err != nil {
		return ZoomEnd{}, err
	}
	if spec.From != nil {
		return ZoomEnd{}, rule.E(0, "bad-number", "zoom endpoint cannot be a ramp")
	}
	if spec.End == nil {
		return ZoomEnd{}, rule.E(0, "bad-number", "zoom endpoint is empty")
	}
	return *spec.End, nil
}

func zoomAtoms(b []byte) ([]ZoomAtom, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, rule.E(0, "bad-number", "zoom min: %s", err.Error())
	}
	if len(raw) == 0 {
		return nil, rule.E(0, "bad-number", "zoom min is empty")
	}
	out := make([]ZoomAtom, len(raw))
	for i, r := range raw {
		if err := out[i].UnmarshalJSON(r); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// UnmarshalJSON decodes a named point, a uv point, or a from/to ramp.
func (p *PanSpec) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return rule.E(0, "bad-number", "%s", err.Error())
		}
		p.End = &PanEnd{Name: s}
		return nil
	}
	raw, err := object(b)
	if err != nil {
		return err
	}
	if _, ok := raw["from"]; ok || has(raw, "to") {
		if err := allow(raw, "from", "to"); err != nil {
			return err
		}
		if !has(raw, "from") || !has(raw, "to") {
			return rule.E(0, "missing-field", "pan ramp needs from and to")
		}
		from, err := panEnd(raw["from"])
		if err != nil {
			return err
		}
		to, err := panEnd(raw["to"])
		if err != nil {
			return err
		}
		p.From, p.To = &from, &to
		return nil
	}
	end, err := panEnd(b)
	if err != nil {
		return err
	}
	p.End = &end
	return nil
}

// MarshalJSON writes a pan spec.
func (p PanSpec) MarshalJSON() ([]byte, error) {
	if p.From != nil || p.To != nil {
		return json.Marshal(struct {
			From *PanEnd `json:"from"`
			To   *PanEnd `json:"to"`
		}{p.From, p.To})
	}
	if p.End == nil {
		return []byte("null"), nil
	}
	return p.End.MarshalJSON()
}

// MarshalJSON writes a pan endpoint.
func (e PanEnd) MarshalJSON() ([]byte, error) {
	if e.Name != "" && !e.HasUV {
		return json.Marshal(e.Name)
	}
	return json.Marshal(struct {
		U float64 `json:"u"`
		V float64 `json:"v"`
	}{e.U, e.V})
}

func panEnd(b []byte) (PanEnd, error) {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return PanEnd{}, rule.E(0, "bad-number", "%s", err.Error())
		}
		return PanEnd{Name: s}, nil
	}
	raw, err := object(b)
	if err != nil {
		return PanEnd{}, err
	}
	if err := allow(raw, "u", "v"); err != nil {
		return PanEnd{}, err
	}
	if !has(raw, "u") || !has(raw, "v") {
		return PanEnd{}, rule.E(0, "missing-field", "pan point needs u and v")
	}
	u, err := num(raw["u"])
	if err != nil {
		return PanEnd{}, err
	}
	v, err := num(raw["v"])
	if err != nil {
		return PanEnd{}, err
	}
	return PanEnd{U: u, V: v, HasUV: true}, nil
}

// UnmarshalJSON decodes a timing policy.
func (t *Timing) UnmarshalJSON(b []byte) error {
	raw, err := object(b)
	if err != nil {
		return err
	}
	if err := allow(raw, "policy", "clip", "count", "duration"); err != nil {
		return err
	}
	if _, ok := raw["policy"]; !ok {
		return rule.E(0, "missing-field", "timing.policy")
	}
	t.Policy, err = str(raw["policy"])
	if err != nil {
		return err
	}
	if v, ok := raw["clip"]; ok {
		t.Clip, err = num(v)
		if err != nil {
			return err
		}
		t.HasClip = true
	}
	if v, ok := raw["duration"]; ok {
		t.Duration, err = num(v)
		if err != nil {
			return err
		}
		t.HasDuration = true
	}
	if v, ok := raw["count"]; ok {
		n, err := num(v)
		if err != nil {
			return err
		}
		if n != float64(int(n)) || n < 1 {
			return rule.E(0, "bad-number", "timing.count %v", n)
		}
		t.Count = int(n)
		t.HasCount = true
	}
	return nil
}

// MarshalJSON writes a timing policy with only the fields it uses.
func (t Timing) MarshalJSON() ([]byte, error) {
	m := map[string]any{"policy": t.Policy}
	order := []string{"policy"}
	if t.HasDuration {
		m["duration"] = t.Duration
		order = append(order, "duration")
	}
	if t.HasClip {
		m["clip"] = t.Clip
		order = append(order, "clip")
	}
	if t.HasCount {
		m["count"] = t.Count
		order = append(order, "count")
	}
	return marshalOrdered(m, order...)
}

// UnmarshalJSON decodes a pool.
func (p *Pool) UnmarshalJSON(b []byte) error {
	raw, err := object(b)
	if err != nil {
		return err
	}
	if err := allow(raw, "dir", "sort", "list", "files"); err != nil {
		return err
	}
	n := 0
	if _, ok := raw["dir"]; ok {
		n++
	}
	if _, ok := raw["list"]; ok {
		n++
	}
	if _, ok := raw["files"]; ok {
		n++
	}
	if n != 1 {
		return rule.E(0, "pool", "pool needs one of dir, list, or files")
	}
	if v, ok := raw["dir"]; ok {
		p.Kind = "dir"
		p.Dir, err = str(v)
		if err != nil {
			return err
		}
		p.Sort = "natural"
		if s, ok := raw["sort"]; ok {
			p.Sort, err = str(s)
			if err != nil {
				return err
			}
		}
		return nil
	}
	if _, ok := raw["sort"]; ok {
		return rule.E(0, "unknown-field", "sort is only valid on a directory pool")
	}
	if v, ok := raw["list"]; ok {
		p.Kind = "list"
		p.List, err = str(v)
		return err
	}
	p.Kind = "files"
	if err := json.Unmarshal(raw["files"], &p.Files); err != nil {
		return rule.E(0, "pool", "%s", err.Error())
	}
	return nil
}

// MarshalJSON writes a pool.
func (p Pool) MarshalJSON() ([]byte, error) {
	switch p.Kind {
	case "dir":
		m := map[string]any{"dir": p.Dir}
		order := []string{"dir"}
		if p.Sort != "" && p.Sort != "natural" {
			m["sort"] = p.Sort
			order = append(order, "sort")
		}
		return marshalOrdered(m, order...)
	case "list":
		return json.Marshal(struct {
			List string `json:"list"`
		}{p.List})
	case "files":
		return json.Marshal(struct {
			Files []string `json:"files"`
		}{p.Files})
	default:
		return nil, rule.E(0, "pool", "empty pool")
	}
}

// UnmarshalJSON decodes a schedule.
func (s *Schedule) UnmarshalJSON(b []byte) error {
	raw, err := object(b)
	if err != nil {
		return err
	}
	if err := allow(raw, "policy", "seed"); err != nil {
		return err
	}
	if _, ok := raw["policy"]; !ok {
		return rule.E(0, "missing-field", "schedule.policy")
	}
	s.Policy, err = str(raw["policy"])
	if err != nil {
		return err
	}
	if v, ok := raw["seed"]; ok {
		n, err := num(v)
		if err != nil {
			return err
		}
		if n < 0 || n > 1<<53-1 || n != math.Trunc(n) {
			return rule.E(0, "bad-number", "seed %v", n)
		}
		s.Seed = uint64(n)
		s.HasSeed = true
	}
	return nil
}

// MarshalJSON writes a schedule.
func (s Schedule) MarshalJSON() ([]byte, error) {
	m := map[string]any{"policy": s.Policy}
	order := []string{"policy"}
	if s.HasSeed {
		m["seed"] = s.Seed
		order = append(order, "seed")
	}
	return marshalOrdered(m, order...)
}

// MarshalJSON writes modes with sorted keys.
func (m ModeMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	buf := []byte{'{'}
	for i, k := range keys {
		if i > 0 {
			buf = append(buf, ',')
		}
		kb, _ := json.Marshal(k)
		vb, err := json.Marshal(m[k])
		if err != nil {
			return nil, err
		}
		buf = append(buf, kb...)
		buf = append(buf, ':')
		buf = append(buf, vb...)
	}
	buf = append(buf, '}')
	return buf, nil
}

func marshalOrdered(m map[string]any, keys ...string) ([]byte, error) {
	buf := []byte{'{'}
	n := 0
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		if n > 0 {
			buf = append(buf, ',')
		}
		n++
		kb, _ := json.Marshal(k)
		vb, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		buf = append(buf, kb...)
		buf = append(buf, ':')
		buf = append(buf, vb...)
	}
	buf = append(buf, '}')
	return buf, nil
}

func has(m map[string]json.RawMessage, k string) bool {
	_, ok := m[k]
	return ok
}

func num(b []byte) (float64, error) {
	var n float64
	if err := json.Unmarshal(b, &n); err != nil {
		return 0, rule.E(0, "bad-number", "%s", err.Error())
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, rule.E(0, "bad-number", "not a finite number")
	}
	return n, nil
}

func str(b []byte) (string, error) {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return "", rule.E(0, "bad-number", "%s", err.Error())
	}
	return s, nil
}
