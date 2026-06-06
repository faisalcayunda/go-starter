---
status: Proposed
date: 2026-06-06
deciders: [isal]
tags: [builder, tooling, go]
---

# ADR-001: Stack untuk Builder gostarter

## Status

**Proposed** — 2026-06-06.

Catatan verifikasi: seluruh status maintenance library di dokumen ini diverifikasi
langsung via web (pkg.go.dev + halaman GitHub repo) pada tanggal keputusan
(2026-06-06), bukan dari ingatan. Lihat bagian **Referensi**.

## Context

`gostarter` adalah CLI berbasis Go yang men-generate struktur project Go
best-practice (pengalaman mirip `laravel new` / `composer create-project`). v1
mendukung 3 mode arsitektur: monolith sederhana, modular monolith, dan
microservice.

Builder ini adalah **tool terpisah** dari project yang dihasilkannya. Untuk
membangun builder-nya, kita butuh keputusan stack yang tegas pada empat komponen:

1. **Command framework** — builder harus jalan dalam dua mode:
   - **Interaktif**: wizard (`gostarter create`) yang menuntun user memilih
     arsitektur, module path, dan opsi DB.
   - **Non-interaktif**: sepenuhnya via flag (`gostarter create --arch=monolith
     --module=... --db=postgres`) agar bisa dipakai di CI/script.
   Dibutuhkan subcommand bertingkat (`create`, `add service`), persistent +
   local flags, hook `PreRunE`/`PostRunE`, dan shell completion.

2. **TUI / prompt library** — untuk pengalaman wizard yang nyaman (Select,
   Input, Confirm) sebagai *progressive enhancement* di atas mode flag.

3. **Template engine** — output builder adalah file teks/kode `.go`, `go.mod`,
   `docker-compose.yml`, dll. Template harus *modular* dan dibundel ke dalam
   binary (single-binary distribution, tanpa file template eksternal saat
   runtime).

4. **Manipulasi `go.mod`** — builder perlu menulis/menyunting `go.mod` project
   hasil generate secara terprogram (set `module`, `go` directive, `require`)
   dengan hasil deterministik.

### Batasan keras (mengikat keputusan)

- Hasil generate **wajib** lolos `go vet ./... && go build ./... && go test
  ./...` **tanpa edit manual**, dan `docker compose up` jalan untuk stack
  ber-DB.
- Project hasil generate **TIDAK BOLEH** meng-import package apa pun dari builder
  (**zero lock-in**). Semua dependency di ADR ini hanya melekat pada binary
  builder, **tidak** muncul di `go.mod` output.
- Reproducibility: stack builder harus memakai rilis yang dapat di-pin secara
  stabil sehingga build builder deterministik lintas waktu/mesin.

## Decision

Satu pilihan final per komponen:

| Komponen | Keputusan final | Versi terverifikasi (2026-06-06) |
|---|---|---|
| Command framework | **`spf13/cobra`** | v1.10.2 (publish 2025-12-03) |
| TUI / prompt | **`charmbracelet/huh`** (di atas `bubbletea` + `lipgloss`) | huh v1.0.0 (2026-02-23), bubbletea v1.3.10 (2025-09-17), lipgloss v1.1.0 (2025-03-12) |
| Template engine | **stdlib `text/template` + `embed.FS`** (+ custom `FuncMap`, + `go/format`) | stdlib (mengikuti rilis Go; `embed` stabil sejak Go 1.16) |
| Manipulasi `go.mod` | **`golang.org/x/mod/modfile`** | v0.36.0 (publish 2026-05-08) |

### 1. Command framework → `spf13/cobra`

Final: **`spf13/cobra` v1.10.2**. Menyediakan subcommand bertingkat, persistent +
local flags, hook `PreRunE`/`PostRunE`, dan shell completion otomatis — persis
yang dibutuhkan untuk model `create` / `add service` dan dual-mode
interaktif/non-interaktif. Ini de facto standard Go CLI (dipakai kubectl,
docker, gh, hugo), sehingga konvensi dan ekosistem contoh terbesar.

### 2. TUI / prompt → `charmbracelet/huh` (di atas `bubbletea` + `lipgloss`)

Final: **`charmbracelet/huh`** untuk wizard deklaratif (`Select` arsitektur,
`Input` module path, `Confirm` DB). `huh` dibangun di atas `bubbletea` (event
loop) + `lipgloss` (styling) — satu ekosistem (Charm), terpelihara aktif. Mode
non-interaktif tetap lewat flag cobra (progressive enhancement: kalau semua flag
lengkap, wizard di-skip).

**Pinning versi (penting — koreksi atas asumsi awal):** verifikasi pkg.go.dev
2026-06-06 menunjukkan jalur **v2** ekosistem Charm masih *pre-release / beta /
untagged*:

- `huh/v2` → masih `v2.0.0-...` (pseudo-version, belum ada tag stabil di
  pkg.go.dev).
