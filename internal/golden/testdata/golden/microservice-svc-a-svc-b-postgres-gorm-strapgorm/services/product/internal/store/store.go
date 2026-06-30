// store.go memuat koneksi GORM service product. Import driver di-BRANCH per .DB
// (postgres|mysql) — dependency driver disumbang modul feature-strapgorm-
// microservice-<driver> sehingga go.mod hanya memuat driver terpilih (jujur).
package store

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open membuka koneksi PostgreSQL
// via GORM dari DSN dan menyetel batas pool yang wajar. Pemanggil (cmd/main.go)
// menutup koneksi lewat sql.DB di balik *gorm.DB saat shutdown bila perlu.
func Open(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("product: open gorm: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("product: sql.DB: %w", err)
	}

	// Batas pool wajar untuk skeleton; sesuaikan dengan beban produksi.
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

// AutoMigrate menerapkan skema Product ke database. Dipanggil saat startup;
// untuk skema produksi yang dikontrol versi, lebih disukai migrasi terkelola.
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&Product{}); err != nil {
		return fmt.Errorf("product: auto-migrate: %w", err)
	}
	return nil
}

// Close menutup pool koneksi di balik *gorm.DB. Dipanggil cmd/main.go saat
// shutdown agar slot koneksi DB dirilis dengan rapi.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
