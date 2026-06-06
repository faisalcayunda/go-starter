// create.go — subcommand "gostarter create".
//
// Mengimplementasikan flow MVP (SPEC §5.1 subset Fase 3):
//   - parse flag → answers.Answers, ATAU jalankan wizard huh (interaktif),
//   - Answers.Validate (field-level),
//   - resolver.Resolve → plan.GeneratePlan (default + constraint, SPEC §6),
//   - EnsureEmptyDir(target) (kecuali --dry-run),
//   - generator.Generate (RealWriter atau DryRunWriter),
//   - post-gen hooks (Gofmt → GoModTidy fail-fast → GitInit warn-only) kecuali dry-run,
//   - cetak ringkasan / tree.
//
// SCOPE FASE 4a: arch ∈ {monolith, modular-monolith}, kind=rest,
// http ∈ {net/http, chi, echo}, db ∈ {none, postgres, mysql}, add-on ci
// (CI ∈ {github-actions, gitlab-ci}) + observability. Mode interaktif hanya
// menawarkan subset itu (lihat package prompt). Mode non-interaktif menerima
// flag subset penuh (dipakai smoke/matrix test).

package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/faisalcayunda/gostarter/internal/answers"
	"github.com/faisalcayunda/gostarter/internal/fsutil"
	"github.com/faisalcayunda/gostarter/internal/generator"
	"github.com/faisalcayunda/gostarter/internal/hooks"
	"github.com/faisalcayunda/gostarter/internal/module"
	"github.com/faisalcayunda/gostarter/internal/plan"
	"github.com/faisalcayunda/gostarter/internal/prompt"
	"github.com/faisalcayunda/gostarter/internal/resolver"
	"github.com/faisalcayunda/gostarter/templates"
)

// createFlags menampung nilai mentah flag subcommand create (sebelum diresolusi
// ke answers.Answers). Pemisahan ini memudahkan deteksi "flag di-set eksplisit".
type createFlags struct {
	name     string
	module   string
	arch     string
	kind     string
	http     string
	db       string
	access   string
	migrate  string
	ci       string   // --ci (github-actions|gitlab-ci|none); provider CI
	addons   []string // --addons (csv)
	features []string // --feature (digabung union dengan addons, M-2)

	// Microservice (Fase 4b, arch=microservice). Hanya relevan bila --arch=microservice.
	services    []string // --service (repeatable); daftar service awal
	servicesCSV []string // --services (csv); digabung union dengan --service
	comm        string   // --comm (grpc default; rest/event ditolak ramah v1)
	gateway     bool     // --gateway
	noGateway   bool     // --no-gateway (menang atas --gateway)

	// presetAddons diisi dari --config (config.go mapPresetToFlags); digabung
	// UNION dengan addons+features (M-2). Bukan flag — sumbernya preset YAML.
	presetAddons []string

	git   bool
	noGit bool

	config string // --config <file.yaml> (preset, SPEC §5.4)

	dryRun         bool
	output         string
	yes            bool
	nonInteractive bool
}

