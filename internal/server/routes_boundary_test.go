package server_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestAPIRejectsNonPositiveAndMalformedIDs(t *testing.T) {
	handler := testHandler(t, false)
	for _, id := range []string{"0", "-1", "not-a-number"} {
		response := request(handler, http.MethodGet, "/api/v1/traces/"+id, nil, adminToken)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_id"`) {
			t.Fatalf("id %q: %d %s", id, response.Code, response.Body)
		}
	}
}

func TestAPISearchLimitBoundaries(t *testing.T) {
	handler := testHandler(t, false)
	for _, test := range []struct {
		value  string
		status int
		code   string
	}{
		{"malformed", http.StatusBadRequest, "invalid_limit"},
		{"0", http.StatusBadRequest, "invalid_limit"},
		{"1", http.StatusOK, ""},
		{"200", http.StatusOK, ""},
		{"201", http.StatusBadRequest, "invalid_limit"},
	} {
		response := request(handler, http.MethodGet, "/api/v1/search?limit="+test.value, nil, adminToken)
		if response.Code != test.status {
			t.Fatalf("limit %q: status=%d want=%d body=%s", test.value, response.Code, test.status, response.Body)
		}
		if test.code != "" && !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
			t.Fatalf("limit %q: %s", test.value, response.Body)
		}
	}
}
