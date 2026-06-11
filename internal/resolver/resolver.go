// Package resolver menerjemahkan answers.Answers menjadi plan.GeneratePlan
// dengan menerapkan default resolution + constraint matrix (SPEC §6).
//
// FASE 3 (scope MVP — monolith / rest / net/http / db ∈ {none, postgres}):
// resolver memilih modul aktif dari Registry, menegakkan constraint yang relevan
// (C1 migrate↔db, C2 access↔db, C14 compose service hanya bila db≠none), merakit
// []FileOp (Mode render/copy/mkdir/merge) dengan urutan deterministik, mengumpulkan
// plan.Deps (dedup+sort by Path) dari gomod[] modul aktif, dan plan.Hooks.
//
// FASE 4a (perluasan subset): resolver kini mengaktifkan modul:
//   - arch-modular  menggantikan arch-monolith bila arch=modular-monolith
//     (tepat satu modul arch-* aktif; mutual conflicts via manifest);
//   - http-chi / http-echo menggantikan routing net/http core bila dipilih.
//     Dedup file routing default dilakukan oleh `when` pada manifest core
//     (server net/http aktif hanya saat http=net/http) — resolver hanya
//     mengaktifkan modul http-* yang relevan, tidak meng-emit file yatim;
//   - db-mysql  analog db-postgres bila db=mysql;
//   - addon-ci  bila CI ∈ {github-actions, gitlab-ci} (provider memilih file
//     .github/workflows/ci.yml ATAU .gitlab-ci.yml via `when` di manifest);
//   - addon-observability  bila Obs=true (otel + prometheus /metrics + health).
//
// FASE 4b (arch=microservice): resolusi BERBEDA dari monolith/modular. Layout
// monorepo single-module (riset 02): file ROOT (go.mod/Makefile/buf/compose/
// README/gitignore) di-emit SEKALI dari modul arch-microservice; lalu untuk
// SETIAP service di Answers.Services di-emit set file PER-SERVICE (proto/<svc>.proto,
// services/<svc>/cmd/main.go, services/<svc>/internal/...) dengan me-render ULANG
// template per-service yang sama memakai plan.FileOp.DataOverride (Service, IsFirst,
// Others, ModulePath). Tiap service juga MENYUMBANG fragmen ke anchor
// region:services pada docker-compose.yml root. Hook "buf-generate" (order 5)
// ditambahkan agar gen/go/** di-COMMIT saat builder generate (project "go build
// ./..." HIJAU tanpa buf). Determinisme dijaga: Services di-sort by Name, FileOp
// di-sort by TargetPath. addon-docker compose monolith di-GATE ke non-microservice
// agar tak bentrok dengan compose milik arch-microservice.
//
// Otak keputusan terpusat di sini (ADR-002 §Decision 2): template tidak pernah
// mengambil keputusan struktural. ErrConstraint dipakai untuk kombinasi hard-invalid
// (fail-fast, SPEC §6.4 / DoD #6).
package resolver

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"text/template"

	"github.com/faisalcayunda/gostarter/internal/answers"
	"github.com/faisalcayunda/gostarter/internal/modpath"
	"github.com/faisalcayunda/gostarter/internal/module"
	"github.com/faisalcayunda/gostarter/internal/plan"
)

// ErrConstraint menandai kombinasi opsi yang melanggar constraint matrix SPEC §6.1
// (hard-invalid). Dibungkus dengan pesan yang menyebut flag bermasalah.
var ErrConstraint = errors.New("constraint")

// goVersionDefault adalah go directive default untuk project hasil generate
// (ADR-002/ADR-003: vars GoVersion "1.24"; fiber → 1.25 di Fase 4).
const goVersionDefault = "1.24"

// goVersionMicroservice adalah go directive untuk arch=microservice. Disetel 1.25
// (BUKAN 1.24 seperti monolith) karena google.golang.org/grpc v1.81.1 mensyaratkan
// go ≥ 1.25.0 — `go mod tidy` akan menaikkannya otomatis, jadi menyetel 1.25 sejak
// awal menjaga go.mod, Dockerfile (FROM golang:{{ .GoVersion }}), dan README
// konsisten (Docker build memakai image 1.25 yang mampu mem-build modul gRPC ini).
const goVersionMicroservice = "1.25"

// Alokasi port deterministik per-service (byte-identical, SPEC §5.2 / L-3). Port
// diturunkan dari INDEX service pada urutan sortedServiceNames (BUKAN hitung jumlah
// direktori saat runtime): gRPC service ke-i = grpcPortBase+i; HTTP/gateway demo
// service pertama = httpGatewayBase (+i bila kelak >1 service ber-HTTP).
const (
	grpcPortBase    = 50051
	httpGatewayBase = 8081
)

