package server

import (
	"compress/gzip"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// gzipWriters reuses compressors: each one holds several hundred KB of state.
var gzipWriters = sync.Pool{New: func() any {
	writer, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
	return writer
}}

// compress gzips text responses to GET requests from clients that accept
// gzip. JSON, HTML, JavaScript, and CSS shrink several times over; images,
// fonts, and downloads are already compressed or opaque, so they pass through.
// Range requests pass through because a byte range of the original cannot be
// served from a compressed body.
func compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Add("Vary", "Accept-Encoding")
		if r.Header.Get("Range") != "" || !acceptsGzip(r.Header.Get("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}
		response := &gzipResponse{ResponseWriter: w}
		defer response.close()
		next.ServeHTTP(response, r)
	})
}

func acceptsGzip(header string) bool {
	for _, part := range strings.Split(header, ",") {
		name, params, _ := strings.Cut(part, ";")
		if name = strings.TrimSpace(name); name != "gzip" && name != "*" {
			continue
		}
		if q, ok := strings.CutPrefix(strings.ReplaceAll(params, " ", ""), "q="); ok {
			if value, err := strconv.ParseFloat(q, 64); err == nil && value == 0 {
				return false
			}
		}
		return true
	}
	return false
}

func compressible(contentType string) bool {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	switch {
	case strings.HasPrefix(mediaType, "text/"):
		return true
	case mediaType == "application/json", mediaType == "application/x-ndjson", mediaType == "application/javascript", mediaType == "image/svg+xml":
		return true
	}
	return false
}

// gzipResponse decides whether to compress when the handler writes its
// header, from the status and Content-Type the handler set.
type gzipResponse struct {
	http.ResponseWriter
	writer  *gzip.Writer
	decided bool
}

func (response *gzipResponse) WriteHeader(status int) {
	if !response.decided {
		response.decided = true
		header := response.Header()
		if status >= 200 && status != http.StatusNoContent && status != http.StatusNotModified &&
			header.Get("Content-Encoding") == "" && compressible(header.Get("Content-Type")) {
			header.Del("Content-Length")
			header.Set("Content-Encoding", "gzip")
			response.writer = gzipWriters.Get().(*gzip.Writer)
			response.writer.Reset(response.ResponseWriter)
		}
	}
	response.ResponseWriter.WriteHeader(status)
}

func (response *gzipResponse) Write(data []byte) (int, error) {
	if !response.decided {
		// net/http would sniff the type on this first write; do it here so the choice can use it.
		if response.Header().Get("Content-Type") == "" {
			response.Header().Set("Content-Type", http.DetectContentType(data))
		}
		response.WriteHeader(http.StatusOK)
	}
	if response.writer != nil {
		return response.writer.Write(data)
	}
	return response.ResponseWriter.Write(data)
}

// Flush sends what is compressed so far, so a streamed response still arrives as it is written.
func (response *gzipResponse) Flush() {
	if response.writer != nil {
		_ = response.writer.Flush()
	}
	_ = http.NewResponseController(response.ResponseWriter).Flush()
}

func (response *gzipResponse) Unwrap() http.ResponseWriter {
	return response.ResponseWriter
}

func (response *gzipResponse) close() {
	if response.writer == nil {
		return
	}
	_ = response.writer.Close()
	response.writer.Reset(io.Discard)
	gzipWriters.Put(response.writer)
}
