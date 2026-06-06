// Package fsutil mengabstraksi penulisan ke disk sehingga mode --dry-run dapat
// memakai writer yang sama tanpa menyentuh filesystem.
//
// Implementasi mengikuti ADR-002 §3.5 (kontrak tipe) & §8 (safety flow):
// RealWriter menulis ke disk; DryRunWriter hanya mencatat rencana (SPEC §5.4);
// EnsureEmptyDir menegakkan proteksi overwrite (US-06 Skenario 3).
package fsutil

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Writer adalah abstraksi target penulisan project hasil generate.
// RealWriter menulis ke disk; DryRunWriter hanya mencatat rencana (SPEC §5.4).
type Writer interface {
	// Mkdir membuat direktori pada path.
	Mkdir(path string) error
	// WriteFile menulis data ke path dengan permission perm.
	WriteFile(path string, data []byte, perm fs.FileMode) error
}

// RealWriter menulis project hasil generate ke filesystem sungguhan. Stateless,
// sehingga method memakai value-receiver (ADR-002 §3.5).
type RealWriter struct{}

// dirPerm adalah permission default direktori yang dibuat RealWriter.Mkdir.
const dirPerm fs.FileMode = 0o755

// Mkdir membuat direktori (beserta seluruh parent) pada path. Idempoten:
// memanggilnya pada direktori yang sudah ada bukan error (os.MkdirAll).
func (RealWriter) Mkdir(path string) error {
	if err := os.MkdirAll(path, dirPerm); err != nil {
		return fmt.Errorf("fsutil: mkdir %q: %w", path, err)
	}
	return nil
}

// WriteFile menulis data ke path dengan permission perm. Direktori induk dibuat
// lebih dulu (MkdirAll) agar pemanggil tidak wajib menerbitkan ModeMkdir untuk
// tiap level path — menjaga generator tetap sederhana.
func (RealWriter) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("fsutil: mkdir parent %q: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		return fmt.Errorf("fsutil: write %q: %w", path, err)
	}
	return nil
}

// DryRunWriter tidak menulis apa pun; ia mengakumulasi operasi untuk preview
// --dry-run (SPEC §5.4). Karena ia menyimpan state (Planned), method-nya wajib
// pointer-receiver dan pemanggil meneruskan &DryRunWriter{} sebagai Writer
// (ADR-002 §3.5).
type DryRunWriter struct {
	// Planned adalah daftar path yang AKAN ditulis/dibuat (urut sesuai plan),
	// dipakai untuk mencetak tree preview tanpa menyentuh disk.
	Planned []string
}

// Mkdir mencatat path direktori terencana tanpa menyentuh disk.
func (w *DryRunWriter) Mkdir(path string) error {
	w.Planned = append(w.Planned, path)
	return nil
}

// WriteFile mencatat path file terencana tanpa menulis apa pun. Argumen data &
// perm sengaja diabaikan: dry-run hanya merekam STRUKTUR rencana (SPEC §5.4),
// bukan isi.
func (w *DryRunWriter) WriteFile(path string, _ []byte, _ fs.FileMode) error {
	w.Planned = append(w.Planned, path)
	return nil
}

