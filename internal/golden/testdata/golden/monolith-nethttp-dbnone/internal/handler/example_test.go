package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHello memverifikasi handler contoh memakai net/http/httptest (tanpa DB,
// tanpa server nyata) — sehingga `go test ./...` hijau offline.
func TestHello(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantMsg string
	}{
		{name: "default", query: "", wantMsg: "hello, world"},
		{name: "with name", query: "?name=gopher", wantMsg: "hello, gopher"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/hello"+tc.query, nil)
			rec := httptest.NewRecorder()

			Hello(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
			}

			var got helloResponse
			if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got.Message != tc.wantMsg {
				t.Errorf("message = %q, want %q", got.Message, tc.wantMsg)
			}
		})
	}
}
