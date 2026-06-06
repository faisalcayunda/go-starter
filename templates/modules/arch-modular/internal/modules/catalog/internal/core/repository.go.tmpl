package core

import (
	"context"
	"sync"
)

// Repository adalah port penyimpanan produk milik domain catalog. Service
// bergantung pada interface ini (bukan implementasi konkret) agar mudah dites &
// agar storage in-memory contoh bisa ditukar dengan DB nyata tanpa menyentuh
// service.
type Repository interface {
	// FindByID mengembalikan produk untuk id, atau ErrNotFound bila tak ada.
	FindByID(ctx context.Context, id string) (Product, error)
}

// memRepo adalah implementasi Repository berbasis map in-memory — murni stdlib,
// tanpa database. Cocok sebagai contoh siap-jalan & untuk test.
type memRepo struct {
	mu       sync.RWMutex
	products map[string]Product
}

// NewMemRepository membuat Repository in-memory yang sudah terisi beberapa produk
// contoh, sehingga endpoint langsung mengembalikan data tanpa setup eksternal.
func NewMemRepository() Repository {
	return &memRepo{
		products: map[string]Product{
			"p-1": {ID: "p-1", Name: "Kopi Gayo 250g", PriceCents: 8500000, SKU: "SKU-KOPI-001"},
			"p-2": {ID: "p-2", Name: "Teh Melati 100g", PriceCents: 3200000, SKU: "SKU-TEH-002"},
		},
	}
}

// FindByID mencari produk pada map dengan penguncian baca.
func (r *memRepo) FindByID(_ context.Context, id string) (Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.products[id]
	if !ok {
		return Product{}, ErrNotFound
	}
	return p, nil
}
