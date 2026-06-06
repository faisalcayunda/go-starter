---
status: Proposed
date: 2026-06-06
deciders: [isal]
tags: [builder, architecture, go]
relates: [ADR-001, ADR-003]
---

# ADR-002: Arsitektur Internal Builder gostarter

## Status

**Proposed** — 2026-06-06.

ADR ini mengikat ke **ADR-001** (stack builder: `cobra` + `huh` + `text/template`
/`embed.FS` + `x/mod/modfile` + `go/format`) dan menjadi **acuan kontrak** bagi
**ADR-003** (sistem template & merge). ADR-002 **melebur** dokumen kerja
"BACKBONE Desain Internal Builder" yang sebelumnya dirujuk ADR-003: seluruh
materi BACKBONE §1 (layout), §2.5/§2.6 (Manifest, FuncMap, assembler), §4 (sistem
template & merge), §5 (safety), §6 (testability) kini hidup di sini. Setelah ADR
ini, ADR-003 cukup merujuk **ADR-002 §X** — tidak ada lagi dokumen BACKBONE
terpisah.

Kontrak tipe/interface di **§Decision 3** bersifat **kanonik dan final untuk Fase
2**: skeleton Go di `internal/**` dan contoh manifest di ADR-003 **wajib**
diselaraskan persis ke kontrak ini (penyelarasan skeleton & ADR-003 adalah
follow-up langsung dari ADR ini; lihat Consequences).

## Context

`gostarter` adalah satu binary builder yang men-generate project Go best-practice
untuk tiga arsitektur (monolith / modular-monolith / microservice), dalam dua mode
input yang **wajib menghasilkan output byte-identical**: wizard interaktif (`huh`)
dan flag non-interaktif (`cobra`) — SPEC §5.2. Hasil generate wajib lolos
`go vet ./... && go build ./... && go test ./...` tanpa edit manual, `docker
compose up` jalan untuk stack ber-DB, dan **tidak meng-import apa pun dari
builder** (zero lock-in, SPEC §6 / §8 N1).

ADR-001 sudah mengunci **stack**; ADR-003 sudah mengunci **bentuk data template &
merge** (`module.yaml`, anchor, fragment). Yang belum dikunci — dan menjadi tujuan
ADR ini — adalah **arsitektur internal builder**: bagaimana package disusun,
bagaimana data mengalir dari prompt sampai disk, dan **kontrak tipe/interface inti
yang Go-verbatim** yang menjadi sumber kebenaran tunggal bagi seluruh skeleton.

Tekanan desain yang mengikat ADR ini:

1. **Satu struct konfigurasi tunggal** (`answers.Answers`) ditulis oleh kedua mode
   input → satu jalur resolusi & render → byte-identical (SPEC §5.2 poin 1–2).
2. **Keputusan struktural diambil sebelum render.** Percabangan "file ada/tidak"
   harus final di `plan.GeneratePlan` agar `--dry-run` akurat (SPEC §5.4 / US-06)
   dan golden-file stabil (ADR-003 D4).
3. **Determinisme byte-for-byte.** Urutan file, urutan fragment per anchor, dan
   urutan `require` di `go.mod` tidak boleh bergantung urutan input (SPEC §5.2
   poin 3; ADR-001 §3–§4 → `go/format` + `modfile.Format`).
4. **Zero lock-in & single-binary.** Template di-embed (`embed.FS`); dependency
   builder (`cobra`/`huh`/`modfile`) tidak pernah masuk `go.mod` output.
5. **Permukaan eksternal sempit & ter-mock.** `huh`/`cobra`/`os` diisolasi di
   balik interface (`Prompter`, `Writer`, `PostGenHook`) agar core (resolver +
   generator) dapat diuji tanpa TTY/disk/toolchain.

> **Catatan divergensi yang diselesaikan ADR ini.** Skeleton Fase 2 saat ini
> memakai penamaan lama (`plan.FileOpKind`/`OpRender`/`OpMkdir`/`OpMerge`,
> `module.Manifest.Files []string`, `module.Manifest.Vars map[string]string`,
> tanpa `When`, tanpa `ModeCopy`, tanpa `FileOp.Fragments`, modul `core-monolith`).
> ADR-003 sebagian sudah memakai penamaan baru (`ModeMerge`, `FileSpec`,
> `contributes[].when`). **§Decision 3 di bawah adalah kontrak kanonik tunggal**
> yang menyatukan keduanya; perbedaan diselesaikan ke arah penamaan
> `Mode`/`ModeRender|ModeCopy|ModeMkdir|ModeMerge`, `[]FileSpec`,
> `map[string]any`, `When`, dan modul `core` + `arch-*`.

## Decision

### 1. Struktur repo builder (dogfood layout 01-monolith Kandidat B)

Builder **men-dogfood** rekomendasi default-nya sendiri: layout **Kandidat B**
(layered, `cmd/` + `internal/` package-by-feature) dari riset `01-monolith.md`
§4.1. Seluruh logika hidup di bawah `internal/` (boundary berduri Go) sehingga
tidak ada konsumen luar yang bisa meng-import-nya — sekaligus menegakkan invarian
zero lock-in (project hasil generate tidak pernah meng-import builder).

```
gostarter/                                  # module github.com/faisalcayunda/gostarter
├── go.mod                                  # go 1.26; require cobra/huh/x-mod (Fase 3)
├── cmd/
│   └── gostarter/
│       └── main.go                         # entrypoint tipis: bangun cobra root, Execute()
├── internal/
│   ├── doc.go                              # dokumentasi arsitektur paket internal
│   ├── answers/
│   │   └── answers.go                      # struct konfigurasi tunggal + enum + Validate()
│   ├── prompt/
│   │   └── prompt.go                        # interface Prompter (impl huh di Fase 3)
│   ├── resolver/
│   │   └── resolver.go                      # interface Resolver: Answers → GeneratePlan (+constraint SPEC §6)
│   ├── plan/
│   │   └── plan.go                          # tipe GeneratePlan/FileOp/Fragment/ModuleDep/HookSpec
│   ├── module/
│   │   ├── manifest.go                      # tipe Manifest/FileSpec/MergeContribution + enum mode
│   │   └── registry.go                      # interface Registry: Load/Get/All (validasi katalog ADR-003 D6)
│   ├── generator/
│   │   ├── generator.go                     # interface Generator: eksekusi GeneratePlan
│   │   ├── renderer.go                      # interface Renderer + FuncMap (text/template + go/format)
│   │   └── merge.go                          # interface MergeAssembler (skeleton+fragment → byte)
│   ├── fsutil/
│   │   └── writer.go                        # interface Writer + RealWriter/DryRunWriter + EnsureEmptyDir
│   └── hooks/
│       └── hooks.go                         # interface PostGenHook + Gofmt/GoModTidy/GitInit
└── templates/
    ├── templates.go                         # //go:embed modules → var FS embed.FS
    └── modules/
        └── <module-name>/                   # satu dir per modul (ADR-003 D1)
            ├── module.yaml
            ├── <path>/<file>.tmpl
            └── fragments/<fragment>.tmpl
```

**Tanggung jawab tiap package (single responsibility):**

