package request

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONStrictAndBounded(t *testing.T) {
	for _, tc := range []struct {
		name, body, contentType string
		status                  int
	}{
		{"valid", `{"name":"BPM"}`, "application/json; charset=utf-8", 200},
		{"unknown field", `{"name":"BPM","role":"admin"}`, "application/json", 400},
		{"two documents", `{"name":"BPM"}{"name":"Other"}`, "application/json", 400},
		{"too large", `{"name":"Nombre demasiado largo"}`, "application/json", 400},
		{"wrong type", `{"name":"BPM"}`, "text/plain", 415},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()
			var target struct {
				Name string `json:"name"`
			}
			limit := int64(64)
			if tc.name == "too large" {
				limit = 12
			}
			ok := DecodeJSON(w, r, &target, limit)
			if tc.status == 200 {
				if !ok || target.Name != "BPM" {
					t.Fatal("valid request rejected")
				}
			} else if ok || w.Code != tc.status {
				t.Fatalf("status = %d, success = %v", w.Code, ok)
			}
		})
	}
}
