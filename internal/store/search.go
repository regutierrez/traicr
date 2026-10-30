package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/codesearch/index"
	searchtext "github.com/regutierrez/traicr/internal/search"
)

var ErrInvalidQuery = errors.New("invalid search query")

func (s *Store) Search(ctx context.Context, query SearchQuery) (SearchPage, error) {
	prepared, err := s.prepareSearch(ctx, query)
	if err != nil {
		return SearchPage{}, err
	}
	return s.collectSearch(ctx, prepared)
}

type preparedSearch struct {
	query      SearchQuery
	mode       string
	expression *regexp.Regexp
	index      string
	indexQuery string
	broad      bool
	position   cursor
}

func (s *Store) prepareSearch(ctx context.Context, query SearchQuery) (preparedSearch, error) {
	mode := query.Mode
	if mode == "" {
		mode = "fulltext"
	}
	if mode != "fulltext" && mode != "exact" && mode != "regex" {
		return preparedSearch{}, fmt.Errorf("%w: mode must be fulltext, exact, or regex", ErrInvalidQuery)
	}
	var err error
	query.After, err = searchDate(query.After, false)
	if err != nil {
		return preparedSearch{}, fmt.Errorf("%w: after must be YYYY-MM-DD or an RFC 3339 timestamp", ErrInvalidQuery)
	}
	query.Before, err = searchDate(query.Before, true)
	if err != nil {
		return preparedSearch{}, fmt.Errorf("%w: before must be YYYY-MM-DD or an RFC 3339 timestamp", ErrInvalidQuery)
	}
	prepared := preparedSearch{query: query, mode: mode}
	var trigramQuery string
	if mode == "regex" && query.Query != "" {
		prepared.expression, err = regexp.Compile(query.Query)
		if err != nil {
			return preparedSearch{}, fmt.Errorf("%w: invalid regular expression: %v", ErrInvalidQuery, err)
		}
		if parsed, parseErr := syntax.Parse(query.Query, syntax.Perl); parseErr == nil {
			trigramQuery = regexTrigramQuery(parsed)
		}
	} else if mode == "exact" {
		if candidate := trigramCandidate(query.Query); candidate != "" {
			trigramQuery = ftsPhrase(candidate)
		}
	}
	if mode == "fulltext" && query.Query != "" {
		prepared.index = "search_words"
		prepared.indexQuery = query.Query
	} else if trigramQuery != "" {
		prepared.index = "search_trigrams"
		prepared.indexQuery = trigramQuery
	}
	prepared.position, err = decodeCursor(ctx, s.db, query.Cursor, "events", "1=1")
	if err != nil {
		return preparedSearch{}, err
	}
	if prepared.index != "" {
		prepared.broad, err = s.candidateIsBroad(ctx, prepared.index, prepared.indexQuery)
		if err != nil {
			if prepared.index == "search_words" && (strings.Contains(err.Error(), "fts5:") || strings.Contains(err.Error(), "unterminated string") || strings.Contains(err.Error(), "no such column:")) {
				return preparedSearch{}, fmt.Errorf("%w: invalid full-text syntax", ErrInvalidQuery)
			}
			return preparedSearch{}, err
		}
	}
	return prepared, nil
}

func (s *Store) candidateIsBroad(ctx context.Context, index, query string) (bool, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT rowid FROM "+index+" WHERE "+index+" MATCH ? LIMIT 257", query)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
	}
	return count == 257, rows.Err()
}

func (s *Store) collectSearch(ctx context.Context, prepared preparedSearch) (SearchPage, error) {
	limit := pageLimit(prepared.query.Limit)
	page := SearchPage{Results: []SearchResult{}}
	seen := map[int64]bool{}
	before := prepared.position.Last
	batchSize := max(256, limit*2)
	deadline := time.Now().Add(s.searchBudget)
	for len(page.Results) <= limit {
		statement, args := searchStatement(prepared, before, batchSize)
		rows, err := s.db.QueryContext(ctx, statement, args...)
		if err != nil {
			return SearchPage{}, err
		}
		batchEvents, lastEventID, collectErr := collectSearchBatch(ctx, rows, prepared, limit, &page, seen)
		rows.Close()
		if collectErr != nil {
			return SearchPage{}, collectErr
		}
		if len(page.Results) > limit || batchEvents < batchSize {
			break
		}
		before = lastEventID
		if time.Now().After(deadline) {
			// Every Event from before up has been read; the next page starts below it.
			page.NextCursor = encodeCursor(cursor{Last: before, Max: prepared.position.Max})
			page.Partial = true
			return page, nil
		}
	}
	if len(page.Results) > limit {
		page.Results = page.Results[:limit]
		page.NextCursor = encodeCursor(cursor{Last: page.Results[len(page.Results)-1].EventID, Max: prepared.position.Max})
	}
	return page, nil
}

