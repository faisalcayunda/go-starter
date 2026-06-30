package httpserver

import (
	"encoding/json"
	"net/http"
)

// Healthz adalah handler liveness/readiness sederhana: selalu mengembalikan
// 200 dengan body JSON {"status":"ok"}. Dipakai oleh load balancer / orchestrator
// (mis. health check docker-compose) untuk memastikan service hidup.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
