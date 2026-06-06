// addservice.go — subcommand "gostarter add service <name>".
//
// Beroperasi pada project microservice monorepo yang SUDAH ADA (US-05): scaffold
// services/<name>/ + proto/<name>/v1/<name>.proto lewat jalur TERPADU resolver→
// generator (SAMA seperti create — bukan island), sisip blok ke docker-compose.yml
// (anchor region:services, idempoten) via generator.MergeAssembler, lalu jalankan
// buf generate → gofmt → go mod tidy. Menolak ramah bila bukan project microservice
// atau service sudah ada/reserved. Mendukung --dry-run.
//
// Invarian (vs create): TIDAK EnsureEmptyDir (project existing), TIDAK menulis ulang
// go.mod/file root. Penulisan FileOp service baru lewat generator.execFileOp
// (containment fsutil.JoinTarget, H-1 — tak ada string-concat path). Nama service
// dari ARG; nama project/module dari go.mod via modpath.Base (H-2 — bukan
// filepath.Base(projectDir) yang bisa "." saat dijalankan di cwd).

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"

	"github.com/faisalcayunda/gostarter/internal/answers"
	"github.com/faisalcayunda/gostarter/internal/fsutil"
	"github.com/faisalcayunda/gostarter/internal/generator"
	"github.com/faisalcayunda/gostarter/internal/hooks"
	"github.com/faisalcayunda/gostarter/internal/module"
	"github.com/faisalcayunda/gostarter/internal/plan"
	"github.com/faisalcayunda/gostarter/internal/resolver"
	"github.com/faisalcayunda/gostarter/templates"
)

// composeFileName adalah nama file orkestrasi yang disisipi blok service baru.
const composeFileName = "docker-compose.yml"

// addServiceFlags menampung flag subcommand "add service".
type addServiceFlags struct {
	output string // -o/--output: root project (default: cwd)
	dryRun bool   // --dry-run: preview file baru tanpa menulis
}

// newAddCmd membangun subcommand "add" + child "service".
func newAddCmd() *cobra.Command {
	add := &cobra.Command{
		Use:   "add",
		Short: "Tambah komponen ke project yang sudah ada",
		Long:  "Perintah inkremental untuk project gostarter existing. Saat ini: `add service <name>` (microservice monorepo).",
		Args:  cobra.NoArgs,
	}
	add.AddCommand(newAddServiceCmd())
	return add
}

// newAddServiceCmd membangun "gostarter add service <name>".
func newAddServiceCmd() *cobra.Command {
	f := &addServiceFlags{}

	cmd := &cobra.Command{
		Use:   "service <name>",
		Short: "Tambah satu service gRPC ke project microservice existing",
		Long: "Scaffold service baru di project microservice monorepo (jalur terpadu resolver→generator):\n" +
			"  services/<name>/{cmd,internal} + proto/<name>/v1/<name>.proto,\n" +
			"  sisip blok service ke docker-compose.yml, lalu buf generate → gofmt → go mod tidy.\n\n" +
			"Dijalankan dari ROOT project (atau pakai -o <dir>). Menolak bila bukan project\n" +
			"microservice gostarter, atau bila nama service sudah ada / reserved ('gateway').\n" +
			"Prasyarat: `buf` + plugin Go di PATH (untuk regen gen/go/<name>/).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddService(cmd, f, args[0])
		},
	}

	fl := cmd.Flags()
	fl.StringVarP(&f.output, "output", "o", "", "root project microservice (default: direktori kerja)")
	fl.BoolVar(&f.dryRun, "dry-run", false, "preview file baru tanpa menulis ke disk")
	return cmd
}

// microProjectInfo memuat metadata project microservice existing yang terdeteksi.
type microProjectInfo struct {
	modulePath   string // dari go.mod (H-2: sumber nama module — bukan dir)
	goVersion    string // dari go.mod (kosong → resolver default microservice)
	serviceCount int    // jumlah service existing (alokasi port service baru, L-3)
}

