// Package main adalah entrypoint service "svc-b".
//
// Service ini menjalankan gRPC server yang mengimplementasikan Ping.
package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	svc_bv1 "github.com/example/demo-ms/gen/go/svc-b/v1"
	"github.com/example/demo-ms/libs/health"
	"github.com/example/demo-ms/libs/logger"
	"github.com/example/demo-ms/services/svc-b/internal/config"
	"github.com/example/demo-ms/services/svc-b/internal/server"
)

func main() {
	log := logger.New("svc-b")
	cfg := config.Load()

	// signal.NotifyContext membatalkan ctx saat SIGINT/SIGTERM → graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── gRPC server ──────────────────────────────────────────────────────────
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Error("gagal listen gRPC", "addr", cfg.GRPCAddr, "error", err)
		os.Exit(1)
	}
	grpcSrv := grpc.NewServer()
	svc_bv1.RegisterSvcBServiceServer(grpcSrv, server.New(log))
	health.Register(grpcSrv)

	grpcErr := make(chan error, 1)
	go func() {
		log.Info("gRPC server mendengarkan", "addr", cfg.GRPCAddr)
		grpcErr <- grpcSrv.Serve(lis)
	}()

	// Tunggu error fatal atau sinyal shutdown.
	select {
	case err := <-grpcErr:
		if err != nil {
			log.Error("gRPC server berhenti dengan error", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		log.Info("sinyal shutdown diterima, menutup service")
	}

	// Graceful shutdown: hentikan gRPC dengan rapi.
	grpcSrv.GracefulStop()
}
