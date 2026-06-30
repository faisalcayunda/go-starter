// Package httpserver merakit *http.Server dengan router labstack/echo/v4 sebagai
// pengganti http.ServeMux. *echo.Echo mengimplementasikan http.Handler (punya
// ServeHTTP), sehingga dipasang sebagai Handler pada *http.Server — tanda tangan
// New() identik dengan profil net/http dan composition root (internal/app) tidak
// berubah saat menukar framework. Health check (Healthz) tetap dari health.go.
package httpserver

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

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

// New membangun *http.Server lengkap dengan router echo. Middleware bawaan echo
// (RequestID, Recover) dipasang lebih dulu; logger slog aplikasi dibungkus sebagai
// middleware tambahan agar format log konsisten dengan profil stdlib. Rute inti
// (/healthz, /api/hello) didaftarkan di sini; modul tambahan menyumbang rute lewat
// anchor "routes" di bawah.
func New(opts Options, logger *slog.Logger) *http.Server {
	e := echo.New()
	// Banner & port-log bawaan echo dimatikan: siklus hidup server (Listen/
	// Shutdown) dikelola composition root lewat *http.Server, bukan echo.Start.
	e.HideBanner = true
	e.HidePort = true

	// Middleware echo standar: korelasi request dan pemulihan panic.
	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	// Middleware slog aplikasi (mencatat metode, path, dan durasi tiap request).
	e.Use(logRequests(logger))

	// Rute health check selalu tersedia (lihat health.go). Healthz ber-tanda
	// tangan net/http, dipasang ke echo lewat echo.WrapHandler.
	e.GET("/healthz", echo.WrapHandler(http.HandlerFunc(Healthz)))

	// Rute domain contoh. handler.Hello murni net/http; dibungkus echo.WrapHandler
	// — handler bersama tidak terikat framework, sehingga test handler core hijau.
	e.GET("/api/hello", echo.WrapHandler(http.HandlerFunc(handler.Hello)))

	// Contoh path param idiomatik echo: GET /api/hello/:name membaca segmen path
	// lewat c.Param. Menunjukkan pola routing echo tanpa menambah dependency.
	e.GET("/api/hello/:name", func(c echo.Context) error {
		name := c.Param("name")
		// Teruskan ke handler bersama lewat query agar perilaku konsisten dengan
		// /api/hello?name=... (satu sumber kebenaran salam).
		req := c.Request()
		q := req.URL.Query()
		q.Set("name", name)
		req.URL.RawQuery = q.Encode()
		handler.Hello(c.Response(), req)
		return nil
	})

	// region:routes (feature-strapgorm) — daftarkan GET /api/products (Strapi-style
	// query builder di atas koneksi GORM bersama yang disetel di wiring main).
	// ListHandler() net/http dibungkus echo.WrapHandler lalu dipasang pada router e.
	e.GET("/api/products", echo.WrapHandler(product.ListHandler()))
	// Anchor routes: modul tambahan menyumbang pendaftaran rute echo di sini lewat
	// ModeMerge (mis. e.Group / grup resource baru). Marker komentar netral ini
	// tetap ada meski belum ada penyumbang, demi idempotensi merge & `add service`.

	return &http.Server{
		Addr:         opts.Addr,
		Handler:      e,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	}
}

// logRequests mengembalikan middleware echo (echo.MiddlewareFunc) yang mencatat
// tiap request memakai slog — selaras dengan profil net/http.
func logRequests(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			req := c.Request()
			logger.Info("http request",
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Duration("took", time.Since(start)),
			)
			return err
		}
	}
}
