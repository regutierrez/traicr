package search

import (
	"strings"
	"testing"
)

func TestSnippetCutsAtWordEdgesAndMarksCuts(t *testing.T) {
	text := strings.Repeat("alphab ", 40) + "needle" + strings.Repeat(" omega", 40)
	got := Snippet(text, "needle")
	if !strings.HasPrefix(got, "…alphab ") || !strings.HasSuffix(got, " omega…") || !strings.Contains(got, "needle") {
		t.Fatalf("snippet = %q; want whole words with both cuts marked", got)
	}
	if got := Snippet("short needle text", "needle"); got != "short needle text" {
		t.Fatalf("uncut snippet = %q; want the text unchanged", got)
	}
}
