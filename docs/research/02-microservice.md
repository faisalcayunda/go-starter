# R0.2 — Riset Struktur Folder: Microservice

> **Status:** Draft riset Fase 0 · **Tanggal:** 2026-06-06
> **Konteks produk:** `gostarter` — CLI generator project Go best-practice. Mode arsitektur ke-3 (microservice) harus bisa men-generate **N service sekaligus** + `docker compose` untuk menjalankan semuanya + contoh **2 service yang saling memanggil**, dengan aturan keras: hasil generate lolos `go vet ./... && go build ./... && go test ./...` tanpa edit manual, dan **TIDAK meng-import package apa pun dari builder** (zero lock-in).

---

## 1. Pendahuluan & Pertanyaan Riset

Dokumen ini meneliti struktur folder yang akan di-generate untuk mode **microservice**. Berbeda dengan monolith (R0.1) yang menghasilkan satu unit deploy, mode microservice menghasilkan **banyak unit deploy** sekaligus. Ini mengubah pertanyaan desain dari "bagaimana satu project ditata" menjadi "bagaimana **kumpulan** service ditata, di-wire, dan dijalankan bersama — dan mana yang paling mudah di-generate serta dipelihara oleh sebuah tool".

Karena `gostarter` adalah *generator* (bukan framework runtime), kriteria yang paling menentukan bukan sekadar "mana arsitektur paling benar secara teori", melainkan **mana yang paling deterministik untuk di-template, di-build dalam satu perintah, dan dijalankan dalam satu `docker compose up`**.

### Pertanyaan riset

1. **Monorepo vs polyrepo (per-repo):** mana yang lebih mudah di-generate sekaligus (N service dalam satu aksi) dan lebih mudah dipelihara/extend oleh tool? Apa implikasi masing-masing terhadap `go.mod`, build, dan compose?
2. **Contract-first (protobuf/gRPC):** di mana `proto/` diletakkan, di mana kode hasil generate ditaruh, dan bagaimana 2 service saling memanggil lewat stub gRPC?
3. **Tooling proto:** `buf` atau `protoc`? Mana yang lebih reproducible dan ramah di-generate?
4. **Shared library antar service:** di mana kode bersama (config, logger, middleware) hidup tanpa menciptakan *hidden coupling* atau melanggar zero lock-in?
5. **API gateway:** perlu di-generate sebagai service tersendiri atau cukup pola edge sederhana?
6. **Pelajaran desain** dari go-kit, go-zero (goctl), dan Kratos — bukan untuk dipakai langsung (zero lock-in), hanya pola layout yang bisa diadopsi.

### Catatan status maintenance pola referensi (verifikasi web, 2026-06-06)

Cek dilakukan via GitHub REST API (`api.github.com/repos/...`) + pkg.go.dev. Ringkasan:

| Referensi | Versi terbaru | Tanggal rilis | Commit terakhir | Archived? | Bintang | Lisensi | Verdict |
|---|---|---|---|---|---|---|---|
| **go-zero** (`zeromicro/go-zero`) | v1.10.2 | 2026-05-31 | 2026-06-03 | Tidak | ~33.1k | MIT | **Aktif** — boleh dijadikan referensi pola |
| **Kratos** (`go-kratos/kratos`) | v2.9.2 | 2025-12-05 | 2026-06-05 | Tidak | ~25.7k | MIT | **Aktif** — boleh dijadikan referensi pola |
| **kratos-layout** (`go-kratos/kratos-layout`) | v2.9.2 | 2025-12-15 | 2026-06-01 | Tidak | ~478 | MIT | **Aktif** — referensi layout per-service |
| **go-kit** (`go-kit/kit`) | v0.13.0 | 2023-08-25 | 2024-03-13 | Tidak (tapi dorman) | ~27.4k | MIT | **Hindari sebagai default** — rilis terakhir ~3 thn, commit terakhir ~2 thn lalu |

