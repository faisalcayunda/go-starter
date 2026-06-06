# 05 — Decision Matrix: Jawaban User → Modul Template → File Generate → Dependency `go.mod`

> **Status:** Source of truth untuk pemetaan keputusan generator `gostarter` v1.
> **Tanggal:** 2026-06-06
> **Turunan dari:** [01-monolith.md](./01-monolith.md), [02-microservice.md](./02-microservice.md), [03-libraries.md](./03-libraries.md), [04-competitors-tooling.md](./04-competitors-tooling.md).
> **Aturan keras (berlaku untuk semua output):** hasil generate lolos `go vet ./... && go build ./... && go test ./...` **tanpa edit manual**; `docker compose up` jalan untuk stack ber-DB; project hasil generate **TIDAK meng-import package apa pun dari builder** (zero lock-in); hanya struktur + wiring + 1 contoh minimal (bukan business logic).

---

## 1. Pendahuluan

Dokumen ini adalah **peta keputusan deterministik** yang menghubungkan empat lapis:

```
[jawaban user]  →  [modul template yang aktif]  →  [file/folder yang di-generate]  →  [dependency go.mod yang ditambahkan]
```

Tujuannya: untuk setiap pertanyaan yang diajukan generator (mode interaktif via `huh`) atau setiap flag yang diberikan (mode non-interaktif via `cobra`), kita tahu **persis** modul template mana yang aktif, file/folder apa yang ditulis, dan baris `require` apa yang masuk ke `go.mod`. Ini adalah kontrak yang membuat builder bisa diuji (titik sisip deterministik) dan membuat aturan keras "build/test hijau tanpa edit" bisa dijamin.

### 1.1 Konvensi & definisi

- **Modul template** = potongan template yang di-compose (pola "composition by category" yang diadopsi dari go-blueprint, diperluas dengan dimensi **arsitektur** — lihat [04 §2.2](./04-competitors-tooling.md)). Satu project = penjumlahan beberapa modul template aktif.
- **Default by construction** = nilai yang dipilih bila flag tidak diberikan / user menekan Enter. Semua default mengikuti rekomendasi dokumen 01–03 (paling ringan, pure-Go, zero-CGO, zero lock-in).
- **"core"** = modul template yang **selalu** aktif tanpa memandang jawaban (skeleton, `go.mod`, `README.md`, `.gitignore`).
- **Dependency runtime** yang masuk ke `go.mod` hasil generate **selalu** library publik pihak ketiga atau stdlib — **tidak pernah** package builder `gostarter`. Versi di-pin ke rilis stabil terverifikasi 2026-06-06 (lihat [03](./03-libraries.md)).
- **Pure-Go / zero-CGO** adalah syarat default agar `CGO_ENABLED=0` aman → cross-compile & Docker base minimal mulus.

### 1.2 Cara membaca matrix

Kolom `dependency go.mod` mencantumkan **module path + versi pin**. Tanda `—` berarti **tidak ada dependency baru** (stdlib / pola bahasa murni). Tanda `(stdlib)` menegaskan zero-dependency. Versi mengikuti [03-libraries.md](./03-libraries.md) §2.

---

## 2. Matrix Utama

### 2.1 Dimensi 1 — Tipe Arsitektur (pertanyaan pertama, menentukan segalanya)