// newCreateCmd membangun subcommand "create".
func newCreateCmd() *cobra.Command {
	f := &createFlags{}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Generate project Go baru",
		Long: "Generate project Go best-practice. Tanpa flag wajib & tanpa --non-interactive,\n" +
			"wizard interaktif (huh) dijalankan. Dengan --name (atau --non-interactive),\n" +
			"jalur flag dipakai — output byte-identical dengan wizard (SPEC §5.2).\n\n" +
			"Subset Fase 4a:\n" +
			"  --arch  monolith | modular-monolith\n" +
			"  --http  net/http | chi | echo\n" +
			"  --db    none | postgres | mysql\n" +
			"  --addons/--feature  docker,makefile,golangci,env,ci,observability\n" +
			"  --ci    github-actions | gitlab-ci (provider CI bila addon 'ci' aktif)\n" +
			"  --config <file.yaml>  preset jawaban (precedence: default < preset < flag).\n" +
			"(microservice & gin/fiber & sqlite/mongo menyusul di fase berikutnya.)",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCreate(cmd, f)
		},
	}

	fl := cmd.Flags()
	fl.StringVar(&f.name, "name", "", "nama project (^[a-z][a-z0-9-]*$); wajib di mode non-interaktif")
	fl.StringVar(&f.module, "module", "", "Go module path (default github.com/<name>)")
	fl.StringVar(&f.arch, "arch", string(answers.ArchMonolith), "arsitektur: monolith|modular-monolith|microservice")
	fl.StringVar(&f.kind, "kind", string(answers.KindREST), "jenis aplikasi (Fase 4a: rest)")

	// Microservice (arch=microservice). --service repeatable + --services csv
	// digabung; --comm grpc default (rest/event ditolak ramah v1); gateway opsional.
	fl.StringArrayVar(&f.services, "service", nil, "nama service (repeatable; microservice). Contoh: --service order --service user")
	fl.StringSliceVar(&f.servicesCSV, "services", nil, "daftar service csv (microservice). Contoh: --services order,user")
	fl.StringVar(&f.comm, "comm", "", "pola komunikasi microservice: grpc (default v1). rest/event menyusul")
	fl.BoolVar(&f.gateway, "gateway", false, "aktifkan API gateway (REST edge → gRPC) untuk microservice")
	fl.BoolVar(&f.noGateway, "no-gateway", false, "nonaktifkan gateway (default; menang atas --gateway)")
	fl.StringVar(&f.http, "http", string(answers.HTTPNetHTTP), "HTTP framework: net/http|chi|echo")
	fl.StringVar(&f.db, "db", string(answers.DBNone), "database: none|postgres|mysql")
	fl.StringVar(&f.access, "access", "", "lapisan akses query (Fase 4a: sqlx; butuh --db≠none, C2)")
	fl.StringVar(&f.migrate, "migrate", "", "tool migrasi (Fase 4a: golang-migrate; butuh --db∉{none}, C1)")
	fl.StringVar(&f.ci, "ci", "", "provider CI bila addon 'ci' aktif: github-actions|gitlab-ci (default github-actions)")
	fl.StringSliceVar(&f.addons, "addons", nil, "add-ons csv: docker,makefile,golangci,env,ci,observability")
	// --feature di-MERGE dengan --addons (union, dedup) — bukan alias last-wins (M-2).
	// SPEC menyebut --feature/--addons; keduanya menulis ke slice terpisah lalu
	// digabung di buildAnswers agar tak saling menimpa.
	fl.StringSliceVar(&f.features, "feature", nil, "add-on tambahan (digabung union dengan --addons)")

	fl.BoolVar(&f.git, "git", false, "jalankan git init + initial commit")
	fl.BoolVar(&f.noGit, "no-git", false, "jangan jalankan git init (default non-interaktif)")

	fl.StringVar(&f.config, "config", "", "preset jawaban dari file YAML (precedence: default < preset < flag eksplisit)")

	fl.BoolVar(&f.dryRun, "dry-run", false, "cetak rencana tanpa menulis ke disk (SPEC §5.4)")
	fl.StringVarP(&f.output, "output", "o", "", "direktori output (default: ./<name>)")
	fl.BoolVar(&f.yes, "yes", false, "lewati konfirmasi (anggap semua jawaban diterima)")
	fl.BoolVar(&f.nonInteractive, "non-interactive", false, "paksa mode non-interaktif (flag-only)")

	return cmd
}

