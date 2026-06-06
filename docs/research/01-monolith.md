# Riset Struktur Folder: Go Monolith & Modular Monolith

> Dokumen riset untuk **gostarter** (CLI generator project Go best-practice).
> Tanggal riset: **2026-06-06**.
> Lingkup: mode arsitektur (1) monolith sederhana dan (2) modular monolith.
> Aturan keras hasil generate (berlaku untuk semua kandidat di bawah):
> hasil harus lolos `go vet ./... && go build ./... && go test ./...` tanpa edit manual,
> `docker compose up` jalan untuk stack ber-DB, dan **project hasil generate TIDAK meng-import package apa pun dari builder** (zero lock-in).

---

## 1. Pendahuluan & Pertanyaan Riset

Pertanyaan paling sering ditanyakan developer Go (baru maupun berpengalaman) adalah *"bagaimana saya menyusun project ini?"*. Go sengaja **tidak** memaksakan satu layout resmi; tim inti Go hanya memberi beberapa konvensi minimal lewat compiler (mis. semantik direktori `internal/`) dan panduan di [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout). Sisanya diserahkan ke developer.

Dokumen ini menjawab pertanyaan riset berikut:

1. **Apa peran `cmd/`, `internal/`, dan `pkg/`?** — mana yang punya makna bagi compiler, mana yang hanya konvensi sosial.
2. **Di mana sebaiknya diletakkan config, migration DB, HTTP handler, dan domain logic?**
3. **Bagaimana struktur tumbuh dari kecil → besar** tanpa rewrite besar — yaitu jalur evolusi dari monolith flat → layered → modular monolith → (opsional) ekstraksi microservice.
4. **Package-by-feature vs package-by-layer** — mana default yang lebih aman untuk generator.
5. **Kapan Clean/Hexagonal Architecture cocok, kapan over-engineering** untuk monolith kecil.
6. **Bagaimana batas (boundary) antar modul** dijaga di modular monolith dan bagaimana komunikasi antar modul disiapkan agar siap diekstraksi.

### 1.1 Temuan kunci dari layout resmi Go

Dari [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout) (panduan resmi tim Go):

- **`internal/`** — satu-satunya direktori dengan **makna khusus di compiler Go**. Package di dalam `internal/` hanya boleh di-import oleh kode yang berbagi parent dari direktori `internal/` tersebut. Panduan resmi: *"it's recommended placing such packages into a directory named `internal`; this prevents other modules from depending on packages we don't necessarily want to expose"* — dan keuntungannya *"we're free to refactor its API and generally move things around without breaking external users."* Untuk **server projects**, panduan resmi eksplisit: *"keep the Go packages implementing the server's logic in the `internal` directory... keep all Go commands together in a `cmd` directory."*
- **`cmd/`** — **konvensi sosial** (bukan aturan compiler). Untuk satu command, boleh `main.go` di root. Untuk banyak command atau repo campuran (kode + non-Go), letakkan tiap entrypoint di `cmd/<nama>/main.go`. Kutipan resmi: *"A common convention is placing all commands in a repository into a `cmd` directory; while this isn't strictly necessary in a repository that consists only of commands, it's very useful in a mixed repository."*
- **`pkg/`** — **TIDAK disebut sama sekali** di panduan resmi. Ini penting: `pkg/` adalah konvensi dari komunitas (lihat §1.2), bukan rekomendasi tim Go.

