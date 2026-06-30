# Menambah Modul Template (Adding Modules)

Panduan kontributor untuk menambah **modul template** baru ke gostarter — mulai
dari anatomi direktori, skema `module.yaml` field-demi-field, konvensi anchor
`region:<name>`, grammar `when`, hingga walkthrough end-to-end dan checklist PR.

> **Sumber kebenaran.** Dokumen ini adalah panduan operasional. Kontrak kanonik
> tetap di [`docs/SPEC.md`](./SPEC.md), [ADR-002](./adr/ADR-002-internal-architecture.md)
> (tipe inti, FuncMap, grammar `when`), dan [ADR-003](./adr/ADR-003-template-system.md)
> (skema `module.yaml`, anchor/merge, validasi, versioning). Bila dokumen ini
> berbeda dengan ADR/SPEC, **ADR/SPEC menang** — laporkan diskrepansinya.

---

## 1. Konsep singkat

gostarter merakit project dari **modul-modul template** per dimensi (arsitektur,
HTTP, DB, addon, dst.) alih-alih menyimpan satu set template lengkap per
kombinasi (*composition by category*, bukan ledakan kombinatorial). Setiap modul:

- **memiliki file sendiri** (`files[]` → satu `.tmpl` per target), dan/atau
- **menyumbang fragmen** ke file *shared* milik modul lain (`contributes[]` →
  fragmen disisipkan ke *named anchor* di skeleton), dan/atau
- **membawa dependency** `go.mod` (`gomod[]` → dirakit via `modfile.Format`).

Resolver mengaktifkan modul sesuai pilihan user (module-level gating), lalu
mengevaluasi `when` per file/fragmen (lapis kedua), menghasilkan `GeneratePlan`
deterministik yang dirender generator. Output selalu **zero lock-in**: tidak ada
import ke builder.

> **Contoh modul fitur yang bergantung pada satu pilihan akses (3 varian per-arch):**
> Add-on `strapgorm` aktif bila diminta DAN prasyarat keras terpenuhi:
> `access=gorm` ∧ `db ∈ {postgres, mysql}` (ditegakkan `answers.Validate`, dicermin
> resolver `checkConstraints` sebagai C-strapgorm). Bentuknya BEDA per arsitektur,
> diwujudkan sebagai modul fitur terpisah yang dipilih resolver per `arch`:
>
> - **`feature-strapgorm`** (`arch=monolith`) — domain `internal/product/**`, **REUSE**
>   `*gorm.DB` milik `access-gorm-<driver>` via package-global `product.SetDB(db)`
>   (fragmen wiring `order: 30` > `25` access-gorm — tanpa pool kedua); menyumbang
>   fragmen ke `region:imports`/`region:wiring` (`main.go`) + `region:imports`/`region:routes`
>   (`internal/httpserver/server.go`, FRAMEWORK-AWARE per `.HTTP`) untuk `GET /api/products`.
> - **`feature-strapgorm-modular`** (`arch=modular-monolith`) — Product jadi domain modular
>   kelas-satu `internal/modules/product/**` (facade + `internal/core` berduri), di-**inject**
>   `*gorm.DB` access=gorm lewat composition root (`product.New(db)`), didaftarkan ke
>   `httpserver.New(...)` via anchor `region:modules` (fragmen `main.go`). Pendaftaran rute
>   SERAGAM lintas HTTP framework (interface `httpserver.Module { RegisterRoutes(*http.ServeMux) }`).
> - **`feature-strapgorm-microservice`** (+ `-postgres`/`-mysql`) (`arch=microservice`) — service
>   `product` MANDIRI (gRPC Ping + HTTP `/api/products`) dgn koneksi GORM **per-service** (BUKAN
>   reuse); modul shared meng-emit kode + proto + fragmen compose service, modul `-<driver>`
>   menyumbang gomod driver + service DB compose (TANPA file `.go` → go.mod jujur, hanya driver
>   terpilih). Driver di-import `store.go` lewat branch `{{ if eq .DB "mysql" }}`.
>
> Modul-modul ini sengaja **tidak** mendeklarasikan `requires: access-gorm-<driver>` (kedua
> driver saling-konflik) — prasyarat di-gate sebelum resolver, dan resolver hanya
> mengaktifkannya pada kombinasi yang membawa driver yang benar. Lihat manifestnya sebagai
> pola modul fitur add-on yang murni aditif di atas pilihan akses + arsitektur tertentu.

