// Package answers mendefinisikan struct konfigurasi internal tunggal yang diisi
// oleh kedua jalur input (wizard huh & flag cobra). Karena keduanya menulis ke
// struct yang sama sebelum render, output mode interaktif dan non-interaktif
// dijamin byte-identical (SPEC §5.2).
//
// FASE 3: Validate() mengimplementasikan validasi field-level (regex name,
// module path legal via x/mod/module, enum). Validasi constraint silang antar
// opsi (SPEC §6.1) BUKAN di sini — itu tugas resolver.Resolve.
//
// FASE 4a: subset diperluas — arch ∈ {monolith, modular-monolith};
// http ∈ {net/http, chi, echo}; db ∈ {none, postgres, mysql}; addon ci
// (CI ∈ {github-actions, gitlab-ci}) + observability (Obs bool) diaktifkan.
// gin/fiber, sqlite/mongo, access/migrate non-default tetap di luar subset
// (ditolak ramah).
//
// FASE 4b: arch=microservice DIAKTIFKAN (layout monorepo single-module gRPC,
// riset 02). Validate() mengizinkan arch=microservice dengan syarat: ≥1 service,
// nama service valid (lowercase, regex sama dgn nama project) & unik, dan
// Comm=grpc (default; rest/event DITOLAK ramah "v1: gRPC"). Gateway opsional
// (default off). Constraint silang (mis. kombinasi monolith-only) tetap di
// resolver. monolith/modular tidak berubah dari 4a.
package answers

import (
	"fmt"
	"regexp"

	"golang.org/x/mod/module"
)

// nameRe memvalidasi nama project (SPEC §4.2 q_name): huruf kecil di awal,
// lalu huruf kecil / angka / dash. Dipakai sebagai folder root & default segment
// terakhir module path.
var nameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Arch adalah tipe arsitektur project (SPEC §4.3 / flag --arch).
type Arch string

const (
	ArchMonolith        Arch = "monolith"
	ArchModularMonolith Arch = "modular-monolith"
	ArchMicroservice    Arch = "microservice"
)

// Kind adalah jenis aplikasi untuk monolith/modular-monolith (SPEC §4.4 / flag --kind).
type Kind string

const (
	KindREST   Kind = "rest"
	KindWeb    Kind = "web"
	KindWorker Kind = "worker"
)

// Comm adalah pola komunikasi antar service pada microservice (SPEC §4.5 / flag --comm).
type Comm string

const (
	CommGRPC  Comm = "grpc"
	CommREST  Comm = "rest"
	CommEvent Comm = "event"
)

// Broker adalah message broker untuk komunikasi event-driven (SPEC §4.5 / flag --broker).
type Broker string

const (
	BrokerNATS     Broker = "nats"
	BrokerKafka    Broker = "kafka"
	BrokerRabbitMQ Broker = "rabbitmq"
)

// HTTPFramework adalah pilihan HTTP framework (SPEC §4.6 / flag --http).
type HTTPFramework string

const (
	HTTPNetHTTP HTTPFramework = "net/http"
	HTTPChi     HTTPFramework = "chi"
	HTTPEcho    HTTPFramework = "echo"
	HTTPGin     HTTPFramework = "gin"
	HTTPFiber   HTTPFramework = "fiber"
)

// DB adalah driver database (SPEC §4.7 / flag --db).
type DB string

const (
	DBNone     DB = "none"
	DBPostgres DB = "postgres"
	DBMySQL    DB = "mysql"
	DBSQLite   DB = "sqlite"
	DBMongo    DB = "mongo"
)

// Access adalah lapisan akses query (SPEC §4.7 / flag --access).
type Access string

const (
	AccessSQLx        Access = "sqlx"
	AccessDatabaseSQL Access = "database/sql"
	AccessSQLC        Access = "sqlc"
	AccessGORM        Access = "gorm"
	AccessEnt         Access = "ent"
)

// Migrate adalah tool migrasi database (SPEC §4.7 / flag --migrate).
type Migrate string

const (
	MigrateGolangMigrate Migrate = "golang-migrate"
	MigrateGoose         Migrate = "goose"
	MigrateAtlas         Migrate = "atlas"
)

// CI adalah penyedia continuous integration (SPEC §4.8 / flag --ci).
type CI string

const (
	CINone          CI = "none"
	CIGitHubActions CI = "github-actions"
	CIGitLabCI      CI = "gitlab-ci"
)

// Auth adalah skema auth scaffold (SPEC §4.8 / flag --auth).
type Auth string

const (
	AuthNone   Auth = "none"
	AuthJWT    Auth = "jwt"
	AuthPaseto Auth = "paseto"
)

