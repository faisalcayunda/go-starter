// Package httpserver merakit *http.Server berbasis net/http stdlib (routing
// http.ServeMux Go 1.22+ dengan pola method+path). Tidak ada dependency
// framework eksternal — murni stdlib — sehingga profil default tetap zero-require.
package httpserver

import (
	"log/slog"
	"net/http"
	"time"

	"example.com/shopdemo/internal/handler"
	// region:imports (feature-strapgorm) — domain Product di-import untuk Mount rute /api/products.
	"example.com/shopdemo/internal/product"
)

// Options adalah parameter konstruksi server yang diturunkan dari config.
type Options struct {
	// Addr adalah alamat listen host:port.
	Addr string
	// ReadTimeout membatasi durasi membaca request.
	ReadTimeout time.Duration
	// WriteTimeout membatasi durasi menulis response.
	WriteTimeout time.Duration
}

// New membangun *http.Server lengkap dengan router (http.ServeMux). Rute inti
// (/healthz) didaftarkan di sini; modul tambahan menyumbang rute lewat anchor
// "routes" di bawah (ModeMerge) atau lewat composition root (internal/app).
func New(opts Options, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()

	// Rute health check selalu tersedia (lihat health.go).
	mux.HandleFunc("GET /healthz", Healthz)

	// Rute domain contoh (profil stdlib default).
	mux.HandleFunc("GET /api/hello", handler.Hello)

	// region:routes (feature-strapgorm) — daftarkan GET /api/products (Strapi-style
	// query builder di atas koneksi GORM bersama yang disetel di wiring main). mux
	// (*http.ServeMux) ada di scope httpserver.New profil net/http.
	product.Mount(mux)
	// Anchor routes: modul tambahan menyumbang pendaftaran rute di sini lewat
	// ModeMerge (mis. rute resource baru). Marker komentar netral ini tetap ada
	// meski belum ada penyumbang, demi idempotensi merge.

	return &http.Server{
		Addr:         opts.Addr,
		Handler:      logRequests(logger, mux),
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	}
}

// logRequests adalah middleware stdlib yang mencatat tiap request memakai slog.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Duration("took", time.Since(start)),
		)
	})
}