func collectSearchBatch(ctx context.Context, rows *sql.Rows, prepared preparedSearch, limit int, page *SearchPage, seen map[int64]bool) (int, int64, error) {
	batchEvents := 0
	var lastEventID int64
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}
		var result SearchResult
		var text string
		if err := rows.Scan(&result.EventID, &result.TraceID, &result.TraceTitle, &result.Harness, &result.Repository, &result.Kind, &result.Role, &result.Model, &result.Tool, &result.Timestamp, &text); err != nil {
			return 0, 0, err
		}
		if result.EventID != lastEventID {
			batchEvents++
			lastEventID = result.EventID
		}
		if seen[result.EventID] || !searchMatches(prepared, text) {
			continue
		}
		seen[result.EventID] = true
		if prepared.expression != nil {
			location := prepared.expression.FindStringIndex(text)
			result.Snippet = searchtext.SnippetRange(text, location[0], location[1])
		} else {
			result.Snippet = searchtext.Snippet(text, strings.Trim(prepared.query.Query, `"`))
		}
		page.Results = append(page.Results, result)
		if len(page.Results) > limit {
			break
		}
	}
	return batchEvents, lastEventID, rows.Err()
}

func searchMatches(prepared preparedSearch, text string) bool {
	return prepared.query.Query == "" || prepared.mode == "fulltext" || prepared.mode == "exact" && strings.Contains(text, prepared.query.Query) || prepared.mode == "regex" && prepared.expression.MatchString(text)
}

func searchStatement(prepared preparedSearch, before int64, batchSize int) (string, []any) {
	if prepared.index == "" {
		return eventFirstSearchStatement(prepared, before, batchSize)
	}
	if !prepared.broad {
		return selectiveSearchStatement(prepared, before)
	}
	return candidateFirstSearchStatement(prepared, before, batchSize)
}

func selectiveSearchStatement(prepared preparedSearch, before int64) (string, []any) {
	query := prepared.query
	joins := ` FROM events e JOIN traces t ON t.id=e.trace_id JOIN event_observations o ON o.event_id=e.id JOIN observation_revisions ox ON ox.observation_id=o.id JOIN trace_revisions rev ON rev.id=ox.revision_id LEFT JOIN repositories repo ON repo.id=t.repository_id JOIN search_chunks sc ON sc.observation_id=o.id AND sc.revision_id=rev.id JOIN ` + prepared.index + ` ON ` + prepared.index + `.rowid=sc.id`
	where := []string{"e.id<=?", "e.id<?", prepared.index + " MATCH ?"}
	args := []any{prepared.position.Max, before, prepared.indexQuery}
	filters := []struct {
		value  string
		clause string
	}{
		{query.Harness, "t.harness=?"}, {query.Model, "o.model=?"}, {query.Repository, "(repo.remote=? OR EXISTS(SELECT 1 FROM repository_paths rp WHERE rp.repository_id=repo.id AND rp.path=?))"},
		{query.After, "o.event_time>=?"}, {query.Before, "o.event_time<=?"}, {query.Role, "o.role=?"}, {query.Kind, "o.kind=?"}, {query.Tool, "o.tool=?"},
	}
	for _, filter := range filters {
		if filter.value == "" {
			continue
		}
		where = append(where, filter.clause)
		args = append(args, filter.value)
		if filter.clause == "(repo.remote=? OR EXISTS(SELECT 1 FROM repository_paths rp WHERE rp.repository_id=repo.id AND rp.path=?))" {
			args = append(args, filter.value)
		}
	}
	if query.Machine != "" {
		where = append(where, "EXISTS(SELECT 1 FROM revision_machines rm WHERE rm.revision_id=rev.id AND rm.machine_id=?)")
		args = append(args, query.Machine)
	}
	return `SELECT e.id,t.id,` + displayTitle + `,t.harness,COALESCE(repo.remote,repo.root,''),o.kind,o.role,o.model,o.tool,o.event_time,ox.searchable_text` + joins + " WHERE " + strings.Join(where, " AND ") + " ORDER BY e.id DESC,o.id DESC", args
}

