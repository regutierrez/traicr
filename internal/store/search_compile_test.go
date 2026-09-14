package store

import (
	"errors"
	"testing"
)

func TestCompileSearchValidatesAndIndexesWithoutEventCursor(t *testing.T) {
	fulltext, err := compileSearch(SearchQuery{Query: "needle", Mode: "fulltext"})
	if err != nil {
		t.Fatal(err)
	}
	if fulltext.index != "search_words" || fulltext.indexQuery != "needle" || fulltext.mode != "fulltext" {
		t.Fatalf("fulltext compile: %+v", fulltext)
	}
	if fulltext.broad || fulltext.position != (cursor{}) {
		t.Fatalf("event-search cursor leaked into compile: %+v", fulltext)
	}

	exact, err := compileSearch(SearchQuery{Query: "abc", Mode: "exact"})
	if err != nil {
		t.Fatal(err)
	}
	if exact.index != "search_trigrams" || exact.indexQuery != `"abc"` {
		t.Fatalf("exact compile: %+v", exact)
	}

	regex, err := compileSearch(SearchQuery{Query: `quoted\s+words`, Mode: "regex"})
	if err != nil || regex.expression == nil || regex.index != "search_trigrams" {
		t.Fatalf("regex compile: %+v %v", regex, err)
	}

	for _, query := range []SearchQuery{
		{Mode: "wildcard"},
		{Query: "[", Mode: "regex"},
		{After: "yesterday"},
	} {
		if _, err := compileSearch(query); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("invalid compile %+v: %v", query, err)
		}
	}
}