> **Penting:** go-kit **tidak** ditandai *archived*, tetapi praktis dorman (rilis terakhir Agustus 2023, commit terakhir Maret 2024). Sesuai aturan keras Fase 0, **go-kit tidak boleh menjadi default**. Polanya (transport/endpoint/service separation) tetap berharga sebagai *pelajaran desain*, tapi `gostarter` tidak akan meng-generate proyek berbasis go-kit. go-zero dan Kratos aktif, namun keduanya juga **bukan** untuk di-import oleh hasil generate (zero lock-in) — kita hanya mengadopsi **pola layout**-nya.

Tooling proto (verifikasi web, 2026-06-06):

| Tool | Versi terbaru | Tanggal rilis | Commit terakhir | Archived? | Lisensi | Verdict |
|---|---|---|---|---|---|---|
| **buf** (`bufbuild/buf`) | v1.70.0 | 2026-05-25 | 2026-06-05 | Tidak | Apache-2.0 | **Aktif — rekomendasi default** |
| **protobuf-go** (`protocolbuffers/protobuf-go`, `protoc-gen-go`) | v1.36.11 | 2025-12-12 | 2026-01-20 | Tidak | BSD-3-Clause | **Aktif — dependency wajib (dipakai buf maupun protoc)** |
| **grpc-go** (`grpc/grpc-go`, `protoc-gen-go-grpc`) | v1.81.1 | 2026-05-14 | 2026-06-05 | Tidak | Apache-2.0 | **Aktif — dependency wajib** |

---

## 2. Kandidat Struktur (tree lengkap)

Tiga kandidat di bawah mewakili tiga titik di spektrum "kemudahan generate". Asumsi contoh: project bernama `shop` dengan 2 service awal — `order` dan `user` — di mana `order` memanggil `user` via gRPC.

### Kandidat A — Monorepo Flat (semua service sejajar di root)

Gaya yang mirip banyak contoh go-zero/looklook: tiap service adalah folder top-level, satu `go.mod` di root.

```
shop/
├── go.mod                      # satu module: github.com/acme/shop
├── go.sum
├── Makefile
├── docker-compose.yml          # menjalankan SEMUA service + DB + gateway
├── .env.example
├── .golangci.yml
├── README.md
├── proto/                      # SEMUA kontrak proto terpusat
│   ├── order/v1/order.proto
│   └── user/v1/user.proto
├── gen/                        # kode hasil generate (buf), di-commit
│   └── go/
│       ├── order/v1/           # order.pb.go, order_grpc.pb.go
│       └── user/v1/            # user.pb.go,  user_grpc.pb.go
├── buf.yaml
├── buf.gen.yaml
├── order/
│   ├── cmd/
│   │   └── main.go             # entrypoint service order
│   ├── internal/
│   │   ├── handler/            # gRPC server impl (implements order/v1)
│   │   ├── client/             # gRPC client ke user (pakai gen/go/user/v1)
│   │   ├── service/            # business wiring (contoh minimal)
│   │   └── config/
│   ├── Dockerfile
│   └── migrations/             # bila service ini punya DB
├── user/
│   ├── cmd/
│   │   └── main.go
│   ├── internal/
│   │   ├── handler/
│   │   ├── service/
│   │   └── config/
│   ├── Dockerfile
│   └── migrations/
└── pkg/                        # shared library antar service (lihat §5)
    ├── logger/
    ├── config/
    └── grpcutil/               # interceptor, dial helper, health
```

**Karakter:** paling rata, paling sedikit kedalaman folder. Mudah dibaca untuk 2–4 service, tapi root cepat penuh saat jumlah service bertambah karena service bercampur dengan file infra (`proto/`, `gen/`, `pkg/`, compose).

---

### Kandidat B — Monorepo Terstruktur (`services/` + `libs/` + `proto/` + `gateway/`) — **kandidat utama**

