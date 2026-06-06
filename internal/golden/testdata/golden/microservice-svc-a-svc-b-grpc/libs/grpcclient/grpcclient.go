// Package grpcclient adalah helper bersama untuk men-dial service lain via gRPC.
// Memusatkan opsi koneksi (kredensial insecure untuk komunikasi internal cluster,
// timeout dial) agar tiap pemanggil tidak mengulang boilerplate yang sama.
package grpcclient

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Dial membuka koneksi gRPC ke addr (mis. "user:9090") dengan kredensial insecure
// — pola lazim untuk trafik service-to-service di dalam jaringan privat (compose/
// k8s). Untuk lintas-batas tepercaya, ganti opsi credentials di sini.
//
// Pemanggil bertanggung jawab memanggil Close() pada koneksi yang dikembalikan.
func Dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

// CallContext mengembalikan context dengan timeout standar untuk satu RPC unary,
// beserta fungsi cancel-nya. Default 5 detik bila timeout ≤ 0.
func CallContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(parent, timeout)
}