func candidateFirstSearchStatement(prepared preparedSearch, before int64, batchSize int) (string, []any) {
	query := prepared.query
	joins := " FROM search_chunks sc INDEXED BY search_chunks_event JOIN indexed_chunks ix ON ix.id=sc.id"
	where := []string{"sc.event_id<=?", "sc.event_id<?"}
	args := []any{prepared.indexQuery, prepared.position.Max, before}
	if query.Harness != "" || query.Repository != "" {
		joins += " JOIN events matched_event ON matched_event.id=sc.event_id JOIN traces matched_trace ON matched_trace.id=matched_event.trace_id LEFT JOIN repositories matched_repo ON matched_repo.id=matched_trace.repository_id"
	}
	if query.Harness != "" {
		where = append(where, "matched_trace.harness=?")
		args = append(args, query.Harness)
	}
	if query.Repository != "" {
		where = append(where, "(matched_repo.remote=? OR EXISTS(SELECT 1 FROM repository_paths rp WHERE rp.repository_id=matched_repo.id AND rp.path=?))")
		args = append(args, query.Repository, query.Repository)
	}
	if query.Model != "" || query.After != "" || query.Before != "" || query.Role != "" || query.Kind != "" || query.Tool != "" {
		joins += " JOIN event_observations matched_observation ON matched_observation.id=sc.observation_id"
		filters := []struct {
			value  string
			clause string
		}{
			{query.Model, "matched_observation.model=?"}, {query.After, "matched_observation.event_time>=?"}, {query.Before, "matched_observation.event_time<=?"}, {query.Role, "matched_observation.role=?"}, {query.Kind, "matched_observation.kind=?"}, {query.Tool, "matched_observation.tool=?"},
		}
		for _, filter := range filters {
			if filter.value != "" {
				where = append(where, filter.clause)
				args = append(args, filter.value)
			}
		}
	}
	if query.Machine != "" {
		where = append(where, "EXISTS(SELECT 1 FROM revision_machines rm WHERE rm.revision_id=sc.revision_id AND rm.machine_id=?)")
		args = append(args, query.Machine)
	}
	outerWhere, outerArgs := observationSearchFilters(query)
	statement := `WITH indexed_chunks(id) AS MATERIALIZED (
		SELECT rowid FROM ` + prepared.index + ` WHERE ` + prepared.index + ` MATCH ?
	), event_batch AS MATERIALIZED (
		SELECT sc.event_id` + joins + ` WHERE ` + strings.Join(where, " AND ") + `
		GROUP BY sc.event_id ORDER BY sc.event_id DESC LIMIT ?
	)
	SELECT DISTINCT e.id,t.id,` + displayTitle + `,t.harness,COALESCE(repo.remote,repo.root,''),
		o.kind,o.role,o.model,o.tool,o.event_time,ox.searchable_text
	FROM event_batch b
	JOIN search_chunks sc INDEXED BY search_chunks_event ON sc.event_id=b.event_id
	JOIN indexed_chunks ix ON ix.id=sc.id
	JOIN events e ON e.id=sc.event_id
	JOIN traces t ON t.id=e.trace_id
	JOIN event_observations o ON o.id=sc.observation_id
	JOIN observation_revisions ox ON ox.observation_id=sc.observation_id AND ox.revision_id=sc.revision_id
	JOIN trace_revisions rev ON rev.id=sc.revision_id
	LEFT JOIN repositories repo ON repo.id=t.repository_id
	WHERE ` + strings.Join(outerWhere, " AND ") + ` ORDER BY e.id DESC,o.id DESC`
	args = append(args, batchSize)
	args = append(args, outerArgs...)
	return statement, args
}

