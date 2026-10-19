package server

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"
)

// TestOpenAPIDocumentsExactlyTheAPIRoutes keeps openapi.json and apiRoutes in
// agreement in both directions, so the agent contract cannot drift.
func TestOpenAPIDocumentsExactlyTheAPIRoutes(t *testing.T) {
	var doc struct {
		OpenAPI string `json:"openapi"`
		Paths   map[string]map[string]struct {
			Tags        []string              `json:"tags"`
			OperationID string                `json:"operationId"`
			Security    []map[string][]string `json:"security"`
			Responses   map[string]any        `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(openAPIDocument, &doc); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.1.") {
		t.Fatalf("openapi = %q", doc.OpenAPI)
	}
	documented := map[string]bool{}
	operationIDs := map[string]bool{}
	for path, operations := range doc.Paths {
		for method, operation := range operations {
			pattern := strings.ToUpper(method) + " " + path
			documented[pattern] = true
			if len(operation.Tags) == 0 || operation.OperationID == "" || operationIDs[operation.OperationID] {
				t.Errorf("%s needs a tag and a unique operationId", pattern)
			}
			operationIDs[operation.OperationID] = true
			if !slices.ContainsFunc(operation.Security, func(s map[string][]string) bool { _, ok := s["adminToken"]; return ok }) {
				t.Errorf("%s does not require adminToken", pattern)
			}
			if !slices.ContainsFunc(slices.Collect(maps.Keys(operation.Responses)), func(code string) bool { return strings.HasPrefix(code, "4") }) {
				t.Errorf("%s documents no error response", pattern)
			}
		}
	}
	registered := map[string]bool{}
	for _, route := range (&application{}).apiRoutes() {
		registered[route.pattern] = true
		if !documented[route.pattern] {
			t.Errorf("route %s is missing from openapi.json", route.pattern)
		}
	}
	for pattern := range documented {
		if !registered[pattern] {
			t.Errorf("openapi.json documents %s, which is not an API route", pattern)
		}
	}
}
