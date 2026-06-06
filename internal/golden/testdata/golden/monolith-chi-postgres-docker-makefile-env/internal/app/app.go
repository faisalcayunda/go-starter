// Package app adalah composition root monolith layered (Kandidat B): satu tempat
// di mana seluruh dependency dirakit menjadi aplikasi yang berjalan. main hanya
// memanggil Run; semua wiring (config → server) ada di sini.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/example/demo-chi/internal/config"
	"github.com/example/demo-chi/internal/httpserver"
)

// Run merakit dan menjalankan aplikasi sampai ctx dibatalkan (sinyal SIGINT/
// SIGTERM dari main), lalu mematikan server dengan graceful shutdown. Mengembalikan
// error non-nil hanya bila server gagal di luar penutupan normal.
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	// Bangun *http.Server inti. Seluruh rute (/healthz, /api/hello) didaftarkan
	// di httpserver.New — composition root cukup menjalankan & mematikannya.
	srv := httpserver.New(httpserver.Options{
		Addr:         cfg.Addr(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}, logger)

	// Jalankan server pada goroutine terpisah agar Run dapat menunggu sinyal
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
		return err
	case <-ctx.Done():
		logger.Info("sinyal shutdown diterima, menutup server dengan rapi")
	}

	// Graceful shutdown: beri tenggang waktu agar request berjalan selesai.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return nil
}