---

## 2. Anatomi sebuah modul

Setiap modul hidup di satu direktori di bawah `templates/modules/<name>/`,
di-embed lewat `templates.FS` (`embed.FS`). Struktur lazimnya:

```
templates/modules/<name>/
├── module.yaml                 # manifest WAJIB (skema §3) — satu per modul
├── <path>/<file>.tmpl          # file yang DIMILIKI modul (files[])
│   └── …                       # path .tmpl relatif terhadap dir modul
└── fragments/                  # konvensi: fragmen merge (contributes[])
    └── <fragment>.tmpl         # disisipkan ke anchor file shared milik modul lain
```

Contoh nyata (`db-postgres`):

```
templates/modules/db-postgres/
├── module.yaml
├── database/postgres.go.tmpl                  # files[] → internal/platform/database/postgres.go
├── migrations/0001_init.up.sql.tmpl           # files[] (mode: copy)
├── migrations/0001_init.down.sql.tmpl         # files[] (mode: copy)
└── fragments/
    ├── compose.postgres.service.yml.tmpl      # → docker-compose.yml anchor services
    ├── compose.postgres.volume.yml.tmpl       # → docker-compose.yml anchor volumes
    ├── env.postgres.tmpl                       # → .env.example anchor database
    ├── main.imports.postgres.tmpl              # → main.go anchor imports
    ├── main.wiring.postgres.tmpl               # → main.go anchor wiring
    └── make.migrate.tmpl                       # → Makefile anchor targets
```

Modul yang hanya menukar satu file bisa jauh lebih ramping (`http-echo` =
`module.yaml` + satu `httpserver/server.go.tmpl`).

**Konvensi penamaan modul (kanonik, ADR-002 §1.1):** `name` = nama direktori,
unik global, prefiks per dimensi: `arch-*`, `http-*`, `db-*`, `addon-*`,
`broker-*`. Contoh: `http-gin`, `db-sqlite`, `addon-taskfile`.

---

## 3. Skema `module.yaml` (field-demi-field)

`module.yaml` adalah cerminan 1:1 tipe `module.Manifest`
([ADR-002 §3.2](./adr/ADR-002-internal-architecture.md), skema
[ADR-003 D2](./adr/ADR-003-template-system.md)). Satu file YAML per direktori.

| Field | Tipe | Wajib | Deskripsi |
|---|---|---|---|
| `name` | string | ✅ | Identitas unik global, **= nama direktori**. Dipakai di `requires`/`conflicts` modul lain dan untuk audit `FileOp.ModuleName`. |
| `description` | string | ✅ | Deskripsi satu baris (dokumentasi + output dry-run). |
| `files` | list&lt;FileSpec&gt; | — | File yang dimiliki modul; 1 entri → 1 target. Kosong bila modul hanya menyumbang fragmen. |
| `gomod` | list&lt;ModuleDep&gt; | — | Dependency `go.mod` yang dibawa modul. |
| `requires` | list&lt;string&gt; | — | Nama modul prasyarat yang **harus** aktif bersama. |
| `conflicts` | list&lt;string&gt; | — | Nama modul yang **tidak boleh** aktif bersama. |
| `contributes` | list&lt;MergeContribution&gt; | — | Kontribusi fragmen ke file shared (→ `ModeMerge`). |
| `vars` | map&lt;string,any&gt; | — | Default var modul; digabung ke context render. `any` agar nilai non-string muat (mis. `DBPort: 5432`). |

### 3.1 `files[]` — `FileSpec`

