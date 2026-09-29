package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/regutierrez/traicr/internal/config"
)

func TestTracesRenameSendsTitleToServer(t *testing.T) {
	var got []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		got = append(got, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization")+" "+r.Header.Get("Content-Type")+" "+body["title_override"])
		if r.URL.Path == "/api/v1/traces/404" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"api_version":1,"error":{"code":"not_found","message":"record not found"}}`))
			return
		}
		title := body["title_override"]
		if title == "" {
			title = "Collected title"
		}
		json.NewEncoder(w).Encode(map[string]any{"id": 42, "native_trace_id": "native-42", "title": title})
	}))
	defer server.Close()
	dir := t.TempDir()
	t.Setenv("TRAICR_CONFIG_DIR", dir)
	if err := config.SaveCollector(filepath.Join(dir, "collector.json"), config.Collector{MachineID: "machine", ServerURL: server.URL, Token: "secret-token"}); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"traces", "rename", "42", "Fix login redirect"}, "42\tFix login redirect\n"},
		{[]string{"traces", "rename", "--clear", "42"}, "42\tCollected title\n"},
	} {
		var stdout, stderr bytes.Buffer
		if err := run(context.Background(), test.args, nil, &stdout, &stderr); err != nil {
			t.Fatalf("%v: %v", test.args, err)
		}
		if stdout.String() != test.want {
			t.Fatalf("%v printed %q, want %q", test.args, stdout.String(), test.want)
		}
	}
	want := []string{
		"PATCH /api/v1/traces/42 Bearer secret-token application/json Fix login redirect",
		"PATCH /api/v1/traces/42 Bearer secret-token application/json ",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("requests %q, want %q", got, want)
	}

	err := run(context.Background(), []string{"traces", "rename", "404", "Missing"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "404 (not_found): record not found") {
		t.Fatalf("server error = %v", err)
	}
	for _, args := range [][]string{
		{"traces", "rename", "42"},
		{"traces", "rename", "42", "   "},
		{"traces", "rename", "--clear", "42", "extra"},
		{"traces", "rename", "abc", "Title"},
		{"traces", "rename", "0", "Title"},
		{"traces", "unknown"},
	} {
		if err := run(context.Background(), args, nil, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Errorf("%v succeeded, want a usage error", args)
		}
	}
	if len(got) != 3 {
		t.Fatalf("usage errors reached the server: %q", got)
	}
}

func TestTracesRenameRequiresLogin(t *testing.T) {
	t.Setenv("TRAICR_CONFIG_DIR", t.TempDir())
	err := run(context.Background(), []string{"traces", "rename", "42", "Title"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("err = %v, want not logged in", err)
	}
}