| Pertanyaan/Jawaban user | Nilai opsi | Modul template yang aktif | File/folder yang di-generate | Dependency `go.mod` yang ditambahkan |
|---|---|---|---|---|
| **Tipe arsitektur?** (`--arch`) — **DEFAULT: `monolith`** | `monolith` | `arch:monolith` (skeleton layered: `cmd/` + `internal/`) | `cmd/api/main.go`, `internal/app/app.go`, `internal/config/config.go`, `internal/http/{router.go,middleware.go}`, `internal/<feature>/{service.go,handler.go,repository.go,service_test.go}`, `README.md`, `.gitignore`, `Makefile` | — (skeleton stdlib; HTTP/DB ditentukan dimensi lain) |
| | `modular-monolith` | `arch:modular-monolith` (boundary `internal/` per modul + contract + in-process event bus) | `cmd/monolith/main.go`, `internal/platform/{config,database,eventbus}/`, `internal/shared/contract/{<m>.go,events.go}`, `internal/modules/<m>/{module.go, internal/{service.go,repository.go,handler.go,service_test.go}, migrations/}` | — (event bus in-process = stdlib; DB/HTTP dari dimensi lain) |
| | `microservice` | `arch:microservice` (monorepo single-module: `services/` + `libs/` + `proto/` + `gen/`) | `go.mod` (1 module root), `Makefile`, `docker-compose.yml`, `buf.yaml`, `buf.gen.yaml`, `proto/<svc>/v1/<svc>.proto`, `gen/go/<svc>/v1/*.pb.go` (di-commit), `services/<svc>/{cmd/main.go, internal/{handler,client,service,config}/, Dockerfile, migrations/}`, `libs/{logger,config,grpcclient,health}/` | `google.golang.org/protobuf v1.36.11`, `google.golang.org/grpc v1.81.1` (stub gRPC; lihat dimensi komunikasi) |

> **Catatan turunan:** modular-monolith & microservice **tidak** memakai `pkg/` (kritik Russ Cox; lihat [01 §1.2](./01-monolith.md)); shared code microservice ada di `libs/` (bukan `pkg/`) agar tidak menyiratkan "publik untuk diimport luar" ([02 §4](./02-microservice.md)).

### 2.2 Dimensi 2 — Jenis Monolith (hanya bila `arch ∈ {monolith, modular-monolith}`)

| Pertanyaan/Jawaban user | Nilai opsi | Modul template yang aktif | File/folder yang di-generate | Dependency `go.mod` yang ditambahkan |
|---|---|---|---|---|
| **Jenis aplikasi?** (`--kind`) — **DEFAULT: `rest`** | `rest` (HTTP REST API) | `kind:rest` | `internal/http/router.go` (routes contoh `GET /healthz`, `GET /v1/<feature>/{id}`), `internal/<feature>/handler.go` | — (default `net/http`) |
| | `web` (server-rendered, HTML) | `kind:web` | `internal/http/router.go` + `internal/web/templates/*.tmpl` (`html/template`), `internal/web/static/`, handler render HTML | — (`html/template` stdlib) |
| | `worker` (background/queue consumer, tanpa HTTP server) | `kind:worker` | `cmd/worker/main.go` (loop konsumsi), `internal/worker/{worker.go,worker_test.go}`; **tanpa** `internal/http/` | — bila in-memory; broker bila dipilih (lihat §2.7) |

> **Implikasi:** `kind:worker` mematikan modul `http` (tidak ada router/middleware). `kind:web` mengaktifkan `html/template` + folder `static/`. Default `rest` paling umum & paling ringan.

### 2.3 Dimensi 3 — HTTP Framework (hanya bila `kind ∈ {rest, web}` atau microservice memakai `gateway`)

| Pertanyaan/Jawaban user | Nilai opsi | Modul template yang aktif | File/folder yang di-generate | Dependency `go.mod` yang ditambahkan |
|---|---|---|---|---|
| **HTTP framework?** (`--http`) — **DEFAULT: `net/http`** | `net/http` (stdlib, routing Go 1.22+) | `http:stdlib` | `internal/http/router.go` (`http.ServeMux`, `GET /path/{id}`), middleware `http.Handler` | — (stdlib) |
| | `chi` | `http:chi` | `router.go` pakai `chi.NewRouter()`, `chi.URLParam` | `github.com/go-chi/chi/v5 v5.3.0` |
| | `gin` | `http:gin` | `router.go` pakai `gin.Engine`, handler `gin.Context` | `github.com/gin-gonic/gin v1.12.0` |
| | `echo` | `http:echo` | `router.go` pakai `echo.New()`, handler `echo.Context` (default v4 LTS) | `github.com/labstack/echo/v4 v4.15.2` |
| | `fiber` ⚠ | `http:fiber` (**cabang template terpisah** — fasthttp) | `router.go` pakai `fiber.New()`, handler `fiber.Ctx`; add-on memakai varian Fiber | `github.com/gofiber/fiber/v3 v3.3.0` (butuh Go 1.25+) |

