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
		return rule.E(slot, "missing-field", "card")
	}
	if !c.HasText() {
		return rule.E(slot, "missing-field", "card text")
	}
	switch c.Align {
	case "", "left", "center", "right":
	default:
		return rule.E(slot, "unknown-field", "card align %q", c.Align)
	}
	switch c.VAlign {
	case "", "upper", "middle", "lower":
	default:
		return rule.E(slot, "unknown-field", "card vAlign %q", c.VAlign)
	}
	switch c.Size {
	case "", "small", "medium", "large":
	default:
		return rule.E(slot, "unknown-field", "card size %q", c.Size)
	}
	switch c.Weight {
	case "", "regular", "bold":
	default:
		return rule.E(slot, "unknown-field", "card weight %q", c.Weight)
	}
	switch c.Face {
	case "", "sans", "serif":
	default:
		return rule.E(slot, "unknown-field", "card face %q", c.Face)
	}
	if c.Color != "" && !colorRe.MatchString(c.Color) {
		return rule.E(slot, "bad-number", "card color %q", c.Color)
	}
	if c.Fill != "" && !colorRe.MatchString(c.Fill) {
		return rule.E(slot, "bad-number", "card fill %q", c.Fill)
	}
	return nil
}