// runAddService menjalankan orchestration add-service lewat jalur terpadu.
func runAddService(cmd *cobra.Command, f *addServiceFlags, name string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	out := cmd.OutOrStdout()

	projectDir := f.output
	if projectDir == "" {
		projectDir = "."
	}
	projectDir = filepath.Clean(projectDir)

	// 1. Validasi nama service (regex/reserved/lowercase) — sumber tunggal
	//    answers.Validate (microservice branch). Duplikat dicek setelah deteksi.
	if err := validateNewServiceName(name); err != nil {
		return err
	}

	// 2. Deteksi project microservice gostarter + baca module path & go version dari
	//    go.mod (H-2). Bukan microservice → tolak ramah (US-05 Sk.2).
	info, err := detectMicroProject(projectDir)
	if err != nil {
		return err
	}

	// 3. Tolak bila service sudah ada (US-05 Sk.3).
	if serviceExists(projectDir, name) {
		return fmt.Errorf("service %q sudah ada di services/%s — pilih nama lain", name, name)
	}

	// 4. Resolusi inkremental (jalur terpadu): FileOp per-service + fragmen compose.
	reg := module.NewRegistry()
	if err := reg.Load(templates.FS); err != nil {
		return fmt.Errorf("memuat katalog template gagal: %w", err)
	}
	res := resolver.New(reg)
	addPlan, err := res.ResolveAddService(resolver.AddServiceInfo{
		ModulePath:  info.modulePath,
		GoVersion:   info.goVersion,
		ServiceName: name,
		Index:       info.serviceCount, // service baru = ke-(N); port base+N (L-3)
	})
	if err != nil {
		return fmt.Errorf("resolusi add-service gagal: %w", err)
	}

	// 5. Rakit compose baru: render fragmen service baru lalu sisip ke compose
	//    on-disk pada anchor region:services (MergeAssembler — marker dipertahankan).
	composePath, err := fsutil.JoinTarget(projectDir, composeFileName)
	if err != nil {
		return fmt.Errorf("path compose: %w", err)
	}
	existingCompose, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("baca %s: %w", composeFileName, err)
	}
	newCompose, err := mergeComposeFragment(existingCompose, addPlan)
	if err != nil {
		return err
	}

	// 6. Dry-run: preview path baru + compose (tanpa menulis & tanpa hook).
	if f.dryRun {
		planned, perr := previewAddService(addPlan, projectDir)
		if perr != nil {
			return perr
		}
		planned = append(planned, composePath)
		printAddServiceDryRun(out, name, projectDir, planned)
		return nil
	}

	// 7. Tulis file baru via generator (containment fsutil.JoinTarget, H-1) — TANPA
	//    EnsureEmptyDir & TANPA go.mod (jalur add-service, project existing).
	if err := writeAddServiceFiles(addPlan, projectDir); err != nil {
		return err
	}
	// 8. Tulis ulang compose dengan blok service baru tersisip.
	if err := os.WriteFile(composePath, []byte(newCompose), 0o644); err != nil {
		return fmt.Errorf("tulis %s: %w", composeFileName, err)
	}

	// 9. Hook: buf generate (regen gen/go/<name>/) → gofmt → go mod tidy. git-init
	//    TIDAK dijalankan (project existing sudah berriwayat).
	warnf := func(format string, args ...any) { fmt.Fprintf(out, format+"\n", args...) }
	if err := runAddServiceHooks(ctx, projectDir, warnf); err != nil {
		return err
	}

	fmt.Fprintf(out, "\n✔ Service '%s' ditambahkan ke %s\n", name, projectDir)
	for _, op := range addPlan.Files {
		fmt.Fprintf(out, "  %s\n", op.TargetPath)
	}
	fmt.Fprintf(out, "  compose:  blok '%s' disisipkan ke %s\n", name, composeFileName)
	fmt.Fprintf(out, "\nLangkah berikutnya:\n  go build ./...\n")
	return nil
}

