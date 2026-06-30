// Package server mengimplementasikan gRPC SvcAService untuk
// service "svc-a". Ia meng-embed Unimplemented...Server (forward-compat:
// menambah rpc baru di proto tak memecah build) lalu menyediakan Ping.
package server

import (
	"context"
	"log/slog"

	svc_av1 "example.com/shopms/gen/go/svc-a/v1"
)

// Server adalah implementasi SvcAServiceServer.
type Server struct {
	svc_av1.UnimplementedSvcAServiceServer
	log *slog.Logger
}

// New membangun Server dengan logger terstruktur.
func New(log *slog.Logger) *Server {
	return &Server{log: log}
}

// Ping mengembalikan "pong dari service svc-a: <name>" — implementasi
// minimal yang membuktikan service hidup & dapat dipanggil lintas-service via
// gRPC. Pesan menyebut identitas service penjawab sehingga pemanggil dapat
// memverifikasi service tujuan mana yang benar-benar merespons.
func (s *Server) Ping(ctx context.Context, req *svc_av1.PingRequest) (*svc_av1.PingResponse, error) {
	s.log.Info("Ping diterima", slog.String("name", req.GetName()))
	return &svc_av1.PingResponse{Message: "pong dari service svc-a: " + req.GetName()}, nil
}