- `lipgloss/v2` → `v2.0.0-beta.3`.
- `bubbletea/v2` → `v2.0.0-beta.6`.

Sebaliknya jalur **v1** sudah **stable**: `huh` v1.0.0 (rilis stabil pertama,
2026-02-23), `bubbletea` v1.3.10, `lipgloss` v1.1.0. Demi batasan keras
(*build/test lolos tanpa edit manual* + reproducibility build builder), default
yang dipilih adalah **rilis stable v1.x** dengan import path tanpa suffix `/v2`
(`github.com/charmbracelet/huh`, `.../bubbletea`, `.../lipgloss`). Migrasi ke v2
dijadwalkan setelah Charm men-tag rilis v2 stabil (lihat Consequences).

### 3. Template engine → stdlib `text/template` + `embed.FS`

Final: **stdlib `text/template`** dengan template dibundel via **`embed.FS`**
(single-binary, tanpa file eksternal runtime). Tambahan:
- **custom `FuncMap`** untuk helper casing (PascalCase / snake_case / kebab-case)
  dan join module-path;
- output file `.go` dilewatkan ke **`go/format`** (gofmt programatik) agar hasil
  selalu terformat & deterministik (mendukung syarat `go vet`/`go build` lolos).

Pendekatan ini menjaga builder bebas dependency template pihak ketiga.

### 4. Manipulasi `go.mod` → `golang.org/x/mod/modfile`

Final: **`golang.org/x/mod/modfile` v0.36.0**. Parse + edit `go.mod` via AST
(`AddRequire` / `DropRequire` / `AddGoStmt` / `SetRequire` / `Format`) —
deterministik dan **tidak butuh binary `go` di PATH saat authoring**. Modul resmi
tim Go.

Pembagian peran yang tegas:
- **Authoring** `go.mod` (menyusun konten saat generate) → `modfile`.
- **Verifikasi/finalisasi** pada project hasil generate → `go mod tidy` via
  `os/exec`. Langkah ini sah dan justru diperlukan untuk memenuhi batasan keras
  (`go vet`/`go build`/`go test` lolos tanpa edit manual). `exec` ditolak hanya
  untuk *authoring*, bukan untuk finalisasi.

## Consequences

### Positif

- **Konvensi & ekosistem terbesar (cobra)** — pola CLI dikenal luas, mudah
  di-maintain, contoh berlimpah, completion gratis.
- **UX wizard kelas atas (huh/Charm)** — prompt deklaratif, ringkas, konsisten.
- **Single-binary** — `embed.FS` membundel semua template; tidak ada file
  eksternal yang harus dikirim bersama binary.
- **Output deterministik** — `go/format` + `modfile` AST menghasilkan keluaran
  stabil byte-for-byte, kompatibel dengan syarat `go vet`/`build`/`test` lolos
  tanpa edit manual.
- **Zero lock-in terjaga** — cobra/huh/modfile hanya dependency *builder*; tidak
  pernah masuk `go.mod` project hasil generate (kecuali user memang memilih
  cobra sebagai bagian stack runtime project, dan itu hanya bila project tersebut
  sendiri sebuah CLI).
- **Authoring `go.mod` tanpa toolchain** — `modfile` tidak butuh `go` di PATH,
  mempermudah testing builder.

### Negatif

- **Kunci ke ekosistem Charm** — huh menarik `bubbletea` + `lipgloss`
  sekaligus; pergantian TUI lib di masa depan berarti menulis ulang layer
  wizard. Risiko ditekan karena ketiganya satu vendor dan terpelihara aktif.
- **Utang migrasi v1 → v2 (Charm)** — default kita di-pin ke v1.x stabil. Saat
  Charm men-tag v2 stabil, perlu migrasi terencana (import path berubah ke
  `/v2`, ada perubahan API). Mitigasi: isolasi seluruh pemakaian huh/bubbletea/
  lipgloss di satu package wizard internal agar permukaan migrasi sempit.
- **`text/template` minim fitur** — tanpa library helper (mis. sprig), semua
  helper casing/path harus ditulis & ditest sendiri di `FuncMap`. Ini disengaja
  (hindari dependency-bloat), tapi menambah sedikit kode pemeliharaan.
- **Dua mekanisme `go.mod`** — `modfile` (authoring) + `go mod tidy` (exec,
  finalisasi) berarti builder bergantung pada toolchain `go` di lingkungan
  *runtime generate*. Dapat diterima karena user gostarter memang dianggap punya
  Go terpasang.

## Alternatives Considered

### TUI / prompt

- **`AlecAivazis/survey` — DITOLAK KERAS.** Repo **archived oleh owner pada
  2024-04-19** dan kini read-only; README eksplisit menyatakan *"This project is
  no longer maintained"* dan mengarahkan pengguna ke `charmbracelet/bubbletea`.
  Tidak boleh dipakai sebagai default (melanggar aturan: dilarang merekomendasi
  library archived/deprecated). Dicatat hanya sebagai *hindari*.
