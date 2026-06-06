---
status: Proposed
date: 2026-06-06
deciders: [isal]
tags: [builder, templates, go]
relates: ADR-002
---

# ADR-003: Sistem Template Modular gostarter

## Status

**Proposed** — 2026-06-06.

Keputusan ini mengikat ke ADR-001 (stack builder) dan ADR-002 (engine template &
merge). ADR-003 menetapkan **bentuk data & konvensi** sistem template modular:
layout direktori `templates/`, skema `module.yaml`, konvensi file `.tmpl`,
strategi conditional render, mekanisme MERGE file shared, validasi manifest, dan
kebijakan versioning template. Tidak memperkenalkan keputusan engine baru di luar
ADR-001/ADR-002 — semua tipe inti (`Manifest`, `MergeContribution`, `FuncMap`,
"fragment + assembler") merupakan cerminan kontrak kanonik ADR-002 §Decision-3
(tipe inti), §Decision-4 (FuncMap), dan §Decision-6 (model eksekusi merge).

## Context

`gostarter` mengomposisikan satu project Go dari banyak dimensi independen:
arsitektur (monolith / modular-monolith / microservice) × HTTP framework
(`net/http` / chi / echo / gin / fiber) × database (postgres / mysql / sqlite /
mongo) × access layer × migration × add-on (docker, makefile, CI, lint, obs,
auth, env) × config loader × logger. Mengikuti SPEC §2 (Glossary "Modul
template", "Manifest modul") pendekatannya adalah **composition by category**
(riset 04 §2.2): tiap dimensi adalah modul template terpisah yang dikomposisikan,
**bukan** satu template raksasa per-kombinasi.

Tekanan desain yang mengikat:

1. **Hindari duplikasi kombinatorial.** Jumlah kombinasi valid sangat besar
   (3 arch × 5 http × 5 db-driver × 5 access × 3 migrate × 2^n add-on …). Satu
   template lengkap per-kombinasi tidak terkelola; modularitas wajib.
2. **Banyak modul menyumbang ke file yang sama (file shared).** `docker-compose.yml`,
   `go.mod`, `Makefile`, `.env.example`, dan wiring di `main.go` dikontribusikan
   oleh beberapa modul sekaligus (mis. `db-postgres` menambah service compose +
   `DATABASE_URL` di `.env.example` + target migrate di `Makefile` + require di
   `go.mod`). Dibutuhkan mekanisme **merge** yang deterministik dan idempotent.
3. **Invarian byte-identical (SPEC §5.2).** Output interaktif (`huh`) dan
   non-interaktif (`cobra`) wajib byte-identical di luar `.git/`. Maka urutan
   file, urutan kontribusi merge, dan urutan `require` go.mod **harus** stabil —
   bukan bergantung urutan input.
4. **Dry-run akurat (SPEC §5.4, US-06).** `--dry-run` harus mencetak rencana
   persis seperti yang akan ditulis tanpa menyentuh disk. Konsekuensinya:
   percabangan struktural (file ada/tidak) harus diketahui **sebelum** render,
   bukan disembunyikan di dalam template.
5. **Single-binary, zero lock-in (ADR-001, SPEC §6 N1).** Template dibundel via
   `embed.FS`; tidak ada file template eksternal saat runtime; tidak ada artefak
   builder yang masuk ke project hasil generate.
6. **needs-step terjaga (SPEC §6.1 C10–C13).** Modul codegen (`sqlc`, `ent`,
   mock) harus mendeklarasikan langkah pasca-generate agar build hijau tanpa edit
   manual.

## Decision

### D1. Layout direktori `templates/` + `modules/<name>/` + `embed.FS`

Seluruh template hidup di bawah `templates/modules/` pada repo builder dan
di-embed lewat **satu** titik embed (`templates/templates.go` → `//go:embed
modules` → `var FS embed.FS`, ADR-002 §Decision-1). Satu **direktori per modul**;
nama direktori = `name` di manifest (unik global). Tiap direktori modul berisi:

```
templates/
├── <module-name>/
│   ├── module.yaml                 # manifest modul (wajib, satu per modul)
│   ├── <path>/<file>.tmpl          # file yang dirender modul (files[])
│   └── fragments/                  # fragmen merge ke file shared (contributes[])
│       └── <fragment>.tmpl
```

Pengelompokan modul mengikuti dimensi SPEC §4 (prefix kategori untuk
keterbacaan, bukan semantik): `core`, `arch-*`, `http-*`, `db-*`, `access-*`,
`migrate-*`, `comm-*`, `broker-*`, `addon-*`, `config-*`, `log-*` (lihat ADR-002
§Decision-1.1 untuk tabel penamaan modul kanonik lengkap). Prefix **tidak**
menyiratkan hierarki — relasi antar-modul dideklarasikan eksplisit lewat
`requires`/`conflicts`, bukan lewat nama folder. Penamaan lama
`core-monolith`/`core-microservice` DIHAPUS: skeleton lintas-arsitektur dimiliki
modul `core` netral, skeleton spesifik arsitektur dimiliki modul `arch-*`
(ADR-002 §Decision-1.1).

File shared (skeleton ber-anchor untuk merge) dimiliki oleh modul `core` atau
modul pemilik natural (mis. skeleton `docker-compose.yml` di `addon-docker`).

### D2. Skema `module.yaml` lengkap

`module.yaml` adalah cerminan 1:1 tipe `module.Manifest` (ADR-002 §Decision-3.2,
kontrak kanonik). Satu file YAML per direktori modul.

| Field | Tipe | Wajib | Deskripsi |
|---|---|---|---|
| `name` | string | ✅ | Identitas modul, unik global, = nama direktori. Dipakai di `requires`/`conflicts` modul lain dan untuk audit `FileOp.ModuleName`. |
| `description` | string | ✅ | Deskripsi singkat satu baris (dokumentasi & output dry-run). |
| `files` | list<FileSpec> | — | File yang dirender modul; 1 entri → 1 target. Kosong bila modul hanya menyumbang ke file shared. |
| `files[].template` | string | ✅ (per entri) | Path `.tmpl` relatif terhadap dir modul di `embed.FS`. |
| `files[].target` | string | ✅ (per entri) | Path relatif project hasil generate. Boleh mengandung placeholder template (mis. `internal/{{.Feature}}/handler.go`), dievaluasi resolver. |
| `files[].mode` | enum | — | `render` (render → tulis, default) · `copy` (salin apa adanya, tanpa render) · `mkdir` (buat direktori). Default `render`. String `mode` dipetakan ke `plan.FileOpMode` (`ModeRender`/`ModeCopy`/`ModeMkdir`) saat load (ADR-002 §Decision-3.3). **Merge tidak di sini** (lihat `contributes`, menghasilkan `ModeMerge`). |
| `files[].when` | string | — | Ekspresi kondisi opsional; grammar formal di ADR-002 §Decision-5 (lihat juga D4). Kosong = aktif selama modulnya aktif. |
| `gomod` | list<ModuleDep> | — | Dependency `go.mod` yang **dibawa** modul ini. |
| `gomod[].path` | string | ✅ (per entri) | Import path, mis. `github.com/go-chi/chi/v5`. |
| `gomod[].version` | string | ✅ (per entri) | Versi pin terverifikasi (riset 05 §2), mis. `v5.3.0`. |
| `requires` | list<string> | — | Nama modul prasyarat yang **harus** aktif bersama modul ini. |
| `conflicts` | list<string> | — | Nama modul yang **tidak boleh** aktif bersama modul ini. |
| `contributes` | list<MergeContribution> | — | Kontribusi ke file SHARED (selalu via assembler, `ModeMerge`). |
| `contributes[].target` | string | ✅ (per entri) | File shared tujuan, mis. `docker-compose.yml`. |
| `contributes[].anchor` | string | ✅ (per entri) | Nama section/anchor di skeleton file shared, mis. `services`. |
| `contributes[].template` (`fragment`) | string | ✅ (per entri) | Path fragmen `.tmpl` (lazim di `fragments/`). |
| `contributes[].order` | int | ✅ (per entri) | Urutan deterministik dalam satu anchor (tie-break: `name` modul). |
| `contributes[].when` | string | — | Kondisi opsional fragment-level; grammar formal di ADR-002 §Decision-5 (lihat juga D4). Kosong = aktif selama modulnya aktif. |
| `vars` | map<string,any> | — | Default var modul; digabung resolver ke proyeksi `Answers` menjadi context render (`plan.FileOp.Data`, ADR-002 §Decision-3.3). Bertipe `any` agar nilai non-string muat (mis. `DBPort: 5432`). |

> **Catatan istilah:** field `contributes[].template` disebut juga **fragment**;
> di tipe `module.MergeContribution` (ADR-002 §Decision-3.2) field ini bernama
> `Fragment` (alias YAML `template`/`fragment` keduanya memetakan ke sana).
> `contributes[].when` adalah **condition** yang sama semantiknya dengan
> `files[].when`. Keduanya dievaluasi resolver, bukan di dalam template.

#### Contoh manifest 1 — `core` (skeleton dasar, dimuat selalu)

```yaml
# templates/core/module.yaml
name: core
description: "Skeleton project dasar: go.mod, README, .gitignore, anchor file shared."

files:
  - template: "README.md.tmpl"
    target:   "README.md"
    mode:     "render"
  - template: "gitignore.tmpl"
    target:   ".gitignore"
    mode:     "render"

# CATATAN: go.mod TIDAK didaftarkan di files[]. Dependency dialirkan via field
# `gomod:` lalu dirakit generator via x/mod/modfile.Format (ADR-002 §Decision-6,
# "Perakitan go.mod"), BUKAN sebagai file render maupun fragment teks. Skeleton
# module/go directive berasal dari plan.GeneratePlan (ModulePath/GoVersion).
gomod: []        # core tidak membawa dependency pihak ketiga

requires: []
conflicts: []

# core MEMILIKI sejumlah file shared sebagai skeleton ber-anchor (kosong-by-default).
# Modul lain menyumbang fragmen ke anchor ini lewat `contributes`.
contributes:
  - target:   ".env.example"
    anchor:   "app"
    template: "fragments/env.app.tmpl"
    order:    0
  - target:   "Makefile"
    anchor:   "targets"
    template: "fragments/make.base.tmpl"
    order:    0
    when:     ".Makefile"

vars:
  GoVersion: "1.24"        # ditimpa C8 (fiber → 1.25) oleh resolver
```

#### Contoh manifest 2 — `http-chi` (HTTP framework, kelas `net/http`)

```yaml
# templates/http-chi/module.yaml
name: http-chi
description: "Router chi v5 (kelas net/http): router.go + middleware standar."

files:
  - template: "internal/transport/http/router.go.tmpl"
    target:   "internal/transport/http/router.go"
    mode:     "render"

gomod:
  - path:    "github.com/go-chi/chi/v5"
    version: "v5.3.0"          # pin terverifikasi 05 §2; dirakit via modfile (ADR-002 §Decision-6)

requires: []                    # chi tidak butuh modul lain
conflicts:
  - http-stdlib
  - http-gin
  - http-echo
  - http-fiber                  # fiber = kelas fasthttp, tak kompatibel (C7/C18)

contributes:
  - target:   "cmd/api/main.go"
    anchor:   "imports"
    template: "fragments/main.imports.chi.tmpl"
    order:    10
  - target:   "cmd/api/main.go"
    anchor:   "routes"
    template: "fragments/main.routes.chi.tmpl"
    order:    10

vars: {}
```

#### Contoh manifest 3 — `db-postgres` (driver + migrasi + compose)

```yaml
# templates/db-postgres/module.yaml
name: db-postgres
description: "PostgreSQL (pgxpool) + migrations + service compose + env."

files:
  - template: "internal/platform/database/postgres.go.tmpl"
    target:   "internal/platform/database/postgres.go"
    mode:     "render"
  - template: "migrations/0001_init.up.sql.tmpl"
    target:   "migrations/0001_init.up.sql"
    mode:     "render"
  - template: "migrations/0001_init.down.sql.tmpl"
    target:   "migrations/0001_init.down.sql"
    mode:     "render"

gomod:
  - path:    "github.com/jackc/pgx/v5"
    version: "v5.10.0"          # pin terverifikasi 05 §2; dirakit via modfile (ADR-002 §Decision-6)

requires: []
conflicts:
  - db-mysql
  - db-sqlite
  - db-mongo

# CATATAN: pgx TIDAK menyumbang ke target go.mod via contributes[]. Require go.mod
# selalu lewat field `gomod:` di atas (ADR-002 §Decision-6). contributes[] hanya
# untuk file shared teks (compose/env/Makefile/main wiring).
contributes:
  - target:   "docker-compose.yml"
    anchor:   "services"
    template: "fragments/compose.postgres.yml.tmpl"
    order:    20
    when:     ".Docker"         # C14: sqlite tak menyumbang service DB; postgres hanya bila docker aktif
  - target:   "docker-compose.yml"
    anchor:   "volumes"
    template: "fragments/compose.postgres.volume.yml.tmpl"
    order:    20
    when:     ".Docker"
  - target:   ".env.example"
    anchor:   "database"
    template: "fragments/env.postgres.tmpl"
    order:    20
  - target:   "Makefile"
    anchor:   "db-targets"
    template: "fragments/make.migrate.tmpl"
    order:    20
    when:     "ne .Migrate \"\""

vars:
  DBPort:  5432
  DBImage: "postgres:17-alpine"
```

### D3. Konvensi file `.tmpl`

- **Ekstensi.** Semua file template berekstensi `.tmpl`. Nama target = nama
  tanpa `.tmpl` (`postgres.go.tmpl` → `postgres.go`).
- **Path mapping.** Path target ditetapkan `files[].target` (untuk file modul) /
  skeleton owner (untuk file shared). Placeholder dinamis di `target` memakai
  sintaks template (mis. `internal/{{.Feature}}/handler.go`) dan dievaluasi
  resolver saat merakit `FileOp` — bukan saat render konten.
- **Gofmt wajib.** File `.go` hasil render **selalu** dilewatkan `go/format`
  (ADR-001 §3) → output rapi & deterministik meski template "kurang rapi".
- **Fragmen merge.** Hidup di `templates/<mod>/fragments/*.tmpl`, **tidak** punya
  entri `files:`, hanya dirujuk `contributes[].template`.
- **Delimiter.** Default `{{ }}`. Untuk file yang sendiri memakai `{{ }}` (mis.
  GitHub Actions `${{ }}`), gunakan delimiter alternatif `[[ ]]` per-file
  (didaftarkan di Fase 3). Pilihan delimiter alternatif = detail implementasi
  ADR-002, bukan keputusan arsitektural baru.
- **Custom `FuncMap`** (ADR-001 §3, tanpa sprig; ADR-002 §Decision-4 — tepat 6
  fungsi kanonik). Signature kanonik yang tersedia di seluruh template:

  | Fungsi | Contoh | Guna |
  |---|---|---|
  | `toCamel(s)` | `user_name` → `userName` | nama variabel Go |
  | `toPascal(s)` | `user_name` → `UserName` | nama tipe / exported |
  | `toSnake(s)` | `UserName` → `user_name` | nama kolom / file |
  | `toKebab(s)` | `UserName` → `user-name` | nama service / image |
  | `modBase(modulePath)` | `github.com/acme/shop` → `shop` | nama biner / package root |
  | `modJoin(modulePath, elem...)` | `(m, "services","user")` → import path | menyusun import path |

  Tiap fungsi `FuncMap` di-unit-test sendiri (ADR-001 §3; ADR-002 §Decision-9).

### D4. Conditional render (kapan modul aktif)

Dua lapis, **keduanya diputuskan di resolver** (bukan di dalam template) agar
`GeneratePlan` final & deterministik sebelum render — syarat dry-run akurat &
golden-file stabil (ADR-002 §Decision-2 alur data & §Decision-8 safety flow). Grammar
ekspresi `when` (lapis 2) bersifat **kanonik** di ADR-002 §Decision-5 (EBNF +
daftar field legal + semantik); D4 tidak mendefinisikan ulang, hanya memakainya:

1. **Module-level gating.** Resolver memilih himpunan modul aktif dari
   `Answers` via decision matrix (riset 05 §2). Modul yang tidak aktif **tidak**
   menyumbang `FileOp` maupun `MergeFragment` sama sekali. Contoh:
   - `db = none` → modul `db-*`, `access-*`, `migrate-*` mati (C2).
   - `kind = worker` → modul `http-*` mati (C9).
   - `arch ≠ microservice` → modul `comm-*`, `broker-*` mati (C5/C3).
2. **File/fragment-level `when`.** Untuk file atau kontribusi dalam modul yang
   **sudah aktif** tetapi masih bergantung kondisi lain. Ekspresi `when`
   dievaluasi resolver atas proyeksi `Answers`/`Vars`; hanya `FileOp`/
   `MergeFragment` yang lolos yang masuk `GeneratePlan`. Contoh:
   - `db-postgres` menyumbang ke `docker-compose.yml` **hanya** bila `Docker`
     aktif (`when: ".Docker"`).
   - target migrate di `Makefile` hanya bila `Migrate` terpilih
     (`when: "ne .Migrate \"\""`).

> `{{if}}` kecil di dalam `.tmpl` boleh untuk variasi **baris**; tetapi
> percabangan **struktural** (file/fragmen ada atau tidak) ditentukan resolver,
> bukan disembunyikan di template. Aturan ini menjaga `DryRunWriter.Planned` =
> persis byte yang akan ditulis (ADR-002 §Decision-8 safety flow).

### D5. MERGE file shared — model "fragment + assembler" dengan named anchors

**Keputusan: fragment + assembler dengan named anchors di skeleton template**
(BUKAN marker-comment di file output). Alasan: dry-run harus tahu konten final
tanpa menulis; golden-file harus deterministik; merge harus idempotent untuk
`add service`. Mekanisme ini adalah cerminan kontrak kanonik ADR-002:
`plan.FileOp{Mode: ModeMerge, Fragments: []plan.Fragment{...}}` (§Decision-3.3)
dirakit `MergeAssembler.Assemble(skeleton []byte, frags []plan.Fragment) ([]byte,
error)` (§Decision-3.4), dieksekusi per algoritma §Decision-6.

**Mekanisme (5 langkah):**

1. **File shared = skeleton ber-anchor.** Tiap file shared punya satu template
   skeleton (di `core` atau modul pemilik) yang mendefinisikan **named anchors** —
   section bernama tempat fragmen disisipkan. Contoh anchor: `docker-compose.yml`
   → `services` / `volumes` / `networks`; `Makefile` →
   `targets` / `db-targets` / `proto-targets`; `.env.example` → `app` /
   `database` / `broker`; `main.go` → `imports` / `wiring` / `routes`.
   **`go.mod` BUKAN file merge** (lihat langkah 5).
2. **Modul menyumbang fragmen** lewat `contributes[]` (`target`, `anchor`,
   `template`, `order`, `when`) — cermin `module.MergeContribution`
   (ADR-002 §Decision-3.2).
3. **Resolver mengumpulkan** semua kontribusi modul aktif yang lolos `when`
   (grammar ADR-002 §Decision-5), mengelompokkan per `(target, anchor)`,
   mengurutkan **stabil** by `order` lalu `name` modul (tie-break), me-render tiap
   fragmen menjadi `plan.Fragment{Anchor, Content, Order}`, dan menghasilkan satu
   `plan.FileOp{Mode: ModeMerge, TemplatePath: <skeleton>, Fragments: [...]}` per
   file shared.
4. **`MergeAssembler.Assemble(skeleton, frags)`** merender skeleton, lalu untuk
   tiap anchor menyisipkan `Fragment.Content` terurut → satu konten final tunggal
   (algoritma 5 langkah di ADR-002 §Decision-6). File `.go` hasil akhir dilewatkan
   `go/format` oleh generator.
5. **Kasus khusus `go.mod`.** `go.mod` **tidak** diproses sebagai `ModeMerge` dan
   **tidak** menjadi fragment teks. Kontribusi `require` dideklarasikan modul di
   field `gomod:` (bukan `contributes` ke target `go.mod`), dialirkan resolver ke
   `plan.Deps` (`[]plan.ModuleDep`, dedup + sort by `Path`) lalu dirakit generator
   via `x/mod/modfile.Format` (ADR-001 §4; ADR-002 §Decision-6 "Perakitan go.mod").
   Urutan `require` dinormalisasi (SPEC §5.2 poin 3). Wiring `main.go` tetap
   ditangani sebagai merge (fragmen ke anchor `imports`/`wiring`/`routes`), hasil
   akhir dilewatkan `go/format`.

**Anchor matrix (file shared → penyumbang).** `go.mod` sengaja **tidak** ada di
tabel ini: ia bukan file merge, dirakit terpisah via `plan.Deps` → `modfile.Format`
(langkah 5 di atas; ADR-002 §Decision-6).

| File shared | Anchor | Penyumbang (modul) |
|---|---|---|
| `docker-compose.yml` | `services` | `addon-docker` (app), `db-*` (kecuali sqlite, C14), `broker-*`, gateway, tiap `services/<svc>` |
| `docker-compose.yml` | `volumes` | `db-postgres`/`db-mysql`/`db-mongo`/`db-sqlite` (volume file) |
| `Makefile` | `db-targets` | `migrate-*` |
| `Makefile` | `proto-targets` | `comm-grpc` (microservice) |
| `.env.example` | `database` | `db-*` |
| `.env.example` | `broker` | `broker-*` |
| `cmd/.../main.go` | `imports` / `wiring` / `routes` | `db-*`, `addon-auth-jwt`, `addon-obs`, `http-*`, modul domain |

**Contoh konkret — `docker-compose.yml`, anchor `services`:**

*Skeleton (`core`/`addon-docker`), sebelum merge:*

```yaml
# fragments/compose.skeleton.yml.tmpl
services:
  # gostarter:services            ← named anchor (marker komentar netral, lihat catatan add service)
volumes:
  # gostarter:volumes
```

*Fragmen `db-postgres` (`fragments/compose.postgres.yml.tmpl`):*

```yaml
  postgres:
    image: {{ .DBImage }}
    environment:
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    ports:
      - "{{ .DBPort }}:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
```

*Hasil setelah merge (Docker aktif + db=postgres + app dari `addon-docker`, order app=10 < postgres=20):*

```yaml
services:
  app:
    build: .
    depends_on: [postgres]
    env_file: [.env]
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
```

**Contoh konkret — `go.mod` (BUKAN merge/anchor — dirakit via `modfile`):**

*Sebelum (require modul terkumpul di `plan.Deps`, `[]plan.ModuleDep`, dedup+sort
by `Path`):* `core` (kosong) + `http-chi` (`github.com/go-chi/chi/v5 v5.3.0`) +
`db-postgres` (`github.com/jackc/pgx/v5 v5.10.0`). Tiap dep berasal dari field
`gomod:` modulnya (bukan `contributes`).

*Sesudah (dirakit generator via `modfile.Format`, require dinormalisasi terurut —
ADR-002 §Decision-6):*

```
module github.com/acme/shop

go 1.24

require (
	github.com/go-chi/chi/v5 v5.3.0
	github.com/jackc/pgx/v5 v5.10.0
)
```

**Kenapa bukan marker-comment di file output (untuk generate awal):** marker di
file *output* menuntut membaca-balik file yang sudah ada untuk menyisipkan —
rapuh untuk dry-run dan rawan non-determinisme. Anchor di **skeleton template**
(sumber, bukan output) lebih bersih dan deterministik.

**Pengecualian `add service` (US-05):** karena project sudah ada di disk,
`add service` memang membaca `docker-compose.yml` existing. Untuk itu, generator
**menanam marker komentar netral** (mis. `# gostarter:services`) pada anchor saat
generate awal, sehingga penyisipan inkremental tetap deterministik & idempotent.
Marker berupa komentar YAML/Makefile yang sah, tidak memengaruhi runtime project,
dan tidak melanggar zero lock-in. Generator yang sama dipakai; merge bersifat
idempotent terhadap anchor (menjalankan `add service` dua kali untuk nama yang
sama ditolak di validasi, bukan menduplikasi blok — SPEC US-05 Sk.3).

### D6. Validasi manifest (`requires`/`conflicts`) saat load

`module.Registry.Load(fsys)` memvalidasi seluruh manifest saat memuat
`embed.FS` (kontrak `Registry` di ADR-002 §Decision-3.2). Pemeriksaan, fail-fast
bila gagal:

1. **Sintaks & field wajib.** `name`, `description` ada; tiap `files[]` punya
   `template`+`target`; tiap `gomod[]` punya `path`+`version`; tiap
   `contributes[]` punya `target`+`anchor`+`template`+`order`.
2. **Keunikan `name`.** Tidak ada dua modul bernama sama; `name` = nama
   direktori.
3. **Referensi `requires`/`conflicts` ada.** Setiap nama yang dirujuk harus
   merujuk modul yang benar-benar terdaftar di registry (tidak ada dangling).
4. **Konsistensi requires/conflicts.** Sebuah modul tidak boleh sekaligus
   `requires` dan `conflicts` modul yang sama. Relasi conflicts simetris secara
   logis (A conflicts B ⇒ B tak boleh aktif bersama A) — tidak harus dideklarasi
   dua arah, tetapi resolver memperlakukannya simetris.
5. **Anchor merujuk skeleton.** Setiap `contributes[].(target, anchor)` harus
   menunjuk file shared yang skeleton-nya mendefinisikan anchor tersebut.
6. **Path `template` ada di `embed.FS`.** Setiap `files[].template` dan
   `contributes[].template` benar-benar ada di subtree modul.

Validasi **antar-pilihan-user** (mis. C1 migrate↔db, C-mongo, C6 gateway↔arch)
**bukan** di sini — itu di `resolver.Resolve` atas himpunan modul aktif yang
sudah terpilih. `Load` memvalidasi **katalog** (integritas manifest); `Resolve`
memvalidasi **seleksi** (kombinasi user terhadap constraint matrix SPEC §6).
Saat `Resolve` memilih modul aktif, ia memeriksa `requires` (semua prasyarat
ikut aktif) dan `conflicts` (tidak ada pasangan bentrok) → bila gagal,
`ErrConstraint` fail-fast (SPEC §6.4 / Definition of Done #6).

### D7. Kebijakan versioning template (singkat)

Template di-versioning bersama binary builder (semver builder), bukan
per-modul. Aturan dampak:

- **PATCH** — perbaikan template yang **tidak** mengubah output untuk input yang
  sama: fix typo komentar, perbaikan formatting yang sudah dinormalisasi gofmt,
  perbaikan bug render yang menghasilkan output salah → benar.
- **MINOR** — penambahan kemampuan tanpa memecah kombinasi lama: modul template
  baru, anchor baru, opsi `when` baru, bump versi `gomod[].version` ke patch
  terbaru (praktik keamanan rutin, mis. pgx — riset 05 §2). Output kombinasi
  yang sudah ada **boleh** berubah byte (mis. versi dependency naik), tetapi
  tetap lolos Definition of Done.
- **MAJOR** — perubahan yang memecah kontrak: menghapus/merename modul atau
  anchor, mengubah layout output default suatu arsitektur, mengganti default
  dimensi (mis. default HTTP framework).

Golden-file (ADR-002 §Decision-9) di-regenerasi (`-update`) bersama setiap
perubahan yang mengubah output, dan diff golden ditinjau sebagai bagian review.

## Consequences

### Positif

- **Tanpa duplikasi kombinatorial.** Satu modul per-dimensi; kombinasi dirakit,
  bukan disalin per-kombinasi (composition by category, riset 04 §2.2).
- **Merge deterministik & byte-identical.** `order` + tie-break `name` +
  `modfile.Format` + `go/format` menjamin output stabil lintas mode
  (SPEC §5.2). Tidak ada ketergantungan pada urutan input flag/jawaban.
- **Dry-run akurat.** Semua percabangan struktural di resolver → `GeneratePlan`
  final sebelum render → preview = byte yang akan ditulis (SPEC US-06).
- **Idempotent `add service`.** Marker netral di anchor file shared memungkinkan
  penyisipan inkremental deterministik tanpa men-generate ulang (SPEC US-05).
- **Validasi dua lapis jelas.** `Load` (integritas katalog) vs `Resolve`
  (kombinasi user) memisahkan kegagalan author-template dari kegagalan input
  user — pesan error lebih tajam.
- **Zero lock-in & single-binary terjaga.** Template embedded; tidak ada artefak
  builder di project hasil generate (SPEC §6 N1).

### Negatif

- **Beban anchor & skeleton.** Setiap file shared butuh skeleton ber-anchor yang
  dirawat; menambah anchor baru menyentuh skeleton + manifest penyumbang.
  Mitigasi: anchor sedikit & stabil; divalidasi saat `Load` (D6 poin 5).
- **Marker komentar di output.** `add service` menanam komentar netral
  (`# gostarter:...`) di file shared. Komentar ini muncul di project hasil
  generate. Diterima karena netral (tidak memengaruhi runtime) dan diperlukan
  untuk idempotensi inkremental; bukan pelanggaran zero lock-in (bukan import).
- **`when` adalah mini-bahasa kondisi.** Ekspresi `when` perlu evaluator di
  resolver (Fase 3). Dijaga minimal (boolean field + `eq`/`ne` ringan); bukan
  bahasa ekspresi penuh, untuk menghindari kompleksitas & menjaga determinisme.
  Grammar formal & daftar field legal terkunci di ADR-002 §Decision-5.
- **Disiplin urutan wajib.** Lupa `order` atau tie-break tak konsisten dapat
  memecah byte-identical. Mitigasi: `order` wajib di manifest + test invarian
  byte-identical (ADR-002 §Decision-9).

## Alternatives Considered

- **Template monolitik per-kombinasi — DITOLAK.** Satu set template lengkap per
  kombinasi valid. Tidak terkelola (ledakan kombinatorial), duplikasi masif,
  mustahil dijaga konsisten. Bertentangan dengan composition by category yang
  jadi pembeda gostarter (riset 04 §2.2–§2.3, SPEC §2).
- **Go `embed` tanpa manifest (konvensi-folder murni) — DITOLAK.** Mengandalkan
  konvensi nama folder/file untuk menyimpulkan dimensi, dependency, dan relasi.
  Tidak bisa mendeklarasikan `gomod`, `requires`/`conflicts`, atau kontribusi
  merge secara eksplisit → relasi antar-modul jadi implisit & rapuh, validasi
  `Load` mustahil. Manifest `module.yaml` (SPEC §2 Glossary "Manifest modul")
  adalah sumber kebenaran eksplisit yang dibutuhkan generator.
- **Marker-comment di file OUTPUT untuk semua merge — DITOLAK (untuk generate
  awal).** Menyisipkan dengan membaca-balik file output yang sudah ada. Rapuh
  untuk dry-run (file belum ada) dan rawan non-determinisme. Anchor di skeleton
  *template* dipilih sebagai gantinya; marker output hanya dipakai terbatas untuk
  `add service` (project existing).
- **`Masterminds/sprig` untuk helper template — DITOLAK (rujuk ADR-001 §"Template
  engine").** Over-engineering untuk segelintir helper casing/path; menambah
  dependency tanpa nilai tambah. Helper ditulis sendiri di `FuncMap` (D3).
- **`a-h/templ` sebagai engine — DITOLAK (rujuk ADR-001).** Ditujukan untuk
  komponen HTML; tidak cocok meng-generate file `.go`/config arbitrer; menambah
  toolchain codegen yang tak diperlukan. Engine tetap stdlib `text/template` +
  `embed.FS` + `go/format`.

## References

- `docs/SPEC.md` — §2 (Glossary: Modul template, Manifest modul, needs-step,
  Dry-run), §4 (Question Flow), §5.2 (Invarian Byte-Identical), §5.4 (dry-run +
  proteksi overwrite), §6 (Constraint matrix C1/C2/C3/C6/C8/C9/C14/C-mongo),
  §7 (US-01..US-07, Definition of Done).
- `docs/adr/ADR-001-builder-stack.md` — keputusan stack: `text/template` +
  `embed.FS` + `FuncMap` + `go/format`; `x/mod/modfile`; penolakan sprig & templ.
- `docs/adr/ADR-002-internal-architecture.md` — **kontrak kanonik** yang dilebur
  dari dokumen BACKBONE: §Decision-1 (layout repo & §Decision-1.1 penamaan modul
  `core`/`arch-*`/fitur), §Decision-2 (alur data end-to-end), §Decision-3 (tipe
  inti Go-verbatim: `module.Manifest`/`FileSpec`/`MergeContribution`/`Registry`,
  `plan.FileOp`/`FileOpMode`/`Fragment`/`ModuleDep`, `Renderer`/`MergeAssembler`),
  §Decision-4 (FuncMap 6 fungsi), §Decision-5 (grammar `when`), §Decision-6 (model
  eksekusi merge + perakitan `go.mod` via `modfile.Format`), §Decision-8 (safety
  flow), §Decision-9 (testability/golden-file). ADR-003 merujuk ke §Decision-N ini
  sebagai sumber kebenaran — **tidak ada lagi dokumen BACKBONE terpisah**.
- `docs/research/04-competitors-tooling.md` §2.2–§2.3 — composition by category.
- `docs/research/05-decision-matrix.md` §2–§3 — pin versi dependency
  (`go-chi/chi/v5 v5.3.0`, `jackc/pgx/v5 v5.10.0`) + constraint matrix.