> **`net/http`, `chi`, `gin`, `echo` = kelas `net/http`** → berbagi satu set template add-on (otel `otelhttp`, health `http.Handler`, middleware logging/recovery). **`fiber` = kelas `fasthttp`** → memicu cabang template terpisah untuk observability/health/middleware (`otelfiber`, signature `fiber.Ctx`). Lihat tabel constraint §3.

### 2.4 Dimensi 4a — Database (driver)

| Pertanyaan/Jawaban user | Nilai opsi | Modul template yang aktif | File/folder yang di-generate | Dependency `go.mod` yang ditambahkan |
|---|---|---|---|---|
| **Pakai database?** (`--db`) — **DEFAULT: `none`** | `none` | — (DB dimatikan) | tidak ada `migrations/`, tidak ada `docker-compose.yml` service DB, tidak ada `store.go`/`repository.go` impl DB | — |
| | `postgres` | `db:postgres` | `internal/platform/database/postgres.go` (`pgxpool`), `migrations/0001_init.{up,down}.sql`, blok `postgres` di `docker-compose.yml`, `.env` `DATABASE_URL` | `github.com/jackc/pgx/v5 v5.10.0` |
| | `mysql` | `db:mysql` | `database/mysql.go`, `migrations/...`, blok `mysql` di compose, `.env` | `github.com/go-sql-driver/mysql v1.9.3` (MPL-2.0 — catat di docs lisensi) |
| | `sqlite` | `db:sqlite` | `database/sqlite.go` (pure-Go, CGO-free), `migrations/...`; **tanpa** service DB di compose (file lokal) | `modernc.org/sqlite v1.39.0` |
| | `mongo` | `db:mongo` | `database/mongo.go` (driver **v2**), blok `mongo` di compose, `.env` | `go.mongodb.org/mongo-driver/v2 v2.6.0` |

> **Semua driver default pure-Go (zero-CGO)** → `CGO_ENABLED=0` aman, Docker base minimal. `mattn/go-sqlite3` **tidak** ditawarkan sebagai default (butuh CGO) — hanya opt-in dengan peringatan ([03 §3.3](./03-libraries.md)).

### 2.5 Dimensi 4b — Access Layer (hanya bila `db ≠ none`)

| Pertanyaan/Jawaban user | Nilai opsi | Modul template yang aktif | File/folder yang di-generate | Dependency `go.mod` yang ditambahkan |
|---|---|---|---|---|
| **Lapisan akses query?** (`--access`) — **DEFAULT: `sqlx`** | `sqlx` | `access:sqlx` | `repository.go` pakai `sqlx.DB` (`Get`/`Select`/`StructScan`) | `github.com/jmoiron/sqlx v1.4.0` |
| | `database/sql` | `access:stdlib` | `repository.go` pakai `*sql.DB` murni | — (stdlib; driver tetap dari §2.4) |
| | `sqlc` ⚙ | `access:sqlc` (**butuh langkah generate**) | `sqlc.yaml`, `db/query/*.sql`, `db/schema/*.sql`, output `db/sqlc/*.go` (**di-commit** / di-generate post-gen) | `github.com/jackc/pgx/v5` (runtime; output sqlc memakai pgx/`database/sql`) |
| | `gorm` | `access:gorm` | `repository.go` pakai `*gorm.DB`, model ber-tag GORM | `gorm.io/gorm v1.31.1` + driver GORM (mis. `gorm.io/driver/postgres`) |
| | `ent` ⚙ | `access:ent` (**butuh `go generate`**) | `ent/schema/*.go`, output `ent/*.go` (**di-commit** / di-generate post-gen) | `entgo.io/ent v0.14.6` |

> **`sqlc` & `ent` adalah code generator** → generator **wajib** menjalankan langkah generate di post-gen **atau** men-commit output, agar `go build` lolos tanpa edit manual (lihat constraint §3). Default `sqlx` = zero codegen, beku & stabil.

