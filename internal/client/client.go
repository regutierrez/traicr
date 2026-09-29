// Package client calls the Traicr server's JSON API with the login saved by
// `traicr login`.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/regutierrez/traicr/internal/config"
	"github.com/regutierrez/traicr/internal/store"
)

// ErrNotLoggedIn means the collector configuration has no server login.
var ErrNotLoggedIn = errors.New("not logged in; run traicr login URL and provide a token")

type Client struct {
	server string
	token  string
	http   *http.Client
}

// New returns a client for the saved server. A nil httpClient uses the default.
func New(cfg config.Collector, httpClient *http.Client) (*Client, error) {
	if cfg.ServerURL == "" || cfg.Token == "" {
		return nil, ErrNotLoggedIn
	}
	if _, err := Endpoint(cfg.ServerURL, "/"); err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	copied := *httpClient
	// Never replay the bearer token to wherever a redirect points.
	copied.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{server: cfg.ServerURL, token: cfg.Token, http: &copied}, nil
}

// Endpoint joins an API path onto the saved server URL after checking that
// the URL is safe to send the admin token to.
func Endpoint(server, apiPath string) (string, error) {
	parsed, err := url.Parse(server)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("server URL must be an absolute http or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("server URL must not contain credentials, a query, or a fragment")
	}
	parsed.Path = path.Join(parsed.Path, apiPath)
	return parsed.String(), nil
}

// SetTitleOverride sets a trace's manual title, or clears it when title is empty.
func (c *Client) SetTitleOverride(ctx context.Context, id int64, title string) (store.Trace, error) {
	var trace store.Trace
	err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/v1/traces/%d", id), map[string]string{"title_override": title}, &trace)
	return trace, err
}

// Error is an error response from the server API.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("server returned %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("server returned %d (%s): %s", e.Status, e.Code, e.Message)
}

func (c *Client) do(ctx context.Context, method, apiPath string, body, result any) error {
	endpoint, err := Endpoint(c.server, apiPath)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return responseError(response)
	}
	if result == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(result); err != nil {
		return fmt.Errorf("decode server response: %w", err)
	}
	return nil
}

func responseError(response *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &payload) == nil && payload.Error.Code != "" {
		return &Error{Status: response.StatusCode, Code: payload.Error.Code, Message: payload.Error.Message}
	}
	return &Error{Status: response.StatusCode, Message: strings.TrimSpace(string(data))}
}
