// repository.go memisahkan akses data Product dari handler: handler memanggil
// Repository (interface), bukan *gorm.DB langsung — memudahkan pengujian &
// penggantian backend. Adapter gormRepository merakit query lewat strapgorm.
package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/faisalcayunda/strapgorm"
	"github.com/faisalcayunda/strapgorm/paginate"
	"github.com/faisalcayunda/strapgorm/parser"
	"gorm.io/gorm"
)

// ErrInvalidQuery menandai kegagalan VALIDASI query (field/operator/sort/fields tak
// dikenal terhadap skema Product) — kesalahan KLIEN (handler → HTTP 400). Kegagalan
// EKSEKUSI (DB down, timeout, error driver) di-return TANPA sentinel ini sehingga
// handler memetakannya ke 500. Pemisahan dilakukan via ToSQL (DryRun: bangun &
// validasi SQL TANPA mengeksekusi) sebelum Paginate — bukan tebak-tebak string error.
var ErrInvalidQuery = errors.New("invalid query parameters")

// Repository adalah port (hexagonal) untuk akses data Product. List menerima
// QueryParams hasil parse query string Strapi dan mengembalikan halaman Product
// beserta metadata pagination.
type Repository interface {
	List(ctx context.Context, qp parser.QueryParams) ([]Product, paginate.Meta, error)
}

// gormRepository adalah adapter Repository di atas *gorm.DB memakai strapgorm.
// db DIPINJAM dari koneksi access=gorm (REUSE — di-inject facade), adapter ini
// TIDAK membuka koneksi sendiri.
type gormRepository struct {
	db *gorm.DB
}

// NewGormRepository membuat Repository di atas koneksi GORM yang ada. Facade
// (product.New) meneruskan *gorm.DB yang SAMA dengan access=gorm.
func NewGormRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

// List menjalankan query Strapi-style atas tabel products: strapgorm mem-validasi
// setiap token (field/operator) dari QueryParams terhadap skema Product, merakit
// SQL parameterized yang aman, lalu Paginate menjalankannya dan menghitung Meta.
// Error (mis. field/operator tak dikenal) di-wrap dengan konteks dan diterjemahkan
// handler menjadi 400.
func (r *gormRepository) List(ctx context.Context, qp parser.QueryParams) ([]Product, paginate.Meta, error) {
	b := strapgorm.New[Product](r.db).FromParams(qp)

	// Tahap 1 — VALIDASI (DryRun): ToSQL membangun & memvalidasi SQL terhadap skema
	// Product TANPA menyentuh DB. Error di sini = token query tak valid → klien (400).
	if _, _, err := b.ToSQL(ctx, qp.Pagination); err != nil {
		return nil, paginate.Meta{}, fmt.Errorf("%w: %w", ErrInvalidQuery, err)
	}

	// Tahap 2 — EKSEKUSI: query valid; error di sini = kegagalan DB/eksekusi → server (500).
	items, meta, err := b.Paginate(ctx, qp.Pagination)
	if err != nil {
		return nil, paginate.Meta{}, fmt.Errorf("product: list: %w", err)
	}
	return items, meta, nil
}