// Nama modul template kanonik (ADR-002 §Decision 1.1).
//
// Catatan arsitektur HTTP: server net/http default (server, handler contoh,
// wiring) dimiliki bersama oleh `core` + modul arch-*. Untuk chi/echo, modul
// http-chi / http-echo menyumbang/menggantikan routing; file routing net/http
// default core di-gate via `when` (eq .HTTP "net/http") di manifest core agar
// TIDAK ter-emit ganda — resolver hanya memutuskan modul mana yang AKTIF, bukan
// meng-emit file yatim (pelajaran Fase 3: hindari double-wiring & file yatim).
const (
	modCore     = "core"
	modArchMono = "arch-monolith"
	modArchMod  = "arch-modular"
	// modArchMicro memiliki layout monorepo microservice (Fase 4b): file ROOT
	// shared (go.mod direktif via plan, Makefile target proto, buf.yaml/
	// buf.gen.yaml, docker-compose.yml ber-anchor region:services, README, .gitignore,
	// libs/**) + template PER-SERVICE (proto, services/<svc>/cmd/main.go, services/<svc>/
	// internal/**) yang di-render ulang per service via DataOverride. Modul ini
	// self-contained (requires: []) & saling-konflik dengan arch-monolith/arch-modular.
	//
	// Gateway (Answers.Gateway) TIDAK memiliki modul terpisah: gateway in-proses
	// HTTP→gRPC adalah file per-service yang di-gate `.IsFirst` (demo inter-service,
	// selalu hadir untuk service pertama) ATAU `.Gateway` (edge opsional) DI DALAM
	// manifest arch-microservice. Resolver hanya memproyeksikan `.Gateway` ke data
	// agar manifest dapat menggate-nya; tak ada modul "gateway" yang diaktifkan.
	modArchMicro = "arch-microservice"

	modHTTPChi  = "http-chi"
	modHTTPEcho = "http-echo"

	modDBPostgres = "db-postgres"
	modDBMySQL    = "db-mysql"

	// access-gorm-* : lapisan akses GORM. Aktif HANYA bila access=gorm, dipasangkan
	// dengan driver DB terpilih (postgres/mysql). Mengganti koneksi sqlx/pgxpool/
	// database-sql milik db-* (yang di-gate `ne .Access "gorm"`) → tepat satu
	// mekanisme akses ter-emit (hindari double-wiring koneksi DB).
	modAccessGormPostgres = "access-gorm-postgres"
	modAccessGormMySQL    = "access-gorm-mysql"

	modAddonDocker = "addon-docker"
	modAddonMake   = "addon-makefile"
	modAddonLint   = "addon-golangci"
	modAddonEnv    = "addon-env"
	modAddonCI     = "addon-ci"
	modAddonObs    = "addon-observability"
)

// Resolver mengubah Answers menjadi GeneratePlan. Implementasi:
//   - meresolusi default (SPEC §6.2),
//   - memvalidasi constraint matrix relevan (SPEC §6.1) — fail-fast hard-invalid,
//   - menyeleksi modul aktif + evaluasi `when`,
//   - menghasilkan rencana deterministik (urutan stabil) demi invarian §5.2.
type Resolver interface {
	Resolve(a answers.Answers) (plan.GeneratePlan, error)
	// ResolveAddService merakit rencana inkremental untuk `gostarter add service`
	// (US-05): FileOp per-service arch-microservice untuk SATU service baru + SATU
	// kontribusi compose ke anchor region:services. TIDAK menyentuh file root/
	// service existing; tanpa go.mod (dipertahankan apa adanya). Lihat addservice.go.
	ResolveAddService(info AddServiceInfo) (AddServicePlan, error)
}

// resolver adalah implementasi default; ia membaca manifest modul dari Registry.
type resolver struct {
	reg module.Registry
}

// New mengembalikan Resolver yang membaca katalog modul dari reg.
func New(reg module.Registry) Resolver {
	return &resolver{reg: reg}
}

// Resolve menjalankan pipeline keputusan penuh untuk subset MVP.
func (r *resolver) Resolve(a answers.Answers) (plan.GeneratePlan, error) {
	// 1. Validasi field-level dulu (regex name, module path, enum subset MVP).
	if err := a.Validate(); err != nil {
		return plan.GeneratePlan{}, err
	}

	// 2. Resolusi default (SPEC §6.2) di atas salinan Answers.
	a = applyDefaults(a)

	// 3. Constraint matrix relevan (SPEC §6.1, fail-fast hard-invalid).
	if err := checkConstraints(a); err != nil {
		return plan.GeneratePlan{}, err
	}

	// 4. Seleksi modul aktif (module-level gating, ADR-003 D4 lapis 1).
	active, err := r.activeModules(a)
	if err != nil {
		return plan.GeneratePlan{}, err
	}

	// 5. Tegakkan requires/conflicts atas himpunan aktif (ADR-003 D6 / SPEC §6.4).
	if err := checkRelations(active); err != nil {
		return plan.GeneratePlan{}, err
	}

	// 6. Go directive: microservice memakai 1.25 (grpc v1.81.1 butuh go ≥ 1.25),
	// arch lain memakai default 1.24. Ditetapkan sekali di sini agar plan.GoVersion
	// & data["GoVersion"] (proyeksi render) SELALU sama (byte-identical, §5.2).
	goVer := goVersionFor(a)

	// 7. Context render = proyeksi Answers + Vars modul aktif (digabung).
	data := buildData(a, active, goVer)

	// 8. Rakit FileOp (files[] + merge contributes[]) dengan evaluasi `when`.
	files, err := buildFiles(active, a, data)
	if err != nil {
		return plan.GeneratePlan{}, err
	}

	// 9. Kumpulkan dependency go.mod (dedup + sort by Path).
	deps := collectDeps(active)

	// 10. Hook pasca-generate terurut.
	hooks := buildHooks(a)

	return plan.GeneratePlan{
		ProjectName: a.Name,
		ModulePath:  a.Module,
		GoVersion:   goVer,
		Files:       files,
		Deps:        deps,
		Hooks:       hooks,
	}, nil
}

// goVersionFor memilih go directive sesuai arsitektur: microservice → 1.25
// (google.golang.org/grpc v1.81.1 mensyaratkan go ≥ 1.25.0), arch lain → 1.24.
// Sumber tunggal agar plan.GoVersion & data["GoVersion"] tak pernah divergen.
func goVersionFor(a answers.Answers) string {
	if a.Arch == answers.ArchMicroservice {
		return goVersionMicroservice
	}
	return goVersionDefault
}