```yaml
files:
  - template: database/postgres.go.tmpl       # path .tmpl relatif dir modul (WAJIB)
    target:   internal/platform/database/postgres.go   # path di project hasil (WAJIB)
    mode:     render                            # render | copy | mkdir (default: render)
    when:     "eq .DB \"postgres\""             # kondisi opsional (kosong = aktif selalu)
```

| Field | Wajib | Catatan |
|---|---|---|
| `template` | ✅ | Path `.tmpl` relatif dir modul; **harus ada** di `embed.FS` (divalidasi `Load`). |
| `target` | ✅ | Path relatif project hasil. Boleh memuat placeholder template (mis. `cmd/{{ modBase .ModulePath }}/main.go`, `services/{{ .Service }}/cmd/main.go`) yang dievaluasi resolver. |
| `mode` | — | `render` (default): render → `go/format` bila `.go` → tulis. `copy`: salin byte apa adanya **tanpa** render/gofmt (sumber WAJIB sudah gofmt-clean). `mkdir`: buat direktori. |
| `when` | — | Ekspresi kondisi (§5). Kosong = aktif selama modulnya aktif. |

**Aturan `mode`:**
- `render` untuk file dinamis (mengandung `{{ }}`) atau `.go` yang perlu
  di-gofmt setelah substitusi.
- `copy` untuk file **statik murni** (tanpa `{{ }}`) — `.sql`, atau `.go` yang
  sudah final & gofmt-clean. `copy` **tidak** melewati `go/format`, jadi sumber
  wajib valid apa adanya.
- `go.mod` **TIDAK** pernah di `files[]` — ia dirakit dari `gomod[]` via
  `modfile.Format` (lihat §3.2).

### 3.2 `gomod[]` — `ModuleDep`

```yaml
gomod:
  - path:    github.com/go-chi/chi/v5     # import path (WAJIB)
    version: v5.3.0                         # versi pin terverifikasi (WAJIB)
```

- **Hanya dependency yang BENAR-BENAR di-import** oleh kode modul. Dep yang
  dideklarasikan tetapi tak di-import akan **di-prune `go mod tidy`** → `go.mod`
  project tidak jujur (kegagalan e2e). Tambahkan dep bersamaan dengan kode yang
  meng-import-nya.
- Versi adalah **pin terverifikasi** (cek `pkg.go.dev` / `go list -m`). Catat
  tanggal verifikasi di komentar manifest.
- `go.mod` **bukan** target merge dan **bukan** fragmen. `require` dirakit
  resolver → `plan.Deps` (dedup + sort by path) → generator `modfile.Format`
  (ADR-003 D5 langkah 5).
- Tool CLI yang dipakai lewat Makefile (mis. `golang-migrate`) **tidak** masuk
  `gomod[]` — ia bukan import.

### 3.3 `requires` / `conflicts`

```yaml
requires:
  - core            # modul prasyarat (menyediakan anchor yang kita sumbang)
conflicts:
  - http-echo       # framework HTTP saling eksklusif
  - http-gin
```

- `requires`: kalau modul ini menyumbang ke anchor milik `core`/modul lain,
  deklarasikan prasyaratnya supaya anchor pasti ada.
- `conflicts`: untuk dimensi *mutual exclusive* (satu HTTP framework, satu DB
  driver, satu arsitektur). Relasi conflicts **simetris secara logis** — resolver
  memperlakukan A↔B dua arah meski dideklarasikan sekali.
- Sebuah modul **tidak boleh** sekaligus `requires` dan `conflicts` modul yang
  sama (divalidasi `Load`, ADR-003 D6 poin 4).
- Constraint **antar-pilihan user** (mis. migrate butuh db) **bukan** di sini —
  itu di `resolver.Resolve` atas himpunan modul terpilih (constraint matrix
  SPEC §6).

### 3.4 `contributes[]` — `MergeContribution`

```yaml
contributes:
  - target:   docker-compose.yml          # file shared tujuan (WAJIB)
    anchor:   services                      # nama anchor di skeleton (WAJIB)
    fragment: fragments/compose.postgres.service.yml.tmpl  # fragmen .tmpl (WAJIB)
    order:    20                            # urutan deterministik dalam anchor (WAJIB)
    when:     ".Docker"                     # kondisi opsional
```

