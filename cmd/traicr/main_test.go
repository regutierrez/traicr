package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoginReadsTokenFromInputWithoutPrintingIt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TRAICR_CONFIG_DIR", dir)
	t.Setenv("TRAICR_ADMIN_TOKEN", "")
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"login", "https://traicr.example"}, strings.NewReader("very-secret\n"), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String()+stderr.String(), "very-secret") {
		t.Fatal("login printed the token")
	}
	info, err := os.Stat(filepath.Join(dir, "collector.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("configuration mode is %o, want 600", info.Mode().Perm())
	}
}
