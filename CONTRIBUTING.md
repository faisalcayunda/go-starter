# Kontribusi ke gostarter

Terima kasih sudah berkontribusi! gostarter adalah CLI generator project Go untuk
tiga paradigma arsitektur (monolith, modular-monolith, microservice) dengan
**zero lock-in** dan kontrak **build hijau** yang ditegakkan.

> **Mau menambah modul template?** Baca panduan khusus
> [`docs/adding-modules.md`](docs/adding-modules.md) — anatomi modul, skema
> `module.yaml`, anchor `region:<name>`, grammar `when`, walkthrough, dan
> checklist PR. Halaman ini mencakup alur kerja umum.

---

## 1. Prasyarat

- **Go ≥ 1.25** — floor `go.mod` (`go 1.25.0`); dikembangkan & diuji pada 1.25.x **dan** 1.26.x (matrix CI).
- **golangci-lint v2** — gerbang lint (config `.golangci.yml`).
- Opsional: **goreleaser v2** (rilis), **buf** (proto microservice; dibutuhkan
  saat menjalankan e2e microservice yang men-generate `gen/go/`).

Pasang tool via toolchain Go-mu (mis. `asdf`) dan pastikan di `PATH`.

---

## 2. Alur kerja

1. **Fork & branch.** Fork repo, buat branch dari `main`
   (mis. `feat/db-sqlite`, `fix/merge-order`).

2. **Bangun & uji lokal.**
   ```sh
   make build          # kompilasi seluruh paket builder
   make test           # go test ./... -count=1 — SELURUH paket, termasuk golden (mode banding)
   make lint           # golangci-lint (config .golangci.yml)
   make golden-verify  # HANYA paket golden — compare-only eksplisit (mode banding)
   make e2e            # matrix-4a + microservice-e2e + smoke
   ```
   Atau gerbang penuh sekaligus (cerminan CI):
   ```sh
   make ci             # = lint + test + golden-verify + e2e
   ```

3. **Pasang pre-commit hook** (sangat disarankan):
   ```sh
   make install-hooks  # symlink scripts/pre-commit → .git/hooks/pre-commit
   ```
   Hook membatalkan commit bila gagal: **gofmt → go vet → golangci-lint →
   go test** (cepat, tanpa e2e). Alternatif berbasis framework: `pre-commit
   install` (memakai `.pre-commit-config.yaml`). Lewati sementara (tidak
   disarankan): `git commit --no-verify`.

4. **Commit (conventional commits).** Format pesan:
   ```
   <type>(<scope>): <ringkasan singkat>
   ```
   `type` lazim: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`.
   `scope` opsional (mis. `module`, `resolver`, `golden`). Contoh:
   ```
   feat(module): tambah modul db-sqlite (pure-Go, modernc.org/sqlite)
   fix(resolver): perbaiki tie-break order pada anchor services
   docs: lengkapi panduan adding-modules
   ```
   gostarter di-versioning **semver** dan menurunkan changelog dari git
   (`.goreleaser.yaml`) — pesan yang rapi memudahkan rilis.

5. **Jalankan script verifikasi end-to-end** sebelum push:
   ```sh
   bash scripts/matrix-4a.sh          # 6 kombinasi wajib (monolith/modular)
   bash scripts/microservice-e2e.sh   # 2 service saling memanggil via gRPC
   bash scripts/smoke.sh              # smoke cepat
   ```
   (semuanya terbungkus `make e2e`). Untuk kombinasi ber-DB / non-stdlib, e2e
   menjalankan `go mod tidy` (butuh jaringan). Docker build di-SKIP bila daemon
   tidak ada — bukan kegagalan.

6. **Buka PR ke `main`.** Sertakan ringkasan perubahan, kombinasi/dimensi yang
   terdampak, dan — bila output berubah — **diff golden** yang sudah ditinjau.

> **Catatan golden — lokal vs CI.** Snapshot golden diperiksa di **dua tempat**, dan
> perilakunya **berbeda** antara `make` lokal dan CI:
>
> - **Lokal:** `make test` = `go test ./... -count=1` → menjalankan **seluruh** paket,
>   **termasuk** `internal/golden` dalam **mode banding** (compare). Jadi `make test`
>   lokal **sudah** memeriksa drift golden. `make golden-verify` adalah target
>   **compare-only eksplisit** yang menguji **hanya** paket golden
>   (`go test ./internal/golden/... -count=1`) — dipakai `make ci` agar gerbang golden
>   tetap eksplisit & terpisah.
> - **CI** (`.github/workflows/ci.yml`): job `test` **mengecualikan** golden
>   (`go test $(go list ./... | grep -v internal/golden)`) supaya tidak dijalankan dua
>   kali; golden dijalankan terpisah di job `golden` (`go test ./internal/golden/`).
>   Inilah satu-satunya tempat golden berjalan di CI.
>
> Bila perubahanmu mengubah output: jalankan `make golden-update`, **tinjau** diff
> `internal/golden/testdata/golden/`, lalu commit snapshot baru bersama perubahan.

---

## 3. Definition of Done (sebelum PR)

- [ ] `make ci` hijau lokal (lint + test + golden-verify + e2e).
- [ ] Pre-commit hook terpasang & lulus (gofmt/vet/lint/test).
- [ ] Output project hasil generate **zero lock-in**: lolos `go vet ./... &&
      go build ./... && go test ./...` **tanpa edit manual**, tidak meng-import
      builder, tidak menyebut "gostarter" (kecuali marker netral `add service`).
- [ ] Bila menambah/mengubah modul: checklist di
      [`docs/adding-modules.md`](docs/adding-modules.md) §9 terpenuhi
      (manifest valid, guard anti-drift selaras, golden ditinjau).
- [ ] Diff golden (bila ada) ditinjau dan dijelaskan di PR.
- [ ] Conventional commit.

---

## 4. Struktur repo (ringkas)

- `cmd/gostarter/` — entrypoint CLI.
- `internal/` — paket builder: `answers`, `module` (registry/validateCatalog),
  `plan`, `resolver` (gating + `when` + guard anti-drift), `generator`
  (renderer + merge-assembler), `golden` (snapshot harness), `cli`, dll.
- `templates/modules/<name>/` — modul template (`module.yaml` + `.tmpl`),
  di-embed via `templates.FS`.
- `docs/` — `SPEC.md`, `adr/` (ADR-001/002/003), `research/`, panduan
  (`adding-modules.md`, `release.md`, `versioning.md`).
- `scripts/` — `pre-commit`, `matrix-4a.sh`, `microservice-e2e.sh`, `smoke.sh`.

---

## 5. Sumber kebenaran

Perubahan harus konsisten dengan:

- [`docs/SPEC.md`](docs/SPEC.md) — spesifikasi fungsional & invarian.
- [ADR-001](docs/adr/ADR-001-builder-stack.md) — stack builder.
- [ADR-002](docs/adr/ADR-002-internal-architecture.md) — arsitektur internal,
  tipe kanonik, FuncMap, grammar `when`.
- [ADR-003](docs/adr/ADR-003-template-system.md) — sistem template, skema
  `module.yaml`, anchor/merge, validasi, versioning.

Bila usulan bertentangan dengan ADR/SPEC, diskusikan di issue/PR lebih dulu —
perubahan kontrak butuh update ADR yang sepadan.
