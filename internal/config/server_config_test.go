package config

import (
	"strings"
	"testing"
)

func TestLoadServerConfigRequiresAdminToken(t *testing.T) {
	t.Setenv("TRAICR_ADMIN_TOKEN", "")

	_, err := LoadServerConfig()

	if err == nil {
		t.Fatal("LoadServerConfig() error = nil, want missing admin token error")
	}
	if !strings.Contains(err.Error(), "TRAICR_ADMIN_TOKEN is required") {
		t.Fatalf("LoadServerConfig() error = %q, want actionable token error", err)
	}
}

func TestLoadServerConfigUsesContainerDefaults(t *testing.T) {
	t.Setenv("TRAICR_ADMIN_TOKEN", "test-token")
	t.Setenv("TRAICR_LISTEN_ADDRESS", "")
	t.Setenv("TRAICR_DATA_DIR", "")

	got, err := LoadServerConfig()
	if err != nil {
		t.Fatalf("LoadServerConfig() error = %v", err)
	}

	if got.ListenAddress != ":8080" {
		t.Errorf("ListenAddress = %q, want :8080", got.ListenAddress)
	}
	if got.DataDir != "/data" {
		t.Errorf("DataDir = %q, want /data", got.DataDir)
	}
	if got.AdminToken != "test-token" {
		t.Errorf("AdminToken = %q, want test-token", got.AdminToken)
	}
}

func TestLoadServerConfigUsesEnvironmentOverrides(t *testing.T) {
	t.Setenv("TRAICR_ADMIN_TOKEN", "test-token")
	t.Setenv("TRAICR_LISTEN_ADDRESS", "127.0.0.1:9090")
	t.Setenv("TRAICR_DATA_DIR", "/srv/traicr")

	got, err := LoadServerConfig()
	if err != nil {
		t.Fatalf("LoadServerConfig() error = %v", err)
	}

	if got.ListenAddress != "127.0.0.1:9090" {
		t.Errorf("ListenAddress = %q, want override", got.ListenAddress)
	}
	if got.DataDir != "/srv/traicr" {
		t.Errorf("DataDir = %q, want override", got.DataDir)
	}
}

func TestLoadServerConfigTitleWorker(t *testing.T) {
	t.Setenv("TRAICR_ADMIN_TOKEN", "test-token")
	for _, test := range []struct {
		name, url, key, model, effort, wantErr string
		enabled                                bool
	}{
		{name: "off by default"},
		{name: "configured", url: "https://opencode.ai/zen/go/v1/", key: "key", model: "longcat-2.5-preview-free", effort: "minimal", enabled: true},
		{name: "local model without key", url: "http://127.0.0.1:11434/v1", model: "local", enabled: true},
		{name: "model required", url: "https://opencode.ai/zen/go/v1", key: "key", wantErr: "TRAICR_TITLE_MODEL is required"},
		{name: "url required", key: "key", model: "m", wantErr: "TRAICR_TITLE_API_URL is required"},
		{name: "url with credentials", url: "https://user:pass@example.com/v1", model: "m", wantErr: "TRAICR_TITLE_API_URL must be"},
		{name: "relative url", url: "opencode.ai/v1", model: "m", wantErr: "TRAICR_TITLE_API_URL must be"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TRAICR_TITLE_API_URL", test.url)
			t.Setenv("TRAICR_TITLE_API_KEY", test.key)
			t.Setenv("TRAICR_TITLE_MODEL", test.model)
			t.Setenv("TRAICR_TITLE_REASONING_EFFORT", test.effort)
			got, err := LoadServerConfig()
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("err = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil || got.Titles.Enabled() != test.enabled {
				t.Fatalf("titles = %+v, err = %v, want enabled=%v", got.Titles, err, test.enabled)
			}
			if test.name == "configured" && (got.Titles.APIURL != "https://opencode.ai/zen/go/v1" || got.Titles.ReasoningEffort != "minimal") {
				t.Fatalf("titles = %+v", got.Titles)
			}
		})
	}
}
