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

	"github.com/example/demo-chi/internal/app"
	"github.com/example/demo-chi/internal/config"
	// region:imports (db-postgres) — paket koneksi DB di-import untuk wiring di main.
	"github.com/example/demo-chi/internal/platform/database"
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

	// region:wiring (db-postgres) — buka pool koneksi PostgreSQL dari env DSN.
	pool, err := database.Connect(ctx)
	if err != nil {
		logger.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	// Anchor wiring: modul tambahan (DB, dll.) menyumbang inisialisasi di sini
	// lewat ModeMerge. Pada profil stdlib default seluruh wiring ada di app.Run.

	// Seluruh penyusunan dependency & siklus hidup server ada di app.Run.
	if err := app.Run(ctx, cfg, logger); err != nil {
		logger.Error("aplikasi berhenti dengan error", slog.Any("error", err))
		os.Exit(1)
	}
}