Pemisahan jelas antara *unit deploy* (`services/`), *kode bersama* (`libs/`), *kontrak* (`proto/` + `gen/`), dan *edge* (`gateway/`). Satu `go.mod` di root (single-module monorepo).

```
shop/
├── go.mod                          # satu module: github.com/acme/shop
├── go.sum
├── Makefile                        # target: proto, build, up, down, test, lint
├── docker-compose.yml              # orkestrasi semua service + gateway + DB
├── .env.example
├── .golangci.yml
├── README.md
├── buf.yaml                        # workspace buf
├── buf.gen.yaml                    # plugin: protoc-gen-go + protoc-gen-go-grpc
├── proto/                          # KONTRAK terpusat (single source of truth)
│   ├── order/v1/order.proto
│   └── user/v1/user.proto
├── gen/                            # output buf generate, DI-COMMIT
│   └── go/
│       ├── order/v1/
│       │   ├── order.pb.go
│       │   └── order_grpc.pb.go
│       └── user/v1/
│           ├── user.pb.go
│           └── user_grpc.pb.go
├── services/
│   ├── order/
│   │   ├── cmd/
│   │   │   └── main.go             # wiring: config → server → register handler
│   │   ├── internal/
│   │   │   ├── handler/            # impl OrderServiceServer
│   │   │   │   └── order.go
│   │   │   ├── client/             # UserServiceClient (contoh call lintas service)
│   │   │   │   └── user.go
│   │   │   ├── service/            # logika contoh minimal
│   │   │   └── config/
│   │   │       └── config.go       # baca dari ENV
│   │   ├── migrations/             # 1 contoh migration bila DB dipilih
│   │   │   └── 0001_init.sql
│   │   └── Dockerfile
│   └── user/
│       ├── cmd/
│       │   └── main.go
│       ├── internal/
│       │   ├── handler/
│       │   │   └── user.go
│       │   ├── service/
│       │   └── config/
│       │       └── config.go
│       ├── migrations/
│       │   └── 0001_init.sql
│       └── Dockerfile
├── gateway/                        # OPSIONAL — API gateway (HTTP → gRPC), 1 service
│   ├── cmd/
│   │   └── main.go                 # REST edge yang mem-proxy ke order/user
│   ├── internal/
│   │   └── router/
│   └── Dockerfile
└── libs/                           # SHARED LIBRARY antar service (internal repo)
    ├── logger/                     # wrapper log/slog
    │   └── logger.go
    ├── config/                     # loader ENV generik
    │   └── config.go
    ├── grpcclient/                 # dial + interceptor (timeout, logging)
    │   └── dial.go
    └── health/                     # health check pattern
        └── health.go
```

**Karakter:** setiap kategori file punya rumah yang jelas dan **stabil**. Saat menambah service ke-N, generator hanya menambah folder di `services/<name>/`, satu file proto di `proto/<name>/v1/`, dan satu blok di `docker-compose.yml` — **lokasi penyisipan deterministik**. Inilah yang membuat subcommand `add service <name>` (Fase 4 T4.3) bisa diandalkan.

---

### Kandidat C — Polyrepo / Per-Repo (satu service = satu repo/module)

Setiap service berdiri sendiri sebagai module independen, dengan kontrak proto dibagikan lewat repo terpisah (atau module `proto`/`contracts` yang di-publish). `gostarter` akan men-generate **beberapa direktori repo terpisah** + satu repo orkestrasi.