### 2.6 Dimensi 4c — Migration (hanya bila `db ≠ none`)

| Pertanyaan/Jawaban user | Nilai opsi | Modul template yang aktif | File/folder yang di-generate | Dependency `go.mod` yang ditambahkan |
|---|---|---|---|---|
| **Tool migrasi?** (`--migrate`) — **DEFAULT: `golang-migrate`** | `golang-migrate` | `migrate:golang-migrate` | `migrations/0001_init.{up,down}.sql`, target `make migrate-up/down`, dok CLI di `README.md` | `github.com/golang-migrate/migrate/v4 v4.19.1` (sbg library; CLI dipakai via Makefile/Docker) |
| | `goose` | `migrate:goose` | `migrations/0001_init.sql` (format goose, bisa `embed.FS`), target Makefile | `github.com/pressly/goose/v3 v3.27.1` |
| | `atlas` (CE) | `migrate:atlas` | `atlas.hcl`, `migrations/` (atlas), target Makefile | — (binary CE Apache-2.0 dipakai via Makefile; **gunakan Community Edition**, binary default ber-EULA) |

> Migration **butuh DB** — opsi ini tidak muncul bila `db = none` (constraint §3). File migrasi default = SQL `up/down` polos → zero lock-in.

### 2.7 Dimensi 5 — Komunikasi Microservice (hanya bila `arch = microservice`)

| Pertanyaan/Jawaban user | Nilai opsi | Modul template yang aktif | File/folder yang di-generate | Dependency `go.mod` yang ditambahkan |
|---|---|---|---|---|
| **Pola komunikasi?** (`--comm`) — **DEFAULT: `grpc`** | `grpc` | `comm:grpc` | `proto/<svc>/v1/<svc>.proto`, `gen/go/.../*_grpc.pb.go` (di-commit), `services/order/internal/client/user.go` (contoh 2-service call), `libs/grpcclient/dial.go` | `google.golang.org/grpc v1.81.1`, `google.golang.org/protobuf v1.36.11` |
| | `rest` | `comm:rest` + `gateway` | `gateway/{cmd/main.go, internal/router/}`, `gateway/Dockerfile`, blok `gateway` di compose; service expose HTTP | `github.com/go-chi/chi/v5 v5.3.0` (atau pilihan `--http`) |
| | `event` (event-driven) | `comm:event` (**butuh broker** — lihat §2.8) | `services/<svc>/internal/{publisher,subscriber}/`, contoh 1 publisher + 1 subscriber | sesuai broker (§2.8) |

### 2.8 Dimensi 5b — Message Broker (hanya bila `arch = microservice` **dan** `comm = event`)

| Pertanyaan/Jawaban user | Nilai opsi | Modul template yang aktif | File/folder yang di-generate | Dependency `go.mod` yang ditambahkan |
|---|---|---|---|---|
| **Broker?** (`--broker`) — **DEFAULT: `nats`** | `nats` | `broker:nats` | `libs/broker/nats.go`, contoh pub/sub di `services/<svc>/internal/`, blok `nats` di `docker-compose.yml` | `github.com/nats-io/nats.go v1.52.0` |
| | `kafka` | `broker:kafka` | `libs/broker/kafka.go` (`franz-go`/`kgo`), blok `kafka` (+`zookeeper`/`kraft`) di compose | `github.com/twmb/franz-go v1.21.2` |
| | `rabbitmq` | `broker:rabbitmq` | `libs/broker/rabbitmq.go` (AMQP), blok `rabbitmq` di compose | `github.com/rabbitmq/amqp091-go v1.11.0` |

> Broker **hanya relevan** bila `arch = microservice` **dan** `comm = event` (constraint §3). `streadway/amqp` & sarama-as-default **dihindari** ([03 §4.1](./03-libraries.md)); kafka default = `franz-go` (pure-Go, modern), `IBM/sarama` hanya alternatif opt-in.

### 2.9 Dimensi 6 — Add-ons (multi-select, berlaku lintas arsitektur)

