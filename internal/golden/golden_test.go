// Package golden berisi harness golden-file (snapshot) untuk gostarter (Fase 5,
// T5.2). Untuk SETIAP kombinasi kunci, harness:
//
//  1. me-RESOLVE answers.Answers → plan.GeneratePlan via resolver.New (katalog
//     nyata dari templates.FS),
//  2. me-RENDER plan ke direktori temporer (t.TempDir) via generator.New +
//     RealWriter — render KONTEN penuh (bukan sekadar tree dry-run), lalu
//  3. mengumpulkan map[path]->content dari pohon hasil render dan
//     MEMBANDINGKANNYA byte-per-byte dengan snapshot yang DI-COMMIT di
//     testdata/golden/<combo>/.
//
// Render memakai generator kanonik yang SAMA dengan jalur CLI create (renderer +
// merge-assembler atas templates.FS) sehingga golden menangkap output produksi
// yang sebenarnya. go build TIDAK dijalankan di golden test — golden = perbandingan
// konten hasil RENDER (cepat, deterministik), bukan verifikasi runtime.
//
// Regenerasi snapshot: jalankan dengan flag -update ATAU set UPDATE_GOLDEN=1.
// Saat aktif, harness MENULIS ulang testdata/golden/<combo>/ alih-alih
// membandingkan, lalu test lulus tanpa assert (snapshot baru = sumber kebenaran).
//
//	go test ./internal/golden/ -update            # regen semua kombinasi
//	UPDATE_GOLDEN=1 go test ./internal/golden/     # idem (tanpa flag)
//	go test ./internal/golden/                      # bandingkan (mode normal)
//
// Determinisme (byte-identical, SPEC §5.2) dijaga oleh resolver/generator; golden
// hanya men-snapshot. Karena urutan FileOp & isi sudah deterministik, snapshot
// stabil lintas run/OS (path dinormalisasi ke slash POSIX).
//
// Microservice (kombinasi #4): hook "buf-generate" yang meng-emit gen/go/** TIDAK
// dijalankan oleh generator.Generate (itu post-gen hook orchestrator). Maka render
// menghasilkan file NON-gen (proto + services/** + libs/** + root) — persis yang
// ingin di-snapshot. Sebagai jaring pengaman, gen/go/** tetap diabaikan eksplisit.
package golden

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/faisalcayunda/gostarter/internal/answers"
	"github.com/faisalcayunda/gostarter/internal/fsutil"
	"github.com/faisalcayunda/gostarter/internal/generator"
	"github.com/faisalcayunda/gostarter/internal/module"
	"github.com/faisalcayunda/gostarter/internal/plan"
	"github.com/faisalcayunda/gostarter/internal/resolver"
	"github.com/faisalcayunda/gostarter/templates"
)

// updateFlag mengaktifkan mode regenerasi snapshot (`-update`). Selain flag, env
// UPDATE_GOLDEN (non-kosong) juga mengaktifkannya — berguna di lingkungan yang
// lebih nyaman lewat env (mis. `make golden-update`). Pengecekan flag pointer
// dereference ditunda ke updateMode() agar aman terhadap urutan init test.
var updateFlag = flag.Bool("update", false, "tulis ulang snapshot golden di testdata/golden/<combo>/ alih-alih membandingkan")

// updateMode melaporkan apakah harness berjalan dalam mode regenerasi snapshot.
func updateMode() bool {
	if updateFlag != nil && *updateFlag {
		return true
	}
	return os.Getenv("UPDATE_GOLDEN") != ""
}

// goldenCase mendefinisikan satu kombinasi kunci yang di-snapshot. dir adalah
// subdirektori di testdata/golden/. answers di-resolve & di-render apa adanya.
type goldenCase struct {
	dir     string          // subdir snapshot: testdata/golden/<dir>/
	answers answers.Answers // input resolver (sudah lengkap; default tetap diisi resolver)
}

