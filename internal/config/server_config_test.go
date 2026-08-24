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
