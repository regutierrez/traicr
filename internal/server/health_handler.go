// Package server owns the Traicr HTTP server lifecycle and routes.
package server

import (
	"encoding/json"
	"net/http"
)

func serveProcessHealth(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(struct {
		Status string `json:"status"`
	}{
		Status: "ok",
	})
}