func eventFirstSearchStatement(prepared preparedSearch, before int64, batchSize int) (string, []any) {
	query := prepared.query
	traceWhere := []string{"e.id<=?", "e.id<?"}
	traceArgs := []any{prepared.position.Max, before}
	if query.Harness != "" {
		traceWhere = append(traceWhere, "t.harness=?")
		traceArgs = append(traceArgs, query.Harness)
	}
	if query.Repository != "" {
		traceWhere = append(traceWhere, "(repo.remote=? OR EXISTS(SELECT 1 FROM repository_paths rp WHERE rp.repository_id=repo.id AND rp.path=?))")
		traceArgs = append(traceArgs, query.Repository, query.Repository)
	}
	observationWhere, observationArgs := observationSearchFilters(query)
	innerWhere := append(append([]string{}, observationWhere...), "o.event_id=e.id")
	statement := `WITH event_batch AS MATERIALIZED (
		SELECT e.id FROM events e
		JOIN traces t ON t.id=e.trace_id
		LEFT JOIN repositories repo ON repo.id=t.repository_id
		WHERE ` + strings.Join(traceWhere, " AND ") + ` AND EXISTS(
			SELECT 1 FROM event_observations o
			JOIN observation_revisions ox ON ox.observation_id=o.id
			JOIN trace_revisions rev ON rev.id=ox.revision_id
			WHERE ` + strings.Join(innerWhere, " AND ") + `
		) ORDER BY e.id DESC LIMIT ?
	)
	SELECT DISTINCT e.id,t.id,` + displayTitle + `,t.harness,COALESCE(repo.remote,repo.root,''),
		o.kind,o.role,o.model,o.tool,o.event_time,ox.searchable_text
	FROM event_batch b
	JOIN events e ON e.id=b.id
	JOIN traces t ON t.id=e.trace_id
	JOIN event_observations o ON o.event_id=e.id
	JOIN observation_revisions ox ON ox.observation_id=o.id
	JOIN trace_revisions rev ON rev.id=ox.revision_id
	LEFT JOIN repositories repo ON repo.id=t.repository_id
	WHERE ` + strings.Join(observationWhere, " AND ") + ` ORDER BY e.id DESC,o.id DESC`
	args := append(traceArgs, observationArgs...)
	args = append(args, batchSize)
	args = append(args, observationArgs...)
	return statement, args
}

func observationSearchFilters(query SearchQuery) ([]string, []any) {
	where := []string{"1=1"}
	args := []any{}
	filters := []struct {
		value  string
		clause string
	}{
		{query.Model, "o.model=?"}, {query.After, "o.event_time>=?"}, {query.Before, "o.event_time<=?"}, {query.Role, "o.role=?"}, {query.Kind, "o.kind=?"}, {query.Tool, "o.tool=?"},
	}
	for _, filter := range filters {
		if filter.value != "" {
			where = append(where, filter.clause)
			args = append(args, filter.value)
		}
	}
	if query.Machine != "" {
		where = append(where, "EXISTS(SELECT 1 FROM revision_machines rm WHERE rm.revision_id=rev.id AND rm.machine_id=?)")
		args = append(args, query.Machine)
	}
	return where, args
}

// maxTrigramPhrases caps the index query built from a regex; a larger one costs more
// to evaluate than it saves.
const maxTrigramPhrases = 256

// regexTrigramQuery returns an FTS5 query for the trigram index that every chunk
// holding a match satisfies, or "" when the index cannot narrow the expression.
// The longest literal every match contains always fits in one chunk. codesearch
// adds the trigrams a match must contain, such as ("acd" OR "bcd") for [ab]cd, but
// the index matches one chunk at a time and trigrams from distant parts of a match
// can sit in different chunks, so they are used only when every match fits inside
// the chunk overlap.
func regexTrigramQuery(expression *syntax.Regexp) string {
	var parts []string
	literal := trigramCandidate(requiredLiteral(expression))
	if longest := maxMatchBytes(expression); longest >= 0 && longest <= searchtext.ChunkOverlap-utf8.UTFMax {
		// The index ignores case; without (?i) codesearch would track every case
		// variant and soon give up on the trigrams after the first few letters.
		lowerFoldedLiterals(expression)
		if query, phrases := ftsQuery(index.RegexpQuery(expression), asciiLower(literal)); query != "" && phrases <= maxTrigramPhrases {
			parts = append(parts, "("+query+")")
		}
	}
	if literal != "" {
		parts = append(parts, ftsPhrase(literal))
	}
	return strings.Join(parts, " AND ")
}

// lowerFoldedLiterals rewrites case-insensitive ASCII literals as lower-case
// case-sensitive ones. Only the index query reads the rewritten expression.
func lowerFoldedLiterals(expression *syntax.Regexp) {
	if expression.Op == syntax.OpLiteral && expression.Flags&syntax.FoldCase != 0 {
		for _, r := range expression.Rune {
			if r >= utf8.RuneSelf {
				return
			}
		}
		for i, r := range expression.Rune {
			if 'A' <= r && r <= 'Z' {
				expression.Rune[i] = r + 'a' - 'A'
			}
		}
		expression.Flags &^= syntax.FoldCase
	}
	for _, sub := range expression.Sub {
		lowerFoldedLiterals(sub)
	}
}