```
# Repo orkestrasi (deploy-time)
shop-deploy/
├── docker-compose.yml          # menjalankan semua service via image
├── .env.example
└── README.md

# Repo kontrak (di-publish, di-import oleh tiap service)
shop-contracts/
├── go.mod                      # module: github.com/acme/shop-contracts
├── buf.yaml
├── buf.gen.yaml
├── proto/
│   ├── order/v1/order.proto
│   └── user/v1/user.proto
└── gen/go/                     # di-commit & di-tag, jadi dependency
    ├── order/v1/
    └── user/v1/

# Repo service order (module independen)
shop-order/
├── go.mod                      # module: github.com/acme/shop-order
│                               #   require github.com/acme/shop-contracts vX.Y.Z
├── go.sum
├── cmd/main.go
├── internal/
│   ├── handler/
│   ├── client/                 # import gen dari shop-contracts
│   └── config/
├── migrations/
├── Dockerfile
└── Makefile

# Repo service user (module independen)
shop-user/
├── go.mod                      # module: github.com/acme/shop-user
├── go.sum
├── cmd/main.go
├── internal/
│   ├── handler/
│   └── config/
├── migrations/
├── Dockerfile
└── Makefile
```

**Karakter:** memodelkan kepemilikan tim per-service secara nyata (boundary keras = boundary repo). Tapi untuk *generator*, ini berarti menghasilkan **4+ direktori dengan 4+ `go.mod`**, kontrak yang harus di-*publish/version* sebelum service bisa `go build` (atau dipaksa pakai `replace` lokal), dan `go vet/build/test ./...` dari satu root **tidak** mencakup semua repo. Aturan keras "lolos satu perintah tanpa edit manual" jadi jauh lebih sulit dijamin.

---

## 3. Tabel Trade-off

Skala: ⭐ (buruk) → ⭐⭐⭐⭐⭐ (sangat baik), dari sudut pandang **generator + pemeliharaan template**.

| Kriteria | A — Monorepo Flat | B — Monorepo Terstruktur | C — Polyrepo |
|---|---|---|---|
| **Kesederhanaan (jumlah service kecil)** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐ |
| **Skalabilitas (banyak service / banyak tim)** | ⭐⭐ (root penuh) | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ (boundary repo nyata) |
| **Testability satu perintah** (`go test ./...` cakup semua) | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐ (per-repo, tak ter-cover dari 1 root) |
| **Kemudahan di-generate (N service sekaligus)** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐ (N repo + publish kontrak) |
| **Kemudahan extend (`add service`)** | ⭐⭐⭐ (root campur) | ⭐⭐⭐⭐⭐ (lokasi sisip deterministik) | ⭐⭐ (scaffold repo+module baru) |
| **`docker compose up` jalan sekali** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ (butuh image/replace antar repo) |
| **Sharing kode (config/logger)** | ⭐⭐⭐⭐ (`pkg/`) | ⭐⭐⭐⭐⭐ (`libs/`, jelas) | ⭐⭐ (harus jadi module ter-publish) |
| **Zero lock-in dipertahankan** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **Kesiapan migrasi ke per-repo nanti** | ⭐⭐⭐ | ⭐⭐⭐⭐ (folder service sudah self-contained) | — (sudah per-repo) |

**Bacaan utama:** Kandidat C unggul **hanya** pada dimensi yang relevan saat organisasi sudah besar (boundary tim per-repo). Pada **semua** dimensi yang menjadi aturan keras `gostarter` — generate sekali, lolos `go vet/build/test ./...` dari satu root tanpa edit manual, `docker compose up` jalan — Kandidat C justru paling lemah. Kandidat B menang di hampir semua kolom yang penting bagi generator, sambil tetap menjaga folder service cukup *self-contained* sehingga ekstraksi ke per-repo di masa depan tetap murah.

---

## 4. KEPUTUSAN: **MONOREPO** (Kandidat B — `services/` + `libs/` + `proto/` + `gen/` + `gateway/` opsional)

Template microservice `gostarter` v1 memakai **monorepo single-module** dengan layout Kandidat B.

### Alasan (terikat langsung ke aturan keras produk)

1. **Generator harus menghasilkan N service dalam satu aksi.** Dengan monorepo, "menambah service" = menambah subtree `services/<name>/`, satu `proto/<name>/v1/<name>.proto`, satu blok di `docker-compose.yml`, dan satu plugin output di `gen/`. Semua titik sisip **deterministik dan idempotent** — fondasi yang membuat subcommand `add service` (Fase 4 T4.3) bisa benar. Polyrepo menuntut generate banyak `go.mod` + skema versioning kontrak, yang jauh lebih rapuh untuk di-template.

