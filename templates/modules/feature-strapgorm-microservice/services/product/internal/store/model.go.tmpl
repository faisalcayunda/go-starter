// Package store adalah lapisan data service "product": model Product + koneksi
// GORM + repository berbasis strapgorm (Strapi-style query builder) + handler HTTP
// GET /api/products. Service product memiliki koneksi GORM SENDIRI (per-service
// DB, idiomatik microservice) — tiap service mengelola datanya sendiri.
package store

// Product adalah model contoh yang dipetakan ke tabel products. Tag gorm
// menentukan kolom & index; tag json menentukan nama field pada body response
// DAN nama field yang dikenali strapgorm dari query string (whitelist).
//
// Sesuaikan field dengan kebutuhan domain Anda. strapgorm hanya mengizinkan
// filter/sort/fields atas nama json yang ada di sini (token tak dikenal → error
// 400 di handler), sehingga query string yang masuk selalu tervalidasi terhadap
// skema model ini.
type Product struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"size:255;index;not null" json:"name"`
	SKU         string `gorm:"size:64;uniqueIndex;not null" json:"sku"`
	Description string `gorm:"type:text" json:"description"`
}

// Searchable mengembalikan nama-nama field (json name) yang ikut pada pencarian
// teks bebas Strapi (?search= / ?_q=). strapgorm memetakan tiap nama ke kolom
// lewat skema model dan merakit predikat ILIKE OR yang aman (nilai selalu di-bind
// sebagai parameter — tak ada konkatenasi SQL). Implementasi interface
// schema.Searchable strapgorm.
func (Product) Searchable() []string {
	return []string{"name", "sku", "description"}
}