// applyDefaults mengisi nilai default SPEC §6.2 untuk field yang relevan ke subset
// MVP. Field di luar subset (mis. Comm/Broker) tidak disentuh (monolith).
func applyDefaults(a answers.Answers) answers.Answers {
	if a.Arch == "" {
		a.Arch = answers.ArchMonolith
	}
	// Microservice (Fase 4b): Comm default ke grpc (satu-satunya yang
	// diimplementasi v1). Kind TIDAK di-default (tiap service = gRPC server;
	// Kind tak relevan untuk microservice). Field monolith/modular lain dibiarkan.
	if a.Arch == answers.ArchMicroservice {
		if a.Comm == "" {
			a.Comm = answers.CommGRPC
		}
		if a.ConfigLoader == "" {
			a.ConfigLoader = answers.ConfigLoaderGodotenv
		}
		if a.Log == "" {
			a.Log = answers.LogSlog
		}
		if a.CI == "" {
			a.CI = answers.CINone
		}
		if a.Auth == "" {
			a.Auth = answers.AuthNone
		}
		return a
	}
	if a.Kind == "" {
		a.Kind = answers.KindREST
	}
	if a.HTTP == "" {
		a.HTTP = answers.HTTPNetHTTP
	}
	if a.DB == "" {
		a.DB = answers.DBNone
	}
	// Access & migrate hanya relevan bila DB ∈ {postgres, mysql} (SPEC §6.2).
	// SQL driver subset Fase 4a default ke sqlx + golang-migrate.
	if a.DB == answers.DBPostgres || a.DB == answers.DBMySQL {
		if a.Access == "" {
			a.Access = answers.AccessSQLx
		}
		if a.Migrate == "" {
			a.Migrate = answers.MigrateGolangMigrate
		}
		// Docker default (SPEC §6.2): db≠none ⇒ docker=true. Default deterministik
		// (tidak bergantung input lain) → byte-identical aman (SPEC §5.2). Jalur CLI
		// menandai keputusan eksplisit user via DockerSet (mis. user menyentuh
		// --addons/--feature tanpa docker); saat itu default ini DIHORMATI dan tidak
		// memaksakan docker. Jalur non-CLI (Answers tanpa gating, mis. resolver
		// langsung/test) DockerSet=false ⇒ default berlaku tanpa syarat.
		if !a.Docker && !a.DockerSet {
			a.Docker = true
		}
	}
	if a.ConfigLoader == "" {
		a.ConfigLoader = answers.ConfigLoaderGodotenv
	}
	if a.Log == "" {
		a.Log = answers.LogSlog
	}
	if a.CI == "" {
		a.CI = answers.CINone
	}
	if a.Auth == "" {
		a.Auth = answers.AuthNone
	}
	return a
}

// checkConstraints menegakkan constraint matrix SPEC §6.1 yang relevan untuk
// subset MVP. Hard-invalid → ErrConstraint (fail-fast, menyebut flag).
func checkConstraints(a answers.Answers) error {
	// C1: migration butuh DB (--migrate di-set ∧ --db=none → invalid).
	if a.Migrate != "" && a.DB == answers.DBNone {
		return fmt.Errorf("%w: --migrate %q membutuhkan --db (C1); db saat ini 'none'", ErrConstraint, a.Migrate)
	}
	// C2: access butuh DB (--access di-set ∧ --db=none → invalid).
	if a.Access != "" && a.DB == answers.DBNone {
		return fmt.Errorf("%w: --access %q membutuhkan --db (C2); db saat ini 'none'", ErrConstraint, a.Access)
	}
	// C18 (Fase 4a): observability wiring butuh permukaan HTTP/server. kind=worker
	// tak punya server → --obs tak relevan. (kind=worker masih di luar subset
	// Fase 4a, jadi guard ini defensif untuk saat kind diaktifkan Fase 4b.)
	if a.Obs && a.Kind == answers.KindWorker {
		return fmt.Errorf("%w: --obs membutuhkan permukaan HTTP/server (C18); kind 'worker' tidak punya server", ErrConstraint)
	}
	return nil
}

// activeModules menyeleksi himpunan modul aktif dari Answers dan mengambil
// manifestnya dari Registry. Error bila manifest yang diharapkan tak ada di
// katalog (integritas — seharusnya sudah lolos Load).
func (r *resolver) activeModules(a answers.Answers) ([]module.Manifest, error) {
	// Microservice (Fase 4b) memiliki himpunan modul SENDIRI: layout monorepo
	// gRPC milik arch-microservice, BUKAN core+arch-monolith (yang merakit server
	// HTTP monolith). Dipisah agar tak menyeret modul khas monolith (core server,
	// http-*, db-*, addon-docker compose monolith, addon-observability server-wiring)
	// yang tidak relevan & berpotensi bentrok anchor.
	if a.Arch == answers.ArchMicroservice {
		return r.activeMicroserviceModules(a)
	}

	var names []string

	// Selalu aktif: core (pemilik file shared + wiring + skeleton server net/http
	// default yang di-gate via `when`).
	names = append(names, modCore)

	// Arsitektur: TEPAT SATU modul arch-* aktif. arch-modular menggantikan
	// arch-monolith bila arch=modular-monolith (mutual conflicts ditegakkan
	// checkRelations dari manifest). Default monolith.
	switch a.Arch {
	case answers.ArchModularMonolith:
		names = append(names, modArchMod)
	default: // monolith (sudah didefault applyDefaults)
		names = append(names, modArchMono)
	}

	// HTTP router: chi/echo mengaktifkan modul http-* yang menggantikan routing
	// net/http core. Untuk net/http tidak ada modul http-* terpisah — routing
	// default dimiliki core (di-gate `when` eq .HTTP "net/http"). Ini mencegah
	// double-wiring & file routing yatim (pelajaran Fase 3).
	switch a.HTTP {
	case answers.HTTPChi:
		names = append(names, modHTTPChi)
	case answers.HTTPEcho:
		names = append(names, modHTTPEcho)
	}

	// DB driver (none → tidak ada modul db). postgres & mysql analog.
	switch a.DB {
	case answers.DBPostgres:
		names = append(names, modDBPostgres)
	case answers.DBMySQL:
		names = append(names, modDBMySQL)
	}

	// Lapisan akses GORM: bila access=gorm (hanya valid db∈{postgres,mysql} —
	// ditegakkan answers.Validate), aktifkan modul access-gorm-<driver> yang
	// MENGGANTIKAN koneksi sqlx/pgxpool/database-sql db-* (di-gate `ne .Access
	// "gorm"`). db-* TETAP aktif (menyumbang env/compose/migrate); hanya file +
	// wiring koneksinya yang di-gate off → tepat satu koneksi DB ter-emit. sqlx &
	// database-sql tidak mengaktifkan modul tambahan (koneksi default db-* sudah
	// memakai sqlx/database-sql).
	if a.Access == answers.AccessGORM {
		switch a.DB {
		case answers.DBPostgres:
			names = append(names, modAccessGormPostgres)
		case answers.DBMySQL:
			names = append(names, modAccessGormMySQL)
		}
	}

	// Add-on per Answers.
	if a.Docker {
		names = append(names, modAddonDocker)
	}
	if a.Makefile {
		names = append(names, modAddonMake)
	}
	if a.Lint {
		names = append(names, modAddonLint)
	}
	if a.EnvExample {
		names = append(names, modAddonEnv)
	}
	// CI: addon-ci aktif bila provider dipilih (github-actions/gitlab-ci).
	// 'none'/kosong → tanpa CI. Pemilihan file ci.yml vs .gitlab-ci.yml
	// dilakukan oleh `when` (eq .CI "...") di manifest addon-ci.
	if a.CI == answers.CIGitHubActions || a.CI == answers.CIGitLabCI {
		names = append(names, modAddonCI)
	}
	// Observability: addon-observability aktif bila Obs=true (otel tracing +
	// prometheus /metrics + health, wired ke server).
	if a.Obs {
		names = append(names, modAddonObs)
	}

	return r.resolveManifests(names)
}

