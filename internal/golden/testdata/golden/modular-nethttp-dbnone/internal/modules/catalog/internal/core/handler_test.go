package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetProduct memverifikasi endpoint catalog memakai repository in-memory +
// net/http/httptest — tanpa DB, tanpa server nyata — sehingga `go test ./...`
// hijau offline.
func TestGetProduct(t *testing.T) {
	h := NewHandler(NewService(NewMemRepository()))
	mux := http.NewServeMux()
	h.Routes(mux)

	t.Run("ditemukan", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/catalog/products/p-1", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var got productResponse
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got.ID != "p-1" || got.Name == "" {
			t.Errorf("respons produk tak sesuai: %+v", got)
		}
	})

	t.Run("tidak ditemukan", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/catalog/products/tidak-ada", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}
