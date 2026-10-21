package motion

import (
	"testing"

	"kbvmaker/v1/clock"
	"kbvmaker/v1/doc"
	"kbvmaker/v1/rule"
)

func TestReverseMovesTravelBack(t *testing.T) {
	place := clock.Place{Song: 8, ClipDur: 4, OpaqueStart: 0.5, OpaqueEnd: 3.5, FPS: 8}
	tests := []struct {
		mode   string
		u0, u1 float64
		v0, v1 float64
	}{
		{"in-left-right", 0, 1, 0.5, 0.5},
		{"in-right-left", 1, 0, 0.5, 0.5},
		{"in-top-bottom", 0.5, 0.5, 0, 1},
		{"in-bottom-top", 0.5, 0.5, 1, 0},
	}
	for _, tc := range tests {
		t.Run(tc.mode, func(t *testing.T) {
			_, _, _, spans, err := Resolve(doc.Slot{Src: "a.jpg", Mode: tc.mode}, nil, place, 0.25, 0.5, 1.5, 1)
			if err != nil {
				t.Fatal(err)
			}
			last := spans[len(spans)-1]
			if last.U0 != tc.u0 || last.U1 != tc.u1 || last.V0 != tc.v0 || last.V1 != tc.v1 {
				t.Fatalf("pan from (%v,%v) to (%v,%v)", last.U0, last.V0, last.U1, last.V1)
			}
		})
	}
}

func TestReverseMoveNeedsRoomToTravel(t *testing.T) {
	place := clock.Place{ClipDur: 1, OpaqueStart: 0, OpaqueEnd: 0.2, FPS: 8}
	_, _, _, _, err := Resolve(doc.Slot{Mode: "in-bottom-top"}, nil, place, 0.2, 1, 1.2, 1)
	re, ok := rule.As(err)
	if !ok || re.Code != "no-travel" {
		t.Fatal(err)
	}
}