| Field | Wajib | Catatan |
|---|---|---|
| `target` | ✅ | File shared tujuan (mis. `docker-compose.yml`, `.env.example`, `Makefile`, `cmd/{{ modBase .ModulePath }}/main.go`, `internal/httpserver/server.go`). |
| `anchor` | ✅ | Nama anchor `region:<name>` di skeleton `target` (§4). Pasangan `(target, anchor)` **harus** menunjuk skeleton yang mendefinisikan anchor itu (divalidasi `Load`, D6 poin 5). |
| `fragment` | ✅ | Path fragmen `.tmpl`. **Alias `template` juga sah** — keduanya memetakan ke field `Fragment`. Konvensi: simpan di `fragments/`. |
| `order` | ✅ | Urutan dalam satu `(target, anchor)`. Tie-break: **nama modul**. Pakai konvensi order yang konsisten (lihat catatan di bawah). |
| `when` | — | Kondisi fragment-level (§5). Kosong = aktif selama modulnya aktif. |

**Konvensi `order` (lihat manifest nyata):** `core`/skeleton owner = `0`,
addon-env = `5`, arsitektur per-service = `10`, db-* / http-* wiring = `20`,
addon-observability = `30`. Order menjaga blok deterministik (mis. var dasar env
sebelum blok OTEL). Bila ragu, ikuti modul sejenis yang sudah ada.

### 3.5 `vars`

```yaml
vars:
  DBPort: 5432        # int — boleh non-string (map[string]any)
  DBName: app
```

Default var modul, digabung resolver ke proyeksi `Answers` menjadi context render
(`FileOp.Data`). Dirujuk di template via `{{ .DBPort }}`. Jangan ulang field yang
sudah ada di `Answers` kecuali memang override default modul.

---

## 4. Konvensi anchor `region:<name>` dan model merge

File *shared* (mis. `docker-compose.yml`, `.env.example`, `Makefile`, `main.go`,
`internal/httpserver/server.go`) dimiliki oleh **satu modul skeleton** (`core`
atau pemilik natural, mis. microservice memiliki compose-nya sendiri). Skeleton
mendeklarasikan **named anchor** — komentar `region:<name>` yang sah di bahasa
file (komentar YAML/Makefile/Go) — sebagai titik sisip fragmen.

Mekanisme (ADR-003 D5): resolver mengumpulkan semua `contributes[]` modul aktif
yang lolos `when`, mengelompokkan per `(target, anchor)`, mengurutkan stabil by
`order` lalu `name`, me-render tiap fragmen, lalu `MergeAssembler` menyisipkan
konten ke anchor → satu file final tunggal (file `.go` dilewatkan `go/format`).

**Penting:** anchor ada di **skeleton (sumber template)**, bukan di file output.
Merge tahu konten final tanpa membaca-balik output → dry-run akurat, golden
deterministik. (Pengecualian: `add service` menanam marker netral di project
*existing* untuk penyisipan idempoten — bukan jejak builder, tidak melanggar
zero lock-in.)

### 4.1 Daftar anchor standar

| File shared | Anchor standar | Penyumbang lazim |
|---|---|---|
| `docker-compose.yml` | `services`, `volumes` | `addon-docker` (app), `db-*` (kecuali sqlite), `broker-*`, gateway, tiap `services/<svc>` |
| `.env.example` | `app`, `database`, `broker` | `core`/`addon-env` (app), `db-*` (database), `broker-*` (broker), `addon-observability` (app) |
| `Makefile` | `targets` (juga `db-targets`, `proto-targets`) | `addon-makefile`, `db-*` (target migrate), `comm-grpc` (proto) |
| `cmd/.../main.go` | `imports`, `wiring`, `routes` | `db-*`, `http-*`, `addon-observability`, modul domain |
| `internal/httpserver/server.go` | `imports`, `routes` | `addon-observability`, modul yang menambah middleware/route |

