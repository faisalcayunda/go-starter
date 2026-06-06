# demo-modular

Project Go best-practice (**modular monolith**, `net/http` stdlib).

- **Module path:** `github.com/example/demo-modular`
- **Arsitektur:** modular monolith (siap ekstraksi ke microservice)
- **HTTP:** `net/http` (stdlib, routing Go 1.22+)

## Struktur

```
cmd/demo-modular/main.go   composition root TUNGGAL — rakit semua domain + server
internal/
  config/                  pemuat konfigurasi dari environment (stdlib, shared)
  httpserver/              konstruksi *http.Server netral-domain + /healthz
  shared/
    contract/              kontrak (port) antar-domain — interface + tipe bersama
  modules/
    catalog/               domain "katalog produk"
      catalog.go           facade publik (yang boleh disentuh composition root)
      internal/core/       implementasi PRIVAT (berduri) — tak bisa di-import domain lain
        model.go  repository.go  service.go  handler.go
    orders/                domain "pesanan"
      orders.go            facade publik
      internal/core/       implementasi PRIVAT (berduri)
        model.go  repository.go  id.go  service.go  handler.go
```

> **Boundary berduri yang dipaksa compiler:** detail tiap domain hidup di
> `internal/modules/<domain>/internal/core/`. Menurut semantik direktori
> `internal/` Go, paket itu **hanya** bisa di-import dari bawah
> `internal/modules/<domain>/`. Jadi `orders` **secara fisik tak bisa** menyentuh
> `catalog/internal/core` — `go build` menolaknya. Domain hanya saling kenal lewat
> `shared/contract`.

## Apa itu "modular monolith"?

Satu binary, satu deploy (seperti monolith) — tetapi kode dipecah menjadi
**domain yang loosely coupled**, masing-masing dengan batas tegas. Tiap domain
tinggal di `internal/<domain>/` sehingga **compiler Go menolak** kode domain lain
meng-import isinya (semantik direktori `internal/`). Batas ini "berduri": bukan
sekadar konvensi, melainkan dipaksa toolchain.

## Aturan boundary (wajib dijaga)

1. **Domain TIDAK saling meng-import langsung.** `orders` tidak boleh
   `import ".../internal/catalog"`. Compiler menolaknya.
2. **Komunikasi antar-domain hanya lewat `internal/shared/contract`** — interface
   kecil (port) + tipe data bersama yang minimal.
3. **Composition root (`cmd/demo-modular/main.go`) satu-satunya tempat
   domain "saling kenal"** — di sanalah implementasi konkret di-inject ke interface.

## Contoh komunikasi antar-domain (orders → catalog)

Saat membuat pesanan, domain `orders` perlu memverifikasi produk & mengambil
harganya dari domain `catalog`. Ia melakukannya **lewat kontrak**, bukan paket
konkret:

- `internal/shared/contract` mendefinisikan `Catalog` interface (`Lookup`).
- `catalog.Service` **mengimplementasikan** `contract.Catalog`.
- domain `orders` **bergantung pada** `contract.Catalog` (interface), bukan pada
  paket `catalog`.
- `main.go` meng-inject `catalogMod.Service()` ke `orders.New(...)` sebagai
  `contract.Catalog`.

Hasilnya: `orders` dapat dites terisolasi dengan `contract.Catalog` tiruan (lihat
`internal/modules/orders/internal/core/handler_test.go`) — tanpa DB, tanpa domain
catalog.

## Menjalankan

```bash
go run ./cmd/demo-modular
```

Server berjalan di `:8080` (atur lewat `PORT`).

```bash
curl localhost:8080/healthz                         # {"status":"ok"}
curl localhost:8080/api/catalog/products/p-1         # produk dari domain catalog
curl -X POST localhost:8080/api/orders \
  -H 'Content-Type: application/json' \
  -d '{"product_id":"p-1","quantity":2}'                      # pesanan (verifikasi via catalog)
```

## Build & test

```bash
go vet ./...
go build ./...
go test ./...
```

Ketiganya hijau tanpa edit manual & tanpa jaringan (profil tanpa database = murni
stdlib, nol dependency eksternal).

## Cara ekstraksi sebuah domain ke microservice

Justru di sinilah modular monolith membayar dirinya. Untuk memindah `catalog`
menjadi service terpisah:

1. **Pindahkan `internal/modules/catalog/` apa adanya** ke repo/service baru —
   isinya sudah mandiri (tak ada domain lain yang meng-importnya).
2. **Buat klien** (gRPC/HTTP) yang mengimplementasikan `contract.Catalog` di sisi
   `orders`, menggantikan pemanggilan in-process dengan panggilan jaringan.
3. **Inject klien itu** di composition root alih-alih `catalog.Service`.
   `orders.Service` **tidak berubah satu baris pun** — ia hanya tahu interface.
4. Pertahankan `contract` sebagai paket/share bersama (atau salin definisinya ke
   kontrak proto/OpenAPI) agar kedua sisi sepakat.

Karena seluruh ketergantungan lintas-domain sudah menempel pada interface di
`shared/contract`, ekstraksi menjadi operasi terlokalisasi — bukan rewrite.

## Menambah domain baru

1. Buat `internal/modules/<domain>/` dengan facade `<domain>.go` + sub-paket
   privat `internal/core/` (model, repository, service, handler) — ikuti pola
   `catalog`/`orders`.
2. Bila domain perlu dipanggil domain lain, tambahkan port-nya di
   `internal/shared/contract`.
3. Rakit facade & pasang RegisterRoutes-nya di composition root (`main.go`).

## Konfigurasi environment

| Variabel | Default | Keterangan |
|---|---|---|
| `HOST` | _(kosong)_ | Alamat bind; kosong = semua interface |
| `PORT` | `8080` | Port HTTP server |
| `READ_TIMEOUT` | `5` | Timeout baca request (detik) |
| `WRITE_TIMEOUT` | `10` | Timeout tulis response (detik) |
| `SHUTDOWN_TIMEOUT` | `10` | Tenggang graceful shutdown (detik) |
