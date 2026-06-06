// Package main adalah entrypoint service "svc-a".
//
// Service ini menjalankan gRPC server yang mengimplementasikan Ping.
// Sebagai service PERTAMA, ia JUGA menjalankan HTTP server kecil (endpoint
// /call?to=<svc>) yang memanggil service lain via gRPC — bukti inter-service call.
package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	svc_av1 "github.com/example/demo-ms/gen/go/svc-a/v1"
	"github.com/example/demo-ms/libs/health"
	"github.com/example/demo-ms/libs/logger"
	"github.com/example/demo-ms/services/svc-a/internal/config"
	"github.com/example/demo-ms/services/svc-a/internal/gateway"
	"github.com/example/demo-ms/services/svc-a/internal/server"
)

func main() {
	log := logger.New("svc-a")
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
	svc_av1.RegisterSvcAServiceServer(grpcSrv, server.New(log))
	health.Register(grpcSrv)

	grpcErr := make(chan error, 1)
	go func() {
		log.Info("gRPC server mendengarkan", "addr", cfg.GRPCAddr)
		grpcErr <- grpcSrv.Serve(lis)
	}()

	// ── HTTP gateway (service pertama) ───────────────────────────────────────
	gw := gateway.New(log, cfg.Downstreams)
	httpSrv := &http.Server{
		Addr:        cfg.HTTPAddr,
		Handler:     gw.Handler(),
		ReadTimeout: 5 * time.Second,
	}
	httpErr := make(chan error, 1)
	go func() {
		log.Info("HTTP gateway mendengarkan", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErr <- err
			return
		}
		httpErr <- nil
	}()

	// Tunggu error fatal atau sinyal shutdown.
	select {
	case err := <-grpcErr:
		if err != nil {
			log.Error("gRPC server berhenti dengan error", "error", err)
			os.Exit(1)
		}
	case err := <-httpErr:
		if err != nil {
			log.Error("HTTP gateway berhenti dengan error", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		log.Info("sinyal shutdown diterima, menutup service")
	}

	// Graceful shutdown: hentikan gRPC dengan rapi, lalu HTTP.
	grpcSrv.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("gagal menutup HTTP gateway", "error", err)
	}
}
