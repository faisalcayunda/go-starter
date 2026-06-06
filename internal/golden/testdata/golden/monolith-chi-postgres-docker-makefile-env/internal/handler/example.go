// Package handler memuat HTTP handler contoh project hasil generate. Handler di
// sini murni stdlib (net/http) tanpa dependency database, sehingga profil default
// langsung kompilasi & lulus test secara offline. Ganti/ tambah handler ini
// dengan logika domain Anda.
package handler

import (
	"encoding/json"
	"net/http"
)

// helloResponse adalah bentuk body JSON yang dikembalikan Hello.
type helloResponse struct {
	Message string `json:"message"`
}

// Hello adalah handler contoh untuk GET /api/hello. Ia mengembalikan salam JSON.
// Bila query string "name" diberikan, salam disesuaikan; jika tidak, memakai
// "world" sebagai default.
func Hello(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "world"
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(helloResponse{Message: "hello, " + name})
}