| Package | Tanggung jawab | Dependency keluar | Bukan tanggung jawabnya |
|---|---|---|---|
| `cmd/gostarter` | Bangun command-tree cobra (`create`, `add service`), parse flag → `answers.Answers`, pilih jalur interaktif/non-interaktif, panggil orchestration. | `cobra`, `prompt`, `resolver`, `generator`, `hooks`, `fsutil` | Tidak ada keputusan default/constraint (itu resolver). |
| `answers` | Definisi struct konfigurasi tunggal + semua enum + `Validate()` (validasi field-level: regex name, module path, enum). | hanya stdlib (`errors`, `x/mod/module` di Fase 3) | Tidak meresolusi default/constraint silang. |
| `prompt` | Kontrak `Prompter` — menjalankan wizard `huh`, menulis ke `answers.Answers`. | `huh` (Fase 3), `answers` | Tidak merender, tidak menulis disk. |
| `resolver` | **Otak keputusan.** Resolusi default (SPEC §6.2) + penegakan constraint matrix (SPEC §6.1, C1–C20/C-mongo) + seleksi modul aktif + evaluasi `when` → `plan.GeneratePlan` deterministik. | `answers`, `plan`, `module` | Tidak merender template, tidak menyentuh disk. |
| `plan` | Tipe data `GeneratePlan` (output resolver, input generator). Tidak ada perilaku. | hanya stdlib (`io/fs`) | Tidak ada logika. |
| `module` | Tipe `Manifest`/`FileSpec`/`MergeContribution` (cermin `module.yaml`) + `Registry` (load+validasi katalog ADR-003 D6). | `embed.FS`/`io/fs`, YAML parser (Fase 3) | Tidak memvalidasi kombinasi user (itu resolver). |
| `generator` | Eksekusi `GeneratePlan`: render (`Renderer`), merge (`MergeAssembler`), rakit `go.mod` (`modfile`), tulis via `Writer`. | `plan`, `module`, `fsutil`, `text/template`, `go/format`, `x/mod/modfile` | Tidak mengambil keputusan struktural (sudah final di plan). |
| `fsutil` | Abstraksi penulisan: `RealWriter` (disk) / `DryRunWriter` (preview) + `EnsureEmptyDir` (proteksi overwrite). | `os`, `io/fs` | Tidak tahu template/plan. |
| `hooks` | Hook pasca-generate (`gofmt`, `go mod tidy`, `git init`) via `os/exec`. | `os/exec`, `context` | Tidak menulis file project (hanya post-processing dir). |
| `templates` | Titik embed tunggal modul template. | `embed` | Tidak ada logika. |

**Aturan dependency (acyclic, satu arah ke bawah):**
`cmd` → {`prompt`, `resolver`, `generator`, `hooks`, `fsutil`}; `resolver` →
{`answers`, `plan`, `module`}; `generator` → {`plan`, `module`, `fsutil`,
`templates`}. `answers`/`plan` adalah daun (hanya stdlib). Tidak ada siklus;
`module` tidak meng-import `resolver`/`generator`.

#### 1.1 Penamaan modul template kanonik (KANONIK)

Nama modul template = nama direktori `templates/modules/<name>/` = field `name` di
`module.yaml` (unik global, ADR-003 D1). Prefix kategori = keterbacaan, **bukan**
hierarki; relasi dideklarasikan eksplisit via `requires`/`conflicts`. **Penamaan
lama `core-monolith`/`core-microservice` DIHAPUS** dan diganti dengan satu modul
`core` netral-arsitektur + modul `arch-*` per arsitektur.

| Kategori | Nama kanonik | Peran |
|---|---|---|
| **Inti** | `core` | Pemilik skeleton file shared lintas-arsitektur: `go.mod`, `docker-compose.yml`, `Makefile`, `.env.example`, `README.md`, `.gitignore`, dan skeleton wiring `main`. Selalu aktif (05 §1.1 "core"). |
| **Arsitektur** | `arch-monolith` · `arch-modular` · `arch-microservice` | Skeleton spesifik arsitektur (layout Kandidat B / C / monorepo). Tepat satu aktif (mutual `conflicts`). |
| **HTTP** | `http-stdlib` · `http-chi` · `http-echo` · `http-gin` · `http-fiber` | Router per framework (05 §2.3). `http-fiber` = kelas fasthttp (cabang terpisah). |
| **DB driver** | `db-postgres` · `db-mysql` · `db-sqlite` · `db-mongo` | Driver + skeleton DB + kontribusi compose/env (05 §2.4). |
| **Access** | `access-sqlx` · `access-stdlib` · `access-sqlc` · `access-gorm` · `access-ent` | Lapisan query (05 §2.5). |
| **Migrate** | `migrate-golang-migrate` · `migrate-goose` · `migrate-atlas` | Tool migrasi (05 §2.6). |
| **Comm** (microservice) | `comm-grpc` · `comm-rest` · `comm-event` | Pola komunikasi (05 §2.7). |
| **Broker** (event) | `broker-nats` · `broker-kafka` · `broker-rabbitmq` | Message broker (05 §2.8). |
| **Add-on** | `addon-docker` · `addon-makefile` · `addon-taskfile` · `addon-ci-github` · `addon-ci-gitlab` · `addon-lint` · `addon-obs` · `addon-auth-jwt` · `addon-auth-paseto` · `addon-env-example` | Komponen opsional (05 §2.9). |
| **Config** | `config-godotenv` · `config-koanf` · `config-viper` · `config-env` | Config loader (05 §2.9). |
| **Log** | `log-slog` · `log-zerolog` · `log-zap` | Logger (05 §2.9). |

> File shared dimiliki `core` (mayoritas) atau modul pemilik natural (mis. skeleton
> `docker-compose.yml` boleh dipegang `addon-docker`); modul lain hanya
> `contributes` fragment ke anchor-nya (ADR-003 D5).

### 2. Alur data end-to-end

```
            ┌────────────────────────── cmd/gostarter (cobra) ──────────────────────────┐
            │  root.Execute → subcommand `create`                                        │
            │  flag lengkap? ──no──► prompt.Prompter.Ask(ctx) ──┐ (huh wizard)           │
            │       │ yes                                        │                        │
            │       ▼                                            ▼                        │
            │  parse flag → answers.Answers ◄───── tulis ke struct yang SAMA ────────────┤
            └───────────────────────────────────┬───────────────────────────────────────┘
                                                 │  answers.Answers (Validate() field-level)
                                                 ▼
                            ┌────────────── resolver.Resolver.Resolve ──────────────┐
                            │ 1. resolusi default (SPEC §6.2)                        │
                            │ 2. seleksi modul aktif (Registry + decision matrix 05) │
                            │ 3. cek requires/conflicts (fail-fast ErrConstraint)    │
                            │ 4. constraint matrix SPEC §6.1 (C1..C-mongo)           │
                            │ 5. evaluasi `when` per FileSpec/MergeContribution      │
                            │ 6. rakit []FileOp (Mode*) + []ModuleDep + []HookSpec   │
                            │    urutan stabil (sort) → byte-identical               │
                            └───────────────────────────────┬───────────────────────┘
                                                            │  plan.GeneratePlan (final, deterministik)
                                                            ▼
            ┌──────────────────────── generator.Generator.Generate ───────────────────────┐
            │ fsutil.EnsureEmptyDir(target)  (skip bila DryRunWriter)                       │
            │ untuk tiap FileOp (urutan plan):                                              │
            │   ModeMkdir  → Writer.Mkdir                                                   │
            │   ModeCopy   → baca embed.FS apa adanya → Writer.WriteFile                    │
            │   ModeRender → Renderer.Render(tmpl,data) → (go/format bila .go) → WriteFile  │
            │   ModeMerge  → MergeAssembler.Assemble(skeleton, Fragments) → WriteFile       │
            │ go.mod  → rakit dari plan.Deps via modfile.Format (BUKAN merge teks)          │
            └───────────────────────────────────┬──────────────────────────────────────────┘
                                                │  (RealWriter: file di disk · DryRunWriter: Planned terkumpul)
                                                ▼
            ┌──────────────────────────── hooks (RealWriter saja) ─────────────────────────┐
            │ urut HookSpec.Order:  Gofmt → GoModTidy → [GitInit bila --git]                │
            │ fail-fast pada Gofmt/GoModTidy; GitInit warn-only                              │
            └──────────────────────────────────────────────────────────────────────────────┘
                                                │
                                                ▼
                                 DryRunWriter? → cetak tree Planned (nol tulis)
                                 RealWriter?   → project siap di disk
```

**Urutan langkah (tekstual, otoritatif):**