// runCreate menjalankan orchestration create.
func runCreate(cmd *cobra.Command, f *createFlags) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	out := cmd.OutOrStdout()

	// 0. Preset --config (SPEC §5.4): muat YAML lalu overlay ke flag mentah HANYA
	// untuk field yang tidak di-set eksplisit (precedence default < preset < flag).
	// Memakai cobra Flags().Changed sebagai sumber kebenaran "flag eksplisit".
	if f.config != "" {
		p, err := loadPreset(f.config)
		if err != nil {
			return err
		}
		mapPresetToFlags(p, f, cmd.Flags().Changed)
	}

	// 1. Tentukan jalur input: flag (non-interaktif) atau wizard (interaktif).
	a, err := buildAnswers(ctx, cmd, f)
	if err != nil {
		return err
	}

	// 2. Validasi field-level (SPEC §4 — regex name, module path, enum). Microservice
	// (Fase 4b) kini mengalir lewat jalur TERPADU yang sama dengan monolith/modular:
	// Validate (q_svc berlaku — M-3 hilang) → Resolve (arch-microservice + per-service
	// DataOverride + hook buf-generate) → EnsureEmptyDir → Generate → hooks. Tidak ada
	// cabang dini ke island internal/cli/micro lagi.
	if err := a.Validate(); err != nil {
		return fmt.Errorf("validasi input gagal: %w", err)
	}

	// 3. Resolusi default + constraint matrix → GeneratePlan (SPEC §6).
	reg := module.NewRegistry()
	if err := reg.Load(templates.FS); err != nil {
		return fmt.Errorf("memuat katalog template gagal: %w", err)
	}
	res := resolver.New(reg)
	p, err := res.Resolve(a)
	if err != nil {
		// ErrConstraint hard-invalid → fail-fast (SPEC §6.4 / DoD #6).
		return fmt.Errorf("resolusi/constraint gagal: %w", err)
	}

	// 4. Tentukan direktori target.
	target := f.output
	if target == "" {
		target = p.ProjectName
		if target == "" {
			target = a.Name
		}
	}
	target = filepath.Clean(target)

	// 5. Pilih writer: DryRunWriter (preview) atau RealWriter (tulis).
	if f.dryRun {
		dw := &fsutil.DryRunWriter{}
		if err := runGenerate(reg, p, target, dw); err != nil {
			return err
		}
		printDryRun(out, p, target, dw)
		return nil
	}

	// 6. Generate ke disk. Proteksi overwrite (EnsureEmptyDir) dilakukan SATU KALI
	// di dalam generator.Generate (otoritas tunggal, M-1) — tidak diduplikasi di
	// sini agar tak ada cek ganda.
	if err := runGenerate(reg, p, target, fsutil.RealWriter{}); err != nil {
		return err
	}

	// 7. Post-gen hooks (Gofmt → GoModTidy fail-fast → GitInit warn-only).
	if err := runHooks(ctx, out, target, p); err != nil {
		return err
	}

	// 8. Ringkasan.
	printSummary(out, p, target)
	return nil
}

// buildAnswers memilih jalur input dan mengembalikan answers.Answers terisi.
//
// Aturan (ADR-002 §2 langkah 1): bila --non-interactive, --config (preset), ATAU
// semua field wajib (name) sudah terisi via flag/preset → jalur flag; selain itu
// → wizard huh. --config selalu non-interaktif (SPEC §5.4).
func buildAnswers(ctx context.Context, cmd *cobra.Command, f *createFlags) (answers.Answers, error) {
	useFlags := f.nonInteractive || f.config != "" || f.name != ""
	if !useFlags {
		// Mode interaktif: jalankan wizard. Default git interaktif = yes (SPEC §4.9).
		pr := &prompt.HuhPrompter{DefaultGit: true}
		return pr.Ask(ctx)
	}

	// Mode non-interaktif: --name wajib (SPEC §5.1 / US-02 Sk.3).
	if f.name == "" {
		return answers.Answers{}, fmt.Errorf("--name wajib di mode non-interaktif (mis. --name my-app); atau jalankan tanpa --non-interactive/--config untuk wizard interaktif")
	}
	return flagsToAnswers(cmd, f), nil
}

