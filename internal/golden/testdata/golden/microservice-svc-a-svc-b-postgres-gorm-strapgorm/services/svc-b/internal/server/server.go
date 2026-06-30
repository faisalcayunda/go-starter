// Package server mengimplementasikan gRPC SvcBService untuk
// service "svc-b". Ia meng-embed Unimplemented...Server (forward-compat:
// menambah rpc baru di proto tak memecah build) lalu menyediakan Ping.
package server

import (
	"context"
	"log/slog"

	svc_bv1 "example.com/shopms/gen/go/svc-b/v1"
)

// Server adalah implementasi SvcBServiceServer.
type Server struct {
	svc_bv1.UnimplementedSvcBServiceServer
	log *slog.Logger
}

// New membangun Server dengan logger terstruktur.
func New(log *slog.Logger) *Server {
	return &Server{log: log}
}

// Ping mengembalikan "pong dari service svc-b: <name>" — implementasi
// minimal yang membuktikan service hidup & dapat dipanggil lintas-service via
// gRPC. Pesan menyebut identitas service penjawab sehingga pemanggil dapat
// memverifikasi service tujuan mana yang benar-benar merespons.
func (s *Server) Ping(ctx context.Context, req *svc_bv1.PingRequest) (*svc_bv1.PingResponse, error) {
	s.log.Info("Ping diterima", slog.String("name", req.GetName()))
	return &svc_bv1.PingResponse{Message: "pong dari service svc-b: " + req.GetName()}, nil
}
