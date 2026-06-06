// Package main adalah composition root TUNGGAL modular monolith.
//
// Inilah satu-satunya tempat domain "saling kenal": ia merakit tiap domain dari
// internal/modules/<domain> (facade), lalu meng-inject port catalog ke domain
// orders SEBAGAI interface contract.Catalog. Di luar sini, domain hanya bicara
// lewat kontrak — bukan paket konkret tetangga (boundary berduri dipaksa compiler).
//
// Satu binary, satu deploy (esensi monolith); modularitas dijaga batas internal/
// per domain (esensi modular). Saat ekstraksi ke microservice, ganti implementasi
// catalog yang di-inject di sini dengan klien gRPC/HTTP — domain orders tak berubah.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/demo-modular/internal/config"
	"github.com/example/demo-modular/internal/httpserver"
	"github.com/example/demo-modular/internal/modules/catalog"
	"github.com/example/demo-modular/internal/modules/orders"
	// region:imports
)

func main() {
	// Logger slog terstruktur (stdlib) menulis ke stdout dengan format teks.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// signal.NotifyContext mengembalikan context yang dibatalkan saat SIGINT /
	// SIGTERM diterima — sumber sinyal graceful shutdown untuk seluruh aplikasi.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Konfigurasi dibaca sekali dari environment (lihat internal/config).
	cfg := config.Load()

	// ── Rakit domain ─────────────────────────────────────────────────────────
	// catalogMod menyediakan port contract.Catalog. INJEKSI ANTAR-DOMAIN:
	// catalogMod.Service() diteruskan ke orders sebagai contract.Catalog —
	// orders tidak meng-import paket catalog, hanya mengenal interfacenya.
	catalogMod := catalog.New()
	ordersMod := orders.New(catalogMod.Service())

	// region:wiring
	// Anchor wiring: modul tambahan (DB, observability, dll.) menyumbang
	// inisialisasi di sini lewat ModeMerge. Pada profil stdlib default seluruh
	// wiring domain ada di atas.

	// ── Server: pasang rute kedua domain lewat httpserver netral-domain ──────
	srv := httpserver.New(httpserver.Options{
		Addr:         cfg.Addr(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}, logger, catalogMod, ordersMod)

	// Jalankan server pada goroutine terpisah agar main dapat menunggu sinyal
	// shutdown lewat ctx.Done.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server mulai mendengarkan", slog.String("addr", cfg.Addr()))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	// Tunggu: error fatal dari server, atau sinyal shutdown dari ctx.
	select {
	case err := <-serverErr:
		if err != nil {
			logger.Error("server berhenti dengan error", slog.Any("error", err))
			os.Exit(1)
		}
		return
	case <-ctx.Done():
		logger.Info("sinyal shutdown diterima, menutup server dengan rapi")
	}

	// Graceful shutdown: beri tenggang waktu agar request berjalan selesai.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("gagal menutup server", slog.Any("error", err))
		os.Exit(1)
	}
}
