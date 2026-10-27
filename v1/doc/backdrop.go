package doc

import "regexp"

import "kbvmaker/v1/rule"

var colorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Backdrop is the area the fitted photograph does not cover.
// A nil Blur means the kind's default has not been applied yet.
type Backdrop struct {
	Kind  string   `json:"kind"`
	Blur  *float64 `json:"blur,omitempty"`
	Color string   `json:"color,omitempty"`
	Name  string   `json:"name,omitempty"`
	Src   string   `json:"src,omitempty"`
}

// Sigma is the Gaussian blur, or 0 when this kind has none.
func (b Backdrop) Sigma() float64 {
	if b.Blur == nil {
		return 0
	}
	return *b.Blur
}

func blurBackdrop(sigma float64) Backdrop {
	return Backdrop{Kind: "blur", Blur: &sigma}
}

// finish applies defaults and rejects fields the kind does not use.
func (b *Backdrop) finish(slot int) error {
	switch b.Kind {
	case "blur":
		if b.Color != "" || b.Name != "" || b.Src != "" {
			return rule.E(slot, "unknown-field", "a blur backdrop only accepts a blur amount")
		}
		if b.Blur == nil {
			v := 40.0
			b.Blur = &v
		}
		if *b.Blur < 0 {
			return rule.E(slot, "bad-number", "blur %v must be zero or more", *b.Blur)
		}
	case "color":
		if b.Blur != nil || b.Name != "" || b.Src != "" {
			return rule.E(slot, "unknown-field", "a color backdrop only accepts color")
		}
		if !colorRe.MatchString(b.Color) {
			return rule.E(slot, "bad-number", "color %q must be a #RRGGBB color", b.Color)
		}
	case "pattern":
		if b.Blur != nil || b.Src != "" {
			return rule.E(slot, "unknown-field", "a pattern backdrop only accepts name and color")
		}
		if b.Name != "grain" && b.Name != "gradient" {
			return rule.E(slot, "unknown-field", "pattern %q is not supported", b.Name)
		}
		if b.Color == "" {
			b.Color = "#000000"
		}
		if !colorRe.MatchString(b.Color) {
			return rule.E(slot, "bad-number", "color %q must be a #RRGGBB color", b.Color)
		}
	case "image":
		if b.Color != "" || b.Name != "" {
			return rule.E(slot, "unknown-field", "an image backdrop only accepts src and blur")
		}
		if b.Src == "" || IsURL(b.Src) {
			return rule.E(slot, "missing-media", "an image backdrop needs a picture file")
		}
		if b.Blur == nil {
			v := 0.0
			b.Blur = &v
		}
		if *b.Blur < 0 {
			return rule.E(slot, "bad-number", "blur %v must be zero or more", *b.Blur)
		}
	case "":
		return rule.E(slot, "missing-field", "a backdrop needs a kind")
	default:
		return rule.E(slot, "unknown-field", "backdrop kind %q is not supported", b.Kind)
	}
	return nil
}
