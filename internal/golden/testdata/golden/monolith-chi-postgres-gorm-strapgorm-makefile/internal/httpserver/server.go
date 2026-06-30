// Package httpserver merakit *http.Server dengan router go-chi/chi/v5 sebagai
// pengganti http.ServeMux. chi tetap kompatibel dengan http.Handler standar
// (router-nya SENDIRI adalah http.Handler), sehingga tanda tangan New() identik
// dengan profil net/http: composition root (internal/app) tidak berubah saat
// menukar framework. Health check (Healthz) tetap dari health.go milik paket ini.
package httpserver

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"example.com/shopdemo/internal/handler"
	// region:imports (feature-strapgorm) — domain Product di-import untuk Mount rute /api/products.
	"example.com/shopdemo/internal/product"
)

// Options adalah parameter konstruksi server yang diturunkan dari config.
// Bentuknya identik lintas framework agar wiring composition root stabil.
type Options struct {
	// Addr adalah alamat listen host:port.
	Addr string
	// ReadTimeout membatasi durasi membaca request.
	ReadTimeout time.Duration
	// WriteTimeout membatasi durasi menulis response.
	WriteTimeout time.Duration
}

// New membangun *http.Server lengkap dengan router chi. Middleware bawaan chi
// (RequestID, Logger, Recoverer) dipasang lebih dulu; logger slog aplikasi
// dibungkus sebagai middleware tambahan agar format log konsisten dengan profil
// stdlib. Rute inti (/healthz, /api/hello) didaftarkan di sini; modul tambahan
// menyumbang rute lewat anchor "routes" di bawah.
func New(opts Options, logger *slog.Logger) *http.Server {
	r := chi.NewRouter()

	// Middleware chi standar: korelasi request, log akses, dan pemulihan panic.
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// Middleware slog aplikasi (mencatat metode, path, dan durasi tiap request).
	r.Use(logRequests(logger))

	// Rute health check selalu tersedia (lihat health.go; Healthz ber-tanda
	// tangan http.HandlerFunc sehingga dipasang langsung ke chi).
	r.Get("/healthz", Healthz)

	// Rute domain contoh. handler.Hello murni net/http (http.HandlerFunc),
	// dipasang apa adanya — handler bersama tidak terikat framework.
	r.Get("/api/hello", handler.Hello)

	// Contoh path param idiomatik chi: GET /api/hello/{name} membaca segmen path
	// lewat chi.URLParam. Menunjukkan pola routing chi tanpa menambah dependency.
	r.Get("/api/hello/{name}", func(w http.ResponseWriter, req *http.Request) {
		name := chi.URLParam(req, "name")
		// Teruskan ke handler bersama lewat query agar perilaku konsisten dengan
		// /api/hello?name=... (satu sumber kebenaran salam).
		q := req.URL.Query()
		q.Set("name", name)
		req.URL.RawQuery = q.Encode()
		handler.Hello(w, req)
	})

	// region:routes (feature-strapgorm) — daftarkan GET /api/products (Strapi-style
	// query builder di atas koneksi GORM bersama yang disetel di wiring main).
	// ListHandler() http.HandlerFunc dipasang idiomatik chi pada router r.
	r.Get("/api/products", product.ListHandler())
	// Anchor routes: modul tambahan menyumbang pendaftaran rute chi di sini lewat
	// ModeMerge (mis. r.Mount / r.Route resource baru). Marker komentar netral ini
	// tetap ada meski belum ada penyumbang, demi idempotensi merge & `add service`.

	return &http.Server{
		Addr:         opts.Addr,
		Handler:      r,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	}
}

// logRequests mengembalikan middleware chi (func(http.Handler) http.Handler) yang
// mencatat tiap request memakai slog — selaras dengan profil net/http.
func logRequests(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
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
}
