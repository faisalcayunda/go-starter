// Package config memuat konfigurasi runtime service "product" dari environment.
// Memakai libs/config bersama (stdlib) agar konsisten lintas service.
package config

import (
	"time"

	"example.com/shopms/libs/config"
)

// Config memuat pengaturan runtime service product.
type Config struct {
	// GRPCAddr adalah alamat bind gRPC server (Ping + health).
	GRPCAddr string
	// HTTPAddr adalah alamat bind HTTP server (GET /api/products via strapgorm).
	HTTPAddr string
	// DBDSN adalah connection string GORM untuk koneksi DB per-service product.
	DBDSN string
	// ShutdownTimeout adalah tenggang graceful shutdown.
	ShutdownTimeout time.Duration
}

// Load membaca konfigurasi dari environment dengan default yang wajar. Tidak
// pernah gagal: nilai absen jatuh ke default sehingga service selalu bisa start.
// Default DSN menunjuk service DB pada jaringan docker-compose (host = nama
// service DB); override lewat PRODUCT_DB_DSN untuk lingkungan lain.
func Load() Config {
	return Config{
		GRPCAddr:        config.Get("PRODUCT_GRPC_ADDR", ":50060"),
		HTTPAddr:        config.Get("PRODUCT_HTTP_ADDR", ":8082"),
		DBDSN:           config.Get("PRODUCT_DB_DSN", "host=postgres user=app password=app dbname=app port=5432 sslmode=disable"),
		ShutdownTimeout: config.GetDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}