// goldenCases adalah kombinasi KUNCI (T5.2). Dipilih untuk mencakup lintasan
// resolusi yang berbeda secara struktural:
//
//  1. monolith / net-http / db=none / addons=[makefile]
//     → monolith stdlib murni (jalur paling minimal + satu addon merge Makefile).
//  2. monolith / chi / db=postgres / addons=[docker,makefile,env]
//     → router non-default (http-chi menggantikan routing core) + db driver +
//     tiga addon (docker compose, Makefile, .env.example) → banyak target merge.
//  3. modular-monolith / net-http / db=none
//     → arch-modular menggantikan arch-monolith (layout berbeda).
//  4. microservice / services=[svc-a,svc-b] / comm=grpc
//     → layout monorepo gRPC + ekspansi per-service (proto/services/libs/root).
//     gen/go/** dikecualikan (di-emit hook buf, bukan render).
func goldenCases() []goldenCase {
	return []goldenCase{
		{
			dir: "monolith-nethttp-dbnone",
			answers: answers.Answers{
				Name:     "demo-mono",
				Module:   "github.com/example/demo-mono",
				Arch:     answers.ArchMonolith,
				Kind:     answers.KindREST,
				HTTP:     answers.HTTPNetHTTP,
				DB:       answers.DBNone,
				Makefile: true,
			},
		},
		{
			dir: "monolith-chi-postgres-docker-makefile-env",
			answers: answers.Answers{
				Name:       "demo-chi",
				Module:     "github.com/example/demo-chi",
				Arch:       answers.ArchMonolith,
				Kind:       answers.KindREST,
				HTTP:       answers.HTTPChi,
				DB:         answers.DBPostgres,
				Docker:     true,
				Makefile:   true,
				EnvExample: true,
			},
		},
		{
			dir: "modular-nethttp-dbnone",
			answers: answers.Answers{
				Name:   "demo-modular",
				Module: "github.com/example/demo-modular",
				Arch:   answers.ArchModularMonolith,
				Kind:   answers.KindREST,
				HTTP:   answers.HTTPNetHTTP,
				DB:     answers.DBNone,
			},
		},
		{
			// monolith / net-http / db=postgres / access=gorm
			// → jalur akses GORM: koneksi gorm.go + repository.go MENGGANTIKAN
			// pgxpool postgres.go (di-gate off); wiring GORM ter-AUTO-WIRE ke main.
			// Membuktikan tepat satu mekanisme akses ter-emit + dep gorm+driver.
			dir: "monolith-nethttp-postgres-gorm",
			answers: answers.Answers{
				Name:   "demo-gorm",
				Module: "github.com/example/demo-gorm",
				Arch:   answers.ArchMonolith,
				Kind:   answers.KindREST,
				HTTP:   answers.HTTPNetHTTP,
				DB:     answers.DBPostgres,
				Access: answers.AccessGORM,
			},
		},
		{
			dir: "microservice-svc-a-svc-b-grpc",
			answers: answers.Answers{
				Name:     "demo-ms",
				Module:   "github.com/example/demo-ms",
				Arch:     answers.ArchMicroservice,
				Services: []answers.Service{{Name: "svc-a"}, {Name: "svc-b"}},
				Comm:     answers.CommGRPC,
			},
		},
		{
			// monolith / net-http / db=postgres / access=gorm / addons=[strapgorm,makefile]
			// → add-on strapgorm (Strapi-style query builder di atas GORM): domain
			// contoh Product (model + repository GORM via strapgorm + handler GET
			// /api/products) yang me-REUSE *gorm.DB milik access-gorm-postgres (TANPA
			// pool kedua). Membuktikan: go.mod me-require strapgorm @ pseudo-version
			// pin + go directive 1.25; internal/product/** ter-emit; wiring main
			// (region:imports/wiring) & server.go (region:imports/routes) ter-AUTO-WIRE
			// memakai var `db` access-gorm yang sama (single pool). Makefile ikut untuk
			// menutup jalur merge addon di atas kombinasi strapgorm.
			dir: "monolith-nethttp-postgres-gorm-strapgorm-makefile",
			answers: answers.Answers{
				Name:      "shopdemo",
				Module:    "example.com/shopdemo",
				Arch:      answers.ArchMonolith,
				Kind:      answers.KindREST,
				HTTP:      answers.HTTPNetHTTP,
				DB:        answers.DBPostgres,
				Access:    answers.AccessGORM,
				Strapgorm: true,
				Makefile:  true,
			},
		},
		{
			// monolith / chi / db=postgres / access=gorm / addons=[strapgorm,makefile]
			// → strapgorm DI ATAS router chi: constraint strapgorm (arch=monolith +
			// access=gorm + db∈{postgres,mysql}) TIDAK membatasi HTTP framework, jadi
			// kombinasi ini valid. Membuktikan pendaftaran rute FRAMEWORK-AWARE: anchor
			// region:routes httpserver.New profil chi memakai variabel router `r` yang
			// in-scope (r.Get("/api/products", product.ListHandler())) — BUKAN `mux`.
			// Mencegah regresi "undefined: mux" pada router non-net/http.
			dir: "monolith-chi-postgres-gorm-strapgorm-makefile",
			answers: answers.Answers{
				Name:      "shopdemo",
				Module:    "example.com/shopdemo",
				Arch:      answers.ArchMonolith,
				Kind:      answers.KindREST,
				HTTP:      answers.HTTPChi,
				DB:        answers.DBPostgres,
				Access:    answers.AccessGORM,
				Strapgorm: true,
				Makefile:  true,
			},
		},
		{
			// monolith / echo / db=postgres / access=gorm / addons=[strapgorm,makefile]
			// → strapgorm DI ATAS router echo. Profil echo memakai variabel router `e`
			// dan TIDAK punya `mux` di scope region:routes — fragmen rute FRAMEWORK-AWARE
			// mendaftarkan via e.GET("/api/products", echo.WrapHandler(product.ListHandler())).
			// Kombinasi inilah yang dahulu GAGAL kompilasi ("undefined: mux") sebelum
			// fragmen rute di-gate per HTTP; golden ini mengunci perbaikannya.
			dir: "monolith-echo-postgres-gorm-strapgorm-makefile",
			answers: answers.Answers{
				Name:      "shopdemo",
				Module:    "example.com/shopdemo",
				Arch:      answers.ArchMonolith,
				Kind:      answers.KindREST,
				HTTP:      answers.HTTPEcho,
				DB:        answers.DBPostgres,
				Access:    answers.AccessGORM,
				Strapgorm: true,
				Makefile:  true,
			},
		},
		{
			// modular-monolith / net-http / db=postgres / access=gorm / strapgorm
			// → Product = domain modular kelas-satu internal/modules/product/**
			// (facade product.go + internal/core berduri). Di-inject *gorm.DB
			// access=gorm lewat composition root cmd/<app>/main.go; productMod
			// disisipkan ke httpserver.New via anchor region:modules. Mengunci
			// wiring modular (imports/wiring/modules) + go directive 1.25.
			dir: "modular-nethttp-postgres-gorm-strapgorm",
			answers: answers.Answers{
				Name:      "shopmod",
				Module:    "example.com/shopmod",
				Arch:      answers.ArchModularMonolith,
				Kind:      answers.KindREST,
				HTTP:      answers.HTTPNetHTTP,
				DB:        answers.DBPostgres,
				Access:    answers.AccessGORM,
				Strapgorm: true,
			},
		},
		{
			// microservice / svc-a+svc-b / db=postgres / access=gorm / strapgorm
			// → service product MANDIRI (gRPC Ping + HTTP /api/products via strapgorm)
			// dgn koneksi GORM sendiri. proto/product + services/product/** ter-emit;
			// docker-compose menambah service product + postgres + volume. go.mod
			// jujur (gorm + strapgorm + driver/postgres saja). gen/ DIKECUALIKAN
			// (di-emit hook buf, bukan render). go directive 1.25.
			dir: "microservice-svc-a-svc-b-postgres-gorm-strapgorm",
			answers: answers.Answers{
				Name:      "shopms",
				Module:    "example.com/shopms",
				Arch:      answers.ArchMicroservice,
				Services:  []answers.Service{{Name: "svc-a"}, {Name: "svc-b"}},
				Comm:      answers.CommGRPC,
				DB:        answers.DBPostgres,
				Access:    answers.AccessGORM,
				Strapgorm: true,
			},
		},
	}
}