Bentuk marker di skeleton (komentar sesuai bahasa file):

```yaml
# docker-compose.yml.tmpl (skeleton)
services:
  # region:services
volumes:
  # region:volumes
```

```go
// main.go.tmpl (skeleton)
import (
    // region:imports
)
func main() {
    // region:wiring
}
```

Fragmen menambahkan baris pertamanya sebagai komentar penanda asal (konvensi,
bukan wajib): `# region:services (db-postgres) — service PostgreSQL.`

### 4.2 Menambah anchor baru

Bila modul butuh anchor yang belum ada: tambahkan `region:<name>` ke **skeleton**
file shared (di modul pemiliknya), lalu rujuk dari `contributes[].anchor`. `Load`
memvalidasi anchor↔skeleton (D6 poin 5) — anchor yang dirujuk tanpa ada di
skeleton = fail-fast. Jaga anchor **sedikit & stabil**.

---

## 5. Grammar `when` (ringkas)

`when` adalah mini-bahasa kondisi **boolean** yang dievaluasi di **resolver**
(bukan di dalam `.tmpl`), sehingga `GeneratePlan` final sebelum render. Grammar
formal lengkap + EBNF + daftar field legal ada di
[ADR-002 §5](./adr/ADR-002-internal-architecture.md). Ringkasnya:

- **Atom:** field bool (`.Docker`), atau call biner `eq`/`ne` atas field string
  vs literal (`eq .Arch "monolith"`, `ne .Migrate ""`).
- **Logika:** `and`, `or`, `not`, kurung `( )`. Presedensi: `not` > `and` > `or`.
- **Kosong** (`when: ""` atau absen) = **selalu true**.
- **Tidak ada** `lt/gt/le/ge`, aritmetika, atau pemanggilan FuncMap — sengaja
  minimal demi determinisme.
- **Hanya field di daftar legal** (ADR-002 §5.2: `.Arch`, `.HTTP`, `.DB`,
  `.Migrate`, `.Comm`, `.Docker`, `.Makefile`, `.Obs`, `.EnvExample`, `.Gateway`,
  dst.) — merujuk field di luar daftar → `ErrConstraint` fail-fast.

Contoh nyata:

```yaml
when: ".Docker"                                       # fragment compose hanya bila Docker aktif
when: "ne .Migrate \"\""                               # target migrate hanya bila tool migrasi terpilih
when: "eq .HTTP \"chi\""                               # file server.go versi chi
when: "and .Makefile (ne .Migrate \"\")"               # target make hanya bila Makefile + migrate
when: "and (eq .Arch \"monolith\") (eq .HTTP \"net/http\")"  # gate ganda (cegah double-ownership)
```

> **Module-level gating mendahului `when`.** Modul aktif/mati ditentukan dulu di
> resolver; `when` hanya dievaluasi untuk modul yang sudah aktif (lapis 2). Kalau
> file/fragmen `when`-nya false, ia **tidak** menghasilkan `FileOp`/`Fragment`
> sama sekali (bukan sekadar di-comment).

---

## 6. FileOp.Data dan DataOverride (per-instance / per-service)

Context render template adalah `FileOp.Data` (proyeksi `Answers` + `Vars` modul).
Untuk template yang dirender **lebih dari sekali dengan nilai berbeda** (mis. satu
template entrypoint service di-emit per nama service microservice), resolver
memakai **`DataOverride`** — map yang di-*merge* di atas `Data` saat render (key
override menang).

Penanda kontrak per-service di manifest = placeholder `{{ .Service }}` pada
`target`/`when` (lihat `arch-microservice`):

```yaml
files:
  # PER-SERVICE: target memuat {{ .Service }} → resolver emit sekali per nama service
  - template: services/cmd/main.go.tmpl
    target:   services/{{ .Service }}/cmd/main.go
    mode:     render
  # Gateway hanya untuk service pertama (.IsFirst di-set lewat DataOverride)
  - template: services/internal/gateway/gateway.go.tmpl
    target:   services/{{ .Service }}/internal/gateway/gateway.go
    mode:     render
    when:     ".IsFirst"
```