// validateNewServiceName memvalidasi nama service baru lewat answers.Validate
// (microservice branch — sumber tunggal regex/reserved/lowercase). Project Name &
// Module diisi placeholder valid agar hanya aturan q_svc yang relevan diuji.
func validateNewServiceName(name string) error {
	a := answers.Answers{
		Name:     "svc-project",
		Module:   "example.com/svc-project",
		Arch:     answers.ArchMicroservice,
		Services: []answers.Service{{Name: name}},
	}
	// answers.Validate sudah menghasilkan pesan deskriptif (menyebut nama & aturan);
	// diteruskan apa adanya agar tak ada prefiks ganda ("tidak valid: … tidak valid").
	if err := a.Validate(); err != nil {
		return err
	}
	return nil
}

// detectMicroProject memverifikasi dir adalah project microservice gostarter
// (marker: buf.yaml + proto/ + services/ + go.mod) lalu membaca module path & go
// directive dari go.mod (H-2). Bila bukan → error ramah (US-05 Sk.2).
func detectMicroProject(dir string) (microProjectInfo, error) {
	for _, m := range []string{"buf.yaml", "proto", "services", "go.mod"} {
		p, jerr := fsutil.JoinTarget(dir, m)
		if jerr != nil {
			return microProjectInfo{}, jerr
		}
		if _, err := os.Stat(p); err != nil {
			return microProjectInfo{}, fmt.Errorf(
				"%q bukan project microservice gostarter (marker %q tidak ada): `add service` hanya untuk monorepo microservice",
				dir, m,
			)
		}
	}

	gomodPath, err := fsutil.JoinTarget(dir, "go.mod")
	if err != nil {
		return microProjectInfo{}, err
	}
	modPath, goVer, err := readGoMod(gomodPath)
	if err != nil {
		return microProjectInfo{}, err
	}
	servicesDir, err := fsutil.JoinTarget(dir, "services")
	if err != nil {
		return microProjectInfo{}, err
	}
	count, err := countServices(servicesDir)
	if err != nil {
		return microProjectInfo{}, err
	}
	return microProjectInfo{modulePath: modPath, goVersion: goVer, serviceCount: count}, nil
}

// readGoMod mem-parse go.mod dan mengembalikan (module path, go version). Memakai
// x/mod/modfile (sama dengan generator.writeGoMod) agar parsing konsisten. Module
// path divalidasi non-kosong; nama project diturunkan modpath.Base oleh pemanggil.
func readGoMod(path string) (string, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("baca go.mod: %w", err)
	}
	f, err := modfile.Parse(path, raw, nil)
	if err != nil {
		return "", "", fmt.Errorf("parse go.mod: %w", err)
	}
	if f.Module == nil || f.Module.Mod.Path == "" {
		return "", "", fmt.Errorf("go.mod tidak memuat deklarasi module")
	}
	goVer := ""
	if f.Go != nil {
		goVer = f.Go.Version
	}
	return f.Module.Mod.Path, goVer, nil
}