// TestGolden menjalankan snapshot/komparasi untuk tiap kombinasi kunci. Mode
// regenerasi (-update / UPDATE_GOLDEN) menulis ulang snapshot; mode normal
// membandingkan tree + isi byte-per-byte.
func TestGolden(t *testing.T) {
	reg := loadRegistry(t)
	res := resolver.New(reg)

	for _, gc := range goldenCases() {
		t.Run(gc.dir, func(t *testing.T) {
			t.Parallel()

			// 1. Resolve answers → plan.
			p, err := res.Resolve(gc.answers)
			if err != nil {
				t.Fatalf("Resolve(%s): %v", gc.dir, err)
			}

			// 2. Render plan ke temp dir via generator kanonik + RealWriter.
			rendered := renderToMap(t, reg, p)

			goldenDir := filepath.Join("testdata", "golden", gc.dir)

			if updateMode() {
				writeSnapshot(t, goldenDir, rendered)
				t.Logf("snapshot %s diperbarui (%d file)", gc.dir, len(rendered))
				return
			}

			// 3. Bandingkan dengan snapshot yang di-commit.
			want := readSnapshot(t, goldenDir)
			compareSnapshots(t, gc.dir, want, rendered)
		})
	}
}

// loadRegistry memuat katalog modul NYATA dari templates.FS (sama dengan jalur
// CLI create). Gagal Load = fatal (katalog rusak — di luar lingkup golden).
func loadRegistry(t *testing.T) module.Registry {
	t.Helper()
	reg := module.NewRegistry()
	if err := reg.Load(templates.FS); err != nil {
		t.Fatalf("muat registry dari templates.FS: %v", err)
	}
	return reg
}

