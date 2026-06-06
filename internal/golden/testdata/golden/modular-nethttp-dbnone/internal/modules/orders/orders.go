// Package orders adalah FACADE (permukaan publik) domain orders pada modular
// monolith. Hanya paket inilah yang disentuh composition root. Detail
// implementasi (model, repository, service, handler) terkurung di sub-paket
// privat internal/core, yang tak bisa di-import domain lain (boundary berduri).
//
// Ketergantungan orders pada catalog dinyatakan sebagai contract.Catalog yang
// di-inject lewat New — orders TIDAK pernah meng-import paket catalog.
package orders

import (
	"net/http"

	"github.com/example/demo-modular/internal/modules/orders/internal/core"
	"github.com/example/demo-modular/internal/shared/contract"
)

// Module adalah unit domain orders yang sudah dirakit (service + handler).
type Module struct {
	handler *core.Handler
}

// New merakit domain orders dengan storage in-memory contoh (murni stdlib) dan
// port catalog (contract.Catalog) yang diteruskan composition root. Domain orders
// memakai catalog HANYA lewat interface ini — siap ditukar dengan klien jaringan
// saat ekstraksi ke microservice.
func New(catalog contract.Catalog) *Module {
	svc := core.NewService(core.NewMemRepository(), catalog)
	return &Module{handler: core.NewHandler(svc)}
}

// RegisterRoutes memasang rute HTTP domain orders pada mux composition root.
func (m *Module) RegisterRoutes(mux *http.ServeMux) {
	m.handler.Routes(mux)
}