Resolver meng-emit set per-service sekali per nama di `Answers.Services`, dengan
`DataOverride` menambah field render khusus:

- `.Service` (string) — nama service yang sedang dirender (mis. `"order"`),
- `.IsFirst` (bool) — `true` hanya untuk service pertama,
- `.Others` ([]string) — nama service lain (untuk inter-service call).

Field global (`.ModulePath`, `.DB`, `.Docker`, dst.) tetap diwarisi dari `Data`
induk. `Fragment.DataOverride` bekerja sama untuk merge per-service (mis. satu
compose-service per service ke anchor `services`). Sebagian besar modul **tidak**
perlu ini — pakai hanya untuk pola fan-out per-instance.

---

## 7. Aturan zero lock-in (WAJIB)

Project hasil generate **tidak boleh** meng-import apa pun dari builder, dan
**tidak boleh** menyebut builder di output (SPEC §6 N1, §7.4).

- ❌ Jangan tulis header `// Code generated by gostarter` atau sebut "gostarter"
  di file output (komentar, string, dll.).
- ❌ Jangan import package builder (`github.com/faisalcayunda/gostarter/...`)
  dari template output.
- ✅ Output hanya boleh import: stdlib, library publik pihak ketiga (sesuai
  `gomod[]`), dan package milik project itu sendiri (`{{ modJoin .ModulePath … }}`).
- ✅ **Pengecualian sah:** header `// Code generated by protoc-gen-go` di stub
  `gen/*.pb.go` microservice adalah konvensi protobuf yang sah — bukan jejak
  builder.
- ✅ Marker `# gostarter:<anchor>` yang ditanam `add service` di file shared
  *existing* adalah komentar netral untuk idempotensi inkremental — bukan import,
  tidak melanggar lock-in.

Verifikasi: e2e menjalankan `go vet ./... && go build ./... && go test ./...`
pada project hasil generate **tanpa edit manual**, dan memastikan tidak ada import
builder.

---

## 8. Cara menguji modul baru

Empat lapis pengujian, dari cepat ke menyeluruh:

1. **Validasi katalog (`Load` / `validateCatalog`).**
   `module.Registry.Load(templates.FS)` mem-parse + memvalidasi seluruh manifest
   (field wajib, keunikan `name`=dir, referensi `requires`/`conflicts` tak
   dangling, anchor↔skeleton, path template ada). Jalankan:
   ```
   go test ./internal/module/
   ```
   `TestLoad_RealCatalog` memuat katalog nyata — modul baru yang manifestnya cacat
   gagal di sini lebih dulu (fail-fast, ADR-003 D6).

2. **Test resolver + guard anti-drift.**
   Resolver test (`internal/resolver/`) memverifikasi keputusan struktural
   (modul aktif, `when`, file/fragment yang dihasilkan, constraint). Bila modul
   baru dipakai di fixture fake (`mvpRegistry`), **wajib** ditambahkan ke
   `fakeModulesUnderGuard` — `TestFakeRegistryMatchesRealManifests` adalah GUARD
   ANTI-DRIFT: ia membandingkan fixture fake vs manifest NYATA (GoMod eksak;
   Files/Contributes eksak/subset). Fixture yang menyimpang dari manifest (mis.
   versi dep) → test gagal sampai diselaraskan.
   ```
   go test ./internal/resolver/
   ```

3. **Golden snapshot (`-update`).**
   Bila modul masuk kombinasi yang di-snapshot, regenerasi & tinjau diff golden:
   ```
   make golden-update     # = UPDATE_GOLDEN=1 go test ./internal/golden/
   make golden-verify     # mode banding (CI gate) — wajib hijau
   ```
   Diff golden **ditinjau** sebagai bagian review — golden harus berubah hanya
   sesuai perubahan output yang dimaksud. Untuk kombinasi baru, tambahkan
   `goldenCase` di `internal/golden/golden_test.go` lalu `-update`.

