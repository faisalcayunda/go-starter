// Package config memuat konfigurasi runtime project hasil generate dari
// environment variable. Tidak ada dependency eksternal — murni stdlib (os) —
// sehingga profil tanpa database tetap zero-require & build hijau offline.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config memuat seluruh pengaturan runtime aplikasi. Nilai diambil dari
// environment saat startup (lihat Load); field baru ditambahkan seiring modul
// yang aktif (mis. DSN database) lewat anchor di file ini pada Fase berikutnya.
type Config struct {
	// Host adalah alamat bind HTTP server (default kosong = semua interface).
	Host string
	// Port adalah port TCP HTTP server (default 8080).
	Port string
	// ReadTimeout membatasi durasi membaca seluruh request, termasuk body.
	ReadTimeout time.Duration
	// WriteTimeout membatasi durasi menulis response.
	WriteTimeout time.Duration
	// ShutdownTimeout adalah tenggang graceful shutdown sebelum koneksi dipaksa tutup.
	ShutdownTimeout time.Duration
}

// Load membaca konfigurasi dari environment dengan default yang wajar. Fungsi ini
// tidak pernah gagal: nilai yang tidak valid/absen jatuh ke default sehingga
// aplikasi selalu bisa start (zero-config, SPEC §6.3).
func Load() Config {
	return Config{
		Host:            getenv("HOST", ""),
		Port:            getenv("PORT", "8080"),
		ReadTimeout:     getDuration("READ_TIMEOUT", 5*time.Second),
		WriteTimeout:    getDuration("WRITE_TIMEOUT", 10*time.Second),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

// Addr mengembalikan alamat listen gabungan host:port untuk http.Server.
func (c Config) Addr() string {
	return c.Host + ":" + c.Port
}

// getenv mengembalikan nilai environment key, atau fallback bila kosong/absen.
func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// getDuration membaca durasi dari environment. Nilai numerik murni diperlakukan
// sebagai detik (mis. "15"); selain itu di-parse sebagai time.Duration (mis.
// "1500ms"). Nilai invalid jatuh ke fallback.
func getDuration(key string, fallback time.Duration) time.Duration {
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
