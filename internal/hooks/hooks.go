// Package hooks mendefinisikan hook pasca-generate (gofmt, go mod tidy, git
// init) yang dijalankan setelah file project ditulis ke disk.
//
// Fase 3: implementasi nyata via os/exec sesuai ADR-002 §7. Urutan kanonik
// (HookSpec.Order menaik):
//
//	Order  5  BufGenerate → "buf generate" (HANYA arch=microservice; menghasilkan
//	                        gen/go/** sebelum gofmt/tidy agar paket gRPC ada). fail-fast.
//	Order 10  Gofmt       → "gofmt -w ." (jaring pengaman; mayoritas .go sudah
//	                        lewat go/format saat render, ini menutup sisa). fail-fast.
//	Order 20  GoModTidy   → "go mod tidy" (finalisasi go.mod + buat go.sum). fail-fast.
//	Order 30  GitInit      → "git init" + "git add -A" + initial commit. warn-only.
//
// Hanya RealWriter yang memicu hook; DryRunWriter tidak pernah menjalankan hook.
package hooks

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
)

// Nama kanonik hook — dipakai di plan.HookSpec.Name agar orchestrator dapat
// memetakan HookSpec → PostGenHook konkret.
const (
	NameBufGenerate = "buf-generate"
	NameGofmt       = "gofmt"
	NameGoModTidy   = "go-mod-tidy"
	NameGitInit     = "git-init"
)

// Order kanonik hook (ADR-002 §7). Diekspor agar resolver dapat mengisi
// plan.HookSpec.Order secara konsisten.
const (
	OrderBufGenerate = 5
	OrderGofmt       = 10
	OrderGoModTidy   = 20
	OrderGitInit     = 30
)

// PostGenHook adalah satu langkah pasca-generate yang dijalankan pada direktori
// project hasil generate.
type PostGenHook interface {
	// Name mengembalikan identitas hook (cocok dengan plan.HookSpec.Name).
	Name() string
	// Run mengeksekusi hook pada projectDir.
	Run(ctx context.Context, projectDir string) error
}

// runCmd menjalankan perintah eksternal di dir kerja projectDir, mengembalikan
// error yang menyertakan output (stdout+stderr) bila gagal — memudahkan
// diagnosa kegagalan hook (ADR-002 §7: pesan error menyebut hook yang gagal).
func runCmd(ctx context.Context, projectDir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v gagal: %w\n%s", name, args, err, out)
	}
	return nil
}

// BufGenerate menjalankan "buf generate" untuk menghasilkan kode gRPC (gen/go/**)
// dari kontrak proto/ project hasil generate. HANYA dipakai untuk arch=microservice
// (resolver yang menambahkannya ke plan.Hooks). Dijalankan PALING DULU (order 5)
// agar paket hasil generate tersedia sebelum gofmt & go mod tidy.
//
// Catatan: gen/ DI-COMMIT — builder menjalankan hook ini saat create sehingga
// project hasil generate dapat "go build ./..." TANPA buf. "buf" hanya prasyarat
// saat BUILDER menghasilkan project, bukan saat konsumen mem-build project.
type BufGenerate struct{}

// Name mengembalikan NameBufGenerate.
func (BufGenerate) Name() string { return NameBufGenerate }

// Run menjalankan "buf generate" di projectDir. fail-fast: tanpa gen/ kode gRPC,
// project tidak akan ter-build (DoD microservice). Bila "buf" tidak ada di PATH,
// mengembalikan error yang JELAS beserta perintah instalasinya.
func (BufGenerate) Run(ctx context.Context, projectDir string) error {
	if _, err := exec.LookPath("buf"); err != nil {
		return fmt.Errorf(
			"microservice butuh buf — install: go install github.com/bufbuild/buf/cmd/buf@latest: %w",
			err,
		)
	}
	return runCmd(ctx, projectDir, "buf", "generate")
}

// Gofmt menjalankan "gofmt -w ." atas seluruh file Go project hasil generate.
// Jaring pengaman terhadap template yang merender Go "kurang rapi" (ADR-002 §7).
type Gofmt struct{}

// Name mengembalikan NameGofmt.
func (Gofmt) Name() string { return NameGofmt }

// Run menjalankan "gofmt -w ." di projectDir. fail-fast: kegagalan format
// adalah prasyarat "build hijau tanpa edit" (DoD #1).
func (Gofmt) Run(ctx context.Context, projectDir string) error {
	return runCmd(ctx, projectDir, "gofmt", "-w", ".")
}

// GoModTidy menjalankan "go mod tidy" untuk memfinalisasi go.mod/go.sum project.
//
// Catatan ADR-002 §7: "go mod init" TIDAK diperlukan karena generator sudah
// merakit go.mod (module + go directive + require) via x/mod/modfile.Format.
// Hook ini hanya menjalankan tidy untuk menormalkan require & membuat go.sum.
type GoModTidy struct{}

