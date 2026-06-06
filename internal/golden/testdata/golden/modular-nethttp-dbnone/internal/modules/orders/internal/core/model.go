// Package core memuat implementasi PRIVAT domain orders (model, repository,
// service, handler). Ia hidup di internal/modules/orders/internal/core sehingga
// HANYA bisa di-import dari bawah internal/modules/orders/ (semantik direktori
// internal/ Go). Domain catalog tidak akan bisa menyentuhnya — boundary berduri.
//
// orders memanggil domain catalog HANYA lewat interface contract.Catalog yang
// di-inject facade; ia tidak pernah meng-import paket catalog (lihat service.go).
package core

import "errors"

// ErrProductNotFound dikembalikan saat membuat pesanan untuk produk yang tidak
// ada di katalog (hasil verifikasi lewat contract.Catalog).
var ErrProductNotFound = errors.New("orders: produk tidak ada di katalog")

// Order adalah model domain pesanan (internal ke orders).
type Order struct {
	// ID adalah identitas unik pesanan.
	ID string
	// ProductID merujuk produk yang dipesan (diverifikasi ke catalog).
	ProductID string
	// Quantity adalah jumlah unit yang dipesan.
	Quantity int
	// TotalCents adalah total harga (harga satuan dari catalog × Quantity).
	TotalCents int64
}
