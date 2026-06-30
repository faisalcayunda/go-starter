// Package config memuat konfigurasi runtime bersama untuk seluruh service dalam
// monorepo ini. Tidak ada dependency eksternal — murni stdlib (os) — sehingga
// tiap service tetap zero-extra-require di luar gRPC/protobuf.
package config

import (
	"os"
	"strconv"
	"time"
)

// Get mengembalikan nilai environment key, atau fallback bila kosong/absen.
func Get(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// GetDuration membaca durasi dari environment. Nilai numerik murni diperlakukan
// sebagai detik (mis. "15"); selain itu di-parse sebagai time.Duration (mis.
// "1500ms"). Nilai invalid jatuh ke fallback.
func GetDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return fallback
}
