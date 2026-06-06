// Package core memuat implementasi PRIVAT domain catalog (model, repository,
// service, handler). Ia hidup di internal/modules/catalog/internal/core sehingga
// — menurut semantik direktori internal/ Go — HANYA bisa di-import oleh kode di
// bawah internal/modules/catalog/ (yakni facade catalog.go). Domain lain (orders)
// di internal/modules/orders TIDAK akan bisa meng-import paket ini: compiler
// menolaknya. Inilah boundary "berduri" yang dipaksa toolchain, bukan sekadar
// konvensi.
package core

import "errors"

// ErrNotFound dikembalikan saat produk dengan id tertentu tidak ada di repo.
var ErrNotFound = errors.New("catalog: produk tidak ditemukan")

// Product adalah model domain produk milik catalog (lengkap, internal). Field
// internal seperti SKU sengaja TIDAK dibagikan lewat contract lintas-domain.
type Product struct {
	// ID adalah identitas unik produk.
	ID string
	// Name adalah nama produk.
	Name string
	// PriceCents adalah harga dalam satuan sen.
	PriceCents int64
	// SKU adalah kode stok internal — tidak diekspos ke domain lain.
	SKU string
}
