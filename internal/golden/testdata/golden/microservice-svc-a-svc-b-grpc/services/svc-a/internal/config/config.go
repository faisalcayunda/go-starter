// Package config memuat konfigurasi runtime service "svc-a" dari
// environment. Memakai libs/config bersama (stdlib) agar konsisten lintas service.
package config

import (
	"time"

	"github.com/example/demo-ms/libs/config"
)

// Downstream mendeskripsikan satu service tujuan yang dapat dipanggil endpoint
// /call milik service ini (bukti inter-service call via gRPC).
type Downstream struct {
	// Name adalah nama logis service tujuan (cocok dengan ?to=<name>).
	Name string
	// Addr adalah alamat gRPC host:port service tujuan (mis. "svc-b:50052").
	Addr string
}

// Config memuat pengaturan runtime service "svc-a".
type Config struct {
	// GRPCAddr adalah alamat bind gRPC server (mis. ":50051").
	GRPCAddr string
	// HTTPAddr adalah alamat bind HTTP server kecil (endpoint /call) — service
	// pertama mengekspos bukti inter-service call.
	HTTPAddr string
	// Downstreams adalah daftar service gRPC yang dapat dipanggil lewat /call?to=.
	Downstreams []Downstream
	// ShutdownTimeout adalah tenggang graceful shutdown.
	ShutdownTimeout time.Duration
}

// Load membaca konfigurasi dari environment dengan default yang wajar. Tidak
// pernah gagal: nilai absen jatuh ke default sehingga service selalu bisa start.
func Load() Config {
	return Config{
		GRPCAddr: config.Get("GRPC_ADDR", ":50051"),
		HTTPAddr: config.Get("HTTP_ADDR", ":8081"),
		Downstreams: []Downstream{
			{Name: "svc-b", Addr: config.Get("SVC_B_GRPC_ADDR", "svc-b:50052")},
		},
		ShutdownTimeout: config.GetDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}
