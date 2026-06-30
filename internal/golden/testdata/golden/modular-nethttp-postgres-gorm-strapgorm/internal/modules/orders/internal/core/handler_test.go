package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/shopmod/internal/shared/contract"
)

// fakeCatalog adalah implementasi tiruan contract.Catalog untuk menguji domain
// orders SECARA TERISOLASI — tanpa meng-import domain catalog & tanpa DB. Ini
// membuktikan nilai boundary: orders dites lewat kontrak, bukan lewat tetangga.
type fakeCatalog struct {
	products map[string]contract.Product
}

func (f fakeCatalog) Lookup(_ context.Context, id string) (contract.Product, error) {
	p, ok := f.products[id]
	if !ok {
		return contract.Product{}, contract.ErrNotFound
	}
	return p, nil
}

// TestCreateOrder memverifikasi endpoint orders memakai fakeCatalog + httptest.
func TestCreateOrder(t *testing.T) {
	cat := fakeCatalog{products: map[string]contract.Product{
		"p-1": {ID: "p-1", Name: "Kopi", PriceCents: 8500000},
	}}
	h := NewHandler(NewService(NewMemRepository(), cat))
	mux := http.NewServeMux()
	h.Routes(mux)

	t.Run("produk valid", func(t *testing.T) {
		body := strings.NewReader(`{"product_id":"p-1","quantity":2}`)
		req := httptest.NewRequest(http.MethodPost, "/api/orders", body)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
		}
		var got orderResponse
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got.TotalCents != 17000000 {
			t.Errorf("total = %d, want %d (harga katalog × qty)", got.TotalCents, 17000000)
		}
	})

	t.Run("produk tidak ada di katalog", func(t *testing.T) {
		body := strings.NewReader(`{"product_id":"tidak-ada","quantity":1}`)
		req := httptest.NewRequest(http.MethodPost, "/api/orders", body)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
		}
	})
}
