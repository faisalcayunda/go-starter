# 04 — Analisis Kompetitor & Stack Builder gostarter

> Dokumen riset untuk **gostarter** — CLI Go yang men-generate struktur project Go best-practice (pengalaman mirip `laravel new` / `composer create-project`).
> Verifikasi status maintenance setiap library dilakukan per **2026-06-06** lewat pkg.go.dev (versi + tanggal publish) dan halaman GitHub (badge archived / rilis terakhir). Jangan mengandalkan ingatan — semua angka di bawah berasal dari verifikasi web pada tanggal tersebut.

## 1. Pendahuluan

gostarter v1 menargetkan tiga mode arsitektur dalam satu generator: (1) monolith sederhana, (2) modular monolith, (3) microservice. Aturan keras hasil generate:

- Lolos `go vet ./... && go build ./... && go test ./...` **tanpa edit manual**.
- `docker compose up` jalan untuk stack yang memakai DB.
- Project hasil generate **TIDAK BOLEH** meng-import package apa pun dari builder (**zero lock-in**).
- Hanya struktur + wiring + 1 contoh minimal; bukan business logic.

Dokumen ini menjawab dua pertanyaan riset:

1. **Analisis Kompetitor** — siapa generator/scaffolding Go yang ada, apa yang mereka dukung, dan di mana celah pasar yang diisi gostarter.
2. **Stack Builder gostarter** — library apa yang dipakai untuk membangun builder-nya sendiri (bukan output generate), beserta alasan dan sumber resmi tiap pilihan.

Pemisahan penting: stack builder di Bagian 3 **hanya dipakai oleh tool builder**. Output generate tetap zero lock-in karena tidak meng-import satu pun dependency builder.

---

## 2. Analisis Kompetitor

> Verifikasi maintenance per **2026-06-06** via pkg.go.dev + halaman GitHub (releases/commits). Tidak ada satu pun kompetitor yang dirilis sebagai produk **multi-arsitektur** (monolith + modular monolith + microservice) dalam satu generator dengan jaminan "build/test hijau tanpa edit + zero lock-in" — itu ruang kosong yang diisi gostarter.

### 2.1 Tabel Ringkas

