// Package httpserver merakit *http.Server berbasis net/http stdlib (routing
// http.ServeMux Go 1.22+ dengan pola method+path). Tidak ada dependency framework
// eksternal — murni stdlib — sehingga profil modular default tetap zero-require.
//
// Server ini netral terhadap domain: composition root (cmd) menyerahkan daftar
// "router domain" (apa pun yang punya method Routes(*http.ServeMux)) lalu server
// memasang rute tiap domain. Dengan begitu menambah/menghapus domain tidak
// menyentuh paket ini — selaras prinsip modular monolith (boundary & isolasi).
package httpserver

import (
	"log/slog"
	"net/http"
	"time"
	// region:imports
)

// Module adalah kontrak minimal yang dipenuhi tiap facade domain: ia tahu cara
// mendaftarkan rutenya sendiri pada mux. Facade catalog & orders keduanya
// memenuhi interface ini lewat RegisterRoutes. Server tetap netral terhadap
// domain — ia tidak meng-import paket domain mana pun.
type Module interface {
	RegisterRoutes(mux *http.ServeMux)
}

// Options adalah parameter konstruksi server yang diturunkan dari config.
type Options struct {
	// Addr adalah alamat listen host:port.
	Addr string
	// ReadTimeout membatasi durasi membaca request.
	ReadTimeout time.Duration
	// WriteTimeout membatasi durasi menulis response.
	WriteTimeout time.Duration
}

// New membangun *http.Server dengan mux yang sudah memuat rute health check inti
// (/healthz) dan rute tiap domain yang diserahkan composition root. Rute domain
// dipasang lewat modules — bukan di-hardcode — agar server tetap netral-domain.
func New(opts Options, logger *slog.Logger, modules ...Module) *http.Server {
	mux := http.NewServeMux()

	// Rute health check selalu tersedia (lihat health.go).
	mux.HandleFunc("GET /healthz", Healthz)

	// Pasang rute tiap domain. Composition root menentukan urutan & himpunan
	// domain; server tidak tahu detail domain mana pun.
	for _, m := range modules {
		m.RegisterRoutes(mux)
	}

	// region:routes
	// Anchor routes: modul tambahan (mis. add-on) dapat menyumbang pendaftaran
	// rute di sini lewat ModeMerge. Marker komentar netral ini tetap ada meski
	// belum ada penyumbang, demi idempotensi merge.

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