2. **Aturan keras "`go vet ./... && go build ./... && go test ./...` lolos tanpa edit manual".** Satu module di root membuat ketiga perintah ini mencakup **semua** service sekaligus, langsung hijau setelah generate. Pada polyrepo, perintah dari satu root tidak menjangkau repo lain, dan service tak bisa di-build sebelum module kontrak di-publish/di-`replace` — melanggar "tanpa edit manual".

3. **Aturan keras "`docker compose up` jalan untuk stack ber-DB".** Monorepo punya satu `docker-compose.yml` di root yang membangun tiap `services/<name>/Dockerfile` (konteks build = root yang sama), menyalakan DB, dan menyetel env service-to-service (mis. `USER_GRPC_ADDR=user:9090`). Polyrepo butuh image yang sudah ter-publish atau orkestrasi `replace` lintas-repo — tidak bisa dijamin "jalan sekali" oleh generator.

4. **Contoh 2 service saling memanggil jadi sepele.** `order` mengimpor stub `gen/go/user/v1` (path dalam module yang sama), mem-`dial` `user` lewat `libs/grpcclient`. Tidak ada module boundary yang harus dilintasi → contoh call antar service kompilasi tanpa langkah tambahan.

5. **Zero lock-in tetap aman.** Monorepo vs polyrepo tidak memengaruhi lock-in: hasil generate hanya mengimpor `libs/` & `gen/` **milik project itu sendiri**, plus dependency publik (grpc-go, protobuf-go). **Tidak ada** import dari `gostarter`. `libs/` adalah kode yang dimiliki user, bukan package builder.

6. **Tetap siap di-pisah nanti (escape hatch).** Karena tiap `services/<name>/` self-contained (punya `cmd/`, `internal/`, `Dockerfile`, `migrations/` sendiri) dan kontrak sudah terpisah di `proto/` + `gen/`, memecah satu service ke repo sendiri di kemudian hari tinggal "angkat folder + tambah `go.mod`". Monorepo adalah default yang aman; per-repo adalah keputusan organisasi, bukan keputusan generator v1.

### Yang TIDAK dipilih dan alasan singkat

- **Kandidat A (flat):** baik untuk 2 service, tapi root cepat berantakan dan titik-sisip `add service` kurang stabil (service bercampur file infra). B = A + disiplin folder, dengan biaya kompleksitas mendekati nol.
- **Kandidat C (polyrepo):** ditolak sebagai **default v1** karena bertentangan langsung dengan tiga aturan keras (satu perintah build/test, `docker compose up` sekali, tanpa edit manual). Boleh menjadi **mode lanjutan v2** (`--layout per-repo`) bila ada permintaan.

### Keputusan turunan

- **API gateway:** di-generate sebagai service **opsional** (`gateway/`) hanya jika user memilih pola REST-edge; default-nya komunikasi gRPC langsung antar service. Gateway bukan keharusan untuk contoh 2-service.
- **Shared library:** diletakkan di `libs/` (bukan `pkg/` agar tidak menyiratkan "publik untuk di-import luar"). Isi minimal: `logger` (wrapper `log/slog`), `config` (loader ENV), `grpcclient` (dial + interceptor), `health`. Semua dimiliki project, nol dependensi ke builder.
- **Kontrak proto di `proto/`, kode hasil generate di `gen/` dan DI-COMMIT.** Meng-commit hasil generate membuat `go build ./...` hijau **tanpa** mewajibkan user menjalankan `buf generate` lebih dulu — krusial untuk aturan "lolos tanpa edit manual". Regenerasi tetap tersedia via `make proto`.

---

## 5. Rekomendasi Tooling Proto: **buf** (default), protoc sebagai fallback yang didokumentasikan

