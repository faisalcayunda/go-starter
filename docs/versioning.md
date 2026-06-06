# Kebijakan Versioning — gostarter

> **Status:** Stabil (Fase 6)
> **Berlaku untuk:** rilis biner CLI `gostarter` dan template/modul yang dirender-nya.
> **Sumber kebenaran terkait:** [`docs/SPEC.md`](./SPEC.md), [`docs/adr/ADR-003-template-system.md`](./adr/ADR-003-template-system.md), [`docs/release.md`](./release.md), [`docs/adding-modules.md`](./adding-modules.md).

Dokumen ini mendefinisikan **kapan menaikkan versi gostarter** dan bagaimana perubahan
pada template/modul dipetakan ke `PATCH` / `MINOR` / `MAJOR`. Tujuannya: kontrak yang
jelas bagi pengguna yang me-regenerate project dan bagi kontributor yang menambah modul.

---

## 1. Semantic Versioning untuk Tool

gostarter memakai [Semantic Versioning 2.0.0](https://semver.org/lang/id/) —
`MAJOR.MINOR.PATCH` (mis. `v1.4.2`). Versi tunggal ini mencakup **dua hal sekaligus**:

1. **Perilaku CLI** — flag, prompt interaktif (`huh`), resolusi constraint, format `--config`.
2. **Output yang dirender** — struktur file, isi template, dependency yang di-pin di `go.mod` hasil generate.

Karena keluaran tool *adalah* produknya, **kompatibilitas dinilai dari sudut pandang pengguna
yang menjalankan ulang gostarter dengan input yang sama**. Pertanyaan kunci untuk setiap perubahan:

> *"Jika pengguna me-regenerate kombinasi yang sudah ada dengan versi baru, apakah hasilnya
> berubah dengan cara yang akan mengejutkan atau merusak mereka?"*

Jawabannya menentukan bump (lihat §2).

Versi di-inject saat build (`-ldflags -X .../internal/cli.version`) dan dapat diperiksa via:

```bash
gostarter version      # atau: gostarter --version
```

Detail proses rilis ada di [`docs/release.md`](./release.md).

---

## 2. Pemetaan Jenis Perubahan → Bump

### PATCH — `vX.Y.Z` → `vX.Y.(Z+1)`

Perbaikan yang **tidak mengubah output bermakna** untuk kombinasi mana pun yang sudah ada.

- Perbaikan bug pada engine builder yang **tidak** mengubah byte hasil render kombinasi valid (mis. pesan error lebih jelas, perbaikan exit code, penanganan path).
- Perbaikan template yang murni kosmetik dan **tidak** mengubah golden snapshot (mis. typo komentar yang ternyata tidak ter-render, perbaikan formatting yang sudah dinormalisasi `gofmt`).
- Patch dependency yang di-pin di output (mis. `chi v5.1.0` → `v5.1.1`) **selama** tidak ada perubahan API yang merembet ke template.
- Perbaikan dokumentasi, CI internal builder, atau test — tanpa dampak ke output.

> Aturan praktis: **PATCH tidak boleh mengubah golden snapshot kombinasi yang ada.**
> Jika `make golden-verify` gagal, perubahan itu **minimal MINOR** (lihat §3).

### MINOR — `vX.Y.Z` → `vX.(Y+1).0`

Penambahan **backward-compatible**: kemampuan baru muncul, kombinasi lama tetap valid dan
menghasilkan output yang sama (atau perubahan output yang disengaja dan tidak merusak alur lama).

- **Opsi baru** pada flag yang ada: nilai enum baru untuk `--http`, `--db`, `--arch`, `--kind`, anggota `--addons` baru, mode `--comm`/gateway baru. Default lama tidak berubah.
- **Modul/template baru** yang hanya aktif bila pengguna memilihnya (mis. menambah `--http=gin`, `--db=sqlite`). Pengguna yang tidak memilihnya tidak terpengaruh.
- **Flag baru** yang punya default no-op (perilaku lama dipertahankan bila flag tidak dipakai).
- Minor bump dependency yang di-pin (fitur baru, backward-compatible).
- Perubahan output **disengaja** pada kombinasi yang ada (mis. memperbaiki bug template yang menghasilkan file salah) — selama tidak memaksa pengguna mengubah cara pakai. Ini mengubah golden snapshot dan harus dicatat di changelog (§3).

### MAJOR — `vX.Y.Z` → `v(X+1).0.0`

Perubahan **breaking** bagi pengguna yang me-regenerate atau bagi otomasi yang memanggil CLI.

- **Perubahan struktur output** untuk kombinasi yang sudah ada: rename/pindah folder atau file, mengubah layout default, mengubah package path yang dirender.
- **Menghapus atau merename opsi/flag** (mis. menghapus nilai `--db=mongo`, mengganti nama `--addons` lama).
- **Mengubah default**: mengganti `--arch` default dari `monolith`, `--http` default dari `net/http`, atau default add-ons mana pun — karena pengguna yang mengandalkan default lama akan mendapat output berbeda secara diam-diam.
- **Major bump dependency** yang di-pin yang memaksa perubahan kode di project hasil generate (mis. naik major framework HTTP dengan API breaking).
- Mengubah format/skema `--config` preset sehingga file preset lama gagal dibaca.

> **Catatan default:** mengubah *default* selalu dinilai breaking meskipun semua opsi lama masih
> tersedia, karena pengguna yang tidak menyebut flag secara eksplisit akan mendapat output yang
> berubah. Default adalah bagian dari kontrak.

---

## 3. Hubungan dengan Golden Snapshot

Golden snapshot (`internal/golden/testdata/`) adalah **kontrak output byte-identical**: untuk setiap
kombinasi yang diuji, output render harus persis sama byte-per-byte dengan snapshot. Snapshot inilah
detektor utama perubahan output, sehingga ia terikat langsung ke kebijakan bump:

| Sinyal golden | Arti | Bump minimal |
|---|---|---|
| `make golden-verify` **hijau** tanpa perubahan testdata | output tidak berubah | PATCH boleh |
| Snapshot kombinasi **yang sudah ada** berubah (disengaja) | output kombinasi lama bergeser | **MINOR** (atau MAJOR bila strukturnya breaking — §2) |
| Snapshot **baru** ditambah untuk opsi/modul baru, snapshot lama utuh | fitur ditambah, lama aman | **MINOR** |
| Snapshot kombinasi lama **dihapus/restruktur** (rename folder, hapus file) | kontrak lama dilanggar | **MAJOR** |

**Prosedur saat output sengaja diubah:**

1. Terapkan perubahan template/engine.
2. Jalankan `make golden-update` (men-set `UPDATE_GOLDEN=1`) untuk meregenerasi snapshot.
3. **Review diff snapshot** — pastikan hanya kombinasi yang dimaksud yang berubah; perubahan tak terduga = regresi, hentikan.
4. Verifikasi ulang dengan `make golden-verify` (dan gerbang penuh `make ci`).
5. **Catat di changelog** dengan klasifikasi bump yang sesuai (§2). Setiap perubahan golden yang menyentuh kombinasi yang ada **wajib** muncul di changelog rilis.

> Golden snapshot yang berubah **tanpa entri changelog** dianggap regresi tak disengaja — bukan rilis.

---

## 4. Versi Dependency pada Output Hasil Generate

Project yang dihasilkan gostarter mem-**pin** versi dependency di `go.mod` (mis. router, driver DB,
library validasi). Kebijakan pin:

- Versi yang di-pin adalah **bagian dari output** dan tunduk pada §2: patch dep = PATCH, minor dep = MINOR, major dep yang merembet ke kode template = MAJOR.
- **Update keamanan** pada dependency yang di-pin diprioritaskan dan dirilis secepatnya; tetap mengikuti aturan bump berdasarkan dampak ke output (umumnya PATCH/MINOR).
- Pengguna yang sudah memegang project hasil generate **bebas** menaikkan dependency sendiri (`go get -u`); gostarter tidak mengelola dependency project yang sudah ter-generate.
- Daftar versi yang di-pin per opsi mengikuti `docs/SPEC.md` dan `docs/research/03-libraries.md` sebagai sumber kebenaran.

---

## 5. Non-Goal: Upgrade Project Lama

gostarter adalah **generator sekali-jalan (scaffolder)**, bukan framework runtime atau migrator.
Secara eksplisit **bukan tujuan**:

- Menyediakan jalur "upgrade in-place" untuk project yang sudah ter-generate ke versi gostarter baru.
- Mem-patch atau memigrasi kode yang sudah ditulis pengguna di atas scaffold.
- Menjaga kompatibilitas mundur dengan project yang dibuat versi gostarter lama saat pengguna menjalankan `gostarter add ...`.

**Model yang benar:** untuk mendapat output versi baru, pengguna **me-regenerate** ke direktori bersih
lalu mengadopsi perubahan secara manual (atau lewat diff/VCS) — **bukan** menjalankan "migrate".
Sekali sebuah project di-scaffold, ia menjadi milik pengguna; gostarter tidak punya kepemilikan
lanjutan atasnya. Semver gostarter mengikat **output saat generate**, bukan siklus hidup project
setelahnya.

---

## 6. Tabel Ringkas: Contoh Perubahan → Bump

| Contoh perubahan | Bump | Alasan |
|---|---|---|
| Perbaiki pesan error CLI, exit code, atau path handling tanpa ubah output | PATCH | Tidak menyentuh golden kombinasi mana pun |
| Patch dep di-pin (`chi v5.1.0` → `v5.1.1`), API sama | PATCH | Output bergeser minor, tanpa breaking; sering tetap PATCH bila golden tak berubah signifikan |
| Tambah `--http=gin` (opsi baru, default tetap `net/http`) | MINOR | Kombinasi lama utuh, snapshot baru ditambah |
| Tambah `--db=sqlite` + modul migrasinya | MINOR | Backward-compatible, opt-in |
| Tambah add-on baru di `--addons` | MINOR | Tidak aktif kecuali dipilih |
| Perbaiki bug template yang merender file salah pada kombinasi yang ada | MINOR | Output kombinasi lama berubah disengaja, non-breaking; golden di-update + changelog |
| Minor bump dep di-pin (fitur baru, kompatibel) | MINOR | Output berubah tapi tidak memaksa perubahan kode pengguna |
| Ubah default `--arch` dari `monolith` ke lain | MAJOR | Pengguna tanpa flag eksplisit dapat output berbeda |
| Ubah layout default monolith (pindah/rename folder) | MAJOR | Struktur output kombinasi lama berubah |
| Hapus opsi `--db=mongo` atau rename flag | MAJOR | Input lama jadi invalid |
| Major bump framework HTTP yang memaksa edit kode hasil generate | MAJOR | Breaking bagi project yang di-regenerate |
| Ubah skema file `--config` preset hingga preset lama gagal dibaca | MAJOR | Kontrak input rusak |

---

## 7. Ringkasan Aturan

1. Versi tunggal `MAJOR.MINOR.PATCH` mencakup perilaku CLI **dan** output yang dirender.
2. Kompatibilitas dinilai dari **pengguna yang me-regenerate kombinasi yang ada**.
3. **PATCH tidak boleh mengubah golden** kombinasi yang ada; jika `golden-verify` gagal → minimal MINOR.
4. **Opsi/modul/flag baru yang opt-in** = MINOR; **mengubah default, struktur output, atau menghapus opsi** = MAJOR.
5. Perubahan golden yang disengaja: `make golden-update` → review diff → `make golden-verify`/`make ci` → **catat di changelog**.
6. Versi dependency yang di-pin adalah bagian dari output dan tunduk pada aturan bump yang sama.
7. **Upgrade project lama bukan tujuan** — pengguna me-regenerate, bukan migrate.
