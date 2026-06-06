# 03 — Pemilihan Library per Concern

> **Status:** Sumber kebenaran (source of truth) untuk pemilihan library `gostarter` v1.
> **Tanggal verifikasi:** 2026-06-06
> **Konteks produk:** CLI Go yang men-generate struktur project Go best-practice (3 mode: monolith sederhana, modular monolith, microservice). Aturan keras hasil generate: lolos `go vet ./... && go build ./... && go test ./...` **tanpa edit manual**, `docker compose up` jalan untuk stack ber-DB, dan project hasil generate **TIDAK BOLEH meng-import package apa pun dari builder** (zero lock-in).

---

## 1. Pendahuluan

Dokumen ini adalah **sumber kebenaran tunggal** untuk keputusan pemilihan library di `gostarter`, dipecah per *concern* (HTTP, config, logging, database, validasi, DI, messaging, observability, auth, testing). Untuk tiap concern ditetapkan satu **default** (yang di-generate jika user tidak memilih) plus **alternatif** yang ditawarkan di prompt interaktif.

### Kriteria penilaian

Setiap kandidat dinilai terhadap kriteria berikut:

1. **Status maintenance** — versi & tanggal rilis terakhir, ada/tidaknya badge `archived`/`deprecated`. Diverifikasi via pkg.go.dev + halaman GitHub repo (bukan dari ingatan).
2. **Popularitas** — jumlah stars / adopsi ekosistem sebagai proksi maturity & ketersediaan contoh.
3. **Kualitas dokumentasi** — kelengkapan docs resmi & contoh.
4. **Lisensi** — diutamakan permisif (MIT/BSD/Apache-2.0); copyleft file-level (MPL-2.0) dicatat; lisensi ber-EULA dicatat sebagai peringatan.
5. **Stabilitas API** — major version matang, frekuensi breaking change.
6. **Beban dependency** — jumlah transitive deps, kebutuhan CGO, dampak ke build size & supply chain.

### Prinsip pemandu khusus generator

- **Zero lock-in by construction** — default diutamakan pada stdlib / pola bahasa murni / library berfootprint minimal agar project hasil generate tidak terikat ke builder maupun framework berat.
- **Pure-Go / zero-CGO** — default driver & klien diutamakan pure-Go agar `go build ./...` dan `docker compose up` jalan tanpa toolchain C, cross-compile mulus, dan Docker image bisa pakai base minimal.
- **Codegen harus aman terhadap aturan "tanpa edit manual"** — library berbasis code-generation (sqlc, ent, mock generator) hanya boleh default jika output ter-generate ikut di-commit / di-generate di langkah post-gen sebelum build.

---

## 2. Tabel Ringkas Master

| concern | default | alternatif | status default | alasan singkat |
|---|---|---|---|---|
| HTTP Router / Framework | stdlib `net/http` (routing Go 1.22+) | chi, gin, echo, fiber | stdlib (GA sejak Go 1.22) | Zero dependency mutlak, `http.Handler` murni → zero lock-in & kompatibel semua middleware `net/http`. |
| Config | `joho/godotenv` (atau env murni) | koanf, viper | v1.5.1 (5 Feb 2023), feature-complete, MIT | Paling ringan & idiomatik; hanya inject `.env`, sisanya stdlib; zero transitive deps. |
| Logging | stdlib `log/slog` | zerolog, zap | stdlib (sejak Go 1.21) | Zero dependency, `slog.Handler` interface standar → zero lock-in, idiomatik. |
| DB Access (lapisan query) | `jmoiron/sqlx` | sqlc, GORM, ent | v1.4.0 (23 Apr 2024), MIT | Extension tipis di atas `database/sql`, zero magic, API beku & stabil. |
| DB Driver — Postgres | `jackc/pgx/v5` | — | v5.10.0 (3 Jun 2026), MIT | Driver Postgres de-facto, pure-Go, `pgxpool` built-in, sangat aktif. |
| DB Driver — MySQL | `go-sql-driver/mysql` | — | v1.9.x (2025), MPL-2.0 | Driver MySQL resmi de-facto, pure-Go (tanpa CGO). |
| DB Driver — SQLite | `modernc.org/sqlite` | mattn/go-sqlite3 (CGO) | v1.39.x (28 Mei 2026), BSD-3 | **Pure-Go CGO-FREE** → `CGO_ENABLED=0`, cross-compile & Docker minimal. |
| DB Driver — MongoDB | `mongo-go-driver/v2` | — | v2.6.0 (27 Apr 2026), Apache-2.0 | Driver resmi; wajib path v2 (v1 deprecated). |
| Migration | `golang-migrate/migrate/v4` | goose, atlas | v4.19.1 (29 Nov 2025), MIT | File SQL `up/down` polos → zero lock-in, multi-DB, API beku. |
| Validation | `go-playground/validator/v10` | — | v10.30.3 (29 Mei 2026), MIT | Standar de-facto validasi struct, aktif, struct-tag, zero codegen. |
| DI / Wiring | Manual constructor injection | uber-go/fx | N/A (pola bahasa Go murni) | Nol dependency → zero lock-in absolut, langsung build, transparan. |
| Messaging — NATS | `nats-io/nats.go` | — | v1.52.0 (7 Mei 2026), Apache-2.0 | Klien resmi, pure-Go, broker mudah `docker compose up`. |
| Messaging — Kafka | `twmb/franz-go` | segmentio/kafka-go, IBM/sarama | v1.21.2 (15 Mei 2026), BSD-3 | Feature-complete, pure-Go, API modern, tren adopsi naik. |
| Messaging — RabbitMQ | `rabbitmq/amqp091-go` | — | v1.11.0 (21 Apr 2026), BSD-2 | Klien AMQP resmi RabbitMQ (penerus streadway/amqp). |
| Observability — Tracing/Metrics | `go.opentelemetry.io/otel` | — | v1.44.0 (27 Mei 2026), Apache-2.0 | Standar vendor-neutral; Traces & Metrics GA; OTLP → zero lock-in. |
| Observability — Metrics endpoint | `prometheus/client_golang` | — | v1.23.2 (5 Sep 2025), Apache-2.0 | Klien metrics de-facto, `/metrics` handler standar, stabil. |
| Observability — Health check | health buatan sendiri (`net/http`) | alexliesenfeld/health | stdlib | `/healthz` & `/readyz` cukup stdlib → zero dependency. |
| Auth (token) | `golang-jwt/jwt/v5` | aidantwoods/go-paseto | v5.3.1 (28 Jan 2026), MIT | JWT de-facto, hanya stdlib crypto → zero external deps. |
| Testing — assertion | `stretchr/testify` | — | v1.11.1 (27 Agu 2025), MIT | Assertion + suite paling banyak dipakai, gold-standard. |
| Testing — mock gen | `uber-go/mock` | vektra/mockery | v0.5.2 (28 Apr 2025), Apache-2.0 | Fork resmi terawat dari golang/mock (archived); codegen → file biasa. |
| Testing — integration | `testcontainers/testcontainers-go` | — | v0.42.0 (9 Apr 2026), MIT | Standar integration test container; **wajib build-tag `integration`**. |
| Testing — BDD (opsional) | — | onsi/ginkgo + gomega | — | Opt-in; default cukup `testing` + testify (lebih ringan & netral). |

---

## 3. Detail per Concern

### 3.1 HTTP Router / Framework