// flagsToAnswers memetakan flag mentah → answers.Answers (jalur non-interaktif).
// Resolusi default antar-opsi (mis. access/migrate dari db) dilakukan resolver;
// di sini hanya pemetaan langsung + git precedence.
func flagsToAnswers(cmd *cobra.Command, f *createFlags) answers.Answers {
	mod := f.module
	if mod == "" {
		mod = "github.com/" + f.name
	}

	// M-2: --addons, --feature, dan add-on dari preset (--config) digabung secara
	// UNION (dedup, urut stabil) — bukan last-wins. Ketiganya menulis ke slice
	// terpisah; di sini disatukan menjadi satu himpunan.
	addonSet := make(map[string]bool, len(f.addons)+len(f.features)+len(f.presetAddons))
	for _, a := range mergeAddons(f.addons, f.features, f.presetAddons) {
		addonSet[a] = true
	}

	// git precedence: --no-git menang atas --git; default non-interaktif = false
	// (SPEC §5.1 / §6.3). --git men-set true, lalu --no-git meng-override ke false.
	git := f.git
	if f.noGit {
		git = false
	}

	a := answers.Answers{
		Name:    f.name,
		Module:  mod,
		Arch:    answers.Arch(f.arch),
		Kind:    answers.Kind(f.kind),
		HTTP:    answers.HTTPFramework(f.http),
		DB:      answers.DB(f.db),
		Access:  answers.Access(f.access),
		Migrate: answers.Migrate(f.migrate),

		// Microservice (arch=microservice). --service + --services digabung UNION
		// dengan URUTAN DIPERTAHANKAN (service pertama = pemanggil; order penting,
		// jadi BUKAN mergeAddons yang sort alfabet). Comm/Gateway dipetakan apa
		// adanya; resolusi default (grpc, gateway implisit rest) ada di
		// runCreateMicroservice (cabang khusus, bukan resolver utama).
		Services: toServiceList(mergeServices(f.services, f.servicesCSV)),
		Comm:     answers.Comm(f.comm),
		Gateway:  resolveGatewayFlag(f.gateway, f.noGateway),

		Docker: addonSet["docker"],
		// DockerSet: user membuat keputusan EKSPLISIT soal docker bila ia menyentuh
		// --addons / --feature (apa pun isinya) atau preset --config menyebut add-on
		// (presetAddons terisi). Bila docker tak masuk daftar yang dikurasi user,
		// resolver TIDAK boleh memaksakan default db→docker (SPEC §6.2). Tanpa
		// sentuhan flag add-on sama sekali → DockerSet=false → resolver menerapkan
		// default deterministik (docker=true bila db∈{postgres,mysql}).
		DockerSet: addonSet["docker"] ||
			cmd.Flags().Changed("addons") ||
			cmd.Flags().Changed("feature") ||
			len(f.presetAddons) > 0,
		Makefile:   addonSet["makefile"],
		Lint:       addonSet["golangci"],
		EnvExample: addonSet["env"],
		// observability add-on (otel + /metrics + health) → Obs (SPEC §4.8/§5.1 --obs).
		Obs: addonSet["observability"],

		Git: git,

		// Opsi terkunci default (SPEC §6.2).
		ConfigLoader:  answers.ConfigLoaderGodotenv,
		Log:           answers.LogSlog,
		ValidateInput: f.kind != string(answers.KindWorker),
	}

	// CI provider: add-on "ci" mengaktifkan CI; provider berasal dari --ci.
	// Default provider = github-actions bila addon ci aktif tetapi --ci kosong
	// (SPEC §5.1/§6.2 default --ci=github-actions). Bila addon ci TIDAK aktif,
	// CI = none (tetap konsisten dengan resolver yang men-default-kan kosong→none).
	a.CI = resolveCI(addonSet["ci"], f.ci)

	// access/migrate: bila flag eksplisit diberikan, hormati apa adanya agar
	// constraint C1/C2 (mis. --migrate tanpa --db) terdeteksi resolver. Bila kosong
	// DAN db terpilih (≠none), resolver yang men-default-kan ke sqlx + golang-migrate
	// (applyDefaults). Tidak memaksa di sini — biarkan resolver jadi satu-satunya
	// otak keputusan default+constraint (ADR-002 §2).
	return a
}

// resolveCI menerjemahkan (addon "ci" aktif?, nilai --ci) → answers.CI.
//
// Aturan (SPEC §5.1/§6.2):
//   - addon ci tidak aktif         → CINone (apa pun --ci; modul CI HANYA
//     diaktifkan oleh add-on 'ci', BUKAN oleh --ci sendirian).
//   - addon ci aktif, --ci diisi   → nilai apa adanya (github-actions|gitlab-ci|
//     none) agar validasi enum dilakukan answers.Validate (otoritas tunggal).
//   - addon ci aktif, --ci kosong  → CIGitHubActions (default provider).
//
// Urutan cek penting: ciAddonActive di-gate DULU agar `--ci <provider>` tanpa
// `--addons ci` tidak diam-diam mengaktifkan modul CI (resolver meng-aktif-kan
// addon-ci dari nilai CI; CINone menjaganya tetap mati). Tidak memvalidasi enum
// di sini — biarkan answers.Validate menolak nilai asing dengan pesan ramah
// (single source of truth, ADR-002 §2).
func resolveCI(ciAddonActive bool, ciFlag string) answers.CI {
	if !ciAddonActive {
		return answers.CINone
	}
	if ciFlag != "" {
		return answers.CI(ciFlag)
	}
	return answers.CIGitHubActions
}

// ── Microservice (arch=microservice, Fase 4b) ────────────────────────────────

