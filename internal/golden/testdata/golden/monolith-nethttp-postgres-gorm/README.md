# demo-gorm

Project Go best-practice (monolith layered, `net/http` stdlib).

- **Module path:** `github.com/example/demo-gorm`
- **Arsitektur:** monolith (layered)
- **HTTP:** `net/http` (stdlib, routing Go 1.22+)

## Struktur

```
cmd/demo-gorm/main.go   entrypoint tipis (config + logger + app.Run)
internal/
  app/         composition root — merakit & menjalankan server (graceful shutdown)
  config/      pemuat konfigurasi dari environment (stdlib)
  handler/     HTTP handler contoh (GET /api/hello)
  httpserver/  konstruksi *http.Server + handler /healthz
```

## Menjalankan

```bash
go run ./cmd/demo-gorm
```

Server berjalan di `:8080` secara default (atur lewat `PORT`).

```bash
curl localhost:8080/healthz       # {"status":"ok"}
curl localhost:8080/api/hello      # {"message":"hello, world"}
curl 'localhost:8080/api/hello?name=gopher'
```

## Build & test

```bash
go vet ./...
go build ./...
go test ./...
```

Ketiga perintah di atas hijau tanpa edit manual dan tanpa koneksi jaringan
(profil tanpa database = murni stdlib, nol dependency eksternal).

## Konfigurasi environment

| Variabel | Default | Keterangan |
|---|---|---|
| `HOST` | _(kosong)_ | Alamat bind; kosong = semua interface |
| `PORT` | `8080` | Port HTTP server |
| `READ_TIMEOUT` | `5` | Timeout baca request (detik) |
| `WRITE_TIMEOUT` | `10` | Timeout tulis response (detik) |
| `SHUTDOWN_TIMEOUT` | `10` | Tenggang graceful shutdown (detik) |