// Tree merender Planned menjadi pohon teks indentasi untuk preview --dry-run.
// Root adalah label baris teratas (lazimnya direktori target); setiap path di
// Planned ditampilkan relatif terhadap root bila berada di bawahnya. Output
// deterministik: path disalin lalu di-sort sehingga urutan rekam tidak
// memengaruhi tampilan.
//
// Memakai package `path` (POSIX/slash-only, SPEC §2.1), BUKAN path/filepath
// (OS-aware), agar konsisten dengan Planned yang diisi generator memakai slash
// POSIX (m-5). Ini menjaga preview --dry-run identik lintas OS.
func (w *DryRunWriter) Tree(root string) string {
	var b strings.Builder
	b.WriteString(root)
	b.WriteString("\n")

	// Salin agar tidak memodifikasi urutan rekam asli, lalu dedup + sort.
	paths := make([]string, 0, len(w.Planned))
	seen := make(map[string]struct{}, len(w.Planned))
	cleanRoot := path.Clean(filepath.ToSlash(root))
	prefix := cleanRoot + "/"
	for _, p := range w.Planned {
		sp := filepath.ToSlash(p)
		var rel string
		switch {
		case sp == cleanRoot:
			continue // root sendiri, sudah dicetak sebagai label
		case strings.HasPrefix(sp, prefix):
			rel = strings.TrimPrefix(sp, prefix)
		default:
			// Path di luar root → tampilkan apa adanya (slash-normalized).
			rel = sp
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		paths = append(paths, rel)
	}
	sort.Strings(paths)

	for _, rel := range paths {
		depth := strings.Count(rel, "/")
		b.WriteString(strings.Repeat("  ", depth+1))
		b.WriteString(path.Base(rel))
		b.WriteString("\n")
	}
	return b.String()
}

// JoinTarget menggabungkan root target dengan path relatif memakai slash POSIX
// (SPEC §2.1: separator POSIX di v1) DAN menegakkan containment: hasil join WAJIB
// tetap di bawah target. path.Join saja TIDAK cukup — path.Join("proj", "../../x")
// = "../x" lolos keluar target (B-1 path traversal). Karena itu, setelah join,
// dihitung filepath.Rel(target, joined); bila relatifnya naik ke atas (diawali
// ".." atau berupa absolut) → ditolak.
//
// SATU SUMBER KEBENARAN untuk penggabungan+containment path target (H-1): dipakai
// generator.execFileOp/writeGoMod (jalur create) MAUPUN add-service (jalur tulis
// inkremental) agar tidak ada string-concat path mentah di mana pun. rel kosong →
// kembalikan target apa adanya.
func JoinTarget(target, rel string) (string, error) {
	if rel == "" {
		return target, nil
	}
	joined := path.Join(target, rel)

	// Hitung path relatif joined terhadap target memakai semantik OS-aware
	// (filepath.Rel) agar konsisten dengan penulisan disk. Bila rel keluar dari
	// target, hasilnya diawali ".." → tolak.
	relCheck, err := filepath.Rel(filepath.Clean(target), filepath.FromSlash(joined))
	if err != nil {
		return "", fmt.Errorf("path %q keluar dari target (tidak dapat dihitung relatif): %w", rel, err)
	}
	relCheck = filepath.ToSlash(relCheck)
	if relCheck == ".." || strings.HasPrefix(relCheck, "../") || filepath.IsAbs(relCheck) {
		return "", fmt.Errorf("path %q keluar dari direktori target (path traversal ditolak)", rel)
	}
	return joined, nil
}

// EnsureEmptyDir memastikan path tujuan kosong atau belum ada (proteksi
// overwrite, SPEC §5.4 / US-06 Skenario 3; ADR-002 §8). Aturan:
//
//   - path belum ada                  → ok (nil): akan dibuat saat generate.
//   - path ada & berupa direktori kosong → ok (nil).
//   - path ada & berupa direktori berisi → error jelas (menyebut path).
//   - path ada tetapi BUKAN direktori  → error (file menghalangi target).
//
// Tidak ada penulisan yang dilakukan di sini; fungsi murni pemeriksaan.
func EnsureEmptyDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Belum ada → akan dibuat. Aman.
			return nil
		}
		return fmt.Errorf("fsutil: stat %q: %w", path, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("target %q sudah ada dan berupa file (bukan direktori); hapus file itu atau pilih tujuan lain dengan -o <dir>", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("fsutil: baca direktori %q: %w", path, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("direktori target %q tidak kosong (%d entri) — untuk mencegah overwrite, kosongkan direktori itu atau pilih tujuan lain dengan -o <dir>", path, len(entries))
	}

	// Ada & kosong → aman.
	return nil
}
