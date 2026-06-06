package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateNewServiceName memverifikasi T5.5 (add-service): nama service baru
// divalidasi via answers.Validate (sumber tunggal) — nama valid lolos; nama
// invalid/reserved/format-salah ditolak dengan pesan deskriptif (menyebut nama +
// aturan), tanpa prefiks ganda.
func TestValidateNewServiceName(t *testing.T) {
	t.Run("nama valid lolos", func(t *testing.T) {
		for _, name := range []string{"order", "user-svc", "svc2"} {
			if err := validateNewServiceName(name); err != nil {
				t.Errorf("validateNewServiceName(%q) = %v, mau nil", name, err)
			}
		}
	})

	cases := []struct {
		name     string
		svc      string
		wantSubs []string
	}{
		{"huruf besar", "OrderSvc", []string{"OrderSvc", "huruf kecil"}},
		{"reserved gateway", "gateway", []string{"gateway", "reserved"}},
		{"kosong", "", []string{"kosong"}},
		{"karakter ilegal", "svc_a", []string{"svc_a"}},
		{"diawali angka", "2svc", []string{"2svc"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNewServiceName(tc.svc)
			if err == nil {
				t.Fatalf("validateNewServiceName(%q) harus error", tc.svc)
			}
			for _, sub := range tc.wantSubs {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("pesan harus memuat %q, dapat: %v", sub, err)
				}
			}
			// Tak boleh ada prefiks ganda "tidak valid: ... tidak valid".
			if strings.Count(err.Error(), "tidak valid") > 1 {
				t.Errorf("pesan tak boleh punya prefiks 'tidak valid' ganda: %v", err)
			}
		})
	}
}

// TestDetectMicroProject_NotMicroservice memverifikasi T5.5: direktori yang BUKAN
// project microservice gostarter ditolak RAMAH — pesan menyebut marker yang hilang
// & menjelaskan `add service` hanya untuk monorepo microservice.
func TestDetectMicroProject_NotMicroservice(t *testing.T) {
	dir := t.TempDir() // kosong → tak ada buf.yaml/proto/services/go.mod.
	_, err := detectMicroProject(dir)
	if err == nil {
		t.Fatalf("direktori non-microservice harus ditolak")
	}
	if !strings.Contains(err.Error(), "microservice") {
		t.Errorf("pesan harus menyebut 'microservice': %v", err)
	}
	// Marker pertama yang dicek = buf.yaml.
	if !strings.Contains(err.Error(), "buf.yaml") {
		t.Errorf("pesan harus menyebut marker yang hilang (buf.yaml): %v", err)
	}
}

// TestDetectMicroProject_PartialMarkers memverifikasi deteksi menolak project yang
// hanya punya SEBAGIAN marker (mis. buf.yaml ada tapi services/ tidak) — pesan
// menyebut marker SPESIFIK yang hilang.
func TestDetectMicroProject_PartialMarkers(t *testing.T) {
	dir := t.TempDir()
	// Sediakan buf.yaml + proto/ + go.mod, TANPA services/.
	if err := os.WriteFile(filepath.Join(dir, "buf.yaml"), []byte("version: v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "proto"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := detectMicroProject(dir)
	if err == nil {
		t.Fatalf("project tanpa services/ harus ditolak")
	}
	if !strings.Contains(err.Error(), "services") {
		t.Errorf("pesan harus menyebut marker 'services' yang hilang: %v", err)
	}
}

// TestServiceExists memverifikasi deteksi service existing (dasar tolak duplikat):
// services/<name> yang berupa direktori → true; tak ada → false.
func TestServiceExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "services", "order"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !serviceExists(dir, "order") {
		t.Errorf("serviceExists harus true untuk services/order yang ada")
	}
	if serviceExists(dir, "user") {
		t.Errorf("serviceExists harus false untuk service yang tak ada")
	}
}

// TestReadGoMod memverifikasi parsing go.mod: module path & go version terbaca;
// go.mod tanpa deklarasi module → error ramah.
func TestReadGoMod(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "go.mod")
		if err := os.WriteFile(p, []byte("module github.com/acme/platform\n\ngo 1.25\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		mod, ver, err := readGoMod(p)
		if err != nil {
			t.Fatalf("readGoMod gagal: %v", err)
		}
		if mod != "github.com/acme/platform" {
			t.Errorf("module = %q, mau github.com/acme/platform", mod)
		}
		if ver != "1.25" {
			t.Errorf("go version = %q, mau 1.25", ver)
		}
	})
	t.Run("tanpa deklarasi module", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "go.mod")
		if err := os.WriteFile(p, []byte("go 1.25\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readGoMod(p); err == nil {
			t.Errorf("go.mod tanpa module harus error")
		}
	})
	t.Run("file tak ada", func(t *testing.T) {
		if _, _, err := readGoMod(filepath.Join(t.TempDir(), "tidak-ada.mod")); err == nil {
			t.Errorf("go.mod tak ada harus error")
		}
	})
}

// L-2: TestResolveCI_DefaultProvider DIHAPUS — duplikat penuh dari kasus
// {"addon on, ci flag kosong → default github", true, "", CIGitHubActions} di
// TestResolveCI_Gating (create_test.go), yang sudah menguji resolveCI(true, "")
// == github-actions secara table-driven bersama jalur gating lainnya. Tidak ada
// pertanggungjawaban unik yang hilang dengan menghapusnya.