// activeMicroserviceModules menyeleksi himpunan modul untuk arch=microservice
// (Fase 4b). Berbeda dari monolith/modular: TIDAK menyertakan core (yang merakit
// server monolith), http-* (tiap service = gRPC, bukan HTTP framework monolith),
// db-* (DB per-service di luar fokus v1 — minimal gRPC), addon-docker monolith
// (compose dimiliki arch-microservice), atau addon-observability (server-wiring
// monolith). Himpunan:
//   - arch-microservice (SELALU) — layout monorepo + file ROOT + template per-service
//     (termasuk gateway in-proses per-service yang di-gate `.IsFirst`/`.Gateway` DI
//     DALAM manifest; tak ada modul "gateway" terpisah untuk diaktifkan);
//   - addon-golangci / addon-ci bila dipilih (netral-arch).
//
// addon-makefile SENGAJA TIDAK diaktifkan: Makefile (skeleton + target "proto"
// untuk buf) DIMILIKI arch-microservice sendiri; addon-makefile menyumbang target
// monolith ("run" untuk cmd/<app>) yang tak relevan & berisiko bentrok anchor.
// addon-docker juga TIDAK: compose (ber-anchor region:services) dimiliki
// arch-microservice agar tiap service menyumbang servicenya sendiri.
func (r *resolver) activeMicroserviceModules(a answers.Answers) ([]module.Manifest, error) {
	names := []string{modArchMicro}
	if a.Lint {
		names = append(names, modAddonLint)
	}
	if a.CI == answers.CIGitHubActions || a.CI == answers.CIGitLabCI {
		names = append(names, modAddonCI)
	}
	return r.resolveManifests(names)
}

