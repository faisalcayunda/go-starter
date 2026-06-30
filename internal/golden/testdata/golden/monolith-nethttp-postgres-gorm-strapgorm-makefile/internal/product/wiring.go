// wiring.go menjembatani koneksi GORM (dibuka di main, paket platform/database)
// ke pendaftaran rute (di paket httpserver). Karena composition root monolith
// memisahkan pembukaan koneksi (main) dari perakitan router (httpserver.New),
// jembatan paket-level ini meneruskan *gorm.DB yang SAMA tanpa mengubah signature
// app.Run / httpserver.New dan TANPA membuka pool koneksi kedua.
//
// Urutan pasti (lihat main & httpserver.New): main membuka db lalu memanggil
// SetDB(db) SEBELUM app.Run; httpserver.New (dipanggil di dalam app.Run) merakit
// repo + handler dari db itu lalu mendaftarkan rute pada anchor region:routes. Tak
// ada akses konkuren: SetDB dipanggil sekali saat startup sebelum server menerima
// request.
//
// Pendaftaran rute NETRAL-FRAMEWORK: ListHandler() mengembalikan http.HandlerFunc
// murni net/http yang bisa dipasang di SEMUA router (mux.HandleFunc untuk net/http,
// r.Get untuk chi, e.GET via echo.WrapHandler untuk echo). Mount(mux) dipertahankan
// sebagai shortcut net/http (memakai ListHandler di balik layar) tanpa memaksa
// router lain punya *http.ServeMux di scope-nya.
package product

import (
	"net/http"

	"gorm.io/gorm"
)

// db menyimpan koneksi GORM yang dipinjam dari access=gorm (REUSE, bukan pool
// baru). Disetel sekali via SetDB saat startup.
var db *gorm.DB

// SetDB mencatat koneksi GORM bersama yang dibuka di main (access=gorm). Dipanggil
// dari wiring main SEBELUM server dirakit, sehingga Mount dapat memakainya.
func SetDB(conn *gorm.DB) {
	db = conn
}

// AutoMigrate menerapkan skema Product ke database memakai koneksi bersama.
// Dipanggil dari wiring main setelah SetDB. Mengembalikan error agar pemanggil
// memutuskan fatal/lanjut.
func AutoMigrate() error {
	return db.AutoMigrate(&Product{})
}

// ListHandler merakit repository + handler Product di atas koneksi bersama yang
// sudah disetel via SetDB, lalu mengembalikan handler GET list (ListProducts)
// sebagai http.HandlerFunc murni net/http. Bentuk netral-framework ini dipasang
// langsung oleh anchor region:routes tiap profil HTTP: mux.HandleFunc (net/http),
// r.Get (chi), atau e.GET lewat echo.WrapHandler (echo) — tanpa memaksa router
// punya tipe spesifik. Dipanggil dari httpserver.New saat perakitan rute.
func ListHandler() http.HandlerFunc {
	repo := NewGormRepository(db)
	h := NewHandler(repo)
	return h.ListProducts
}

// Mount mendaftarkan rute domain Product (GET /api/products) pada mux net/http.
// Shortcut untuk profil net/http yang punya *http.ServeMux di scope region:routes;
// di balik layar ia memakai ListHandler() agar logika perakitan tunggal. Profil
// chi/echo memanggil ListHandler() langsung pada router masing-masing.
func Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/products", ListHandler())
}
