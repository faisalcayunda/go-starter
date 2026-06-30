package core

import (
	"context"
	"sync"
)

// Repository adalah port penyimpanan pesanan milik domain orders. Service
// bergantung pada interface ini agar mudah dites & agar storage in-memory contoh
// bisa ditukar dengan DB nyata tanpa menyentuh service.
type Repository interface {
	// Save menyimpan (atau menimpa) sebuah pesanan.
	Save(ctx context.Context, o Order) error
}

// memRepo adalah implementasi Repository berbasis map in-memory — murni stdlib.
type memRepo struct {
	mu     sync.Mutex
	orders map[string]Order
}

// NewMemRepository membuat Repository in-memory kosong untuk domain orders.
func NewMemRepository() Repository {
	return &memRepo{orders: make(map[string]Order)}
}

// Save menyimpan pesanan dengan penguncian tulis.
func (r *memRepo) Save(_ context.Context, o Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[o.ID] = o
	return nil
}
