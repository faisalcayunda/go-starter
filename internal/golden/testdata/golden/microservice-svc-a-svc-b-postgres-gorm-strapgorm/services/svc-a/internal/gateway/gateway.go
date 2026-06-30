// Package gateway menyediakan HTTP server kecil milik service "svc-a"
// (service pertama). Endpoint GET /call?to=<svc> mem-dial service tujuan via
// libs/grpcclient lalu memanggil Ping("ping") dan membalas JSON {"downstream": ...}
// — bukti konkret dua service saling memanggil via gRPC (T4.2 acceptance).
package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"google.golang.org/grpc"

	svc_bv1 "example.com/shopms/gen/go/svc-b/v1"
	"example.com/shopms/libs/grpcclient"
	"example.com/shopms/services/svc-a/internal/config"
)

// pinger menyeragamkan klien Ping lintas service tujuan (tiap service punya tipe
// client tersendiri dari gen/, namun semua mengekspos Ping dengan bentuk sama).
type pinger interface {
	ping(conn *grpc.ClientConn, name string) (string, error)
}

// svcBPinger memanggil Ping pada service "svc-b".
type svcBPinger struct{}

func (svcBPinger) ping(conn *grpc.ClientConn, name string) (string, error) {
	client := svc_bv1.NewSvcBServiceClient(conn)
	ctx, cancel := grpcclient.CallContext(context.Background(), 0)
	defer cancel()
	resp, err := client.Ping(ctx, &svc_bv1.PingRequest{Name: name})
	if err != nil {
		return "", err
	}
	return resp.GetMessage(), nil
}

// Gateway memetakan nama service tujuan → (alamat gRPC, pinger).
type Gateway struct {
	log     *slog.Logger
	targets map[string]target
}

type target struct {
	addr   string
	pinger pinger
}

// New membangun Gateway dari daftar downstream pada konfigurasi.
func New(log *slog.Logger, downstreams []config.Downstream) *Gateway {
	targets := make(map[string]target, len(downstreams))
	for _, d := range downstreams {
		targets[d.Name] = target{addr: d.Addr, pinger: pingerFor(d.Name)}
	}
	return &Gateway{log: log, targets: targets}
}

// pingerFor memilih pinger sesuai nama service tujuan.
func pingerFor(name string) pinger {
	switch name {
	case "svc-b":
		return svcBPinger{}
	default:
		return nil
	}
}

// Handler mengembalikan http.Handler dengan rute /call dan /healthz.
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /call", g.handleCall)
	return mux
}

// handleCall mem-dial service tujuan (?to=<svc>) lalu memanggil Ping("ping").
func (g *Gateway) handleCall(w http.ResponseWriter, r *http.Request) {
	to := r.URL.Query().Get("to")
	tgt, ok := g.targets[to]
	if !ok || tgt.pinger == nil {
		http.Error(w, "service tujuan tidak dikenal: "+to, http.StatusBadRequest)
		return
	}

	conn, err := grpcclient.Dial(tgt.addr)
	if err != nil {
		g.log.Error("gagal dial", slog.String("to", to), slog.Any("error", err))
		http.Error(w, "gagal menghubungi service tujuan", http.StatusBadGateway)
		return
	}
	defer func() { _ = conn.Close() }()

	msg, err := tgt.pinger.ping(conn, "ping")
	if err != nil {
		g.log.Error("gagal Ping", slog.String("to", to), slog.Any("error", err))
		http.Error(w, "service tujuan menolak panggilan", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"downstream": msg})
}