| Pertanyaan/Jawaban user | Nilai opsi | Modul template yang aktif | File/folder yang di-generate | Dependency `go.mod` yang ditambahkan |
|---|---|---|---|---|
| **Docker** (`--docker`) — **DEFAULT: `on` bila `db≠none` atau microservice; else `off`**) | `on` | `addon:docker` | `Dockerfile` (multi-stage, base minimal/distroless), `docker-compose.yml` (+ service DB/broker bila ada), `.dockerignore` | — |
| **Makefile** (`--makefile`) — **DEFAULT: `on`** | `on` | `addon:makefile` (core) | `Makefile` (`vet`, `build`, `test`, `lint`, `run`, `migrate-*`, `proto` bila microservice) | — |
| **CI** (`--ci`) — **DEFAULT: `github-actions`** | `github-actions` | `addon:ci` | `.github/workflows/ci.yml` (vet+build+test+lint) | — |
| | `none` | — | tidak ada workflow | — |
| **Linter** (`--lint`) — **DEFAULT: `on`** | `on` | `addon:golangci-lint` | `.golangci.yml` (ruleset baseline) | — (binary `golangci-lint` dipakai via Makefile/CI, bukan dependency runtime) |
| **Observability** (`--obs`) — **DEFAULT: `off`** | `on` | `addon:observability` (**kelas `net/http` only secara default**) | `internal/platform/otel/otel.go` (`otelhttp`), `/metrics` handler, blok `prometheus`/`otel-collector` di compose | `go.opentelemetry.io/otel v1.44.0`, `github.com/prometheus/client_golang v1.23.2` |
| **Auth / JWT** (`--auth`) — **DEFAULT: `off`** | `jwt` | `addon:auth-jwt` | `internal/auth/{jwt.go,middleware.go,jwt_test.go}`, contoh middleware proteksi 1 route | `github.com/golang-jwt/jwt/v5 v5.3.1` |
| | `paseto` | `addon:auth-paseto` | `internal/auth/{paseto.go,middleware.go}` | `github.com/aidantwoods/go-paseto v1.5.4` (+ `golang.org/x/crypto`) |
| **Validation** (`--validate`) — **DEFAULT: `on` bila ada handler input; else `off`**) | `on` | `addon:validation` | 1 struct contoh ber-tag `validate:"..."` + wiring `validator.New()` | `github.com/go-playground/validator/v10 v10.30.3` |
| **.env / config loader** (`--config`) — **DEFAULT: `godotenv`** | `godotenv` | `config:godotenv` | `.env.example`, `internal/config/config.go` (load `.env` + `os.Getenv`) | `github.com/joho/godotenv v1.5.1` |
| | `koanf` | `config:koanf` | `config.go` pakai koanf provider env/file | `github.com/knadh/koanf/v2 v2.3.5` |
| | `viper` | `config:viper` | `config.go` pakai viper | `github.com/spf13/viper v1.21.0` (deps berat) |
| | `env` (murni stdlib) | `config:env` | `config.go` pakai `os.Getenv` saja | — (stdlib) |
| **Logging** (`--log`) — **DEFAULT: `slog`** | `slog` | `log:slog` (core) | wiring `*slog.Logger` (JSON handler) di `app.go`/`main.go` | — (stdlib) |
| | `zerolog` | `log:zerolog` | adapter di belakang `slog.Handler` | `github.com/rs/zerolog v1.35.1` |
| | `zap` | `log:zap` | adapter di belakang `slog.Handler` | `go.uber.org/zap v1.28.0` |

### 2.10 Dimensi 7 — Testing (selalu aktif sebagai baseline; opsi memperluas)

