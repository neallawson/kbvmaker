package plan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"kbvmaker/v1/compile"
	"kbvmaker/v1/doc"
)

func TestFixedDoesNotChangeThePlanOrTheRender(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	const body = `{
		"montage": 1,
		"defaults": {"hold": 0.5, "join": {"type": "fade", "duration": 1}, "zoomSpan": 0.25, "ease": "linear"},
		"timing": {"policy": "fixed-clip", "clip": 5},
		"slots": [
			{"src": "a.jpg", "mode": "hold"%s},
			{"card": {"title": "End"}, "mode": "hold"%s}
		]
	}`
	open, err := doc.Decode([]byte(fmt.Sprintf(body, "", "")))
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := doc.Decode([]byte(fmt.Sprintf(body, `, "fixed": true`, `, "fixed": true`)))
	if err != nil {
		t.Fatal(err)
	}
	if !pinned.Slots[0].Fixed || !pinned.Slots[1].Fixed {
		t.Fatal("fixed was not read")
	}
	open.Dir, pinned.Dir = dir, dir
	left, err := Resolve(context.Background(), open, fakeImages(dir))
	if err != nil {
		t.Fatal(err)
	}
	right, err := Resolve(context.Background(), pinned, fakeImages(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left, right) {
		t.Fatalf("plan changed\nopen %#v\npinned %#v", left, right)
	}
	cmds, err := compile.Commands(left, compile.Options{})
	if err != nil {
		t.Fatal(err)
	}
	again, err := compile.Commands(right, compile.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cmds, again) {
		t.Fatal("render commands changed")
	}
}
