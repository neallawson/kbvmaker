//go:build production || dev

package main

import "testing"

func TestClientBuildHasThePage(t *testing.T) {
	client, err := clientAssets()
	if err != nil {
		t.Fatal(err)
	}
	file, err := client.Open("index.html")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
}
