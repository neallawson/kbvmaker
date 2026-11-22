package identity

import "testing"

func TestText(t *testing.T) {
	const want = "kbvmaker version dev, Copyright (c) 2026 Neal Lawson, built unknown\nhttps://github.com/neallawson/kbvmaker\n"
	if got := Text(); got != want {
		t.Fatalf("Text() = %q", got)
	}
}