// ftsQuery writes a codesearch query in FTS5 syntax and counts its phrases. It
// returns "" when the query cannot narrow anything beyond the literal the caller
// also requires. codesearch works on bytes and the index on characters, so a
// trigram that is not three whole characters is left out, which only widens the
// query. The index ignores case, so case variants merge.
func ftsQuery(query *index.Query, literal string) (string, int) {
	if query.Op != index.QAnd && query.Op != index.QOr {
		return "", 0
	}
	var parts []string
	seen := map[string]bool{}
	phrases := 0
	add := func(part string, count int) {
		if !seen[part] {
			seen[part] = true
			parts = append(parts, part)
			phrases += count
		}
	}
	for _, trigram := range query.Trigram {
		// A trigram that is not three whole characters cannot be looked up; one
		// inside the literal is already required.
		if !utf8.ValidString(trigram) || utf8.RuneCountInString(trigram) != 3 || strings.Contains(literal, asciiLower(trigram)) {
			if query.Op == index.QOr {
				return "", 0
			}
			continue
		}
		add(ftsPhrase(asciiLower(trigram)), 1)
	}
	for _, sub := range query.Sub {
		part, count := ftsQuery(sub, literal)
		if part == "" {
			if query.Op == index.QOr {
				return "", 0
			}
			continue
		}
		add("("+part+")", count)
	}
	operator := " AND "
	if query.Op == index.QOr {
		operator = " OR "
	}
	return strings.Join(parts, operator), phrases
}

func ftsPhrase(text string) string {
	return `"` + strings.ReplaceAll(text, `"`, `""`) + `"`
}

func asciiLower(text string) string {
	return strings.Map(func(r rune) rune {
		if 'A' <= r && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}, text)
}

// maxMatchBytes returns the most bytes one match of the expression can span, or -1
// when there is no limit.
func maxMatchBytes(expression *syntax.Regexp) int {
	switch expression.Op {
	case syntax.OpEmptyMatch, syntax.OpNoMatch, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return 0
	case syntax.OpLiteral:
		// A case-insensitive letter can match a longer encoding, such as the Kelvin sign.
		return len(expression.Rune) * utf8.UTFMax
	case syntax.OpCharClass, syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		return utf8.UTFMax
	case syntax.OpCapture, syntax.OpQuest:
		return maxMatchBytes(expression.Sub[0])
	case syntax.OpRepeat:
		if longest := maxMatchBytes(expression.Sub[0]); expression.Max >= 0 && longest >= 0 {
			return longest * expression.Max
		}
	case syntax.OpConcat, syntax.OpAlternate:
		total := 0
		for _, sub := range expression.Sub {
			longest := maxMatchBytes(sub)
			if longest < 0 {
				return -1
			}
			if expression.Op == syntax.OpConcat {
				total += longest
			} else {
				total = max(total, longest)
			}
		}
		return total
	}
	return -1
}

// requiredLiteral returns the longest text every match of a regular expression
// contains, so the trigram index can narrow a regex with no literal prefix, such as
// (?i)needle or [0-9]+-needle. The trigram index folds case like (?i) does for ASCII
// letters (ſ and the Kelvin sign included), so a case-insensitive literal qualifies
// only when it is ASCII.
func requiredLiteral(expression *syntax.Regexp) string {
	switch expression.Op {
	case syntax.OpLiteral:
		if expression.Flags&syntax.FoldCase != 0 {
			for _, r := range expression.Rune {
				if r >= utf8.RuneSelf {
					return ""
				}
			}
		}
		return string(expression.Rune)
	case syntax.OpCapture, syntax.OpPlus:
		return requiredLiteral(expression.Sub[0])
	case syntax.OpRepeat:
		if expression.Min > 0 {
			return requiredLiteral(expression.Sub[0])
		}
	case syntax.OpConcat:
		longest := ""
		for _, sub := range expression.Sub {
			if literal := requiredLiteral(sub); utf8.RuneCountInString(literal) > utf8.RuneCountInString(longest) {
				longest = literal
			}
		}
		return longest
	}
	return ""
}

func trigramCandidate(value string) string {
	if utf8.RuneCountInString(value) < 3 {
		return ""
	}
	end := min(len(value), 512)
	for end < len(value) && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end]
}

func searchDate(value string, endOfDay bool) (string, error) {
	if value == "" {
		return "", nil
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		if endOfDay {
			parsed = parsed.Add(24*time.Hour - time.Nanosecond)
		}
		return parsed.UTC().Format("2006-01-02T15:04:05.000000000Z"), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", err
	}
	return parsed.UTC().Format("2006-01-02T15:04:05.000000000Z"), nil
}