// ConfigLoader adalah library pemuat konfigurasi runtime (SPEC §5.1 / flag --config-loader).
type ConfigLoader string

const (
	ConfigLoaderGodotenv ConfigLoader = "godotenv"
	ConfigLoaderKoanf    ConfigLoader = "koanf"
	ConfigLoaderViper    ConfigLoader = "viper"
	ConfigLoaderEnv      ConfigLoader = "env"
)

// Log adalah library logging runtime (SPEC §5.1 / flag --log).
type Log string

const (
	LogSlog    Log = "slog"
	LogZerolog Log = "zerolog"
	LogZap     Log = "zap"
)

// Service adalah satu unit deploy dalam monorepo microservice (SPEC §4.5).
// Pada skeleton hanya nama yang dimodelkan; field tambahan menyusul di Fase 3 bila perlu.
type Service struct {
	Name string
}

// Answers adalah hasil terkumpul dari semua pertanyaan SPEC §4 / flag SPEC §5.1.
// Satu instance ini menjadi input tunggal resolver (lihat package resolver).
//
// Catatan: setiap field memetakan 1:1 ke flag pada SPEC §5.1; nilai default
// diresolusi oleh resolver, bukan oleh struct ini.
type Answers struct {
	// q_name (SPEC §4.2)
	Name   string // --name
	Module string // --module (default github.com/<name>)

	// q_arch (SPEC §4.3)
	Arch Arch // --arch

	// q_kind (SPEC §4.4) — relevan untuk monolith/modular-monolith
	Kind Kind // --kind

	// q_svc group (SPEC §4.5) — relevan untuk microservice
	Services []Service // --service (repeatable) / --services (csv)
	Comm     Comm      // --comm
	Broker   Broker    // --broker (hanya bila Comm == CommEvent)
	Gateway  bool      // --gateway / --no-gateway

	// q_http (SPEC §4.6)
	HTTP HTTPFramework // --http

	// q_db group (SPEC §4.7)
	DB      DB      // --db
	Access  Access  // --access (bila DB != none, != mongo)
	Migrate Migrate // --migrate (bila DB not in {none, mongo})

	// q_addons (SPEC §4.8)
	Docker bool // --docker / --no-docker
	// DockerSet menandai bahwa Docker dipilih SECARA EKSPLISIT oleh user (jalur
	// CLI: --addons/--feature menyentuh docker, atau preset menyetelnya). Dipakai
	// HANYA oleh resolver.applyDefaults untuk membedakan "docker off karena user
	// memilih demikian" dari "docker off karena default belum dihitung" (SPEC §6.2:
	// docker default true bila db≠none). Tidak diproyeksikan ke context render
	// (buildData) sehingga TIDAK memengaruhi byte-identical (SPEC §5.2).
	DockerSet  bool
	Makefile   bool // --makefile / --no-makefile
	Taskfile   bool // --taskfile
	CI         CI   // --ci
	Lint       bool // --lint / --no-lint
	Obs        bool // --obs / --no-obs
	Auth       Auth // --auth
	EnvExample bool // --env-example / --no-env-example

	// Opsi terkunci (flag-only, SPEC §5.1)
	ConfigLoader ConfigLoader // --config-loader
	Log          Log          // --log
	// ValidateInput memetakan flag --validate / --no-validate (SPEC §5.1).
	// Dinamai *Input* (bukan Validate) untuk menghindari tabrakan dengan method
	// Validate() pada tipe Answers — Go melarang field dan method bernama sama.
	ValidateInput bool

	// Flag advanced (SPEC §6.5) — default off, di luar 8 langkah inti
	Mock        bool // --mock
	Integration bool // --integration

	// q_git (SPEC §4.9)
	Git bool // --git / --no-git
}

