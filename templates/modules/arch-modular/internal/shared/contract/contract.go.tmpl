// Package contract memuat kontrak antar-domain (port) modular monolith.
//
// Inilah SATU-SATUNYA cara domain saling berkomunikasi: lewat interface kecil
// yang didefinisikan di sini, BUKAN dengan meng-import paket internal domain
// tetangga. Compiler Go menegakkan batas ini — paket di internal/<domain> tidak
// bisa di-import lintas domain (boundary "berduri"), sehingga ketergantungan
// antar-domain hanya boleh menempel pada interface di paket netral ini.
//
// Saat ekstraksi ke microservice nanti: interface di sini tetap, hanya
// implementasinya yang diganti dari pemanggilan in-process (langsung) menjadi
// klien gRPC/HTTP ke service terpisah. Domain pemakai tidak perlu berubah.
package contract

import (
	"context"
	"errors"
)

// ErrNotFound adalah error kontrak lintas-domain untuk "entitas tidak ditemukan".
// Domain penyedia (mis. catalog) boleh mengembalikan error ini lewat port agar
// domain pemakai dapat membedakan "tidak ada" dari kegagalan lain tanpa
// bergantung pada tipe error internal domain penyedia.
var ErrNotFound = errors.New("contract: entitas tidak ditemukan")

// Product adalah representasi minimal sebuah produk yang boleh dilihat domain
// lain. Hanya field yang memang perlu dibagikan lintas-domain yang ada di sini;
// detail internal domain catalog tidak bocor lewat tipe ini.
type Product struct {
	// ID adalah identitas unik produk.
	ID string
	// Name adalah nama produk yang ditampilkan.
	Name string
	// PriceCents adalah harga dalam satuan sen (integer, hindari float untuk uang).
	PriceCents int64
}

// Catalog adalah port (kontrak) yang diekspos domain catalog ke domain lain.
// Domain orders memanggil Lookup lewat interface ini untuk memverifikasi produk
// dan mengambil harga saat membuat pesanan — TANPA mengetahui implementasi
// konkret catalog. Inilah titik komunikasi in-process antar-domain.
type Catalog interface {
	// Lookup mengembalikan produk untuk id tertentu. Mengembalikan error bila
	// produk tidak ditemukan, sehingga pemanggil bisa menolak permintaan dengan
	// rapi alih-alih bekerja dengan data parsial.
	Lookup(ctx context.Context, id string) (Product, error)
}
