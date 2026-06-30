// Package config memuat konfigurasi runtime service "svc-b" dari
// environment. Memakai libs/config bersama (stdlib) agar konsisten lintas service.
package config

import (
	"time"

	"example.com/shopms/libs/config"
)

// Config memuat pengaturan runtime service "svc-b".
type Config struct {
	// GRPCAddr adalah alamat bind gRPC server (mis. ":50052").
	GRPCAddr string
	// ShutdownTimeout adalah tenggang graceful shutdown.
	ShutdownTimeout time.Duration
}

// Load membaca konfigurasi dari environment dengan default yang wajar. Tidak
// pernah gagal: nilai absen jatuh ke default sehingga service selalu bisa start.
func Load() Config {
	return Config{
		GRPCAddr:        config.Get("GRPC_ADDR", ":50052"),
		ShutdownTimeout: config.GetDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}
