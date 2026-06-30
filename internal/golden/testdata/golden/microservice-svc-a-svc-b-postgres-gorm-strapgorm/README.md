# shopms

Project Go best-practice (**microservice**, monorepo single-module, komunikasi **gRPC**).

- **Module path:** `example.com/shopms`
- **Arsitektur:** microservice monorepo single-module (satu `go.mod` di root)
- **Kontrak:** Protobuf di `proto/`, stub gRPC di `gen/go/` (**di-commit**)
- **Komunikasi:** gRPC antar service

## Struktur

```
go.mod                     satu module: example.com/shopms
buf.yaml  buf.gen.yaml      konfigurasi buf (lint + codegen managed mode)
Makefile                   target: proto, build, test, up, down
docker-compose.yml         orkestrasi semua service (build dari Dockerfile)
Dockerfile                 image multi-service (build arg SERVICE memilih service)
proto/                     KONTRAK proto per service (sumber kebenaran)
  <svc>/v1/<svc>.proto
gen/go/                    STUB hasil `buf generate` — DI-COMMIT (build hijau tanpa buf)
  <svc>/v1/<svc>.pb.go  <svc>/v1/<svc>_grpc.pb.go
libs/                      kode bersama antar service (dimiliki project, bukan builder)
  config/                  loader env (stdlib)
  logger/                  slog terstruktur + atribut service
  health/                  pendaftaran gRPC Health Checking Protocol
  grpcclient/              helper dial + timeout RPC
services/                  unit deploy — satu folder per service
  <svc>/
    cmd/main.go            entrypoint: gRPC server (+ HTTP /call pada service pertama)
    internal/
      config/              konfigurasi runtime service
      server/             implementasi PingServer
      gateway/             (service pertama) HTTP /call → gRPC service lain
```

## Prasyarat

Build & run project **tidak** memerlukan `buf` — stub `gen/go/` sudah di-commit.
`buf` hanya diperlukan saat **meregenerasi** stub setelah mengubah `proto/`:

```bash
go install github.com/bufbuild/buf/cmd/buf@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

## Menjalankan

```bash
go build ./...                 # hijau tanpa buf (gen/ sudah di-commit)
go test ./...
docker compose up --build      # menyalakan semua service
make proto                     # regenerasi gen/ setelah mengubah proto/
```

## Bukti inter-service call

Tiap service mengekspos `rpc Ping(PingRequest) returns (PingResponse)`. Service
**pertama** juga menjalankan HTTP kecil dengan endpoint `GET /call?to=<svc>` yang
mem-dial service tujuan via `libs/grpcclient`, memanggil `Ping("ping")`, lalu
membalas JSON `{"downstream": "pong dari service <svc>: ping"}` — bukti dua service
saling memanggil via gRPC (pesan menyebut identitas service penjawab):

```bash
# setelah `docker compose up`, dari host:
curl "http://localhost:8080/call?to=<service-kedua>"
# → {"downstream":"pong dari service <service-kedua>: ping"}
```

## Menambah service

```bash
gostarter add service <nama>
```

Subcommand ini menambah `services/<nama>/`, `proto/<nama>/v1/<nama>.proto`,
meregenerasi `gen/go/<nama>/`, dan mendaftarkan service ke `docker-compose.yml`.