1. **`cmd`** membangun root cobra + subcommand `create` / `add service`. Untuk
   `create`: bila semua flag wajib lengkap (mode non-interaktif) → parse langsung
   ke `answers.Answers`; bila tidak → jalankan `Prompter.Ask` (wizard `huh`).
   **Kedua jalur menulis ke instance `answers.Answers` yang sama** (penjamin
   byte-identical, SPEC §5.2 poin 1).
2. **`answers.Validate()`** memeriksa field-level (regex `name`, module path legal
   via `x/mod/module`, enum valid). Gagal → exit non-zero, tidak ada file ditulis.
3. **`resolver.Resolve(a)`** menjalankan resolusi default + constraint + seleksi
   modul + evaluasi `when` → `plan.GeneratePlan`. Ini satu-satunya tempat
   "judgment" SPEC §6 dijalankan. Constraint hard-invalid → `ErrConstraint`
   fail-fast (SPEC §6.4 / DoD #6).
4. **`generator.Generate(p, target, w)`** mengeksekusi plan ke `target` memakai
   `Writer` (real atau dry-run). `EnsureEmptyDir` dipanggil sebelum tulis (real
   saja).
5. **`hooks`** dijalankan hanya untuk `RealWriter`, urut `HookSpec.Order`
   (gofmt → go mod tidy → git init opsional).
6. **Output:** `RealWriter` → project siap (build hijau); `DryRunWriter` → cetak
   `Planned` sebagai tree + daftar `Deps` (SPEC §5.4), nol penulisan.

`add service` (US-05) memakai jalur yang sama dengan dua perbedaan: target =
project existing (bukan dir kosong), dan merge bersifat **inkremental** terhadap
marker netral di file shared (ADR-003 D5) — bukan `EnsureEmptyDir`.

### 3. Kontrak interface & tipe inti (Go verbatim — KANONIK)

Blok berikut adalah **sumber kebenaran tunggal**. Skeleton `internal/**` dan contoh
manifest ADR-003 disalin/diselaraskan **persis** ke sini. Komentar boleh diringkas
di kode, tetapi nama, tipe, signature, dan konstanta **tidak boleh** berubah.

#### 3.1 `internal/answers` — struct konfigurasi tunggal + enum

```go
package answers

import "errors"

var errNotImplemented = errors.New("answers: not implemented (Fase 3)")

type Arch string

const (
	ArchMonolith        Arch = "monolith"
	ArchModularMonolith Arch = "modular-monolith"
	ArchMicroservice    Arch = "microservice"
)

type Kind string

const (
	KindREST   Kind = "rest"
	KindWeb    Kind = "web"
	KindWorker Kind = "worker"
)

type Comm string

const (
	CommGRPC  Comm = "grpc"
	CommREST  Comm = "rest"
	CommEvent Comm = "event"
)

type Broker string

const (
	BrokerNATS     Broker = "nats"
	BrokerKafka    Broker = "kafka"
	BrokerRabbitMQ Broker = "rabbitmq"
)

type HTTPFramework string

const (
	HTTPNetHTTP HTTPFramework = "net/http"
	HTTPChi     HTTPFramework = "chi"
	HTTPEcho    HTTPFramework = "echo"
	HTTPGin     HTTPFramework = "gin"
	HTTPFiber   HTTPFramework = "fiber"
)

type DB string

const (
	DBNone     DB = "none"
	DBPostgres DB = "postgres"
	DBMySQL    DB = "mysql"
	DBSQLite   DB = "sqlite"
	DBMongo    DB = "mongo"
)

type Access string

const (
	AccessSQLx        Access = "sqlx"
	AccessDatabaseSQL Access = "database/sql"
	AccessSQLC        Access = "sqlc"
	AccessGORM        Access = "gorm"
	AccessEnt         Access = "ent"
)

type Migrate string

const (
	MigrateGolangMigrate Migrate = "golang-migrate"
	MigrateGoose         Migrate = "goose"
	MigrateAtlas         Migrate = "atlas"
)

type CI string

const (
	CINone          CI = "none"
	CIGitHubActions CI = "github-actions"
	CIGitLabCI      CI = "gitlab-ci"
)

type Auth string

const (
	AuthNone   Auth = "none"
	AuthJWT    Auth = "jwt"
	AuthPaseto Auth = "paseto"
)

type ConfigLoader string

const (
	ConfigLoaderGodotenv ConfigLoader = "godotenv"
	ConfigLoaderKoanf    ConfigLoader = "koanf"
	ConfigLoaderViper    ConfigLoader = "viper"
	ConfigLoaderEnv      ConfigLoader = "env"
)

type Log string

const (
	LogSlog    Log = "slog"
	LogZerolog Log = "zerolog"
	LogZap     Log = "zap"
)

// Service adalah satu unit deploy dalam monorepo microservice (SPEC §4.5).
type Service struct {
	Name string
}

// Answers adalah hasil terkumpul dari semua pertanyaan SPEC §4 / flag SPEC §5.1.
// Satu instance ini menjadi input tunggal resolver. Setiap field memetakan 1:1 ke
// flag SPEC §5.1; default diresolusi oleh resolver, bukan struct ini.
type Answers struct {
	// q_name (SPEC §4.2)
	Name   string // --name
	Module string // --module (default github.com/<name>)

	// q_arch (SPEC §4.3)
	Arch Arch // --arch

	// q_kind (SPEC §4.4) — monolith/modular-monolith
	Kind Kind // --kind

	// q_svc group (SPEC §4.5) — microservice
	Services []Service // --service (repeatable) / --services (csv)
	Comm     Comm      // --comm
	Broker   Broker    // --broker (hanya bila Comm == CommEvent)
	Gateway  bool      // --gateway / --no-gateway

	// q_http (SPEC §4.6)
	HTTP HTTPFramework // --http

	// q_db group (SPEC §4.7)
	DB      DB      // --db
	Access  Access  // --access (bila DB != none, != mongo)
	Migrate Migrate // --migrate (bila DB not in {none, mongo})

	// q_addons (SPEC §4.8)
	Docker     bool // --docker / --no-docker
	Makefile   bool // --makefile / --no-makefile
	Taskfile   bool // --taskfile
	CI         CI   // --ci
	Lint       bool // --lint / --no-lint
	Obs        bool // --obs / --no-obs
	Auth       Auth // --auth
	EnvExample bool // --env-example / --no-env-example

	// Opsi terkunci (flag-only, SPEC §5.1)
	ConfigLoader ConfigLoader // --config-loader
	Log          Log          // --log
	// ValidateInput memetakan --validate / --no-validate. Dinamai *Input* agar tidak
	// bentrok dengan method Validate() (Go melarang field & method bernama sama).
	ValidateInput bool

	// Flag advanced (SPEC §6.5) — default off
	Mock        bool // --mock
	Integration bool // --integration

	// q_git (SPEC §4.9)
	Git bool // --git / --no-git
}

// Validate memeriksa konsistensi field-level Answers (regex name, module path
// legal, enum) — BUKAN constraint silang antar-opsi (itu resolver.Resolve).
func (a Answers) Validate() error {
	return errNotImplemented
}
```

#### 3.2 `internal/module` — Manifest, FileSpec, MergeContribution, Registry

```go
package module

// FileSpec adalah satu file yang dibawa modul (cermin module.yaml `files[]`,
// ADR-003 D2). Mode menentukan cara file diproses.
type FileSpec struct {
	// Template adalah path .tmpl relatif terhadap dir modul di embed.FS.
	Template string `yaml:"template"`
	// Target adalah path relatif project hasil generate. Boleh mengandung
	// placeholder template (mis. "internal/{{.Feature}}/handler.go") yang
	// dievaluasi resolver saat merakit FileOp.
	Target string `yaml:"target"`
	// Mode: "render" (default), "copy", atau "mkdir". Merge TIDAK di sini
	// (lihat MergeContribution / contributes).
	Mode string `yaml:"mode"`
	// When adalah ekspresi kondisi opsional (ADR-002 §5). Kosong = aktif selama
	// modulnya aktif.
	When string `yaml:"when"`
}

// MergeContribution adalah kontribusi satu modul ke file SHARED (selalu via
// assembler, menghasilkan FileOp ber-Mode ModeMerge). Cermin module.yaml
// `contributes[]` (ADR-003 D2/D5).
type MergeContribution struct {
	// Target adalah file shared tujuan (mis. "docker-compose.yml").
	Target string `yaml:"target"`
	// Anchor adalah nama section pada skeleton file shared (mis. "services").
	Anchor string `yaml:"anchor"`
	// Fragment adalah path fragmen .tmpl (lazim di fragments/). Di module.yaml
	// field ini boleh ditulis sebagai `template` atau `fragment` (alias);
	// keduanya memetakan ke Fragment.
	Fragment string `yaml:"fragment"`
	// When adalah kondisi opsional fragment-level (ADR-002 §5). Kosong = aktif.
	When string `yaml:"when"`
	// Order menentukan urutan deterministik dalam satu (Target, Anchor).
	// Tie-break: nama modul.
	Order int `yaml:"order"`
}

// ModuleDep adalah satu dependency go.mod yang dibawa modul (cermin
// module.yaml `gomod[]`). Identik dengan plan.ModuleDep.
type ModuleDep struct {
	Path    string `yaml:"path"`
	Version string `yaml:"version"`
}

// Manifest adalah representasi in-memory satu module.yaml (skema ADR-003 D2).
type Manifest struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Files       []FileSpec          `yaml:"files"`
	GoMod       []ModuleDep         `yaml:"gomod"`
	Requires    []string            `yaml:"requires"`
	Conflicts   []string            `yaml:"conflicts"`
	Contributes []MergeContribution `yaml:"contributes"`
	// Vars adalah default var modul yang digabung ke template context. Bertipe
	// map[string]any agar nilai non-string muat (mis. DBPort: 5432).
	Vars map[string]any `yaml:"vars"`
}
```

```go
package module

import "io/fs"

// Registry adalah indeks manifest modul template, di-query per nama.
type Registry interface {
	// Load men-scan fsys (mis. templates.FS), mem-parse tiap module.yaml, lalu
	// MEMVALIDASI katalog (ADR-003 D6: field wajib, keunikan name, referensi
	// requires/conflicts, anchor↔skeleton, path template ada). Fail-fast.
	Load(fsys fs.FS) error
	// Get mengembalikan manifest by nama; ok=false bila tak ada.
	Get(name string) (Manifest, bool)
	// All mengembalikan seluruh manifest, urut deterministik by Name.
	All() []Manifest
}
```

#### 3.3 `internal/plan` — GeneratePlan, FileOp, FileOpMode, Fragment, ModuleDep, HookSpec

```go
package plan

import "io/fs"

// FileOpMode membedakan cara satu FileOp diproses generator.
type FileOpMode int

const (
	// ModeRender me-render satu template (.tmpl) → (go/format bila .go) → tulis.
	ModeRender FileOpMode = iota
	// ModeCopy menyalin file dari embed.FS apa adanya (tanpa render).
	ModeCopy
	// ModeMkdir membuat direktori.
	ModeMkdir
	// ModeMerge merakit file shared dari Fragments terurut per anchor.
	ModeMerge
)

// Fragment adalah satu potongan terender yang disisipkan pada anchor sebuah file
// shared (dipakai hanya oleh ModeMerge). Resolver mengisi Content dari render
// fragmen modul (atau menunda render ke assembler — lihat §6); Order menentukan
// urutan dalam satu anchor.
type Fragment struct {
	Anchor  string // nama anchor tujuan pada skeleton (mis. "services")
	Content string // konten fragmen (terender) yang disisipkan
	Order   int    // urutan deterministik dalam satu anchor
	// DataOverride (OPSIONAL) adalah override data render KHUSUS fragmen ini,
	// di-MERGE di atas data context FileOp induk saat render fragmen (key fragmen
	// menang). Dipakai saat satu skeleton merge merakit fragmen PER-SERVICE dari
	// template SAMA dengan nilai berbeda (mis. Service, GrpcPort). nil → pakai data
	// induk apa adanya. Lihat catatan "Ekstensi DataOverride" di bawah.
	DataOverride map[string]any
}

// FileOp adalah satu operasi file dalam GeneratePlan.
type FileOp struct {
	Mode FileOpMode // jenis operasi (render/copy/mkdir/merge)
	// TargetPath adalah path relatif tujuan di project hasil generate.
	TargetPath string
	// ModuleName adalah nama modul asal (audit/dry-run; "" untuk file shared
	// hasil merge multi-modul).
	ModuleName string
	// TemplatePath adalah path template sumber di embed.FS (untuk ModeRender/
	// ModeCopy, dan skeleton untuk ModeMerge). Kosong untuk ModeMkdir.
	TemplatePath string
	// Fragments adalah daftar fragmen terurut untuk ModeMerge (kosong untuk mode
	// lain).
	Fragments []Fragment
	// Perm adalah permission file/direktori.
	Perm fs.FileMode
	// Data adalah context render template GLOBAL (proyeksi Answers + Vars modul).
	// Untuk ModeRender/ModeMerge ini menjadi data dasar render.
	Data any
	// DataOverride (OPSIONAL) adalah override data render KHUSUS FileOp ini,
	// di-MERGE di ATAS Data saat render — key di DataOverride MENANG. Dipakai saat
	// satu template per-service di-render N kali dengan nilai berbeda (mis. Service,
	// GrpcPort) tanpa menduplikasi seluruh context. nil → render hanya dengan Data.
	// Hanya bermakna untuk ModeRender & ModeMerge. Lihat catatan di bawah.
	DataOverride map[string]any
}

// ModuleDep adalah satu dependency go.mod yang masuk ke project hasil generate.
// Identik bentuk dengan module.ModuleDep.
type ModuleDep struct {
	Path    string // mis. "github.com/go-chi/chi/v5"
	Version string // mis. "v5.3.0"
}

// HookSpec mendeskripsikan satu hook pasca-generate beserta urutannya.
type HookSpec struct {
	Name  string   // cocok dengan PostGenHook.Name()
	Order int      // urutan eksekusi (menaik)
	Args  []string // argumen tambahan bila ada
}

// GeneratePlan adalah rencana lengkap hasil resolver: operasi file, dependency
// go.mod terkumpul (dedup+sort), dan hook pasca-generate. Generator
// mengeksekusinya secara deterministik.
type GeneratePlan struct {
	ProjectName string     // nama folder root
	ModulePath  string     // go module path output
	GoVersion   string     // go directive output (mis. "1.24"; "1.25" bila fiber C8 ATAU arch=microservice — grpc v1.81.1 butuh go ≥ 1.25)
	Files       []FileOp   // operasi file terurut (stabil)
	Deps        []ModuleDep // dependency go.mod (dedup+sort) → modfile.Format
	Hooks       []HookSpec // hook pasca-generate terurut
}
```

##### Ekstensi `FileOp.Data` / `DataOverride` (render per-instance — Fase 4b)

`FileOp.Data` (dan `Fragment.DataOverride`) memperluas kontrak §3.3 secara
**kanonik** untuk mendukung render PER-INSTANCE tanpa menduplikasi seluruh context.
Motivasi & semantik:

- **Alasan (render per-service untuk microservice).** Arsitektur microservice
  (modul `arch-microservice`) meng-emit satu set file PER service (`proto/<svc>/…`,
  `services/<svc>/…`) dari template yang **sama** namun nilai berbeda (nama service,
  port gRPC, daftar downstream). Alih-alih menyalin context render N kali, resolver
  mengisi `FileOp.Data` dengan context **global** (proyeksi Answers + Vars modul) lalu
  `FileOp.DataOverride` dengan field **per-service** sintetik: `Service`, `IsFirst`,
  `Others`, `Downstreams` (`[{Name, Port}]`), `GrpcPort`, `GatewayPort`, `ModulePath`.
  Hal yang sama berlaku untuk `Fragment.DataOverride` pada fragmen compose per-service.

- **Semantik merge (global + op).** Saat render, generator menggabungkan
  `mergeRenderData(Data, DataOverride)` — **key di `DataOverride` MENANG** per-key.
  `Data` (map global) **tidak** dimutasi (dibagi lintas FileOp); merge menghasilkan
  salinan dangkal. `DataOverride` nil/kosong → render hanya dengan `Data` (jalur lama,
  byte-identical untuk FileOp tanpa override). Hanya bermakna untuk `ModeRender` &
  `ModeMerge`; diabaikan `ModeCopy`/`ModeMkdir`.

- **Determinisme.** Resolver WAJIB mengisi `DataOverride` dengan nilai deterministik
  (port = `grpcPortBase + indeks` pada `sortedServiceNames`; daftar service ter-sort).
  Karena render hanya membaca per-key (bukan urutan iterasi map), output tetap
  **byte-identical** untuk Answers yang sama (SPEC §5.2). Urutan service input (mis.
  `--service`) TIDAK bocor ke output — FileOp & Fragment di-sort sebelum eksekusi.

> **Jejak konsolidasi (M-1).** Jalur generate microservice sebelumnya berupa island
> terpisah (`internal/cli/micro`) yang menyusun `plan.GeneratePlan` sendiri. Sejak
> konsolidasi Fase 4b, microservice memakai pipeline **terpadu** resolver→generator yang
> sama dengan monolith/modular; `FileOp.Data`/`DataOverride` adalah mekanisme kanonik yang
> menggantikannya. `gostarter add service` juga memakai jalur ini
> (`resolver.ResolveAddService` → `generator.GenerateFiles`).

#### 3.4 `internal/generator` — Generator, Renderer, MergeAssembler, FuncMap

```go
package generator

import (
	"github.com/faisalcayunda/gostarter/internal/fsutil"
	"github.com/faisalcayunda/gostarter/internal/plan"
)

// Generator mengeksekusi GeneratePlan ke direktori target memakai Writer.
// target = root project hasil generate; w = RealWriter (tulis) / DryRunWriter
// (preview).
type Generator interface {
	// Generate: alur create penuh — EnsureEmptyDir (RealWriter) → semua FileOp →
	// rakit go.mod.
	Generate(p plan.GeneratePlan, target string, w fsutil.Writer) error
	// GenerateFiles: HANYA p.Files (tanpa EnsureEmptyDir & tanpa go.mod) — jalur
	// inkremental `add service` (US-05) pada project existing. Containment FileOp
	// tetap via fsutil.JoinTarget (B-1).
	GenerateFiles(p plan.GeneratePlan, target string, w fsutil.Writer) error
}

// Renderer me-render satu template bernama dengan data context menjadi byte.
// Output file .go dilewatkan ke go/format oleh pemanggil (generator) agar
// deterministik.
type Renderer interface {
	Render(name string, data any) ([]byte, error)
}

// MergeAssembler merakit satu file shared: render skeleton, lalu untuk tiap anchor
// menyisipkan Fragments terurut → satu konten final tunggal. Untuk file .go hasil
// akhir dilewatkan go/format oleh pemanggil.
type MergeAssembler interface {
	Assemble(skeleton []byte, frags []plan.Fragment) ([]byte, error)
}
```

```go
package generator

// FuncMap mengembalikan helper template builder (ADR-001 §3; ADR-002 §4). Tujuh
// fungsi kanonik: toCamel, toPascal, toSnake, toScreamingSnake, toKebab, modBase, modJoin.
func FuncMap() map[string]any {
	// Fase 3 mengisi 7 fungsi; lihat ADR-002 §4 untuk signature & perilaku.
	return map[string]any{}
}
```

#### 3.5 `internal/fsutil` — Writer, RealWriter, DryRunWriter, EnsureEmptyDir

```go
package fsutil

import "io/fs"

// Writer adalah abstraksi target penulisan. RealWriter → disk; DryRunWriter →
// hanya mencatat rencana (SPEC §5.4).
type Writer interface {
	Mkdir(path string) error
	WriteFile(path string, data []byte, perm fs.FileMode) error
}

// RealWriter menulis project ke filesystem sungguhan.
type RealWriter struct{}

func (RealWriter) Mkdir(path string) error                            { panic("TODO: Fase 3") }
func (RealWriter) WriteFile(path string, data []byte, perm fs.FileMode) error { panic("TODO: Fase 3") }

// DryRunWriter tidak menulis apa pun; ia mengakumulasi operasi untuk preview
// --dry-run di field Planned (urutan = urutan FileOp).
type DryRunWriter struct {
	// Planned adalah daftar path yang AKAN ditulis/dibuat (urut sesuai plan),
	// dipakai untuk mencetak tree preview tanpa menyentuh disk.
	Planned []string
}

func (*DryRunWriter) Mkdir(path string) error                            { panic("TODO: Fase 3") }
func (*DryRunWriter) WriteFile(path string, data []byte, perm fs.FileMode) error { panic("TODO: Fase 3") }

// EnsureEmptyDir memastikan path tujuan kosong atau belum ada (proteksi overwrite,
// SPEC §5.4 / US-06 Sk.3). Error bila folder tujuan tidak kosong.
func EnsureEmptyDir(path string) error { panic("TODO: Fase 3") }
```

> **Catatan pointer-receiver `DryRunWriter`.** Karena `DryRunWriter` mengakumulasi
> state (`Planned`), method-nya **wajib** pointer-receiver dan pemanggil
> meneruskan `&DryRunWriter{}` sebagai `fsutil.Writer`. `RealWriter` stateless →
> value-receiver. Skeleton Fase 2 harus mengikuti bentuk ini agar Fase 3 tidak
> mengubah signature publik.

#### 3.6 `internal/prompt` & `internal/resolver` & `internal/hooks`

```go
package prompt

import (
	"context"

	"github.com/faisalcayunda/gostarter/internal/answers"
)

// Prompter menjalankan wizard interaktif (huh, Fase 3) dan mengisi answers.Answers.
type Prompter interface {
	Ask(ctx context.Context) (answers.Answers, error)
}
```

```go
package resolver

import (
	"github.com/faisalcayunda/gostarter/internal/answers"
	"github.com/faisalcayunda/gostarter/internal/plan"
)

// Resolver mengubah Answers → GeneratePlan: resolusi default (SPEC §6.2),
// penegakan constraint matrix (SPEC §6.1), seleksi modul aktif + evaluasi `when`,
// menghasilkan rencana deterministik (urut stabil) demi invarian §5.2.
type Resolver interface {
	Resolve(a answers.Answers) (plan.GeneratePlan, error)
}
```

```go
package hooks

import "context"

// PostGenHook adalah satu langkah pasca-generate pada direktori project.
type PostGenHook interface {
	Name() string
	Run(ctx context.Context, projectDir string) error
}

// Implementasi kanonik: Gofmt, GoModTidy, GitInit (lihat ADR-002 §7).
type Gofmt struct{}
type GoModTidy struct{}
type GitInit struct{}
```

### 4. FuncMap kanonik (7 fungsi)

Helper template ditulis sendiri (tanpa `sprig`, ADR-001 §3). Tepat **tujuh** fungsi,
masing-masing di-unit-test sendiri (ADR-002 §9). Semantik **deterministik &
murni** (input sama → output sama, tanpa side-effect/locale).

| Fungsi | Signature | Perilaku | Contoh I/O |
|---|---|---|---|
| `toCamel` | `func(s string) string` | Normalisasi token (pisah pada `_`, `-`, spasi, dan batas case) lalu gabung camelCase: token pertama huruf-kecil, token berikut Pascal. | `toCamel("user_name")` → `"userName"` |
| `toPascal` | `func(s string) string` | Seperti `toCamel` tetapi token pertama juga di-Pascal-kan. Untuk nama tipe/identifier exported. | `toPascal("user_name")` → `"UserName"` |
| `toSnake` | `func(s string) string` | Normalisasi token lalu gabung `snake_case` huruf-kecil; sisip `_` pada batas case dari input camel/Pascal. | `toSnake("UserName")` → `"user_name"` |
| `toScreamingSnake` | `func(s string) string` | Seperti `toSnake` tetapi HURUF-BESAR (`SCREAMING_SNAKE_CASE`); untuk nama environment variable (konvensi POSIX, mis. override alamat downstream gRPC). | `toScreamingSnake("svc-b")` → `"SVC_B"` |
| `toKebab` | `func(s string) string` | Seperti `toSnake` tetapi pemisah `-`; untuk nama service/image Docker. | `toKebab("UserName")` → `"user-name"` |
| `modBase` | `func(modulePath string) string` | Ambil segmen terakhir module path (base), strip suffix versi mayor `/vN` bila ada. Untuk nama biner / package root. | `modBase("github.com/acme/shop")` → `"shop"` |
| `modJoin` | `func(modulePath string, elem ...string) string` | Gabung module path + elemen path import dengan `/` (path.Join-style, slash-only POSIX, SPEC §2.1). Untuk menyusun import path internal. | `modJoin("github.com/acme/fleet","services","user")` → `"github.com/acme/fleet/services/user"` |

> **Aturan implementasi (mengikat Fase 3):** normalisasi token dipusatkan di satu
> helper internal `splitTokens(s)` agar `toCamel`/`toPascal`/`toSnake`/`toScreamingSnake`/`toKebab`
> konsisten; `modBase`/`modJoin` murni manipulasi path (tidak memvalidasi module
> path — validasi ada di `answers.Validate`). Batas case: transisi `lower→Upper`
> dan akhir run akronim (mis. `HTTPServer` → `http_server`). Karakter non-alnum di
> luar pemisah dibuang.

### 5. Grammar ekspresi `when` (FORMAL)

`when` adalah **mini-bahasa kondisi fragment/file-level**, dievaluasi **di
resolver** (bukan di dalam `.tmpl`) sehingga `GeneratePlan` final sebelum render
(syarat dry-run akurat & golden stabil, ADR-003 D4). Bentuk yang dipilih:
**ekspresi boolean gaya `text/template`** yang dievaluasi atas proyeksi `Answers`
(+`Vars`). Satu bentuk konsisten dipakai di SELURUH manifest.

#### 5.1 Sintaks (EBNF)

```
when      = expr ;
expr      = orExpr ;
orExpr    = andExpr , { "or" , andExpr } ;
andExpr   = unary  , { "and" , unary } ;
unary     = [ "not" ] , atom ;
atom      = boolField                       (* mis. .Docker  *)
          | "(" , expr , ")"
          | call ;
call      = func , SP , arg , SP , arg ;     (* fungsi biner: eq / ne *)
func      = "eq" | "ne" ;
arg       = field | string | number ;
field     = "." , ident , { "." , ident } ; (* mis. .Arch , .Migrate *)
string    = '"' , { char } , '"' ;          (* mis. "microservice", "" *)
number    = digit , { digit } ;             (* mis. 5432 *)
boolField = field ;                          (* field bertipe bool → true/false *)
ident     = letter , { letter | digit } ;
```

Aturan tambahan:
- Ekspresi **kosong** (`when: ""` atau field absen) = **selalu true** (file/fragment
  aktif selama modulnya aktif).
- Operator: `eq`/`ne` (binary, prefix — gaya `text/template`), `and`/`or`/`not`
  (logika). `and` mengikat lebih kuat dari `or`; `not` mengikat paling kuat;
  kurung `( )` untuk override presedensi.
- **Tidak ada** `lt/gt/le/ge`, tidak ada aritmetika, tidak ada pemanggilan
  FuncMap di `when` — sengaja minimal demi determinisme (ADR-003 Consequences).

#### 5.2 Daftar field yang boleh dirujuk

Field dirujuk dengan gaya `.<Nama>` mengacu pada **proyeksi `Answers` (sudah
diresolusi default)**. Hanya field berikut yang legal di `when`; merujuk field di
luar daftar → `ErrConstraint` saat `Resolve` (fail-fast, bukan silent-false):

| Field `when` | Tipe | Sumber `Answers` |
|---|---|---|
| `.Arch` | string | `Arch` (`monolith`/`modular-monolith`/`microservice`) |
| `.Kind` | string | `Kind` (`rest`/`web`/`worker`) |
| `.Comm` | string | `Comm` (`grpc`/`rest`/`event`) |
| `.Broker` | string | `Broker` |
| `.HTTP` | string | `HTTP` |
| `.DB` | string | `DB` |
| `.Access` | string | `Access` |
| `.Migrate` | string | `Migrate` (`""` bila tak dipilih) |
| `.CI` | string | `CI` |
| `.Auth` | string | `Auth` |
| `.ConfigLoader` | string | `ConfigLoader` |
| `.Log` | string | `Log` |
| `.Docker` | bool | `Docker` |
| `.Makefile` | bool | `Makefile` |
| `.Taskfile` | bool | `Taskfile` |
| `.Lint` | bool | `Lint` |
| `.Obs` | bool | `Obs` |
| `.EnvExample` | bool | `EnvExample` |
| `.ValidateInput` | bool | `ValidateInput` |
| `.Gateway` | bool | `Gateway` |
| `.Git` | bool | `Git` |
| `.Mock` | bool | `Mock` |
| `.Integration` | bool | `Integration` |

> Field string dibandingkan dengan literal string (`eq .Arch "microservice"`);
> field bool dipakai langsung sebagai atom (`.Docker`) atau dinegasi (`not .Docker`).

#### 5.3 Semantik evaluasi & lokasi evaluator

- **Evaluator** hidup di `resolver` (Fase 3): fungsi `evalWhen(expr string, a
  answers.Answers) (bool, error)` yang mem-parse ekspresi sekali per `(target,
  anchor)`/FileSpec dan mengevaluasinya atas `Answers` final. Implementasi boleh
  memakai `text/template` dengan `{{if <expr>}}` di balik layar **atau** parser
  kecil khusus — yang penting hasilnya **boolean murni & deterministik**.
- Hanya `FileSpec`/`MergeContribution` yang `when`-nya **true** yang masuk
  `GeneratePlan`. Yang false **tidak** menghasilkan `FileOp`/`Fragment` sama
  sekali (bukan sekadar di-comment di output).
- Module-level gating (modul aktif/mati) **mendahului** `when` (ADR-003 D4 lapis
  1); `when` hanya dievaluasi untuk modul yang sudah aktif (lapis 2).

#### 5.4 Contoh (≥3)

```yaml
# 1. fragment compose postgres hanya bila Docker aktif (C14: sqlite tak menyumbang)
when: ".Docker"

# 2. target migrate di Makefile hanya bila tool migrasi terpilih
when: "ne .Migrate \"\""

# 3. blok wiring gRPC hanya untuk microservice berkomunikasi gRPC
when: "and (eq .Arch \"microservice\") (eq .Comm \"grpc\")"

# 4. middleware auth hanya bila auth != none DAN bukan worker (C17)
when: "and (ne .Auth \"none\") (ne .Kind \"worker\")"

# 5. cabang observability kelas net/http (bukan fiber)
when: "and .Obs (ne .HTTP \"fiber\")"
```

### 6. Model eksekusi GeneratePlan oleh Generator

`Generator.Generate(p, target, w)` memproses `p.Files` **berurutan sesuai urutan
dalam plan** (urutan sudah distabilkan resolver). Per `FileOp.Mode`:

| Mode | Proses |
|---|---|
| `ModeMkdir` | `w.Mkdir(join(target, op.TargetPath))`. Idempoten (mkdir-all). |
| `ModeCopy` | Baca `op.TemplatePath` dari `templates.FS` **apa adanya** (tanpa render) → `w.WriteFile`. Untuk aset biner/non-template (mis. file statik). |
| `ModeRender` | `Renderer.Render(op.TemplatePath, op.Data)` → bila target `.go`: lewatkan `go/format.Source` (gagal format = error, bukan tulis mentah) → `w.WriteFile`. |
| `ModeMerge` | Render skeleton (`op.TemplatePath`) → `MergeAssembler.Assemble(skeleton, op.Fragments)` → bila `.go`: `go/format` → `w.WriteFile`. |

**Algoritma `MergeAssembler.Assemble(skeleton, frags)`:**

1. Render `skeleton` (lewat `Renderer`) → byte ber-anchor (anchor = komentar
   netral `# gostarter:<anchor>` / `// gostarter:<anchor>` sesuai sintaks file —
   ADR-003 D5).
2. Kelompokkan `frags` per `Anchor`; dalam tiap anchor **sort by `Order` lalu
   tie-break stabil** (urutan sudah final dari resolver — assembler hanya
   menegakkan).
3. Untuk tiap anchor, sisipkan `Content` fragmen terurut menggantikan/menyusul
   baris marker anchor, menjaga indentasi marker.
4. Gabung → satu byte tunggal. Anchor tanpa fragmen → marker tetap (untuk
   idempotensi `add service`) atau dibersihkan sesuai kebijakan file (ADR-003 D5).
5. Kembalikan byte final (pemanggil yang menjalankan `go/format` bila `.go`).

**Perakitan `go.mod` (BUKAN merge teks — KHUSUS):**

`go.mod` **tidak** diproses sebagai `ModeMerge`. Resolver mengumpulkan seluruh
`gomod[]` modul aktif ke `plan.Deps` (dedup by `Path`, ambil versi tertinggi bila
ganda, lalu **sort by `Path`**). Generator merakit `go.mod` via
`golang.org/x/mod/modfile` (ADR-001 §4):

```
f := modfile.Parse / new(modfile.File)
f.AddModuleStmt(p.ModulePath)
f.AddGoStmt(p.GoVersion)
for _, d := range p.Deps {            // p.Deps sudah dedup+sort
    f.AddRequire(d.Path, d.Version)
}
f.SortBlocks() ; f.Cleanup()
out, _ := f.Format()                  // deterministik, byte-stabil
w.WriteFile(join(target,"go.mod"), out, 0o644)
```

Hasil `require` ter-normalisasi & terurut (SPEC §5.2 poin 3) sehingga byte-identical
antar mode. `go.sum` dibuat oleh `go mod tidy` (hook, §7), bukan oleh builder.

**Urutan operasi & idempotensi:**

- Resolver menghasilkan `p.Files` dengan urutan **deterministik** (sort by
  `TargetPath`; `ModeMkdir` parent mendahului file di dalamnya). Maka dua input
  yang ekuivalen → urutan FileOp identik → output byte-identical (SPEC §5.2,
  US-02/US-03 Sk.3).
- `Generate` **idempoten terhadap target kosong**: dijalankan dua kali pada dir
  bersih menghasilkan byte yang sama. Pada target tidak kosong, `EnsureEmptyDir`
  menolak lebih dulu (proteksi overwrite, US-06 Sk.3) — kecuali jalur
  `add service` yang inkremental.

### 7. Hook orchestration

Hook pasca-generate hanya berjalan untuk `RealWriter` (DryRunWriter tidak pernah
memicu hook). Urutan ditentukan `HookSpec.Order` (menaik); urutan kanonik:

```
Order 10  Gofmt      → gofmt -w atas seluruh .go (jaring pengaman; mayoritas .go
                       sudah lewat go/format saat render, ini menutup sisa)
Order 20  GoModTidy  → `go mod tidy` (finalisasi go.mod + buat go.sum; ADR-001 §4)
Order 30  GitInit    → `git init` + `git add -A` + initial commit (HANYA bila --git)
```

**Error-handling:**

- **`Gofmt` & `GoModTidy` = fail-fast.** Keduanya prasyarat "build hijau tanpa
  edit" (DoD #1). Gagal → `Generate`/orchestration mengembalikan error,
  exit non-zero. (Project sudah ter-tulis; pesan error menyebut hook yang gagal.)
- **`GitInit` = warn-only.** Kegagalan `git` (mis. git tak terpasang) **tidak**
  membatalkan generate — project tetap valid tanpa `.git/`. Builder mencetak
  peringatan dan exit 0. Konsisten dengan SPEC §5.2 poin 5 (git tak mempengaruhi
  isi project).

**Kapan `GitInit` di-skip:**

- Mode **non-interaktif / CI**: `--git` default **false** → `GitInit` tidak masuk
  `plan.Hooks` (SPEC §5.1 / §6.3). Eksplisit `--no-git` juga skip.
- Mode **interaktif**: `q_git` default **yes** → `GitInit` masuk plan kecuali user
  memilih no.
- `GitInit` masuk `plan.Hooks` **hanya** bila `Answers.Git == true`; resolver yang
  memutuskan (bukan generator).

### 8. Safety flow (EnsureEmptyDir × DryRunWriter)

```
Generate(p, target, w):
  if _, isDry := w.(*fsutil.DryRunWriter); !isDry {
      if err := fsutil.EnsureEmptyDir(target); err != nil { return err }   // US-06 Sk.3
  }
  ... proses FileOp via w ...
  if isDry { print tree dari w.Planned ; print p.Deps ; return nil }       // SPEC §5.4
```

- **`EnsureEmptyDir(target)`** dipanggil **sebelum** operasi tulis apa pun, **hanya
  untuk RealWriter**. Bila `target` ada & tidak kosong → error, nol penulisan
  (US-06 Sk.3). Bila belum ada/kosong → lanjut. (Jalur `add service` melewati cek
  ini karena memang menulis ke project existing.)
- **`--dry-run`** memilih `&DryRunWriter{}` sebagai `Writer`. Setiap `Mkdir`/
  `WriteFile` hanya **meng-append path ke `Planned`** (tidak menyentuh disk). Di
  akhir, generator mencetak `Planned` sebagai tree + daftar `Deps` (rencana
  `go.mod`). Karena seluruh percabangan struktural sudah final di `GeneratePlan`,
  preview = **persis** byte/relasi yang akan ditulis (SPEC §5.4, US-06 Sk.1).
- **Interaksi keduanya:** dry-run **tidak** memanggil `EnsureEmptyDir` (tak relevan
  — tak ada tulis) dan **tidak** menjalankan hook (§7). Untuk kombinasi invalid,
  `--dry-run` tetap menjalankan `Resolve` → bila `ErrConstraint`, exit non-zero
  **sebelum** mencetak rencana (US-06 Sk.2).

### 9. Testability

#### 9.1 Golden-file (snapshot output deterministik)

Layout `testdata/`:

```
internal/generator/testdata/golden/
├── <combo-key>/                       # satu direktori per kombinasi kunci
│   ├── answers.yaml                   # input Answers untuk kombinasi ini
│   └── want/                          # pohon file output yang diharapkan (byte-exact)
│       ├── go.mod
│       ├── cmd/...
│       └── ...
└── ...
```

- **Satu golden per kombinasi kunci.** `<combo-key>` = string ringkas deterministik
  (mis. `monolith-rest-chi-postgres-sqlx`). Test memuat `answers.yaml` → `Resolve`
  → `Generate` ke memori (writer in-memory) → bandingkan byte-exact dengan `want/`.
- **Flag `-update`.** `go test ./internal/generator -run Golden -update` me-regen
  isi `want/` dari output saat ini. Diff golden ditinjau sebagai bagian review
  (ADR-003 D7). Tanpa `-update`, mismatch = test gagal.
- **Invarian byte-identical (SPEC §5.2) diuji** dengan menjalankan jalur input yang
  berbeda (proyeksi "interaktif" vs "flag") ke `Answers` setara → output **harus**
  identik dengan golden yang sama.
- **Unit-test FuncMap.** Tiap dari 7 fungsi (§4) punya tabel test I/O sendiri.
- **Unit-test `when`.** Tabel kasus per operator (`eq`/`ne`/`and`/`or`/`not`/kosong)
  + kasus field ilegal → `ErrConstraint`.

#### 9.2 E2E "generate → go build"

- **Pemilihan kombinasi (~6–10, mewakili percabangan utama):**
  1. `monolith-rest-nethttp-nodb` (profil zero-config, SPEC §6.3)
  2. `monolith-rest-chi-postgres-sqlx-migrate-docker`
  3. `monolith-worker-nodb` (cabang C9: tanpa HTTP)
  4. `modular-rest-nethttp-postgres` (US-03: boundary berduri)
  5. `microservice-grpc-2svc-postgres-docker` (US-04: gen/ di-commit)
  6. `microservice-rest-gateway` (C6: gateway implisit)
  7. `monolith-rest-fiber-mysql-gorm` (cabang fasthttp C7/C8 + Go 1.25)
  8. `monolith-rest-sqlite` (C14: compose tanpa service DB)
  9. `monolith-rest-mongo` (C-mongo: skip access/migrate SQL)
  10. `monolith-rest-postgres-sqlc` (C10: needs-step codegen di-commit)
- **Mekanisme.** Test E2E (ber-build-tag `e2e` agar `go test ./...` default tetap
  cepat & offline) men-`Generate` tiap kombinasi ke `t.TempDir()` dengan
  `RealWriter`, lalu menjalankan `go vet ./... && go build ./... && go test ./...`
  via `os/exec` di temp dir itu. Lolos = hijau tanpa edit (DoD #1).
- **Hermetis.** E2E memerlukan toolchain `go` di PATH (wajar — user gostarter
  punya Go). Tidak memerlukan Docker (testcontainers di balik build-tag
  `integration`, C13). Kombinasi microservice diverifikasi build **tanpa**
  `buf generate` karena `gen/go/` di-commit (US-04 Sk.1).

## Consequences

### Positif

- **Sumber kebenaran tunggal.** §Decision 3 mengakhiri divergensi tipe; skeleton &
  ADR-003 tinggal menyalin. Tidak ada lagi "kontrak ganda".
- **Byte-identical by construction.** Satu `Answers` → satu `Resolve` → satu
  `Generate`; semua urutan distabilkan resolver; `go.mod` via `modfile.Format`;
  `.go` via `go/format`. Invarian SPEC §5.2 terjamin struktural, bukan disiplin.
- **Dry-run akurat & murah.** Percabangan final di `GeneratePlan` → `DryRunWriter`
  cukup mengakumulasi `Planned`; preview = output sesungguhnya.
- **Core ter-testable tanpa I/O.** `Prompter`/`Writer`/`PostGenHook` mengisolasi
  `huh`/`os`/`exec`; resolver+generator diuji murni in-memory + golden-file.
- **Zero lock-in & single-binary terjaga.** Dependency builder hanya di binary;
  template di-embed; output tidak pernah meng-import builder.
- **Dogfood layout.** Builder memakai Kandidat B yang ia rekomendasikan → bukti
  hidup kelayakan layout default.

### Negatif

- **Penyelarasan wajib (one-off).** Skeleton Fase 2 (`FileOpKind`→`FileOpMode`,
  `[]string`→`[]FileSpec`, `map[string]string`→`map[string]any`, tambah `When`/
  `ModeCopy`/`Fragments`/`merge.go`, `DryRunWriter.Planned` + pointer-receiver,
  rename `core-monolith`→`core`+`arch-monolith`) dan ADR-003 harus diubah agar
  sesuai §3. Pekerjaan mekanis tetapi menyentuh banyak file.
- **`when` adalah mini-bahasa.** Butuh evaluator + test sendiri di resolver
  (Fase 3). Ditekan dengan grammar minimal (§5) tanpa aritmetika/FuncMap.
- **Dua mekanisme `go.mod`.** `modfile` (authoring) + `go mod tidy` (finalisasi)
  berarti runtime generate butuh toolchain `go` — diterima (ADR-001).
- **Beban golden-file.** Setiap perubahan output meminta regen `-update` + review
  diff. Sengaja, demi mendeteksi regresi byte-level.

## Alternatives Considered

- **Tanpa `resolver` (keputusan di template / generator) — DITOLAK.** Menyebar
  default & constraint SPEC §6 ke dalam `.tmpl` atau generator membuat percabangan
  struktural tersembunyi → dry-run tak akurat, golden tak stabil, byte-identical
  rapuh. Resolver sebagai satu-satunya "otak keputusan" adalah prasyarat SPEC §5.2
  & §5.4.
- **Marker-comment di file OUTPUT untuk semua merge — DITOLAK (untuk generate
  awal).** Menyisip dengan membaca-balik file output menuntut file sudah ada →
  rapuh untuk dry-run & rawan non-determinisme. Dipilih anchor di **skeleton
  template** + assembler (ADR-003 D5). Marker output hanya dipakai terbatas untuk
  `add service` (project existing).
- **`go.mod` sebagai fragment teks (ModeMerge) — DITOLAK.** Merakit `require` dari
  potongan teks rawan format/urutan tak konsisten → memecah byte-identical.
  Dialirkan via `plan.Deps` → `modfile.Format` (ADR-001 §4) yang menormalisasi.
- **Library scaffolding pihak ketiga (cookiecutter-style / go-blueprint sebagai
  dependency) — DITOLAK (rujuk ADR-001).** Menambah lock-in & toolchain;
  bertentangan dengan single-binary + zero lock-in. Engine tetap stdlib
  `text/template` + `embed.FS` + `go/format` + `x/mod/modfile`.
- **`sprig` untuk FuncMap — DITOLAK (rujuk ADR-001 §"Template engine").**
  Over-engineering untuk 6 helper; ditulis sendiri (§4) dan di-unit-test.
- **Map mode sebagai string bebas (tanpa enum `FileOpMode`) — DITOLAK.** Enum
  ber-konstanta (`ModeRender/ModeCopy/ModeMkdir/ModeMerge`) memberi exhaustiveness
  & menghindari salah-ketik; string `mode` di `module.yaml` dipetakan ke enum saat
  load (`module.FileSpec.Mode` string → `plan.FileOpMode`).

## References

- `docs/SPEC.md` — §4 (Question Flow), §5 (Flags + §5.2 byte-identical + §5.4
  dry-run), §6 (Constraint matrix C1–C20/C-mongo + default resolution + zero-config),
  §7 (US-01..US-07 + Definition of Done).
- `docs/adr/ADR-001-builder-stack.md` — stack: `cobra`, `huh`, `text/template` +
  `embed.FS` + `FuncMap` + `go/format`, `x/mod/modfile`; penolakan `sprig`/`templ`/
  `survey`; pembagian authoring (`modfile`) vs finalisasi (`go mod tidy`).
- `docs/adr/ADR-003-template-system.md` — bentuk data template: `module.yaml`,
  `FileSpec`, `MergeContribution`, anchor/fragment, conditional `when` (D4),
  merge fragment+assembler (D5), validasi `Load` (D6), versioning (D7). ADR-003
  merujuk balik ke ADR-002 §3/§4/§5/§6 sebagai kontrak (menggantikan rujukan
  BACKBONE).
- `docs/research/01-monolith.md` — §1.1 (`internal/`/`cmd/`/`pkg/`), §2 Kandidat
  A/B/C (tree layout), §4 (default B untuk monolith, C untuk modular). Layout
  builder men-dogfood Kandidat B.
- `docs/research/05-decision-matrix.md` — §2 (dimensi → modul aktif → file → dep),
  §3 (constraint C1–C20/C-mongo), §4 (default resolution + zero-config). Sumber
  decision matrix yang dijalankan resolver.
