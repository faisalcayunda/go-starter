package core

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Handler adalah adapter transport HTTP domain orders. Tipis: menerjemahkan
// request → Service → response JSON.
type Handler struct {
	svc *Service
}

// NewHandler merakit Handler dengan Service domain orders.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// createOrderRequest adalah bentuk body JSON permintaan pembuatan pesanan.
type createOrderRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// orderResponse adalah bentuk body JSON pesanan yang dikembalikan.
type orderResponse struct {
	ID         string `json:"id"`
	ProductID  string `json:"product_id"`
	Quantity   int    `json:"quantity"`
	TotalCents int64  `json:"total_cents"`
}

// Routes mendaftarkan rute domain orders pada mux composition root.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/orders", h.createOrder)
}

// createOrder menangani POST /api/orders. Ia mendelegasikan verifikasi produk &
// perhitungan total ke Service (yang memanggil domain catalog lewat kontrak).
func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body JSON tidak valid"})
		return
	}

	order, err := h.svc.Create(r.Context(), req.ProductID, req.Quantity)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "produk tidak ada di katalog"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, orderResponse{
		ID:         order.ID,
		ProductID:  order.ProductID,
		Quantity:   order.Quantity,
		TotalCents: order.TotalCents,
	})
}

// writeJSON menulis status + body JSON. Helper lokal ke domain (mandiri).
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
