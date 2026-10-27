package plan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"kbvmaker/v1/doc"
	"kbvmaker/v1/media"
	"kbvmaker/v1/rule"
)

func TestResolveListsEveryMissingAsset(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := doc.Decode([]byte(`{
		"montage": 1,
		"tracks": [{"src": "song.mp3"}],
		"slots": [
			{"src": "a.jpg", "mode": "in-center"},
			{"src": "b.jpg", "mode": "out-center"},
			{"src": "c.jpg", "mode": "in-center"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	d.Dir = dir
	_, err = Resolve(context.Background(), d, &media.Fake{})
	re, ok := rule.As(err)
	if !ok || re.Code != "missing-media" || re.Slot != 0 {
		t.Fatalf("error %+v", err)
	}
	want := "Missing soundtrack: song.mp3\nMissing picture 2: b.jpg\nMissing picture 3: c.jpg"
	if re.Msg != want {
		t.Fatalf("message %q", re.Msg)
	}
}

func TestResolveKeepsTheSlotWhenOnePictureIsMissing(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"song.mp3", "a.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d, err := doc.Decode([]byte(`{
		"montage": 1,
		"tracks": [{"src": "song.mp3"}],
		"slots": [
			{"src": "a.jpg", "mode": "in-center"},
			{"src": "b.jpg", "mode": "out-center"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	d.Dir = dir
	_, err = Resolve(context.Background(), d, &media.Fake{
		Durations: map[string]float64{filepath.Join(dir, "song.mp3"): 30},
	})
	re, ok := rule.As(err)
	if !ok || re.Slot != 2 || re.Msg != "Missing picture 2: b.jpg" {
		t.Fatalf("error %+v", err)
	}
}