4. **E2E `generate → go build`.**
   Script e2e men-generate project nyata lalu `go vet/build/test` (+ inter-service
   untuk microservice). Modul yang menambah dimensi baru sebaiknya masuk matrix:
   ```
   make e2e               # matrix-4a + microservice-e2e + smoke
   ```
   Untuk kombinasi ber-DB / non-stdlib, `go mod tidy` butuh jaringan (e2e
   menanganinya); Docker build di-SKIP bila daemon absen.

Gerbang penuh sebelum PR: `make ci` (= `lint test golden-verify e2e`).

---

## 9. Walkthrough: menambah modul `db-sqlite`

Contoh end-to-end menambah driver DB baru (`db-sqlite`) — pola identik untuk
`http-gin` (tukar `server.go` + `gomod` framework, gate `when: eq .HTTP "gin"`).

**Tujuan.** DB embedded `database/sql` + `modernc.org/sqlite` (pure-Go, tanpa
cgo), aktif bila `db=sqlite`. Tidak menyumbang service ke compose (file-based,
bukan service) — tapi tetap menyumbang env (path file DB) + wiring main + migrasi.

### Langkah

1. **Buat direktori & manifest.**
   ```
   templates/modules/db-sqlite/
   ├── module.yaml
   ├── database/sqlite.go.tmpl
   ├── migrations/0001_init.up.sql.tmpl      # mode: copy
   ├── migrations/0001_init.down.sql.tmpl    # mode: copy
   └── fragments/
       ├── env.sqlite.tmpl                    # → .env.example anchor database
       ├── main.imports.sqlite.tmpl           # → main.go anchor imports
       ├── main.wiring.sqlite.tmpl            # → main.go anchor wiring
       └── make.migrate.tmpl                  # → Makefile anchor targets
   ```

2. **`module.yaml`** (cermin `db-postgres`, sesuaikan):
   ```yaml
   name: db-sqlite
   description: >-
     SQLite (database/sql + modernc.org/sqlite, pure-Go): skeleton koneksi dari
     path file env + migrasi awal, wiring init DB ke main, target migrate Makefile.
   files:
     - template: database/sqlite.go.tmpl
       target:   internal/platform/database/sqlite.go
       mode:     render
     - template: migrations/0001_init.up.sql.tmpl
       target:   migrations/0001_init.up.sql
       mode:     copy
     - template: migrations/0001_init.down.sql.tmpl
       target:   migrations/0001_init.down.sql
       mode:     copy
   gomod:
     - path:    modernc.org/sqlite
       version: v1.XX.X            # PIN terverifikasi (pkg.go.dev, catat tanggal)
   requires:
     - core
   conflicts:                       # DB driver mutual exclusive
     - db-postgres
     - db-mysql
     - db-mongo
   contributes:
     - target:   .env.example
       anchor:   database
       fragment: fragments/env.sqlite.tmpl
       order:    20
       when:     ".EnvExample"
     - target:   cmd/{{ modBase .ModulePath }}/main.go
       anchor:   imports
       fragment: fragments/main.imports.sqlite.tmpl
       order:    20
     - target:   cmd/{{ modBase .ModulePath }}/main.go
       anchor:   wiring
       fragment: fragments/main.wiring.sqlite.tmpl
       order:    20
     - target:   Makefile
       anchor:   targets
       fragment: fragments/make.migrate.tmpl
       order:    20
       when:     "and .Makefile (ne .Migrate \"\")"
   vars:
     DBFile: app.db
   ```
   > Catatan: SQLite **tidak** menyumbang ke `docker-compose.yml` `services`/
   > `volumes` (file-based) — sengaja hilangkan kontribusi compose (analog C14
   > untuk sqlite di anchor matrix ADR-003).

3. **Tulis template & fragmen.** `database/sqlite.go.tmpl` meng-import **hanya**
   dep yang dideklarasikan di `gomod[]` (kalau di-import tapi tak dipakai → prune
   `go mod tidy`). Fragmen diawali komentar penanda `region`:
   ```
   # region:database (db-sqlite) — path file SQLite.
   ```
   File `.sql` statik → `mode: copy`, pastikan gofmt-irrelevant tapi valid.
   **Jangan** sebut "gostarter" / import builder di mana pun.

