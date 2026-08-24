package version

import "testing"

func TestBuildInfoUsesDeterministicDevelopmentDefaults(t *testing.T) {
	got := CurrentBuildInfo()

	if got.Version != "development" {
		t.Fatalf("Version = %q, want development", got.Version)
	}
	if got.Commit != "unknown" {
		t.Fatalf("Commit = %q, want unknown", got.Commit)
	}
	if got.BuildDate != "unknown" {
		t.Fatalf("BuildDate = %q, want unknown", got.BuildDate)
	}
}

func TestBuildInfoString(t *testing.T) {
	info := BuildInfo{
		Version:   "v1.2.3",
		Commit:    "abc123",
		BuildDate: "2026-08-22T10:00:00Z",
	}

	want := "v1.2.3 (commit abc123, built 2026-08-22T10:00:00Z)"
	if got := info.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
