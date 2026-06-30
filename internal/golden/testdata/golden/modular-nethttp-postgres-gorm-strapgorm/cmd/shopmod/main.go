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

	"example.com/shopmod/internal/config"
	"example.com/shopmod/internal/httpserver"
	"example.com/shopmod/internal/modules/catalog"
	"example.com/shopmod/internal/modules/orders"
	// region:imports (access-gorm-postgres) — paket koneksi GORM di-import untuk wiring di main.
	"example.com/shopmod/internal/platform/database"
	// region:imports (feature-strapgorm-modular) — facade domain Product di-import untuk wiring & route.
	"example.com/shopmod/internal/modules/product"
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
	// region:wiring (feature-strapgorm-modular) — rakit domain Product di atas koneksi GORM
	// access=gorm (var db dari fragmen access-gorm di atas, REUSE tanpa pool kedua) lalu
	// migrasi tabel products. productMod disisipkan ke httpserver.New via region:modules.
	productMod := product.New(db)
	if err := productMod.AutoMigrate(); err != nil {
		logger.Error("auto-migrate products (strapgorm)", "error", err)
		os.Exit(1)
	}
	// Anchor wiring: modul tambahan (DB, observability, dll.) menyumbang
	// inisialisasi di sini lewat ModeMerge. Pada profil stdlib default seluruh
	// wiring domain ada di atas.

	// ── Server: pasang rute tiap domain lewat httpserver netral-domain ───────
	// Daftar modul ditulis multiline dengan anchor region:modules di akhir varargs
	// agar add-on (mis. feature-strapgorm-modular) dapat menyisipkan domainnya
	// sendiri lewat ModeMerge tanpa menyentuh signature New(). Tanpa penyumbang,
	// marker komentar netral tetap ada (idempotensi merge) & valid sebagai argumen
	// (trailing comma + komentar di antara argumen = legal Go).
	srv := httpserver.New(httpserver.Options{
		Addr:         cfg.Addr(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}, logger,
		catalogMod,
		ordersMod,
		// region:modules (feature-strapgorm-modular) — daftarkan domain Product ke server.
		productMod,
	)

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
