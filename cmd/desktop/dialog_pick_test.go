package main

import "testing"

func TestMontagePath(t *testing.T) {
	if got := montagePath("/films/piece"); got != "/films/piece.montage.json" {
		t.Fatalf("bare %s", got)
	}
	if got := montagePath("/films/piece.montage.json"); got != "/films/piece.montage.json" {
		t.Fatalf("already %s", got)
	}
	if got := montagePath("/films/piece.JSON"); got != "/films/piece.JSON" {
		t.Fatalf("json %s", got)
	}
}

func TestVideoExt(t *testing.T) {
	if got := withExt("/films/out", ".mp4"); got != "/films/out.mp4" {
		t.Fatalf("bare %s", got)
	}
	if got := withExt("/films/out.MP4", ".mp4"); got != "/films/out.MP4" {
		t.Fatalf("already %s", got)
	}
}
