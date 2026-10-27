package doc

import (
	"strings"

	"kbvmaker/v1/rule"
)

// Card is a title or credits screen painted at render.
type Card struct {
	Title    string   `json:"title,omitempty"`
	Subtitle string   `json:"subtitle,omitempty"`
	Line     string   `json:"line,omitempty"`
	Credits  []string `json:"credits,omitempty"`
	Align    string   `json:"align,omitempty"`
	VAlign   string   `json:"vAlign,omitempty"`
	Size     string   `json:"size,omitempty"`
	Weight   string   `json:"weight,omitempty"`
	Face     string   `json:"face,omitempty"`
	Color    string   `json:"color,omitempty"`
	Fill     string   `json:"fill,omitempty"`
	Box      bool     `json:"box,omitempty"`
}

// HasText reports title or credit lines after trimming.
func (c Card) HasText() bool {
	if strings.TrimSpace(c.Title) != "" {
		return true
	}
	for _, line := range c.Credits {
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

// Finish checks the card fields. Empty placement and style fields take the paint defaults.
func (c *Card) Finish(slot int) error {
	if c == nil {
		return rule.E(slot, "missing-field", "the card is missing")
	}
	if !c.HasText() {
		return rule.E(slot, "missing-field", "a card needs a title or credits")
	}
	switch c.Align {
	case "", "left", "center", "right":
	default:
		return rule.E(slot, "unknown-field", "card alignment %q is not supported", c.Align)
	}
	switch c.VAlign {
	case "", "upper", "middle", "lower":
	default:
		return rule.E(slot, "unknown-field", "card vertical alignment %q is not supported", c.VAlign)
	}
	switch c.Size {
	case "", "small", "medium", "large":
	default:
		return rule.E(slot, "unknown-field", "card size %q is not supported", c.Size)
	}
	switch c.Weight {
	case "", "regular", "bold":
	default:
		return rule.E(slot, "unknown-field", "card weight %q is not supported", c.Weight)
	}
	switch c.Face {
	case "", "sans", "serif":
	default:
		return rule.E(slot, "unknown-field", "card face %q is not supported", c.Face)
	}
	if c.Color != "" && !colorRe.MatchString(c.Color) {
		return rule.E(slot, "bad-number", "card color %q must be a #RRGGBB color", c.Color)
	}
	if c.Fill != "" && !colorRe.MatchString(c.Fill) {
		return rule.E(slot, "bad-number", "card background %q must be a #RRGGBB color", c.Fill)
	}
	return nil
}
