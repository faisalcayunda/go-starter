// handler.go memuat HTTP handler Product (net/http stdlib + encoding/json).
// Handler men-decode query string Strapi via parser.FromURL, memanggil Repository,
// lalu menulis JSON {"data":..,"meta":..} (200), {"error":..} (400 untuk query tak
// valid), atau {"error":..} (500 untuk kegagalan eksekusi/DB). Klasifikasi memakai
// sentinel ErrInvalidQuery (lihat repository.go) — bukan tebak string error.
package product

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/faisalcayunda/strapgorm/parser"
)

// Handler mengikat Repository ke HTTP handler. Dibuat sekali saat wiring lalu
// di-mount ke router (lihat Mount di wiring.go).
type Handler struct {
	repo Repository
}

// NewHandler membuat Handler di atas Repository.
func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// listResponse adalah amplop sukses GET /api/products: koleksi data + metadata
// pagination Strapi.
type listResponse struct {
	Data any `json:"data"`
	Meta any `json:"meta"`
}

// errorResponse adalah amplop error JSON (mis. query string tak valid / field
// tak dikenal).
type errorResponse struct {
	Error string `json:"error"`
}

// ListProducts menangani GET /api/products. Query string Strapi
// (filters[..]/sort/pagination[..]/fields/search) di-parse via parser.FromURL,
// lalu Repository.List menjalankannya. Error parse ATAU field/operator tak dikenal
// (di-validasi strapgorm terhadap skema Product) → 400; sukses → 200 dengan
// {"data":[..],"meta":{..}}.
func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	qp, err := parser.FromURL(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, meta, err := h.repo.List(r.Context(), qp)
	if err != nil {
		// Klasifikasi by TIPE error (bukan tebak string): repository menandai
		// kegagalan VALIDASI query dengan ErrInvalidQuery (field/operator tak dikenal
		// terhadap skema Product) → 400 dgn detail validasi yang aman (mis.
		// "invalid query parameters: strapgorm: building filters: ... unknown field").
		// Kegagalan lain (eksekusi/DB down/timeout) → 500 TANPA membocorkan internal.
		if errors.Is(err, ErrInvalidQuery) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, listResponse{Data: items, Meta: meta})
}

// writeJSON menulis payload sebagai JSON dengan status code yang diberikan.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeError menulis amplop error JSON dengan status code yang diberikan.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
