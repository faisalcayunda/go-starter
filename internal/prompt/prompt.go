// Package prompt mendefinisikan dan mengimplementasikan kontrak wizard
// interaktif builder (charmbracelet/huh).
//
// Jalur flag (cobra) menulis ke struct yang sama (answers.Answers) sehingga
// output mode interaktif byte-identical dengan jalur non-interaktif (SPEC §5.2).
//
// SCOPE FASE 4a (scope-lock): wizard HANYA menawarkan opsi yang sudah
// diimplementasi end-to-end agar tidak menghasilkan project rusak —
//
//	arch   ∈ {monolith (default), modular-monolith}   (microservice = Fase 4b)
//	kind   = rest      (info-only; tetap satu nilai)
//	http   ∈ {net/http (default), chi, echo}          (gin/fiber menyusul)
//	db     ∈ {none (default), postgres, mysql}         (sqlite/mongo menyusul)
//	addons ⊆ {docker, makefile, golangci, env, ci, observability}
//	         (README.md & .gitignore SELALU dari core — bukan add-on, C-1)
//	ci-provider ∈ {github-actions (default), gitlab-ci}  (muncul bila addon ci dipilih)
//	git    = confirm (default mengikuti pemanggil)
//
// Opsi di luar subset (gin/fiber, sqlite/mongo, microservice) TIDAK ditawarkan di
// wizard pada Fase 4a (menyusul Fase 4b).
package prompt

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"golang.org/x/mod/module"

	"github.com/faisalcayunda/gostarter/internal/answers"
)

// nameRE adalah regex validasi nama project (SPEC §4.2): huruf kecil, angka,
// dash; diawali huruf.
var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Prompter menjalankan wizard interaktif dan mengumpulkan jawaban user ke dalam
// answers.Answers.
type Prompter interface {
	Ask(ctx context.Context) (answers.Answers, error)
}

// Nilai kanonik add-on multiselect (selaras flag --addons / SPEC §4.8).
//
// Add-on Fase 4a yang sah = docker, makefile, golangci, env, ci, observability.
// README.md & .gitignore BUKAN add-on: keduanya deliverable baseline yang SELALU
// dimiliki modul `core` (C-1) — bukan opsi yang bisa dimatikan.
const (
	addonDocker   = "docker"
	addonMakefile = "makefile"
	addonGolangci = "golangci"
	addonEnv      = "env"
	addonCI       = "ci"            // mengaktifkan CI; provider dipilih terpisah
	addonObs      = "observability" // otel tracing + prometheus /metrics + health
)

// HuhPrompter adalah implementasi Prompter berbasis charmbracelet/huh.
//
// Default boleh diisi pemanggil (mis. dari nilai flag yang sudah ter-set) agar
// wizard memulai dari kondisi yang konsisten. Nilai nol HuhPrompter tetap valid.
type HuhPrompter struct {
	// DefaultGit mengisi nilai awal konfirmasi git (mode interaktif default yes,
	// SPEC §4.9). Pemanggil dari cmd menyetelnya.
	DefaultGit bool
}

// New mengembalikan Prompter huh dengan default interaktif (git=yes, SPEC §4.9).
func New() Prompter {
	return &HuhPrompter{DefaultGit: true}
}

