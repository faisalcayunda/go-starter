// Package main adalah entrypoint service "product" (add-on strapgorm).
//
// Service ini menjalankan DUA permukaan dalam satu binary:
//   - gRPC server (Ping + grpc.health.v1) — konsistensi dgn mesh & probe kesehatan;
//   - HTTP server (GET /api/products) — Strapi-style query builder strapgorm di atas
//     koneksi GORM SENDIRI (per-service DB).
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

	productv1 "example.com/shopms/gen/go/product/v1"
	"example.com/shopms/libs/health"
	"example.com/shopms/libs/logger"
	"example.com/shopms/services/product/internal/config"
	"example.com/shopms/services/product/internal/server"
	"example.com/shopms/services/product/internal/store"
)

func main() {
	log := logger.New("product")
	cfg := config.Load()

	// signal.NotifyContext membatalkan ctx saat SIGINT/SIGTERM → graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── DB (GORM, per-service) ───────────────────────────────────────────────
	db, err := store.Open(cfg.DBDSN)
	if err != nil {
		log.Error("buka koneksi DB", "error", err)
		os.Exit(1)
	}
	// Tutup pool koneksi saat proses keluar (rilis slot koneksi DB dengan rapi).
	defer func() { _ = store.Close(db) }()
	if err := store.AutoMigrate(db); err != nil {
		log.Error("auto-migrate products", "error", err)
		os.Exit(1)
	}

	// ── gRPC server (Ping + health) ──────────────────────────────────────────
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Error("gagal listen gRPC", "addr", cfg.GRPCAddr, "error", err)
		os.Exit(1)
	}
	grpcSrv := grpc.NewServer()
	productv1.RegisterProductServiceServer(grpcSrv, server.New(log))
	health.Register(grpcSrv)

	grpcErr := make(chan error, 1)
	go func() {
		log.Info("gRPC server mendengarkan", "addr", cfg.GRPCAddr)
		grpcErr <- grpcSrv.Serve(lis)
	}()

	// ── HTTP server (GET /api/products via strapgorm) ────────────────────────
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /api/products", store.ListHandler(db))
	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	httpErr := make(chan error, 1)
	go func() {
		log.Info("HTTP server mendengarkan", "addr", cfg.HTTPAddr)
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
			log.Error("HTTP server berhenti dengan error", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		log.Info("sinyal shutdown diterima, menutup service product")
	}

	// Graceful shutdown dibatasi cfg.ShutdownTimeout untuk KEDUA server.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	// gRPC: GracefulStop menunggu RPC aktif selesai, TAPI dibatasi tenggang shutdown.
	// Bila ada RPC yang menggantung (mis. streaming/downstream lambat), Stop()
	// memaksa terminasi agar proses tetap keluar dalam tenggang — bukan diam menunggu
	// sampai di-SIGKILL orchestrator.
	grpcStopped := make(chan struct{})
	go func() {
		grpcSrv.GracefulStop()
		close(grpcStopped)
	}()
	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcSrv.Stop()
	}

	// HTTP: graceful shutdown dengan tenggang yang sama.
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("gagal menutup HTTP server", "error", err)
	}
}
