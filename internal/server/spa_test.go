package server

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadSPAFallsBackWhenIndexMissing(t *testing.T) {
	doc, err := loadSPA(fstest.MapFS{})
	if err == nil {
		t.Fatal("expected an error when the embedded UI is missing")
	}
	if !strings.Contains(string(doc.html), "Build the UI in web/app") {
		t.Fatalf("fallback document: %s", doc.html)
	}
	if doc.policy == "" {
		t.Fatal("fallback document has no CSP")
	}
}

func TestLoadSPAReadsIndex(t *testing.T) {
	files := fstest.MapFS{
		"static/ui/index.html": {Data: []byte("<!doctype html><title>built</title>")},
	}
	doc, err := loadSPA(files)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc.html), "<title>built</title>") {
		t.Fatalf("built document: %s", doc.html)
	}
}

func TestUIAssetsRequiresIndex(t *testing.T) {
	if _, err := uiAssets(fstest.MapFS{}); err == nil {
		t.Fatal("expected an error when static/ui is missing")
	}
	if _, err := uiAssets(fstest.MapFS{"static/ui/placeholder": {Data: []byte("x")}}); err == nil {
		t.Fatal("expected an error when index.html is missing")
	}
	files := fstest.MapFS{"static/ui/index.html": {Data: []byte("<!doctype html>")}}
	ui, err := uiAssets(files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.ReadFile(ui, "index.html"); err != nil {
		t.Fatal(err)
	}
}