// Ask menjalankan wizard huh dan mengembalikan answers.Answers terisi.
//
// Field di luar subset Fase 3 di-set ke nilai default terkunci (lihat akhir
// fungsi) sehingga resolver menerima Answers yang lengkap & konsisten.
func (p *HuhPrompter) Ask(ctx context.Context) (answers.Answers, error) {
	var (
		name       string
		mod        string
		arch       = string(answers.ArchMonolith)
		httpFw     = string(answers.HTTPNetHTTP)
		db         = string(answers.DBNone)
		access     = string(answers.AccessSQLx)
		git        = p.DefaultGit
		addons     = []string{addonMakefile, addonGolangci, addonEnv, addonCI}
		ciProvider = string(answers.CIGitHubActions)

		// Microservice (q_svc, muncul bila arch=microservice). comm dikunci grpc v1
		// (rest/event ditandai "menyusul"); gateway opsional (default off).
		svcList = "order,user"
		comm    = string(answers.CommGRPC)
		gateway = false
	)

	form := huh.NewForm(
		// Grup 1 — identitas project (q_name, SPEC §4.2).
		huh.NewGroup(
			huh.NewInput().
				Title("Nama project?").
				Description("huruf kecil, angka, dash; diawali huruf (mis. shop)").
				Value(&name).
				Validate(validateName),
			huh.NewInput().
				Title("Go module path?").
				Description("default github.com/<name>; boleh diedit").
				Value(&mod).
				Validate(validateModuleOptional),
		),
		// Grup 2 — arsitektur (q_arch, SPEC §4.3). monolith | modular-monolith |
		// microservice (Fase 4b: monorepo gRPC).
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Tipe arsitektur?").
				Options(
					huh.NewOption("Monolith (layered, satu internal/)", string(answers.ArchMonolith)),
					huh.NewOption("Modular monolith (domain ber-internal/ sendiri)", string(answers.ArchModularMonolith)),
					huh.NewOption("Microservice (monorepo gRPC, N service)", string(answers.ArchMicroservice)),
				).
				Value(&arch),
			huh.NewNote().
				Title("Catatan").
				Description("kind = rest (terkunci untuk monolith) · microservice = monorepo single-module gRPC"),
		),
		// Grup q_svc — daftar service + komunikasi + gateway. HANYA muncul saat
		// arch=microservice (WithHideFunc dievaluasi saat navigasi).
		huh.NewGroup(
			huh.NewInput().
				Title("Daftar service awal (pisah koma; minimal 1):").
				Description("contoh: order,user — service pertama memanggil yang kedua via gRPC").
				Value(&svcList).
				Validate(validateServiceList),
			huh.NewSelect[string]().
				Title("Pola komunikasi antar service?").
				Options(
					huh.NewOption("gRPC (default v1)", string(answers.CommGRPC)),
					huh.NewOption("REST-comm (v1: gRPC — menyusul)", string(answers.CommREST)),
					huh.NewOption("Event-driven (v1: gRPC — menyusul)", string(answers.CommEvent)),
				).
				Value(&comm).
				Validate(validateCommGRPCOnly),
			huh.NewConfirm().
				Title("Aktifkan API gateway (REST edge → gRPC)?").
				Value(&gateway),
		).WithHideFunc(func() bool {
			return arch != string(answers.ArchMicroservice)
		}),
		// Grup 3 — HTTP framework (q_http, SPEC §4.6). Subset Fase 4a:
		// net/http (default) | chi | echo. gin/fiber menyusul. Disembunyikan untuk
		// microservice (v1 micro memakai gRPC; HTTP framework tak relevan).
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("HTTP framework?").
				Options(
					huh.NewOption("net/http (stdlib, zero dependency)", string(answers.HTTPNetHTTP)),
					huh.NewOption("chi (go-chi/chi v5)", string(answers.HTTPChi)),
					huh.NewOption("echo (labstack/echo v4)", string(answers.HTTPEcho)),
				).
				Value(&httpFw),
		).WithHideFunc(func() bool { return arch == string(answers.ArchMicroservice) }),
		// Grup 4 — database (q_db, SPEC §4.7). Subset Fase 4a: none | postgres | mysql.
		// Disembunyikan untuk microservice (v1 micro skeleton db=none — murni gRPC).
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Pakai database?").
				Options(
					huh.NewOption("Tidak (none) — murni stdlib", string(answers.DBNone)),
					huh.NewOption("PostgreSQL (pgx/v5 + golang-migrate)", string(answers.DBPostgres)),
					huh.NewOption("MySQL (go-sql-driver/mysql + golang-migrate)", string(answers.DBMySQL)),
				).
				Value(&db),
		).WithHideFunc(func() bool { return arch == string(answers.ArchMicroservice) }),
		// Grup q_access — lapisan akses query. HANYA muncul bila db∈{postgres,mysql}
		// (C2; access tak relevan db=none) DAN bukan microservice. gorm mengaktifkan
		// jalur GORM (koneksi gorm.Open + model + repository) menggantikan koneksi
		// sqlx/pgxpool/database-sql default.
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Lapisan akses query?").
				Options(
					huh.NewOption("sqlx (jmoiron/sqlx — default)", string(answers.AccessSQLx)),
					huh.NewOption("database/sql (stdlib)", string(answers.AccessDatabaseSQL)),
					huh.NewOption("GORM (gorm.io/gorm + driver)", string(answers.AccessGORM)),
				).
				Value(&access),
		).WithHideFunc(func() bool {
			return arch == string(answers.ArchMicroservice) ||
				(db != string(answers.DBPostgres) && db != string(answers.DBMySQL))
		}),
		// Grup 5 — add-ons multiselect (subset Fase 4a). Disembunyikan untuk
		// microservice (v1 micro menyediakan docker/makefile/compose bawaan).
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Add-ons (pilih banyak):").
				Options(
					huh.NewOption("Docker + compose", addonDocker),
					huh.NewOption("Makefile", addonMakefile),
					huh.NewOption("golangci-lint", addonGolangci),
					huh.NewOption(".env.example", addonEnv),
					huh.NewOption("CI (lint+test+build)", addonCI),
					huh.NewOption("Observability (otel + /metrics + health)", addonObs),
				).
				Value(&addons),
		).WithHideFunc(func() bool { return arch == string(answers.ArchMicroservice) }),
		// Grup 6 — provider CI (q_addons[ci], SPEC §4.8). Hanya muncul bila add-on
		// "ci" dipilih (depends-on dinamis via WithHideFunc — dievaluasi saat navigasi)
		// DAN bukan microservice.
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Provider CI?").
				Options(
					huh.NewOption("GitHub Actions (.github/workflows/ci.yml)", string(answers.CIGitHubActions)),
					huh.NewOption("GitLab CI (.gitlab-ci.yml)", string(answers.CIGitLabCI)),
				).
				Value(&ciProvider),
		).WithHideFunc(func() bool {
			// Sembunyikan bila microservice ATAU "ci" TIDAK ada di pilihan add-on.
			return arch == string(answers.ArchMicroservice) || !containsString(addons, addonCI)
		}),
		// Grup 7 — git (q_git, SPEC §4.9).
		huh.NewGroup(
			huh.NewConfirm().
				Title("Jalankan `git init` + initial commit?").
				Value(&git),
		),
	)

	if err := form.RunWithContext(ctx); err != nil {
		return answers.Answers{}, fmt.Errorf("wizard dibatalkan: %w", err)
	}

	name = strings.TrimSpace(name)
	mod = strings.TrimSpace(mod)
	if mod == "" {
		mod = "github.com/" + name
	}

	// CABANG MICROSERVICE: bila arch=microservice, Answers difokuskan ke q_svc
	// (Services/Comm/Gateway). Field monolith (HTTP/DB/addons) tak relevan untuk
	// arch microservice dan dibiarkan zero — jalur generate TERPADU (resolver→
	// generator, modul arch-microservice) menanganinya. comm dikunci grpc v1.
	if arch == string(answers.ArchMicroservice) {
		a := answers.Answers{
			Name:     name,
			Module:   mod,
			Arch:     answers.ArchMicroservice,
			Services: parseServiceList(svcList),
			Comm:     answers.Comm(comm),
			Gateway:  gateway,
			Git:      git,
		}
		return a, nil
	}

	addonSet := make(map[string]bool, len(addons))
	for _, a := range addons {
		addonSet[a] = true
	}

	a := answers.Answers{
		Name:   name,
		Module: mod,

		// Arsitektur & HTTP (subset Fase 4a, dipilih user).
		Arch: answers.Arch(arch),
		Kind: answers.KindREST, // terkunci Fase 4a
		HTTP: answers.HTTPFramework(httpFw),

		// Database subset.
		DB: answers.DB(db),

		// Add-ons subset.
		Docker:     addonSet[addonDocker],
		Makefile:   addonSet[addonMakefile],
		Lint:       addonSet[addonGolangci],
		EnvExample: addonSet[addonEnv],
		Obs:        addonSet[addonObs],

		// Git.
		Git: git,

		// Opsi terkunci default (SPEC §6.2) — tetap diisi agar Answers konsisten.
		ConfigLoader: answers.ConfigLoaderGodotenv,
		Log:          answers.LogSlog,
		// validate on untuk kind=rest (SPEC §6.2).
		ValidateInput: true,
	}

	// CI: bila add-on "ci" aktif → provider dari pilihan user (default
	// github-actions). Bila tidak aktif → none. Selaras jalur flag (resolveCI).
	if addonSet[addonCI] {
		a.CI = answers.CI(ciProvider)
	} else {
		a.CI = answers.CINone
	}

	// access/migrate hanya relevan bila db∈{postgres,mysql} (resolver menegakkan;
	// di sini diisi default agar konsisten dengan jalur flag — byte-identical §5.2).
	if a.DB == answers.DBPostgres || a.DB == answers.DBMySQL {
		// access dipilih user di grup q_access (default sqlx bila tak disentuh).
		a.Access = answers.Access(access)
		a.Migrate = answers.MigrateGolangMigrate
		// docker default true bila db≠none (SPEC §5.1) — hanya bila user belum
		// secara eksplisit mematikannya; di multiselect, ketiadaan = false, jadi
		// hormati pilihan user apa adanya (tidak memaksa).
	}

	return a, nil
}

