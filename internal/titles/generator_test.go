package titles

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/regutierrez/traicr/internal/config"
)

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestGeneratorSendsChatCompletionAndCleansReply(t *testing.T) {
	var request *http.Request
	var body map[string]any
	client := &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		request = r
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		reply := `{"choices":[{"message":{"role":"assistant","content":"\"Rename traces from the CLI.\"","reasoning_content":"thinking"}}]}`
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(reply))}, nil
	})}
	cfg := config.TitleConfig{APIURL: "https://opencode.ai/zen/go/v1", APIKey: "secret", Model: "longcat-2.5-preview-free", ReasoningEffort: "minimal"}
	title, err := NewGenerator(cfg, "v1.2.3", client).Generate(context.Background(), 42, "User message 1:\nrename traces")
	if err != nil || title != "Rename traces from the CLI" {
		t.Fatalf("title = %q, %v", title, err)
	}
	if request.URL.String() != "https://opencode.ai/zen/go/v1/chat/completions" || request.Method != http.MethodPost {
		t.Fatalf("request %s %s", request.Method, request.URL)
	}
	for header, want := range map[string]string{"Authorization": "Bearer secret", "User-Agent": "traicr/v1.2.3", "Content-Type": "application/json", "X-Opencode-Session": "traicr-title-42"} {
		if got := request.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	messages, _ := body["messages"].([]any)
	if body["model"] != "longcat-2.5-preview-free" || body["reasoning_effort"] != "minimal" || body["max_tokens"] != float64(maxCompletionTokens) || len(messages) != 2 {
		t.Fatalf("body = %v", body)
	}
	if first, _ := messages[0].(map[string]any); first["role"] != "system" || first["content"] != SystemPrompt {
		t.Fatalf("system message = %v", messages[0])
	}
}

func TestGeneratorOmitsProviderSpecificFieldsElsewhere(t *testing.T) {
	var header http.Header
	var raw []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Clone()
		raw, _ = io.ReadAll(r.Body)
		w.Write([]byte(`{"choices":[{"message":{"content":"Local title"}}]}`))
	}))
	defer server.Close()
	title, err := NewGenerator(config.TitleConfig{APIURL: server.URL + "/v1", Model: "local"}, "dev", nil).Generate(context.Background(), 1, "prompt")
	if err != nil || title != "Local title" {
		t.Fatalf("title = %q, %v", title, err)
	}
	if header.Get("Authorization") != "" || header.Get("X-Opencode-Session") != "" || strings.Contains(string(raw), "reasoning_effort") {
		t.Fatalf("unexpected fields: headers=%v body=%s", header, raw)
	}
}

func TestGeneratorClassifiesErrors(t *testing.T) {
	for _, test := range []struct {
		status     int
		retryAfter string
		rejected   bool
		wait       time.Duration
	}{
		{status: 400, rejected: true},
		{status: 413, rejected: true},
		{status: 401},
		{status: 404},
		{status: 429, retryAfter: "120", wait: 2 * time.Minute},
		{status: 503},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if test.retryAfter != "" {
				w.Header().Set("Retry-After", test.retryAfter)
			}
			w.WriteHeader(test.status)
			w.Write([]byte(`{"type":"error","error":{"type":"X","message":"provider says no"}}`))
		}))
		_, err := NewGenerator(config.TitleConfig{APIURL: server.URL, Model: "m"}, "dev", nil).Generate(context.Background(), 1, "prompt")
		server.Close()
		var requestErr *RequestError
		if !errors.As(err, &requestErr) || requestErr.Status != test.status || requestErr.Rejected() != test.rejected || requestErr.RetryAfter != test.wait || !strings.Contains(err.Error(), "provider says no") {
			t.Errorf("status %d: err = %#v", test.status, err)
		}
	}
}

func TestGeneratorRetriesOnceWhenTheReplyIsNotATitle(t *testing.T) {
	for _, test := range []struct {
		replies []string
		want    string
		calls   int
	}{
		{replies: []string{"<longcat_tool_call>Bash", "Show the Svelte plugin's value"}, want: "Show the Svelte plugin's value", calls: 2},
		{replies: []string{"<longcat_tool_call>Bash", ""}, want: "", calls: 2},
	} {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			content, _ := json.Marshal(test.replies[calls])
			calls++
			w.Write([]byte(`{"choices":[{"message":{"content":` + string(content) + `}}]}`))
		}))
		title, err := NewGenerator(config.TitleConfig{APIURL: server.URL, Model: "m"}, "dev", nil).Generate(context.Background(), 1, "prompt")
		server.Close()
		if err != nil || title != test.want || calls != test.calls {
			t.Errorf("replies %q: title = %q, err = %v, calls = %d", test.replies, title, err, calls)
		}
	}
}
