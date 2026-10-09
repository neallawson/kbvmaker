package media

import "testing"

func TestParseImageRotation(t *testing.T) {
	info, err := parseImage([]byte(`{"streams":[{"width":200,"height":400,"tags":{"rotate":"90"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 400 || info.Height != 200 {
		t.Fatalf("tag rotate: %dx%d", info.Width, info.Height)
	}
	info, err = parseImage([]byte(`{"streams":[{"width":200,"height":400,"side_data_list":[{"rotation":-90}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 400 || info.Height != 200 {
		t.Fatalf("display matrix: %dx%d", info.Width, info.Height)
	}
	info, err = parseImage([]byte(`{"streams":[{"width":200,"height":400}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 200 || info.Height != 400 {
		t.Fatalf("unrotated: %dx%d", info.Width, info.Height)
	}
}
