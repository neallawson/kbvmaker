//go:build linux

package main

import (
	"os"
	"path/filepath"
	"testing"
)

var haveGTK bool

func TestMain(m *testing.M) {
	if os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" {
		haveGTK = startGTK()
	}
	os.Exit(m.Run())
}

func needDisplay(t *testing.T) {
	t.Helper()
	if !haveGTK {
		t.Skip("no display for the GTK dialog")
	}
}

func TestPictureDialogReopensInTheLastFolder(t *testing.T) {
	needDisplay(t)
	first := t.TempDir()
	second := t.TempDir()
	a := filepath.Join(first, "a.jpg")
	b := filepath.Join(second, "b.jpg")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d := &Dialogs{}

	t.Setenv("KBV_DIALOG_PICK", a)
	got, err := d.OpenImages()
	if err != nil || len(got) != 1 || got[0] != a {
		t.Fatalf("first pick %v %v", got, err)
	}
	if d.imageDir != first {
		t.Fatalf("remembered %q", d.imageDir)
	}

	t.Setenv("KBV_DIALOG_PICK", b)
	paths, shown := pickGTK("multi", "Add pictures", d.imageDir, "", "Images", []string{"*.jpg"})
	if shown != first {
		t.Fatalf("second dialog opened in %q, want %q", shown, first)
	}
	if len(paths) != 1 || paths[0] != b {
		t.Fatalf("second pick %v", paths)
	}
}

func TestSaveDialogReturnsANewMontageInTheLastFolder(t *testing.T) {
	needDisplay(t)
	dir := t.TempDir()
	d := &Dialogs{montageDir: dir}
	t.Setenv("KBV_DIALOG_PICK", "save")
	got, err := d.SaveMontage()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "piece.montage.json"); got != want {
		t.Fatalf("save path %q, want %q", got, want)
	}
}

func TestAudioDialogStartsInItsOwnFolder(t *testing.T) {
	needDisplay(t)
	music := t.TempDir()
	song := filepath.Join(music, "song.wav")
	if err := os.WriteFile(song, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := &Dialogs{audioDir: music, imageDir: t.TempDir()}
	t.Setenv("KBV_DIALOG_PICK", song)
	paths, shown := pickGTK("open", "Add song", d.audioDir, "", "Audio", []string{"*.wav"})
	if shown != music || len(paths) != 1 || paths[0] != song {
		t.Fatalf("shown %q paths %v", shown, paths)
	}
}
