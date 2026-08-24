// Package server owns the Traicr HTTP server lifecycle and routes.
package server

import (
	"encoding/json"
	"net/http"
)

// NewHTTPHandler returns the server routes implemented by the current milestone.
func NewHTTPHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", serveProcessHealth)
	return mux
}

func serveProcessHealth(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(struct {
		Status string `json:"status"`
	}{
		Status: "ok",
	})
}