// renderToMap mengeksekusi plan ke t.TempDir() via generator kanonik (renderer +
// merge-assembler atas templates.FS) memakai RealWriter, lalu memuat seluruh pohon
// hasil render menjadi map[relpath]->content. Path dinormalisasi ke slash POSIX &
// relatif terhadap root project agar snapshot stabil lintas OS. gen/go/**
// dikecualikan (di-emit hook buf, bukan render).
func renderToMap(t *testing.T, reg module.Registry, p plan.GeneratePlan) map[string]string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "out")

	g := generator.New(reg, generator.NewRenderer(templates.FS), generator.NewMergeAssembler())
	if err := g.Generate(p, target, fsutil.RealWriter{}); err != nil {
		t.Fatalf("Generate ke %q: %v", target, err)
	}

	out := make(map[string]string)
	walkErr := filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(target, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if isExcluded(rel) {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		out[rel] = string(b)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %q: %v", target, walkErr)
	}
	return out
}

// isExcluded melaporkan apakah sebuah path relatif harus DIABAIKAN dari snapshot.
// Seluruh subtree gen/ adalah artefak hook buf-generate (bukan output render) —
// di-skip agar golden microservice tetap stabil tanpa toolchain buf, dan agar TIDAK
// PERNAH ada artefak protobuf tersimpan ke snapshot (M-1: kecualikan 'gen/' penuh,
// bukan hanya 'gen/go/' — apa pun yang di-emit buf di bawah gen/ tetap di luar golden).
func isExcluded(rel string) bool {
	return rel == "gen" || strings.HasPrefix(rel, "gen/")
}

// ── Snapshot I/O ─────────────────────────────────────────────────────────────

// writeSnapshot menulis ulang testdata/golden/<combo>/: menghapus snapshot lama
// lebih dulu (agar file yang TIDAK lagi di-emit tidak tertinggal), lalu menulis
// tiap file rendered ke path mirror. Dipakai HANYA di mode -update/UPDATE_GOLDEN.
func writeSnapshot(t *testing.T, goldenDir string, files map[string]string) {
	t.Helper()
	if err := os.RemoveAll(goldenDir); err != nil {
		t.Fatalf("hapus snapshot lama %q: %v", goldenDir, err)
	}
	for rel, content := range files {
		dst := filepath.Join(goldenDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
			t.Fatalf("tulis snapshot %q: %v", dst, err)
		}
	}
}