| Pertanyaan/Jawaban user | Nilai opsi | Modul template yang aktif | File/folder yang di-generate | Dependency `go.mod` yang ditambahkan |
|---|---|---|---|---|
| **Assertion lib** (selalu) | `testify` | `test:testify` (core) | `*_test.go` contoh memakai `require`/`assert` | `github.com/stretchr/testify v1.11.1` |
| **Mock gen** (`--mock`) — **DEFAULT: `off`** | `uber-mock` | `test:mock` (**output di-commit**) | `internal/<feature>/mocks/*.go` (di-commit) + directive `//go:generate mockgen ...` | `go.uber.org/mock v0.5.2` |
| | `mockery` | `test:mockery` (**output di-commit**) | `.mockery.yaml` + mock di-commit | `github.com/vektra/mockery/v3` (tool; output pakai testify/mock) |
| **Integration test** (`--integration`) — **DEFAULT: `off`**; aktif-otomatis-disarankan bila `db≠none` | `on` | `test:testcontainers` (**build-tag `integration`**) | `internal/<feature>/repository_integration_test.go` ber-`//go:build integration` | `github.com/testcontainers/testcontainers-go v0.42.0` |
| **BDD** (`--bdd`) — **DEFAULT: `off`** | `ginkgo` | `test:ginkgo` | `*_suite_test.go` (ginkgo/gomega) | `github.com/onsi/ginkgo/v2 v2.29.0`, `github.com/onsi/gomega v1.39.1` |

> **testcontainers WAJIB di balik build-tag `integration`** → `go test ./...` default = hanya unit test (no Docker, selalu hijau); `go test -tags=integration ./...` = jalankan container. Ini yang menjaga aturan keras "test hijau tanpa edit" di mesin tanpa Docker ([03 §3.6 & §4.2](./03-libraries.md)).

---

## 3. Tabel Constraint — Kombinasi INVALID / Tidak Didukung

Notasi: **requires** = opsi A hanya valid bila prasyarat B terpenuhi; **conflicts** = A & B tidak boleh bersamaan; **needs-step** = valid tapi memicu langkah generate/commit wajib.

