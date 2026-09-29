package titles

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/regutierrez/traicr/internal/config"
)

const (
	requestTimeout = 2 * time.Minute
	// Reasoning models spend completion tokens thinking before the title.
	maxCompletionTokens = 2048
)

// Generator asks an OpenAI-compatible chat-completions API for a title.
type Generator struct {
	endpoint  string
	apiKey    string
	model     string
	effort    string
	userAgent string
	// OpenCode routes and caches by a stable per-conversation session ID.
	sessionHeader bool
	http          *http.Client
}

// NewGenerator returns a generator for cfg. A nil client uses the default.
func NewGenerator(cfg config.TitleConfig, version string, client *http.Client) *Generator {
	if client == nil {
		client = &http.Client{}
	}
	host := ""
	if parsed, err := url.Parse(cfg.APIURL); err == nil {
		host = parsed.Hostname()
	}
	return &Generator{
		endpoint:      cfg.APIURL + "/chat/completions",
		apiKey:        cfg.APIKey,
		model:         cfg.Model,
		effort:        cfg.ReasoningEffort,
		userAgent:     "traicr/" + version,
		sessionHeader: host == "opencode.ai" || strings.HasSuffix(host, ".opencode.ai"),
		http:          client,
	}
}

// RequestError is an unsuccessful response from the model API.
type RequestError struct {
	Status     int
	Message    string
	RetryAfter time.Duration
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("title API returned %d: %s", e.Status, e.Message)
}

// Rejected reports whether the API refused this particular request, so
// retrying the same trace will not help. Auth, rate-limit, and server errors
// are not rejections: they would fail for every trace.
func (e *RequestError) Rejected() bool {
	return e.Status == http.StatusBadRequest || e.Status == http.StatusRequestEntityTooLarge || e.Status == http.StatusUnprocessableEntity
}

// unusableReplyAttempts is how many times one excerpt is sent when the model
// replies with something that is not a title, such as a tool call.
const unusableReplyAttempts = 2

// Generate returns a cleaned title for the excerpt, or "" when the model's
// replies have no usable title.
func (g *Generator) Generate(ctx context.Context, traceID int64, prompt string) (string, error) {
	for attempt := 1; ; attempt++ {
		title, err := g.generate(ctx, traceID, prompt)
		if err != nil || title != "" || attempt == unusableReplyAttempts {
			return title, err
		}
	}
}

func (g *Generator) generate(ctx context.Context, traceID int64, prompt string) (string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body := struct {
		Model           string    `json:"model"`
		Messages        []message `json:"messages"`
		MaxTokens       int       `json:"max_tokens"`
		ReasoningEffort string    `json:"reasoning_effort,omitempty"`
	}{
		Model:           g.model,
		Messages:        []message{{Role: "system", Content: SystemPrompt}, {Role: "user", Content: prompt}},
		MaxTokens:       maxCompletionTokens,
		ReasoningEffort: g.effort,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", g.userAgent)
	if g.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+g.apiKey)
	}
	if g.sessionHeader {
		request.Header.Set("x-opencode-session", "traicr-title-"+strconv.FormatInt(traceID, 10))
	}
	response, err := g.http.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		requestErr := &RequestError{Status: response.StatusCode, Message: errorMessage(data)}
		if seconds, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && seconds > 0 {
			requestErr.RetryAfter = time.Duration(seconds) * time.Second
		}
		return "", requestErr
	}
	var reply struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &reply); err != nil {
		return "", fmt.Errorf("decode title API response: %w", err)
	}
	if len(reply.Choices) == 0 {
		return "", errors.New("title API response has no choices")
	}
	return cleanTitle(reply.Choices[0].Message.Content), nil
}

// errorMessage keeps the provider's own message short and never echoes the request.
func errorMessage(data []byte) string {
	var payload struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(data, &payload) == nil && len(payload.Error) > 0 {
		var detail struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(payload.Error, &detail) == nil && detail.Message != "" {
			return truncate(detail.Message, 300)
		}
		var text string
		if json.Unmarshal(payload.Error, &text) == nil && text != "" {
			return truncate(text, 300)
		}
	}
	return truncate(strings.TrimSpace(string(data)), 300)
}
