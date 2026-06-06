package core

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Handler adalah adapter transport HTTP domain catalog. Tipis: menerjemahkan
// request → Service → response JSON. Logika bisnis ada di Service, bukan di sini.
type Handler struct {
	svc *Service
}

// NewHandler merakit Handler dengan Service domain catalog.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// productResponse adalah bentuk body JSON yang dikembalikan endpoint produk.
type productResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
}

// Routes mendaftarkan rute domain catalog pada mux yang diberikan composition
// root. Tiap domain memasang rutenya sendiri sehingga penambahan/penghapusan
// domain terlokalisasi (siap ekstraksi).
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/catalog/products/{id}", h.getProduct)
}

// getProduct menangani GET /api/catalog/products/{id}.
func (h *Handler) getProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "produk tidak ditemukan"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "kesalahan internal"})
		return
	}
	writeJSON(w, http.StatusOK, productResponse{ID: p.ID, Name: p.Name, PriceCents: p.PriceCents})
}

// writeJSON menulis status + body JSON. Helper lokal ke domain (mandiri, tak
// bergantung paket transport bersama).
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