| concern | kandidat | status maintenance (versi & tanggal rilis terakhir) | rekomendasi (default / alternatif / hindari) | alasan |
|---|---|---|---|---|
| HTTP Router / Framework | **stdlib `net/http`** (routing enhancements Go 1.22+) | Bagian dari Go stdlib; routing method-based + path wildcards `{id}`/`{path...}` GA sejak Go 1.22 (Feb 2024), stabil dan terus dipelihara di rilis Go terbaru (Go 1.24/1.25, 2025–2026). Tidak ada versi terpisah, tidak archived. | **Default** | Zero dependency mutlak — `http.ServeMux` adalah `http.Handler`, jadi project hasil generate tidak meng-import apa pun (zero lock-in by construction). API distandarkan oleh tim Go, kompatibel penuh dengan seluruh ekosistem middleware `net/http` (otel `otelhttp`, health check, `gzip`, dll). Langsung `go build`/`go vet`/`go test` tanpa modul pihak ketiga. |
| HTTP Router / Framework | **chi** (`go-chi/chi/v5`) | v5.3.0 — 22 Mei 2026. Aktif, tidak archived/deprecated. ~22.3k stars. Lisensi MIT. | **Alternatif** (rekomendasi kuat #1) | Dibangun 100% di atas `net/http` standar (`http.Handler`/`http.HandlerFunc`) → kompatibel penuh dengan middleware ekosistem `net/http` (otel, dll). API sangat stabil (v5 sejak 2020, nyaris tanpa breaking change). Menambah route grouping, `URLParam`, middleware stack yang lebih ergonomis. Dependency footprint nyaris nol (hanya stdlib). Cocok untuk template yang butuh sedikit lebih dari stdlib tanpa mengorbankan kompatibilitas. |
| HTTP Router / Framework | **echo** (`labstack/echo`) | v5 line: v5.1.1 — 1 Mei 2026; v4 line: v4.15.2 — 1 Mei 2026 (v4 LTS, security/bug fix s/d 2026-12-31). Aktif, tidak archived. ~32.4k stars. Lisensi MIT. | **Alternatif** | Full framework di atas `net/http` (bukan fasthttp) → masih kompatibel dengan handler/middleware `net/http` via adaptor. Fitur lengkap (binding, validation hooks, middleware bawaan). Catatan: pakai abstraksi `echo.Context` sendiri → menambah surface API yang membuat project lebih terikat ke framework (lock-in moderat). v5 baru rilis awal 2026 (breaking changes); untuk default generator, v4 LTS lebih konservatif bila dipilih. |
| HTTP Router / Framework | **gin** (`gin-gonic/gin`) | v1.12.0 — 28 Feb 2026. Aktif, tidak archived/deprecated. ~88.6k stars (terpopuler). Lisensi MIT. | **Alternatif** | Framework paling populer, di atas `net/http` (kompatibel adaptor middleware `net/http`). Ekosistem & contoh sangat banyak → familiar bagi banyak developer. Catatan: `gin.Context` adalah abstraksi sendiri (lock-in moderat), dan masih di major v1 (>10 tahun, API matang & stabil). Kandidat alternatif "battle-tested" yang aman ditawarkan. |
| HTTP Router / Framework | **fiber** (`gofiber/fiber/v3`) | v3.3.0 — 22 Mei 2026 (v3 GA Feb 2026, butuh Go 1.25+). Aktif, tidak archived. ~39.8k stars. Lisensi MIT. | **Alternatif (dengan peringatan)** | Performa tinggi, API ala Express. **Tetapi** dibangun di atas `valyala/fasthttp`, **bukan** `net/http` → TIDAK kompatibel dengan ekosistem middleware/`http.Handler` standar. Otel, health, dan add-on harus pakai varian khusus Fiber (`otelfiber`), bukan `otelhttp` standar. Ini bertentangan dengan tujuan zero lock-in & interoperabilitas template add-on. Tawarkan hanya untuk user yang sadar memilih jalur fasthttp. |

- **Default:** **stdlib `net/http`** (routing enhancements Go 1.22+) — sejak Go 1.22 `http.ServeMux` sudah mendukung method-based routing (`GET /users/{id}`) dan path wildcards, sehingga kebutuhan routing dasar v1 tercukupi tanpa dependency apa pun. Ini memenuhi aturan keras "zero lock-in" secara struktural (project hasil generate tidak meng-import package eksternal), langsung lolos `go vet ./... && go build ./... && go test ./...`, dan handler-nya `http.Handler` murni sehingga semua add-on template (otel `otelhttp`, health check, middleware logging/recovery) bisa di-wire tanpa adaptor.

- **Alternatif ditawarkan di prompt interaktif:** `chi` (paling direkomendasikan sebagai upgrade ringan dari stdlib — tetap `net/http`-compatible, API stabil), `gin` (paling populer, ekosistem terbesar), `echo` (framework lengkap, pilih v4 LTS untuk konservatif atau v5 untuk yang terbaru), `fiber` (high-performance fasthttp — dengan peringatan kompatibilitas).

- **Catatan/implikasi untuk generator:**
  - **Pemisahan dua kelas kandidat.** `net/http`, `chi`, `echo`, dan `gin` semuanya berbasis `net/http`. Hanya `fiber` yang berbasis `fasthttp`. Generator sebaiknya memperlakukan ini sebagai *flag arsitektural*, bukan sekadar pilihan rasa.
  - **Dampak `fiber`/fasthttp ke template add-ons.** Karena fasthttp tidak kompatibel dengan middleware/`http.Handler` standar `net/http`, semua add-on yang men-generate template harus punya jalur khusus Fiber: observability harus pakai `otelfiber` (bukan `otelhttp` standar), health/metrics/middleware logging harus pakai signature `fiber.Ctx` (bukan `http.Handler`). Artinya jika `fiber` dipilih, generator perlu cabang template terpisah untuk otel, health, recovery, dan middleware lain — menambah beban maintenance builder dan memecah keseragaman wiring.
  - **Konsistensi add-on jika default/chi/gin/echo dipilih.** Untuk keempat kandidat berbasis `net/http`, satu set template add-on yang sama (mis. `otelhttp` untuk tracing, `http.Handler` health endpoint) bisa dipakai ulang lintas pilihan router dengan adaptor minimal — ini menyederhanakan matrix template generator secara signifikan.
  - **Lock-in vs ergonomi.** `net/http` dan `chi` paling selaras dengan aturan "project tidak meng-import package dari builder & minim lock-in" karena handler tetap `http.Handler` standar. `gin`/`echo`/`fiber` memperkenalkan tipe `Context` framework di signature handler contoh → bukan lock-in ke builder (tetap legal), tapi mengikat business logic user ke framework tersebut; layak disebut di prompt interaktif sebagai trade-off.

**Sumber:**
- stdlib net/http routing Go 1.22+: https://go.dev/blog/routing-enhancements
- chi (pkg.go.dev): https://pkg.go.dev/github.com/go-chi/chi/v5 — GitHub: https://github.com/go-chi/chi
- echo (pkg.go.dev): https://pkg.go.dev/github.com/labstack/echo/v4 — GitHub releases: https://github.com/labstack/echo/releases
- gin (pkg.go.dev): https://pkg.go.dev/github.com/gin-gonic/gin — GitHub: https://github.com/gin-gonic/gin
- fiber (pkg.go.dev): https://pkg.go.dev/github.com/gofiber/fiber/v3 — GitHub: https://github.com/gofiber/fiber
- fasthttp/Fiber inkompatibilitas dengan middleware net/http standar (otel): https://last9.io/blog/instrumenting-fasthttp-with-opentelemetry/ — https://docs.gofiber.io/contrib/otelfiber/

---

### 3.2 Config & Logging

| concern | kandidat | status maintenance (versi & tanggal rilis terakhir) | rekomendasi (default / alternatif / hindari) | alasan |
|---|---|---|---|---|
| Config | **env murni / `joho/godotenv`** | Aktif, feature-complete. Rilis terakhir **v1.5.1 — 5 Feb 2023**. Tidak archived. Maintainer menyatakan library "feature complete" (tidak menerima fitur baru, hanya bugfix). ~8.4k stars, MIT. ([pkg.go.dev](https://pkg.go.dev/github.com/joho/godotenv), [GitHub](https://github.com/joho/godotenv)) | **DEFAULT** | Paling ringan & idiomatik: hanya memuat `.env` ke env var, sisanya pakai `os.Getenv`/parsing stdlib. Zero transitive deps berarti `go build` cepat & supply-chain minimal. "Feature-complete" di sini bukan abandonware — justru ideal untuk concern yang stabil. Cocok untuk semua 3 mode arsitektur. |
| Config | **`knadh/koanf`** (v2) | Sangat aktif. Rilis terakhir **v2.3.5 — 30 Mei 2026**. Tidak archived. ~4.1k stars, MIT. Core ringan; provider/parser eksternal di-install terpisah (~11 imports core). ([pkg.go.dev](https://pkg.go.dev/github.com/knadh/koanf/v2), [GitHub](https://github.com/knadh/koanf)) | **ALTERNATIF** | Alternatif "viper tanpa bloat": arsitektur core + provider modular, jauh lebih sedikit dependency daripada viper, dukung env/JSON/TOML/YAML/file/S3. Pas saat user butuh multi-source/multi-format config tapi tetap mau build ramping. |
| Config | **`spf13/viper`** | Aktif. Rilis terakhir **v1.21.0 — 8 Sep 2025**. Tidak archived. ~29k stars, MIT. ([pkg.go.dev](https://pkg.go.dev/github.com/spf13/viper), [GitHub](https://github.com/spf13/viper)) | **ALTERNATIF (catatan: dep berat)** | De-facto standar & paling populer (integrasi mulus dengan `spf13/cobra`). Tapi menarik banyak transitive dependency dan memperbesar build size — README koanf sendiri menjadikannya pembanding "bloat". Tawarkan sebagai opsi familiar, bukan default ramping. |
| Logging | **stdlib `log/slog`** | Bagian standard library Go (sejak Go 1.21, Aug 2023). Mengikuti Go compatibility promise — tanpa version churn / risiko maintainer hilang. ([pkg.go.dev](https://pkg.go.dev/log/slog)) | **DEFAULT** | Zero dependency, idiomatik, JSON/text handler + levels + context bawaan. Karena `slog.Handler` adalah interface standar, library lain bisa emit log terstruktur tanpa memaksakan dependency logger — paling aman untuk hasil generate yang harus zero lock-in & langsung `go build`. ([Dash0](https://www.dash0.com/guides/golang-logging-libraries)) |
| Logging | **`rs/zerolog`** | Sangat aktif. Rilis terakhir **v1.35.1 — 20 Apr 2026**. Tidak archived. ~12.4k stars, MIT. ([pkg.go.dev](https://pkg.go.dev/github.com/rs/zerolog), [GitHub](https://github.com/rs/zerolog)) | **ALTERNATIF** | Zero-allocation JSON logger dengan fluent API; sangat ringan (dependency footprint kecil). Pilih saat butuh throughput tinggi pada hot path tapi tetap ingin build ramping. |
| Logging | **`uber-go/zap`** | Sangat aktif. Rilis terakhir **v1.28.0 — 28 Apr 2026** (sebelumnya v1.27.1 — 19 Nov 2025). Tidak archived. ~24.4k stars, MIT. ([pkg.go.dev](https://pkg.go.dev/go.uber.org/zap), [GitHub](https://github.com/uber-go/zap)) | **ALTERNATIF** | Logger berkinerja tinggi battle-tested skala Uber; dua API (typed `Logger` zero-alloc + `SugaredLogger`). Pilih untuk microservice high-throughput. Sedikit lebih banyak dependency daripada zerolog/slog. ([Medium 2025 guide](https://medium.com/@mamidipaka2003/mastering-production-grade-logging-in-go-golang-the-complete-2025-guide-to-uber-zap-94622c874f1b)) |

- **Default:** **Config = `joho/godotenv`** (atau env murni) + **Logging = stdlib `log/slog`** — keduanya paling ringan & idiomatik. godotenv hanya inject `.env`, sisanya stdlib, sehingga hasil generate punya transitive deps minimal dan `go vet ./... && go build ./... && go test ./...` langsung lolos tanpa edit manual. `slog` adalah standard library, jadi nol risiko supply-chain dan nol lock-in — selaras penuh dengan aturan keras "project hasil generate TIDAK BOLEH meng-import package dari builder, dan harus langsung build".
- **Alternatif ditawarkan di prompt interaktif:** Config → `knadh/koanf` (multi-source ramping) atau `spf13/viper` (populer, deps berat — terutama bila user juga pakai cobra); Logging → `rs/zerolog` (zero-alloc, ringan) atau `uber-go/zap` (high-throughput, Uber-scale).
- **Catatan/implikasi untuk generator:**
  - **Default ke yang paling ringan & idiomatik:** `godotenv`/env + `slog` adalah kombinasi dengan transitive-dependency paling sedikit → build tercepat, surface supply-chain terkecil, dan zero lock-in by construction (slog = stdlib).
  - **Kapan butuh zerolog/zap?** Hanya saat ada kebutuhan kinerja logging nyata pada hot path / high-throughput (mis. microservice dengan volume log tinggi) di mana selisih alokasi memori signifikan. Untuk ~95% project, `slog` sudah satu liga performanya dan menang di kesederhanaan + nol dependency. Jika mode **microservice** dipilih dan user menandai prioritas performa, tawarkan zerolog (footprint lebih kecil) atau zap (ekosistem lebih luas) sebagai upgrade opt-in.
  - **Pola wiring yang disarankan:** generate logger di belakang `*slog.Logger` (atau abstraksi `slog.Handler`) sebagai default, sehingga bila user nanti pasang zap/zerolog cukup ganti handler tanpa menyentuh call-site — menjaga zero lock-in.
  - **Untuk config:** semua kandidat MIT-licensed, tidak ada yang archived/deprecated per 2026-06-06. Hindari menjadikan `viper` sebagai default karena beban dependency-nya berat; cukup tawarkan sebagai opsi (relevan bila user juga generate CLI dengan cobra).

**Sumber:**
- Viper: https://pkg.go.dev/github.com/spf13/viper — https://github.com/spf13/viper
- Koanf: https://pkg.go.dev/github.com/knadh/koanf/v2 — https://github.com/knadh/koanf
- godotenv: https://pkg.go.dev/github.com/joho/godotenv — https://github.com/joho/godotenv
- log/slog: https://pkg.go.dev/log/slog
- zerolog: https://pkg.go.dev/github.com/rs/zerolog — https://github.com/rs/zerolog
- zap: https://pkg.go.dev/go.uber.org/zap — https://github.com/uber-go/zap
- Panduan pemilihan: https://www.dash0.com/guides/golang-logging-libraries — https://www.dash0.com/faq/best-go-logging-tools-in-2025-a-comprehensive-guide

---

### 3.3 Database Access, Drivers & Migration

| concern | kandidat | status maintenance (versi & tanggal rilis terakhir) | rekomendasi | alasan |
|---|---|---|---|---|
| Access | **jackc/pgx/v5** | v5.10.0 — 3 Jun 2026; aktif (MIT, ~8.600 importer) | **DEFAULT (Postgres)** | Driver Postgres de-facto di ekosistem Go: pure-Go (tanpa CGO), `pgxpool` built-in, dukung `database/sql` maupun native API. Pemeliharaan sangat aktif (rilis < 1 minggu lalu), API v5 stabil, dependency ringkas (20 import). Tidak memaksa pola apa pun ke project hasil generate. |
| Access | **jmoiron/sqlx** | v1.4.0 — 23 Apr 2024; pemeliharaan minim (MIT, ~17,6k stars) | **DEFAULT (lapisan query)** | Extension tipis di atas `database/sql` (`StructScan`, `Get`, `Select`, named params). Zero magic, idiomatik, mudah dipahami pemula — cocok untuk template "minimal wiring". Catatan: tempo rilis lambat (commit master >1 thn idle), tapi **tidak archived/deprecated**, API beku & stabil, masih jadi standar de-facto. Risiko rendah karena permukaannya tipis. |
| Access | **sqlc** (sqlc-dev/sqlc) | v1.31.1 — 22 Apr 2026; aktif (MIT, ~17,8k stars) | **ALTERNATIF (opt-in)** | Code-generator type-safe dari SQL mentah → tidak ada runtime lock-in (output hanya pakai `database/sql`/`pgx`). Sangat solid, tapi menambah **langkah generate** (`sqlc generate`) + file `sqlc.yaml` ke alur. Ditawarkan sebagai mode, bukan default, karena menambah beban toolchain. |
| Access | **GORM** (go-gorm/gorm) | v1.31.1 — 2 Nov 2025; aktif (MIT, ~39,8k stars) | **ALTERNATIF (opt-in)** | ORM full-featured paling populer (associations, hooks, auto-migrate). Cocok untuk tim yang mau ORM, tetapi opinionated & dependency lebih berat — tidak ideal sebagai default "minimal/zero-magic". Project hasil generate tetap zero lock-in ke builder (GORM hanya dependency normal). |
| Access | **ent** (ent/ent) | v0.14.6 — 23 Mar 2026; aktif (Apache-2.0, ~17,1k stars) | **ALTERNATIF (opt-in)** | Entity framework graph-based + code-gen dari schema Go. Sangat type-safe, tapi paradigma & langkah `go generate` cukup berat untuk starter minimal. Ditawarkan untuk yang butuh model relasi kompleks. |
| Driver | **go-sql-driver/mysql** | v1.9.3/v1.9.x — 2025 (terbaru); aktif (MPL-2.0, driver MySQL standar Go) | **DEFAULT (MySQL)** | Driver MySQL `database/sql` resmi de-facto, pure-Go (tanpa CGO), kompatibel MySQL/MariaDB/TiDB, support MySQL 9 VECTOR & kompresi zlib. Stabil, dependency minimal. Lisensi MPL-2.0 (file-level copyleft) → tetap aman dipakai sebagai dependency. |
| Driver | **modernc.org/sqlite** | v1.39.x — 28 Mei 2026 (port SQLite 3.51.2); aktif (BSD-3-Clause) | **DEFAULT (SQLite)** | **Pure-Go, CGO-FREE.** Krusial untuk aturan keras generator: `go build ./...` jalan tanpa toolchain C, cross-compile mulus, dan Docker image bisa pakai base minimal (`scratch`/`alpine` tanpa gcc/musl-dev). Aktif dipelihara, banyak GOOS/GOARCH didukung. |
| Driver | **mattn/go-sqlite3** | v1.14.x — masih aktif (MIT, populer) | **HINDARI sebagai default (boleh opt-in)** | Driver SQLite paling matang, **tetapi butuh CGO** → memaksa `CGO_ENABLED=1` + C toolchain di build & Docker, merusak janji "langsung build tanpa edit" dan menyulitkan image kecil/cross-compile. Bagus untuk kasus yang perlu ekstensi C SQLite, tapi bukan default. |
| Driver | **mongo-go-driver/v2** (mongodb/mongo-go-driver) | v2.6.0 — 27 Apr 2026; aktif (Apache-2.0, module `go.mongodb.org/mongo-driver/v2`) | **DEFAULT (MongoDB)** | Driver MongoDB **resmi**. Wajib pakai path **v2** (`go.mongodb.org/mongo-driver/v2`) — v1 sudah diberi deprecation notice. Pure-Go, stabil, dukung transaksi/change streams. Satu-satunya pilihan waras untuk Mongo. |
| Migration | **golang-migrate/migrate/v4** | v4.19.1 — 29 Nov 2025; aktif (MIT, ~18,5k stars) | **DEFAULT** | Standar de-facto migrasi Go: file SQL `up/down` polos, CLI + library, multi-DB (Postgres/MySQL/SQLite/Mongo dll). API v3/v4 beku & stabil. File migrasi murni SQL → **zero lock-in**; project hasil generate cukup punya folder `migrations/` + CLI, tanpa import builder. |
| Migration | **pressly/goose** | v3.27.1 — 24 Apr 2026; aktif (MIT) | **ALTERNATIF** | Migrasi via SQL maupun fungsi Go, embeddable, pemeliharaan sangat aktif. Sedikit lebih "Go-native" (bisa `embed.FS`). Ditawarkan sebagai alternatif populer bagi yang ingin migrasi ter-embed di binary. |
| Migration | **ariga/atlas** | v1.2.0 — 10 Apr 2026; aktif (CE Apache-2.0; binary default ber-EULA) | **ALTERNATIF (advanced)** | Declarative/schema-as-code (mirip Terraform untuk DB) + diffing otomatis. Sangat kuat, tapi paradigma deklaratif & **dualitas lisensi** (Community Edition Apache-2.0 vs binary default di bawah Atlas EULA) bikin kurang pas sebagai default starter sederhana. Pakai CE bila dipilih. |

- **Default:** **pgx/v5 (Postgres) + go-sql-driver/mysql (MySQL) + modernc.org/sqlite (SQLite) + mongo-go-driver/v2 (Mongo)** untuk driver; **sqlx** sebagai lapisan query di atas `database/sql`; **golang-migrate/v4** untuk migrasi. Alasan: semua **pure-Go (zero CGO)** sehingga `go vet/build/test ./...` dan `docker compose up` jalan tanpa toolchain C maupun edit manual; semuanya dependency normal yang **tidak menarik package builder** (zero lock-in); migrasi berupa file SQL polos + CLI sehingga project hasil generate tetap mandiri.
- **Alternatif ditawarkan di prompt interaktif:** Access → `sqlc` (type-safe codegen), `GORM` (ORM), `ent` (entity framework). Migration → `goose`, `atlas`. Driver SQLite → `mattn/go-sqlite3` (hanya jika user sadar konsekuensi CGO).
- **Catatan/implikasi untuk generator:**
  - **sqlc adalah code-generator (butuh langkah generate).** Bila mode sqlc dipilih, template harus menyertakan `sqlc.yaml` + folder `query/` & `schema/`, lalu **langkah post-gen menjalankan `sqlc generate`** sebelum `go build` agar package `db` ter-generate ada (kalau tidak, build gagal). Idem untuk **ent** (`go generate ./ent`). Pastikan output codegen ikut di-commit/di-generate agar aturan "lolos build tanpa edit manual" tetap terpenuhi; pertimbangkan pin versi tool via `tools.go`/`go run` agar reproducible.
  - **modernc.org/sqlite itu pure-Go (tanpa CGO)** → ini sebabnya ia jadi **default SQLite**: `CGO_ENABLED=0` aman, cross-compile & Docker multi-stage dengan base minimal (`scratch`/distroless) mulus, dan tidak perlu gcc/musl-dev di image. Sebaliknya `mattn/go-sqlite3` memaksa `CGO_ENABLED=1` + C toolchain, melanggar janji "langsung `go build` & `docker compose up` tanpa edit" → karenanya **bukan default**.
  - **mongo-go-driver wajib path v2** (`go.mongodb.org/mongo-driver/v2`); v1 sudah deprecated — template harus pakai v2 agar tidak menghasilkan import usang.
  - **go-sql-driver/mysql** berlisensi MPL-2.0 dan **atlas binary default** ber-EULA (gunakan Community Edition Apache-2.0) — catat di dokumentasi lisensi project hasil generate bila relevan.

**Sumber:**
- pgx: https://pkg.go.dev/github.com/jackc/pgx/v5 · https://github.com/jackc/pgx/releases
- sqlx: https://github.com/jmoiron/sqlx · https://github.com/jmoiron/sqlx/releases
- sqlc: https://github.com/sqlc-dev/sqlc · https://github.com/sqlc-dev/sqlc/releases
- GORM: https://github.com/go-gorm/gorm · https://github.com/go-gorm/gorm/releases
- ent: https://github.com/ent/ent · https://pkg.go.dev/entgo.io/ent
- go-sql-driver/mysql: https://github.com/go-sql-driver/mysql · https://github.com/go-sql-driver/mysql/releases
- mattn/go-sqlite3: https://github.com/mattn/go-sqlite3 · https://pkg.go.dev/github.com/mattn/go-sqlite3
- modernc.org/sqlite: https://pkg.go.dev/modernc.org/sqlite
- mongo-go-driver: https://github.com/mongodb/mongo-go-driver/releases · https://pkg.go.dev/go.mongodb.org/mongo-driver/v2/mongo
- golang-migrate: https://github.com/golang-migrate/migrate · https://github.com/golang-migrate/migrate/releases
- goose: https://github.com/pressly/goose/releases · https://pkg.go.dev/github.com/pressly/goose/v3
- atlas: https://github.com/ariga/atlas/releases · https://atlasgo.io/community-edition

---

### 3.4 Validation & Dependency Wiring

| concern | kandidat | status maintenance (versi & tanggal rilis terakhir) | rekomendasi (default / alternatif / hindari) | alasan |
|---|---|---|---|---|
| Validation | `go-playground/validator/v10` | **v10.30.3** — 29 Mei 2026; aktif (1.188+ commits, ~20k stars, MIT, **tidak** archived/deprecated) | **Default** | Standar de-facto validasi struct di Go, masih dirilis aktif (rilis terbaru < 2 minggu lalu per hari ini). Berbasis struct-tag, zero codegen, satu import langsung jalan. Cocok untuk semua mode arsitektur (monolith/modular/microservice). |
| DI/wiring | **Manual constructor injection** | N/A (pola bahasa Go murni, bukan library — tanpa dependency eksternal) | **Default** | Tidak ada dependency apa pun → menjaga jaminan *zero lock-in* secara absolut. Hasil generate langsung `go build` tanpa tool tambahan, paling mudah dipahami pembaca baru. |
| DI/wiring | `google/wire` (compile-time codegen) | **v0.7.0** — 22 Agu 2025; **ARCHIVED 25 Agu 2025** (read-only, README: *"This project is no longer maintained"*, ~14.4k stars, Apache-2.0) | **Hindari (sebagai default)** | Repo sudah di-archive & read-only → melanggar aturan "jangan jadikan default library archived". Juga butuh step codegen (`wire`) + import `github.com/google/wire` di kode hasil generate → menambah lock-in & memperumit alur build. |
| DI/wiring | `uber-go/fx` (runtime DI) | **v1.24.0** — 13 Mei 2025; aktif/stabil (v1 strict semver, ~7.5k stars, MIT, tidak archived) | **Alternatif** | DI runtime matang & teruji skala besar (backbone service Go di Uber), tapi memaksa kode hasil generate meng-import `go.uber.org/fx` di seluruh wiring → menambah dependency + lock-in dan menyembunyikan alur inisialisasi (kurang transparan untuk pemula). Tidak ideal sebagai default, tapi berharga untuk pengguna yang memang mau DI container. |

- **Default:** **Manual constructor injection** (untuk wiring) + **`go-playground/validator/v10`** (untuk validation) — manual DI tidak menambah satu pun dependency, jadi jaminan *zero lock-in* terpenuhi mutlak dan hasil generate langsung lolos `go vet ./... && go build ./... && go test ./...` tanpa tool eksternal (mis. tanpa perlu menjalankan `wire` codegen). `validator/v10` aman dipakai karena aktif dirilis, MIT, dan idiomatik via struct-tag.
- **Alternatif ditawarkan di prompt interaktif:** `uber-go/fx` (runtime DI container, untuk pengguna yang sengaja ingin lifecycle/hooks & DI otomatis). `google/wire` **tidak** ditawarkan sebagai opsi default; jika tetap diminta, sajikan dengan peringatan eksplisit bahwa repo sudah *archived* sejak 25 Agu 2025 dan tidak menerima fitur baru.
- **Catatan/implikasi untuk generator:**
  - **Kenapa manual DI paling cocok sebagai default:** (1) **Zero lock-in by construction** — manual constructor injection adalah pola bahasa Go murni (`func NewService(repo Repo) *Service`), nol baris import dari pihak ketiga, sehingga aturan keras "project hasil generate TIDAK BOLEH meng-import package apa pun dari builder/DI framework" terpenuhi tanpa usaha. (2) **Langsung build** — `wire` butuh langkah codegen sebelum `go build` (jika file `wire_gen.go` belum di-generate, build gagal → melanggar aturan "lolos tanpa edit manual"); `fx` butuh runtime container. Manual DI tidak butuh keduanya. (3) **Mudah dipahami** — wiring eksplisit di `main.go`/`bootstrap` terbaca top-to-bottom, ideal untuk pengalaman "laravel new" di mana pengguna membaca hasil generate untuk belajar. (4) **Skala arsitektur** — manual DI scaling rapi untuk monolith & modular monolith; untuk microservice, tiap service tetap punya `main.go` ringkas. Container baru memberi nilai ketika jumlah dependency sangat besar — itulah kenapa `fx` disediakan sebagai opt-in, bukan default.
  - **Konsekuensi build hasil generate:** untuk validator, generator cukup menambahkan `github.com/go-playground/validator/v10` ke `go.mod` + 1 contoh struct ber-tag minimal; ini tidak melanggar zero lock-in karena yang dilarang adalah import **dari builder**, bukan dari library publik pihak ketiga. Pastikan versi dipin ke rilis stabil terbaru (v10.30.3) di template `go.mod`.
  - **Hindari menanam `google/wire`** di template default maupun di `go.mod` hasil generate, karena dependency archived akan menua tanpa patch keamanan dan memberi sinyal buruk pada project yang seharusnya "best-practice & fresh".

**Sumber:**
- go-playground/validator — pkg.go.dev (v10.30.3, 29 Mei 2026, MIT): https://pkg.go.dev/github.com/go-playground/validator/v10
- go-playground/validator — GitHub repo (≈20k stars, aktif, tidak archived): https://github.com/go-playground/validator
- go-playground/validator — Releases: https://github.com/go-playground/validator/releases
- google/wire — GitHub repo (ARCHIVED 25 Agu 2025, README "no longer maintained", v0.7.0 22 Agu 2025, Apache-2.0, ≈14.4k stars): https://github.com/google/wire
- google/wire — pkg.go.dev: https://pkg.go.dev/github.com/google/wire
- uber-go/fx — GitHub repo (≈7.5k stars, MIT, aktif): https://github.com/uber-go/fx
- uber-go/fx — CHANGELOG (v1.24.0, 13 Mei 2025; "Unreleased: No changes yet"): https://github.com/uber-go/fx/blob/master/CHANGELOG.md
- uber-go/fx — Releases: https://github.com/uber-go/fx/releases

---

### 3.5 Messaging & Observability

| concern | kandidat | status maintenance (versi & tanggal rilis terakhir) | rekomendasi (default / alternatif / hindari) | alasan |
|---|---|---|---|---|
| Messaging — NATS | `nats-io/nats.go` | v1.52.0 — 7 Mei 2026; aktif, tidak archived; ~6.6k stars; Apache-2.0; dependency footprint kecil (pure Go) | **Default (microservice)** | Maintained resmi oleh nats-io; pure Go, dependency ringan, mendukung core NATS + JetStream + micro service API dalam satu klien. Cocok untuk hasil generate: server NATS gampang di-`docker compose up` tanpa Zookeeper/broker berat, dan wiring contoh minimal. ([github](https://github.com/nats-io/nats.go) · [pkg.go.dev](https://pkg.go.dev/github.com/nats-io/nats.go)) |
| Messaging — Kafka | `twmb/franz-go` | v1.21.2 (modul `pkg/kgo`) — 15 Mei 2026; aktif, tidak archived; ~2.9k stars; BSD-3-Clause; pure Go, tanpa CGO | **Default (Kafka)** | Feature-complete & pure Go (Kafka 0.8–4.2+), mendukung transaksi, consumer group, exactly-once. API modern (mis. `iter.Seq`), aktif dirilis. Pure Go → zero-CGO, build mulus untuk hasil generate. ([github](https://github.com/twmb/franz-go) · [pkg.go.dev/kgo](https://pkg.go.dev/github.com/twmb/franz-go/pkg/kgo)) |
| Messaging — Kafka | `segmentio/kafka-go` | v0.4.51 — 23 Apr 2026; aktif, tidak archived; ~8.6k stars; MIT; pure Go | **Alternatif** | Populer, API sederhana (`Reader`/`Writer`), pure Go. Masih di seri v0.x (API belum dijanjikan stabil v1) dan beberapa fitur lanjutan kurang lengkap dibanding franz-go, sehingga sebagai alternatif, bukan default. ([github](https://github.com/segmentio/kafka-go) · [pkg.go.dev](https://pkg.go.dev/github.com/segmentio/kafka-go)) |
| Messaging — Kafka | `IBM/sarama` | v1.50.2 — 5 Jun 2026; **masih aktif** (bukan archived/deprecated); ~12.5k stars; MIT | **Alternatif (catatan: gunakan hanya bila ada ketergantungan eksisting)** | Tetap dirilis rutin oleh IBM dan paling banyak dipakai secara historis, jadi BUKAN library archived. Namun API-nya lebih low-level/verbose dan berat secara konfigurasi; banyak proyek baru bermigrasi ke franz-go. Layak ditawarkan tapi tidak jadi default. ([github](https://github.com/IBM/sarama) · [pkg.go.dev](https://pkg.go.dev/github.com/IBM/sarama)) |
| Messaging — RabbitMQ | `rabbitmq/amqp091-go` | v1.11.0 — 21 Apr 2026; aktif, tidak archived; ~2.0k stars; BSD-2-Clause; di-maintain tim inti RabbitMQ | **Default (RabbitMQ)** | Klien AMQP 0.9.1 resmi RabbitMQ (penerus `streadway/amqp` yang sudah tidak dimaintain). Satu-satunya pilihan default yang masuk akal untuk RabbitMQ; broker mudah di-`docker compose up`. ([github](https://github.com/rabbitmq/amqp091-go) · [pkg.go.dev](https://pkg.go.dev/github.com/rabbitmq/amqp091-go)) |
| Observability — Tracing/Metrics | `go.opentelemetry.io/otel` | v1.44.0 (trace/metric API & SDK GA; logs masih beta) — 27 Mei 2026; aktif; ~6.4k stars; Apache-2.0 | **Default** | Standar industri vendor-neutral; Traces & Metrics sudah **Stable (GA)**. Instrumentasi netral → hasil generate bisa di-export ke backend apa pun (OTLP) tanpa lock-in. ([github](https://github.com/open-telemetry/opentelemetry-go) · [pkg.go.dev](https://pkg.go.dev/go.opentelemetry.io/otel)) |
| Observability — Metrics | `prometheus/client_golang` | v1.23.2 — 5 Sep 2025; aktif, tidak archived; lisensi Apache-2.0 | **Default (metrics endpoint)** | Klien metrics de-facto untuk Go; `/metrics` handler standar, Prometheus mudah di-scrape via `docker compose`. Sangat stabil & ringan. Catatan: untuk tracing tetap pakai OTel; keduanya komplementer. ([github](https://github.com/prometheus/client_golang) · [pkg.go.dev](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus)) |
| Observability — Health check | `alexliesenfeld/health` | last update 17 Mar 2026; aktif; ~830 stars; MIT | **Alternatif** | Library health-check fleksibel (sync/async, caching). Bagus, tapi menambah satu dependency untuk hal yang kecil. Tawarkan sebagai opsi, bukan default. ([github](https://github.com/alexliesenfeld/health) · [pkg.go.dev](https://pkg.go.dev/github.com/alexliesenfeld/health)) |
| Observability — Health check | health buatan sendiri (net/http) | n/a (stdlib) | **Default (health check)** | Endpoint `/healthz` & `/readyz` cukup ditulis dengan `net/http` stdlib — zero dependency, langsung build, zero lock-in. Paling sesuai filosofi generator. ([pkg.go.dev/net/http](https://pkg.go.dev/net/http)) |

- **Default:**
  - Messaging microservice: **NATS (`nats-io/nats.go`)** untuk transport ringan default; **Kafka → `twmb/franz-go`**; **RabbitMQ → `rabbitmq/amqp091-go`**. Semuanya pure Go (zero/low CGO) sehingga `go build ./...` langsung lolos, dependency footprint terkendali, dan broker-nya gampang dinyalakan lewat `docker compose up`. Tidak ada yang memaksa import package builder → zero lock-in tetap terjaga.
  - Observability: **OpenTelemetry-Go (`go.opentelemetry.io/otel`)** untuk tracing/metrics (vendor-neutral, OTLP), **`prometheus/client_golang`** untuk endpoint `/metrics`, dan **health check buatan sendiri dengan `net/http` stdlib** (zero dependency). Kombinasi ini stabil (API GA), ringan, dan hasil generate bisa di-wire ke backend apa pun tanpa lock-in.

- **Alternatif ditawarkan di prompt interaktif:**
  - Kafka client: `segmentio/kafka-go` (API lebih simpel) atau `IBM/sarama` (jika tim sudah punya basis kode sarama).
  - Health check: `alexliesenfeld/health` (bila butuh sync/async + caching siap pakai daripada menulis sendiri).
  - Transport messaging: pilihan antara NATS / Kafka / RabbitMQ ditawarkan sesuai kebutuhan stack target.

- **Catatan/implikasi untuk generator:**
  - **Status sarama vs franz-go:** `IBM/sarama` **MASIH AKTIF** dimaintain (rilis v1.50.2 pada 5 Jun 2026, ~12.5k stars, MIT) — **bukan** library archived/deprecated, jadi boleh ditawarkan sebagai alternatif. Namun untuk **klien Kafka default**, pilih **`twmb/franz-go`** (v1.21.2, 15 Mei 2026): feature-complete, pure Go, API lebih modern/ringkas, dan tren adopsi proyek baru mengarah ke sini. Rekomendasi default Kafka berbasis maintenance + kualitas API = **franz-go**.
  - Catat juga `streadway/amqp` (pendahulu amqp091-go) sudah **tidak dimaintain** — JANGAN dipakai; gunakan `rabbitmq/amqp091-go` resmi.
  - `prometheus/client_golang` rilis stabil terakhir yang terverifikasi adalah **v1.23.2 (5 Sep 2025)** — masih aktif dan stabil; tidak ada tanda archived.
  - Untuk menjaga **zero lock-in**, generator hanya menulis wiring + 1 contoh minimal (mis. satu publisher/subscriber NATS, satu `/metrics` handler, satu `/healthz`), tanpa abstraksi milik builder.

**Sumber:**
- https://github.com/nats-io/nats.go · https://pkg.go.dev/github.com/nats-io/nats.go
- https://github.com/twmb/franz-go · https://pkg.go.dev/github.com/twmb/franz-go/pkg/kgo
- https://github.com/segmentio/kafka-go · https://pkg.go.dev/github.com/segmentio/kafka-go
- https://github.com/IBM/sarama · https://pkg.go.dev/github.com/IBM/sarama
- https://github.com/rabbitmq/amqp091-go · https://pkg.go.dev/github.com/rabbitmq/amqp091-go
- https://github.com/open-telemetry/opentelemetry-go · https://pkg.go.dev/go.opentelemetry.io/otel
- https://github.com/prometheus/client_golang/releases · https://pkg.go.dev/github.com/prometheus/client_golang/prometheus
- https://github.com/alexliesenfeld/health · https://pkg.go.dev/github.com/alexliesenfeld/health

---

### 3.6 Auth Scaffold & Testing

| concern | kandidat | status maintenance (versi & tanggal rilis terakhir) | rekomendasi (default / alternatif / hindari) | alasan |
|---|---|---|---|---|
| Auth (token) | **golang-jwt/jwt/v5** | v5.3.1 — 28 Jan 2026; aktif, tidak archived; MIT; ~7.5k★ | **Default** | Standar de-facto JWT di ekosistem Go, RFC 7519. Hanya bergantung pada stdlib crypto (`crypto/ecdsa`, `crypto/rsa`, `ed25519`) — **zero external deps**, jadi tidak menambah beban dependency pada project hasil generate. API v5 stabil dan banyak dipakai. Cocok untuk scaffold auth minimal yang langsung `go build`. ([github releases](https://github.com/golang-jwt/jwt/releases), [pkg.go.dev v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5)) |
| Auth (token) | aidantwoods/go-paseto | v1.5.4 — 18 Feb 2026; aktif; MIT; ~366★ | **Alternatif** | PASETO menghindari kelas kerentanan JWT (algorithm-confusion, `alg:none`); ini fork PASETO Go yang paling aktif dirawat. Bukan default karena adopsi/komunitas jauh lebih kecil dari JWT dan menarik dependency tambahan (mis. `golang.org/x/crypto`), tapi layak sebagai opsi keamanan-first. ([github releases](https://github.com/aidantwoods/go-paseto/releases)) |
| Auth (token) | o1egl/paseto | v2.0.0 — 18 Jan 2020; rilis terakhir >6 thn lalu, repo dorman (bukan archived); MIT; ~938★ | **Hindari (sebagai default)** | Tidak archived secara formal, namun praktis stagnan sejak 2020 dan maintainer menyatakan "spec selesai, tidak ada update". Untuk PASETO pilih `aidantwoods/go-paseto` yang aktif. Boleh disebut sebagai catatan, bukan default. ([github](https://github.com/o1egl/paseto), [issue #36 "is it still active?"](https://github.com/o1egl/paseto/issues/36)) |
| Testing (assertion) | **stretchr/testify** | v1.11.1 — 27 Agu 2025; aktif; MIT; ~26k★ | **Default** | Assertion + suite toolkit paling banyak dipakai di Go; ekspektasi gold-standard untuk test hasil generate. Beban dependency ringan dan kompatibel `go test`. ([github releases](https://github.com/stretchr/testify/releases)) |
| Testing (mock gen) | **uber-go/mock** (`go.uber.org/mock`) | v0.5.2 — 28 Apr 2025; aktif; Apache-2.0; ~3k★ | **Default (mock)** | Fork resmi & terawat dari `golang/mock` (yang sudah archived). Code-generation berbasis `//go:generate` — mock dihasilkan sebagai file biasa, tidak ada lock-in ke builder. Drop-in untuk pengguna `golang/mock`. ([github](https://github.com/uber-go/mock), [pkg.go.dev](https://pkg.go.dev/go.uber.org/mock/gomock)) |
| Testing (mock gen) | golang/mock | v1.6.0 — 11 Jun 2021; **ARCHIVED 27 Jun 2023, read-only** | **Hindari** | README resmi: "no longer maintained… use go.uber.org/mock instead". Tidak boleh jadi default. ([github archived](https://github.com/golang/mock)) |
| Testing (mock gen) | vektra/mockery | v3.7.0 — 6 Mar 2026; aktif; BSD-3-Clause; ~7.1k★ | **Alternatif** | Generator mock berbasis interface (gaya testify-mock) yang sangat populer; enak untuk project yang sudah pakai `testify/mock`. Alternatif kuat bila tim lebih suka mock expressive dibanding gaya gomock. ([github releases](https://github.com/vektra/mockery/releases)) |
| Testing (integration) | **testcontainers/testcontainers-go** | v0.42.0 — 9 Apr 2026; aktif; MIT; ~4.8k★ | **Default (khusus test berlabel `integration`)** | Standar untuk integration test berbasis container (Postgres/Redis dll.) — relevan untuk stack DB. Disisihkan ke build-tag `integration` agar tidak masuk `go test` default (butuh Docker). ([github releases](https://github.com/testcontainers/testcontainers-go/releases)) |
| Testing (BDD, opsional) | onsi/ginkgo + onsi/gomega | ginkgo v2.29.0 — 17 Mei 2026 (~9k★); gomega v1.39.1 — 30 Jan 2026 (~2.3k★); keduanya aktif; MIT | **Alternatif (opt-in)** | Framework BDD matang & terawat. Tidak dijadikan default karena menambah gaya/DSL dan dependency yang tidak perlu untuk scaffold minimal; idiom `testing` + `testify` lebih ringan dan netral. ([ginkgo releases](https://github.com/onsi/ginkgo/releases), [gomega](https://github.com/onsi/gomega/releases)) |

- **Default:** **golang-jwt/jwt/v5** (auth) + **testify** (assertion) + **uber-go/mock** (mock generation) + **testcontainers-go** untuk integration test berlabel. Kombinasi ini cocok untuk hasil generate karena: (1) golang-jwt hanya pakai stdlib crypto → beban dependency minimal & langsung `go build`; (2) ketiga library testing menghasilkan kode/test sebagai file biasa tanpa meng-import package builder apa pun → **zero lock-in**; (3) semuanya aktif dirawat per 2026 dan lulus `go vet`/`go build`/`go test` out-of-the-box.

- **Alternatif ditawarkan di prompt interaktif:**
  - Auth: `aidantwoods/go-paseto` (PASETO security-first, hindari kerentanan JWT).
  - Mock: `vektra/mockery` (gaya testify-mock) sebagai pengganti `uber-go/mock`.
  - BDD: `ginkgo + gomega` (opt-in untuk tim yang menyukai BDD).

- **Catatan/implikasi untuk generator:**
  - **testcontainers-go butuh Docker daemon saat test dijalankan.** Jika test container dimasukkan ke `*_test.go` biasa, maka aturan keras "`go test ./...` tanpa edit manual" akan **gagal di mesin tanpa Docker** (CI minimal, sandbox). Karena itu generator HARUS memisahkan test integrasi di balik build tag, mis. file `//go:build integration`, sehingga: `go test ./...` default = hanya unit test (testify + mock, no Docker, selalu hijau), sedangkan `go test -tags=integration ./...` = jalankan testcontainers (butuh Docker, sejalan dengan `docker compose up` untuk stack DB).
  - **uber-go/mock & mockery sama-sama code generator** (`//go:generate mockgen ...` / `mockery`). Untuk memenuhi "langsung build tanpa edit manual", generator harus **men-commit file mock hasil generate** ke output (bukan hanya menulis directive `go:generate`), agar `go build`/`go test` langsung jalan tanpa perlu menjalankan tool generator lebih dulu. Direktif `go:generate` boleh disertakan sebagai dokumentasi untuk regenerasi.
  - **golang-jwt** tidak punya implikasi build khusus (zero external deps) — aman jadi default lintas ketiga mode arsitektur (monolith/modular/microservice).

**Sumber:**
- golang-jwt/jwt: https://github.com/golang-jwt/jwt/releases · https://pkg.go.dev/github.com/golang-jwt/jwt/v5
- aidantwoods/go-paseto: https://github.com/aidantwoods/go-paseto/releases
- o1egl/paseto: https://github.com/o1egl/paseto · https://github.com/o1egl/paseto/issues/36
- stretchr/testify: https://github.com/stretchr/testify/releases
- uber-go/mock: https://github.com/uber-go/mock · https://pkg.go.dev/go.uber.org/mock/gomock
- golang/mock (archived): https://github.com/golang/mock
- vektra/mockery: https://github.com/vektra/mockery/releases
- testcontainers-go: https://github.com/testcontainers/testcontainers-go/releases
- onsi/ginkgo: https://github.com/onsi/ginkgo/releases · onsi/gomega: https://github.com/onsi/gomega/releases

---

## 4. Catatan Lintas-Concern

### 4.1 Library yang sengaja DIHINDARI sebagai default

| library | concern | status | alasan menghindari | pengganti default |
|---|---|---|---|---|
| `google/wire` | DI / wiring | **ARCHIVED 25 Agu 2025** (read-only, "no longer maintained") | Repo di-archive → melanggar aturan "jangan jadikan default library archived"; butuh step codegen + import → menambah lock-in. | Manual constructor injection |
| `golang/mock` | Testing — mock gen | **ARCHIVED 27 Jun 2023** (read-only; README: "use go.uber.org/mock instead") | Tidak lagi dimaintain; sudah ada fork resmi terawat. | `uber-go/mock` |
| `streadway/amqp` | Messaging — RabbitMQ | **Tidak dimaintain** (digantikan klien resmi) | Pendahulu yang ditinggalkan; klien resmi RabbitMQ kini menjadi standar. | `rabbitmq/amqp091-go` |
| `mattn/go-sqlite3` | DB Driver — SQLite | Aktif, **tetapi butuh CGO** | `CGO_ENABLED=1` + C toolchain memecah janji "langsung `go build` & `docker compose up` tanpa edit" dan menyulitkan image kecil/cross-compile. Boleh opt-in bila user sadar konsekuensi. | `modernc.org/sqlite` (pure-Go) |
| `o1egl/paseto` | Auth (token) | Dorman sejak v2.0.0 (18 Jan 2020); **bukan archived** | Stagnan >6 tahun; untuk PASETO pilih fork yang aktif. | `aidantwoods/go-paseto` (jika PASETO dibutuhkan) |

> **Catatan status:** `IBM/sarama` **TIDAK** dihindari — masih aktif (v1.50.2, 5 Jun 2026) dan boleh ditawarkan sebagai alternatif Kafka, hanya bukan default. `spf13/viper` juga aktif; bukan default semata karena beban dependency.

### 4.2 Kombinasi yang saling memengaruhi

- **HTTP framework menentukan seluruh matrix add-on (`net/http` vs `fasthttp`).** Empat kandidat (`net/http`, `chi`, `gin`, `echo`) berbasis `net/http` sehingga berbagi satu set template add-on (otel `otelhttp`, health `http.Handler`, middleware logging/recovery). Hanya `fiber` yang berbasis `fasthttp` → membutuhkan cabang template terpisah (`otelfiber`, signature `fiber.Ctx`). Memilih `fiber` menggandakan biaya maintenance template observability/health/middleware di builder. Perlakukan ini sebagai *flag arsitektural*.
- **sqlc + pgx saling melengkapi.** sqlc dapat men-generate output yang menggunakan `pgx` (atau `database/sql`) sebagai runtime; tidak ada konflik. Bila user memilih sqlc, default driver Postgres tetap `pgx/v5`, dan output codegen-nya menempel di atas pgx tanpa runtime lock-in tambahan. Konsekuensi: mode sqlc menambah langkah `sqlc generate` sebelum `go build` (lihat §3.3).
- **Logger di belakang `*slog.Logger`.** Default logging `slog` sebaiknya di-wire di belakang `*slog.Logger`/`slog.Handler`. Jika user nanti memilih `zap`/`zerolog`, cukup ganti handler tanpa menyentuh call-site — menjaga zero lock-in dan menghindari rework wiring lintas concern.
- **DI vs codegen (wire/sqlc/ent/mock) vs aturan "tanpa edit manual".** Default manual DI sengaja menghindari ketergantungan pada langkah codegen. Untuk semua library berbasis codegen yang dipilih opt-in (sqlc, ent, uber-go/mock, mockery), generator **wajib** menjalankan langkah generate di post-gen atau men-commit output ter-generate, agar `go build ./...` & `go test ./...` lolos tanpa intervensi.
- **testcontainers di balik build-tag `integration`.** Default `go test ./...` (unit, testify + uber-go/mock) harus selalu hijau tanpa Docker; testcontainers dipisah ke `//go:build integration` dan baru dijalankan dengan `go test -tags=integration ./...` — sejalan dengan `docker compose up` untuk stack ber-DB.
- **Catatan lisensi untuk dokumentasi project hasil generate.** Mayoritas default MIT/BSD/Apache-2.0. Pengecualian yang perlu dicatat: `go-sql-driver/mysql` = MPL-2.0 (copyleft file-level, tetap aman sebagai dependency), dan `ariga/atlas` binary default ber-EULA (gunakan Community Edition Apache-2.0). Bila MySQL/atlas masuk stack, catat di dokumentasi lisensi output.
