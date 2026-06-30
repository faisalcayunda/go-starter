// Package main adalah entrypoint tipis aplikasi.
//
// Tanggung jawabnya minimal: muat konfigurasi dari environment, siapkan logger
// slog, lalu serahkan seluruh wiring ke composition root (internal/app). Pola ini
// menjaga main tetap kecil — semua keputusan dependency dirakit di satu tempat
// (internal/app.Run), bukan tersebar di main.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"example.com/shopdemo/internal/app"
	"example.com/shopdemo/internal/config"
	// region:imports (access-gorm-postgres) — paket koneksi GORM di-import untuk wiring di main.
	"example.com/shopdemo/internal/platform/database"
	// region:imports (feature-strapgorm) — domain Product di-import untuk wiring koneksi GORM bersama.
	"example.com/shopdemo/internal/product"
)

func main() {
	// Logger slog terstruktur (stdlib) menulis ke stdout dengan format teks.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// signal.NotifyContext mengembalikan context yang dibatalkan saat SIGINT /
	// SIGTERM diterima — menjadi sumber sinyal graceful shutdown untuk seluruh
	// aplikasi (server berhenti menerima koneksi lalu menutup dengan rapi).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Konfigurasi dibaca sekali dari environment (lihat internal/config).
	cfg := config.Load()

	// region:wiring (access-gorm-postgres) — buka koneksi GORM (PostgreSQL) dari env DSN.
	db, err := database.Connect(ctx)
	if err != nil {
		logger.Error("connect postgres (gorm)", "error", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close(db) }()
	if err := database.AutoMigrate(db); err != nil {
		logger.Error("auto-migrate postgres (gorm)", "error", err)
		os.Exit(1)
	}
	// region:wiring (feature-strapgorm) — REUSE koneksi GORM access=gorm (var db di atas);
	// catat ke paket product lalu migrasi tabel products. Tanpa pool koneksi kedua.
	product.SetDB(db)
	if err := product.AutoMigrate(); err != nil {
		logger.Error("auto-migrate products (strapgorm)", "error", err)
		os.Exit(1)
	}
	// Anchor wiring: modul tambahan (DB, dll.) menyumbang inisialisasi di sini
	// lewat ModeMerge. Pada profil stdlib default seluruh wiring ada di app.Run.

	// Seluruh penyusunan dependency & siklus hidup server ada di app.Run.
	if err := app.Run(ctx, cfg, logger); err != nil {
		logger.Error("aplikasi berhenti dengan error", slog.Any("error", err))
		os.Exit(1)
	}
}
