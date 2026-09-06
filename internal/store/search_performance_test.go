package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/regutierrez/traicr/internal/domain"
)

func TestSearchBoundedCandidatePages(t *testing.T) {
	s := seedSearchCorpus(t, 5000)
	ctx := context.Background()
	for _, statement := range []string{
		`INSERT INTO repositories(id,identity,remote,root) VALUES(1,'benchmark','example.invalid/benchmark','/work/benchmark')`,
		`UPDATE traces SET repository_id=1`,
		`UPDATE event_observations SET model='rare-model',role='tool',kind='tool_result',tool='rare-tool',event_time='2026-08-23T10:00:00.000000000Z',event_json=json_set(event_json,'$.model','rare-model','$.role','tool','$.kind','tool_result','$.tool','rare-tool','$.timestamp','2026-08-23T10:00:00.000000000Z') WHERE event_id=1`,
		`UPDATE event_observations SET event_time='2026-08-21T10:00:00.000000000Z',event_json=json_set(event_json,'$.timestamp','2026-08-21T10:00:00.000000000Z') WHERE event_id=2`,
	} {
		if _, err := s.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.Search(ctx, SearchQuery{Query: "needlecommon", Mode: "fulltext", Limit: 25})
	if err != nil || len(first.Results) != 25 || first.NextCursor == "" {
		t.Fatalf("first common page=%+v err=%v", first, err)
	}
	second, err := s.Search(ctx, SearchQuery{Query: "needlecommon", Mode: "fulltext", Limit: 25, Cursor: first.NextCursor})
	if err != nil || len(second.Results) != 25 || second.Results[0].EventID >= first.Results[len(first.Results)-1].EventID {
		t.Fatalf("second common page=%+v err=%v", second, err)
	}
	for _, check := range []struct {
		query SearchQuery
		count int
		first int64
	}{
		{SearchQuery{Query: "rare-selective-marker-4990", Mode: "exact"}, 1, 4990},
		{SearchQuery{Query: `rare-selective-marker-4990$`, Mode: "regex"}, 1, 4990},
		{SearchQuery{Query: "needlecommon event 000001", Mode: "exact"}, 1, 1},
		{SearchQuery{Query: `(?s).*needlecommon event 000001`, Mode: "regex"}, 1, 1},
		{SearchQuery{Query: `(?s).*absent-selective-marker`, Mode: "regex"}, 0, 0},
		{SearchQuery{Query: "absent-selective-marker", Mode: "exact"}, 0, 0},
		{SearchQuery{Query: "needlecommon", Model: "model-b"}, 50, 5000},
		{SearchQuery{Query: "needlecommon", Model: "rare-model"}, 1, 1},
		{SearchQuery{Query: "needlecommon", Role: "tool"}, 1, 1},
		{SearchQuery{Query: "needlecommon", Kind: "tool_result"}, 1, 1},
		{SearchQuery{Query: "needlecommon", Tool: "rare-tool"}, 1, 1},
		{SearchQuery{Query: "needlecommon", After: "2026-08-23"}, 1, 1},
		{SearchQuery{Query: "needlecommon", Before: "2026-08-21"}, 1, 2},
		{SearchQuery{Query: "needlecommon", Harness: "synthetic"}, 50, 5000},
		{SearchQuery{Query: "needlecommon", Harness: "missing"}, 0, 0},
		{SearchQuery{Query: "needlecommon", Repository: "example.invalid/benchmark"}, 50, 5000},
		{SearchQuery{Query: "needlecommon", Repository: "missing"}, 0, 0},
		{SearchQuery{Query: "needlecommon", Machine: "machine"}, 50, 5000},
		{SearchQuery{Query: "needlecommon", Machine: "missing"}, 0, 0},
	} {
		page, searchErr := s.Search(ctx, check.query)
		if searchErr != nil || len(page.Results) != check.count {
			t.Fatalf("query=%+v results=%d want=%d err=%v", check.query, len(page.Results), check.count, searchErr)
		}
		if check.count > 0 && page.Results[0].EventID != check.first {
			t.Fatalf("query=%+v first=%d want=%d", check.query, page.Results[0].EventID, check.first)
		}
		for _, result := range page.Results {
			if check.query.Model != "" && result.Model != check.query.Model {
				t.Fatalf("model filter returned %+v", result)
			}
		}
	}
	query := SearchQuery{Query: `(?s).*needlecommon event 00[0-9]*[02468](?: .*)?$`, Mode: "regex", Limit: 200}
	page, err := s.Search(ctx, query)
	if err != nil || len(page.Results) != 200 || page.NextCursor == "" {
		t.Fatalf("exactly full candidate batch lost cursor: page=%+v err=%v", page, err)
	}
	query.Cursor = page.NextCursor
	page, err = s.Search(ctx, query)
	if err != nil || len(page.Results) != 200 || page.Results[0].EventID != 4600 {
		t.Fatalf("fallback second page=%+v err=%v", page, err)
	}
}

func BenchmarkSearch100K(b *testing.B) {
	s := seedSearchCorpus(b, 100000)
	ctx := context.Background()
	queries := map[string]SearchQuery{
		"fulltext-common":     {Query: "needlecommon", Mode: "fulltext"},
		"regex-common":        {Query: `needlecommon event`, Mode: "regex"},
		"exact-rare":          {Query: "rare-selective-marker-99800", Mode: "exact"},
		"exact-common-prefix": {Query: "needlecommon event 000001", Mode: "exact"},
		"exact-miss":          {Query: "absent-selective-marker", Mode: "exact"},
	}
	for name, query := range queries {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, err := s.Search(ctx, query); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func seedSearchCorpus(tb testing.TB, count int) *Store {
	tb.Helper()
	s, err := Open(tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { s.Close() })
	tx, err := s.db.Begin()
	if err != nil {
		tb.Fatal(err)
	}
	now := "2026-08-22T10:00:00.000000000Z"
	statements := []string{
		`INSERT INTO source_machines(id,hostname,os,arch,first_seen,last_seen) VALUES('machine','host','linux','amd64',?,?)`,
		`INSERT INTO traces(id,harness,native_trace_id,title,working_directory,parent_native_trace_id,created_at,updated_at) VALUES(1,'synthetic','scale','Scale','/work','',?,?)`,
		`INSERT INTO trace_revisions(id,trace_id,digest,native_updated_at,collected_at,adapter,status,normalizer_version) VALUES(1,1,'revision',?,?,'synthetic','normalized',1)`,
	}
	for _, statement := range statements {
		if _, err = tx.Exec(statement, now, now); err != nil {
			tx.Rollback()
			tb.Fatal(err)
		}
	}
	if _, err = tx.Exec(`INSERT INTO revision_machines(revision_id,machine_id) VALUES(1,'machine')`); err != nil {
		tx.Rollback()
		tb.Fatal(err)
	}
	events, err := tx.Prepare(`INSERT INTO events(id,trace_id,event_key,sort_time,sort_key,preferred_observation_id) VALUES(?,1,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		tb.Fatal(err)
	}
	observations, err := tx.Prepare(`INSERT INTO event_observations(id,event_id,digest,parent_key,branch,kind,role,model,provider,tool,call_id,event_time,text,event_json) VALUES(?,?,'digest','','','message','assistant',?,'','','',?,?,?)`)
	if err != nil {
		tx.Rollback()
		tb.Fatal(err)
	}
	revisions, err := tx.Prepare(`INSERT INTO observation_revisions(observation_id,revision_id,searchable_text) VALUES(?,1,?)`)
	if err != nil {
		tx.Rollback()
		tb.Fatal(err)
	}
	chunks, err := tx.Prepare(`INSERT INTO search_chunks(observation_id,revision_id,event_id,content) VALUES(?,1,?,?)`)
	if err != nil {
		tx.Rollback()
		tb.Fatal(err)
	}
	defer events.Close()
	defer observations.Close()
	defer revisions.Close()
	defer chunks.Close()
	payload := ""
	if count >= 100000 {
		payload = strings.Repeat(" representative payload", 120)
	}
	for i := 1; i <= count; i++ {
		key := fmt.Sprintf("event-%06d", i)
		text := fmt.Sprintf("needlecommon event %06d%s", i, payload)
		if i%4990 == 0 {
			text += fmt.Sprintf(" rare-selective-marker-%d", i)
		}
		model := "model-a"
		if i%2 == 0 {
			model = "model-b"
		}
		encoded, marshalErr := json.Marshal(domain.Event{Key: key, Kind: "message", Role: "assistant", Model: model, Timestamp: now, Text: text})
		if marshalErr != nil {
			tx.Rollback()
			tb.Fatal(marshalErr)
		}
		if _, err = events.Exec(i, key, now, key, i); err == nil {
			_, err = observations.Exec(i, i, model, now, text, encoded)
		}
		if err == nil {
			_, err = revisions.Exec(i, text)
		}
		if err == nil {
			_, err = chunks.Exec(i, i, text)
		}
		if err != nil {
			tx.Rollback()
			tb.Fatalf("seed event %d: %v", i, err)
		}
	}
	if err = tx.Commit(); err != nil {
		tb.Fatal(err)
	}
	return s
}