// resolveManifests mengambil manifest tiap nama dari registry, lalu mengurutkan
// stabil by Name agar deterministik (byte-identical, §5.2). Error bila ada nama
// yang tak ada di katalog (integritas — seharusnya sudah lolos Load).
func (r *resolver) resolveManifests(names []string) ([]module.Manifest, error) {
	out := make([]module.Manifest, 0, len(names))
	for _, n := range names {
		m, ok := r.reg.Get(n)
		if !ok {
			return nil, fmt.Errorf("%w: modul %q tidak ditemukan di registry (katalog tidak lengkap)", ErrConstraint, n)
		}
		out = append(out, m)
	}
	// Urutkan stabil by Name agar urutan deterministik (byte-identical, §5.2).
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// checkRelations menegakkan requires/conflicts atas himpunan modul aktif
// (ADR-003 D6 / SPEC §6.4). requires yang tak terpenuhi & conflicts yang aktif
// bersama → ErrConstraint.
func checkRelations(active []module.Manifest) error {
	set := make(map[string]bool, len(active))
	for _, m := range active {
		set[m.Name] = true
	}
	for _, m := range active {
		for _, req := range m.Requires {
			if !set[req] {
				return fmt.Errorf("%w: modul %q membutuhkan %q yang tidak aktif", ErrConstraint, m.Name, req)
			}
		}
		for _, con := range m.Conflicts {
			if set[con] {
				return fmt.Errorf("%w: modul %q konflik dengan %q (keduanya aktif)", ErrConstraint, m.Name, con)
			}
		}
	}
	return nil
}

// buildData menyusun context render: proyeksi Answers di-merge dengan Vars tiap
// modul aktif. Vars modul di-merge berurutan (modul belakangan menimpa) lalu
// field Answers ditaruh di atasnya (Answers menang) agar deterministik.
func buildData(a answers.Answers, active []module.Manifest, goVer string) map[string]any {
	data := make(map[string]any)
	for _, m := range active {
		for k, v := range m.Vars {
			data[k] = v
		}
	}
	// Proyeksi field Answers yang lazim dipakai template (subset MVP).
	//
	// Catatan kunci: template & target placeholder memakai `.ModulePath`
	// (mis. {{ modJoin .ModulePath ... }}, {{ modBase .ModulePath }}), bukan
	// `.Module`. Sediakan keduanya agar konsisten lintas template & `when`.
	data["Name"] = a.Name
	data["Module"] = a.Module
	data["ModulePath"] = a.Module
	data["Arch"] = string(a.Arch)
	data["Kind"] = string(a.Kind)
	data["HTTP"] = string(a.HTTP)
	data["DB"] = string(a.DB)
	data["Access"] = string(a.Access)
	data["Migrate"] = string(a.Migrate)
	data["ConfigLoader"] = string(a.ConfigLoader)
	data["Log"] = string(a.Log)
	data["CI"] = string(a.CI)
	data["Auth"] = string(a.Auth)
	data["Docker"] = a.Docker
	data["Makefile"] = a.Makefile
	data["Lint"] = a.Lint
	data["EnvExample"] = a.EnvExample
	// Obs (observability) diproyeksikan agar template addon-observability &
	// wiring server (chi/echo/net-http) dapat membaca {{ .Obs }} / {{ if .Obs }}.
	data["Obs"] = a.Obs
	data["Git"] = a.Git
	// Microservice (Fase 4b): proyeksikan Comm, Gateway, & daftar nama service
	// terurut (sort) agar template ROOT (mis. README, buf.gen.yaml, compose) dapat
	// menyebut seluruh service & template per-service membaca {{ .Comm }} dst.
	// .Services = []string nama terurut (deterministik). Field per-service ({{ .Service }},
	// {{ .IsFirst }}, {{ .Others }}) DISUNTIKKAN per-FileOp via DataOverride saat
	// ekspansi (lihat expandServiceFiles), BUKAN di data global.
	data["Comm"] = string(a.Comm)
	data["Gateway"] = a.Gateway
	data["Services"] = sortedServiceNames(a)
	// GoVersion ditetapkan PALING AKHIR dari sumber kanonik (goVer = plan.GoVersion,
	// dipilih goVersionFor per-arch) agar Vars modul TIDAK dapat men-shadow-nya.
	// Ditaruh setelah loop Vars memastikan presedensi resolver menang — go directive
	// project selalu deterministik & selaras plan.GeneratePlan.GoVersion (§5.2).
	data["GoVersion"] = goVer
	return data
}

// buildFiles merakit []plan.FileOp dari modul aktif (ADR-002 §6):
//
//   - contributes[] dikumpulkan dulu per target shared (mis. docker-compose.yml,
//     Makefile, .env.example, cmd/<app>/main.go, server.go ber-anchor). Tiap
//     target shared menghasilkan SATU FileOp ModeMerge yang skeletonnya = entri
//     files[] milik modul pemilik target itu (owner) — bukan path sintetik.
//   - files[] yang BUKAN target merge → FileOp render/copy/mkdir biasa
//     (evaluasi files[].when).
//
// Penting (ADR-002 §6 / ADR-003 D5): sebuah file shared dimiliki satu modul via
// files[] (skeleton ber-anchor) DAN menjadi tujuan contributes[] modul-modul lain
// (termasuk dirinya). Resolver memetakan owner-files[]→skeleton ModeMerge, BUKAN
// dua FileOp (render + merge) untuk path yang sama.
//
// Urutan akhir distabilkan by TargetPath agar byte-identical (SPEC §5.2).
func buildFiles(active []module.Manifest, a answers.Answers, data map[string]any) ([]plan.FileOp, error) {
	// (a) Kumpulkan kontribusi merge per (target, anchor), terkumpul lalu disusun
	//     menjadi daftar fragmen terurut per target.
	mergeTargets, mergeOrder, err := collectMergeFragments(active, a, data)
	if err != nil {
		return nil, err
	}

	// (b) File dari files[]. Bila target sebuah file juga merupakan target merge,
	//     file itu menjadi SKELETON FileOp ModeMerge (owner). Selain itu → FileOp
	//     render/copy/mkdir biasa.
	//
	//     PER-SERVICE (Fase 4b): bila target memuat placeholder {{ .Service }}
	//     (mis. proto/{{ .Service }}.proto, services/{{ .Service }}/cmd/main.go),
	//     file di-emit SEKALI PER service di Answers.Services — tiap FileOp
	//     membawa DataOverride {Service, IsFirst, Others, ModulePath} sehingga
	//     template per-service yang SAMA di-render ulang dengan nilai berbeda.
	//     Target tanpa {{ .Service }} = file ROOT, di-emit sekali (perilaku 4a).
	var ops []plan.FileOp
	skeletonOwned := make(map[string]bool) // target merge yang sudah punya skeleton owner
	for _, m := range active {
		for _, f := range m.Files {
			// Per-service: ekspansi N FileOp (satu per service) bila target
			// mengandung {{ .Service }}. Hanya relevan untuk arch=microservice.
			// PENTING: cek ini SEBELUM evalWhen outer — `when` file per-service boleh
			// menyebut field PER-SERVICE (mis. ".IsFirst") yang TIDAK ada di Answers;
			// evaluasinya ditunda ke expandServiceFile (per service, dgn overlay data).
			if isPerServiceTarget(f.Target) {
				svcOps, serr := expandServiceFile(m, f, a, data)
				if serr != nil {
					return nil, serr
				}
				ops = append(ops, svcOps...)
				continue
			}

			ok, err := evalWhen(f.When, a)
			if err != nil {
				return nil, fmt.Errorf("modul %q file %q: %w", m.Name, f.Target, err)
			}
			if !ok {
				continue
			}

			// Target placeholder (mis. cmd/{{ modBase .ModulePath }}/main.go) di-render
			// ke path konkret sebelum dipakai sebagai TargetPath & sebagai kunci merge.
			target, terr := renderTargetPath(f.Target, data)
			if terr != nil {
				return nil, fmt.Errorf("modul %q file %q: %w", m.Name, f.Target, terr)
			}
			// Defense-in-depth (B-1): tolak target ber-".." / absolut SETELAH render
			// placeholder — placeholder bisa mengevaluasi ke path tak terduga.
			if serr := checkSafeTargetPath(target); serr != nil {
				return nil, fmt.Errorf("%w: modul %q file %q: %v", ErrConstraint, m.Name, f.Target, serr)
			}

			if frags, isMerge := mergeTargets[target]; isMerge {
				// File ini adalah skeleton ber-anchor untuk target merge.
				skeletonOwned[target] = true
				ops = append(ops, plan.FileOp{
					Mode:         plan.ModeMerge,
					TargetPath:   target,
					ModuleName:   m.Name,
					TemplatePath: joinModulePath(m.Name, f.Template),
					Fragments:    frags,
					Perm:         permFor(f.Mode),
					Data:         data,
				})
				continue
			}

			ops = append(ops, plan.FileOp{
				Mode:         modeFromString(f.Mode),
				TargetPath:   target,
				ModuleName:   m.Name,
				TemplatePath: joinModulePath(m.Name, f.Template),
				Perm:         permFor(f.Mode),
				Data:         data,
			})
		}
	}

	// (c) Setiap target merge WAJIB punya skeleton owner (satu entri files[] yang
	//     menghasilkan target itu). Bila tidak ada → katalog tak konsisten
	//     (cermin validasi anchor↔skeleton ADR-003 D6.5) → fail-fast.
	for _, target := range mergeOrder {
		if !skeletonOwned[target] {
			return nil, fmt.Errorf("%w: target merge %q tidak punya skeleton owner di modul aktif (tak ada files[] yang menghasilkannya)", ErrConstraint, target)
		}
	}

	// Urutan akhir deterministik by TargetPath (ADR-002 §6 idempotensi).
	sort.SliceStable(ops, func(i, j int) bool { return ops[i].TargetPath < ops[j].TargetPath })
	return ops, nil
}

// collectMergeFragments mengumpulkan contributes[] modul aktif yang lolos `when`,
// mengelompokkannya per target shared, dan mengembalikan:
//   - map target → []plan.Fragment terurut (Order naik, tie-break nama modul),
//   - daftar target dalam urutan kemunculan (untuk validasi & determinisme).
//
// Content tiap Fragment diisi PATH fragmen di embed.FS (render ditunda ke
// generator/assembler), Anchor = nama anchor tujuan.
func collectMergeFragments(active []module.Manifest, a answers.Answers, data map[string]any) (map[string][]plan.Fragment, []string, error) {
	type targetFrags struct {
		frags []fragWithMod
	}
	byTarget := make(map[string]*targetFrags)
	var targetOrder []string

	for _, m := range active {
		for _, c := range m.Contributes {
			ok, err := evalWhen(c.When, a)
			if err != nil {
				return nil, nil, fmt.Errorf("modul %q contributes %q/%q: %w", m.Name, c.Target, c.Anchor, err)
			}
			if !ok {
				continue
			}
			// Target boleh memuat placeholder (mis. cmd/{{ modBase .ModulePath }}/
			// main.go); render ke path konkret agar cocok dengan kunci skeleton.
			target, terr := renderTargetPath(c.Target, data)
			if terr != nil {
				return nil, nil, fmt.Errorf("modul %q contributes %q: %w", m.Name, c.Target, terr)
			}
			// Defense-in-depth (B-1): tolak target ber-".." / absolut SETELAH render.
			if serr := checkSafeTargetPath(target); serr != nil {
				return nil, nil, fmt.Errorf("%w: modul %q contributes %q: %v", ErrConstraint, m.Name, c.Target, serr)
			}
			tf, seen := byTarget[target]
			if !seen {
				tf = &targetFrags{}
				byTarget[target] = tf
				targetOrder = append(targetOrder, target)
			}

			// PER-SERVICE (Fase 4b): kontribusi di-EKSPANSI satu fragment per service
			// bila SALAH SATU benar:
			//   (a) fragment/target ber-placeholder {{ .Service }} (eksplisit), ATAU
			//   (b) kontribusi berasal dari modul arch-microservice ke anchor "services"
			//       (compose region:services) — kontrak terdokumentasi manifest: fragment
			//       compose.service.yml.tmpl memakai {{ .Service }}/{{ .IsFirst }}/{{ .Others }}
			//       di KONTEN-nya meski path fragment & target root tak memuat placeholder.
			// Tiap fragment membawa DataOverride {Service, IsFirst, Others, ModulePath} agar
			// template fragmen yang SAMA di-render berbeda per service. Order = c.Order + idx
			// menjaga urutan deterministik & mencegah tabrakan order antar service (idx ikut
			// urutan sortedServiceNames). Tie-break sort tetap by nama modul.
			perService := isPerServiceTarget(c.Fragment) || isPerServiceTarget(c.Target) ||
				(a.Arch == answers.ArchMicroservice && m.Name == modArchMicro && c.Anchor == "services")
			if perService {
				for idx, svc := range sortedServiceNames(a) {
					tf.frags = append(tf.frags, fragWithMod{
						frag: plan.Fragment{
							Anchor:       c.Anchor,
							Content:      joinModulePath(m.Name, c.Fragment),
							Order:        c.Order + idx,
							DataOverride: serviceData(a, svc),
						},
						module: m.Name,
					})
				}
				continue
			}

			tf.frags = append(tf.frags, fragWithMod{
				frag: plan.Fragment{
					Anchor:  c.Anchor,
					Content: joinModulePath(m.Name, c.Fragment), // path fragmen; render ditunda ke generator
					Order:   c.Order,
				},
				module: m.Name,
			})
		}
	}

	out := make(map[string][]plan.Fragment, len(byTarget))
	for target, tf := range byTarget {
		frags := tf.frags
		// Sort stabil: order naik, tie-break nama modul (ADR-003 D5).
		sort.SliceStable(frags, func(i, j int) bool {
			if frags[i].frag.Order != frags[j].frag.Order {
				return frags[i].frag.Order < frags[j].frag.Order
			}
			return frags[i].module < frags[j].module
		})
		fr := make([]plan.Fragment, 0, len(frags))
		for _, f := range frags {
			fr = append(fr, f.frag)
		}
		out[target] = fr
	}
	return out, targetOrder, nil
}

// fragWithMod memasangkan fragment dengan nama modul asal untuk tie-break sort.
type fragWithMod struct {
	frag   plan.Fragment
	module string
}

// collectDeps mengumpulkan gomod[] dari modul aktif → plan.Deps (dedup by Path,
// HIGHEST VERSION WINS bila ganda, sort by Path). Memakai modpath.HigherVersion
// (semver-aware) — kebijakan kanonik ADR-002 §6, IDENTIK dengan generator.
// dedupSortDeps. Sebelumnya memakai pembanding leksikografis (`d.Version > cur`)
// yang menyalahi versi seperti v5.10.0 vs v5.9.0; modpath menyatukan keduanya.
func collectDeps(active []module.Manifest) []plan.ModuleDep {
	byPath := make(map[string]string)
	for _, m := range active {
		for _, d := range m.GoMod {
			if cur, ok := byPath[d.Path]; !ok {
				byPath[d.Path] = d.Version
			} else {
				byPath[d.Path] = modpath.HigherVersion(cur, d.Version)
			}
		}
	}
	if len(byPath) == 0 {
		return nil
	}
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	deps := make([]plan.ModuleDep, 0, len(paths))
	for _, p := range paths {
		deps = append(deps, plan.ModuleDep{Path: p, Version: byPath[p]})
	}
	return deps
}

// buildHooks menyusun hook pasca-generate terurut (ADR-002 §7):
// [BufGenerate(5, hanya microservice)] → Gofmt(10) → GoModTidy(20) → GitInit(30, hanya bila --git).
//
// Microservice (Fase 4b): hook "buf-generate" (order 5) DIJALANKAN PALING DULU
// agar gen/go/** ada (di-COMMIT) sebelum gofmt & go mod tidy — project hasil
// generate "go build ./..." HIJAU TANPA buf (buf hanya prasyarat saat BUILDER
// generate, bukan saat konsumen build). Nama & order selaras hooks.NameBufGenerate
// / hooks.OrderBufGenerate (literal di sini, konsisten dengan gofmt/go-mod-tidy/
// git-init yang juga literal demi tak menyeret dependensi paket hooks ke resolver).
func buildHooks(a answers.Answers) []plan.HookSpec {
	var hooks []plan.HookSpec
	if a.Arch == answers.ArchMicroservice {
		hooks = append(hooks, plan.HookSpec{Name: "buf-generate", Order: 5})
	}
	hooks = append(hooks,
		plan.HookSpec{Name: "gofmt", Order: 10},
		plan.HookSpec{Name: "go-mod-tidy", Order: 20},
	)
	if a.Git {
		hooks = append(hooks, plan.HookSpec{Name: "git-init", Order: 30})
	}
	return hooks
}

// ── Helper murni ─────────────────────────────────────────────────────────────

// modeFromString memetakan string mode module.yaml → plan.FileOpMode.
// "" → render (default, ADR-003 D2). Merge tidak datang dari sini.
func modeFromString(s string) plan.FileOpMode {
	switch s {
	case "copy":
		return plan.ModeCopy
	case "mkdir":
		return plan.ModeMkdir
	default: // "render" atau kosong
		return plan.ModeRender
	}
}

// permFor mengembalikan permission default per mode (file 0644, direktori 0755).
func permFor(mode string) fs.FileMode {
	if mode == "mkdir" {
		return 0o755
	}
	return 0o644
}

// checkSafeTargetPath menolak path target (SUDAH ter-render) yang dapat memecah
// containment project (B-1 path traversal): diawali "/" (absolut) atau memuat
// segmen "..". Lapis kedua setelah validasi katalog di registry — menangkap kasus
// di mana placeholder target mengevaluasi ke path berbahaya.
func checkSafeTargetPath(target string) error {
	t := strings.TrimSpace(target)
	if strings.HasPrefix(t, "/") {
		return fmt.Errorf("target %q absolut (harus relatif terhadap project)", target)
	}
	for _, seg := range strings.Split(t, "/") {
		if seg == ".." {
			return fmt.Errorf("target %q mengandung '..' (path traversal)", target)
		}
	}
	return nil
}

// joinModulePath menggabung nama modul dengan path relatif di dalam dir modul
// pada embed.FS (templates/modules/<mod>/<rel>). Slash-only POSIX (SPEC §2.1).
func joinModulePath(mod, rel string) string {
	if rel == "" {
		return ""
	}
	return "modules/" + mod + "/" + strings.TrimPrefix(rel, "/")
}

// renderTargetPath mengevaluasi placeholder template pada path target FileSpec/
// MergeContribution (mis. "cmd/{{ modBase .ModulePath }}/main.go") menjadi path
// konkret memakai data context (ADR-003 D3: placeholder target dievaluasi
// resolver, bukan saat render konten). Hanya FuncMap path-murni (modBase/modJoin)
// yang tersedia di sini — cukup untuk target, dan menghindari import siklik ke
// generator. Path tanpa placeholder dikembalikan apa adanya (fast-path).
func renderTargetPath(target string, data any) (string, error) {
	if !strings.Contains(target, "{{") {
		return target, nil
	}
	tmpl, err := template.New("target").Funcs(template.FuncMap{
		"modBase": modBaseHelper,
		"modJoin": modJoinHelper,
	}).Parse(target)
	if err != nil {
		return "", fmt.Errorf("parse target %q: %w", target, err)
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("evaluasi target %q: %w", target, err)
	}
	return sb.String(), nil
}

// modBaseHelper & modJoinHelper mendelegasikan ke internal/modpath (satu sumber
// kebenaran; identik dengan generator.modBase/modJoin) agar penyelesaian target
// placeholder di resolver dan render konten di generator memakai semantik yang
// SAMA — mencegah divergensi path yang memecah byte-identical (SPEC §5.2).
func modBaseHelper(modulePath string) string { return modpath.Base(modulePath) }

func modJoinHelper(modulePath string, elem ...string) string {
	return modpath.Join(modulePath, elem...)
}

// ── Helper microservice (Fase 4b) ────────────────────────────────────────────

// isPerServiceTarget melaporkan apakah sebuah path target/fragment ber-template
// menyebut field {{ .Service }} — penanda bahwa entri itu di-emit SEKALI PER
// service (bukan sekali untuk seluruh project). Deteksi sengaja literal &
// whitespace-toleran ("{{.Service}}" maupun "{{ .Service }}") agar manifest
// arch-microservice bebas menulis spasi. Hanya bermakna saat arch=microservice;
// untuk arch lain tak ada Answers.Services sehingga ekspansi menghasilkan 0 op.
func isPerServiceTarget(s string) bool {
	if !strings.Contains(s, "{{") {
		return false
	}
	compact := strings.ReplaceAll(s, " ", "")
	return strings.Contains(compact, "{{.Service}}") ||
		strings.Contains(compact, "{{.Service.") // dukung {{ .Service.Foo }} bila kelak ada
}

// sortedServiceNames mengembalikan nama service dari Answers.Services TERURUT
// (sort by Name) — sumber tunggal urutan service yang deterministik (byte-identical,
// §5.2). Dipakai oleh ekspansi file/fragment per-service maupun proyeksi
// data["Services"]. Duplikat seharusnya sudah ditolak Validate(); di sini tetap
// stabil bila ada.
func sortedServiceNames(a answers.Answers) []string {
	names := make([]string, 0, len(a.Services))
	for _, s := range a.Services {
		names = append(names, s.Name)
	}
	sort.Strings(names)
	return names
}

// serviceData membangun map override per-service yang disuntikkan ke FileOp.DataOverride
// / Fragment.DataOverride saat ekspansi. Berisi:
//   - Service : nama service ini (segmen path & nilai render, mis. nama paket proto);
//   - IsFirst : true HANYA untuk service pertama (urutan sortedServiceNames) — service
//     pertama menjalankan JUGA HTTP server kecil (GET /call) yang memanggil
//     service lain via gRPC (bukti inter-service call, acceptance T4.2);
//   - Others  : nama service LAIN (semua kecuali Service ini) — target panggilan gRPC;
//   - ModulePath : module path project (agar template per-service merakit import path
//     gen/go & libs tanpa bergantung data global yang mungkin di-shadow);
//   - GrpcPort : port gRPC bind service ini = grpcPortBase + INDEX (L-3 — index pada
//     urutan sortedServiceNames, BUKAN hitung jumlah direktori runtime);
//   - GatewayPort : port HTTP/gateway demo (httpGatewayBase) — bermakna untuk service
//     pertama yang mengekspos /call; disuntikkan ke semua agar template seragam.
//
// Determinisme: nilai diturunkan murni dari Answers + urutan tetap → byte-identical.
func serviceData(a answers.Answers, svc string) map[string]any {
	all := sortedServiceNames(a)
	others := make([]string, 0, len(all))
	// downstreams memasangkan tiap service LAIN dengan port gRPC-nya (grpcPortBase +
	// indeks-NYA) — bukan port pemanggil. Diperlukan agar alamat dial inter-service
	// benar di jaringan compose (svc-b mendengarkan di port-nya sendiri, bukan port
	// service pertama). Memperbaiki bug template lama "{{ $.GrpcPort }}" (port caller).
	downstreams := make([]map[string]any, 0, len(all))
	idx := 0
	for i, n := range all {
		if n == svc {
			idx = i
			continue
		}
		others = append(others, n)
		downstreams = append(downstreams, map[string]any{
			"Name": n,
			"Port": grpcPortBase + i,
		})
	}
	isFirst := len(all) > 0 && all[0] == svc
	return map[string]any{
		"Service":     svc,
		"IsFirst":     isFirst,
		"Others":      others,
		"Downstreams": downstreams,
		"ModulePath":  a.Module,
		"GrpcPort":    grpcPortBase + idx,
		"GatewayPort": httpGatewayBase,
	}
}

// expandServiceFile meng-emit SATU plan.FileOp per service untuk FileSpec
// per-service (target ber-{{ .Service }}). Tiap FileOp:
//   - TargetPath di-render dengan data global + override service (mis.
//     services/{{ .Service }}/cmd/main.go → services/svc-a/cmd/main.go);
//   - Data tetap = data global (context dasar), DataOverride = serviceData(svc)
//     sehingga generator me-render template per-service yang SAMA dengan nilai
//     berbeda (Service/IsFirst/Others/ModulePath menang atas data global);
//   - Mode mengikuti FileSpec (render default; copy untuk aset statik).
//
// Per-service file BUKAN target merge (proto & main per service unik per path) —
// jadi tak menyentuh jalur skeleton/merge. Urutan akhir tetap distabilkan by
// TargetPath di buildFiles (service di-iterasi terurut demi determinisme defensif).
func expandServiceFile(m module.Manifest, f module.FileSpec, a answers.Answers, data map[string]any) ([]plan.FileOp, error) {
	var ops []plan.FileOp
	for _, svc := range sortedServiceNames(a) {
		override := serviceData(a, svc)

		// `when` per-service dievaluasi dgn overlay (mendukung field per-service
		// seperti ".IsFirst") — mis. gateway in-proses hanya untuk service pertama.
		// File yang `when`-nya false untuk service ini di-skip (tak di-emit).
		ok, werr := evalWhenService(f.When, a, override)
		if werr != nil {
			return nil, fmt.Errorf("modul %q file %q (service %q): %w", m.Name, f.Target, svc, werr)
		}
		if !ok {
			continue
		}

		// Gabung data global + override untuk evaluasi placeholder target
		// (target butuh .Service DAN mungkin .ModulePath/modBase). Override menang.
		merged := make(map[string]any, len(data)+len(override))
		for k, v := range data {
			merged[k] = v
		}
		for k, v := range override {
			merged[k] = v
		}
		target, terr := renderTargetPath(f.Target, merged)
		if terr != nil {
			return nil, fmt.Errorf("modul %q file %q (service %q): %w", m.Name, f.Target, svc, terr)
		}
		if serr := checkSafeTargetPath(target); serr != nil {
			return nil, fmt.Errorf("%w: modul %q file %q (service %q): %v", ErrConstraint, m.Name, f.Target, svc, serr)
		}
		ops = append(ops, plan.FileOp{
			Mode:         modeFromString(f.Mode),
			TargetPath:   target,
			ModuleName:   m.Name,
			TemplatePath: joinModulePath(m.Name, f.Template),
			Perm:         permFor(f.Mode),
			Data:         data,
			DataOverride: override,
		})
	}
	return ops, nil
}