4. **Daftarkan aktivasi di resolver** (bila DB driver dipetakan dari `Answers.DB`
   di resolver — ikuti pola `db-postgres`/`db-mysql`). Tambah `db-sqlite` ke
   mapping module-level gating bila perlu, dan ke constraint matrix (migrate↔db).

5. **Uji berlapis:**
   ```
   go test ./internal/module/        # validateCatalog: manifest valid?
   go test ./internal/resolver/      # resolver + guard anti-drift
   ```
   Bila `db-sqlite` masuk fixture fake: tambahkan ke `mvpRegistry` **dan**
   `fakeModulesUnderGuard` (GoMod eksak), jika tidak guard fail-fast.

6. **Golden + e2e:**
   ```
   make golden-update && make golden-verify   # tinjau diff
   make e2e                                    # build hijau project hasil
   ```
   Tambah kombinasi `db=sqlite` ke `goldenCase` dan/atau matrix e2e bila ingin
   coverage permanen.

7. **Gerbang final & lint:**
   ```
   make ci      # lint + test + golden-verify + e2e — wajib hijau
   ```

### Checklist PR modul baru

- [ ] Direktori `templates/modules/<name>/` dengan `module.yaml` (skema §3).
- [ ] `name` = nama direktori, unik, prefiks dimensi (`arch-`/`http-`/`db-`/`addon-`/`broker-`).
- [ ] `files[]`: `template`+`target` ada; `mode` benar (`copy` untuk statik, `render` untuk dinamis/.go); placeholder target memakai FuncMap (`modBase`/`modJoin`).
- [ ] `gomod[]`: **hanya** dep yang benar-benar di-import; versi pin terverifikasi (komentar + tanggal); tidak akan di-prune `go mod tidy`.
- [ ] `requires`/`conflicts`: prasyarat anchor dideklarasikan; mutual-exclusive lengkap; tidak ada modul di `requires` ∩ `conflicts`.
- [ ] `contributes[]`: `target`+`anchor`+`fragment`+`order` ada; anchor benar-benar ada di skeleton (`region:<name>`); `order` konsisten dengan konvensi; `when` (jika ada) memakai field legal saja.
- [ ] Fragmen di `fragments/`, diawali komentar penanda `region:<name> (<modul>) — …`.
- [ ] **Zero lock-in**: tidak ada import builder / sebutan "gostarter" di output (kecuali marker netral `add service`); pengecualian `protoc-gen-go` hanya untuk microservice.
- [ ] `go test ./internal/module/` (validateCatalog) hijau.
- [ ] `go test ./internal/resolver/` hijau; fixture fake + `fakeModulesUnderGuard` diselaraskan (guard anti-drift).
- [ ] `make golden-update` → diff golden ditinjau & sesuai maksud; `make golden-verify` hijau.
- [ ] `make e2e` hijau (build project hasil `go vet/build/test` tanpa edit manual).
- [ ] `make ci` hijau (gerbang penuh).
- [ ] Conventional commit + PR menjelaskan dimensi/kombinasi baru dan diff golden.

---

## 10. Referensi

- [`docs/SPEC.md`](./SPEC.md) — §2 (Glossary), §5.2 (byte-identical), §6 (constraint matrix), §6 N1 (zero lock-in).
- [ADR-002](./adr/ADR-002-internal-architecture.md) — §3.2 (tipe `Manifest`/`FileSpec`/`MergeContribution`), §4 (FuncMap), §5 (grammar `when`), §6 (eksekusi `GeneratePlan`), §9 (testability).
- [ADR-003](./adr/ADR-003-template-system.md) — D2 (skema `module.yaml`), D4 (conditional render), D5 (merge + anchor), D6 (validasi `Load`), D7 (versioning).
- [`CONTRIBUTING.md`](../CONTRIBUTING.md) — alur kerja kontributor, target `make`, pre-commit.