// serviceExists melaporkan apakah services/<name> sudah ada (containment-aman).
func serviceExists(dir, name string) bool {
	p, err := fsutil.JoinTarget(dir, "services/"+name)
	if err != nil {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// countServices menghitung subdir di services/ (tiap subdir = satu service) — dasar
// alokasi port deterministik service baru (L-3). Service baru = ke-(count).
func countServices(servicesDir string) (int, error) {
	entries, err := os.ReadDir(servicesDir)
	if err != nil {
		return 0, fmt.Errorf("baca direktori services/: %w", err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			n++
		}
	}
	return n, nil
}

// mergeComposeFragment merender fragmen compose service baru lalu menyisipkannya
// ke compose on-disk pada anchor region:services via generator.MergeAssembler
// (marker DIPERTAHANKAN — idempotensi add-service berikutnya). Render fragmen
// memakai renderer kanonik generator atas templates.FS (jalur terpadu).
func mergeComposeFragment(existing []byte, addPlan resolver.AddServicePlan) (string, error) {
	r := generator.NewRenderer(templates.FS)
	rendered, err := r.Render(addPlan.ComposeTemplatePath, mergeComposeData(addPlan.Compose.DataOverride))
	if err != nil {
		return "", fmt.Errorf("render fragmen compose: %w", err)
	}
	frag := plan.Fragment{
		Anchor:  addPlan.Compose.Anchor,
		Content: strings.TrimRight(string(rendered), "\n"),
		Order:   addPlan.Compose.Order,
	}
	asm := generator.NewMergeAssembler()
	merged, err := asm.Assemble(existing, []plan.Fragment{frag})
	if err != nil {
		return "", fmt.Errorf("sisip blok compose: %w", err)
	}
	return string(merged), nil
}

// mergeComposeData menyiapkan data render fragmen compose: DataOverride per-service
// sudah memuat semua field yang dibutuhkan fragmen (Service/IsFirst/Others/
// GatewayPort). Dikembalikan apa adanya (fragmen tak butuh data global lain).
func mergeComposeData(override map[string]any) map[string]any {
	if override == nil {
		return map[string]any{}
	}
	return override
}

// writeAddServiceFiles mengeksekusi FileOp service baru via generator (TANPA
// EnsureEmptyDir / TANPA go.mod). Memakai generator.GenerateFiles — jalur terpadu
// yang menulis hanya p.Files (containment fsutil.JoinTarget, H-1).
func writeAddServiceFiles(addPlan resolver.AddServicePlan, projectDir string) error {
	g := generator.New(module.NewRegistry(), generator.NewRenderer(templates.FS), generator.NewMergeAssembler())
	if err := g.GenerateFiles(plan.GeneratePlan{Files: addPlan.Files}, projectDir, fsutil.RealWriter{}); err != nil {
		return fmt.Errorf("tulis file service baru gagal: %w", err)
	}
	return nil
}

// previewAddService menjalankan FileOp service baru lewat DryRunWriter untuk
// mengumpulkan path terencana (tanpa menulis) — preview --dry-run.
func previewAddService(addPlan resolver.AddServicePlan, projectDir string) ([]string, error) {
	dw := &fsutil.DryRunWriter{}
	g := generator.New(module.NewRegistry(), generator.NewRenderer(templates.FS), generator.NewMergeAssembler())
	if err := g.GenerateFiles(plan.GeneratePlan{Files: addPlan.Files}, projectDir, dw); err != nil {
		return nil, fmt.Errorf("preview file service baru gagal: %w", err)
	}
	return append([]string(nil), dw.Planned...), nil
}

// runAddServiceHooks menjalankan hook pasca-add: buf-generate → gofmt → go-mod-tidy
// (git-init TIDAK — project existing). Urutan literal selaras hooks.Order*.
func runAddServiceHooks(ctx context.Context, projectDir string, warnf func(string, ...any)) error {
	specs := []hooks.Spec{
		{Name: hooks.NameBufGenerate, Order: hooks.OrderBufGenerate},
		{Name: hooks.NameGofmt, Order: hooks.OrderGofmt},
		{Name: hooks.NameGoModTidy, Order: hooks.OrderGoModTidy},
	}
	return hooks.Run(ctx, projectDir, hooks.Build(specs), warnf)
}

// printAddServiceDryRun mencetak preview path baru (--dry-run).
func printAddServiceDryRun(out io.Writer, name, projectDir string, planned []string) {
	fmt.Fprintf(out, "[dry-run] `add service %s` di project %s — file baru:\n\n", name, projectDir)
	paths := append([]string(nil), planned...)
	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintf(out, "  %s\n", p)
	}
	fmt.Fprintln(out, "\n(tidak ada file ditulis — --dry-run; buf generate juga dilewati)")
}
