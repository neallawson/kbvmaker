package schedule

import (
	"os"
	"path/filepath"
	"testing"

	"kbvmaker/v1/doc"
)

func TestSplitMixVectors(t *testing.T) {
	r := NewRNG(0)
	if got := r.Next(); got != 16294208416658607535 {
		t.Fatalf("first %d", got)
	}
	if got := r.Next(); got != 7960286522194355700 {
		t.Fatalf("second %d", got)
	}
}

func TestNaturalSort(t *testing.T) {
	names := []string{"Greenhouse10.jpg", "Greenhouse2.jpg", "a02", "a2"}
	sortStrings(names, NaturalLess)
	want := []string{"Greenhouse2.jpg", "Greenhouse10.jpg", "a2", "a02"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("order %v, want %v", names, want)
		}
	}
}

func TestShuffleIsPermutation(t *testing.T) {
	files := []string{"a", "b", "c", "d"}
	got, err := order(files, "shuffle", 1, len(files))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, f := range got {
		seen[f]++
	}
	if len(seen) != 4 {
		t.Fatalf("shuffle %v", got)
	}
	again, _ := order(files, "shuffle", 1, len(files))
	for i := range got {
		if got[i] != again[i] {
			t.Fatal("same seed diverged")
		}
	}
}

func TestLoopNoImmediateRepeat(t *testing.T) {
	files := []string{"a", "b", "c"}
	got, err := order(files, "loop", 7, 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 12 {
		t.Fatalf("len %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i] == got[i-1] {
			t.Fatalf("repeat at %d in %v", i, got)
		}
	}
}

func TestDirectoryPoolNaturalOrder(t *testing.T) {
	dir := t.TempDir()
	photos := filepath.Join(dir, "photos")
	if err := os.Mkdir(photos, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Greenhouse10.jpg", "Greenhouse2.jpg", "notes.txt", ".hidden.jpg"} {
		if err := os.WriteFile(filepath.Join(photos, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d := &doc.Document{
		Dir:      dir,
		Timing:   doc.Timing{Policy: "fit-audio"},
		Defaults: doc.Defaults{Join: doc.Join{Type: "fade", Duration: 1.5}},
		Pool:     &doc.Pool{Kind: "dir", Dir: "photos", Sort: "natural"},
		Schedule: &doc.Schedule{Policy: "given"},
	}
	slots, err := Generate(d, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 2 || slots[0].Src != "photos/Greenhouse2.jpg" || slots[1].Src != "photos/Greenhouse10.jpg" {
		t.Fatalf("slots %+v", slots)
	}
	if slots[0].Mode != "in-center" {
		t.Fatalf("mode %s", slots[0].Mode)
	}
}