// readSnapshot memuat seluruh pohon testdata/golden/<combo>/ menjadi
// map[relpath]->content (slash POSIX). Snapshot yang belum ada → fatal dengan
// pesan yang menyarankan menjalankan -update lebih dulu.
func readSnapshot(t *testing.T, goldenDir string) map[string]string {
	t.Helper()
	info, err := os.Stat(goldenDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("snapshot golden %q tidak ada — jalankan `go test ./internal/golden/ -update` untuk men-generate-nya dahulu", goldenDir)
	}
	out := make(map[string]string)
	walkErr := filepath.WalkDir(goldenDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(goldenDir, path)
		if rerr != nil {
			return rerr
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("baca snapshot %q: %v", goldenDir, walkErr)
	}
	return out
}

// ── Komparasi ────────────────────────────────────────────────────────────────

// compareSnapshots membandingkan file tree + isi want (snapshot di-commit) vs got
// (hasil render). Mismatch dilaporkan via t.Errorf dengan diff ringkas: file
// hilang/ekstra disebut by path; konten berbeda menampilkan diff baris pertama
// yang menyimpang + ringkasan panjang.
func compareSnapshots(t *testing.T, combo string, want, got map[string]string) {
	t.Helper()

	// (a) Tree: path yang ada di want tapi tidak di got, dan sebaliknya.
	wantPaths := sortedKeys(want)
	gotPaths := sortedKeys(got)

	var missing, extra []string
	for _, p := range wantPaths {
		if _, ok := got[p]; !ok {
			missing = append(missing, p)
		}
	}
	for _, p := range gotPaths {
		if _, ok := want[p]; !ok {
			extra = append(extra, p)
		}
	}
	if len(missing) > 0 {
		t.Errorf("[%s] %d file ADA di snapshot tapi TIDAK di-render (golden tertinggal? jalankan -update bila memang dihapus):\n  %s",
			combo, len(missing), strings.Join(missing, "\n  "))
	}
	if len(extra) > 0 {
		t.Errorf("[%s] %d file DI-RENDER tapi TIDAK ada di snapshot (output baru? jalankan -update bila memang ditambah):\n  %s",
			combo, len(extra), strings.Join(extra, "\n  "))
	}

	// (b) Isi: untuk path yang ada di keduanya, bandingkan byte-per-byte.
	for _, p := range wantPaths {
		gContent, ok := got[p]
		if !ok {
			continue // sudah dilaporkan sebagai missing
		}
		if want[p] != gContent {
			t.Errorf("[%s] isi %q berbeda dari snapshot:\n%s", combo, p, lineDiff(want[p], gContent))
		}
	}
}

// lineDiff menghasilkan diff ringkas dua string: baris pertama yang menyimpang
// (1-indexed) + cuplikan want vs got pada baris itu, plus ringkasan jumlah baris.
// Ini bukan diff penuh (cukup untuk lokalisasi cepat saat golden gagal).
func lineDiff(want, got string) string {
	wl := strings.Split(want, "\n")
	gl := strings.Split(got, "\n")
	n := min(len(wl), len(gl))
	for i := 0; i < n; i++ {
		if wl[i] != gl[i] {
			return formatDiffAt(i+1, wl[i], gl[i], len(wl), len(gl))
		}
	}
	// Tidak ada baris yang berbeda dalam prefiks bersama → panjang berbeda.
	if len(wl) != len(gl) {
		return formatDiffAt(n+1, sliceLine(wl, n), sliceLine(gl, n), len(wl), len(gl))
	}
	return "  (string berbeda tetapi tak ada baris yang menyimpang — kemungkinan whitespace trailing)"
}

// formatDiffAt memformat baris diff pada nomor line (1-indexed).
func formatDiffAt(line int, want, got string, wantLines, gotLines int) string {
	var b strings.Builder
	b.WriteString("  baris pertama berbeda di #")
	b.WriteString(strconv.Itoa(line))
	b.WriteString(" (want ")
	b.WriteString(strconv.Itoa(wantLines))
	b.WriteString(" baris, got ")
	b.WriteString(strconv.Itoa(gotLines))
	b.WriteString(" baris)\n")
	b.WriteString("    - want: ")
	b.WriteString(quoteTrunc(want))
	b.WriteString("\n    + got:  ")
	b.WriteString(quoteTrunc(got))
	return b.String()
}

// sliceLine mengembalikan baris idx bila ada, atau "<EOF>" bila di luar batas.
func sliceLine(lines []string, idx int) string {
	if idx < len(lines) {
		return lines[idx]
	}
	return "<EOF>"
}

// quoteTrunc memotong baris panjang agar diff tetap ringkas (maks 120 rune).
func quoteTrunc(s string) string {
	const max = 120
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

// sortedKeys mengembalikan key map terurut (path stabil untuk laporan diff).
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