// Name mengembalikan NameGoModTidy.
func (GoModTidy) Name() string { return NameGoModTidy }

// Run menjalankan "go mod tidy" di projectDir. fail-fast (prasyarat DoD #1).
func (GoModTidy) Run(ctx context.Context, projectDir string) error {
	return runCmd(ctx, projectDir, "go", "mod", "tidy")
}

// GitInit menjalankan "git init" + "git add -A" + initial commit. Dijalankan
// HANYA bila Answers.Git == true (resolver yang memutuskan via plan.Hooks).
//
// Error-handling: warn-only (ADR-002 §7). Kegagalan git (mis. git tak terpasang)
// TIDAK membatalkan generate; orchestrator mencetak peringatan dan exit 0.
type GitInit struct{}

// Name mengembalikan NameGitInit.
func (GitInit) Name() string { return NameGitInit }

// Run menjalankan inisialisasi git + initial commit di projectDir. Bila git
// tidak tersedia, mengembalikan error (orchestrator memperlakukannya warn-only).
func (GitInit) Run(ctx context.Context, projectDir string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git tidak ditemukan di PATH: %w", err)
	}
	if err := runCmd(ctx, projectDir, "git", "init"); err != nil {
		return err
	}
	if err := runCmd(ctx, projectDir, "git", "add", "-A"); err != nil {
		return err
	}
	// Commit awal. -q untuk output ringkas; identitas commit mengikuti konfigurasi
	// git user lokal. Bila identitas belum di-set, git gagal → warn-only di orchestrator.
	// Pesan commit netral (zero lock-in: tidak menyebut builder di riwayat project).
	return runCmd(ctx, projectDir, "git", "commit", "-q", "-m", "Initial commit")
}

// registry memetakan nama hook → implementasi PostGenHook konkret.
var registry = map[string]PostGenHook{
	NameBufGenerate: BufGenerate{},
	NameGofmt:       Gofmt{},
	NameGoModTidy:   GoModTidy{},
	NameGitInit:     GitInit{},
}

// ByName mengembalikan PostGenHook untuk sebuah nama (cocok dengan
// plan.HookSpec.Name); ok=false bila nama tak dikenal.
func ByName(name string) (PostGenHook, bool) {
	h, ok := registry[name]
	return h, ok
}

// Plan adalah satu hook terurut yang akan dijalankan orchestrator, hasil
// penerjemahan plan.HookSpec. WarnOnly menandai hook yang kegagalannya tidak
// membatalkan generate (mis. GitInit).
type Plan struct {
	Hook     PostGenHook
	Order    int
	WarnOnly bool
}

// Spec adalah bentuk minimal plan.HookSpec yang dibutuhkan Run untuk menghindari
// import siklik ke package plan. Orchestrator memetakan plan.HookSpec → Spec.
type Spec struct {
	Name  string
	Order int
}

// Build menerjemahkan daftar Spec (dari plan.GeneratePlan.Hooks) menjadi []Plan
// terurut (by Order menaik). Hook tak dikenal di-skip diam-diam (resolver hanya
// mengisi nama kanonik). GitInit ditandai warn-only sesuai ADR-002 §7.
func Build(specs []Spec) []Plan {
	plans := make([]Plan, 0, len(specs))
	for _, s := range specs {
		h, ok := ByName(s.Name)
		if !ok {
			continue
		}
		plans = append(plans, Plan{
			Hook:     h,
			Order:    s.Order,
			WarnOnly: s.Name == NameGitInit,
		})
	}
	sort.SliceStable(plans, func(i, j int) bool { return plans[i].Order < plans[j].Order })
	return plans
}

// Run mengeksekusi serangkaian hook terurut pada projectDir.
//
// Orchestration (ADR-002 §7):
//   - fail-fast pada hook non-warn-only (Gofmt, GoModTidy): error pertama
//     menghentikan eksekusi dan dikembalikan.
//   - warn-only pada GitInit: kegagalan tidak membatalkan; pesan dikirim ke warnf
//     (bila non-nil) dan eksekusi lanjut.
//
// warnf boleh nil (kegagalan warn-only diabaikan diam-diam).
func Run(ctx context.Context, projectDir string, plans []Plan, warnf func(format string, args ...any)) error {
	for _, p := range plans {
		err := p.Hook.Run(ctx, projectDir)
		if err == nil {
			continue
		}
		if p.WarnOnly {
			if warnf != nil {
				warnf("peringatan: hook %s gagal (diabaikan): %v", p.Hook.Name(), err)
			}
			continue
		}
		return fmt.Errorf("hook %s gagal: %w", p.Hook.Name(), err)
	}
	return nil
}

// ErrSkipped dikembalikan helper opsional bila tak ada hook yang dijalankan.
var ErrSkipped = errors.New("hooks: tidak ada hook untuk dijalankan")