> **Sumber:** [Organizing a Go module — go.dev/doc/modules/layout](https://go.dev/doc/modules/layout)

### 1.2 `golang-standards/project-layout` — kenapa BUKAN standar resmi

Repo [github.com/golang-standards/project-layout](https://github.com/golang-standards/project-layout) (54k+ bintang) sering dikira "standar resmi" karena nama organisasinya mengandung kata `golang-standards`. **Itu menyesatkan.**

- **README-nya sendiri kini memuat disclaimer eksplisit** (terverifikasi 2026-06-06): *"This is **NOT an official standard defined by the core Go dev team**. This is a set of common historical and emerging project layout patterns in the Go ecosystem."*
- **Russ Cox** (tech lead Go) mengkritik repo ini secara publik: ia menyatakan repo ini *"is not a Go standard"* dan bahkan klaim "common historical and emerging patterns" pun tidak akurat — *"the vast majority of packages in the Go ecosystem do not put the importable packages in a `pkg` subdirectory."*
- Standar minimal versi Russ Cox justru sangat sederhana: taruh `LICENSE` dan `go.mod` di root, lalu taruh kode Go di root atau susun ke pohon direktori sesukamu. *"That's it, this is the 'standard', not that complicated."*

**Implikasi untuk gostarter:** Jangan jadikan layout `pkg/`-heavy ala project-layout sebagai default. Ikuti panduan resmi `go.dev` (mulai flat, `internal/` untuk kode privat, `cmd/` saat butuh banyak entrypoint). `pkg/` hanya layak muncul kalau memang ada kode yang sengaja diniatkan untuk di-import project lain — yang untuk monolith aplikasi biasanya **tidak ada**.

> **Status maintenance (verifikasi 2026-06-06):** repo **tidak archived/deprecated**, masih ada aktivitas commit, tetapi **"No releases published"** (tidak ada rilis bertag). Tetap relevan sebagai *referensi pola*, bukan sebagai standar.
> **Sumber:** [golang-standards/project-layout](https://github.com/golang-standards/project-layout) · [Issue #117 "this is not a standard Go project layout"](https://github.com/golang-standards/project-layout/issues/117) · [HN diskusi Russ Cox](https://news.ycombinator.com/item?id=26967105)

### 1.3 Package-by-feature vs package-by-layer

Dua mazhab utama menyusun package:

- **Package-by-layer** — kelompokkan berdasarkan peran teknis: `handlers/`, `services/`, `repositories/`, `models/`. Familiar bagi pendatang dari Java/Spring, tapi membuat satu fitur tersebar di banyak package dan mendorong package gemuk yang saling tergantung.
- **Package-by-feature** — kelompokkan berdasarkan domain/fitur: `user/`, `order/`, `payment/`, masing-masing berisi handler + service + repository fitur tersebut. Lebih selaras dengan filosofi Go.

Talk klasik **Kat Zien — "How Do You Structure Your Go Apps?"** (GopherCon UK 2018) membahas spektrum: *flat structure → group-by-layer → group-by-module/feature → Domain-Driven Design → Hexagonal*. Prinsip utamanya: **struktur project harus mencerminkan cara kerja software**, konsisten, mudah dipahami, dinavigasi, dan dites.

**Ben Johnson — "Standard Package Layout"** (gobeyond.dev) memberi prinsip kunci yang sangat "Go": *"packages are layers, not groups"*. Karena Go melarang dependency melingkar, ia menyarankan **domain types diletakkan di root package** (tipe data sederhana + interface kontrak antar-layer, tanpa dependency pihak ketiga), lalu implementasi (DB, HTTP) jadi sub-package yang **semuanya bergantung ke root domain** — bukan sebaliknya. Ini meratakan hierarki dan mencegah package gemuk seperti `models` atau `utils`.

> **Sumber:** [Kat Zien — How Do You Structure Your Go Apps? (GopherCon UK 2018, YouTube)](https://www.youtube.com/watch?v=VQym87o91f8) · [JetBrains GoLand — Catching Up With Kat Zien (2023)](https://blog.jetbrains.com/go/2023/04/11/catching-up-with-kat-zien-on-the-structure-of-go-apps-in-2023/) · [Ben Johnson — Standard Package Layout](https://www.gobeyond.dev/standard-package-layout/) · [Ben Johnson — Packages as layers, not groups](https://www.gobeyond.dev/packages-as-layers/)

**Keputusan untuk gostarter:** untuk monolith sangat sederhana, **flat** sudah cukup. Begitu ada >1 domain, **default ke package-by-feature** (selaras Go, mudah jadi cikal-bakal modul). Package-by-layer murni hanya disediakan sebagai varian "layered/standar" karena familiaritasnya, bukan default yang dianjurkan untuk skala besar.

### 1.4 Clean / Hexagonal Architecture: kapan cocok, kapan over-engineering

Clean/Hexagonal (ports & adapters) **berguna** ketika: logika bisnis cukup kaya, butuh testability tinggi tanpa DB/HTTP nyata, beberapa orang bekerja paralel, atau ingin menghindari framework lock-in. Three Dots Labs merekomendasikan 4 layer (Domain → Application → Ports → Adapters) dengan aturan *"outer layers can refer to inner layers, but not vice versa"* — **tapi** mereka eksplisit pragmatis: *"apply Clean Architecture where it makes sense"*, dan mereka sendiri **tidak** merefaktor service kecil (`users`) karena nyaris tak ada application logic.

Peringatan komunitas (2024–2026) konsisten: banyak project Go "clean architecture" jadi *over-engineered* — *"15 layers of abstraction for a CRUD API"*, atau *"Java with worse syntax"* (controllers + services + repositories + DTOs + mappers + validators + factories untuk CRUD sepele). Hexagonal **bukan** soal "20 direktori dan 50 interface", melainkan menjaga business logic tetap murni dan testable.

**Keputusan untuk gostarter:**
- **Monolith sederhana** → JANGAN paksakan Clean Architecture penuh. Cukup pemisahan ringan (transport ↔ service ↔ storage) tanpa berlapis-lapis interface. Biarkan kompleksitas yang mendorong arsitektur, bukan sebaliknya.
- **Modular monolith** → terapkan **batas via interface per modul** (esensi hexagonal) **secukupnya**, hindari berlapis-lapis abstraksi spekulatif.

> **Sumber:** [Three Dots Labs — Introducing Clean Architecture](https://threedots.tech/post/introducing-clean-architecture/) · [Why Clean Architecture Struggles in Golang (DEV)](https://dev.to/lucasdeataides/why-clean-architecture-struggles-in-golang-and-what-works-better-m4g) · [Hexagonal Architecture in Go — Serge Skoredin](https://skoredin.pro/blog/golang/hexagonal-architecture-go)

### 1.5 Modular monolith: boundary & kesiapan ekstraksi

Modular monolith = deployment tunggal (seperti monolith) + modularitas (seperti microservice): aplikasi dipecah jadi **modul domain yang loosely coupled** dalam satu codebase, tiap modul punya batas jelas, **public interface minimal**, dan **tidak membocorkan internal-nya**.

Pola Go yang konsisten dari sumber 2024–2026:
- **Batas dipaksakan lewat `internal/` per modul** — compiler menolak import lintas-`internal/`, jadi modul lain tak bisa menyentuh internal modul tetangga (boundary "berduri", bukan sekadar konvensi).
- **Komunikasi antar modul lewat interface kecil dan/atau in-process event bus** — bukan memanggil struct konkret modul lain. Ini yang membuat ekstraksi nanti jadi "operasi bedah" yang rapi.
- **Composition root tipis** (di `cmd/` atau `internal/app`) yang merakit semua modul.
- **Jalur ekstraksi ke microservice:** karena cross-module sudah lewat interface dan domain event sudah ada, mengubah event in-process → publish ke message broker, dan memindah satu modul ke service terpisah, jadi perubahan terlokalisasi.

Referensi konkret: **powerman/go-monolith-example** — "monolith with embedded microservices" + Clean Architecture, modul tersusun agar *"can be easily extracted from monolith into separate projects"*, tiap modul punya CLI subcommand, DB migration, port, dan metric sendiri, serta **menghindari global object** agar modul tak saling bentrok.

> **Status maintenance powerman/go-monolith-example (verifikasi 2026-06-06):** rilis terakhir **v0.5.2, 29 Mei 2021**; README menargetkan **Go 1.16**; **tidak archived/deprecated**. → Gunakan sebagai **referensi pola/konsep** (boundary `internal/`, embedded microservice, ekstraksi), **bukan** sebagai dependency atau template yang di-copy mentah, karena sudah agak lama (toolchain Go saat ini jauh lebih baru). Pola intinya masih valid dan banyak dipakai ulang di artikel 2024–2026.
> **Sumber:** [powerman/go-monolith-example](https://github.com/powerman/go-monolith-example) · [Designing a Modular Monolith in Go — daveamit.com (2026)](https://daveamit.com/posts/2026-02-13-modular-monolith/) · [Modular Monolith Pattern (DEV)](https://dev.to/shieldstring/modular-monolith-pattern-building-scalable-systems-without-microservice-overhead-1gol)

### 1.6 Di mana config, migration, handler, domain diletakkan? (ringkas)

| Artefak | Lokasi yang dianjurkan | Alasan |
|---|---|---|
| Entrypoint (`main`) | `cmd/<app>/main.go` (atau root `main.go` untuk satu command) | Konvensi resmi `cmd/`; tipis, hanya wiring |
| Config loader | `internal/config` (atau `config/` untuk file `.yaml/.env`) | Privat ke aplikasi; tak perlu diekspos |
| Migration DB | `migrations/` (root, file `.sql`) | Dipakai tool migrasi & `docker compose`; bukan kode aplikasi |
| HTTP/gRPC handler | `internal/<feature>/transport` atau `internal/http` | Adapter/port; bergantung ke domain, bukan sebaliknya |
| Domain logic & types | root package (cara Ben Johnson) atau `internal/<feature>` | Inti bisnis; tanpa dependency infra |
| Storage/repository | `internal/<feature>/storage` (impl interface domain) | Adapter; mudah di-mock saat test |
| Kode yang sengaja dibagikan ke luar | `pkg/` (HANYA jika benar-benar perlu) | Konvensi komunitas; default-nya kosong untuk aplikasi |

---

## 2. Kandidat Struktur (≥3) dengan Tree Lengkap

### Kandidat A — Monolith **sangat sederhana** (flat / near-flat)

Cocok untuk: CRUD kecil, prototipe, satu domain, satu entrypoint. Sesuai standar minimal Russ Cox & panduan "mulai dari satu package".

```
myapp/
├── go.mod
├── go.sum
├── main.go                  # entrypoint; wiring HTTP + DB + handler
├── handler.go               # HTTP handler (1 file, satu package main)
├── store.go                 # akses DB (interface + impl sederhana)
├── model.go                 # domain types (struct + sedikit interface)
├── handler_test.go
├── store_test.go
├── config/
│   └── config.yaml          # contoh config (di-load via env override)
├── migrations/
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── docker-compose.yml       # app + postgres (untuk stack ber-DB)
├── Dockerfile
├── Makefile                 # vet/build/test/migrate targets
└── README.md
```

> Catatan: jika belum butuh DB, `migrations/`, `docker-compose.yml`, dan `store.go` di-skip oleh generator. Tetap lolos `go vet/build/test`.

### Kandidat B — **Layered / standar** (server project, `cmd/` + `internal/`)

Cocok untuk: aplikasi yang mulai tumbuh, 1 tim, butuh pemisahan transport/service/storage tapi belum perlu modularisasi domain penuh. Mengikuti rekomendasi resmi server project (`cmd/` + `internal/`), package-by-feature di dalam `internal/`.

```
myapp/
├── go.mod
├── go.sum
├── cmd/
│   └── api/
│       └── main.go              # entrypoint tipis; panggil internal/app
├── internal/
│   ├── app/
│   │   └── app.go               # composition root: rakit config+db+router
│   ├── config/
│   │   └── config.go            # load env/file ke struct
│   ├── http/
│   │   ├── router.go            # routing + middleware
│   │   └── middleware.go
│   ├── user/                    # package-by-feature
│   │   ├── service.go           # business logic
│   │   ├── handler.go           # HTTP handler fitur user
│   │   ├── repository.go        # interface + impl storage
│   │   └── service_test.go
│   └── platform/
│       └── database/
│           └── postgres.go      # koneksi DB, shared infra
├── migrations/
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── config/
│   └── config.yaml
├── docker-compose.yml
├── Dockerfile
├── Makefile
└── README.md
```

> `pkg/` SENGAJA tidak ada (tak ada kode yang diniatkan diekspor ke luar). Sesuai kritik Russ Cox.

### Kandidat C — **Modular monolith** siap-ekstraksi

Cocok untuk: banyak domain, beberapa orang/sub-tim, antisipasi ekstraksi sebagian modul ke microservice. Tiap modul punya `internal/`-nya sendiri (boundary "berduri"), komunikasi lintas modul lewat **interface/contract** + **in-process event bus**.

```
myapp/
├── go.mod
├── go.sum
├── cmd/
│   └── monolith/
│       └── main.go                  # composition root tunggal: daftarkan semua modul
├── internal/
│   ├── platform/                    # infra lintas-modul (shared, privat)
│   │   ├── config/
│   │   │   └── config.go
│   │   ├── database/
│   │   │   └── postgres.go
│   │   └── eventbus/
│   │       └── bus.go               # in-process event bus (pub/sub)
│   ├── shared/
│   │   └── contract/
│   │       ├── user.go              # interface publik modul user (yg boleh dipakai modul lain)
│   │       └── events.go            # definisi domain event lintas modul
│   └── modules/
│       ├── user/
│       │   ├── module.go            # init modul: implement contract.UserService, register routes/handlers
│       │   ├── internal/            # BERDURI: tak bisa di-import modul lain
│       │   │   ├── service.go
│       │   │   ├── repository.go
│       │   │   ├── handler.go
│       │   │   └── service_test.go
│       │   └── migrations/
│       │       ├── 0001_user.up.sql
│       │       └── 0001_user.down.sql
│       └── order/
│           ├── module.go            # panggil user via contract.UserService (interface), bukan struct konkret
│           ├── internal/
│           │   ├── service.go
│           │   ├── repository.go
│           │   ├── handler.go
│           │   └── service_test.go
│           └── migrations/
│               ├── 0001_order.up.sql
│               └── 0001_order.down.sql
├── migrations/                      # opsional: agregat/registry migrasi global
├── config/
│   └── config.yaml
├── docker-compose.yml
├── Dockerfile
├── Makefile
└── README.md
```

**Mengapa siap ekstraksi:**
1. **`internal/` per modul** memaksa isolasi — `order` tak bisa menyentuh `user/internal/*`, hanya `shared/contract`.
2. **Komunikasi lewat interface** (`contract.UserService`) + **event bus in-process** → saat ekstraksi, ganti impl interface dengan klien gRPC/HTTP dan ganti event bus dengan message broker; isi `internal/` modul dipindah apa adanya jadi service baru.
3. **Migration & (opsional) subcommand per modul** mengikuti pola powerman/go-monolith-example, sehingga satu modul punya semua yang dibutuhkan untuk berdiri sendiri.

---

## 3. Tabel Trade-off Antar Kandidat

| Kriteria | A — Flat sederhana | B — Layered/standar | C — Modular monolith |
|---|---|---|---|
| **Kesederhanaan** | ★★★★★ Paling mudah dibaca; nyaris tanpa boilerplate | ★★★★ Masih ringkas; satu konsep "fitur" | ★★ Paling banyak file/konsep (event bus, contract, module init) |
| **Skalabilitas tim** | ★ Konflik tinggi bila >2–3 orang; semua di satu package | ★★★ Fitur terpisah, beberapa orang bisa paralel | ★★★★★ Boundary `internal/` berduri → sub-tim per modul tanpa saling injak |
| **Testability** | ★★★ Bisa, tapi mudah tergoda mock DB nyata | ★★★★ Interface repository per fitur → unit test mudah | ★★★★★ Service murni di balik interface + event bus mockable |
| **Kesiapan migrasi ke microservice** | ★ Rendah; perlu refactor besar | ★★★ Sedang; fitur sudah terpisah tapi belum ada kontrak/eventing lintas-modul | ★★★★★ Tinggi; tinggal swap interface→RPC & eventbus→broker, pindahkan `internal/` modul |

> Skala: ★ (lemah) – ★★★★★ (kuat). Trade-off inti: **kesederhanaan berbanding terbalik dengan kesiapan ekstraksi**. Pilih sesuai ukuran tim & horizon pertumbuhan, jangan bayar ongkos C kalau kebutuhanmu A.

---

## 4. Rekomendasi DEFAULT

### 4.1 Default untuk mode **"Monolith sederhana"** → **Kandidat B (Layered/standar)**

Alasan memilih B (bukan A) sebagai *default* mode "monolith sederhana": A terlalu mudah berubah jadi berantakan begitu tumbuh, sedangkan B sudah memberi kerangka tumbuh (cmd/ + internal/ + package-by-feature) **tanpa** ongkos modular monolith. Generator tetap boleh menawarkan varian "flat" (A) untuk kasus paling minimal.

**Alasan per-folder (Kandidat B):**

| Folder | Alasan | Sumber |
|---|---|---|
| `cmd/api/main.go` | Konvensi resmi untuk entrypoint; tipis, hanya wiring → mudah tambah command lain nanti (`cmd/worker`, `cmd/migrate`) tanpa rombak | [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout) |
| `internal/` | Memaksa privasi di level compiler; bebas refactor API internal tanpa memikirkan importer luar — sesuai rekomendasi resmi untuk **server projects** | [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout) |
| `internal/app/app.go` | Composition root tunggal → satu tempat merakit dependency; `main.go` tetap bersih | [Three Dots Labs](https://threedots.tech/post/introducing-clean-architecture/) |
| `internal/config` | Config privat aplikasi; tak perlu diekspos ke modul lain | [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout) |
| `internal/<feature>/` (mis. `user/`) | **Package-by-feature** → satu fitur kohesif (service+handler+repo) di satu tempat; selaras filosofi Go ("packages are layers") | [Kat Zien talk](https://www.youtube.com/watch?v=VQym87o91f8) · [Ben Johnson](https://www.gobeyond.dev/standard-package-layout/) |
| `internal/http` | Adapter transport terpisah dari logic → handler tipis, service testable | [Three Dots Labs](https://threedots.tech/post/introducing-clean-architecture/) |
| `internal/platform/database` | Infra DB shared di satu tempat → reuse antar fitur | pola umum modular monolith Go ([daveamit.com](https://daveamit.com/posts/2026-02-13-modular-monolith/)) |
| `migrations/` (root, `.sql`) | Dipakai tool migrasi & `docker compose`; bukan kode aplikasi → terpisah dari `internal/` | konvensi ekosistem (mis. golang-migrate) |
| **TANPA `pkg/`** | Tidak ada kode yang diniatkan untuk di-import project lain; menambah `pkg/` di aplikasi justru anti-pola menurut kritik Russ Cox | [HN / Issue #117](https://github.com/golang-standards/project-layout/issues/117) |

### 4.2 Default untuk mode **"Modular monolith"** → **Kandidat C**

**Alasan per-folder (Kandidat C):**

| Folder | Alasan | Sumber |
|---|---|---|
| `cmd/monolith/main.go` | Composition root tunggal yang mendaftarkan semua modul → satu binary, satu deploy (esensi monolith) | [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout) |
| `internal/modules/<m>/internal/` | **Boundary berduri**: compiler menolak import lintas-`internal/`, jadi modul lain tak bisa mengakses isi modul tetangga → isolasi nyata, bukan sekadar janji | [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout) · [powerman](https://github.com/powerman/go-monolith-example) |
| `internal/shared/contract/` | Interface publik antar-modul + definisi event → modul berkomunikasi via kontrak, bukan struct konkret → siap di-swap jadi RPC saat ekstraksi | [daveamit.com (2026)](https://daveamit.com/posts/2026-02-13-modular-monolith/) · [powerman](https://github.com/powerman/go-monolith-example) |
| `internal/platform/eventbus/` | In-process event bus → komunikasi async antar modul tanpa coupling; saat ekstraksi tinggal ganti ke message broker | [Modular Monolith Pattern (DEV)](https://dev.to/shieldstring/modular-monolith-pattern-building-scalable-systems-without-microservice-overhead-1gol) |
| `internal/modules/<m>/module.go` | Entry tiap modul: implement contract, register route/handler, hindari global object → modul mandiri & tak bentrok | [powerman](https://github.com/powerman/go-monolith-example) |
| `internal/modules/<m>/migrations/` | Migrasi per-modul → satu modul membawa skema-nya sendiri → ekstraksi = pindahkan folder modul utuh | [powerman](https://github.com/powerman/go-monolith-example) |
| `internal/platform/` (config, database) | Infra lintas-modul yang sengaja dibagikan, tetap privat ke aplikasi | pola umum ([daveamit.com](https://daveamit.com/posts/2026-02-13-modular-monolith/)) |
| **TANPA `pkg/`** | Sama seperti B: aplikasi tidak mengekspor library; `pkg/` hanya menambah noise | [Russ Cox / Issue #117](https://github.com/golang-standards/project-layout/issues/117) |

**Aturan komunikasi antar modul (wajib di-generate sebagai contoh minimal):**
- Modul **A → B** hanya boleh lewat `shared/contract` (interface) **atau** event bus. **Dilarang** import `modules/B/internal/*` (dan compiler memang menolaknya).
- Composition root (`cmd/monolith/main.go`) yang meng-inject impl konkret ke interface → satu-satunya tempat modul "saling kenal".

---

## 5. Daftar Sumber

**Layout resmi & standar minimal Go**
- Organizing a Go module (panduan resmi tim Go) — https://go.dev/doc/modules/layout
- `cmd/go` (Go Packages) — https://pkg.go.dev/cmd/go

**Debat `golang-standards/project-layout`**
- golang-standards/project-layout — https://github.com/golang-standards/project-layout
- Issue #117 "this is not a standard Go project layout" — https://github.com/golang-standards/project-layout/issues/117
- Issue #185 "rename with a note in the README" — https://github.com/golang-standards/project-layout/issues/185
- Hacker News — kritik Russ Cox — https://news.ycombinator.com/item?id=26967105

**Package-by-feature vs package-by-layer**
- Kat Zien — How Do You Structure Your Go Apps? (GopherCon UK 2018) — https://www.youtube.com/watch?v=VQym87o91f8
- JetBrains GoLand — Catching Up With Kat Zien on the Structure of Go Apps (2023) — https://blog.jetbrains.com/go/2023/04/11/catching-up-with-kat-zien-on-the-structure-of-go-apps-in-2023/
- Ben Johnson — Standard Package Layout — https://www.gobeyond.dev/standard-package-layout/
- Ben Johnson — Packages as layers, not groups — https://www.gobeyond.dev/packages-as-layers/

**Clean / Hexagonal Architecture di Go**
- Three Dots Labs — Introducing Clean Architecture — https://threedots.tech/post/introducing-clean-architecture/
- Why Clean Architecture Struggles in Golang (DEV) — https://dev.to/lucasdeataides/why-clean-architecture-struggles-in-golang-and-what-works-better-m4g
- Hexagonal Architecture in Go — Serge Skoredin — https://skoredin.pro/blog/golang/hexagonal-architecture-go

**Modular monolith & ekstraksi microservice**
- powerman/go-monolith-example — https://github.com/powerman/go-monolith-example
- Designing a Modular Monolith in Go (2026) — https://daveamit.com/posts/2026-02-13-modular-monolith/
- Modular Monolith Pattern (DEV) — https://dev.to/shieldstring/modular-monolith-pattern-building-scalable-systems-without-microservice-overhead-1gol

> **Catatan maintenance (verifikasi 2026-06-06):**
> - `powerman/go-monolith-example` — rilis terakhir **v0.5.2 (29 Mei 2021)**, target **Go 1.16**, **tidak archived**. Gunakan sebagai referensi POLA, bukan template di-copy mentah (toolchain usang).
> - `golang-standards/project-layout` — **tidak archived**, **tanpa rilis bertag**, README kini eksplisit menyatakan **bukan standar resmi**. Referensi pola saja.
> - `go.dev/doc/modules/layout`, Three Dots Labs, gobeyond.dev — sumber hidup/aktif, dipakai sebagai dasar default.