| Tool | Mode | Arsitektur didukung | Stack opsi | Status maintenance (per 2026-06-06) | Catatan UX |
|---|---|---|---|---|---|
| **melkeydev/go-blueprint** | Interaktif (TUI Bubbletea) **+** flags | Web-service layout tunggal (bukan monolith/microservice eksplisit); flat `cmd/ internal/` | Framework: std `net/http`, Chi, Gin, Fiber, HttpRouter, Gorilla/mux, Echo · DB driver: MySQL, Postgres, SQLite, Mongo, Redis, ScyllaDB · Advanced: HTMX+Templ, Tailwind, React/Vite, WebSocket, Docker, GitHub Actions, GoReleaser | **Aktif (melambat / hampir stagnan).** Rilis terakhir **v0.10.11 (10 Jul 2024)** — verifikasi langsung halaman releases menunjukkan tahun **2024**, bukan 2025. 8.8k★. Tidak ada rilis baru ~23 bulan terakhir per 2026-06-06. | Paling matang. Flow tanya berurutan + ada "Blueprint UI" web untuk menyusun command/preview struktur. |
| **SchwarzIT/go-template** (`gt`) | Interaktif (prompt berurutan) | Production-ready service tunggal (DDD-leaning, `pkg/ internal/`) | Opsional: gRPC, OpenAPI/HTTP, CI (GitLab/GitHub), linter, Dockerfile, Helm/k8s. Berbasis 1 template-repo besar yang dikonfigurasi via Q&A | **Stagnan.** Berbasis korporat (Schwarz/Lidl Grup). Rilis terakhir **v0.5.0 (8 Agu 2023)** — ~2,8 tahun tanpa rilis baru per 2026-06-06; tidak ada penanda archived. Sumber: [releases](https://github.com/SchwarzIT/go-template/releases). | "Wizard" tanya-jawab → init git + go module otomatis. Sangat opinionated, sulit dikustom keluar dari template tunggalnya. |
| **create-go-app/cgapp** | Interaktif **+** flags | Layout backend + frontend terpisah (full-stack), bukan microservice multi-service | Backend: net/http, Fiber, go-chi · Frontend: React/Preact/Vue/Svelte/Solid/Lit/Qwik/Next/Nuxt (JS+TS) via Vite · Deploy: Docker + Ansible (Makefile-driven) | **Stagnan / risiko abandon.** Rilis terakhir **v4.1.0 (21 Agu 2023)** — ~2.8 tahun lalu. Belum archived, tapi tidak ada rilis baru. **Hindari sebagai referensi DB-stack modern.** | Pengalaman mirip `create-react-app`. Kuat di full-stack scaffolding, lemah di backend-architecture depth. |
| **go-zero `goctl`** | CLI deklaratif (file `.api`/`.proto` → codegen), **bukan** wizard interaktif | Microservice (API gateway + RPC services), monolith-API juga bisa | Generate dari DSL: HTTP API, gRPC/RPC, model dari DDL SQL, k8s/Dockerfile, Swagger. Built-in: rate-limit, breaker, load-shedding | **Sangat aktif.** Modul terakhir publish **~12 Feb 2026** (Go 1.23, MCP SDK); repo update ~13 Mei 2026. Komunitas besar. | Bukan "new project wizard" tapi **spec-first codegen**. Mengikat erat ke framework go-zero (lock-in tinggi). |
| **Kratos CLI** (`kratos new`) | Semi-interaktif (pilih template Service/Admin) | Microservice (API-first, proto-driven) | Clone `kratos-layout`; proto → HTTP+gRPC codegen, wire DI, config, registry | **Sangat aktif.** Kratos terakhir **v2.9.2 (5 Des 2025)**; jalur **v3** sedang berjalan. | `kratos new <svc>` clone layout repo + ganti module path. Per-service, bukan orkestrasi multi-service. Lock-in ke framework Kratos. |
| **cookiecutter** (pola generik) | Interaktif (prompt dari `cookiecutter.json`) | Agnostik — apa pun yang ditaruh di template | Apa saja (Jinja2 templating, hooks pre/post-gen). Bukan Go-specific | **Aktif.** Python 3.10–3.14, ekosistem template ramai. | Pola referensi: prompt-driven + post-gen hooks. Tidak ada jaminan "go build hijau" — itu tanggung jawab author template. |

> **Catatan akurasi (verifikasi 2026-06-06):** sumber awal menyebut rilis go-blueprint "v0.10.11 (10 Jul 2025)". Verifikasi langsung halaman [releases](https://github.com/Melkeydev/go-blueprint/releases) mengoreksi tanggal menjadi **10 Jul 2024**. Implikasi positioning **bertambah kuat**: jeda tanpa rilis ~23 bulan (bukan ~11 bulan) — go-blueprint lebih dekat ke stagnan daripada "melambat".

**Sumber:** [Melkeydev/go-blueprint](https://github.com/Melkeydev/go-blueprint) · [releases](https://github.com/Melkeydev/go-blueprint/releases) · [pkg.go.dev/go-blueprint](https://pkg.go.dev/github.com/melkeydev/go-blueprint) · [SchwarzIT/go-template](https://github.com/SchwarzIT/go-template) · [create-go-app/cli](https://github.com/create-go-app/cli) · [create-go-app/cli releases](https://github.com/create-go-app/cli/releases) · [zeromicro/go-zero](https://github.com/zeromicro/go-zero) + [goctl](https://github.com/zeromicro/go-zero/tree/master/tools/goctl) · [go-kratos/kratos](https://github.com/go-kratos/kratos) + [kratos releases](https://github.com/go-kratos/kratos/releases) + [kratos-layout](https://github.com/go-kratos/kratos-layout) · [cookiecutter/cookiecutter](https://github.com/cookiecutter/cookiecutter)

---

### 2.2 Subbagian Khusus: melkeydev/go-blueprint (kompetitor paling mirip)

**Status:** v0.10.11 (**10 Jul 2024** — terverifikasi via halaman releases), 8.8k★ — aktif tapi nyaris stagnan (jeda rilis ~23 bulan per 2026-06-06). Lisensi MIT, 92% Go. Sumber: [releases](https://github.com/Melkeydev/go-blueprint/releases) · [commits/main](https://github.com/Melkeydev/go-blueprint/commits/main).

**Flow pertanyaan (UX):**

1. Jalankan `go-blueprint create` → TUI (Bubbletea/Huh-style) prompt berurutan.
2. Urutan tanya: **nama project** → **framework** (std/Chi/Gin/Fiber/HttpRouter/Gorilla/Echo) → **DB driver** (MySQL/Postgres/SQLite/Mongo/Redis/ScyllaDB) → **(opsional) advanced features** via `--advanced` → **git init** (none/skip/commit).
3. Mode non-interaktif penuh via flags:
   `go-blueprint create --name my-project --framework gin --driver postgres --git commit`
   Advanced: `--advanced --feature htmx,docker,githubaction`.
4. Ada **"Blueprint UI"** (web app) untuk menyusun command + **preview struktur direktori/file** sebelum generate. Sumber: [README](https://github.com/Melkeydev/go-blueprint/blob/main/README.md).

**Flag yang didukung:** `--name`, `--framework`, `--driver`, `--git` (none/stage/commit), `--advanced`, `--feature` (htmx, tailwind, react, websocket, docker, githubaction).

**Cara penyusunan template internal (pelajaran desain):**

- Template hidup di `cmd/template/`, **dipecah per dimensi**: `framework/`, `dbdriver/`, `advanced/`, `docker/`. Ada file Go pendamping seperti `globalEnv.go`. Sumber: [tree/main/cmd/template](https://github.com/Melkeydev/go-blueprint/tree/main/cmd/template).
- Pendekatan: **composition by category** — generator menumpuk potongan template (framework × db × feature) menjadi satu project, alih-alih 1 template-repo monolitik (kontras dengan go-template/Kratos yang clone 1 repo lalu tambal).
- Template di-embed ke binary (pola umum `go:embed` + `text/template`), sehingga binary self-contained tanpa fetch jaringan.
- **Manipulasi go.mod:** module path diisi dari `--name` saat render `go.mod` (placeholder template diganti nama project); README/docs tidak mendokumentasikan rename antar-direktori internal — konsisten dengan layout flat single-module.

**Kelebihan:**

- UX paling halus di kelas Go generator (TUI + web preview + flags + brew/npm/go install).
- Cakupan framework & DB driver terluas.
- **Zero lock-in** secara de-facto: output tidak meng-import package go-blueprint (model yang ingin ditiru gostarter).
- Output disusun agar langsung `go build`/jalan.

**Kekurangan:**

- **Hanya satu paradigma layout** (web service flat `cmd/ internal/`). Tidak ada konsep **monolith vs modular monolith vs microservice**.
- **Tidak ada multi-service**: tak bisa generate >1 service atau menambah service ke project yang sudah ada (no `add-service`).
- Tidak ada modular-monolith (module boundaries/internal domains) sebagai mode pertama-kelas.
- Rilis nyaris stagnan (~23 bulan tanpa rilis baru per 2026-06-06).

**Pelajaran desain template untuk gostarter:**

1. **Composition by category** (framework × db × feature) lebih scalable daripada 1 template-repo per kombinasi — adopsi, tapi perluas dengan dimensi **arsitektur**.
2. **Embed template + render placeholder** untuk module path = binary self-contained, offline, deterministik → mendukung jaminan "tanpa edit manual".
3. **Web/preview struktur** sebelum generate = pembeda UX kuat; layak dipertimbangkan.
4. Sediakan **mode flags non-interaktif** sejak v1 (penting untuk CI & reproducibility).

---

### 2.3 Gap Analysis → Diferensiator gostarter

| Kebutuhan / Fitur | go-blueprint | go-template | cgapp | goctl | Kratos CLI | cookiecutter | **gostarter (target)** |
|---|---|---|---|---|---|---|---|
| Monolith sederhana | ✓ (flat) | ✓ | ✓ | ~ | ✗ | author | **✓ first-class** |
| **Modular monolith** (module boundaries) | ✗ | ~ | ✗ | ✗ | ✗ | author | **✓ first-class** |
| **Microservice multi-service** (>1 service 1 generator) | ✗ | ✗ | ✗ | ✓ (per-service) | ✓ (per-service) | author | **✓ multi-service dalam 1 project** |
| **`add-service` ke project existing** | ✗ | ✗ | ✗ | ~ (codegen ulang) | ✗ | ✗ | **✓ (diferensiator kunci)** |
| Pilih mode arsitektur saat generate | ✗ | ✗ | ✗ | ✗ | ✗ (template fix) | ✗ | **✓ 3 mode** |
| **Zero lock-in** (output tak import builder) | ✓ | ✓ | ✓ | ✗ (import go-zero) | ✗ (import kratos) | ✓ | **✓ aturan keras** |
| Jaminan `go vet && build && test` hijau tanpa edit | implisit | implisit | rapuh (lama) | ✓ (codegen) | ✓ | tidak dijamin | **✓ kontrak eksplisit** |
| `docker compose up` jalan untuk stack DB | ~ (Docker opsional) | ~ | ~ | ~ | ~ | author | **✓ kontrak eksplisit** |
| Mode interaktif + flags | ✓ | ~ | ✓ | ✗ (spec-first) | ~ | ✓ | **✓ keduanya** |

**Diferensiator inti gostarter (yang TIDAK dipunyai kompetitor mana pun):**

1. **Satu tool, tiga paradigma arsitektur** (monolith / modular monolith / microservice). Semua kompetitor mengunci satu layout: go-blueprint/go-template/cgapp = service flat tunggal; goctl/Kratos = per-service microservice (harus orkestrasi manual untuk multi-service).
2. **Microservice multi-service dalam satu project hasil generate** + perintah **`add-service`** untuk menambah service ke project existing. goctl/Kratos generate satu service per invocation; tidak ada yang mengelola layout multi-service end-to-end ataupun penambahan inkremental.
3. **Modular monolith sebagai mode pertama-kelas** dengan batas modul/domain yang dipaksakan struktur — celah yang benar-benar kosong (go-blueprint flat; Kratos/goctl langsung microservice).
4. **Zero lock-in sebagai aturan keras yang ditegakkan**, bukan kebetulan. goctl & Kratos **gagal** kriteria ini (output mengimpor framework masing-masing). Hanya go-blueprint/go-template/cgapp yang zero-lock-in, tapi mereka tak punya multi-arsitektur.
5. **Kontrak kualitas eksplisit**: `go vet ./... && go build ./... && go test ./...` hijau tanpa edit manual + `docker compose up` jalan untuk stack DB. Kompetitor memperlakukan ini implisit (cgapp bahkan berisiko basi karena stagnan sejak 2023).
6. **Positioning vs yang stagnan**: cgapp stagnan (rilis terakhir Agu 2023) dan go-blueprint nyaris stagnan (tak ada rilis ~23 bulan per 2026-06-06) → ada ruang untuk generator Go modern yang aktif dengan jaminan kualitas yang lebih kuat.

**Sumber tambahan:** [go-zero](https://github.com/zeromicro/go-zero) (microservice, lock-in) · [kratos-layout](https://github.com/go-kratos/kratos-layout) (per-service template) · [create-go-app/cli releases v4.1.0 2023](https://github.com/create-go-app/cli/releases) · [SchwarzIT/go-template](https://github.com/SchwarzIT/go-template).

---

## 3. Stack Builder gostarter

> Konteks: stack ini untuk **builder** gostarter (tool yang men-generate project), bukan untuk project hasil generate. Zero lock-in tetap terjaga karena dependency berikut hanya dipakai oleh builder; output generate hanya berisi struktur + wiring + 1 contoh minimal dan tidak meng-import package apa pun dari builder.
> Status maintenance diverifikasi per **2026-06-06** lewat pkg.go.dev (versi + tanggal publish) dan halaman GitHub (badge archived/rilis terakhir).

### 3.1 Tabel Keputusan Stack

| Komponen | Pilihan (DEFAULT) | Kandidat lain | Status maintenance (verifikasi 2026-06-06) | Alasan ringkas |
|---|---|---|---|---|
| Command framework | **spf13/cobra** v1.10.2 | urfave/cli v3 (v3.9.0); stdlib `flag` | **Aktif.** cobra v1.10.2 publish **2025-12-03** (pkg.go.dev), pre-release v1.10.3-* April 2026. urfave/cli v3.9.0 publish **2026-05-09** — juga aktif. | Subcommand bertingkat (`create`, `add service`), persistent + local flags, hooks Pre/PostRun, shell completion otomatis. De facto standard & dipakai kompetitor langsung (kubectl, gh, hugo, docker). |
| Interactive TUI / prompt | **charmbracelet/huh** v1.0.0 (`github.com/charmbracelet/huh`) + **bubbletea** v1.3.10 + **lipgloss** v1.1.0 | AlecAivazis/survey; manioe/promptui; murni `bufio` stdin | huh v1.0.0 **2026-02-23**, bubbletea v1.3.10 **2025-09-17**, lipgloss v1.1.0 **2025-03-12** — semua rilis **stabil** & **aktif** (Charm). `survey` **ARCHIVED 2024-04-19**, README: "no longer maintained". Catatan: jalur v2 Charm masih pre-release/beta/untagged sebagai modul stabil di pkg.go.dev per 2026-06-06 (huh/v2 belum punya tag stabil, lipgloss/v2 = `v2.0.0-beta.3`, bubbletea/v2 = `v2.0.0-beta.6`), sehingga ditunda demi reproducibility (selaras ADR-001). Sumber versi: <https://pkg.go.dev/github.com/charmbracelet/huh?tab=versions>. | huh = form/prompt deklaratif (Select, Input, Confirm, MultiSelect) untuk wizard `create` (pilih arsitektur, modul path, opsi DB). Dibangun di atas bubbletea (loop) + lipgloss (styling). Survey ditolak karena archived. |
| Template engine | **stdlib `text/template` + `embed.FS`** + custom funcs | `html/template` (tidak relevan, ini bukan HTML); Go templ; sprig (`Masterminds/sprig`) | stdlib = bagian Go release, didukung selama Go didukung. `embed.FS` stabil sejak Go 1.16. | Cukup untuk generate file `.go`, `go.mod`, `docker-compose.yml`, `Dockerfile`. Template dibundel via `embed.FS` (tanpa file eksternal saat runtime). Custom `template.FuncMap` untuk casing (PascalCase/snake_case), modulePath join, dst. Tidak perlu engine eksternal — menambah dependency tanpa nilai tambah berarti. |
| Manipulasi `go.mod` | **`golang.org/x/mod/modfile`** v0.36.0 | `os/exec` memanggil `go mod edit`/`go mod tidy` | x/mod v0.36.0 publish **2026-05-08** (pkg.go.dev) — **aktif**, modul resmi tim Go. | Parse/edit `go.mod` terprogram (`AddRequire`, `DropRequire`, `AddGoStmt`, `SetRequire`, `Format`) tanpa bergantung pada toolchain `go` ada di PATH saat generate. Lebih andal & deterministik daripada parsing output `os/exec`. Catatan: `go mod tidy` tetap dipakai sebagai langkah verifikasi akhir (lewat exec), bukan untuk authoring. |

### 3.2 Narasi per komponen

**1. Command framework — spf13/cobra (DEFAULT)**

cobra adalah de facto standard CLI Go: dipakai `kubectl`, `docker`, `gh` (GitHub CLI), dan Hugo — 173k+ project. Untuk gostarter ini penting karena pengalaman target ("laravel new") menuntut command tree bertingkat (`gostarter create`, `gostarter add service <name>`), flag lengkap (persistent untuk global, local per-subcommand), hooks `PreRunE`/`PostRunE` untuk validasi, serta auto-completion. Status: v1.10.2 publish 2025-12-03, dengan commit & pre-release aktif hingga April 2026 — sehat. urfave/cli v3 (v3.9.0, 2026-05-09) adalah alternatif sah dan juga aktif; API-nya lebih ringkas/deklaratif tetapi ekosistem contoh, generator, dan familiaritas tim lebih besar di cobra. Untuk tool scaffolding yang ingin "terlihat seperti tooling Go pada umumnya", cobra menang dari sisi konvensi & adopsi.

- Sumber: <https://github.com/spf13/cobra> · <https://pkg.go.dev/github.com/spf13/cobra> · <https://cobra.dev/> · <https://pkg.go.dev/github.com/urfave/cli/v3> · <https://github.com/urfave/cli>

**2. Interactive TUI — charmbracelet/huh + bubbletea + lipgloss (DEFAULT)**

Wizard interaktif `create` (pilih monolith / modular monolith / microservice, isi module path, toggle DB) paling bersih memakai **huh** — library form/prompt deklaratif dari Charm, dibangun di atas **bubbletea** (Elm-style event loop) dan **lipgloss** (styling). Ketiganya dipin ke rilis **stabil v1.x**: huh v1.0.0 (2026-02-23), bubbletea v1.3.10 (2025-09-17), lipgloss v1.1.0 (2025-03-12) — dengan import path tanpa suffix `/v2` (`github.com/charmbracelet/huh`, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`). Jalur v2 Charm **masih pre-release/beta/untagged** sebagai modul stabil di pkg.go.dev per 2026-06-06 (huh/v2 belum punya tag stabil, lipgloss/v2 = `v2.0.0-beta.3`, bubbletea/v2 = `v2.0.0-beta.6`), sehingga **ditunda** demi reproducibility build builder (selaras ADR-001); migrasi ke v2 menyusul setelah Charm men-tag rilis v2 stabil. **AlecAivazis/survey DITOLAK**: repo di-**archive 2024-04-19**, README eksplisit "This project is no longer maintained" dan justru mengarahkan ke bubbletea — tidak boleh jadi default. Untuk mode non-interaktif (CI/flag lengkap), cobra flags tetap jalan tanpa huh, jadi TUI bersifat progressive enhancement.

- Sumber: <https://github.com/charmbracelet/huh> · <https://pkg.go.dev/github.com/charmbracelet/huh?tab=versions> · <https://github.com/charmbracelet/bubbletea> · <https://github.com/charmbracelet/lipgloss> · <https://github.com/AlecAivazis/survey> (badge "archived")

**3. Template engine — stdlib text/template + embed.FS (DEFAULT)**

Output generate adalah file teks (`.go`, `go.mod`, `docker-compose.yml`, `Dockerfile`, `Makefile`). `text/template` stdlib sudah cukup: kontrol alur, range, dan `template.FuncMap` untuk fungsi casing (`toPascal`, `toSnake`, `toKebab`), penyusunan module path, dan default value. Template di-embed ke binary lewat `embed.FS` (Go ≥1.16) sehingga builder berdiri sendiri tanpa file template eksternal saat runtime — penting untuk distribusi single-binary. Tidak perlu engine eksternal (mis. sprig/templ): menambah dependency hanya untuk helper yang bisa ditulis 20 baris adalah over-engineering. Catatan teknis: untuk template yang menghasilkan kode Go, lewatkan hasil melalui `go/format` (gofmt) agar output rapi & deterministik — ini stdlib juga.

- Sumber: <https://pkg.go.dev/text/template> · <https://pkg.go.dev/embed> · <https://pkg.go.dev/go/format>

**4. Manipulasi go.mod — golang.org/x/mod/modfile (DEFAULT)**

Saat `add service` atau memilih stack DB, builder perlu menyunting `go.mod` (set module path, `go` directive, tambah/hapus require). **`golang.org/x/mod/modfile`** (modul resmi tim Go, v0.36.0 publish 2026-05-08) menyediakan parse + edit terprogram via AST (`Parse`, `AddRequire`, `DropRequire`, `AddGoStmt`, `SetRequireSeparateIndirect`, `Format`) — deterministik, tidak butuh binary `go` di PATH saat menyusun file, dan tidak rapuh terhadap perubahan format output CLI. Alternatif `os/exec` memanggil `go mod edit -json`/`go mod tidy` lebih sederhana untuk kasus trivial tetapi menambah ketergantungan environment dan parsing teks/JSON yang rawan. Strategi gabungan yang dipakai: **authoring/edit go.mod via modfile**, lalu **verifikasi akhir** ("aturan keras": `go vet`/`build`/`test` lolos) tetap menjalankan `go mod tidy` via exec sebagai langkah finalisasi pada project hasil generate.

- Sumber: <https://pkg.go.dev/golang.org/x/mod/modfile> · <https://pkg.go.dev/golang.org/x/mod> · <https://pkg.go.dev/cmd/go#hdr-Edit_go_mod_from_tools_or_scripts>

---

## 4. Penutup — Ringkasan Diferensiator & Keputusan Stack Builder

### 4.1 Diferensiator utama gostarter

Tidak ada satu pun kompetitor (go-blueprint, go-template, cgapp, goctl, Kratos CLI, cookiecutter) yang menawarkan **multi-arsitektur dalam satu generator** dengan zero lock-in dan jaminan build/test hijau. Empat pembeda yang tidak dimiliki siapa pun:

1. **Satu tool, tiga paradigma** — monolith / modular monolith / microservice; kompetitor mengunci satu layout.
2. **Multi-service dalam satu project + `add-service` inkremental** — goctl/Kratos hanya per-service per invocation.
3. **Modular monolith first-class** — celah pasar yang benar-benar kosong.
4. **Kontrak kualitas eksplisit & zero lock-in yang ditegakkan** — bukan implisit/kebetulan seperti pada kompetitor; goctl & Kratos bahkan gagal kriteria zero lock-in.

Momentum pasar mendukung: cgapp stagnan (rilis terakhir Agu 2023) dan go-blueprint nyaris stagnan (rilis terakhir **Jul 2024**, jeda ~23 bulan per 2026-06-06).

### 4.2 Keputusan stack builder (akan diformalkan di ADR-001)

| Komponen | Keputusan DEFAULT | Status (2026-06-06) |
|---|---|---|
| Command framework | **spf13/cobra** v1.10.2 | Aktif (publish 2025-12-03; pre-release v1.10.3-* April 2026) |
| Interactive TUI | **charmbracelet/huh v1.0.0** (+ bubbletea v1.3.10 + lipgloss v1.1.0) | Aktif (huh v1.0.0 2026-02-23; bubbletea v1.3.10 2025-09-17; lipgloss v1.1.0 2025-03-12). Jalur v2 Charm masih pre-release/beta/untagged di pkg.go.dev per 2026-06-06 → ditunda demi reproducibility (selaras ADR-001) |
| Template engine | **stdlib `text/template` + `embed.FS`** (+ `go/format`) | Stdlib — didukung selama Go didukung |
| Manipulasi `go.mod` | **`golang.org/x/mod/modfile`** v0.36.0 | Aktif (publish 2026-05-08; modul resmi tim Go) |

Library yang **dihindari sebagai default**: `AlecAivazis/survey` (archived 2024-04-19, "no longer maintained"). cgapp tidak relevan sebagai dependency builder, hanya sebagai referensi kompetitor yang stagnan.

Semua dependency di atas **hanya** dipakai oleh builder; output hasil generate tetap **zero lock-in** sesuai aturan keras gostarter. Keputusan ini akan diformalkan di **ADR-001 — Stack Builder gostarter**.