| # | Aturan | Tipe | Kondisi | Perilaku generator |
|---|---|---|---|---|
| C1 | `--migrate` butuh DB | **requires** | `migrate ∈ {golang-migrate,goose,atlas}` tetapi `db = none` | **Invalid** → tolak; mode non-interaktif: error `migration requires --db`; interaktif: opsi migrate tidak ditampilkan. |
| C2 | `--access` butuh DB | **requires** | `access ∈ {sqlx,sqlc,gorm,ent,database/sql}` tetapi `db = none` | **Invalid** → access layer tidak relevan; opsi disembunyikan / error. |
| C3 | Broker butuh microservice + event | **requires** | `broker ∈ {nats,kafka,rabbitmq}` tetapi (`arch ≠ microservice` **atau** `comm ≠ event`) | **Invalid** → tolak; broker hanya muncul saat `arch=microservice && comm=event`. |
| C4 | `comm` hanya untuk microservice | **requires** | `--comm` diberikan tetapi `arch ≠ microservice` | **Invalid** → flag diabaikan dengan error; `comm` tidak relevan untuk monolith. |
| C5 | `--kind` hanya untuk monolith/modular | **requires** | `--kind` diberikan tetapi `arch = microservice` | **Invalid** → microservice memakai pola service (cmd/internal), bukan `kind`. |
| C6 | `gateway` butuh microservice | **requires** | gateway diaktifkan tetapi `arch ≠ microservice` | **Invalid** → gateway adalah service edge khusus microservice. |
| C7 | `fiber` ⇒ cabang add-on fasthttp | **conflicts (partial)** | `http = fiber` **dan** `obs = on` (otel) | **Valid dengan cabang** → generator memakai `otelfiber` + signature `fiber.Ctx`, **bukan** `otelhttp`/`http.Handler`. Set add-on `net/http` standar TIDAK kompatibel dengan fiber. |
| C8 | `fiber` butuh Go 1.25+ | **requires** | `http = fiber` | go directive `go.mod` di-set ≥ 1.25; bila target Go lebih rendah → tolak/peringatan. |
| C9 | `kind = worker` ⇒ tanpa HTTP | **conflicts** | `kind = worker` **dan** `--http` diberikan | **Invalid/diabaikan** → worker tidak punya HTTP server; modul `http:*` dimatikan. |
| C10 | `sqlc` perlu langkah generate | **needs-step** | `access = sqlc` | Generator **wajib** men-commit output `db/sqlc/*.go` ATAU menjalankan `sqlc generate` di post-gen sebelum build. Tanpa ini `go build` gagal. |
| C11 | `ent` perlu langkah generate | **needs-step** | `access = ent` | Sama seperti C10: `go generate ./ent` di post-gen atau commit output. |
| C12 | mock generator perlu output di-commit | **needs-step** | `mock ∈ {uber-mock,mockery}` | File mock **di-commit** (bukan hanya directive `//go:generate`) agar `go build`/`go test` langsung jalan. |
| C13 | testcontainers WAJIB build-tag | **needs-step** | `integration = on` | File test ber-`//go:build integration`; jika tidak, `go test ./...` default gagal di mesin tanpa Docker → melanggar aturan keras. |
| C14 | SQLite tidak butuh service DB di compose | **conflicts (info)** | `db = sqlite` **dan** `docker = on` | compose **tidak** menambah service DB (SQLite = file lokal); volume disiapkan, bukan kontainer DB. |
| C15 | `mattn/go-sqlite3` bukan default (CGO) | **avoid-default** | user minta driver SQLite CGO | Bukan default; hanya opt-in eksplisit + peringatan `CGO_ENABLED=1` memecah build minimal/cross-compile. |
| C16 | Library archived dilarang jadi default | **avoid-default** | user minta `google/wire` (DI) / `golang/mock` / `AlecAivazis/survey` (builder) / `streadway/amqp` | **Ditolak sebagai default**; `wire`/`golang/mock` hanya dengan peringatan archived; DI default = manual constructor injection. |
| C17 | `auth/JWT` butuh ada HTTP handler | **requires** | `auth ∈ {jwt,paseto}` tetapi `kind = worker` (tanpa HTTP) | Middleware auth tidak relevan tanpa HTTP; untuk worker, auth diabaikan (atau token-verify util saja tanpa middleware). |
| C18 | Observability default = kelas `net/http` | **conflicts (partial)** | `obs = on` dengan `http ∈ {net/http,chi,gin,echo}` | Valid, satu set `otelhttp`. Bila `http = fiber` lihat C7 (cabang `otelfiber`). |
| C19 | DI tetap manual (zero codegen) | **info** | semua mode | Default wiring = manual constructor injection (pola bahasa Go); `uber-go/fx` hanya opt-in. Tidak ada langkah codegen DI. |
| C20 | atlas pakai Community Edition | **license** | `migrate = atlas` | Gunakan CE (Apache-2.0); binary default ber-EULA — catat di dokumentasi lisensi. MySQL driver MPL-2.0 juga dicatat. |

---

## 4. Default Behavior (Mode Non-Interaktif / Flag Tidak Lengkap)

Saat generator dijalankan non-interaktif (CI/flags) dan sebagian flag tidak diberikan, berlaku **resolusi default berikut** (urut sesuai dependensi). Semua default = paling ringan, pure-Go, zero-CGO, zero lock-in, sesuai dokumen 01–03.

