package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnsureEmptyDir_EmptyDir: direktori ADA & kosong → ok (nil).
func TestEnsureEmptyDir_EmptyDir(t *testing.T) {
	dir := t.TempDir() // t.TempDir() selalu mengembalikan direktori kosong.
	if err := EnsureEmptyDir(dir); err != nil {
		t.Fatalf("EnsureEmptyDir pada dir kosong: ingin nil, dapat %v", err)
	}
}

// TestEnsureEmptyDir_NonEmptyDir: direktori ADA & berisi → error.
func TestEnsureEmptyDir_NonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	// Isi direktori dengan satu file agar tidak kosong.
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: gagal menulis file: %v", err)
	}
	err := EnsureEmptyDir(dir)
	if err == nil {
		t.Fatalf("EnsureEmptyDir pada dir berisi: ingin error, dapat nil")
	}
	// Pesan error harus menyebut path agar pengguna paham (ADR-002 §8).
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("pesan error tidak menyebut path target: %q", err.Error())
	}
	// T5.5: pesan harus ACTIONABLE — menyebut overwrite & cara perbaikan (-o <dir>).
	if !strings.Contains(err.Error(), "overwrite") {
		t.Errorf("pesan harus menyebut 'overwrite' (alasan proteksi): %q", err.Error())
	}
	if !strings.Contains(err.Error(), "-o") {
		t.Errorf("pesan harus menyebut '-o <dir>' (cara perbaikan): %q", err.Error())
	}
}

// TestEnsureEmptyDir_MissingDir: direktori TAK ADA → ok (akan dibuat).
func TestEnsureEmptyDir_MissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "belum-ada")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("setup: path %q seharusnya belum ada", dir)
	}
	if err := EnsureEmptyDir(dir); err != nil {
		t.Fatalf("EnsureEmptyDir pada dir tak ada: ingin nil, dapat %v", err)
	}
}

// TestEnsureEmptyDir_TargetIsFile: target ADA tetapi berupa file → error.
func TestEnsureEmptyDir_TargetIsFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: gagal menulis file: %v", err)
	}
	err := EnsureEmptyDir(file)
	if err == nil {
		t.Fatalf("EnsureEmptyDir pada file: ingin error, dapat nil")
	}
	// T5.5: pesan menjelaskan target adalah FILE (bukan direktori) + cara perbaikan.
	if !strings.Contains(err.Error(), "file") || !strings.Contains(err.Error(), "-o") {
		t.Errorf("pesan harus menjelaskan target file & cara perbaikan (-o): %q", err.Error())
	}
}

// TestJoinTarget_ContainmentRejectsTraversal memverifikasi JoinTarget menolak rel
// yang KELUAR dari target (B-1 path traversal) dan menerima rel valid di bawahnya.
func TestJoinTarget_ContainmentRejectsTraversal(t *testing.T) {
	target := "/proj/app"

	bad := []string{"../escape", "../../etc/passwd", "a/../../b"}
	for _, rel := range bad {
		if _, err := JoinTarget(target, rel); err == nil {
			t.Errorf("JoinTarget(%q,%q) = nil error, mau ditolak (B-1 traversal)", target, rel)
		}
	}

	good := map[string]string{
		"cmd/app/main.go": filepath.FromSlash("/proj/app/cmd/app/main.go"),
		"go.mod":          filepath.FromSlash("/proj/app/go.mod"),
		"a/../b":          filepath.FromSlash("/proj/app/b"), // naik lalu turun, tetap di bawah target
	}
	for rel, want := range good {
		got, err := JoinTarget(target, rel)
		if err != nil {
			t.Errorf("JoinTarget(%q,%q) error tak terduga: %v", target, rel, err)
			continue
		}
		if filepath.FromSlash(got) != want {
			t.Errorf("JoinTarget(%q,%q) = %q, mau %q", target, rel, got, want)
		}
	}

	// rel kosong → kembalikan target apa adanya.
	if got, err := JoinTarget(target, ""); err != nil || got != target {
		t.Errorf("JoinTarget(target,\"\") = %q,%v; mau target,nil", got, err)
	}
}