- **`manifoldco/promptui` — ditolak.** Aktivitas lesu/tidak terpelihara secara
  konsisten; tidak menawarkan keunggulan dibanding ekosistem Charm yang aktif.
  Dicatat sebagai *hindari* untuk project baru.
- **`charmbracelet/huh/v2` (+ bubbletea/v2 + lipgloss/v2) — ditunda, bukan
  ditolak.** Per 2026-06-06 jalur v2 masih *pre-release/beta/untagged* di
  pkg.go.dev (huh `v2.0.0-...` pseudo, lipgloss `v2.0.0-beta.3`, bubbletea
  `v2.0.0-beta.6`). Ditangguhkan demi reproducibility & batasan build-lolos;
  diadopsi setelah tag v2 stabil tersedia.

### Command framework

- **`urfave/cli/v3` — ditolak (bukan karena maintenance).** Aktif & valid
  (v3.9.0, publish 2026-05-09). Ditolak semata karena adopsi/familiaritas dan
  ekosistem scaffolding lebih kecil dari cobra; bukan masalah pemeliharaan.
- **stdlib `flag` — ditolak.** Terlalu rendah untuk command-tree bertingkat
  (`create`, `add service`) dan tidak punya completion bawaan.

### Template engine

- **`Masterminds/sprig` — ditolak.** Over-engineering untuk segelintir helper
  casing/path yang bisa ditulis sendiri; menambah dependency tanpa nilai tambah
  berarti.
- **`a-h/templ` — ditolak.** Ditujukan untuk komponen HTML; tidak cocok untuk
  meng-generate file `.go`/config arbitrer. Menambah toolchain codegen yang tak
  diperlukan.

### Manipulasi `go.mod`

- **`os/exec` ke `go mod edit` / `go mod tidy` untuk AUTHORING — ditolak.**
  Bergantung pada environment (butuh `go` di PATH saat menyusun konten) dan
  parsing teks/JSON rawan terhadap perubahan format CLI. **Catatan:** `go mod
  tidy` via exec **tetap dipakai** sebagai langkah *verifikasi/finalisasi* pada
  project hasil generate — penolakan hanya berlaku untuk fase authoring.

## Referensi

Diverifikasi pada 2026-06-06 (pkg.go.dev = versi + tanggal publish; GitHub =
badge archived + aktivitas commit/release).

**Command framework**
- cobra repo (aktif, 44.1k stars, rilis terakhir v1.10.2 / 2025-12-04, tanpa badge archived): <https://github.com/spf13/cobra>
- cobra pkg.go.dev (v1.10.2, publish 2025-12-03): <https://pkg.go.dev/github.com/spf13/cobra>
- cobra docs: <https://cobra.dev/>
- urfave/cli v3 pkg.go.dev (aktif, v3.9.0, publish 2026-05-09): <https://pkg.go.dev/github.com/urfave/cli/v3>

**TUI / prompt**
- huh repo (aktif, ~639 commits): <https://github.com/charmbracelet/huh>. Catatan: tag jalur v2 (jika ada di GitHub) belum terbit sebagai modul stabil di pkg.go.dev per 2026-06-06, sehingga tidak dipakai — default tetap rilis stabil v1.0.0.
- huh pkg.go.dev (v1.0.0 stable / 2026-02-23; v0.8.0 / 2025-10-05): <https://pkg.go.dev/github.com/charmbracelet/huh>
- huh/v2 pkg.go.dev (masih pre-release `v2.0.0-...`): <https://pkg.go.dev/github.com/charmbracelet/huh/v2>
- bubbletea pkg.go.dev (v1.3.10 stable / 2025-09-17; v2 = `v2.0.0-beta.6`): <https://pkg.go.dev/github.com/charmbracelet/bubbletea>
- lipgloss pkg.go.dev (v1.1.0 stable / 2025-03-12; v2 = `v2.0.0-beta.3`): <https://pkg.go.dev/github.com/charmbracelet/lipgloss>
- bubbletea repo: <https://github.com/charmbracelet/bubbletea>
- lipgloss repo: <https://github.com/charmbracelet/lipgloss>
- **survey (DITOLAK — archived 2024-04-19, "no longer maintained"):** <https://github.com/AlecAivazis/survey>
- promptui (hindari): <https://github.com/manifoldco/promptui>

**Template engine (stdlib)**
- text/template: <https://pkg.go.dev/text/template>
- embed: <https://pkg.go.dev/embed>
- go/format: <https://pkg.go.dev/go/format>

**Manipulasi go.mod**
- x/mod/modfile pkg.go.dev (v0.36.0, publish 2026-05-08): <https://pkg.go.dev/golang.org/x/mod/modfile>
- x/mod modul: <https://pkg.go.dev/golang.org/x/mod>
- Edit go.mod from tools or scripts: <https://pkg.go.dev/cmd/go#hdr-Edit_go_mod_from_tools_or_scripts>
