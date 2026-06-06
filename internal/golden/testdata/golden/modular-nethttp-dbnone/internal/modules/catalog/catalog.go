// Package catalog adalah FACADE (permukaan publik) domain catalog pada modular
// monolith. Inilah satu-satunya paket catalog yang boleh disentuh composition
// root (cmd). Seluruh detail implementasi (model, repository, service, handler)
// terkurung di sub-paket privat internal/core — yang menurut semantik direktori
// internal/ Go HANYA bisa di-import dari bawah internal/modules/catalog/.
//
// Akibatnya domain lain (orders di internal/modules/orders) TIDAK dapat menyentuh
// isi catalog sama sekali: compiler menolak import lintas-domain ke internal/core.
// Boundary "berduri" ini dipaksa toolchain, bukan sekadar disepakati.
package catalog

import (
	"net/http"

	"github.com/example/demo-modular/internal/modules/catalog/internal/core"
	"github.com/example/demo-modular/internal/shared/contract"
)

// Module adalah unit domain catalog yang sudah dirakit: ia menyimpan service
// (yang memenuhi contract.Catalog) dan handler HTTP-nya. Composition root cukup
// memanggil New lalu memakai Service() & RegisterRoutes().
type Module struct {
	service *core.Service
	handler *core.Handler
}

// New merakit domain catalog dengan storage in-memory contoh (murni stdlib).
// Untuk produksi, ganti repository di dalam fungsi ini dengan implementasi ber-DB
// — composition root & domain lain tak perlu berubah.
func New() *Module {
	svc := core.NewService(core.NewMemRepository())
	return &Module{
		service: svc,
		handler: core.NewHandler(svc),
	}
}

// Service mengembalikan port catalog sebagai contract.Catalog. Composition root
// meneruskan nilai ini ke domain orders sebagai dependency lintas-domain.
func (m *Module) Service() contract.Catalog {
	return m.service
}

// RegisterRoutes memasang rute HTTP domain catalog pada mux composition root.
func (m *Module) RegisterRoutes(mux *http.ServeMux) {
	m.handler.Routes(mux)
}
