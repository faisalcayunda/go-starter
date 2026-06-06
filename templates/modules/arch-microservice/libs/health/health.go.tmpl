// Package health menyediakan pendaftaran gRPC Health Checking Protocol bersama.
// Tiap gRPC server memanggil Register agar klien (dan probe compose/k8s) dapat
// mengecek kesiapan service lewat protokol standar grpc.health.v1.
package health

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// Register memasang health server standar ke s dan menandai keseluruhan service
// (nama kosong "") berstatus SERVING. Mengembalikan *health.Server agar pemanggil
// dapat mengubah status saat shutdown (SetServingStatus → NOT_SERVING).
func Register(s *grpc.Server) *health.Server {
	hs := health.NewServer()
	healthpb.RegisterHealthServer(s, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	return hs
}