// containsString mengembalikan true bila slice memuat target (helper kecil untuk
// HideFunc CI; menghindari import "slices" demi kompatibilitas minimal).
func containsString(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

// svcNameRE memvalidasi satu nama service (SPEC §4.5): huruf kecil di awal,
// lalu huruf kecil/angka/dash.
var svcNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// parseServiceList memecah input "order, user" → ["order","user"] (trim, buang
// kosong, dedup dengan URUTAN DIPERTAHANKAN — service pertama = pemanggil).
func parseServiceList(s string) []answers.Service {
	seen := make(map[string]bool)
	var out []answers.Service
	for _, part := range strings.Split(s, ",") {
		n := strings.TrimSpace(part)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, answers.Service{Name: n})
	}
	return out
}

// validateServiceList menegakkan: ≥1 service, tiap nama cocok regex, unik, dan
// bukan reserved "gateway" (SPEC §4.5 q_svc_list).
func validateServiceList(s string) error {
	parts := strings.Split(s, ",")
	seen := make(map[string]bool)
	count := 0
	for _, part := range parts {
		n := strings.TrimSpace(part)
		if n == "" {
			continue
		}
		if !svcNameRE.MatchString(n) {
			return fmt.Errorf("nama service %q tidak valid: huruf kecil, angka, dash; awali huruf", n)
		}
		if n == "gateway" {
			return fmt.Errorf("nama service %q dilarang: 'gateway' adalah reserved word", n)
		}
		if seen[n] {
			return fmt.Errorf("nama service %q duplikat", n)
		}
		seen[n] = true
		count++
	}
	if count < 1 {
		return errors.New("minimal 1 service (mis. order,user)")
	}
	return nil
}

