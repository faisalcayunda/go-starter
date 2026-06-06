package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/demo-modular/internal/shared/contract"
)

// Service memuat logika bisnis domain orders. Ketergantungannya pada domain
// catalog dinyatakan SEBAGAI INTERFACE (contract.Catalog), bukan paket konkret.
// Compiler bahkan menolak orders meng-import internal core catalog (boundary
// berduri), jadi satu-satunya jalan adalah kontrak ini.
//
// Inilah contoh nyata komunikasi in-process antar-domain: orders → catalog
// dilakukan dengan memanggil method pada interface yang implementasinya
// (catalog) di-inject composition root. Saat ekstraksi ke microservice, cukup
// ganti implementasi catalog dengan klien gRPC/HTTP — service ini tak berubah.
type Service struct {
	repo    Repository
	catalog contract.Catalog
	nextID  func() string
}

// NewService merakit Service dengan repository pesanan dan port catalog. Argumen
// catalog bertipe contract.Catalog (interface) — composition root meneruskan
// implementasi konkret dari domain catalog.
func NewService(repo Repository, catalog contract.Catalog) *Service {
	return &Service{
		repo:    repo,
		catalog: catalog,
		nextID:  newIDGen(),
	}
}

// Create membuat pesanan baru: memverifikasi produk ke domain catalog lewat
// kontrak, menghitung total dari harga katalog, lalu menyimpan pesanan.
// Mengembalikan ErrProductNotFound bila produk tidak ada di katalog.
func (s *Service) Create(ctx context.Context, productID string, qty int) (Order, error) {
	if qty <= 0 {
		return Order{}, errors.New("orders: quantity harus > 0")
	}

	// KOMUNIKASI ANTAR-DOMAIN (in-process via interface): orders memanggil
	// catalog.Lookup tanpa tahu implementasinya. Harga otoritatif berasal dari
	// domain catalog, bukan diduplikasi di orders.
	product, err := s.catalog.Lookup(ctx, productID)
	if err != nil {
		return Order{}, fmt.Errorf("%w: %s", ErrProductNotFound, productID)
	}

	order := Order{
		ID:         s.nextID(),
		ProductID:  product.ID,
		Quantity:   qty,
		TotalCents: product.PriceCents * int64(qty),
	}
	if err := s.repo.Save(ctx, order); err != nil {
		return Order{}, err
	}
	return order, nil
}
