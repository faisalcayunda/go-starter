# SPEC — gostarter

> **Versi SPEC:** 0.1 (Draft Fase 1)
> **Tanggal:** 2026-06-06
> **Status:** Draft
> **Owner:** isal
>
> **Sumber kebenaran:** `docs/research/01-monolith.md`, `02-microservice.md`, `03-libraries.md`, `04-competitors-tooling.md`, `05-decision-matrix.md` + `docs/adr/ADR-001-builder-stack.md` (hasil Fase 0). Tidak ada library/opsi yang ditambahkan di luar dokumen tersebut; detail versi/status mengikuti dokumen itu.
> **Invarian lintas-bagian:** setiap opsi pertanyaan interaktif (§4) punya flag ekuivalen 1:1 (§5); setiap aturan constraint (§6) merujuk id pertanyaan (§4) / nama flag (§5) yang nyata. Jawaban interaktif dan flag setara menghasilkan output **byte-identical** (lihat §5.2).

---

## Daftar Isi

- [1. Pendahuluan & Diferensiator](#1-pendahuluan--diferensiator)
  - [1.1 Masalah yang Dipecahkan](#11-masalah-yang-dipecahkan)
  - [1.2 Untuk Siapa](#12-untuk-siapa)
  - [1.3 Diferensiator Utama](#13-diferensiator-utama)
  - [1.4 Tiga Arsitektur yang Didukung](#14-tiga-arsitektur-yang-didukung)
- [2. Platform & Distribusi](#2-platform--distribusi)
  - [2.1 Platform Target v1](#21-platform-target-v1)
  - [2.2 Distribusi](#22-distribusi)
- [3. Glossary](#3-glossary)
- [4. Question Flow Interaktif](#4-question-flow-interaktif)
  - [4.1 Konvensi Tabel](#41-konvensi-tabel)
  - [4.2 q_name — Nama Project & Module Path](#42-q_name--nama-project--module-path)
  - [4.3 q_arch — Tipe Arsitektur](#43-q_arch--tipe-arsitektur)
  - [4.4 q_kind — Jenis Aplikasi](#44-q_kind--jenis-aplikasi)
  - [4.5 q_svc — Service Awal + Komunikasi + Gateway](#45-q_svc--service-awal--komunikasi--gateway)
  - [4.6 q_http — HTTP Framework](#46-q_http--http-framework)
  - [4.7 q_db — Database](#47-q_db--database)
  - [4.8 q_addons — Add-ons](#48-q_addons--add-ons)
  - [4.9 q_git — git init + initial commit](#49-q_git--git-init--initial-commit)
  - [4.10 Diagram Alur](#410-diagram-alur)
- [5. Mode Non-Interaktif (Flags)](#5-mode-non-interaktif-flags)
  - [5.1 Tabel Ekuivalensi Flag](#51-tabel-ekuivalensi-flag)
  - [5.2 Invarian Byte-Identical](#52-invarian-byte-identical)
  - [5.3 Contoh Perintah Lengkap](#53-contoh-perintah-lengkap)
  - [5.4 Meta-flag](#54-meta-flag)
- [6. Constraint Matrix & Default Resolution](#6-constraint-matrix--default-resolution)
  - [6.1 Tabel Kombinasi VALID / INVALID](#61-tabel-kombinasi-valid--invalid)
  - [6.2 Default Resolution](#62-default-resolution)
  - [6.3 Profil Zero-Config](#63-profil-zero-config)
  - [6.4 Aturan Resolusi Konflik](#64-aturan-resolusi-konflik)
  - [6.5 Catatan Penyelarasan Spesifikasi](#65-catatan-penyelarasan-spesifikasi)
- [7. User Stories & Acceptance Criteria](#7-user-stories--acceptance-criteria)
  - [US-01 — Generate monolith REST via wizard interaktif](#us-01--generate-monolith-rest-via-wizard-interaktif)
  - [US-02 — Generate monolith via flags identik dengan interaktif](#us-02--generate-monolith-via-flags-identik-dengan-interaktif)
  - [US-03 — Generate modular-monolith](#us-03--generate-modular-monolith)
  - [US-04 — Generate microservice](#us-04--generate-microservice)
  - [US-05 — gostarter add service ke project existing](#us-05--gostarter-add-service-ke-project-existing)
  - [US-06 — --dry-run + proteksi overwrite](#us-06----dry-run--proteksi-overwrite)
  - [US-07 — Stack ber-DB → docker compose up jalan](#us-07--stack-ber-db--docker-compose-up-jalan)
  - [Definition of Done (Global)](#definition-of-done-global)
- [8. Non-Goals (Fase Implementasi)](#8-non-goals-fase-implementasi)

---

## 1. Pendahuluan & Diferensiator

**gostarter** adalah CLI berbasis Go yang men-generate struktur project Go best-practice dalam hitungan detik. Pengalaman penggunaannya sengaja meniru `laravel new` / `composer create-project`: satu perintah, sebuah wizard ringkas (atau flag lengkap), lalu sebuah project yang **langsung jalan** — tanpa editing manual, tanpa "TODO: isi sendiri".

### 1.1 Masalah yang Dipecahkan

Go sengaja **tidak** memaksakan satu layout project resmi (lihat 01 §1, `go.dev/doc/modules/layout`). Konsekuensinya, setiap developer Go — baru maupun berpengalaman — berulang kali menjawab pertanyaan yang sama: *"bagaimana saya menyusun project ini?"*, lalu menghabiskan jam-jam awal untuk menata `cmd/`, `internal/`, wiring config, koneksi DB, migration, Dockerfile, dan CI dari nol. Generator yang ada saat ini hanya menyelesaikan sebagian masalah:

- **Mengunci satu paradigma layout.** go-blueprint / go-template / cgapp hanya menghasilkan satu service flat; goctl / Kratos hanya per-service microservice dengan **lock-in** ke framework masing-masing (04 §2.3).
- **Tidak ada modular monolith first-class**, dan tidak ada **multi-service dalam satu project** ataupun penambahan service inkremental (04 §2.3 Gap Analysis).
- **Tidak menjamin kualitas hasil secara eksplisit** — "build hijau" diperlakukan implisit; cgapp bahkan berisiko basi (stagnan sejak Agustus 2023, 04 §2.1).

gostarter menutup celah itu: satu tool yang men-generate project siap-pakai untuk **tiga paradigma arsitektur**, dengan kontrak kualitas yang ditegakkan dan **zero lock-in** sebagai aturan keras.

### 1.2 Untuk Siapa

Proyek **open-source publik** (kandidat lisensi MIT / Apache-2.0, difinalkan di Fase 6). Target pengguna: developer dan tim Go yang ingin memulai project baru dengan struktur best-practice tanpa berdebat layout; pengajar/onboarding yang butuh starter konsisten; serta tim yang menjalankan generate di CI (mode non-interaktif/reproducible). Distribusi via `go install` & goreleaser, dilengkapi dokumentasi publik dan shell completion.

### 1.3 Diferensiator Utama

Tidak ada satu pun kompetitor (go-blueprint, go-template, cgapp, goctl, Kratos CLI, cookiecutter) yang menawarkan kombinasi berikut dalam satu generator (04 §2.3, §4.1):

1. **Tiga paradigma arsitektur dalam satu tool** — monolith sederhana, modular monolith, dan microservice — dipilih saat generate, bukan dikunci di template.
2. **`add service` inkremental** — `gostarter add service <name>` menambah service baru ke project microservice yang sudah ada (diferensiator kunci; goctl/Kratos hanya men-generate satu service per invocation).
3. **Modular monolith sebagai mode pertama-kelas** — boundary domain dipaksakan lewat `internal/` per modul + komunikasi via contract/event bus (01 §1.5, §4.2) — celah pasar yang benar-benar kosong.
4. **Kontrak build hijau yang eksplisit & ditegakkan** — setiap hasil generate wajib lolos `go vet ./... && go build ./... && go test ./...` **tanpa edit manual**, `docker compose up` jalan untuk stack ber-DB, dan project hasil generate **tidak meng-import apa pun dari builder** (zero lock-in). Untuk microservice, stub proto di `gen/go/` **di-commit** sehingga build hijau tanpa perlu menjalankan `buf generate` (02 §2, 04 §1).

### 1.4 Tiga Arsitektur yang Didukung

gostarter v1 mendukung tiga mode arsitektur, dipilih lewat pertanyaan `q_arch` (interaktif) / flag `--arch` (non-interaktif). Default = `monolith`.

#### 1.4.1 Monolith sederhana (`monolith`)

Satu unit deploy, satu binary. Default layout = **layered/standar** (`cmd/<app>/main.go` + `internal/` dengan package-by-feature), yang memberi kerangka tumbuh tanpa ongkos modularisasi penuh; varian **flat** tetap tersedia untuk kasus paling minimal (01 §4.1, Kandidat B/A). Cocok untuk CRUD, prototipe, atau aplikasi satu tim yang belum perlu memecah domain. Jenis aplikasi dipilih via `q_kind`: `rest` (default), `web`, atau `worker` (01 §2).

#### 1.4.2 Modular monolith (`modular-monolith`)

Tetap satu unit deploy & satu binary (seperti monolith), tetapi aplikasi dipecah menjadi **modul domain yang loosely coupled** dengan boundary "berduri": tiap modul punya `internal/`-nya sendiri sehingga compiler menolak import lintas-modul, dan komunikasi antar modul hanya lewat **interface/contract** + **in-process event bus** (01 §1.5, §4.2, Kandidat C). Desain ini sengaja **siap-ekstraksi**: saat satu modul perlu menjadi microservice, interface tinggal di-swap ke klien gRPC dan event bus ke message broker. Cocok untuk banyak domain dan beberapa sub-tim.

#### 1.4.3 Microservice (`microservice`)

Menghasilkan **N service sekaligus** dalam layout **monorepo single-module** (per-repo = roadmap v2): pemisahan jelas antara unit deploy (`services/`), kode bersama (`libs/`), kontrak (`proto/` + `gen/`), dan edge (`gateway/`, OFF by default) — satu `go.mod` di root (02 §2 Kandidat B, kandidat utama). Komunikasi antar service default **gRPC** (opsi `rest` & `event`; jika event → pilih broker NATS default / Kafka / RabbitMQ). `docker compose up` menjalankan semua service + DB; contoh minimal mencakup **2 service yang saling memanggil**, dengan stub proto `gen/go/` **di-commit** agar build hijau tanpa `buf generate` (02 §1, §2).

---

## 2. Platform & Distribusi

### 2.1 Platform Target v1

- **Linux & macOS** — didukung penuh di v1.
- **Windows** — **roadmap** (belum v1). Path handling spesifik-Windows **ditangguhkan** secara sengaja; spesifikasi v1 mengasumsikan separator/semantik path POSIX.

### 2.2 Distribusi

- **`go install`** — instalasi langsung dari source (`go install <module>/cmd/gostarter@latest`).
- **goreleaser** — rilis binary lintas-platform (Linux/macOS v1) untuk distribusi publik.
- **Shell completion** — di-generate otomatis oleh command framework (`spf13/cobra`, ADR-001 §1), tersedia untuk shell yang didukung cobra.
- **Dokumentasi publik** — menyertai rilis open-source.

---

## 3. Glossary

| Istilah | Definisi (dalam konteks gostarter) |
|---|---|
| **Builder** | Tool gostarter itu sendiri — program yang men-generate project. Memakai stack `cobra` + `huh`/`bubbletea`/`lipgloss` + `text/template`/`embed.FS` + `x/mod/modfile` (ADR-001). |
| **Project hasil generate** | Output gostarter. **Tidak** meng-import satu pun dependency builder (zero lock-in). |
| **Go module path** | Identitas modul Go (mis. `github.com/acme/shop`), ditulis di `go.mod`. Divalidasi mengikuti semantik `golang.org/x/mod/module` (lihat §4.2 `q_name`). |
| **Modul template** | Potongan template builder per-dimensi (arsitektur × framework × DB × add-on) yang dikomposisikan menjadi satu project ("composition by category", 04 §2.2). Di-embed ke binary via `embed.FS`. |
| **Manifest modul** | Metadata yang mendeskripsikan sebuah modul template (dimensi, file yang dihasilkan, dependency `go.mod` yang ditambahkan, depends-on/constraint). Dipakai generator untuk merakit & memvalidasi kombinasi terhadap constraint matrix (§6). |
| **Modul domain** (modular monolith) | Unit domain dalam project modular monolith (mis. `user/`, `order/`) dengan `internal/`-nya sendiri (boundary berduri) — berbeda dari "modul template". |
| **Service** (microservice) | Unit deploy dalam monorepo microservice, hidup di `services/<svc>/` di bawah satu module root. |
| **Composition root** | Satu tempat di mana semua dependency dirakit (impl konkret di-inject ke interface). Monolith: `internal/app/app.go`; modular monolith: `cmd/monolith/main.go`; setiap service punya composition root tipis di `cmd/main.go` (01 §4, 02 §2). |
| **Add-on** | Komponen opsional multi-select non-arsitektural: Docker+compose, Makefile/Taskfile, CI, golangci-lint, observability (otel+health), auth scaffold (JWT), `.env.example` (§4.8 `q_addons`). |
| **Dry-run** | Mode `--dry-run`: validasi seluruh input + jalankan constraint matrix + cetak rencana file/folder & dependency `go.mod` **tanpa menulis ke disk**; exit non-zero bila kombinasi invalid (§5.4). |
| **Preset** | File konfigurasi (`--config <file.yaml>`) yang memuat seluruh jawaban; menjalankan generate non-interaktif & reproducible. Precedence: flag eksplisit > preset > default (§5.4, §6.4). |
| **Constraint matrix** | Tabel aturan VALID/INVALID antar-opsi (requires / conflicts / needs-step / avoid-default), diturunkan dari 05 §3 (§6). |
| **needs-step** | Aturan yang menuntut langkah pasca-generate (commit/generate output codegen, mis. `sqlc`, `ent`, mock) sebelum verifikasi build, agar kontrak build hijau terjaga (§6.1: C10–C13). |
| **Mode interaktif / non-interaktif** | Interaktif = wizard `huh`; non-interaktif = flag `cobra`/preset. Keduanya menulis ke satu struct konfigurasi internal yang sama sehingga output **byte-identical** (§5.2). |
| **Zero lock-in** | Aturan keras: project hasil generate tidak boleh meng-import package apa pun dari builder (04 §1). |
| **gen/ (committed stubs)** | Direktori `gen/go/` berisi kode hasil `buf generate` yang **di-commit** ke project microservice, agar build hijau tanpa menjalankan `buf generate` (02 §2). |

---

## 4. Question Flow Interaktif

Wizard `gostarter create` dibangun di atas `charmbracelet/huh` (ADR-001 §2). Pertanyaan diajukan **berurutan**; setiap pertanyaan punya **depends-on** yang menentukan kapan ia muncul. Jika depends-on tidak terpenuhi, pertanyaan **di-skip** (bukan ditampilkan disabled).

### 4.1 Konvensi Tabel

- **id** = identitas stabil pertanyaan (dipakai juga sebagai jangkar di §5 & §6).
- **widget** = `input` / `select` / `multiselect` / `confirm` (tipe `huh`).
- **default** = nilai bila user menekan Enter / tidak mengubah.
- **depends-on** = kondisi kemunculan (rujuk id + nilai pertanyaan sebelumnya).

### 4.2 q_name — Nama Project & Module Path

| Field | Nilai |
|---|---|
| **id** | `q_name` (dua sub-field: `name` + `module`) |
| **prompt** | "Nama project?" lalu "Go module path?" |
| **widget** | `input` (×2) |
| **opsi/default** | `name`: wajib diisi (tanpa default). `module`: default = `github.com/<name>` (di-prefill dari `name`, bisa diedit) |
| **depends-on** | — (selalu pertama) |
| **validasi `name`** | non-empty; cocok regex `^[a-z][a-z0-9-]*$` (huruf kecil, angka, dash; awali huruf). Dipakai sebagai nama folder root & default segment terakhir module. |
| **validasi `module`** | Lihat **Aturan Validasi Module Path** di bawah. |

**Aturan Validasi Module Path** (mengikuti semantik `golang.org/x/mod/module` — ADR-001 §4 memakai `x/mod/modfile`):

1. **Format umum:** satu atau lebih path segment dipisah `/`; segment pertama yang mengandung `.` diperlakukan sebagai host domain (mis. `github.com`). Contoh kanonik: `<host>/<owner>/<repo>` → `github.com/acme/shop`.
2. **Karakter legal per segment:** ASCII huruf `a-z A-Z`, angka `0-9`, dan `. _ - ~`. Karakter lain (spasi, `:`, `@`, non-ASCII, uppercase pada path Go module yang akan di-publish) **ditolak**.
3. **Larangan:** tidak diawali/diakhiri `/`; tidak ada segment kosong (`//`); tidak ada segment `.` atau `..`; tidak diakhiri `/vN` versi mayor palsu (kecuali memang versi modul ≥2 yang disengaja); tidak boleh hanya satu kata tanpa host bila ditujukan untuk publikasi (diizinkan untuk modul lokal, tapi diberi peringatan).
4. **Konsistensi microservice (monorepo single-module):** module path = root module; setiap service hidup di `services/<svc>/` di bawah module yang sama (tidak ada `go.mod` kedua). Path import service = `<module>/services/<svc>/...`.

| Contoh module path | Status | Alasan |
|---|---|---|
| `github.com/acme/shop` | ✅ valid | host + owner + repo, semua lowercase legal |
| `gitlab.com/team-a/billing_svc` | ✅ valid | `-` dan `_` legal dalam segment |
| `example.com/internal/api` | ✅ valid | host bertitik + multi-segment |
| `myapp` | ⚠ valid-with-warning | tanpa host; OK untuk modul lokal, diperingatkan jika akan `go install`/publish |
| `github.com/Acme/Shop` | ❌ invalid | uppercase pada path yang akan di-publish ditolak (case-folding ambiguity) |
| `github.com//shop` | ❌ invalid | segment kosong (`//`) |
| `github.com/acme/shop/` | ❌ invalid | diakhiri `/` |
| `git hub.com/acme/shop` | ❌ invalid | spasi ilegal |
| `github.com/acme/shop@v1` | ❌ invalid | `@` ilegal dalam module path |

### 4.3 q_arch — Tipe Arsitektur

| Field | Nilai |
|---|---|
| **id** | `q_arch` |
| **prompt** | "Tipe arsitektur?" |
| **widget** | `select` |
| **opsi** | `monolith` · `modular-monolith` · `microservice` |
| **default** | `monolith` |
| **depends-on** | — (selalu kedua; menentukan percabangan utama) |
| **validasi** | salah satu dari 3 nilai enum. |

> **Sumber:** 05 §2.1; 01 §4 (default monolith=layered); 02 §4 (microservice=monorepo single-module).

### 4.4 q_kind — Jenis Aplikasi

| Field | Nilai |
|---|---|
| **id** | `q_kind` |
| **prompt** | "Jenis aplikasi?" |
| **widget** | `select` |
| **opsi** | `rest` (HTTP REST API) · `web` (server-rendered HTML) · `worker` (background/cron, tanpa HTTP server) |
| **default** | `rest` |
| **depends-on** | `q_arch ∈ {monolith, modular-monolith}` (di-skip bila `microservice` — lihat C5) |
| **validasi** | salah satu dari 3 nilai enum. |

> **Sumber:** 05 §2.2.

### 4.5 q_svc — Service Awal + Komunikasi + Gateway

`q_svc` adalah **grup** dari 4 sub-pertanyaan yang hanya muncul saat `q_arch = microservice`:

| sub-id | prompt | widget | opsi | default | depends-on | validasi |
|---|---|---|---|---|---|---|
| `q_svc_list` | "Daftar service awal (nama, pisah koma; minimal 1)" | `input` | — | `order,user` | `q_arch = microservice` | ≥1 nama; tiap nama cocok `^[a-z][a-z0-9-]*$`; unik; reserved word `gateway` ditolak sbg nama service |
| `q_comm` | "Pola komunikasi antar service?" | `select` | `grpc` · `rest` · `event` | `grpc` | `q_arch = microservice` | enum |
| `q_broker` | "Message broker?" | `select` | `nats` · `kafka` · `rabbitmq` | `nats` | `q_arch = microservice` **dan** `q_comm = event` (lihat C3) | enum |
| `q_gateway` | "Aktifkan API gateway (REST edge)?" | `confirm` | yes/no | `no` (off) | `q_arch = microservice` | bool. Jika `q_comm = rest`, gateway diaktifkan implisit (lihat C6 / 05 §2.7). |

> **Sumber:** 02 §4 (monorepo, gateway opsional, gen/ di-commit); 05 §2.7–§2.8.

### 4.6 q_http — HTTP Framework

| Field | Nilai |
|---|---|
| **id** | `q_http` |
| **prompt** | "HTTP framework?" |
| **widget** | `select` |
| **opsi** | `net/http` (stdlib, default) · `chi` · `echo` · `gin` · `fiber` |
| **default** | `net/http` |
| **depends-on** | (`q_arch ∈ {monolith, modular-monolith}` **dan** `q_kind ∈ {rest, web}`) **ATAU** (`q_arch = microservice` **dan** gateway aktif / `q_comm = rest`). Di-skip bila `q_kind = worker` (C9). |
| **validasi** | enum. Jika `fiber` dipilih → set `go ≥ 1.25` di `go.mod` (C8) + tandai cabang fasthttp untuk add-on (C7). |

> **Sumber:** 03 §3.1; 05 §2.3. Catatan: `fiber=fasthttp` → memicu cabang template add-on terpisah (otel `otelfiber`, signature `fiber.Ctx`), tidak kompatibel set `net/http`.

### 4.7 q_db — Database

`q_db` adalah **grup** bertingkat (driver → access → migration); sub-pertanyaan access & migration hanya muncul bila driver ≠ `none`.

| sub-id | prompt | widget | opsi | default | depends-on | validasi |
|---|---|---|---|---|---|---|
| `q_db_driver` | "Pakai database?" | `select` | `none` · `postgres` · `mysql` · `sqlite` · `mongo` | `none` | — (lintas arsitektur) | enum |
| `q_access` | "Lapisan akses query?" | `select` | `sqlx` · `database/sql` · `sqlc` · `gorm` · `ent` | `sqlx` | `q_db_driver ≠ none` (C2) | enum. `sqlc`/`ent` → needs-step generate (C10/C11). `mongo` → access SQL tidak relevan; pakai driver mongo langsung (lihat C-mongo). |
| `q_migrate` | "Tool migrasi?" | `select` | `golang-migrate` · `goose` · `atlas` | `golang-migrate` | `q_db_driver ∉ {none, mongo}` (C1) | enum. `atlas` → pakai Community Edition (C20). |

> **Sumber:** 03 §3.3; 05 §2.4–§2.6. Catatan: semua driver default pure-Go/zero-CGO (`sqlite=modernc.org/sqlite`). `sqlite` + docker → tanpa service DB di compose (C14).

### 4.8 q_addons — Add-ons

| Field | Nilai |
|---|---|
| **id** | `q_addons` |
| **prompt** | "Add-ons (pilih banyak):" |
| **widget** | `multiselect` |
| **opsi (label → nilai kanon)** | `Docker + compose` → `docker` · `Makefile` → `makefile` · `Taskfile` → `taskfile` · `CI: GitHub Actions` → `github-actions` · `CI: GitLab CI` → `gitlab-ci` · `golangci-lint` → `lint` · `Observability (otel + health)` → `obs` · `Auth scaffold (JWT)` → `auth-jwt` · `.env.example` → `env-example` |
| **default (pre-checked)** | `docker` (jika `q_db_driver ≠ none` atau microservice), `makefile`, label "GitHub Actions" (= `github-actions`), `lint`, `env-example`. **Tidak** pre-checked: `taskfile`, label "GitLab CI" (= `gitlab-ci`), `obs`, `auth-jwt`. |
| **depends-on** | — (selalu muncul). Catatan dependen: `auth-jwt` di-skip/di-abaikan bila `q_kind = worker` (C17); `obs` mengikuti cabang fiber bila `q_http = fiber` (C7/C18). |
| **validasi** | multiselect dari himpunan nilai di atas. Mutual-exclusive pair CI: hanya satu CI provider yang efektif jika keduanya dicentang → peringatan, ambil `github-actions`. `makefile` vs `taskfile` boleh dua-duanya, tapi default Makefile. |

> **Pemetaan label UI → nilai kanon CI (selaras flag `--ci`, §5.1):** label "GitHub Actions" → `github-actions`; label "GitLab CI" → `gitlab-ci`; bila tidak ada CI yang dipilih (multiselect kosong) ⇒ ekuivalen `--ci=none`. (Catatan: key internal lama `ci-github`/`ci-gitlab` = nilai kanon `github-actions`/`gitlab-ci`; spec konsisten memakai nilai kanon.)

> **Catatan konsistensi dengan flag (§5):** di multiselect interaktif, `auth` hanya menawarkan JWT (sesuai keputusan terkunci); `paseto` tersedia hanya via flag `--auth=paseto` (advanced, 05 §2.9). `validation`, `config-loader`, dan `logging` **tidak** ditanyakan di multiselect — mereka punya default terkunci (`validator/v10` aktif sesuai aturan deterministik `--validate` di §6.2 — yakni on bila `kind ∈ {rest, web}` atau microservice ber-permukaan REST, off bila `kind = worker` / pure gRPC; `godotenv`; `slog`) dan hanya bisa diganti via flag (§5). Ini menjaga wizard tetap ringkas; flag tetap 1:1 lengkap.

> **Sumber:** 05 §2.9; default Docker (05 §4); auth JWT (03 §3.6).

### 4.9 q_git — git init + initial commit

| Field | Nilai |
|---|---|
| **id** | `q_git` |
| **prompt** | "Jalankan `git init` + initial commit?" |
| **widget** | `confirm` |
| **opsi** | yes / no |
| **default** | **yes** (mode interaktif). Mode non-interaktif/CI default **no** (lihat §5 & keputusan terkunci). |
| **depends-on** | — (selalu terakhir, sebelum konfirmasi generate) |
| **validasi** | bool. |

### 4.10 Diagram Alur

```
q_name (name + module path) ──validasi module path──┐
        │
        ▼
      q_arch ── monolith / modular-monolith ─────────► q_kind (rest|web|worker)
        │                                                   │
        │                                          ┌────────┴─ worker ─► (skip q_http)
        │                                          │ rest|web
        │                                          ▼
        │                                        q_http (net/http|chi|echo|gin|fiber)
        │
        └── microservice ──► q_svc_list (≥1)
                              q_comm (grpc|rest|event)
                                 ├─ event ─► q_broker (nats|kafka|rabbitmq)
                                 ├─ rest  ─► gateway=ON (implisit) ─► q_http (untuk edge)
                                 └─ grpc  ─► q_gateway (confirm; jika ON ─► q_http edge)
        │
        ▼  (jalur konvergen — semua arsitektur)
      q_db_driver (none|postgres|mysql|sqlite|mongo)
        ├─ none ─────────────────────────────► (skip q_access, q_migrate)
        └─ ≠none ─► q_access (skip migrate jika mongo)
                    q_migrate (skip jika mongo)
        │
        ▼
      q_addons (multiselect)
        │
        ▼
      q_git (confirm)
        │
        ▼
   [konfirmasi ringkasan] ─► generate ─► go mod tidy ─► (git init bila q_git=yes)
```

---

## 5. Mode Non-Interaktif (Flags)

Semua pertanyaan di §4 punya **flag ekuivalen 1:1**. `gostarter create` dijalankan via `spf13/cobra` (ADR-001 §1).

### 5.1 Tabel Ekuivalensi Flag

| id (§4) | flag | tipe nilai | nilai valid | default | required? |
|---|---|---|---|---|---|
| `q_name.name` | `--name` | string | `^[a-z][a-z0-9-]*$` | — | **ya** (non-interaktif) |
| `q_name.module` | `--module` | string | module path legal (lihat §4.2 `q_name`) | `github.com/<name>` | tidak (diturunkan dari `--name`) |
| `q_arch` | `--arch` | enum | `monolith`·`modular-monolith`·`microservice` | `monolith` | tidak |
| `q_kind` | `--kind` | enum | `rest`·`web`·`worker` | `rest` | tidak (diabaikan bila arch=microservice, C5) |
| `q_svc_list` | `--service` (repeatable) **atau** `--services` (csv) | []string | tiap nama `^[a-z][a-z0-9-]*$`, unik, ≠`gateway` | `order,user` (hanya jika arch=microservice) | tidak |
| `q_comm` | `--comm` | enum | `grpc`·`rest`·`event` | `grpc` | tidak (hanya valid bila arch=microservice, C4) |
| `q_broker` | `--broker` | enum | `nats`·`kafka`·`rabbitmq` | `nats` | tidak (hanya valid bila arch=microservice ∧ comm=event, C3) |
| `q_gateway` | `--gateway` / `--no-gateway` | bool | true/false | `false` (kecuali comm=rest → true, C6) | tidak |
| `q_http` | `--http` | enum | `net/http`·`chi`·`echo`·`gin`·`fiber` | `net/http` | tidak. Diabaikan bila `kind=worker` (C9). Pada microservice hanya relevan bila gateway aktif atau `comm=rest` (C6); pada pure gRPC tanpa gateway, `--http` tidak berpengaruh. |
| `q_db_driver` | `--db` | enum | `none`·`postgres`·`mysql`·`sqlite`·`mongo` | `none` | tidak |
| `q_access` | `--access` | enum | `sqlx`·`database/sql`·`sqlc`·`gorm`·`ent` | `sqlx` | tidak (butuh db≠none, C2). Tidak berlaku untuk `--db=mongo` (driver Mongo dipakai langsung; access layer & migration SQL di-skip — lihat C-mongo). |
| `q_migrate` | `--migrate` | enum | `golang-migrate`·`goose`·`atlas` | `golang-migrate` | tidak (butuh db∉{none,mongo}, C1). Tidak berlaku untuk `--db=mongo` (driver Mongo dipakai langsung; access layer & migration SQL di-skip — lihat C-mongo). |
| `q_addons[docker]` | `--docker` / `--no-docker` | bool | true/false | `true` bila db≠none ∨ arch=microservice; else `false` | tidak |
| `q_addons[makefile]` | `--makefile` / `--no-makefile` | bool | true/false | `true` | tidak |
| `q_addons[taskfile]` | `--taskfile` | bool | true/false | `false` | tidak |
| `q_addons[ci]` | `--ci` | enum | `github-actions`·`gitlab-ci`·`none` | `github-actions` | tidak |
| `q_addons[lint]` | `--lint` / `--no-lint` | bool | true/false | `true` | tidak |
| `q_addons[obs]` | `--obs` / `--no-obs` | bool | true/false | `false` | tidak |
| `q_addons[auth]` | `--auth` | enum | `none`·`jwt`·`paseto` | `none` (jika di-set tanpa nilai → `jwt`) | tidak (butuh HTTP handler, C17) |
| `q_addons[env-example]` | `--env-example` / `--no-env-example` | bool | true/false | `true` | tidak |
| (terkunci, flag only) | `--config-loader` | enum | `godotenv`·`koanf`·`viper`·`env` | `godotenv` | tidak |
| (terkunci, flag only) | `--log` | enum | `slog`·`zerolog`·`zap` | `slog` | tidak |
| (terkunci, flag only) | `--validate` / `--no-validate` | bool | true/false | **on** bila `kind ∈ {rest, web}`; **off** bila `kind = worker`; microservice: **on** bila ada permukaan REST (gateway aktif ∨ `comm=rest`), selain itu **off** (lihat §6.2) | tidak |
| `q_git` | `--git` / `--no-git` | bool | true/false | interaktif=`true`; non-interaktif/CI=`false` | tidak |

> **Catatan:** `--config-loader`, `--log`, `--validate` tidak ditanyakan di wizard (punya default terkunci) tapi **tetap punya flag** agar invarian 1:1 "setiap opsi punya flag" terpenuhi dari sisi flag→opsi. Sebaliknya, `--config` (preset) dan `--dry-run` (§5.4) adalah meta-flag (bukan turunan pertanyaan).

### 5.2 Invarian Byte-Identical

> **Untuk setiap kombinasi pilihan, menjalankan wizard interaktif lalu memilih nilai X di pertanyaan `q_*`, harus menghasilkan output yang BYTE-IDENTICAL dengan menjalankan mode non-interaktif memakai flag setara bernilai X.**

Penjamin invarian:

1. Wizard (`huh`) dan parser flag (`cobra`) menulis ke **satu struct konfigurasi internal yang sama** sebelum render; tidak ada jalur render terpisah.
2. Resolusi default (§6.2) dijalankan **identik** untuk kedua mode setelah input dikumpulkan.
3. Render deterministik: `text/template` + `go/format` (gofmt) + `x/mod/modfile.Format` → keluaran stabil (ADR-001 §3, §4). Urutan `require` di `go.mod` dinormalisasi.
4. `go.sum` & hasil `go mod tidy` dibuat oleh toolchain `go` yang sama; jika non-determinisme muncul dari tidy, gunakan versi pin di template `go.mod` (05 §5).
5. `--git` mempengaruhi keberadaan `.git/` + initial commit, **bukan** isi file project; output tree file (di luar `.git/`) tetap byte-identical antar `--git`/`--no-git`.

### 5.3 Contoh Perintah Lengkap

**(i) Monolith REST — chi + postgres + sqlx + golang-migrate + docker + CI**

```bash
gostarter create \
  --name shop \
  --module github.com/acme/shop \
  --arch monolith \
  --kind rest \
  --http chi \
  --db postgres \
  --access sqlx \
  --migrate golang-migrate \
  --docker \
  --ci github-actions \
  --lint \
  --no-git
```

**(ii) Modular-monolith — 2 fitur, gin + mysql + gorm, observability + auth JWT**

```bash
gostarter create \
  --name billing \
  --module gitlab.com/team-a/billing \
  --arch modular-monolith \
  --kind rest \
  --http gin \
  --db mysql \
  --access gorm \
  --migrate goose \
  --docker \
  --ci gitlab-ci \
  --lint \
  --obs \
  --auth jwt \
  --no-git
```

**(iii) Microservice — 2 service gRPC + compose**

```bash
gostarter create \
  --name fleet \
  --module github.com/acme/fleet \
  --arch microservice \
  --service order --service user \
  --comm grpc \
  --no-gateway \
  --db postgres \
  --access sqlx \
  --migrate golang-migrate \
  --docker \
  --ci github-actions \
  --lint \
  --no-git
```

### 5.4 Meta-flag

| flag | tipe | fungsi |
|---|---|---|
| `--config <file.yaml>` | path | **Preset.** Memuat seluruh jawaban dari file YAML (key = id pertanyaan §4: `name`, `module`, `arch`, `kind`, `service`, `comm`, `broker`, `gateway`, `http`, `db`, `access`, `migrate`, `docker`, `makefile`, `taskfile`, `ci`, `lint`, `obs`, `auth`, `env-example`, `config-loader`, `log`, `validate`, `git`). Precedence: flag eksplisit di CLI **menimpa** nilai dari `--config`; nilai `--config` menimpa default. Mode ini non-interaktif (wizard di-skip). |
| `--dry-run` | bool | Validasi semua input + jalankan constraint matrix (§6) + cetak **rencana** (daftar file/folder yang akan dibuat + dependency `go.mod`) **tanpa menulis** ke disk. Exit non-zero bila ada kombinasi invalid. |

> **Catatan disambiguasi nama:** untuk menjaga 1 flag = 1 makna, `--config` = **preset file builder**, sedangkan pilihan config-loader runtime project memakai flag **`--config-loader`** (nilai `godotenv|koanf|viper|env`, default `godotenv`). Baris "config" di §5.1 merujuk `--config-loader`.

---

## 6. Constraint Matrix & Default Resolution

### 6.1 Tabel Kombinasi VALID / INVALID

Diturunkan langsung dari `05-decision-matrix.md` §3 (C1–C20), diselaraskan ke nama flag di §5.

| # | Aturan | Tipe | Kondisi (flag §5) | Perilaku generator |
|---|---|---|---|---|
| C1 | Migration butuh DB | requires | `--migrate ∈ {golang-migrate,goose,atlas}` ∧ `--db = none` | **Invalid.** Non-interaktif: error `migration requires --db`. Interaktif: `q_migrate` di-skip. |
| C2 | Access butuh DB | requires | `--access ∈ {sqlx,database/sql,sqlc,gorm,ent}` ∧ `--db = none` | **Invalid.** `q_access` di-skip / error. |
| C3 | Broker butuh microservice+event | requires | `--broker` di-set ∧ (`--arch ≠ microservice` ∨ `--comm ≠ event`) | **Invalid.** `q_broker` hanya muncul saat `--arch=microservice ∧ --comm=event`. |
| C4 | `--comm` hanya microservice | requires *(efek: ignore+warn, bukan error)* | `--comm` di-set ∧ `--arch ≠ microservice` | `--comm` **diabaikan dengan peringatan** (bukan fail-fast); generate tetap berjalan. |
| C5 | `--kind` hanya monolith/modular | requires *(efek: ignore+warn, bukan error)* | `--kind` di-set ∧ `--arch = microservice` | `--kind` **diabaikan dengan peringatan** (bukan fail-fast); microservice pakai pola service, bukan kind. |
| C6 | Gateway butuh microservice | requires | `--gateway` ∧ `--arch ≠ microservice` | **Invalid.** Catatan: `--comm=rest` ⇒ `--gateway=true` implisit. |
| C7 | `fiber` ⇒ cabang add-on fasthttp | conflicts (partial) | `--http = fiber` ∧ `--obs` | **Valid dengan cabang.** Generator pakai `otelfiber` + `fiber.Ctx`, bukan `otelhttp`. Set add-on `net/http` standar **tidak** dipakai. |
| C8 | `fiber` butuh Go 1.25+ | requires | `--http = fiber` | go directive `go.mod` ≥ 1.25; target lebih rendah → tolak/peringatan. |
| C9 | `worker` ⇒ tanpa HTTP | conflicts | `--kind = worker` ∧ `--http` di-set | **Diabaikan** → modul `http:*` mati, `q_http` di-skip. |
| C10 | `sqlc` perlu generate | needs-step | `--access = sqlc` | Output `db/sqlc/*.go` **di-commit** atau `sqlc generate` di post-gen sebelum build. |
| C11 | `ent` perlu generate | needs-step | `--access = ent` | `go generate ./ent` di post-gen atau commit output. |
| C12 | mock perlu output di-commit | needs-step | `--mock ∈ {uber-mock,mockery}` | File mock di-commit, bukan hanya directive `//go:generate`. |
| C13 | testcontainers wajib build-tag | needs-step | `--integration` | File test ber-`//go:build integration`; default `go test ./...` tetap hijau tanpa Docker. |
| C14 | SQLite tanpa service DB di compose | conflicts (info) | `--db = sqlite` ∧ `--docker` | compose tidak menambah service DB (SQLite = file lokal); siapkan volume, bukan kontainer DB. |
| C15 | `mattn/go-sqlite3` bukan default | avoid-default | user minta driver SQLite CGO | Bukan default; opt-in eksplisit + peringatan `CGO_ENABLED=1` memecah build minimal/cross-compile. |
| C16 | Library archived dilarang default | avoid-default | user minta `google/wire`/`golang/mock`/`survey`(builder)/`streadway/amqp` | Ditolak sebagai default; hanya dengan peringatan. DI default = manual constructor injection. |
| C17 | Auth butuh HTTP handler | requires | `--auth ∈ {jwt,paseto}` ∧ `--kind = worker` | Middleware auth tidak relevan tanpa HTTP → diabaikan (atau util token-verify saja, tanpa middleware). |
| C18 | Observability default = kelas `net/http` | conflicts (partial) | `--obs` ∧ `--http ∈ {net/http,chi,gin,echo}` | Valid, satu set `otelhttp`. `--http=fiber` → cabang `otelfiber` (lihat C7). |
| C19 | DI tetap manual (zero codegen) | info | semua mode | Wiring = manual constructor injection; `uber-go/fx` hanya opt-in. Tidak ada langkah codegen DI. |
| C20 | atlas pakai Community Edition | license | `--migrate = atlas` | Gunakan CE (Apache-2.0); binary default ber-EULA — catat di docs lisensi. MySQL driver MPL-2.0 juga dicatat. |
| C-mongo | Mongo tanpa SQL access/migration | requires | `--db = mongo` ∧ (`--access ∈ {sqlx,database/sql,sqlc,gorm,ent}` ∨ `--migrate` di-set) | **Invalid/diabaikan.** Mongo memakai driver `mongo-driver/v2` langsung; `q_access`/`q_migrate` di-skip; migrasi schema-less. (Turunan 03 §3.3 — mongo wajib path v2.) |
| C-ci | CI provider tunggal | conflicts (info) | label "GitHub Actions" (= `github-actions`) ∧ label "GitLab CI" (= `gitlab-ci`) dicentang bersamaan (interaktif) | Peringatan → ambil `github-actions` (selaras `--ci` yang enum tunggal `github-actions`·`gitlab-ci`·`none`). |

### 6.2 Default Resolution

Default per-flag (turunan 05 §4) untuk mode non-interaktif / flag tidak lengkap, **diresolusi berurutan**: `arch` → (`kind` | microservice-comm) → `http` → `db` → (`access`, `migrate`, driver) → add-ons → testing/config/log.

| flag | default bila kosong |
|---|---|
| `--arch` | `monolith` |
| `--kind` (monolith/modular) | `rest` |
| `--http` (rest/web atau gateway) | `net/http` |
| `--db` | `none` |
| `--access` (bila db≠none, ≠mongo) | `sqlx` |
| `--migrate` (bila db∉{none,mongo}) | `golang-migrate` |
| driver pg/mysql/sqlite/mongo | `pgx/v5` / `go-sql-driver/mysql` / `modernc.org/sqlite` / `mongo-driver/v2` |
| `--comm` (microservice) | `grpc` |
| `--broker` (microservice+event) | `nats` |
| `--gateway` | `false` (kecuali `--comm=rest` → `true`) |
| `--config-loader` | `godotenv` |
| `--log` | `slog` |
| `--validate` | **on** bila `kind ∈ {rest, web}` (generator menghasilkan handler contoh yang menerima request body/input); **off** bila `kind = worker` (tanpa handler HTTP). Microservice: **on** bila ada permukaan REST (gateway aktif ∨ `comm=rest`), selain itu **off** (pure gRPC tanpa contoh input). |
| `--docker` | `true` bila db≠none ∨ arch=microservice; else `false` |
| `--makefile` | `true` |
| `--taskfile` | `false` |
| `--ci` | `github-actions` |
| `--lint` | `true` |
| `--obs` | `false` |
| `--auth` | `none` (jika diaktifkan tanpa nilai → `jwt`) |
| `--env-example` | `true` |
| `--mock` | `false` (output di-commit bila on) |
| `--integration` | `false` (disarankan on bila db≠none; tetap di balik build-tag) |
| DI/wiring | manual constructor injection (tetap; tanpa flag) |

### 6.3 Profil Zero-Config

```
gostarter create --name myapp
```

Resolusi tanpa flag lain →
`arch=monolith`, `kind=rest`, `http=net/http`, `db=none`, `access=—`, `migrate=—`,
`config-loader=godotenv`, `log=slog`, `validate=on` (karena `kind=rest` ⇒ handler REST contoh punya input request body), `docker=off`, `makefile=on`, `taskfile=off`, `ci=github-actions`, `lint=on`, `obs=off`, `auth=none`, `env-example=on`, **`git=off`** (non-interaktif), testify baseline.

Hasil: project minimal yang **langsung** lolos `go vet ./... && go build ./... && go test ./...` tanpa dependency berat dan tanpa Docker (05 §4.1 poin 5).

### 6.4 Aturan Resolusi Konflik

1. **Resolusi berurutan & abaikan downstream tak-relevan dengan peringatan.** Mis. `--broker` saat `--comm=grpc` → diabaikan + peringatan (C3); `--kind` saat `--arch=microservice` → diabaikan (C5); `--http` saat `--kind=worker` → diabaikan (C9).
2. **Validasi constraint §6.1 dijalankan sebelum render.** Kombinasi hard-invalid (C1, C2, C3, C6, C8, C-mongo) → **fail-fast** dengan pesan menyebut flag bermasalah; tidak menghasilkan project setengah jadi. **Catatan:** C4 dan C5 bertipe `requires` tetapi efeknya **ignore+warn** (bukan fail-fast) — flag tak-relevan diabaikan dengan peringatan, generate tetap berjalan (lihat rule 1 & §6.1).
3. **needs-step (C10–C13) dijalankan otomatis di post-gen** (commit/generate output codegen) sebelum verifikasi `go build`/`go test` → "build hijau tanpa edit" terjaga.
4. **avoid-default (C15–C16):** library archived/CGO tidak pernah dipilih otomatis; hanya muncul atas permintaan eksplisit + peringatan.
5. **Precedence input:** CLI flag eksplisit > `--config` preset > default tabel §6.2.
6. **Implikasi silang terkunci:** `--comm=rest` ⇒ `--gateway=true` + `q_http` aktif untuk edge (C6); `--http=fiber` ⇒ `go≥1.25` (C8) + semua add-on otel/health/middleware ikut cabang fasthttp (C7/C18); `--db=sqlite` ⇒ compose tanpa service DB (C14); `--db=mongo` ⇒ skip access/migrate SQL (C-mongo).

### 6.5 Catatan Penyelarasan Spesifikasi

Penyelarasan yang dibuat selama spesifikasi (transparansi, bukan invensi):

- **`--config` ambigu:** di instruksi punya dua makna potensial (preset builder vs config-loader runtime). Untuk menjaga 1 flag = 1 makna, spec menetapkan `--config` = preset file, dan config-loader runtime project memakai `--config-loader` (nilai & default `godotenv` tetap sesuai 05 §2.9). Semua opsi tetap punya flag (invarian §5 terpenuhi).
- **Wizard ringkas:** `q_addons` sengaja tidak menanyakan `config-loader`/`log`/`validate` di wizard (default terkunci sesuai keputusan), namun ketiganya tetap memiliki flag di §5.1 sehingga invarian "setiap opsi punya flag" terpenuhi penuh dari arah flag→opsi.
- **Flag advanced di luar 8 langkah inti:** `--mock`, `--integration`, `--bdd` (05 §2.10) tidak masuk pohon pertanyaan inti §4 tetapi muncul di constraint C10–C13 karena mereka adalah needs-step; mereka tersedia sebagai flag advanced (default off) dan tidak mengganggu invarian 8-langkah.

---

## 7. User Stories & Acceptance Criteria

> **Konvensi acceptance criteria:** format Given/When/Then. Tiap story ≥2 skenario (happy path + edge/error). Nama opsi/flag PERSIS mengikuti §5.1. Aturan keras lihat **Definition of Done (Global)** di akhir bagian.
> **Persona:** **Dev** = developer pengguna gostarter (lokal). **CI** = pipeline non-interaktif. **Maintainer** = pemelihara project microservice existing.

### US-01 — Generate monolith REST via wizard interaktif

**Sebagai** Dev, **saya ingin** men-generate project monolith REST lewat wizard interaktif (`gostarter create` tanpa flag) **agar** saya bisa memulai project Go best-practice tanpa menghafal flag, dan hasilnya langsung bisa di-build & di-test.

**Acceptance Criteria**

**Skenario 1 — Happy path: wizard default → project hijau**
- **Given** Dev menjalankan `gostarter create` di direktori kosong, dan toolchain Go terpasang
- **When** Dev mengisi `q_name.name=shop`, menerima default `q_name.module=github.com/shop`, lalu menekan Enter pada `q_arch` (`monolith`), `q_kind` (`rest`), `q_http` (`net/http`), `q_db_driver` (`none`), dan menerima add-ons pre-checked (`makefile`, label "GitHub Actions" = `github-actions`, `lint`, `env-example`; `docker` tidak pre-checked karena `db=none`), `q_git=yes`
- **Then** generator membuat layout Kandidat B (layered): `cmd/api/main.go`, `internal/app/app.go`, `internal/config/config.go`, `internal/http/{router.go,middleware.go}`, `internal/<feature>/{service.go,handler.go,repository.go,service_test.go}`, `Makefile`, `.golangci.yml`, `.github/workflows/ci.yml`, `.env.example`, `README.md`, `.gitignore`
- **And** `go vet ./... && go build ./... && go test ./...` lolos tanpa edit manual
- **And** `git init` + initial commit dijalankan (default `yes` di mode interaktif), dan tidak ada satu pun import dari builder gostarter di project

**Skenario 2 — Edge: pilih chi + framework non-default tetap hijau**
- **Given** Dev berada di wizard yang sama
- **When** Dev memilih `q_http=chi` (bukan default `net/http`), `q_db_driver=none`
- **Then** `internal/http/router.go` di-generate memakai `chi.NewRouter()` + `chi.URLParam`, dan `go.mod` menambahkan `github.com/go-chi/chi/v5 v5.3.0`
- **And** `go vet/build/test ./...` tetap hijau tanpa edit manual

**Skenario 3 — Error: nama project invalid**
- **Given** Dev berada pada pertanyaan `q_name.name`
- **When** Dev mengisi `Shop App` (mengandung huruf besar + spasi, melanggar regex `^[a-z][a-z0-9-]*$`)
- **Then** wizard menolak input, menampilkan pesan validasi, dan **tidak** melanjutkan ke `q_arch` sampai input valid
- **And** tidak ada file apa pun ditulis ke disk

### US-02 — Generate monolith via flags identik dengan interaktif

**Sebagai** CI (atau Dev yang scripting), **saya ingin** men-generate project monolith sepenuhnya lewat flag tanpa prompt **agar** generate bisa dijalankan reproducible di pipeline, dengan output **byte-identical** terhadap jalur interaktif untuk pilihan yang sama.

**Acceptance Criteria**

**Skenario 1 — Happy path: flag lengkap → byte-identical dengan wizard**
- **Given** sebuah kombinasi pilihan X (mis. `name=shop`, `module=github.com/acme/shop`, `arch=monolith`, `kind=rest`, `http=chi`, `db=postgres`, `access=sqlx`, `migrate=golang-migrate`, `docker`, `ci=github-actions`, `lint`, `--no-git`)
- **When** CI menjalankan:
  ```bash
  gostarter create --name shop --module github.com/acme/shop \
    --arch monolith --kind rest --http chi \
    --db postgres --access sqlx --migrate golang-migrate \
    --docker --ci github-actions --lint --no-git
  ```
  **dan** Dev menjalankan wizard interaktif memilih nilai X yang sama persis
- **Then** tree file project yang dihasilkan kedua mode (di luar direktori `.git/`) adalah **byte-identical** (mengikuti invarian §5.2: satu struct config internal, resolusi default identik, render deterministik via `text/template`+gofmt+`modfile.Format`)
- **And** kedua project lolos `go vet/build/test ./...` tanpa edit manual

**Skenario 2 — Edge: `--git` vs `--no-git` tidak mengubah isi project**
- **Given** dua perintah identik kecuali `--git` vs `--no-git`
- **When** keduanya dijalankan
- **Then** seluruh tree file di luar `.git/` tetap byte-identical; perbedaan hanya pada keberadaan `.git/` + initial commit (§5.2 poin 5)
- **And** default `git` di mode non-interaktif = `off` (sesuai keputusan terkunci), sehingga `gostarter create --name shop` di CI tidak membuat repo git

**Skenario 3 — Error: `--name` wajib di mode non-interaktif**
- **Given** CI menjalankan `gostarter create --arch monolith` tanpa `--name`
- **When** generator memproses flag
- **Then** generator gagal cepat (exit non-zero) dengan pesan bahwa `--name` required di mode non-interaktif, **tanpa** menghasilkan project setengah jadi

**Skenario 4 — Edge: flag konflik diabaikan sesuai constraint, bukan crash**
- **Given** CI menjalankan `gostarter create --name shop --arch monolith --kind rest --comm grpc`
- **When** generator menjalankan validasi constraint sebelum render
- **Then** `--comm` diabaikan dengan peringatan (C4: `comm` hanya valid untuk microservice), generate tetap berjalan, dan project monolith REST tetap hijau

### US-03 — Generate modular-monolith

**Sebagai** Dev, **saya ingin** men-generate modular-monolith dengan 2 modul domain yang berkomunikasi in-process **agar** saya punya boundary "berduri" antar-modul sejak awal dan siap diekstraksi ke microservice tanpa rewrite besar.

**Acceptance Criteria**

**Skenario 1 — Happy path: layout Kandidat C + boundary terbentuk**
- **Given** Dev menjalankan `gostarter create --name shop --module github.com/acme/shop --arch modular-monolith --kind rest --http net/http --no-git`
- **When** generator merender modul `arch:modular-monolith`
- **Then** project memuat `cmd/monolith/main.go` (composition root tunggal), `internal/platform/{config,database,eventbus}/`, `internal/shared/contract/{<m>.go,events.go}`, dan 2 modul di `internal/modules/<m>/` (mis. `user/` dan `order/`) masing-masing dengan `module.go` + `internal/{service.go,repository.go,handler.go,service_test.go}` + `migrations/`
- **And** contoh komunikasi antar-modul di-generate: `order` memanggil `user` lewat `shared/contract` (interface) atau in-process `eventbus`, **bukan** mengimport `modules/user/internal/*`
- **And** `go vet/build/test ./...` lolos tanpa edit manual

**Skenario 2 — Edge: boundary `internal/` per-modul terbukti dipaksakan compiler**
- **Given** project modular-monolith hasil Skenario 1
- **When** seseorang mencoba menambahkan import dari `internal/modules/order/internal/...` ke dalam modul `user`
- **Then** compiler Go menolak import lintas-`internal/` (boundary berduri terbukti nyata, bukan sekadar konvensi), sesuai pola powerman/go-monolith-example
- **And** jalur sah satu-satunya antar-modul adalah `internal/shared/contract` + `internal/platform/eventbus`

**Skenario 3 — Konsistensi: interaktif ≡ non-interaktif untuk modular-monolith**
- **Given** Dev memilih `q_arch=modular-monolith` di wizard dengan nilai-nilai lain yang sama
- **When** dibandingkan dengan `--arch modular-monolith` + flag setara
- **Then** output kedua mode byte-identical (invarian §5.2)

### US-04 — Generate microservice

**Sebagai** Dev, **saya ingin** men-generate microservice monorepo dengan 2 service gRPC yang saling memanggil dan bisa dijalankan bersama lewat `docker compose` **agar** saya punya contoh kerja end-to-end (order memanggil user) tanpa harus menyiapkan proto/compose manual.

**Acceptance Criteria**

**Skenario 1 — Happy path: 2 service gRPC, stub di-commit, build hijau**
- **Given** Dev menjalankan:
  ```bash
  gostarter create --name fleet --module github.com/acme/fleet \
    --arch microservice --service order --service user \
    --comm grpc --no-gateway \
    --db postgres --access sqlx --migrate golang-migrate \
    --docker --ci github-actions --lint --no-git
  ```
- **When** generator merender modul `arch:microservice` + `comm:grpc`
- **Then** project memakai monorepo single-module (Kandidat B): satu `go.mod` root, `proto/<svc>/v1/<svc>.proto`, `gen/go/<svc>/v1/*.pb.go` **DI-COMMIT**, `services/<svc>/{cmd/main.go, internal/{handler,client,service,config}/, Dockerfile, migrations/}`, `libs/{logger,config,grpcclient,health}/`, `buf.yaml`, `buf.gen.yaml`, `docker-compose.yml`
- **And** `go.mod` menambahkan `google.golang.org/grpc v1.81.1` + `google.golang.org/protobuf v1.36.11`
- **And** `go vet ./... && go build ./... && go test ./...` lolos **tanpa** menjalankan `buf generate` lebih dulu (karena `gen/go/` sudah di-commit)

**Skenario 2 — Contoh call order→user terkompilasi**
- **Given** project hasil Skenario 1
- **When** memeriksa `services/order/internal/client/user.go`
- **Then** `order` mengimpor stub `gen/go/user/v1` (path dalam module yang sama), men-`dial` `user` via `libs/grpcclient`, dan contoh call lintas-service kompilasi tanpa langkah tambahan
- **And** tidak ada module boundary kedua yang harus dilintasi (single-module monorepo)

**Skenario 3 — Edge: nama service `gateway` ditolak**
- **Given** Dev menjalankan `gostarter create --name fleet --arch microservice --service gateway --service user ...`
- **When** generator memvalidasi `q_svc_list`/`--service`
- **Then** generator menolak `gateway` sebagai nama service (reserved word) dan gagal cepat dengan pesan jelas, tanpa menghasilkan project

**Skenario 4 — Edge: `--comm rest` mengaktifkan gateway implisit**
- **Given** Dev menjalankan `... --arch microservice --comm rest ...` (tanpa `--gateway`)
- **When** generator meresolusi default
- **Then** `--gateway=true` di-set implisit (C6 / §5.1) dan `q_http` aktif untuk edge REST; `gateway/{cmd/main.go, internal/router/}` + blok `gateway` di compose di-generate
- **And** project tetap hijau `go vet/build/test ./...`

### US-05 — gostarter add service ke project existing

**Sebagai** Maintainer, **saya ingin** menambahkan service baru ke project microservice yang sudah ada lewat `gostarter add service <name>` **agar** saya bisa tumbuh secara inkremental tanpa men-generate ulang dan tanpa merusak service yang sudah ada.

**Acceptance Criteria**

**Skenario 1 — Happy path: sisip deterministik, build tetap hijau**
- **Given** sebuah project microservice monorepo hasil gostarter (punya `services/`, `proto/`, `gen/`, `docker-compose.yml`, `buf.yaml`)
- **When** Maintainer menjalankan `gostarter add service payment` dari root project
- **Then** generator menambahkan **hanya** titik-sisip deterministik: subtree `services/payment/{cmd,internal,Dockerfile,migrations/}`, `proto/payment/v1/payment.proto`, output `gen/go/payment/v1/*.pb.go` (di-commit), dan satu blok service `payment` di `docker-compose.yml`
- **And** service-service yang sudah ada **tidak diubah** isinya selain titik-sisip yang diperlukan
- **And** `go vet/build/test ./...` lolos tanpa edit manual setelah penambahan

**Skenario 2 — Error: bukan project microservice gostarter**
- **Given** Maintainer berada di direktori project monolith (atau direktori bukan project gostarter, tanpa marker `services/`+`proto/`)
- **When** menjalankan `gostarter add service payment`
- **Then** generator menolak dengan pesan bahwa `add service` hanya berlaku untuk project microservice monorepo, dan tidak menulis apa pun

**Skenario 3 — Edge: nama service sudah ada / reserved**
- **Given** project sudah punya service `user`
- **When** Maintainer menjalankan `gostarter add service user` (duplikat) atau `gostarter add service gateway` (reserved)
- **Then** generator menolak (nama duplikat / reserved `gateway`) dan gagal cepat tanpa memodifikasi project

### US-06 — --dry-run + proteksi overwrite

**Sebagai** Dev, **saya ingin** melihat preview rencana generate dengan `--dry-run` dan dilindungi dari menimpa folder yang sudah berisi **agar** saya bisa memverifikasi kombinasi pilihan sebelum menulis ke disk dan tidak kehilangan pekerjaan yang ada.

**Acceptance Criteria**

**Skenario 1 — Happy path: `--dry-run` mencetak rencana, nol penulisan**
- **Given** Dev menjalankan `gostarter create --name shop --arch microservice --service order --service user --comm grpc --db postgres --dry-run`
- **When** generator memproses input
- **Then** generator memvalidasi semua input + menjalankan constraint matrix (§6) lalu **mencetak rencana**: daftar file/folder yang akan dibuat + daftar dependency `go.mod`
- **And** **tidak ada** file/folder yang ditulis ke disk
- **And** exit code `0` untuk kombinasi valid

**Skenario 2 — Error: `--dry-run` atas kombinasi invalid → exit non-zero**
- **Given** Dev menjalankan `gostarter create --name shop --migrate goose --db none --dry-run`
- **When** constraint matrix dievaluasi (C1: migration butuh DB)
- **Then** generator mencetak error yang menyebut flag bermasalah (`migration requires --db`) dan keluar dengan exit non-zero, **tanpa** menulis apa pun

**Skenario 3 — Edge: proteksi overwrite folder target tak kosong**
- **Given** direktori target `shop/` sudah ada dan **tidak kosong**
- **When** Dev menjalankan `gostarter create --name shop ...` (tanpa dry-run)
- **Then** generator **menolak** menimpa, melaporkan bahwa folder tujuan tidak kosong, dan tidak memodifikasi isi folder yang ada
- **And** pada folder target yang **kosong atau belum ada**, generate berjalan normal

### US-07 — Stack ber-DB → docker compose up jalan

**Sebagai** Dev, **saya ingin** project ber-DB yang langsung bisa dijalankan dengan `docker compose up` (DB + app menyala bersama) **agar** saya bisa menjalankan stack lengkap tanpa konfigurasi infra manual.

**Acceptance Criteria**

**Skenario 1 — Happy path: Postgres + app via compose**
- **Given** Dev men-generate `gostarter create --name shop --arch monolith --kind rest --db postgres --access sqlx --migrate golang-migrate --docker --no-git`
- **When** Dev menjalankan `docker compose up`
- **Then** `docker-compose.yml` menyalakan service `postgres` + service `app` (build dari `Dockerfile` multi-stage base minimal), dengan `.env`/`DATABASE_URL` ter-wire ke service DB
- **And** `go.mod` memakai `github.com/jackc/pgx/v5` (pin patch terbaru seri v5.x), `internal/platform/database/postgres.go` memakai `pgxpool`, dan `migrations/0001_init.{up,down}.sql` tersedia
- **And** project tetap lolos `go vet/build/test ./...` tanpa edit manual

**Skenario 2 — Edge: SQLite tidak menambah service DB di compose**
- **Given** Dev men-generate `... --db sqlite --access sqlx --docker ...`
- **When** memeriksa `docker-compose.yml`
- **Then** compose **tidak** menambahkan service DB (SQLite = file lokal, C14); volume disiapkan untuk file DB, bukan kontainer DB
- **And** driver yang dipakai `modernc.org/sqlite v1.39.0` (pure-Go, zero-CGO), sehingga `CGO_ENABLED=0` aman

**Skenario 3 — Edge: integration test ada tapi `go test ./...` tetap hijau tanpa Docker**
- **Given** project ber-DB dengan integration test (testcontainers)
- **When** Dev menjalankan `go test ./...` di mesin tanpa Docker
- **Then** test default tetap hijau karena file integration ber-`//go:build integration` (C13); container hanya berjalan saat `go test -tags=integration ./...`

**Skenario 4 — Edge/Error: `--db mongo` melewati access/migration SQL**
- **Given** Dev menjalankan `... --db mongo --access sqlx --migrate goose ...`
- **When** generator memvalidasi constraint (C-mongo)
- **Then** `--access`/`--migrate` SQL diabaikan/ditolak; Mongo memakai driver `go.mongodb.org/mongo-driver/v2` langsung, tanpa SQL access layer & migration
- **And** blok `mongo` di compose di-generate; project tetap hijau

### Definition of Done (Global)

Sebuah generate dianggap **selesai dan benar** hanya bila SEMUA berikut terpenuhi (berlaku untuk setiap user story di atas):

1. **Build/test hijau tanpa edit manual.** Hasil generate lolos `go vet ./... && go build ./... && go test ./...` di satu root, tanpa satu pun edit manual. Untuk modul `needs-step` (sqlc, ent, mock — C10/C11/C12) output codegen **di-commit** (atau di-generate di post-gen) sehingga `go build` tetap hijau tanpa langkah manual. Untuk microservice, `gen/go/` **di-commit** sehingga build hijau **tanpa** `buf generate`.
2. **Interaktif ≡ non-interaktif (byte-identical).** Untuk setiap kombinasi pilihan, jalur wizard (`huh`) dan jalur flag (`cobra`) menghasilkan tree file **byte-identical** di luar `.git/` (invarian §5.2). `--git`/`--no-git` hanya memengaruhi keberadaan `.git/` + initial commit, bukan isi project.
3. **`docker compose up` jalan untuk stack ber-DB.** Project ber-DB (kecuali SQLite, C14) menyalakan service DB + app dalam satu `docker compose up`.
4. **Zero lock-in.** Project hasil generate **TIDAK** meng-import package apa pun dari builder gostarter; dependency runtime hanya stdlib atau library publik pihak ketiga sesuai matrix `05`.
5. **Tidak ada library archived sebagai default.** Default tidak pernah memilih library archived/dorman atau CGO (mis. `google/wire`, `golang/mock`, `streadway/amqp`, `mattn/go-sqlite3`, go-kit) — hanya opt-in eksplisit dengan peringatan (C15/C16). Semua versi dependency mengikuti pin terverifikasi 2026-06-06 di `05-decision-matrix.md` §2.
6. **Fail-fast atas kombinasi invalid.** Kombinasi hard-invalid (C1, C2, C3, C6, C8, C-mongo) gagal cepat dengan pesan menyebut flag bermasalah, **tanpa** menghasilkan project setengah jadi; flag downstream tak-relevan (termasuk C4 `--comm` & C5 `--kind` di arch yang salah) diabaikan dengan peringatan (ignore+warn, bukan crash).

---

## 8. Non-Goals (Fase Implementasi)

Penegasan ulang batasan ruang lingkup v1 — apa yang gostarter **tidak** lakukan:

- **N1 — Bukan framework runtime; zero lock-in.** gostarter hanya men-generate struktur. Project hasil generate tidak meng-import package apa pun dari builder; tidak ada library runtime gostarter yang ikut ter-bundle (04 §1, §4.1).
- **N2 — Hanya struktur + wiring + 1 contoh minimal, bukan business logic.** Output berisi layout, dependency-wiring, dan satu contoh fungsional minimal (mis. 1 handler, atau 2 service yang saling memanggil). gostarter **tidak** menulis logika bisnis aplikasi pengguna (04 §1).
- **N3 — Go-only.** Lingkup v1 terbatas pada project Go. Tidak men-generate komponen non-Go sebagai paradigma (mis. frontend SPA, layanan bahasa lain).
- **N4 — Tanpa upgrade/migrasi project lama.** gostarter men-generate project baru (dan menambah service ke project microservice hasil gostarter via `add service`). Ia **tidak** meng-upgrade, me-refactor, atau memigrasi project Go yang sudah ada di luar alur generate-nya.
- **N5 — Tanpa plugin pihak ketiga.** v1 tidak menyediakan sistem plugin pihak ketiga; semua dimensi (arsitektur, framework, DB, add-on) berasal dari modul template bawaan yang ditetapkan di dokumen sumber kebenaran. Library di luar yang tercantum di `docs/research/03` + `05` tidak diperkenalkan.