// mergeServices menggabungkan beberapa sumber nama service (--service repeatable
// + --services csv) menjadi satu daftar UNION dengan URUTAN INPUT DIPERTAHANKAN
// (BUKAN sort alfabet — order service load-bearing: service pertama = pemanggil,
// US-04 Sk.2). Nilai di-trim & di-dedup; kosong diabaikan.
func mergeServices(sources ...[]string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, src := range sources {
		for _, n := range src {
			n = strings.TrimSpace(n)
			if n == "" || seen[n] {
				continue
			}
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// toServiceList memetakan []string nama → []answers.Service (urutan dipertahankan).
func toServiceList(names []string) []answers.Service {
	if len(names) == 0 {
		return nil
	}
	out := make([]answers.Service, len(names))
	for i, n := range names {
		out[i] = answers.Service{Name: n}
	}
	return out
}

// resolveGatewayFlag menerjemahkan (--gateway, --no-gateway) → bool. --no-gateway
// menang atas --gateway; default false (SPEC §5.1 q_gateway default no).
func resolveGatewayFlag(gateway, noGateway bool) bool {
	if noGateway {
		return false
	}
	return gateway
}

// mergeAddons menggabungkan beberapa slice add-on (--addons, --feature, dan
// add-on preset --config) menjadi satu daftar UNION: nilai di-trim, di-dedup,
// lalu diurut stabil (M-2). Urutan stabil (sort) menjamin determinisme tanpa
// bergantung urutan flag/sumber — selaras invarian byte-identical (SPEC §5.2).
// Nilai kosong diabaikan.
func mergeAddons(sources ...[]string) []string {
	total := 0
	for _, s := range sources {
		total += len(s)
	}
	seen := make(map[string]bool, total)
	var out []string
	for _, src := range sources {
		for _, a := range src {
			a = strings.TrimSpace(a)
			if a == "" || seen[a] {
				continue
			}
			seen[a] = true
			out = append(out, a)
		}
	}
	sort.Strings(out)
	return out
}

// runGenerate merakit generator kanonik (renderer + merge-assembler atas
// templates.FS) dan mengeksekusi GeneratePlan ke target via Writer.
func runGenerate(reg module.Registry, p plan.GeneratePlan, target string, w fsutil.Writer) error {
	g := generator.New(reg, generator.NewRenderer(templates.FS), generator.NewMergeAssembler())
	if err := g.Generate(p, target, w); err != nil {
		return fmt.Errorf("generate gagal: %w", err)
	}
	return nil
}

// runHooks menerjemahkan plan.Hooks → hooks.Plan dan menjalankannya pada target.
func runHooks(ctx context.Context, out io.Writer, target string, p plan.GeneratePlan) error {
	specs := make([]hooks.Spec, 0, len(p.Hooks))
	for _, h := range p.Hooks {
		specs = append(specs, hooks.Spec{Name: h.Name, Order: h.Order})
	}
	plans := hooks.Build(specs)
	warnf := func(format string, args ...any) {
		fmt.Fprintf(out, format+"\n", args...)
	}
	return hooks.Run(ctx, target, plans, warnf)
}

// printSummary mencetak ringkasan singkat hasil generate (RealWriter).
func printSummary(out io.Writer, p plan.GeneratePlan, target string) {
	fmt.Fprintf(out, "\n✔ Project '%s' dibuat di %s\n", p.ProjectName, target)
	fmt.Fprintf(out, "  module: %s\n", p.ModulePath)
	fmt.Fprintf(out, "  go:     %s\n", p.GoVersion)
	if len(p.Deps) > 0 {
		fmt.Fprintf(out, "  deps:   %d dependency\n", len(p.Deps))
	}
	fmt.Fprintf(out, "\nLangkah berikutnya:\n  cd %s && go build ./...\n", target)
}

// printDryRun mencetak rencana generate (tree file + daftar deps) tanpa menulis
// (SPEC §5.4 / US-06). Path diambil dari DryRunWriter.Planned (urut sesuai plan).
func printDryRun(out io.Writer, p plan.GeneratePlan, target string, dw *fsutil.DryRunWriter) {
	fmt.Fprintf(out, "[dry-run] rencana untuk project '%s' (module %s, go %s):\n\n", p.ProjectName, p.ModulePath, p.GoVersion)

	paths := append([]string(nil), dw.Planned...)
	sort.Strings(paths)
	fmt.Fprintf(out, "%s/\n", target)
	for _, pth := range paths {
		fmt.Fprintf(out, "  %s\n", pth)
	}

	if len(p.Deps) > 0 {
		fmt.Fprintln(out, "\ngo.mod require:")
		deps := append([]plan.ModuleDep(nil), p.Deps...)
		sort.Slice(deps, func(i, j int) bool { return deps[i].Path < deps[j].Path })
		for _, d := range deps {
			fmt.Fprintf(out, "  %s %s\n", d.Path, d.Version)
		}
	}
	fmt.Fprintln(out, "\n(tidak ada file ditulis — --dry-run)")
}
