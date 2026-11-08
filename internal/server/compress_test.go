package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompressOnlyTextTypes(t *testing.T) {
	cases := []struct {
		contentType string
		body        string
		zipped      bool
	}{
		{"text/html; charset=utf-8", "<p>hi</p>", true},
		{"text/css", "p{}", true},
		{"application/json", "{}", true},
		{"application/x-ndjson", "{}\n", true},
		{"application/javascript", "1", true},
		{"image/svg+xml", "<svg/>", true},
		{"application/octet-stream", "raw", false},
		{"application/zip", "PK", false},
		{"font/woff2", "wOF2", false},
		{"image/png", "\x89PNG", false},
		// Without a Content-Type, the type is sniffed from the first write, as net/http would.
		{"", "<!doctype html><p>hi</p>", true},
		{"", "\x89PNG\r\n\x1a\n", false},
	}
	for _, test := range cases {
		handler := compress(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if test.contentType != "" {
				w.Header().Set("Content-Type", test.contentType)
			}
			_, _ = w.Write([]byte(test.body))
		}))
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if got := response.Header().Get("Content-Encoding") == "gzip"; got != test.zipped {
			t.Errorf("Content-Type %q, body %q: gzipped = %v, want %v", test.contentType, test.body, got, test.zipped)
		}
	}
}