// validateCommGRPCOnly menolak ramah comm selain grpc (v1: hanya gRPC; REST-comm
// & event-driven menyusul). Mengembalikan error agar wizard tidak melanjutkan
// dengan pilihan yang belum diimplementasi.
func validateCommGRPCOnly(s string) error {
	switch answers.Comm(s) {
	case answers.CommGRPC:
		return nil
	case answers.CommREST:
		return errors.New("REST-comm belum tersedia di v1 — pilih gRPC (REST-comm menyusul)")
	case answers.CommEvent:
		return errors.New("event-driven belum tersedia di v1 — pilih gRPC (event-driven menyusul)")
	default:
		return fmt.Errorf("komunikasi %q tidak dikenal: pilih gRPC", s)
	}
}

// validateName menegakkan regex SPEC §4.2: ^[a-z][a-z0-9-]*$, non-empty.
func validateName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("nama project wajib diisi")
	}
	if !nameRE.MatchString(s) {
		return errors.New("nama tidak valid: gunakan huruf kecil, angka, dash; awali huruf (mis. shop)")
	}
	return nil
}

// validateModuleOptional memvalidasi module path bila diisi; kosong dibolehkan
// (akan diturunkan dari name oleh pemanggil).
func validateModuleOptional(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if err := module.CheckPath(s); err != nil {
		return fmt.Errorf("module path tidak valid: %v", err)
	}
	return nil
}
