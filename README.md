# gostarter

> **Generator project Go best-practice — UX gaya `laravel new`.** Satu perintah, sebuah wizard ringkas (atau flag lengkap), lalu project Go yang langsung jalan: `go build ./...` hijau, struktur rapi, tanpa edit manual.

[![CI](https://github.com/faisalcayunda/gostarter/actions/workflows/ci.yml/badge.svg)](https://github.com/faisalcayunda/gostarter/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/faisalcayunda/gostarter.svg)](https://pkg.go.dev/github.com/faisalcayunda/gostarter)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go)](go.mod)

---

## Daftar Isi

- [Apa Itu gostarter](#apa-itu-gostarter)
- [Fitur Utama & Diferensiator](#fitur-utama--diferensiator)
- [Quickstart](#quickstart)
- [Instalasi](#instalasi)
- [Penggunaan](#penggunaan)
  - [Mode Interaktif (wizard `huh`)](#mode-interaktif-wizard-huh)
  - [Mode Non-Interaktif (flag)](#mode-non-interaktif-flag)
  - [Preset `--config`](#preset---config)
  - [`add service` — tambah service inkremental](#add-service--tambah-service-inkremental)
- [Tabel Opsi yang Didukung](#tabel-opsi-yang-didukung)
- [Contoh Output Tree per Arsitektur](#contoh-output-tree-per-arsitektur)
- [Demo](#demo)
- [Dokumentasi](#dokumentasi)
- [Lisensi](#lisensi)

---

## Apa Itu gostarter

`gostarter` men-generate struktur project Go best-practice dalam hitungan detik. Pilih arsitektur, framework HTTP, database, dan add-on lewat wizard interaktif atau flag — generator merakit project lengkap dengan composition root, handler contoh, health check, konfigurasi, dan (opsional) migrasi DB, Dockerfile, Makefile, CI, observability.

Project hasil generate **tidak meng-import apa pun dari builder** — begitu di-generate, ia sepenuhnya milik Anda (**zero lock-in**).

---

## Fitur Utama & Diferensiator

- **3 paradigma arsitektur dalam satu alat** — `monolith` (layered standar), `modular-monolith`, dan `microservice` (monorepo single-module, N service sekaligus). Bukan sekadar satu template; tiga model deploy yang berbeda secara fundamental.
- **Modular monolith sebagai mode pertama-kelas** — boundary domain dipaksakan **oleh compiler** lewat direktori `internal/` per-modul. Jalur sah antar-modul hanya `internal/shared/contract`; import lintas-`internal/` ditolak `go build`. Migrasi ke microservice jadi mekanis, bukan rewrite.
- **`add service` inkremental** — tambah service gRPC baru ke project microservice existing tanpa men-scaffold ulang: generate `services/<name>` + `proto/<name>/v1`, sisip blok ke `docker-compose.yml`, lalu `buf generate` → `gofmt` → `go mod tidy`.
- **Zero lock-in** — output adalah project Go biasa. Tidak ada runtime/SDK builder yang ikut ter-import. Hapus `gostarter`, project tetap jalan.
- **Build-hijau-terjamin** — setiap kombinasi opsi yang didukung dijaga oleh **golden snapshot byte-identical** dan matrix CI; output flag-path identik byte-per-byte dengan output wizard (SPEC §5.2).

---

## Quickstart

```bash
# 1. Install
go install github.com/faisalcayunda/gostarter/cmd/gostarter@latest

# 2. Generate project (wizard interaktif)
gostarter create

# 3. Atau non-interaktif, satu baris
gostarter create --name shop --arch modular-monolith --http chi --db postgres \
  --addons makefile,env,golangci

# 4. Jalankan
cd shop && go build ./...
```

---

## Instalasi

### `go install` (disarankan)

```bash
go install github.com/faisalcayunda/gostarter/cmd/gostarter@latest
```

Binary terpasang di `$(go env GOPATH)/bin/gostarter`. Pastikan direktori itu ada di `PATH`.

### Binary rilis (pre-built)

Unduh arsip untuk OS/arch Anda dari halaman [Releases](https://github.com/faisalcayunda/gostarter/releases), ekstrak, dan letakkan `gostarter` di `PATH`. Tersedia untuk `linux`, `darwin`, `windows` × `amd64`, `arm64`. Lihat [docs/release.md](docs/release.md) untuk tabel binary dan verifikasi checksum.

### Dari source

```bash
git clone https://github.com/faisalcayunda/gostarter
cd gostarter
go build -o gostarter ./cmd/gostarter
```

### Verifikasi versi

```bash
gostarter --version
# gostarter version vX.Y.Z   (binary rilis)
# gostarter version 0.0.0-dev (build dari source tanpa ldflags)
```

> **Prasyarat arsitektur `microservice`:** `buf` + plugin Go (`protoc-gen-go`, `protoc-gen-go-grpc`) harus ada di `PATH` — generator menjalankan `buf generate` untuk membuat stub gRPC.

---

## Penggunaan

`gostarter` punya dua command:

| Command | Fungsi |
|---|---|
| `gostarter create` | Generate project Go baru (interaktif atau flag). |
| `gostarter add service <name>` | Tambah satu service gRPC ke project microservice existing. |

### Mode Interaktif (wizard `huh`)

Tanpa flag wajib (`--name`) dan tanpa `--non-interactive`, `create` menjalankan wizard:

```bash
gostarter create
```

Wizard menanyakan nama, module path, arsitektur, framework HTTP, database, dan add-on secara berurutan. Output-nya **byte-identical** dengan jalur flag yang setara.

### Mode Non-Interaktif (flag)

Sediakan `--name` (atau `--non-interactive`) untuk melewati wizard:

```bash
gostarter create \
  --name shop \
  --module github.com/acme/shop \
  --arch modular-monolith \
  --http chi \
  --db postgres \
  --migrate golang-migrate \
  --addons docker,makefile,golangci,env,ci,observability \
  --ci github-actions \
  --output ./shop \
  --non-interactive
```

Praktis lain:

```bash
# Lihat rencana tanpa menulis file
gostarter create --name shop --arch monolith --dry-run --non-interactive

# Lewati semua konfirmasi
gostarter create --name shop --arch monolith --yes --non-interactive

# Microservice: dua service + API gateway
gostarter create --name platform --arch microservice \
  --services order,user --comm grpc --gateway --non-interactive

# Microservice dengan flag --service repeatable (setara --services)
gostarter create --name platform --arch microservice \
  --service order --service user --comm grpc --non-interactive
```

### Preset `--config`

Simpan jawaban di file YAML dan teruskan lewat `--config`. Mode ini non-interaktif (wizard di-skip). **Presedensi:** `default < preset < flag eksplisit` — flag CLI selalu menang atas preset.

```yaml
# preset.yaml
name:     shop
module:   github.com/acme/shop
arch:     modular-monolith      # monolith | modular-monolith | microservice
kind:     rest
http:     chi                   # net/http | chi | echo
db:       postgres              # none | postgres | mysql
migrate:  golang-migrate
docker:   true
makefile: true
golangci: true                  # alias: lint
env:      true                  # alias: env-example
ci:       github-actions        # github-actions | gitlab-ci | none
obs:      true                  # observability (otel + /metrics + health)
git:      false
```

```bash
gostarter create --config preset.yaml
# Override satu nilai dari preset lewat flag:
gostarter create --config preset.yaml --http echo
```

### `add service` — tambah service inkremental

Dijalankan dari **root** project microservice (atau pakai `-o <dir>`):

```bash
gostarter add service payment
```

Menghasilkan `services/payment/{cmd,internal}` + `proto/payment/v1/payment.proto`, menyisipkan blok service ke `docker-compose.yml`, lalu `buf generate` → `gofmt` → `go mod tidy`. Command menolak jika dijalankan di luar project microservice gostarter, atau jika nama service sudah ada / reserved (`gateway`).

```bash
# Preview tanpa menulis file
gostarter add service payment --dry-run
```

---

## Tabel Opsi yang Didukung

### `gostarter create`

| Flag | Nilai | Default | Keterangan |
|---|---|---|---|
| `--name` | `^[a-z][a-z0-9-]*$` | — | Nama project. Wajib di mode non-interaktif. |
| `--module` | path Go | `github.com/<name>` | Go module path. |
| `--arch` | `monolith` \| `modular-monolith` \| `microservice` | `monolith` | Arsitektur project. |
| `--kind` | `rest` | `rest` | Jenis aplikasi. |
| `--http` | `net/http` \| `chi` \| `echo` | `net/http` | Framework HTTP (monolith / modular-monolith). |
| `--db` | `none` \| `postgres` \| `mysql` | `none` | Database. |
| `--access` | `sqlx` | — | Lapisan akses query (butuh `--db≠none`). |
| `--migrate` | `golang-migrate` | — | Tool migrasi (butuh `--db≠none`). |
| `--addons` | `docker,makefile,golangci,env,ci,observability` (csv) | — | Add-on yang diaktifkan. |
| `--feature` | (sama seperti `--addons`) | — | Add-on tambahan, digabung union dengan `--addons`. |
| `--ci` | `github-actions` \| `gitlab-ci` | `github-actions` | Provider CI (saat addon `ci` aktif). |
| `--comm` | `grpc` | `grpc` | Pola komunikasi microservice (rest/event menyusul). |
| `--services` | csv, mis. `order,user` | — | Daftar service (microservice). |
| `--service` | repeatable, mis. `--service order --service user` | — | Nama service (microservice). |
| `--gateway` / `--no-gateway` | (boolean) | `--no-gateway` (gateway OFF) | Aktifkan / nonaktifkan API gateway (REST edge → gRPC). Default OFF — microservice murni gRPC. Pakai `--gateway` untuk mengaktifkan; `--no-gateway` menang atas `--gateway`. |
| `--config` | `<file.yaml>` | — | Preset jawaban dari YAML (`default < preset < flag`). |
| `--output`, `-o` | direktori | `./<name>` | Direktori output. |
| `--dry-run` | (boolean) | — | Cetak rencana tanpa menulis ke disk. |
| `--yes` | (boolean) | — | Lewati konfirmasi. |
| `--non-interactive` | (boolean) | — | Paksa mode flag-only (wizard di-skip). |
| `--git` / `--no-git` | (boolean) | `--no-git` (non-interaktif) | Jalankan / lewati `git init` + initial commit. |

> `--version` / `-v` adalah flag **root** (`gostarter --version`), bukan flag `create`. Lihat [Verifikasi versi](#verifikasi-versi).

### `gostarter add service <name>`

| Flag | Nilai | Default | Keterangan |
|---|---|---|---|
| `--output`, `-o` | direktori | direktori kerja | Root project microservice. |
| `--dry-run` | (boolean) | — | Preview file baru tanpa menulis ke disk. |

> Tabel ini diverifikasi langsung terhadap `gostarter create --help` dan `gostarter add service --help`.

---

## Contoh Output Tree per Arsitektur

### `monolith` (`--http chi --db postgres --addons makefile,env`)

Satu unit deploy, satu binary. Layout layered standar.

```
monoapi/
├── cmd/monoapi/main.go              # composition root tunggal
├── internal/
│   ├── app/app.go                   # perakitan dependency
│   ├── config/config.go
│   ├── handler/
│   │   ├── example.go
│   │   └── example_test.go
│   ├── httpserver/
│   │   ├── health.go
│   │   └── server.go                # chi router
│   └── platform/database/postgres.go
├── migrations/
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── .env.example                     # addon: env
├── .gitignore
├── Makefile                         # addon: makefile
├── README.md
├── go.mod
└── go.sum
```

### `modular-monolith` (`--http chi --db postgres --addons makefile`)

Satu binary, tapi domain dipisah jadi modul dengan `internal/` per-modul — boundary dipaksakan compiler. Antar-modul hanya lewat `internal/shared/contract`.

```
modularapi/
├── cmd/modularapi/main.go
├── internal/
│   ├── config/config.go
│   ├── httpserver/
│   │   ├── health.go
│   │   └── server.go
│   ├── modules/
│   │   ├── catalog/
│   │   │   ├── catalog.go            # API publik modul
│   │   │   └── internal/core/        # boundary berduri (private)
│   │   │       ├── handler.go
│   │   │       ├── handler_test.go
│   │   │       ├── model.go
│   │   │       ├── repository.go
│   │   │       └── service.go
│   │   └── orders/
│   │       ├── orders.go
│   │       └── internal/core/
│   │           ├── handler.go
│   │           ├── handler_test.go
│   │           ├── id.go
│   │           ├── model.go
│   │           ├── repository.go
│   │           └── service.go
│   ├── platform/database/postgres.go
│   └── shared/contract/contract.go  # satu-satunya jalur antar-modul
├── migrations/
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── .gitignore
├── Makefile
├── README.md
├── go.mod
└── go.sum
```

### `microservice` (`--services order,user --comm grpc --gateway`)

Monorepo single-module, N service sekaligus. Stub gRPC di `gen/go/` di-generate `buf`.

```
microsvc/
├── proto/
│   ├── order/v1/order.proto
│   └── user/v1/user.proto
├── gen/go/                          # stub gRPC (buf generate)
│   ├── order/v1/{order.pb.go, order_grpc.pb.go}
│   └── user/v1/{user.pb.go, user_grpc.pb.go}
├── services/
│   ├── order/
│   │   ├── cmd/main.go
│   │   └── internal/
│   │       ├── config/config.go
│   │       ├── gateway/gateway.go   # addon: gateway (REST edge → gRPC)
│   │       └── server/server.go
│   └── user/
│       ├── cmd/main.go
│       └── internal/
│           ├── config/config.go
│           └── server/server.go
├── libs/                            # shared, di-import lintas service
│   ├── config/config.go
│   ├── grpcclient/grpcclient.go
│   ├── health/health.go
│   └── logger/logger.go
├── buf.yaml
├── buf.gen.yaml
├── docker-compose.yml
├── Dockerfile
├── Makefile
├── .gitignore
├── README.md
├── go.mod
└── go.sum
```

---

## Demo

Sesi `gostarter create` ringkas (jalur non-interaktif satu-baris):

```console
$ gostarter --version
gostarter version 0.0.0-dev

$ gostarter create --name shop --arch modular-monolith \
    --http chi --db postgres --addons makefile,env,golangci \
    --non-interactive --yes
✓ project "shop" ter-generate di ./shop  (modular-monolith · chi · postgres)

$ cd shop && go build ./... && echo BUILD-OK
BUILD-OK

# project Go siap pakai — zero lock-in. 🚀
```

> **GIF demo belum di-commit.** Berkas `assets/demo.gif` **di-generate**, bukan
> di-track di repo — direktori `assets/` hanya berisi `.gitkeep`. Render GIF dengan
> [`vhs`](https://github.com/charmbracelet/vhs) dari skrip [`demo/demo.tape`](demo/demo.tape):
>
> ```bash
> # Pasang vhs (sekali): https://github.com/charmbracelet/vhs
> vhs demo/demo.tape   # menghasilkan assets/demo.gif
> ```
>
> Setelah di-render, GIF muncul di sini:
>
> ![demo gostarter](assets/demo.gif)

---

## Dokumentasi

| Dokumen | Isi |
|---|---|
| [docs/SPEC.md](docs/SPEC.md) | Spesifikasi fungsional & sumber kebenaran. |
| [docs/adr/](docs/adr/) | ADR-001 (stack builder), ADR-002 (arsitektur internal), ADR-003 (sistem template). |
| [docs/adding-modules.md](docs/adding-modules.md) | Panduan menambah template modul baru. |
| [docs/release.md](docs/release.md) | Proses rilis & tabel binary. |
| [docs/versioning.md](docs/versioning.md) | Kebijakan versioning tool & template. |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Panduan kontributor. |

---

## Lisensi

Lisensi final **TBD** — kandidat: **MIT** atau **Apache-2.0**. Lihat berkas `LICENSE` (akan ditambahkan sebelum rilis publik pertama).
