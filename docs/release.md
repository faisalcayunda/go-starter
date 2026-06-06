# Proses Rilis gostarter

Dokumen ini menjelaskan cara me-rilis binary `gostarter` menggunakan
[GoReleaser](https://goreleaser.com) (schema v2). **Rilis dijalankan MANUAL oleh
maintainer** — tidak ada otomasi yang men-tag atau mem-publish rilis tanpa
tindakan eksplisit maintainer.

Konfigurasi rilis ada di [`.goreleaser.yaml`](../.goreleaser.yaml) (root repo).
Kebijakan penomoran versi dibahas terpisah di [versioning.md](./versioning.md).

---

## Ringkas

```bash
# 1. Pastikan working tree bersih & test hijau
make test

# 2. Validasi konfigurasi rilis (tanpa membangun apa pun)
goreleaser check

# 3. Commit semua perubahan rilis (CHANGELOG dirakit otomatis dari git log)
git add -A
git commit -m "chore: prepare release"

# 4. Buat tag semver (huruf 'v' di depan — wajib)
git tag vX.Y.Z

# 5. Jalankan rilis (butuh GITHUB_TOKEN)
export GITHUB_TOKEN="<personal-access-token-dengan-scope-repo>"
goreleaser release --clean
```

Hasil rilis dibuat sebagai **draft GitHub Release** (`release.draft: true`).
Maintainer meninjau draft, lalu mem-publish secara manual dari UI GitHub.

---

## Prasyarat

| Kebutuhan      | Keterangan                                                                 |
|----------------|---------------------------------------------------------------------------|
| Go             | ≥ 1.25 — floor `go.mod` (`go 1.25.0`); diuji pada 1.25.x & 1.26.x (matrix CI). |
| GoReleaser v2  | `goreleaser --version` → `v2.x`. Install: <https://goreleaser.com/install>. |
| `GITHUB_TOKEN` | Personal Access Token dengan scope `repo` (untuk upload asset + buat rilis). |
| Git tag bersih | Tag `vX.Y.Z` mengikuti [SemVer](https://semver.org).                       |

---

## Langkah Detail

### 1. Validasi konfigurasi

```bash
goreleaser check
```

Memvalidasi `.goreleaser.yaml` tanpa membangun. Harus lolos sebelum lanjut.

### 2. Uji snapshot (opsional, tanpa tag)

Bangun artefak lokal tanpa membuat tag atau mem-publish apa pun:

```bash
# build cepat satu target (untuk cek versi ter-inject)
goreleaser build --snapshot --clean --single-target
./dist/gostarter_*/gostarter --version

# build penuh semua platform (tanpa publish)
goreleaser release --snapshot --clean
```

Pada mode snapshot, versi berbentuk `X.Y.Z-snapshot-<commit>` — bukti bahwa
injeksi `-ldflags -X ...internal/cli.version` bekerja. Artefak ada di `dist/`
(sudah di-`.gitignore`).

### 3. Tag & rilis

```bash
git tag vX.Y.Z
export GITHUB_TOKEN="<token>"
goreleaser release --clean
```

GoReleaser akan:

1. Membangun binary untuk semua platform (lihat tabel di bawah).
2. Menyuntikkan versi: `-X github.com/faisalcayunda/gostarter/internal/cli.version={{ .Version }}`.
3. Mengarsipkan (`tar.gz` untuk linux/darwin, `zip` untuk windows).
4. Menghasilkan `checksums.txt` (SHA-256).
5. Merakit changelog dari git log (grup **Features** / **Bug fixes** / **Others**).
6. Membuat **draft** GitHub Release berisi semua asset.

### 4. Publish

Buka draft rilis di GitHub → tinjau changelog & asset → **Publish release**.

---

## Platform yang Dibangun

| OS      | Arch    | Format Arsip |
|---------|---------|--------------|
| linux   | amd64   | `tar.gz`     |
| linux   | arm64   | `tar.gz`     |
| darwin  | amd64   | `tar.gz`     |
| darwin  | arm64   | `tar.gz`     |
| windows | amd64   | `zip`        |
| windows | arm64   | `zip`        |

Semua build memakai `CGO_ENABLED=0` (binary statis, tanpa dependensi C).
Nama arsip: `gostarter_{version}_{os}_{arch}.{ext}`. Checksum gabungan:
`checksums.txt`.

---

## Instalasi (untuk pengguna akhir)

### Opsi A — `go install`

```bash
go install github.com/faisalcayunda/gostarter/cmd/gostarter@latest
gostarter --version
```

### Opsi B — Unduh binary rilis

1. Buka halaman [Releases](https://github.com/faisalcayunda/gostarter/releases)
   dan unduh arsip untuk OS/arch Anda, mis. `gostarter_X.Y.Z_linux_amd64.tar.gz`.
2. Verifikasi checksum:

   ```bash
   # unduh checksums.txt dari rilis yang sama, lalu:
   sha256sum -c checksums.txt --ignore-missing
   ```

3. Ekstrak & pasang ke `PATH`:

   ```bash
   tar -xzf gostarter_X.Y.Z_linux_amd64.tar.gz
   sudo mv gostarter /usr/local/bin/
   gostarter --version
   ```

   Untuk Windows: ekstrak `gostarter_X.Y.Z_windows_amd64.zip`, lalu pindahkan
   `gostarter.exe` ke direktori dalam `PATH`.

Verifikasi versi terinstal:

```bash
gostarter --version   # → gostarter version X.Y.Z
gostarter version     # ekuivalen
```

---

## Rollback

Rilis bersifat draft hingga dipublish. Jika perlu menarik rilis:

- **Belum dipublish:** hapus draft dari UI GitHub Releases.
- **Sudah dipublish:** tandai rilis sebagai *pre-release* atau hapus, lalu hapus
  tag: `git push --delete origin vX.Y.Z` (dan `git tag -d vX.Y.Z` lokal).
  `go install ...@latest` akan kembali memilih tag stabil tertinggi berikutnya.

---

## Catatan

- **Tidak ada langkah rilis yang otomatis di CI** untuk repo ini — `goreleaser
  release` dipicu manual oleh maintainer dengan `GITHUB_TOKEN` di mesin lokal
  (atau workflow yang sengaja dijalankan manual). CI biasa (`.github/workflows/ci.yml`)
  hanya menjalankan build + test + lint, bukan rilis.
- **Jangan** commit direktori `dist/` — sudah di-`.gitignore`.
- Untuk arti perubahan versi (patch/minor/major) dan dampaknya pada output
  template, lihat [versioning.md](./versioning.md).