// TestDryRunWriter_NoDiskWrites: DryRunWriter merekam path tanpa menulis file.
// Memverifikasi (1) Planned terisi sesuai urutan, (2) tidak ada file/dir yang
// benar-benar terbuat di disk (SPEC §5.4 / ADR-002 §8).
func TestDryRunWriter_NoDiskWrites(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, "internal", "handler")
	filePath := filepath.Join(root, "internal", "handler", "handler.go")

	var w Writer = &DryRunWriter{}

	if err := w.Mkdir(dirPath); err != nil {
		t.Fatalf("DryRunWriter.Mkdir: dapat error %v", err)
	}
	if err := w.WriteFile(filePath, []byte("package handler\n"), 0o644); err != nil {
		t.Fatalf("DryRunWriter.WriteFile: dapat error %v", err)
	}

	// (1) Planned harus memuat kedua path, urut sesuai pemanggilan.
	dw := w.(*DryRunWriter)
	want := []string{dirPath, filePath}
	if len(dw.Planned) != len(want) {
		t.Fatalf("Planned: ingin %d entri, dapat %d (%v)", len(want), len(dw.Planned), dw.Planned)
	}
	for i, p := range want {
		if dw.Planned[i] != p {
			t.Errorf("Planned[%d]: ingin %q, dapat %q", i, p, dw.Planned[i])
		}
	}

	// (2) Tidak boleh ada file/dir yang benar-benar terbuat di disk.
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Errorf("DryRunWriter membuat direktori di disk: %q (err=%v)", dirPath, err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("DryRunWriter membuat file di disk: %q (err=%v)", filePath, err)
	}
}

// TestDryRunWriter_Tree: preview tree deterministik & memuat base name file.
func TestDryRunWriter_Tree(t *testing.T) {
	root := "/proj/app"
	w := &DryRunWriter{}
	// Sengaja rekam tidak terurut untuk menguji sort determinisme.
	_ = w.WriteFile(filepath.FromSlash("/proj/app/cmd/app/main.go"), nil, 0o644)
	_ = w.Mkdir(filepath.FromSlash("/proj/app/cmd"))
	_ = w.WriteFile(filepath.FromSlash("/proj/app/go.mod"), nil, 0o644)

	out := w.Tree(root)
	if !strings.HasPrefix(out, root+"\n") {
		t.Errorf("Tree tidak diawali root: %q", out)
	}
	for _, base := range []string{"main.go", "go.mod", "cmd"} {
		if !strings.Contains(out, base) {
			t.Errorf("Tree tidak memuat %q:\n%s", base, out)
		}
	}
}

// TestRealWriter_WriteAndMkdir: RealWriter benar-benar menulis ke disk, dan
// membuat direktori induk file secara otomatis.
func TestRealWriter_WriteAndMkdir(t *testing.T) {
	root := t.TempDir()
	var w Writer = RealWriter{}

	subdir := filepath.Join(root, "a", "b")
	if err := w.Mkdir(subdir); err != nil {
		t.Fatalf("RealWriter.Mkdir: %v", err)
	}
	if info, err := os.Stat(subdir); err != nil || !info.IsDir() {
		t.Fatalf("Mkdir tidak membuat direktori %q (err=%v)", subdir, err)
	}

	// WriteFile ke path yang direktori induknya belum ada → harus dibuat otomatis.
	file := filepath.Join(root, "c", "d", "file.txt")
	content := []byte("halo")
	if err := w.WriteFile(file, content, 0o644); err != nil {
		t.Fatalf("RealWriter.WriteFile: %v", err)
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("baca file hasil tulis: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("isi file: ingin %q, dapat %q", content, got)
	}
}

// TestRealWriter_MkdirIdempotent: Mkdir pada direktori yang sudah ada bukan error.
func TestRealWriter_MkdirIdempotent(t *testing.T) {
	dir := t.TempDir()
	w := RealWriter{}
	if err := w.Mkdir(dir); err != nil {
		t.Fatalf("Mkdir idempoten gagal: %v", err)
	}
}