// Validate memeriksa konsistensi field-level Answers (regex name, module path
// legal via x/mod/module, enum) — BUKAN constraint silang antar-opsi (itu
// resolver.Resolve). Pesan error sengaja ramah & menyebut field bermasalah.
//
// Catatan scope Fase 4a/4b: subset didukung adalah
// arch ∈ {monolith, modular-monolith, microservice} / kind=rest /
// http ∈ {net/http, chi, echo} / db ∈ {none, postgres, mysql}, ditambah addon
// ci (CI ∈ {github-actions, gitlab-ci}) & observability. Untuk microservice
// (Fase 4b) berlaku aturan tambah: ≥1 service, nama service valid & unik,
// Comm=grpc saja (rest/event ditolak ramah). Nilai di luar subset (gin/fiber,
// sqlite/mongo, access/migrate non-default) ditolak ramah ("belum didukung")
// agar wizard tidak menghasilkan project rusak.
func (a Answers) Validate() error {
	// 1. Nama project wajib & cocok regex (SPEC §4.2).
	if a.Name == "" {
		return fmt.Errorf("nama project (--name) wajib diisi dan tidak boleh kosong")
	}
	if !nameRe.MatchString(a.Name) {
		return fmt.Errorf("nama project %q tidak valid: harus diawali huruf kecil lalu hanya huruf kecil, angka, atau dash (regex %s)", a.Name, nameRe.String())
	}

	// 2. Module path wajib & legal mengikuti semantik golang.org/x/mod/module
	//    (SPEC §4.2 Aturan Validasi Module Path).
	if a.Module == "" {
		return fmt.Errorf("module path (--module) wajib diisi (default github.com/%s)", a.Name)
	}
	if err := module.CheckPath(a.Module); err != nil {
		return fmt.Errorf("module path %q tidak valid: %v (contoh valid: github.com/akun/%s)", a.Module, err, a.Name)
	}

	// 3. Arsitektur — Fase 4a/4b mendukung monolith, modular-monolith, dan
	//    microservice. Untuk microservice berlaku validasi tambahan q_svc
	//    (Services / Comm / Gateway) di langkah 3b.
	switch a.Arch {
	case ArchMonolith, ArchModularMonolith, ArchMicroservice:
		// ok
	default:
		return fmt.Errorf("arsitektur %q tidak dikenal: pilih 'monolith', 'modular-monolith', atau 'microservice'", a.Arch)
	}

	// 3b. q_svc (SPEC §4.5) — hanya tervalidasi bila arch=microservice. Aturan:
	//     - ≥1 service (monorepo wajib punya minimal satu unit deploy);
	//     - tiap nama service valid (regex nameRe: lowercase, sama seperti nama
	//       project) — dipakai sbg segmen path services/<name>/ & proto/<name>.proto;
	//     - nama unik (tak ada duplikat — duplikat memecah layout & idempotensi
	//       "add service");
	//     - Comm = grpc (default; rest & event ditolak ramah, "v1: gRPC").
	//     Gateway bool opsional (default off) — tak perlu validasi enum.
	//     Field q_svc untuk monolith/modular diabaikan (tak relevan).
	if a.Arch == ArchMicroservice {
		if len(a.Services) == 0 {
			return fmt.Errorf("arsitektur 'microservice' membutuhkan minimal 1 service (--service / --services); tak ada service diberikan")
		}
		seen := make(map[string]bool, len(a.Services))
		for _, svc := range a.Services {
			if svc.Name == "" {
				return fmt.Errorf("nama service tidak boleh kosong")
			}
			if !nameRe.MatchString(svc.Name) {
				return fmt.Errorf("nama service %q tidak valid: harus diawali huruf kecil lalu hanya huruf kecil, angka, atau dash (regex %s)", svc.Name, nameRe.String())
			}
			// Reserved: "gateway" adalah nama edge/proxy (SPEC §4.5) — tak boleh
			// dipakai sebagai nama service biasa agar tak bentrok dgn dir gateway opsional.
			if svc.Name == "gateway" {
				return fmt.Errorf("nama service %q reserved (dipakai untuk API gateway edge); pilih nama lain", svc.Name)
			}
			if seen[svc.Name] {
				return fmt.Errorf("nama service %q duplikat: nama service harus unik", svc.Name)
			}
			seen[svc.Name] = true
		}
		switch a.Comm {
		case "", CommGRPC:
			// ok — kosong → resolver default ke grpc (satu-satunya yang
			// diimplementasi v1).
		case CommREST:
			return fmt.Errorf("komunikasi %q belum didukung (v1: gRPC); REST-comm menyusul", a.Comm)
		case CommEvent:
			return fmt.Errorf("komunikasi %q belum didukung (v1: gRPC); event-driven (NATS/Kafka/RabbitMQ) menyusul", a.Comm)
		default:
			return fmt.Errorf("komunikasi %q tidak dikenal: pilih 'grpc'", a.Comm)
		}
	}

	// 4. Kind — Fase 4a hanya rest untuk monolith/modular (scope: kind=rest,
	//    http ∈ {net/http, chi, echo}, db ∈ {none, postgres, mysql},
	//    CI ∈ {none, github-actions, gitlab-ci}). web & worker menyusul.
	//    Untuk microservice, Kind tidak relevan (tiap service = gRPC server);
	//    Kind kosong diterima (resolver tak men-default Kind untuk microservice).
	if a.Arch != ArchMicroservice {
		switch a.Kind {
		case KindREST:
			// ok
		case KindWeb, KindWorker:
			return fmt.Errorf("jenis aplikasi %q belum didukung di MVP (Fase 4a hanya 'rest'); akan hadir di fase berikutnya", a.Kind)
		default:
			return fmt.Errorf("jenis aplikasi %q tidak dikenal: pilih 'rest'", a.Kind)
		}
	}

	// Langkah 5–7 (HTTP / DB / Access / Migrate) adalah permukaan monolith/modular.
	// Untuk microservice v1 (pure gRPC, minimal — DB per-service & HTTP edge di luar
	// scope) field ini TIDAK relevan & dibiarkan kosong; resolver tak men-default-nya.
	// Lewati validasinya agar microservice tak menuntut http=net/http / db=none isi.
	if a.Arch != ArchMicroservice {
		// 5. HTTP framework — Fase 4a mendukung net/http (default), chi, echo.
		//    gin/fiber tetap menyusul (fiber juga memicu cabang Go 1.25, C8).
		switch a.HTTP {
		case HTTPNetHTTP, HTTPChi, HTTPEcho:
			// ok
		case HTTPGin, HTTPFiber:
			return fmt.Errorf("HTTP framework %q belum didukung (Fase 4a hanya 'net/http', 'chi', 'echo'); akan hadir di fase berikutnya", a.HTTP)
		default:
			return fmt.Errorf("HTTP framework %q tidak dikenal: pilih 'net/http', 'chi', atau 'echo'", a.HTTP)
		}

		// 6. Database — Fase 4a mendukung none (default), postgres, mysql.
		//    sqlite/mongo tetap menyusul.
		switch a.DB {
		case DBNone:
			// ok — murni stdlib, tanpa access/migrate.
		case DBPostgres, DBMySQL:
			// Access & migrate divalidasi di bawah (relevan hanya bila db != none).
		case DBSQLite, DBMongo:
			return fmt.Errorf("database %q belum didukung (Fase 4a hanya 'none', 'postgres', 'mysql'); akan hadir di fase berikutnya", a.DB)
		default:
			return fmt.Errorf("database %q tidak dikenal: pilih 'none', 'postgres', atau 'mysql'", a.DB)
		}

		// 7. Access & Migrate — hanya tervalidasi bila db ∈ {postgres, mysql}. Untuk
		//    subset Fase 4a, access = sqlx dan migrate = golang-migrate (driver
		//    tunggal yang diimplementasi). Nilai kosong dibiarkan (resolver default).
		if a.DB == DBPostgres || a.DB == DBMySQL {
			switch a.Access {
			case "", AccessSQLx:
				// ok (kosong → resolver default ke sqlx)
			case AccessDatabaseSQL, AccessSQLC, AccessGORM, AccessEnt:
				return fmt.Errorf("access layer %q belum didukung di MVP (Fase 3 hanya 'sqlx'); akan hadir di fase berikutnya", a.Access)
			default:
				return fmt.Errorf("access layer %q tidak dikenal: pilih 'sqlx'", a.Access)
			}
			switch a.Migrate {
			case "", MigrateGolangMigrate:
				// ok (kosong → resolver default ke golang-migrate)
			case MigrateGoose, MigrateAtlas:
				return fmt.Errorf("tool migrasi %q belum didukung di MVP (Fase 3 hanya 'golang-migrate'); akan hadir di fase berikutnya", a.Migrate)
			default:
				return fmt.Errorf("tool migrasi %q tidak dikenal: pilih 'golang-migrate'", a.Migrate)
			}
		}
	}

	// 8. CI — Fase 4a mengaktifkan addon ci. Provider yang didukung:
	//    github-actions & gitlab-ci (CI provider tunggal, C-ci). 'none'/kosong =
	//    tanpa CI. Resolver mengaktifkan modul addon-ci bila CI ∈ {github-actions,
	//    gitlab-ci} dan memilih emit .github/workflows/ci.yml ATAU .gitlab-ci.yml
	//    berdasar nilai ini.
	switch a.CI {
	case "", CINone, CIGitHubActions, CIGitLabCI:
		// ok
	default:
		return fmt.Errorf("CI %q tidak dikenal: pilih 'github-actions', 'gitlab-ci', atau 'none'", a.CI)
	}

	// 9. Auth — Fase 3 belum mengimplementasi auth (Fase 4). Hanya 'none'/kosong.
	switch a.Auth {
	case "", AuthNone:
		// ok
	case AuthJWT, AuthPaseto:
		return fmt.Errorf("auth %q belum didukung di MVP (Fase 3 belum menyertakan auth scaffold); akan hadir di fase berikutnya", a.Auth)
	default:
		return fmt.Errorf("auth %q tidak dikenal: pilih 'none' (atau kosongkan); 'jwt'/'paseto' menyusul", a.Auth)
	}

	return nil
}
