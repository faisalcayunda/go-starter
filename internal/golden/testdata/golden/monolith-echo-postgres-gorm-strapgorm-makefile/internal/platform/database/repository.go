// repository.go berisi contoh repository GORM sederhana di atas *gorm.DB.
// Pola ini memisahkan akses data dari logika domain: handler/service memanggil
// repository, bukan *gorm.DB langsung — memudahkan pengujian & penggantian backend.
package database

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// HealthCheckRepository menyediakan operasi data untuk model HealthCheck.
type HealthCheckRepository struct {
	db *gorm.DB
}

// NewHealthCheckRepository membuat repository di atas koneksi GORM.
func NewHealthCheckRepository(db *gorm.DB) *HealthCheckRepository {
	return &HealthCheckRepository{db: db}
}

// Insert menulis satu baris health_check baru dan mengembalikan record yang
// tersimpan (termasuk ID yang di-generate). CheckedAt diisi otomatis oleh GORM
// via tag autoCreateTime saat Create.
func (r *HealthCheckRepository) Insert(ctx context.Context) (*HealthCheck, error) {
	hc := &HealthCheck{}
	if err := r.db.WithContext(ctx).Create(hc).Error; err != nil {
		return nil, fmt.Errorf("health_check: insert: %w", err)
	}
	return hc, nil
}

// Count mengembalikan jumlah baris pada tabel health_check — contoh query baca
// sederhana yang berguna sebagai probe konektivitas di endpoint health.
func (r *HealthCheckRepository) Count(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&HealthCheck{}).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("health_check: count: %w", err)
	}
	return n, nil
}