**Keputusan:** template `gostarter` memakai **buf** (`bufbuild/buf`) untuk lint, format, breaking-change detection, dan code generation, dengan `buf.gen.yaml` yang memanggil plugin resmi `protoc-gen-go` (`protocolbuffers/protobuf-go`) dan `protoc-gen-go-grpc` (`grpc/grpc-go`).

### Alasan

1. **Reproducible by config, bukan by shell script.** Semua opsi generate hidup di `buf.gen.yaml` yang di-commit, bukan tersebar di flag CLI `protoc -I ... --go_out=...`. Setiap developer dan CI menghasilkan output identik — selaras dengan Definition of Done `gostarter` ("output byte-identical"). Generator hanya perlu menulis 2 file YAML kecil, bukan baris `protoc` panjang yang rapuh terhadap path. ([Buf Docs — Generating code](https://buf.build/docs/generate/))
2. **Tidak perlu meng-install protoc.** buf membawa compiler Protobuf sendiri (ditulis dalam Go), jadi `make proto` cukup bergantung pada binary `buf` + dua plugin Go yang bisa di-`go install`. Lebih sedikit prasyarat sistem = lebih mudah dijamin "jalan di mesin user". ([Buf — new compiler](https://buf.build/blog/bufs-new-compiler))
3. **Managed mode** memindahkan opsi `go_package` keluar dari file `.proto`, menjaga `.proto` tetap netral-bahasa dan mengurangi boilerplate yang harus di-template per file. ([Buf Docs — Generate](https://buf.build/docs/generate/))
4. **Lint + breaking-change detection bawaan** memberi project hasil generate kualitas kontrak yang baik sejak menit pertama, tanpa menambah tooling lain.
5. **Status maintenance sangat sehat:** buf v1.70.0 (rilis 2026-05-25), commit terakhir 2026-06-05, tidak archived, Apache-2.0. ([GitHub — bufbuild/buf](https://github.com/bufbuild/buf))

### Mengapa bukan protoc sebagai default

`protoc` (mentah) tetap sah dan dipakai luas, tetapi untuk *generator* ia bermasalah: invokasi panjang berbasis path yang rapuh, wajib install `protoc` + tiap plugin secara manual, dan tidak ada lint/breaking-check bawaan. Ini menambah permukaan kegagalan "tidak jalan di mesin user". Plugin yang dihasilkan **sama persis** (`protoc-gen-go` & `protoc-gen-go-grpc`), jadi tidak ada lock-in ke buf pada *kode hasil generate* — pindah ke `protoc` murni hanyalah mengganti perintah di `Makefile`. Karena itu protoc didokumentasikan sebagai **fallback di README**, bukan default.

> **Catatan zero lock-in:** buf hanyalah *tool build* (seperti `make`), bukan dependency runtime. Kode `.pb.go` hasil generate hanya bergantung pada `google.golang.org/protobuf` dan `google.golang.org/grpc` — keduanya library resmi & aktif. Tidak ada import `bufbuild/*` di kode hasil generate. Aturan zero lock-in (terhadap *builder*) maupun "tanpa lock-in ke tool proto" sama-sama terjaga.

### Plugin & dependency runtime yang masuk ke hasil generate

| Komponen | Module | Versi terverifikasi (2026-06-06) | Peran |
|---|---|---|---|
| `protoc-gen-go` | `google.golang.org/protobuf` (`protocolbuffers/protobuf-go`) | v1.36.11 (2025-12-12) | Generate `*.pb.go` (message) |
| `protoc-gen-go-grpc` | `google.golang.org/grpc` (`grpc/grpc-go`) | v1.81.1 (2026-05-14) | Generate `*_grpc.pb.go` (server/client stub) |
| buf CLI | `github.com/bufbuild/buf` | v1.70.0 (2026-05-25) | Build-time only (lint/format/generate) |

---

## 6. Pelajaran Desain dari Pola Referensi (bukan untuk di-import — zero lock-in)

| Toolkit | Pelajaran yang diadopsi | Yang TIDAK diadopsi / catatan |
|---|---|---|
| **Kratos** (`kratos-layout`) | Layout per-service `api/` (proto) + `cmd/` + `internal/{server,service,...}` sangat bersih; pemisahan kontrak (`api/`) dari implementasi (`internal/`). Target Makefile untuk regen proto. | Tidak meng-import `go-kratos/kratos`; tidak memakai `wire` sebagai keharusan; `internal/{biz,data,conf}` DDD-nya disederhanakan agar contoh minimal (Non-Goal: bukan business logic). |
| **go-zero** (`goctl`) | Pola "API/gateway sebagai aggregator + service RPC di belakang" memvalidasi keputusan gateway opsional kita; goctl membuktikan nilai *codegen dari kontrak*. | Tidak memakai `.api` DSL milik go-zero (lock-in ke ekosistemnya); kita pakai `.proto` standar + buf agar netral. |
| **go-kit** | Pemisahan konseptual transport ↔ endpoint ↔ service adalah pelajaran arsitektur yang bagus untuk testability. | **Hindari sebagai default** — dorman (rilis terakhir v0.13.0 / Agustus 2023, commit terakhir Maret 2024). Tidak di-generate; hanya inspirasi pemisahan layer. |

---

## 7. Open Questions (untuk user)

1. **Gateway default on/off?** Apakah contoh 2-service default cukup gRPC-to-gRPC (tanpa gateway), dengan `gateway/` hanya muncul bila user memilih add-on REST-edge? (Rekomendasi riset: ya, gateway opsional.)
2. **Commit hasil `gen/`?** Riset merekomendasikan meng-commit `gen/go/` agar `go build` hijau tanpa `buf generate`. Setuju, atau prefer `.gitignore` + wajib `make proto` saat pertama kali (mengorbankan aturan "tanpa edit/langkah manual")?
3. **Mode `--layout per-repo` di v2?** Apakah polyrepo perlu masuk roadmap sebagai mode lanjutan, atau monorepo cukup untuk selamanya?
4. **Komunikasi default antar service:** gRPC saja untuk v1, atau perlu opsi event-driven (NATS/Kafka) sejak awal? (Broker akan dibahas tuntas di R0.3 — library matrix.)

---

## 8. Daftar Sumber (URL)

**Pola referensi & layout**
- go-zero (repo): https://github.com/zeromicro/go-zero
- go-zero (pkg.go.dev): https://pkg.go.dev/github.com/zeromicro/go-zero
- go-zero (rilis): https://github.com/zeromicro/go-zero/releases
- Kratos (repo): https://github.com/go-kratos/kratos
- Kratos (pkg.go.dev): https://pkg.go.dev/github.com/go-kratos/kratos/v2
- kratos-layout (template): https://github.com/go-kratos/kratos-layout
- kratos-layout README (struktur): https://github.com/go-kratos/kratos-layout/blob/main/README.md
- go-kit (repo): https://github.com/go-kit/kit
- go-kit (pkg.go.dev): https://pkg.go.dev/github.com/go-kit/kit

**Tooling proto**
- buf (repo): https://github.com/bufbuild/buf
- buf — Generating code (docs): https://buf.build/docs/generate/
- buf — usage guide: https://buf.build/docs/generate/usage/
- buf — new compiler: https://buf.build/blog/bufs-new-compiler
- protoc-gen-go (repo): https://github.com/protocolbuffers/protobuf-go
- protobuf-go (pkg.go.dev): https://pkg.go.dev/google.golang.org/protobuf
- grpc-go / protoc-gen-go-grpc (repo): https://github.com/grpc/grpc-go
- grpc-go (pkg.go.dev): https://pkg.go.dev/google.golang.org/grpc

> Status maintenance (archived/commit/release terakhir) diverifikasi via GitHub REST API (`https://api.github.com/repos/<owner>/<repo>`) pada 2026-06-06.