| Flag | Default bila kosong | Sumber keputusan |
|---|---|---|
| `--arch` | `monolith` | [01 §4](./01-monolith.md) — layered (cmd/+internal/) sebagai monolith default |
| `--kind` (monolith/modular) | `rest` | jenis aplikasi paling umum & paling ringan |
| `--http` (rest/web) | `net/http` (stdlib) | [03 §3.1](./03-libraries.md) — zero dependency, zero lock-in |
| `--db` | `none` | minimal by default; DB hanya bila diminta |
| `--access` (bila db≠none) | `sqlx` | [03 §3.3](./03-libraries.md) — tipis di atas `database/sql`, zero magic |
| `--migrate` (bila db≠none) | `golang-migrate` | [03 §3.3](./03-libraries.md) — SQL up/down polos, zero lock-in |
| driver Postgres / MySQL / SQLite / Mongo | `pgx/v5` / `go-sql-driver/mysql` / `modernc.org/sqlite` / `mongo-driver/v2` | [03 §3.3](./03-libraries.md) — semua pure-Go |
| `--config` | `godotenv` | [03 §3.2](./03-libraries.md) — paling ringan, idiomatik |
| `--log` | `slog` (stdlib) | [03 §3.2](./03-libraries.md) — zero dependency |
| `--comm` (microservice) | `grpc` | [02 §4](./02-microservice.md) — gRPC langsung antar service; gateway opsional |
| `--broker` (microservice+event) | `nats` | [03 §3.5](./03-libraries.md) — transport ringan, mudah `docker compose up` |
| `--docker` | `on` bila `db≠none` ATAU `arch=microservice`; selain itu `off` | aturan keras `docker compose up` untuk stack ber-DB |
| `--makefile` | `on` | core add-on |
| `--ci` | `github-actions` | reproducibility & kontrak kualitas eksplisit |
| `--lint` | `on` (`.golangci.yml`) | best-practice baseline |
| `--obs` | `off` | opt-in; 95% project tak butuh sejak menit pertama ([03 §3.2](./03-libraries.md)) |
| `--auth` | `off` | opt-in; default = jwt bila diaktifkan |
| `--validate` | `on` bila ada handler input; selain itu `off` | [03 §3.4](./03-libraries.md) |
| `--mock` | `off` | opt-in; output di-commit bila on |
| `--integration` | `off` (disarankan `on` bila db≠none) | testcontainers butuh Docker → build-tag |
| `--bdd` | `off` | testing + testify lebih ringan & netral |
| DI / wiring | manual constructor injection (tetap, tidak ada flag) | [03 §3.4](./03-libraries.md) — zero lock-in absolut |

### 4.1 Aturan resolusi & validasi otomatis

1. **Resolusi berurutan:** `arch` → (`kind`|microservice-comm) → `http` → `db` → (`access`,`migrate`,driver) → add-ons → testing. Flag downstream yang tidak relevan dengan pilihan upstream **diabaikan dengan peringatan** (mis. `--broker` saat `comm=grpc`).
2. **Validasi constraint §3 dijalankan sebelum render.** Kombinasi invalid (C1–C6, C8) → **gagal cepat** dengan pesan error yang menyebut flag bermasalah; tidak menghasilkan project setengah jadi.
3. **needs-step (C10–C13) dijalankan otomatis di post-gen.** Output codegen (sqlc/ent/mock) di-commit atau di-generate sebelum verifikasi `go build`/`go test` — sehingga "build hijau tanpa edit" tetap dijamin.
4. **avoid-default (C15–C16):** library archived/CGO tidak pernah dipilih otomatis; hanya muncul bila user secara eksplisit memintanya, disertai peringatan.
5. **Profil "zero-config":** `gostarter create --name myapp` tanpa flag lain → `arch=monolith`, `kind=rest`, `http=net/http`, `db=none`, `config=godotenv`, `log=slog`, `makefile=on`, `ci=github-actions`, `lint=on`, testify baseline. Hasil: project minimal yang **langsung** lolos `go vet/build/test` tanpa dependency berat dan tanpa Docker.

---

## 5. Catatan Versi (verifikasi 2026-06-06)

Versi pin mengikuti [03-libraries.md](./03-libraries.md) §2 dan [04 §3](./04-competitors-tooling.md). Spot-check ulang pada 2026-06-06 mengonfirmasi: `bufbuild/buf` v1.70.0, `golang-migrate/migrate/v4` v4.19.1 masih rilis terbaru. **Catatan keamanan:** untuk `jackc/pgx/v5`, pin ke **patch terbaru** seri v5.x saat generate sebagai praktik keamanan rutin — generator sebaiknya menarik versi stabil terbaru, bukan mengunci mati ke satu patch lama. (Ekosistem pgx/v5 memang pernah menerbitkan advisory keamanan; lihat daftar resmi di <https://advisories.ecosyste.ms/ecosystems/go/github.com/jackc/pgx/v5> untuk rentang versi terdampak dan versi perbaikan terkini.) Semua dependency di tabel adalah library publik pihak ketiga / stdlib; **tidak satu pun** berasal dari builder `gostarter` (zero lock-in terjaga by construction).
