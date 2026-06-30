package resolver

import (
	"errors"
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/faisalcayunda/gostarter/internal/answers"
	"github.com/faisalcayunda/gostarter/internal/module"
	"github.com/faisalcayunda/gostarter/internal/plan"
	"github.com/faisalcayunda/gostarter/templates"
)

// fakeRegistry adalah Registry in-memory untuk test (tanpa embed.FS / disk).
//
// Manifest disimpan di SLICE (urut sisip, lalu All() mengembalikannya ter-sort by
// Name) agar MENCERMINKAN kontrak registry produksi yang men-sort All() by Name
// (M-5). Map by-name dipertahankan hanya untuk Get O(1). Sebelumnya All()
// meng-iterasi map → urutan non-deterministik, tak cocok dengan registry nyata.
type fakeRegistry struct {
	order []module.Manifest          // urutan sisip (sumber kebenaran)
	byNm  map[string]module.Manifest // indeks Get
}

// newFakeRegistry membangun fakeRegistry dari map manifest. Urutan sisip
// distabilkan dengan men-sort key agar konstruksi sendiri deterministik.
func newFakeRegistry(mods map[string]module.Manifest) *fakeRegistry {
	names := make([]string, 0, len(mods))
	for n := range mods {
		names = append(names, n)
	}
	sort.Strings(names)
	fr := &fakeRegistry{byNm: make(map[string]module.Manifest, len(mods))}
	for _, n := range names {
		fr.order = append(fr.order, mods[n])
		fr.byNm[n] = mods[n]
	}
	return fr
}

func (f *fakeRegistry) Load(fsys fs.FS) error { return nil }
func (f *fakeRegistry) Get(name string) (module.Manifest, bool) {
	m, ok := f.byNm[name]
	return m, ok
}

// All mengembalikan manifest urut deterministik by Name — SAMA seperti registry
// produksi (SPEC §5.2). Slice disalin agar pemanggil tak memodifikasi state.
func (f *fakeRegistry) All() []module.Manifest {
	out := make([]module.Manifest, len(f.order))
	copy(out, f.order)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// mvpRegistry membangun katalog modul subset MVP yang MENCERMINKAN katalog nyata
// (templates/modules/**): core + arch-monolith adalah monolith REST net/http yang
// utuh (tanpa modul http-* terpisah). File shared (docker-compose.yml, Makefile,
// .env.example, cmd/<app>/main.go) DIMILIKI core via files[] (skeleton ber-anchor)
// dan menjadi target merge dari contributes[] modul lain. Manifest sengaja minimal
// tetapi merepresentasikan kontrak owner-skeleton↔merge (ADR-002 §6 / ADR-003 D5).
func mvpRegistry() *fakeRegistry {
	return newFakeRegistry(map[string]module.Manifest{
		modCore: {
			Name:        modCore,
			Description: "skeleton dasar",
			Files: []module.FileSpec{
				{Template: "README.md.tmpl", Target: "README.md", Mode: "render"},
				{Template: "gitignore.tmpl", Target: ".gitignore", Mode: "render"},
				// Skeleton entrypoint (target placeholder dirender resolver). Target
				// memakai .ModulePath PERSIS seperti manifest nyata (buildData
				// menyediakan .Module & .ModulePath; keduanya render ke cmd/<app>/main.go).
				{Template: "cmd/main.go.tmpl", Target: "cmd/{{ modBase .ModulePath }}/main.go", Mode: "render"},
				// Routing net/http default dimiliki core, di-gate eq .HTTP "net/http".
				// chi/echo menggantikan via modul http-* (server.go) → tak ada double-emit.
				// Target server.go selaras manifest nyata (templates/modules/core);
				// skeleton ini ber-anchor region:imports + region:routes → jadi target
				// merge AUTO-WIRE addon-observability.
				{Template: "httpserver/server.go.tmpl", Target: "internal/httpserver/server.go", Mode: "render", When: `and (eq .Arch "monolith") (eq .HTTP "net/http")`},
				// Skeleton file shared ber-anchor (juga target merge).
				{Template: "docker-compose.yml.tmpl", Target: "docker-compose.yml", Mode: "render", When: ".Docker"},
				{Template: "Makefile.tmpl", Target: "Makefile", Mode: "render", When: ".Makefile"},
				{Template: "env.example.tmpl", Target: ".env.example", Mode: "render", When: ".EnvExample"},
			},
			Contributes: []module.MergeContribution{
				{Target: ".env.example", Anchor: "app", Fragment: "fragments/env.app.tmpl", Order: 0, When: ".EnvExample"},
			},
		},
		modArchMono: {
			Name:        modArchMono,
			Description: "layout monolith Kandidat B",
			Requires:    []string{modCore},
			Conflicts:   []string{modArchMod}, // tepat satu arch-* aktif
			Files: []module.FileSpec{
				{Template: "internal/app/app.go.tmpl", Target: "internal/app/app.go", Mode: "render"},
			},
		},
		// arch-modular: layout modular-monolith (per-domain internal/, shared/
		// contract, in-process bus, composition root tunggal di cmd). Dua contoh
		// domain (catalog & orders). Mutual conflict dengan arch-monolith.
		//
		// SUBSET DOKUMENTASIKAN: manifest nyata (templates/modules/arch-modular)
		// memiliki ~19 file (facade + internal/core per domain, README, dst). Fixture
		// ini sengaja menyederhanakan layout domain menjadi satu file per domain agar
		// test resolver fokus ke keputusan struktural; guard membandingkan SUBSET
		// (GoMod kosong identik + ownership server.go modular) — bukan jumlah file
		// penuh. Yang WAJIB selaras manifest nyata: arch-modular OWNS
		// internal/httpserver/server.go saat http=net/http (skeleton ber-anchor →
		// target merge AUTO-WIRE obs di modular).
		modArchMod: {
			Name:        modArchMod,
			Description: "layout modular-monolith per-domain",
			Requires:    []string{modCore},
			Conflicts:   []string{modArchMono},
			Files: []module.FileSpec{
				{Template: "internal/catalog/catalog.go.tmpl", Target: "internal/catalog/catalog.go", Mode: "render"},
				{Template: "internal/orders/orders.go.tmpl", Target: "internal/orders/orders.go", Mode: "render"},
				{Template: "internal/shared/contract/contract.go.tmpl", Target: "internal/shared/contract/contract.go", Mode: "render"},
				{Template: "internal/app/app.go.tmpl", Target: "internal/app/app.go", Mode: "render"},
				// Server netral-domain net/http modular — OWNS server.go saat http=net/http
				// (selaras manifest nyata; mencegah double-ownership dengan http-chi/echo).
				// Skeleton ber-anchor → jadi target merge AUTO-WIRE obs di modular.
				{Template: "internal/httpserver/server.go.tmpl", Target: "internal/httpserver/server.go", Mode: "copy", When: `eq .HTTP "net/http"`},
			},
		},
		// arch-microservice (Fase 4b): layout monorepo single-module gRPC. Fixture
		// MENCERMINKAN manifest nyata (templates/modules/arch-microservice/module.yaml)
		// sebagai SUBSET terdokumentasi: file ROOT (README/.gitignore/buf*/Makefile/
		// compose) + PER-SERVICE (proto/{{ .Service }}/v1/{{ .Service }}.proto,
		// services/{{ .Service }}/cmd/main.go, services/{{ .Service }}/internal/server,
		// gateway gated .IsFirst) di-emit sekali per service via DataOverride. GoMod
		// WAJIB EKSAK: grpc v1.81.1 + protobuf v1.36.11. Conflicts arch-monolith/modular.
		// requires: [] (self-contained — tak menarik core monolith). Contributes compose
		// region:services (anchor "services", order 10) di-ekspansi per-service oleh resolver.
		// libs/** (config/logger/health/grpcclient) tidak direplikasi di fixture (di luar
		// fokus test struktural per-service) → cmpSubset.
		modArchMicro: {
			Name:        modArchMicro,
			Description: "layout microservice monorepo single-module gRPC",
			Requires:    []string{},
			Conflicts:   []string{modArchMono, modArchMod},
			Files: []module.FileSpec{
				// ROOT (sekali).
				{Template: "README.md.tmpl", Target: "README.md", Mode: "render"},
				{Template: "gitignore.tmpl", Target: ".gitignore", Mode: "render"},
				{Template: "buf.yaml.tmpl", Target: "buf.yaml", Mode: "render"},
				{Template: "buf.gen.yaml.tmpl", Target: "buf.gen.yaml", Mode: "render"},
				{Template: "Makefile.tmpl", Target: "Makefile", Mode: "render"},
				{Template: "docker-compose.yml.tmpl", Target: "docker-compose.yml", Mode: "render"},
				// PER-SERVICE (placeholder {{ .Service }} → emit sekali per service).
				{Template: "proto/service.proto.tmpl", Target: "proto/{{ .Service }}/v1/{{ .Service }}.proto", Mode: "render"},
				{Template: "services/cmd/main.go.tmpl", Target: "services/{{ .Service }}/cmd/main.go", Mode: "render"},
				{Template: "services/internal/config/config.go.tmpl", Target: "services/{{ .Service }}/internal/config/config.go", Mode: "render"},
				{Template: "services/internal/server/server.go.tmpl", Target: "services/{{ .Service }}/internal/server/server.go", Mode: "render"},
				// Gateway in-proses HANYA untuk service pertama (gate .IsFirst).
				{Template: "services/internal/gateway/gateway.go.tmpl", Target: "services/{{ .Service }}/internal/gateway/gateway.go", Mode: "render", When: ".IsFirst"},
			},
			GoMod: []module.ModuleDep{
				{Path: "google.golang.org/grpc", Version: "v1.81.1"},
				{Path: "google.golang.org/protobuf", Version: "v1.36.11"},
			},
			Contributes: []module.MergeContribution{
				{Target: "docker-compose.yml", Anchor: "services", Fragment: "fragments/compose.service.yml.tmpl", Order: 10},
			},
			Vars: map[string]any{"GrpcPort": 9090, "GatewayPort": 8080},
		},
		// http-chi: router go-chi menggantikan routing net/http core. Menyediakan
		// internal/httpserver/server.go versi chi (di-gate eq .HTTP "chi"); routing
		// default core di-gate eq .HTTP "net/http" sehingga TIDAK ada double-emit
		// server.go. Target server.go selaras manifest nyata (templates/modules/http-chi).
		modHTTPChi: {
			Name:        modHTTPChi,
			Description: "router go-chi/chi/v5",
			Requires:    []string{modCore},
			Files: []module.FileSpec{
				{Template: "httpserver/server.go.tmpl", Target: "internal/httpserver/server.go", Mode: "render", When: `eq .HTTP "chi"`},
			},
			GoMod: []module.ModuleDep{
				{Path: "github.com/go-chi/chi/v5", Version: "v5.3.0"},
			},
		},
		// http-echo: router labstack/echo menggantikan routing net/http core.
		// Menyediakan internal/httpserver/server.go versi echo (di-gate eq .HTTP
		// "echo"). Target + gomod selaras manifest nyata (templates/modules/http-echo).
		modHTTPEcho: {
			Name:        modHTTPEcho,
			Description: "router labstack/echo/v4",
			Requires:    []string{modCore},
			Files: []module.FileSpec{
				{Template: "httpserver/server.go.tmpl", Target: "internal/httpserver/server.go", Mode: "render", When: `eq .HTTP "echo"`},
			},
			GoMod: []module.ModuleDep{
				// Cermin manifest nyata (templates/modules/http-echo/module.yaml).
				{Path: "github.com/labstack/echo/v4", Version: "v4.15.2"},
			},
		},
		// db-postgres: driver pgx/v5 (pgxpool) + skeleton DB + migrasi + AUTO-WIRE
		// database.Connect(ctx) ke main. Fixture MENCERMINKAN manifest nyata
		// (templates/modules/db-postgres/module.yaml): HANYA pgx/v5 v5.10.0 di gomod
		// (jmoiron/sqlx SENGAJA DIHAPUS — akan di-prune go mod tidy karena belum
		// dipakai kode; lihat manifest §C-2), dan AUTO-WIRE 2 contributes ke
		// cmd/<app>/main.go (anchor imports order 20, anchor wiring order 20)
		// membuktikan Connect ter-wire. SUBSET DOKUMENTASIKAN: compose volume &
		// Makefile-migrate contributes manifest nyata tidak direplikasi di fixture
		// (di luar fokus test); guard membandingkan GoMod (WAJIB identik) + subset
		// contributes yang direplikasi.
		modDBPostgres: {
			Name:        modDBPostgres,
			Description: "postgres pgxpool + migrations + wiring main",
			Files: []module.FileSpec{
				// Koneksi pgxpool DI-GATE `ne .Access "gorm"` (selaras manifest nyata):
				// saat access=gorm digantikan gorm.go milik access-gorm-postgres.
				{Template: "database/postgres.go.tmpl", Target: "internal/platform/database/postgres.go", Mode: "render", When: "ne .Access \"gorm\""},
				{Template: "migrations/0001_init.up.sql.tmpl", Target: "migrations/0001_init.up.sql", Mode: "copy", When: "ne .Migrate \"\""},
				{Template: "migrations/0001_init.down.sql.tmpl", Target: "migrations/0001_init.down.sql", Mode: "copy", When: "ne .Migrate \"\""},
			},
			GoMod: []module.ModuleDep{
				// HANYA pgx/v5 (selaras manifest nyata). sqlx DIHAPUS (C-2).
				{Path: "github.com/jackc/pgx/v5", Version: "v5.10.0"},
			},
			Requires:  []string{modCore},
			Conflicts: []string{modDBMySQL},
			Contributes: []module.MergeContribution{
				{Target: "docker-compose.yml", Anchor: "services", Fragment: "fragments/compose.postgres.service.yml.tmpl", Order: 20, When: ".Docker"},
				{Target: ".env.example", Anchor: "database", Fragment: "fragments/env.postgres.tmpl", Order: 20, When: ".EnvExample"},
				// AUTO-WIRE pgxpool DI-GATE `ne .Access "gorm"` (selaras manifest nyata).
				{Target: "cmd/{{ modBase .ModulePath }}/main.go", Anchor: "imports", Fragment: "fragments/main.imports.postgres.tmpl", Order: 20, When: "ne .Access \"gorm\""},
				{Target: "cmd/{{ modBase .ModulePath }}/main.go", Anchor: "wiring", Fragment: "fragments/main.wiring.postgres.tmpl", Order: 20, When: "ne .Access \"gorm\""},
			},
			Vars: map[string]any{"DBPort": 5432, "DBImage": "postgres:17-alpine"},
		},
		// db-mysql: analog db-postgres (driver + migration + compose service mysql +
		// AUTO-WIRE main). Fixture MENCERMINKAN manifest nyata
		// (templates/modules/db-mysql/module.yaml): HANYA driver go-sql-driver/mysql
		// v1.10.0 (tanpa sqlx; via database/sql stdlib), dan AUTO-WIRE 2 contributes
		// ke cmd/<app>/main.go (anchor imports order 20, anchor wiring order 20).
		// SUBSET DOKUMENTASIKAN: compose volume & Makefile-migrate contributes manifest
		// nyata tidak direplikasi (di luar fokus test). GoMod WAJIB identik manifest.
		modDBMySQL: {
			Name:        modDBMySQL,
			Description: "mysql go-sql-driver + migrations + wiring main",
			Files: []module.FileSpec{
				// Koneksi database/sql DI-GATE `ne .Access "gorm"` (selaras manifest nyata).
				{Template: "database/mysql.go.tmpl", Target: "internal/platform/database/mysql.go", Mode: "render", When: "ne .Access \"gorm\""},
				{Template: "migrations/0001_init.up.sql.tmpl", Target: "migrations/0001_init.up.sql", Mode: "copy", When: "ne .Migrate \"\""},
				{Template: "migrations/0001_init.down.sql.tmpl", Target: "migrations/0001_init.down.sql", Mode: "copy", When: "ne .Migrate \"\""},
			},
			// HANYA driver go-sql-driver/mysql v1.10.0 (tanpa sqlx, selaras manifest).
			GoMod: []module.ModuleDep{
				{Path: "github.com/go-sql-driver/mysql", Version: "v1.10.0"},
			},
			Requires:  []string{modCore},
			Conflicts: []string{modDBPostgres},
			Contributes: []module.MergeContribution{
				{Target: "docker-compose.yml", Anchor: "services", Fragment: "fragments/compose.mysql.service.yml.tmpl", Order: 20, When: ".Docker"},
				{Target: ".env.example", Anchor: "database", Fragment: "fragments/env.mysql.tmpl", Order: 20, When: ".EnvExample"},
				// AUTO-WIRE database/sql DI-GATE `ne .Access "gorm"` (selaras manifest nyata).
				{Target: "cmd/{{ modBase .ModulePath }}/main.go", Anchor: "imports", Fragment: "fragments/main.imports.mysql.tmpl", Order: 20, When: "ne .Access \"gorm\""},
				{Target: "cmd/{{ modBase .ModulePath }}/main.go", Anchor: "wiring", Fragment: "fragments/main.wiring.mysql.tmpl", Order: 20, When: "ne .Access \"gorm\""},
			},
			Vars: map[string]any{"DBPort": 3306, "DBImage": "mysql:8.4"},
		},
		// access-gorm-postgres: lapisan akses GORM untuk PostgreSQL. AKTIF bila
		// access=gorm ∧ db=postgres. Menggantikan koneksi pgxpool db-postgres (di-gate
		// off) → koneksi GORM gorm.go + repository.go + AUTO-WIRE main. Fixture
		// MENCERMINKAN manifest nyata: gomod WAJIB EKSAK (gorm v1.31.1 + driver/postgres
		// v1.6.0, terverifikasi pkg.go.dev 2026-06-06), 2 file render, 2 contributes
		// AUTO-WIRE (imports/wiring order 25). requires core; conflicts access-gorm-mysql.
		modAccessGormPostgres: {
			Name:        modAccessGormPostgres,
			Description: "access GORM postgres (gorm.io/gorm + driver/postgres) + wiring main",
			Files: []module.FileSpec{
				{Template: "database/gorm.go.tmpl", Target: "internal/platform/database/gorm.go", Mode: "render"},
				{Template: "database/repository.go.tmpl", Target: "internal/platform/database/repository.go", Mode: "render"},
			},
			GoMod: []module.ModuleDep{
				{Path: "gorm.io/gorm", Version: "v1.31.1"},
				{Path: "gorm.io/driver/postgres", Version: "v1.6.0"},
			},
			Requires:  []string{modCore},
			Conflicts: []string{modAccessGormMySQL},
			Contributes: []module.MergeContribution{
				{Target: "cmd/{{ modBase .ModulePath }}/main.go", Anchor: "imports", Fragment: "fragments/main.imports.gorm.tmpl", Order: 25},
				{Target: "cmd/{{ modBase .ModulePath }}/main.go", Anchor: "wiring", Fragment: "fragments/main.wiring.gorm.tmpl", Order: 25},
			},
			Vars: map[string]any{"DBPort": 5432, "DBName": "app", "DBUser": "app"},
		},
		// access-gorm-mysql: analog access-gorm-postgres untuk MySQL (driver/mysql).
		// gomod WAJIB EKSAK (gorm v1.31.1 + driver/mysql v1.6.0).
		modAccessGormMySQL: {
			Name:        modAccessGormMySQL,
			Description: "access GORM mysql (gorm.io/gorm + driver/mysql) + wiring main",
			Files: []module.FileSpec{
				{Template: "database/gorm.go.tmpl", Target: "internal/platform/database/gorm.go", Mode: "render"},
				{Template: "database/repository.go.tmpl", Target: "internal/platform/database/repository.go", Mode: "render"},
			},
			GoMod: []module.ModuleDep{
				{Path: "gorm.io/gorm", Version: "v1.31.1"},
				{Path: "gorm.io/driver/mysql", Version: "v1.6.0"},
			},
			Requires:  []string{modCore},
			Conflicts: []string{modAccessGormPostgres},
			Contributes: []module.MergeContribution{
				{Target: "cmd/{{ modBase .ModulePath }}/main.go", Anchor: "imports", Fragment: "fragments/main.imports.gorm.tmpl", Order: 25},
				{Target: "cmd/{{ modBase .ModulePath }}/main.go", Anchor: "wiring", Fragment: "fragments/main.wiring.gorm.tmpl", Order: 25},
			},
			Vars: map[string]any{"DBPort": 3306, "DBName": "app", "DBUser": "app"},
		},
		// addon-ci: emit .github/workflows/ci.yml ATAU .gitlab-ci.yml tergantung
		// provider, masing-masing di-gate via `when` eq .CI "...". Hanya satu yang
		// aktif karena CI enum tunggal (C-ci).
		modAddonCI: {
			Name:        modAddonCI,
			Description: "CI workflow (GitHub Actions atau GitLab CI)",
			Files: []module.FileSpec{
				{Template: "github/ci.yml.tmpl", Target: ".github/workflows/ci.yml", Mode: "render", When: `eq .CI "github-actions"`},
				{Template: "gitlab/gitlab-ci.yml.tmpl", Target: ".gitlab-ci.yml", Mode: "render", When: `eq .CI "gitlab-ci"`},
			},
		},
		// addon-observability: OpenTelemetry tracing + prometheus /metrics +
		// health sebagai PUSTAKA siap-pakai di internal/platform/observability,
		// di-AUTO-WIRE ke server.go (import alias + endpoint /metrics).
		//
		// Fixture ini MENCERMINKAN PERSIS manifest nyata
		// (templates/modules/addon-observability/module.yaml): 3 file mode:copy ke
		// internal/platform/observability/{tracing,metrics,health}.go, 5 gomod
		// (prometheus + 4 otel pada versi terverifikasi 2026-06-06), dan 2
		// contributes AUTO-WIRE ke internal/httpserver/server.go (anchor imports
		// order 30, anchor routes order 30). Guard TestFakeRegistryMatchesRealManifests
		// menegakkan kesetaraan GoMod + Files + Contributes vs manifest nyata.
		modAddonObs: {
			Name:        modAddonObs,
			Description: "OpenTelemetry + prometheus /metrics (internal/platform/observability)",
			Files: []module.FileSpec{
				{Template: "observability/tracing.go.tmpl", Target: "internal/platform/observability/tracing.go", Mode: "copy"},
				{Template: "observability/metrics.go.tmpl", Target: "internal/platform/observability/metrics.go", Mode: "copy"},
				{Template: "observability/health.go.tmpl", Target: "internal/platform/observability/health.go", Mode: "copy"},
			},
			GoMod: []module.ModuleDep{
				{Path: "github.com/prometheus/client_golang", Version: "v1.23.2"},
				{Path: "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp", Version: "v0.69.0"},
				{Path: "go.opentelemetry.io/otel", Version: "v1.44.0"},
				{Path: "go.opentelemetry.io/otel/exporters/stdout/stdouttrace", Version: "v1.44.0"},
				{Path: "go.opentelemetry.io/otel/sdk", Version: "v1.44.0"},
			},
			Requires: []string{modCore},
			Contributes: []module.MergeContribution{
				// Var OTEL ke anchor "app" .env.example milik core (order 30).
				{Target: ".env.example", Anchor: "app", Fragment: "fragments/env.obs.tmpl", Order: 30, When: ".EnvExample"},
				// AUTO-WIRE: import alias obs → region:imports server (order 30).
				{Target: "internal/httpserver/server.go", Anchor: "imports", Fragment: "fragments/server.imports.obs.tmpl", Order: 30},
				// AUTO-WIRE: endpoint /metrics Prometheus → region:routes server (order 30).
				{Target: "internal/httpserver/server.go", Anchor: "routes", Fragment: "fragments/server.routes.obs.tmpl", Order: 30},
			},
		},
		modAddonDocker: {
			Name:        modAddonDocker,
			Description: "Dockerfile + kontribusi service app ke compose",
			Files: []module.FileSpec{
				{Template: "Dockerfile.tmpl", Target: "Dockerfile", Mode: "render"},
				// .dockerignore selaras manifest nyata (templates/modules/addon-docker).
				{Template: "dockerignore.tmpl", Target: ".dockerignore", Mode: "render"},
			},
			Requires: []string{modCore},
			Contributes: []module.MergeContribution{
				{Target: "docker-compose.yml", Anchor: "services", Fragment: "fragments/compose.app.yml.tmpl", Order: 10},
			},
		},
		modAddonMake: {
			Name:        modAddonMake,
			Description: "kontribusi target ke Makefile (skeleton dimiliki core)",
			Contributes: []module.MergeContribution{
				{Target: "Makefile", Anchor: "targets", Fragment: "fragments/make.run.tmpl", Order: 10},
			},
		},
		modAddonLint: {
			Name:        modAddonLint,
			Description: "golangci-lint config",
			Files: []module.FileSpec{
				{Template: "golangci.yml.tmpl", Target: ".golangci.yml", Mode: "copy"},
			},
		},
		modAddonEnv: {
			Name:        modAddonEnv,
			Description: "kontribusi vars ke .env.example (skeleton dimiliki core)",
			Contributes: []module.MergeContribution{
				{Target: ".env.example", Anchor: "app", Fragment: "fragments/env.app.extra.tmpl", Order: 5, When: ".EnvExample"},
			},
		},
		// feature-strapgorm (v1 bounded): domain contoh Product di atas GORM via
		// strapgorm query builder. AKTIF hanya saat access=gorm ∧ db∈{postgres,mysql}
		// ∧ arch=monolith (ditegakkan resolver). Me-REUSE *gorm.DB milik access-gorm-*
		// (requires core; access-gorm-* dijamin aktif resolver). Fixture MENCERMINKAN
		// PERSIS manifest nyata (templates/modules/feature-strapgorm/module.yaml):
		// 4 file domain internal/product/** (model/repository/handler/wiring, mode
		// render), GoMod EKSAK strapgorm @ pseudo-version pin (gorm + driver sudah dari
		// access-gorm; tak diulang), dan 4 contributes AUTO-WIRE order 30 — main.go
		// (imports+wiring: product.SetDB(db)+AutoMigrate me-REUSE var db) & server.go
		// (imports+routes: product.Mount(mux) → GET /api/products). Guard
		// TestFakeRegistryMatchesRealManifests menegakkan kesetaraan (cmpExact).
		modFeatureStrapgorm: {
			Name:        modFeatureStrapgorm,
			Description: "domain Product (strapgorm query builder di atas GORM)",
			Requires:    []string{modCore},
			Files: []module.FileSpec{
				{Template: "internal/product/model.go.tmpl", Target: "internal/product/model.go", Mode: "render"},
				{Template: "internal/product/repository.go.tmpl", Target: "internal/product/repository.go", Mode: "render"},
				{Template: "internal/product/handler.go.tmpl", Target: "internal/product/handler.go", Mode: "render"},
				{Template: "internal/product/wiring.go.tmpl", Target: "internal/product/wiring.go", Mode: "render"},
			},
			GoMod: []module.ModuleDep{
				{Path: strapgormModulePath, Version: strapgormVersion},
			},
			Contributes: []module.MergeContribution{
				// AUTO-WIRE main: import product + product.SetDB(db)+AutoMigrate
				// (order 30 > 25 access-gorm: var `db` sudah dideklarasikan fragmen GORM).
				{Target: "cmd/{{ modBase .ModulePath }}/main.go", Anchor: "imports", Fragment: "fragments/main.imports.strapgorm.tmpl", Order: 30},
				{Target: "cmd/{{ modBase .ModulePath }}/main.go", Anchor: "wiring", Fragment: "fragments/main.wiring.strapgorm.tmpl", Order: 30},
				// AUTO-WIRE server.go (monolith net/http): import product + Mount(mux)
				// → rute GET /api/products pada anchor routes httpserver.New.
				{Target: "internal/httpserver/server.go", Anchor: "imports", Fragment: "fragments/server.imports.strapgorm.tmpl", Order: 30},
				{Target: "internal/httpserver/server.go", Anchor: "routes", Fragment: "fragments/server.routes.strapgorm.tmpl", Order: 30},
			},
		},
	})
}

// baseAnswers mengembalikan Answers valid minimal untuk monolith REST.
func baseAnswers() answers.Answers {
	return answers.Answers{
		Name:   "shop",
		Module: "github.com/acme/shop",
		Arch:   answers.ArchMonolith,
		Kind:   answers.KindREST,
		HTTP:   answers.HTTPNetHTTP,
		DB:     answers.DBNone,
	}
}

// fileOpByTarget mencari FileOp dengan TargetPath tertentu.
func fileOpByTarget(files []plan.FileOp, target string) (plan.FileOp, bool) {
	for _, f := range files {
		if f.TargetPath == target {
			return f, true
		}
	}
	return plan.FileOp{}, false
}

func TestResolve_MonolithNetHTTP_NoDB(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.Docker = true
	a.Makefile = true

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}

	// GoMod harus KOSONG untuk db=none (invarian: murni stdlib, zero require).
	if len(p.Deps) != 0 {
		t.Errorf("db=none harus GoMod kosong, dapat %d: %+v", len(p.Deps), p.Deps)
	}

	// Metadata plan.
	if p.ProjectName != "shop" || p.ModulePath != "github.com/acme/shop" {
		t.Errorf("metadata plan salah: name=%q module=%q", p.ProjectName, p.ModulePath)
	}
	if p.GoVersion != goVersionDefault {
		t.Errorf("GoVersion = %q, mau %q", p.GoVersion, goVersionDefault)
	}

	// Harus ada FileOp dari core, arch, dan addon (docker+makefile).
	// cmd/shop/main.go: target placeholder cmd/{{ modBase .Module }}/main.go
	// dirender resolver → modBase("github.com/acme/shop")="shop".
	wantTargets := []string{
		"README.md",           // core
		".gitignore",          // core
		"cmd/shop/main.go",    // core (skeleton main, target placeholder dirender)
		"internal/app/app.go", // arch-monolith
		"Dockerfile",          // addon-docker
		"Makefile",            // core skeleton + kontribusi addon-makefile (ModeMerge)
		"docker-compose.yml",  // core skeleton + kontribusi addon-docker (ModeMerge)
	}
	for _, target := range wantTargets {
		if _, ok := fileOpByTarget(p.Files, target); !ok {
			t.Errorf("FileOp untuk %q tidak ada di plan", target)
		}
	}

	// TIDAK boleh ada FileOp postgres (db=none).
	if _, ok := fileOpByTarget(p.Files, "internal/platform/database/postgres.go"); ok {
		t.Errorf("postgres.go tidak boleh ada saat db=none")
	}
	// TIDAK boleh ada migration (db=none).
	if _, ok := fileOpByTarget(p.Files, "migrations/0001_init.up.sql"); ok {
		t.Errorf("migration tidak boleh ada saat db=none")
	}

	// compose ada (docker aktif) sebagai ModeMerge; service db tidak boleh muncul
	// karena fragmen postgres tidak aktif (db=none) — hanya app dari addon-docker.
	if compose, ok := fileOpByTarget(p.Files, "docker-compose.yml"); ok {
		if compose.Mode != plan.ModeMerge {
			t.Errorf("docker-compose.yml harus ModeMerge, dapat %v", compose.Mode)
		}
		for _, fr := range compose.Fragments {
			if got := fr.Content; strings.Contains(got, "compose.postgres") {
				t.Errorf("compose tidak boleh punya service db saat db=none, ketemu fragmen: %q", got)
			}
		}
	} else {
		t.Errorf("docker-compose.yml harus ada saat docker aktif")
	}

	// Hooks: gofmt + go-mod-tidy, tanpa git-init (Git=false).
	if !hasHook(p.Hooks, "gofmt") || !hasHook(p.Hooks, "go-mod-tidy") {
		t.Errorf("hooks wajib gofmt + go-mod-tidy, dapat %+v", p.Hooks)
	}
	if hasHook(p.Hooks, "git-init") {
		t.Errorf("git-init tidak boleh ada saat Git=false")
	}

	// Urutan FileOp deterministik by TargetPath.
	for i := 1; i < len(p.Files); i++ {
		if p.Files[i-1].TargetPath > p.Files[i].TargetPath {
			t.Errorf("FileOp tidak terurut by TargetPath: %q > %q", p.Files[i-1].TargetPath, p.Files[i].TargetPath)
		}
	}
}

func TestResolve_MonolithNetHTTP_Postgres(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.DB = answers.DBPostgres
	a.Docker = true
	a.Makefile = true
	a.Git = true
	// Access & Migrate dibiarkan kosong → resolver default ke sqlx + golang-migrate.

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}

	// GoMod harus berisi HANYA pgx (sqlx DIHAPUS — selaras manifest nyata
	// templates/modules/db-postgres: hanya pgx/v5 yang di-import skeleton; sqlx akan
	// di-prune go mod tidy karena belum dipakai kode, jadi tak dicantumkan).
	wantDeps := map[string]string{
		"github.com/jackc/pgx/v5": "v5.10.0",
	}
	if len(p.Deps) != len(wantDeps) {
		t.Fatalf("Deps = %d, mau %d: %+v", len(p.Deps), len(wantDeps), p.Deps)
	}
	for _, d := range p.Deps {
		if v, ok := wantDeps[d.Path]; !ok || v != d.Version {
			t.Errorf("dep tak terduga / versi salah: %+v", d)
		}
	}
	// sqlx TIDAK boleh muncul (regression guard fix code-review #4).
	if hasDep(p.Deps, "github.com/jmoiron/sqlx") {
		t.Errorf("sqlx tidak boleh ada di db-postgres (hanya pgx): %+v", p.Deps)
	}
	// Satu-satunya dep: pgx.
	if p.Deps[0].Path != "github.com/jackc/pgx/v5" {
		t.Errorf("dep tunggal harus pgx: %+v", p.Deps)
	}

	// Migration FileOp harus ada (db=postgres + migrate default golang-migrate → when 'ne .Migrate ""' true).
	if _, ok := fileOpByTarget(p.Files, "migrations/0001_init.up.sql"); !ok {
		t.Errorf("migration up tidak ada di plan saat db=postgres")
	}
	if _, ok := fileOpByTarget(p.Files, "migrations/0001_init.down.sql"); !ok {
		t.Errorf("migration down tidak ada di plan saat db=postgres")
	}
	// postgres.go ada.
	if _, ok := fileOpByTarget(p.Files, "internal/platform/database/postgres.go"); !ok {
		t.Errorf("postgres.go tidak ada di plan saat db=postgres")
	}

	// compose punya service db (fragmen postgres aktif karena docker + db=postgres).
	compose, ok := fileOpByTarget(p.Files, "docker-compose.yml")
	if !ok {
		t.Fatalf("docker-compose.yml tidak ada di plan")
	}
	if compose.Mode != plan.ModeMerge {
		t.Errorf("compose harus ModeMerge, dapat %v", compose.Mode)
	}
	foundPostgresFrag := false
	for _, fr := range compose.Fragments {
		if strings.Contains(fr.Content, "compose.postgres") {
			foundPostgresFrag = true
		}
	}
	if !foundPostgresFrag {
		t.Errorf("compose harus punya fragmen service db postgres, fragments: %+v", compose.Fragments)
	}
	// Fragmen compose harus terurut by Order: app(10) sebelum postgres(20).
	if len(compose.Fragments) >= 2 {
		if compose.Fragments[0].Order > compose.Fragments[1].Order {
			t.Errorf("fragmen compose tidak terurut by Order: %+v", compose.Fragments)
		}
	}

	// Git=true → git-init hadir di hooks.
	if !hasHook(p.Hooks, "git-init") {
		t.Errorf("git-init harus ada saat Git=true, hooks: %+v", p.Hooks)
	}

	// Vars modul (DBImage) harus masuk Data context.
	if compose.Data != nil {
		if m, ok := compose.Data.(map[string]any); ok {
			if m["DBImage"] != "postgres:17-alpine" {
				t.Errorf("Vars DBImage tidak ter-merge ke Data: %v", m["DBImage"])
			}
		}
	}
}

// TestResolve_Postgres_AccessGORM memverifikasi jalur access=gorm + db=postgres:
//   - dep gorm + driver/postgres ada (versi benar); pgx (pgxpool langsung) TIDAK
//     ikut karena koneksi pgxpool db-postgres di-gate off (driver postgres GORM
//     menarik pgx transitif via go mod tidy, bukan deklarasi langsung);
//   - file koneksi GORM (gorm.go + repository.go) ter-emit;
//   - file koneksi pgxpool (internal/platform/database/postgres.go) TIDAK ter-emit;
//   - wiring main memakai fragmen GORM, bukan fragmen pgxpool (tepat satu koneksi).
func TestResolve_Postgres_AccessGORM(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.DB = answers.DBPostgres
	a.Access = answers.AccessGORM

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}

	// Deps: gorm + driver/postgres (versi terverifikasi). pgxpool langsung TIDAK ada.
	if !hasDep(p.Deps, "gorm.io/gorm") {
		t.Errorf("gorm.io/gorm harus ada saat access=gorm: %+v", p.Deps)
	}
	for _, d := range p.Deps {
		if d.Path == "gorm.io/gorm" && d.Version != "v1.31.1" {
			t.Errorf("gorm.io/gorm versi salah: %q (mau v1.31.1)", d.Version)
		}
		if d.Path == "gorm.io/driver/postgres" && d.Version != "v1.6.0" {
			t.Errorf("driver/postgres versi salah: %q (mau v1.6.0)", d.Version)
		}
	}
	if !hasDep(p.Deps, "gorm.io/driver/postgres") {
		t.Errorf("gorm.io/driver/postgres harus ada saat access=gorm+postgres: %+v", p.Deps)
	}
	// Catatan honest-go.mod: pgx/v5 TETAP dideklarasikan db-postgres (gomod tak bisa
	// di-gate per-FileSpec). Saat access=gorm koneksi pgxpool tak di-emit, namun
	// gorm.io/driver/postgres sendiri MEM-BUTUH pgx/v5 (direct dep di go.mod-nya) →
	// `go mod tidy` mendemosikannya ke `// indirect`, BUKAN menghapus. go.mod tetap
	// jujur pasca-tidy & build hijau. Jadi pgx boleh hadir di plan.Deps (pre-tidy).

	// File koneksi GORM ter-emit; koneksi pgxpool TIDAK.
	if _, ok := fileOpByTarget(p.Files, "internal/platform/database/gorm.go"); !ok {
		t.Errorf("gorm.go harus ter-emit saat access=gorm")
	}
	if _, ok := fileOpByTarget(p.Files, "internal/platform/database/repository.go"); !ok {
		t.Errorf("repository.go (GORM) harus ter-emit saat access=gorm")
	}
	if _, ok := fileOpByTarget(p.Files, "internal/platform/database/postgres.go"); ok {
		t.Errorf("postgres.go (pgxpool) TIDAK boleh ter-emit saat access=gorm (double-wiring koneksi)")
	}

	// Wiring main: fragmen GORM hadir, fragmen pgxpool TIDAK.
	main, ok := fileOpByTarget(p.Files, "cmd/shop/main.go")
	if !ok {
		t.Fatalf("cmd/shop/main.go tidak ada di plan")
	}
	var hasGormWiring, hasPgxWiring bool
	for _, fr := range main.Fragments {
		if strings.Contains(fr.Content, "main.wiring.gorm") || strings.Contains(fr.Content, "main.imports.gorm") {
			hasGormWiring = true
		}
		if strings.Contains(fr.Content, "main.wiring.postgres") || strings.Contains(fr.Content, "main.imports.postgres") {
			hasPgxWiring = true
		}
	}
	if !hasGormWiring {
		t.Errorf("wiring GORM harus ter-AUTO-WIRE ke main saat access=gorm, fragments: %+v", main.Fragments)
	}
	if hasPgxWiring {
		t.Errorf("wiring pgxpool TIDAK boleh ter-emit saat access=gorm (digate off), fragments: %+v", main.Fragments)
	}
}

// TestResolve_MySQL_AccessGORM memverifikasi jalur access=gorm + db=mysql (analog
// postgres): dep gorm + driver/mysql ada, koneksi GORM ter-emit, koneksi
// database/sql db-mysql + wiringnya TIDAK ikut.
func TestResolve_MySQL_AccessGORM(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.DB = answers.DBMySQL
	a.Access = answers.AccessGORM

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}

	if !hasDep(p.Deps, "gorm.io/gorm") || !hasDep(p.Deps, "gorm.io/driver/mysql") {
		t.Errorf("gorm + driver/mysql harus ada saat access=gorm+mysql: %+v", p.Deps)
	}
	for _, d := range p.Deps {
		if d.Path == "gorm.io/driver/mysql" && d.Version != "v1.6.0" {
			t.Errorf("driver/mysql versi salah: %q (mau v1.6.0)", d.Version)
		}
	}
	// Honest-go.mod: go-sql-driver/mysql TETAP dideklarasikan db-mysql (gomod tak
	// di-gate), tetapi gorm.io/driver/mysql mem-butuh-nya → `go mod tidy` mendemosikan
	// ke `// indirect`. Build hijau. (Sama dengan kasus pgx di postgres.)
	if _, ok := fileOpByTarget(p.Files, "internal/platform/database/gorm.go"); !ok {
		t.Errorf("gorm.go harus ter-emit saat access=gorm+mysql")
	}
	if _, ok := fileOpByTarget(p.Files, "internal/platform/database/mysql.go"); ok {
		t.Errorf("mysql.go (database/sql) TIDAK boleh ter-emit saat access=gorm")
	}
}

// ── Strapgorm (v1 bounded): add-on Product domain via GORM ────────────────────

// TestResolve_Strapgorm_Postgres memverifikasi jalur HAPPY add-on strapgorm pada
// kombinasi yang diizinkan (monolith + access=gorm + db=postgres):
//   - modul feature-strapgorm aktif → file domain internal/product/** ter-emit;
//   - dep strapgorm hadir dengan PIN versi EKSAK (pseudo-version), berdampingan
//     dengan gorm + driver/postgres (dari access-gorm); pool kedua TIDAK dibuka
//     (postgres.go pgxpool tetap di-gate off oleh access=gorm);
//   - route /api/products ter-AUTO-WIRE ke main.go (anchor imports+wiring);
//   - GoVersion project = 1.25 (strapgorm butuh Go ≥ 1.25).
func TestResolve_Strapgorm_Postgres(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.DB = answers.DBPostgres
	a.Access = answers.AccessGORM
	a.Strapgorm = true

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve strapgorm gagal: %v", err)
	}

	// File domain Product ter-emit (semua dimiliki feature-strapgorm). Selaras
	// manifest nyata: model + repository + handler + wiring (BUKAN service.go).
	for _, target := range []string{
		"internal/product/model.go",
		"internal/product/repository.go",
		"internal/product/handler.go",
		"internal/product/wiring.go",
	} {
		op, ok := fileOpByTarget(p.Files, target)
		if !ok {
			t.Errorf("file domain Product %q harus ter-emit saat strapgorm aktif", target)
			continue
		}
		if op.ModuleName != modFeatureStrapgorm {
			t.Errorf("%q harus dimiliki %q, dapat %q", target, modFeatureStrapgorm, op.ModuleName)
		}
	}

	// Dep strapgorm hadir dengan PIN EKSAK (byte-identical, §5.2).
	if !hasDep(p.Deps, strapgormModulePath) {
		t.Fatalf("dep strapgorm harus ada saat strapgorm aktif: %+v", p.Deps)
	}
	for _, d := range p.Deps {
		if d.Path == strapgormModulePath && d.Version != strapgormVersion {
			t.Errorf("strapgorm versi salah: %q (mau pin %q)", d.Version, strapgormVersion)
		}
	}
	// gorm + driver/postgres tetap hadir (dari access-gorm) — strapgorm di ATAS GORM.
	if !hasDep(p.Deps, "gorm.io/gorm") || !hasDep(p.Deps, "gorm.io/driver/postgres") {
		t.Errorf("gorm + driver/postgres harus tetap ada (strapgorm di atas GORM): %+v", p.Deps)
	}
	// Pool kedua TIDAK dibuka: koneksi pgxpool tetap di-gate off oleh access=gorm.
	if _, ok := fileOpByTarget(p.Files, "internal/platform/database/postgres.go"); ok {
		t.Errorf("postgres.go (pgxpool) TIDAK boleh ter-emit (strapgorm REUSE *gorm.DB, bukan pool kedua)")
	}
	if _, ok := fileOpByTarget(p.Files, "internal/platform/database/gorm.go"); !ok {
		t.Errorf("gorm.go harus ter-emit (strapgorm REUSE koneksi GORM access-gorm)")
	}

	// AUTO-WIRE route /api/products ke main.go (anchor imports + wiring order 30).
	main, ok := fileOpByTarget(p.Files, "cmd/shop/main.go")
	if !ok {
		t.Fatalf("cmd/shop/main.go harus ada di plan")
	}
	if main.Mode != plan.ModeMerge {
		t.Fatalf("main.go harus ModeMerge saat strapgorm auto-wire, dapat %v", main.Mode)
	}
	var sawImports, sawWiring bool
	for _, fr := range main.Fragments {
		if fr.Anchor == "imports" && strings.Contains(fr.Content, "main.imports.strapgorm") {
			sawImports = true
		}
		if fr.Anchor == "wiring" && strings.Contains(fr.Content, "main.wiring.strapgorm") {
			sawWiring = true
		}
	}
	if !sawImports {
		t.Errorf("main.go harus punya fragmen import product (strapgorm) ke anchor imports, fragments: %+v", main.Fragments)
	}
	if !sawWiring {
		t.Errorf("main.go harus punya fragmen wiring /api/products ke anchor wiring, fragments: %+v", main.Fragments)
	}

	// GoVersion project = 1.25 (strapgorm butuh Go ≥ 1.25).
	if p.GoVersion != "1.25" {
		t.Errorf("GoVersion = %q, mau \"1.25\" (strapgorm butuh Go ≥ 1.25)", p.GoVersion)
	}
}

// TestResolve_Strapgorm_MySQL memverifikasi happy path analog pada db=mysql:
// feature-strapgorm aktif, dep strapgorm pin benar, gorm + driver/mysql hadir,
// GoVersion 1.25.
func TestResolve_Strapgorm_MySQL(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.DB = answers.DBMySQL
	a.Access = answers.AccessGORM
	a.Strapgorm = true

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve strapgorm+mysql gagal: %v", err)
	}
	if _, ok := fileOpByTarget(p.Files, "internal/product/handler.go"); !ok {
		t.Errorf("handler Product harus ter-emit saat strapgorm+mysql aktif")
	}
	if !hasDep(p.Deps, strapgormModulePath) {
		t.Errorf("dep strapgorm harus ada: %+v", p.Deps)
	}
	if !hasDep(p.Deps, "gorm.io/driver/mysql") {
		t.Errorf("gorm driver/mysql harus tetap ada (strapgorm di atas GORM): %+v", p.Deps)
	}
	if p.GoVersion != "1.25" {
		t.Errorf("GoVersion = %q, mau \"1.25\"", p.GoVersion)
	}
}

// TestResolve_Strapgorm_Deterministic memverifikasi byte-identical: dua Resolve
// dengan Answers strapgorm yang sama menghasilkan plan IDENTIK (deps sama, dep
// strapgorm pin sama persis, FileOp sama).
func TestResolve_Strapgorm_Deterministic(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.DB = answers.DBPostgres
	a.Access = answers.AccessGORM
	a.Strapgorm = true

	p1, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve p1 gagal: %v", err)
	}
	p2, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve p2 gagal: %v", err)
	}
	if len(p1.Files) != len(p2.Files) || len(p1.Deps) != len(p2.Deps) {
		t.Fatalf("plan tidak deterministik: files %d/%d deps %d/%d", len(p1.Files), len(p2.Files), len(p1.Deps), len(p2.Deps))
	}
	for i := range p1.Files {
		if p1.Files[i].TargetPath != p2.Files[i].TargetPath {
			t.Errorf("FileOp[%d] beda: %q vs %q", i, p1.Files[i].TargetPath, p2.Files[i].TargetPath)
		}
	}
	for i := range p1.Deps {
		if p1.Deps[i] != p2.Deps[i] {
			t.Errorf("Dep[%d] beda: %+v vs %+v", i, p1.Deps[i], p2.Deps[i])
		}
	}
}

// TestResolve_Strapgorm_RejectNonGorm memverifikasi penolakan strapgorm tanpa
// access=gorm (mis. sqlx) → ErrConstraint (C-strapgorm), pesan menyebut syarat.
func TestResolve_Strapgorm_RejectNonGorm(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.DB = answers.DBPostgres
	a.Access = answers.AccessSQLx // bukan gorm → invalid.
	a.Strapgorm = true

	_, err := r.Resolve(a)
	if err == nil {
		t.Fatalf("strapgorm tanpa access=gorm harus ditolak")
	}
	if !strings.Contains(err.Error(), "strapgorm") || !strings.Contains(err.Error(), "gorm") {
		t.Errorf("pesan tolak harus menyebut strapgorm + access gorm: %v", err)
	}
}

// TestResolve_Strapgorm_RejectDBNone memverifikasi penolakan strapgorm saat
// db=none. Ditolak RAMAH di answers.Validate (entry-point, lapis pertama) sebelum
// resolver — strapgorm butuh db∈{postgres,mysql}. (Lapis kedua C-strapgorm di
// checkConstraints mengembalikan ErrConstraint, tetapi Validate menang lebih dulu;
// di sini cukup pastikan Resolve gagal dengan pesan menyebut syarat strapgorm.)
func TestResolve_Strapgorm_RejectDBNone(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.DB = answers.DBNone
	a.Access = answers.AccessGORM
	a.Strapgorm = true

	_, err := r.Resolve(a)
	if err == nil {
		t.Fatalf("strapgorm dengan db=none harus ditolak")
	}
	if !strings.Contains(err.Error(), "strapgorm") || !strings.Contains(err.Error(), "postgres|mysql") {
		t.Errorf("pesan tolak harus menyebut strapgorm + syarat db postgres|mysql: %v", err)
	}
}

// realRegistry memuat registry NYATA dari embed.FS — dipakai test yang melibatkan
// modul strapgorm modular/microservice (tak ada di fixture fake mvpRegistry; fake
// hanya menutup subset monolith MVP, lihat guard TestFakeRegistryMatchesRealManifests).
func realRegistry(t *testing.T) module.Registry {
	t.Helper()
	reg := module.NewRegistry()
	if err := reg.Load(templates.FS); err != nil {
		t.Fatalf("Load registry nyata (templates.FS) gagal: %v", err)
	}
	return reg
}

// TestResolve_Strapgorm_Microservice memverifikasi jalur HAPPY strapgorm pada
// arch=microservice: service product MANDIRI (gRPC + HTTP strapgorm) ter-emit lewat
// feature-strapgorm-microservice + driver -postgres; go.mod jujur (gorm + strapgorm +
// driver postgres saja); GoVersion 1.25.
func TestResolve_Strapgorm_Microservice(t *testing.T) {
	r := New(realRegistry(t))
	a := microAnswers("svc-a", "svc-b")
	a.DB = answers.DBPostgres
	a.Access = answers.AccessGORM
	a.Strapgorm = true

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("strapgorm pada arch=microservice harus resolve: %v", err)
	}
	// Service product ter-emit (proto + kode service) oleh modul shared microservice.
	for _, target := range []string{
		"proto/product/v1/product.proto",
		"services/product/cmd/main.go",
		"services/product/internal/config/config.go",
		"services/product/internal/server/server.go",
		"services/product/internal/store/store.go",
		"services/product/internal/store/model.go",
		"services/product/internal/store/repository.go",
		"services/product/internal/store/handler.go",
	} {
		op, ok := fileOpByTarget(p.Files, target)
		if !ok {
			t.Errorf("file service product %q harus ter-emit saat strapgorm microservice", target)
			continue
		}
		if op.ModuleName != modFeatureStrapgormMicro {
			t.Errorf("%q harus dimiliki %q, dapat %q", target, modFeatureStrapgormMicro, op.ModuleName)
		}
	}
	// go.mod JUJUR: gorm + strapgorm + driver postgres (BUKAN mysql), plus grpc/protobuf.
	if !hasDep(p.Deps, strapgormModulePath) || !hasDep(p.Deps, "gorm.io/gorm") || !hasDep(p.Deps, "gorm.io/driver/postgres") {
		t.Errorf("dep gorm+strapgorm+driver/postgres harus ada: %+v", p.Deps)
	}
	if hasDep(p.Deps, "gorm.io/driver/mysql") {
		t.Errorf("driver/mysql TIDAK boleh ada saat db=postgres (go.mod jujur): %+v", p.Deps)
	}
	// docker-compose merge memuat fragmen service product + DB postgres + volume.
	compose, ok := fileOpByTarget(p.Files, "docker-compose.yml")
	if !ok || compose.Mode != plan.ModeMerge {
		t.Fatalf("docker-compose.yml harus ModeMerge dengan kontribusi product+db")
	}
	var sawProduct, sawDB, sawVol bool
	for _, fr := range compose.Fragments {
		if strings.Contains(fr.Content, "compose.product.service") {
			sawProduct = true
		}
		if strings.Contains(fr.Content, "compose.db.service") {
			sawDB = true
		}
		if fr.Anchor == "volumes" && strings.Contains(fr.Content, "compose.db.volume") {
			sawVol = true
		}
	}
	if !sawProduct || !sawDB || !sawVol {
		t.Errorf("compose harus punya fragmen product+db+volume: product=%v db=%v vol=%v", sawProduct, sawDB, sawVol)
	}
	if p.GoVersion != "1.25" {
		t.Errorf("GoVersion = %q, mau \"1.25\"", p.GoVersion)
	}
}

// TestResolve_Strapgorm_Modular memverifikasi jalur HAPPY strapgorm pada
// arch=modular-monolith: Product = domain modular kelas-satu (facade + internal/core)
// di-emit feature-strapgorm-modular; di-AUTO-WIRE ke cmd/<app>/main.go via anchor
// imports/wiring/modules; dep strapgorm + gorm; GoVersion 1.25.
func TestResolve_Strapgorm_Modular(t *testing.T) {
	r := New(realRegistry(t))
	a := baseAnswers()
	a.Arch = answers.ArchModularMonolith
	a.DB = answers.DBPostgres
	a.Access = answers.AccessGORM
	a.Strapgorm = true

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("strapgorm pada arch=modular-monolith harus resolve: %v", err)
	}
	for _, target := range []string{
		"internal/modules/product/product.go",
		"internal/modules/product/internal/core/model.go",
		"internal/modules/product/internal/core/repository.go",
		"internal/modules/product/internal/core/handler.go",
	} {
		op, ok := fileOpByTarget(p.Files, target)
		if !ok {
			t.Errorf("file domain Product modular %q harus ter-emit", target)
			continue
		}
		if op.ModuleName != modFeatureStrapgormModular {
			t.Errorf("%q harus dimiliki %q, dapat %q", target, modFeatureStrapgormModular, op.ModuleName)
		}
	}
	if !hasDep(p.Deps, strapgormModulePath) || !hasDep(p.Deps, "gorm.io/gorm") || !hasDep(p.Deps, "gorm.io/driver/postgres") {
		t.Errorf("dep gorm+strapgorm+driver/postgres harus ada (reuse access-gorm): %+v", p.Deps)
	}
	// AUTO-WIRE ke composition root cmd/shop/main.go: anchor imports+wiring+modules.
	main, ok := fileOpByTarget(p.Files, "cmd/shop/main.go")
	if !ok || main.Mode != plan.ModeMerge {
		t.Fatalf("cmd/shop/main.go harus ModeMerge (auto-wire strapgorm modular)")
	}
	var sawImports, sawWiring, sawModules bool
	for _, fr := range main.Fragments {
		if fr.Anchor == "imports" && strings.Contains(fr.Content, "main.imports.strapgorm") {
			sawImports = true
		}
		if fr.Anchor == "wiring" && strings.Contains(fr.Content, "main.wiring.strapgorm") {
			sawWiring = true
		}
		if fr.Anchor == "modules" && strings.Contains(fr.Content, "main.modules.strapgorm") {
			sawModules = true
		}
	}
	if !sawImports || !sawWiring || !sawModules {
		t.Errorf("main.go harus punya fragmen imports+wiring+modules strapgorm: imports=%v wiring=%v modules=%v", sawImports, sawWiring, sawModules)
	}
	if p.GoVersion != "1.25" {
		t.Errorf("GoVersion = %q, mau \"1.25\"", p.GoVersion)
	}
}

// TestResolve_NoStrapgorm_GormUnaffected memverifikasi REGRESI: access=gorm tanpa
// strapgorm TIDAK terpengaruh — tak ada file product, tak ada dep strapgorm,
// GoVersion tetap default 1.24 (strapgorm tidak menaikkan tanpa diaktifkan).
func TestResolve_NoStrapgorm_GormUnaffected(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.DB = answers.DBPostgres
	a.Access = answers.AccessGORM
	// Strapgorm = false.

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}
	if _, ok := fileOpByTarget(p.Files, "internal/product/model.go"); ok {
		t.Errorf("file Product TIDAK boleh ada saat strapgorm=false")
	}
	if hasDep(p.Deps, strapgormModulePath) {
		t.Errorf("dep strapgorm TIDAK boleh ada saat strapgorm=false: %+v", p.Deps)
	}
	if p.GoVersion != goVersionDefault {
		t.Errorf("GoVersion = %q, mau %q (strapgorm tidak menaikkan tanpa diaktifkan)", p.GoVersion, goVersionDefault)
	}
}

func TestResolve_Constraint_MigrateNeedsDB(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.DB = answers.DBNone
	a.Migrate = answers.MigrateGolangMigrate // C1: migrate tanpa db → invalid.

	_, err := r.Resolve(a)
	if err == nil {
		t.Fatalf("mau error C1 (migrate butuh db), dapat nil")
	}
	if !errors.Is(err, ErrConstraint) {
		t.Errorf("error harus ErrConstraint, dapat: %v", err)
	}
}

func TestResolve_InvalidName(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.Name = "Shop App" // melanggar regex name.

	if _, err := r.Resolve(a); err == nil {
		t.Errorf("nama invalid harus gagal validasi")
	}
}

func TestEvalWhen(t *testing.T) {
	a := answers.Answers{
		Arch:    answers.ArchMonolith,
		Kind:    answers.KindREST,
		HTTP:    answers.HTTPNetHTTP,
		DB:      answers.DBPostgres,
		Migrate: answers.MigrateGolangMigrate,
		Docker:  true,
		Obs:     false,
		Auth:    answers.AuthNone,
	}

	cases := []struct {
		name    string
		expr    string
		want    bool
		wantErr bool
	}{
		{"kosong selalu true", "", true, false},
		{"spasi selalu true", "   ", true, false},
		{"bool field true", ".Docker", true, false},
		{"bool field false", ".Obs", false, false},
		{"not bool", "not .Docker", false, false},
		{"not not", "not (not .Docker)", true, false},
		{"eq string match", `eq .Arch "monolith"`, true, false},
		{"eq string mismatch", `eq .Arch "microservice"`, false, false},
		{"ne string", `ne .Migrate ""`, true, false},
		{"ne string empty true", `ne .Auth "none"`, false, false},
		{"and true", `and (eq .Arch "monolith") .Docker`, true, false},
		{"and false", `and (eq .Arch "monolith") .Obs`, false, false},
		{"or true", `or .Obs .Docker`, true, false},
		{"or false", `or .Obs (eq .HTTP "fiber")`, false, false},
		{"presedensi and over or", `or .Obs (and .Docker (eq .DB "postgres"))`, true, false},
		{"kurung", `and (or .Obs .Docker) (eq .DB "postgres")`, true, false},
		{"eq dengan number", `eq .DB "postgres"`, true, false},
		// Fail-fast: field ilegal.
		{"field ilegal", ".Bogus", false, true},
		{"field ilegal di eq", `eq .Bogus "x"`, false, true},
		{"string field sbg atom bool", ".Arch", false, true},
		{"sintaks rusak kurung", `and (.Docker`, false, true},
		{"operator di atom", "and", false, true},
		{"token sisa", `.Docker .Obs`, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := evalWhen(tc.expr, a)
			if tc.wantErr {
				if err == nil {
					t.Errorf("evalWhen(%q) mau error, dapat nil (got=%v)", tc.expr, got)
				}
				return
			}
			if err != nil {
				t.Errorf("evalWhen(%q) error tak terduga: %v", tc.expr, err)
				return
			}
			if got != tc.want {
				t.Errorf("evalWhen(%q) = %v, mau %v", tc.expr, got, tc.want)
			}
		})
	}
}

// TestCheckRelations_RequiredAbsent memverifikasi m-6(a): modul aktif yang
// menuntut (requires) modul yang TIDAK aktif → ErrConstraint.
func TestCheckRelations_RequiredAbsent(t *testing.T) {
	active := []module.Manifest{
		{Name: "db-postgres", Requires: []string{"core"}}, // core TIDAK aktif
	}
	err := checkRelations(active)
	if err == nil {
		t.Fatalf("mau error (required absent), dapat nil")
	}
	if !errors.Is(err, ErrConstraint) {
		t.Errorf("error harus ErrConstraint, dapat: %v", err)
	}
	if !strings.Contains(err.Error(), "core") {
		t.Errorf("pesan error harus menyebut modul yang absen 'core': %v", err)
	}
}

// TestCheckRelations_ConflictBothActive memverifikasi m-6(b): dua modul yang
// saling conflict aktif bersamaan → ErrConstraint.
func TestCheckRelations_ConflictBothActive(t *testing.T) {
	active := []module.Manifest{
		{Name: "db-postgres", Conflicts: []string{"db-mysql"}},
		{Name: "db-mysql"},
	}
	err := checkRelations(active)
	if err == nil {
		t.Fatalf("mau error (conflict keduanya aktif), dapat nil")
	}
	if !errors.Is(err, ErrConstraint) {
		t.Errorf("error harus ErrConstraint, dapat: %v", err)
	}
	if !strings.Contains(err.Error(), "db-mysql") {
		t.Errorf("pesan error harus menyebut modul konflik 'db-mysql': %v", err)
	}
}

// TestCheckRelations_AllSatisfied memverifikasi requires terpenuhi & tanpa
// conflict aktif → nil.
func TestCheckRelations_AllSatisfied(t *testing.T) {
	active := []module.Manifest{
		{Name: "core"},
		{Name: "db-postgres", Requires: []string{"core"}, Conflicts: []string{"db-mysql"}},
	}
	if err := checkRelations(active); err != nil {
		t.Errorf("checkRelations(valid) = %v, want nil", err)
	}
}

// TestCheckSafeTargetPath memverifikasi B-1: target ber-".." / absolut ditolak,
// target relatif normal diterima.
func TestCheckSafeTargetPath(t *testing.T) {
	bad := []string{"../escape", "../../etc/passwd", "/abs/path", "a/../../b"}
	for _, tgt := range bad {
		if err := checkSafeTargetPath(tgt); err == nil {
			t.Errorf("checkSafeTargetPath(%q) = nil, want error (B-1)", tgt)
		}
	}
	good := []string{"cmd/app/main.go", "internal/handler/x.go", "Makefile", ".env.example"}
	for _, tgt := range good {
		if err := checkSafeTargetPath(tgt); err != nil {
			t.Errorf("checkSafeTargetPath(%q) = %v, want nil", tgt, err)
		}
	}
}

// ── Fase 4a: arch modular-monolith ───────────────────────────────────────────

// TestResolve_ModularMonolith memverifikasi arch=modular-monolith mengaktifkan
// arch-modular (FileOp domain catalog & orders + shared/contract ada) dan TIDAK
// mengaktifkan arch-monolith (mutual conflict).
func TestResolve_ModularMonolith(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.Arch = answers.ArchModularMonolith

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}

	// Domain modular hadir.
	for _, target := range []string{
		"internal/catalog/catalog.go",
		"internal/orders/orders.go",
		"internal/shared/contract/contract.go",
	} {
		if _, ok := fileOpByTarget(p.Files, target); !ok {
			t.Errorf("FileOp modular %q tidak ada di plan", target)
		}
	}

	// arch-monolith TIDAK boleh ikut menghasilkan apa pun yang khas monolith-only;
	// di sini kita pastikan FileOp app.go berasal dari arch-modular (ModuleName).
	if op, ok := fileOpByTarget(p.Files, "internal/app/app.go"); ok {
		if op.ModuleName != modArchMod {
			t.Errorf("internal/app/app.go harus dimiliki %q, dapat %q", modArchMod, op.ModuleName)
		}
	} else {
		t.Errorf("composition root internal/app/app.go harus ada di modular")
	}
}

// TestResolve_ModularMonolith_NoArchMonolithConflict memverifikasi bahwa modul
// arch-monolith & arch-modular tidak pernah aktif bersamaan (checkRelations
// melindungi via conflicts, tetapi activeModules juga hanya memilih satu).
func TestResolve_ModularMonolith_NoArchMonolithConflict(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.Arch = answers.ArchModularMonolith

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}
	for _, f := range p.Files {
		if f.ModuleName == modArchMono {
			t.Errorf("modul %q tidak boleh aktif saat arch=modular-monolith (FileOp %q)", modArchMono, f.TargetPath)
		}
	}
}

// ── Fase 4a: HTTP framework chi / echo ────────────────────────────────────────

// TestResolve_HTTPChi memverifikasi http=chi: dep chi hadir, server.go dihasilkan
// SEKALI (oleh http-chi, bukan ganda dari core net/http).
func TestResolve_HTTPChi(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.HTTP = answers.HTTPChi

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}

	// Dep chi hadir.
	if !hasDep(p.Deps, "github.com/go-chi/chi/v5") {
		t.Errorf("dep chi harus ada saat http=chi: %+v", p.Deps)
	}

	// server.go dihasilkan TEPAT SATU kali, dan oleh modul http-chi (bukan core).
	server, count := fileOpsByTargetCount(p.Files, "internal/httpserver/server.go")
	if count != 1 {
		t.Fatalf("internal/httpserver/server.go harus muncul tepat 1x, dapat %d (double-wiring?)", count)
	}
	if server.ModuleName != modHTTPChi {
		t.Errorf("server.go harus dimiliki %q saat http=chi, dapat %q", modHTTPChi, server.ModuleName)
	}
}

// TestResolve_HTTPEcho memverifikasi http=echo: dep echo hadir, server.go tunggal
// dari http-echo.
func TestResolve_HTTPEcho(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.HTTP = answers.HTTPEcho

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}
	if !hasDep(p.Deps, "github.com/labstack/echo/v4") {
		t.Errorf("dep echo harus ada saat http=echo: %+v", p.Deps)
	}
	server, count := fileOpsByTargetCount(p.Files, "internal/httpserver/server.go")
	if count != 1 {
		t.Fatalf("server.go harus muncul tepat 1x saat http=echo, dapat %d", count)
	}
	if server.ModuleName != modHTTPEcho {
		t.Errorf("server.go harus dimiliki %q saat http=echo, dapat %q", modHTTPEcho, server.ModuleName)
	}
}

// TestResolve_HTTPNetHTTP_RouterFromCore memverifikasi default net/http: server.go
// berasal dari core (di-gate eq .HTTP "net/http") dan modul http-* tidak aktif.
func TestResolve_HTTPNetHTTP_RouterFromCore(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers() // http=net/http default

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}
	server, count := fileOpsByTargetCount(p.Files, "internal/httpserver/server.go")
	if count != 1 {
		t.Fatalf("server.go harus muncul tepat 1x saat http=net/http, dapat %d", count)
	}
	if server.ModuleName != modCore {
		t.Errorf("server.go harus dimiliki %q saat http=net/http, dapat %q", modCore, server.ModuleName)
	}
	// Tak boleh ada dep chi/echo.
	if hasDep(p.Deps, "github.com/go-chi/chi/v5") || hasDep(p.Deps, "github.com/labstack/echo/v4") {
		t.Errorf("net/http tidak boleh menarik dep chi/echo: %+v", p.Deps)
	}
}

// ── Fase 4a: db mysql ─────────────────────────────────────────────────────────

// TestResolve_DBMySQL memverifikasi db=mysql: dep mysql driver hadir, migration
// di-emit, postgres tidak.
func TestResolve_DBMySQL(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.DB = answers.DBMySQL
	a.Docker = true
	a.EnvExample = true
	// Access & Migrate dibiarkan kosong → default sqlx + golang-migrate.

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}

	if !hasDep(p.Deps, "github.com/go-sql-driver/mysql") {
		t.Errorf("dep mysql driver harus ada saat db=mysql: %+v", p.Deps)
	}
	// Migration hadir (migrate default golang-migrate → when 'ne .Migrate ""' true).
	if _, ok := fileOpByTarget(p.Files, "migrations/0001_init.up.sql"); !ok {
		t.Errorf("migration up harus ada saat db=mysql")
	}
	// mysql.go hadir, postgres.go tidak.
	if _, ok := fileOpByTarget(p.Files, "internal/platform/database/mysql.go"); !ok {
		t.Errorf("mysql.go harus ada saat db=mysql")
	}
	if _, ok := fileOpByTarget(p.Files, "internal/platform/database/postgres.go"); ok {
		t.Errorf("postgres.go tidak boleh ada saat db=mysql")
	}
	// compose punya service mysql (docker aktif).
	if compose, ok := fileOpByTarget(p.Files, "docker-compose.yml"); ok {
		foundMySQL := false
		for _, fr := range compose.Fragments {
			if strings.Contains(fr.Content, "compose.mysql") {
				foundMySQL = true
			}
			if strings.Contains(fr.Content, "compose.postgres") {
				t.Errorf("compose tidak boleh punya service postgres saat db=mysql")
			}
		}
		if !foundMySQL {
			t.Errorf("compose harus punya fragmen service mysql, fragments: %+v", compose.Fragments)
		}
	} else {
		t.Errorf("docker-compose.yml harus ada saat docker aktif")
	}
}

// ── Fase 4a: addon ci ─────────────────────────────────────────────────────────

// TestResolve_AddonCI_GitHub memverifikasi addon-ci aktif & file ci.yml GitHub
// Actions di-emit (bukan .gitlab-ci.yml) saat CI=github-actions.
func TestResolve_AddonCI_GitHub(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.CI = answers.CIGitHubActions

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}
	if _, ok := fileOpByTarget(p.Files, ".github/workflows/ci.yml"); !ok {
		t.Errorf(".github/workflows/ci.yml harus ada saat CI=github-actions")
	}
	if _, ok := fileOpByTarget(p.Files, ".gitlab-ci.yml"); ok {
		t.Errorf(".gitlab-ci.yml tidak boleh ada saat CI=github-actions")
	}
}

// TestResolve_AddonCI_GitLab memverifikasi cabang GitLab CI.
func TestResolve_AddonCI_GitLab(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.CI = answers.CIGitLabCI

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}
	if _, ok := fileOpByTarget(p.Files, ".gitlab-ci.yml"); !ok {
		t.Errorf(".gitlab-ci.yml harus ada saat CI=gitlab-ci")
	}
	if _, ok := fileOpByTarget(p.Files, ".github/workflows/ci.yml"); ok {
		t.Errorf(".github/workflows/ci.yml tidak boleh ada saat CI=gitlab-ci")
	}
}

// TestResolve_NoCI memverifikasi CI=none/kosong → addon-ci tidak aktif (tak ada
// file CI).
func TestResolve_NoCI(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers() // CI kosong → default none

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}
	if _, ok := fileOpByTarget(p.Files, ".github/workflows/ci.yml"); ok {
		t.Errorf("tak boleh ada file CI saat CI=none")
	}
	if _, ok := fileOpByTarget(p.Files, ".gitlab-ci.yml"); ok {
		t.Errorf("tak boleh ada file CI saat CI=none")
	}
}

// ── Fase 4a: addon observability ──────────────────────────────────────────────

// TestResolve_Observability memverifikasi addon-observability aktif saat Obs=true:
// dep otel + prometheus hadir, file observability di-emit.
func TestResolve_Observability(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.Obs = true

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}
	if !hasDep(p.Deps, "go.opentelemetry.io/otel") {
		t.Errorf("dep otel harus ada saat obs=true: %+v", p.Deps)
	}
	if !hasDep(p.Deps, "github.com/prometheus/client_golang") {
		t.Errorf("dep prometheus harus ada saat obs=true: %+v", p.Deps)
	}
	// File observability di PATH NYATA internal/platform/observability/*.go
	// (selaras manifest; BUKAN internal/observability/observability.go lama).
	for _, target := range []string{
		"internal/platform/observability/tracing.go",
		"internal/platform/observability/metrics.go",
		"internal/platform/observability/health.go",
	} {
		op, ok := fileOpByTarget(p.Files, target)
		if !ok {
			t.Errorf("file observability %q harus ada saat obs=true", target)
			continue
		}
		// Ketiga file STATIK MURNI → mode copy (selaras manifest nyata).
		if op.Mode != plan.ModeCopy {
			t.Errorf("%q harus ModeCopy (mode:copy di manifest), dapat %v", target, op.Mode)
		}
	}
	// Path lama TIDAK boleh ada (anti-regression: fixture pernah menyimpang ke
	// internal/observability/observability.go).
	if _, ok := fileOpByTarget(p.Files, "internal/observability/observability.go"); ok {
		t.Errorf("path lama internal/observability/observability.go tidak boleh ada (drift fixture)")
	}
	// AUTO-WIRE: server.go jadi ModeMerge dengan fragmen obs ke anchor imports + routes.
	server, ok := fileOpByTarget(p.Files, "internal/httpserver/server.go")
	if !ok {
		t.Fatalf("internal/httpserver/server.go harus ada saat obs=true")
	}
	if server.Mode != plan.ModeMerge {
		t.Errorf("server.go harus ModeMerge saat obs auto-wire, dapat %v", server.Mode)
	}
	var sawImports, sawRoutes bool
	for _, fr := range server.Fragments {
		if strings.Contains(fr.Content, "server.imports.obs") && fr.Anchor == "imports" {
			sawImports = true
		}
		if strings.Contains(fr.Content, "server.routes.obs") && fr.Anchor == "routes" {
			sawRoutes = true
		}
	}
	if !sawImports {
		t.Errorf("server.go harus punya fragmen obs ke anchor imports, fragments: %+v", server.Fragments)
	}
	if !sawRoutes {
		t.Errorf("server.go harus punya fragmen obs ke anchor routes (/metrics), fragments: %+v", server.Fragments)
	}
}

// TestResolve_NoObservability memverifikasi Obs=false → addon-observability tidak
// aktif (tak ada dep otel/prom, tak ada file).
func TestResolve_NoObservability(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers() // Obs=false

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}
	if hasDep(p.Deps, "go.opentelemetry.io/otel") || hasDep(p.Deps, "github.com/prometheus/client_golang") {
		t.Errorf("obs=false tidak boleh menarik dep otel/prometheus: %+v", p.Deps)
	}
	for _, target := range []string{
		"internal/platform/observability/tracing.go",
		"internal/platform/observability/metrics.go",
		"internal/platform/observability/health.go",
	} {
		if _, ok := fileOpByTarget(p.Files, target); ok {
			t.Errorf("file observability %q tidak boleh ada saat obs=false", target)
		}
	}
	// server.go saat obs=false harus ModeRender biasa (bukan merge — tak ada fragmen obs).
	if server, ok := fileOpByTarget(p.Files, "internal/httpserver/server.go"); ok {
		if server.Mode == plan.ModeMerge {
			t.Errorf("server.go tidak boleh ModeMerge saat obs=false (tak ada auto-wire)")
		}
	}
}

// ── Fase 4a: kombinasi lengkap & microservice masih ditolak ───────────────────

// TestResolve_ModularChiMySQLObsCI memverifikasi kombinasi penuh Fase 4a lolos
// resolve & menyatukan semua dep (chi + mysql + otel + prom) dedup+sort.
func TestResolve_ModularChiMySQLObsCI(t *testing.T) {
	r := New(mvpRegistry())
	a := answers.Answers{
		Name:       "billing",
		Module:     "gitlab.com/team-a/billing",
		Arch:       answers.ArchModularMonolith,
		Kind:       answers.KindREST,
		HTTP:       answers.HTTPChi,
		DB:         answers.DBMySQL,
		Docker:     true,
		EnvExample: true,
		Lint:       true,
		Obs:        true,
		CI:         answers.CIGitLabCI,
	}

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve kombinasi penuh gagal: %v", err)
	}

	for _, dep := range []string{
		"github.com/go-chi/chi/v5",
		"github.com/go-sql-driver/mysql",
		"go.opentelemetry.io/otel",
		"github.com/prometheus/client_golang",
	} {
		if !hasDep(p.Deps, dep) {
			t.Errorf("dep %q harus ada di kombinasi penuh: %+v", dep, p.Deps)
		}
	}
	// Deps tetap sort by Path (determinisme byte-identical).
	for i := 1; i < len(p.Deps); i++ {
		if p.Deps[i-1].Path > p.Deps[i].Path {
			t.Errorf("Deps tidak tersortir by Path: %q > %q", p.Deps[i-1].Path, p.Deps[i].Path)
		}
	}
	// FileOp tetap terurut by TargetPath.
	for i := 1; i < len(p.Files); i++ {
		if p.Files[i-1].TargetPath > p.Files[i].TargetPath {
			t.Errorf("FileOp tidak terurut by TargetPath: %q > %q", p.Files[i-1].TargetPath, p.Files[i].TargetPath)
		}
	}
	// server.go tunggal (http-chi), tak ganda dengan core/arch-modular.
	if _, count := fileOpsByTargetCount(p.Files, "internal/httpserver/server.go"); count != 1 {
		t.Errorf("server.go harus tunggal di kombinasi penuh, dapat %d", count)
	}
}

// ── Fase 4b: arch microservice ────────────────────────────────────────────────

// microAnswers mengembalikan Answers valid minimal untuk microservice dengan
// daftar service yang diberikan (Comm default grpc via resolver).
func microAnswers(services ...string) answers.Answers {
	svc := make([]answers.Service, 0, len(services))
	for _, s := range services {
		svc = append(svc, answers.Service{Name: s})
	}
	return answers.Answers{
		Name:     "platform",
		Module:   "github.com/acme/platform",
		Arch:     answers.ArchMicroservice,
		Services: svc,
	}
}

// fileOpsByTargetAll mengembalikan SEMUA FileOp dengan TargetPath tertentu.
func fileOpsByTargetAll(files []plan.FileOp, target string) []plan.FileOp {
	var out []plan.FileOp
	for _, f := range files {
		if f.TargetPath == target {
			out = append(out, f)
		}
	}
	return out
}

// TestResolve_Microservice_RootAndPerService memverifikasi T4.2: file ROOT
// (go.mod metadata + buf.yaml + compose) ada SEKALI; file PER-SERVICE (proto +
// main) ada untuk tiap service dengan DataOverride.Service benar; hook
// buf-generate ada; arch-monolith/arch-modular TIDAK aktif.
func TestResolve_Microservice_RootAndPerService(t *testing.T) {
	r := New(mvpRegistry())
	a := microAnswers("svc-a", "svc-b")

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve microservice gagal: %v", err)
	}

	// Metadata plan.
	if p.ProjectName != "platform" || p.ModulePath != "github.com/acme/platform" {
		t.Errorf("metadata plan salah: name=%q module=%q", p.ProjectName, p.ModulePath)
	}

	// ── File ROOT: ada TEPAT SATU (tak terduplikasi per service). ──
	for _, root := range []string{"README.md", ".gitignore", "buf.yaml", "buf.gen.yaml", "Makefile", "docker-compose.yml"} {
		ops := fileOpsByTargetAll(p.Files, root)
		if len(ops) != 1 {
			t.Errorf("file ROOT %q harus muncul tepat 1x, dapat %d", root, len(ops))
		}
		// File root TIDAK boleh punya DataOverride per-service.
		if len(ops) == 1 && ops[0].DataOverride != nil {
			if _, hasSvc := ops[0].DataOverride["Service"]; hasSvc {
				t.Errorf("file ROOT %q tidak boleh ber-DataOverride.Service", root)
			}
		}
	}

	// ── File PER-SERVICE: proto + main untuk svc-a & svc-b. ──
	wantPerService := map[string]string{
		"proto/svc-a/v1/svc-a.proto": "svc-a",
		"proto/svc-b/v1/svc-b.proto": "svc-b",
		"services/svc-a/cmd/main.go": "svc-a",
		"services/svc-b/cmd/main.go": "svc-b",
	}
	for target, wantSvc := range wantPerService {
		op, ok := fileOpByTarget(p.Files, target)
		if !ok {
			t.Errorf("file per-service %q tidak ada di plan", target)
			continue
		}
		if op.ModuleName != modArchMicro {
			t.Errorf("%q harus dimiliki %q, dapat %q", target, modArchMicro, op.ModuleName)
		}
		if op.DataOverride == nil {
			t.Fatalf("%q harus punya DataOverride per-service", target)
		}
		if got := op.DataOverride["Service"]; got != wantSvc {
			t.Errorf("%q DataOverride.Service = %v, mau %q", target, got, wantSvc)
		}
		// ModulePath ikut di override (template per-service merakit import gen/go).
		if op.DataOverride["ModulePath"] != "github.com/acme/platform" {
			t.Errorf("%q DataOverride.ModulePath salah: %v", target, op.DataOverride["ModulePath"])
		}
	}

	// ── IsFirst: hanya service PERTAMA (urut sort: svc-a) = true. ──
	mainA, _ := fileOpByTarget(p.Files, "services/svc-a/cmd/main.go")
	if mainA.DataOverride["IsFirst"] != true {
		t.Errorf("service pertama (svc-a) harus IsFirst=true, dapat %v", mainA.DataOverride["IsFirst"])
	}
	mainB, _ := fileOpByTarget(p.Files, "services/svc-b/cmd/main.go")
	if mainB.DataOverride["IsFirst"] != false {
		t.Errorf("service kedua (svc-b) harus IsFirst=false, dapat %v", mainB.DataOverride["IsFirst"])
	}
	// Others svc-a = [svc-b]; Others svc-b = [svc-a].
	if others, ok := mainA.DataOverride["Others"].([]string); !ok || len(others) != 1 || others[0] != "svc-b" {
		t.Errorf("svc-a Others harus [svc-b], dapat %v", mainA.DataOverride["Others"])
	}

	// ── Gateway gated .IsFirst: hanya svc-a punya gateway.go (svc-b tidak). ──
	if _, ok := fileOpByTarget(p.Files, "services/svc-a/internal/gateway/gateway.go"); !ok {
		t.Errorf("service pertama svc-a harus punya gateway.go (in-proses /call)")
	}
	if _, ok := fileOpByTarget(p.Files, "services/svc-b/internal/gateway/gateway.go"); ok {
		t.Errorf("service kedua svc-b TIDAK boleh punya gateway.go (when .IsFirst)")
	}

	// ── Hook buf-generate (order 5) ada untuk microservice. ──
	if !hasHook(p.Hooks, "buf-generate") {
		t.Errorf("hook buf-generate harus ada untuk microservice, hooks: %+v", p.Hooks)
	}
	for _, h := range p.Hooks {
		if h.Name == "buf-generate" && h.Order != 5 {
			t.Errorf("buf-generate harus Order 5, dapat %d", h.Order)
		}
	}

	// ── GoMod: grpc + protobuf (EKSAK), sort by Path. ──
	if !hasDep(p.Deps, "google.golang.org/grpc") || !hasDep(p.Deps, "google.golang.org/protobuf") {
		t.Errorf("microservice harus menarik grpc + protobuf: %+v", p.Deps)
	}

	// ── arch monolith/modular TIDAK aktif. ──
	for _, f := range p.Files {
		if f.ModuleName == modArchMono || f.ModuleName == modArchMod || f.ModuleName == modCore {
			t.Errorf("modul %q tidak boleh aktif saat arch=microservice (FileOp %q)", f.ModuleName, f.TargetPath)
		}
	}

	// ── Determinisme: FileOp terurut by TargetPath. ──
	for i := 1; i < len(p.Files); i++ {
		if p.Files[i-1].TargetPath > p.Files[i].TargetPath {
			t.Errorf("FileOp tidak terurut by TargetPath: %q > %q", p.Files[i-1].TargetPath, p.Files[i].TargetPath)
		}
	}
}

// TestResolve_Microservice_ComposePerService memverifikasi compose root jadi
// ModeMerge dengan fragmen region:services SATU per service (DataOverride.Service),
// terurut deterministik.
func TestResolve_Microservice_ComposePerService(t *testing.T) {
	r := New(mvpRegistry())
	a := microAnswers("svc-a", "svc-b")

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}

	compose, ok := fileOpByTarget(p.Files, "docker-compose.yml")
	if !ok {
		t.Fatalf("docker-compose.yml harus ada (dimiliki arch-microservice)")
	}
	if compose.Mode != plan.ModeMerge {
		t.Fatalf("compose harus ModeMerge, dapat %v", compose.Mode)
	}
	if compose.ModuleName != modArchMicro {
		t.Errorf("compose skeleton harus dimiliki %q, dapat %q", modArchMicro, compose.ModuleName)
	}

	// Satu fragmen region:services per service, masing-masing dgn DataOverride.Service.
	svcSeen := map[string]bool{}
	for _, fr := range compose.Fragments {
		if fr.Anchor != "services" {
			continue
		}
		if fr.DataOverride == nil {
			t.Errorf("fragmen compose per-service harus punya DataOverride, dapat nil")
			continue
		}
		if svc, ok := fr.DataOverride["Service"].(string); ok {
			svcSeen[svc] = true
		}
	}
	if !svcSeen["svc-a"] || !svcSeen["svc-b"] {
		t.Errorf("compose harus punya fragmen per service svc-a & svc-b, dapat: %v", svcSeen)
	}
	// Fragmen terurut by Order (determinisme): Order tak menurun.
	for i := 1; i < len(compose.Fragments); i++ {
		if compose.Fragments[i-1].Order > compose.Fragments[i].Order {
			t.Errorf("fragmen compose tidak terurut by Order: %+v", compose.Fragments)
		}
	}
}

// TestResolve_Microservice_Deterministic memverifikasi byte-identical: dua
// Resolve dgn Answers sama (urutan service dibalik di input) menghasilkan plan
// IDENTIK (FileOp & fragmen ter-sort, tak bergantung urutan input).
func TestResolve_Microservice_Deterministic(t *testing.T) {
	r := New(mvpRegistry())
	a1 := microAnswers("svc-a", "svc-b")
	a2 := microAnswers("svc-b", "svc-a") // urutan input dibalik

	p1, err := r.Resolve(a1)
	if err != nil {
		t.Fatalf("Resolve a1 gagal: %v", err)
	}
	p2, err := r.Resolve(a2)
	if err != nil {
		t.Fatalf("Resolve a2 gagal: %v", err)
	}
	if len(p1.Files) != len(p2.Files) {
		t.Fatalf("jumlah FileOp beda: %d vs %d (non-deterministik)", len(p1.Files), len(p2.Files))
	}
	for i := range p1.Files {
		if p1.Files[i].TargetPath != p2.Files[i].TargetPath {
			t.Errorf("FileOp[%d] TargetPath beda: %q vs %q (urutan service input bocor ke output)", i, p1.Files[i].TargetPath, p2.Files[i].TargetPath)
		}
	}
}

// TestResolve_Microservice_Gateway memverifikasi gateway on → .Gateway
// diproyeksikan ke data (manifest dapat menggate file gateway via when .Gateway).
func TestResolve_Microservice_Gateway(t *testing.T) {
	r := New(mvpRegistry())
	a := microAnswers("svc-a", "svc-b")
	a.Gateway = true

	p, err := r.Resolve(a)
	if err != nil {
		t.Fatalf("Resolve gateway gagal: %v", err)
	}
	// .Gateway diproyeksikan ke data global tiap FileOp (manifest dapat menggate).
	readme, ok := fileOpByTarget(p.Files, "README.md")
	if !ok {
		t.Fatalf("README.md harus ada")
	}
	if data, ok := readme.Data.(map[string]any); ok {
		if data["Gateway"] != true {
			t.Errorf(".Gateway harus diproyeksikan true ke data saat Gateway=true, dapat %v", data["Gateway"])
		}
		if data["Comm"] != "grpc" {
			t.Errorf(".Comm harus default 'grpc', dapat %v", data["Comm"])
		}
		// Services list terurut.
		if svcs, ok := data["Services"].([]string); !ok || len(svcs) != 2 || svcs[0] != "svc-a" {
			t.Errorf(".Services harus [svc-a svc-b] terurut, dapat %v", data["Services"])
		}
	} else {
		t.Fatalf("README.md Data bukan map[string]any")
	}
}

// TestResolve_Microservice_RejectComm memverifikasi comm=rest & comm=event
// ditolak ramah (v1: gRPC).
func TestResolve_Microservice_RejectComm(t *testing.T) {
	for _, comm := range []answers.Comm{answers.CommREST, answers.CommEvent} {
		r := New(mvpRegistry())
		a := microAnswers("svc-a")
		a.Comm = comm
		_, err := r.Resolve(a)
		if err == nil {
			t.Errorf("comm=%q harus ditolak (v1: gRPC)", comm)
			continue
		}
		if !strings.Contains(err.Error(), "gRPC") {
			t.Errorf("pesan tolak comm=%q harus menyebut gRPC: %v", comm, err)
		}
	}
}

// TestResolve_Microservice_RejectNoService memverifikasi microservice tanpa
// service ditolak.
func TestResolve_Microservice_RejectNoService(t *testing.T) {
	r := New(mvpRegistry())
	a := microAnswers() // tanpa service
	_, err := r.Resolve(a)
	if err == nil {
		t.Fatalf("microservice tanpa service harus ditolak")
	}
	if !strings.Contains(err.Error(), "service") {
		t.Errorf("pesan harus menyebut kebutuhan service: %v", err)
	}
}

// TestResolve_Microservice_RejectDupAndReserved memverifikasi nama service
// duplikat & reserved 'gateway' ditolak.
func TestResolve_Microservice_RejectDupAndReserved(t *testing.T) {
	r := New(mvpRegistry())
	// Duplikat.
	if _, err := r.Resolve(microAnswers("svc-a", "svc-a")); err == nil {
		t.Errorf("nama service duplikat harus ditolak")
	}
	// Reserved 'gateway'.
	if _, err := r.Resolve(microAnswers("gateway")); err == nil {
		t.Errorf("nama service 'gateway' (reserved) harus ditolak")
	}
	// Nama invalid (huruf besar).
	if _, err := r.Resolve(microAnswers("SvcA")); err == nil {
		t.Errorf("nama service invalid (huruf besar) harus ditolak")
	}
}

// ── Fase 4a: AUTO-WIRE db → main.go ───────────────────────────────────────────

// TestResolve_DBAutoWireMain memverifikasi bahwa db=postgres DAN db=mysql
// menghasilkan FileOp ModeMerge untuk cmd/<app>/main.go dengan fragmen ke anchor
// imports + wiring — membuktikan database.Connect(ctx) ter-AUTO-WIRE ke main
// (bukan sekadar library-only). Selaras manifest nyata db-postgres / db-mysql
// (contributes main.imports.* + main.wiring.* order 20).
func TestResolve_DBAutoWireMain(t *testing.T) {
	cases := []struct {
		name       string
		db         answers.DB
		importFrag string // substring path fragmen imports yang diharapkan
		wiringFrag string // substring path fragmen wiring yang diharapkan
	}{
		{"postgres", answers.DBPostgres, "main.imports.postgres", "main.wiring.postgres"},
		{"mysql", answers.DBMySQL, "main.imports.mysql", "main.wiring.mysql"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := New(mvpRegistry())
			a := baseAnswers()
			a.DB = tc.db
			// Access & Migrate dibiarkan kosong → default sqlx + golang-migrate.

			p, err := r.Resolve(a)
			if err != nil {
				t.Fatalf("Resolve gagal: %v", err)
			}

			// cmd/<app>/main.go: target placeholder dirender → cmd/shop/main.go.
			main, ok := fileOpByTarget(p.Files, "cmd/shop/main.go")
			if !ok {
				t.Fatalf("cmd/shop/main.go harus ada di plan")
			}
			// AUTO-WIRE ⇒ main.go menjadi ModeMerge (skeleton core + fragmen db).
			if main.Mode != plan.ModeMerge {
				t.Fatalf("cmd/shop/main.go harus ModeMerge saat db=%s (auto-wire), dapat %v", tc.db, main.Mode)
			}

			var sawImports, sawWiring bool
			for _, fr := range main.Fragments {
				if fr.Anchor == "imports" && strings.Contains(fr.Content, tc.importFrag) {
					sawImports = true
				}
				if fr.Anchor == "wiring" && strings.Contains(fr.Content, tc.wiringFrag) {
					sawWiring = true
				}
				// Order kontribusi db harus 20 (selaras manifest nyata).
				if fr.Anchor == "imports" || fr.Anchor == "wiring" {
					if fr.Order != 20 {
						t.Errorf("fragmen db ke anchor %q harus Order 20, dapat %d", fr.Anchor, fr.Order)
					}
				}
			}
			if !sawImports {
				t.Errorf("main.go harus punya fragmen import paket database ke anchor imports, fragments: %+v", main.Fragments)
			}
			if !sawWiring {
				t.Errorf("main.go harus punya fragmen wiring database.Connect ke anchor wiring, fragments: %+v", main.Fragments)
			}
		})
	}
}

// ── helper test ──────────────────────────────────────────────────────────────

// hasDep melaporkan apakah deps memuat path tertentu.
func hasDep(deps []plan.ModuleDep, path string) bool {
	for _, d := range deps {
		if d.Path == path {
			return true
		}
	}
	return false
}

// fileOpsByTargetCount mengembalikan FileOp pertama dengan TargetPath tertentu
// beserta jumlah kemunculannya (untuk mendeteksi double-emit).
func fileOpsByTargetCount(files []plan.FileOp, target string) (plan.FileOp, int) {
	var first plan.FileOp
	count := 0
	for _, f := range files {
		if f.TargetPath == target {
			if count == 0 {
				first = f
			}
			count++
		}
	}
	return first, count
}

func hasHook(hooks []plan.HookSpec, name string) bool {
	for _, h := range hooks {
		if h.Name == name {
			return true
		}
	}
	return false
}

// ── GUARD ANTI-DRIFT: fixture fake vs manifest NYATA ──────────────────────────

// fileCmpStrictness menentukan seberapa ketat Files/Contributes fake dibandingkan
// dengan manifest nyata.
type fileCmpStrictness int

const (
	// cmpExact: jumlah + himpunan entri fake WAJIB SAMA PERSIS dengan manifest nyata.
	// Dipakai untuk modul yang fixture-nya MENCERMINKAN penuh manifest nyata.
	cmpExact fileCmpStrictness = iota
	// cmpSubset: tiap entri fake WAJIB ADA di manifest nyata (fake ⊆ real), tetapi
	// manifest nyata boleh punya entri tambahan yang sengaja TIDAK direplikasi di
	// fixture (subset terdokumentasi). Mencegah fixture mengarang entri yang tak ada
	// di manifest, tanpa memaksa replikasi penuh layout.
	cmpSubset
	// cmpIgnore: perbandingan Files/Contributes DILEWATI sepenuhnya — dipakai HANYA
	// untuk modul yang fixture-nya memodelkan layout SECARA SINTETIK (path file
	// sengaja berbeda demi fokus test struktural, mis. arch-modular yang memadatkan
	// layout per-domain). GoMod TETAP dibandingkan eksak. Gunakan SEHEMAT mungkin &
	// WAJIB disertai catatan alasan (filesNote/contribNote).
	cmpIgnore
)

// driftSpec mendeklarasikan, per modul yang dipakai fake, tingkat ketat
// perbandingan Files & Contributes. GoMod SELALU dibandingkan EKSAK (path+versi)
// untuk SEMUA modul — itu invarian wajib (lihat TestFakeRegistryMatchesRealManifests).
type driftSpec struct {
	files       fileCmpStrictness
	contributes fileCmpStrictness
	// filesNote/contribNote mendokumentasikan alasan subset (jejak audit).
	filesNote   string
	contribNote string
}

// fakeModulesUnderGuard adalah daftar modul yang dipakai fakeRegistry (mvpRegistry)
// beserta tingkat ketat perbandingannya vs manifest nyata templates/modules/**.
//
// Prinsip:
//   - GoMod: EKSAK untuk semua (versi dependency tak boleh menyimpang — sumber bug
//     paling sering & paling halus; lihat kasus sqlx & versi otel/prometheus lama).
//   - Files/Contributes cmpExact untuk modul yang fixture-nya mirror penuh
//     (addon-observability, db-postgres, db-mysql, http-echo, http-chi, addon-ci,
//     addon-docker, addon-makefile, addon-golangci, addon-env, arch-monolith).
//   - Files/Contributes cmpSubset untuk modul yang fixture-nya sengaja
//     disederhanakan (core: layout file shared & gating disederhanakan; arch-modular:
//     layout per-domain disederhanakan). Untuk modul ini tiap entri fake tetap WAJIB
//     ada di manifest nyata (tak ada entri karangan).
var fakeModulesUnderGuard = map[string]driftSpec{
	modCore: {
		files:       cmpSubset,
		contributes: cmpExact,
		filesNote:   "core: subset file shared (README/gitignore/main/server/compose/Makefile/env) — manifest nyata punya lebih (config, health, handler, dst).",
		contribNote: "core: contributes app→.env.example direplikasi penuh.",
	},
	modArchMono: {
		files:       cmpExact,
		contributes: cmpExact,
	},
	modArchMod: {
		files:       cmpIgnore,
		contributes: cmpExact,
		filesNote:   "arch-modular: layout per-domain dimodelkan SINTETIK (catalog/orders/contract/app/server di path padat) — manifest nyata memakai internal/modules/<d>/internal/core/** (~19 file). Path file sengaja berbeda demi fokus test struktural; GoMod (kosong) tetap diverifikasi eksak.",
	},
	modArchMicro: {
		files:       cmpSubset,
		contributes: cmpExact,
		filesNote:   "arch-microservice: fixture mereplikasi file ROOT (README/gitignore/buf*/Makefile/compose) + PER-SERVICE (proto/cmd/config/server/gateway) dengan target & mode PERSIS manifest nyata; libs/** (config/logger/health/grpcclient, 4 file copy) SENGAJA tak direplikasi (di luar fokus test per-service) — subset terdokumentasi. GoMod (grpc+protobuf) tetap diverifikasi EKSAK.",
	},
	modHTTPChi:  {files: cmpExact, contributes: cmpExact},
	modHTTPEcho: {files: cmpExact, contributes: cmpExact},
	modDBPostgres: {
		files:       cmpSubset,
		contributes: cmpSubset,
		filesNote:   "db-postgres: migrasi di-gate `ne .Migrate \"\"` di fixture (manifest nyata copy tanpa gate) — target file sama.",
		contribNote: "db-postgres: compose-volume & Makefile-migrate contributes manifest nyata tidak direplikasi (di luar fokus test).",
	},
	modDBMySQL: {
		files:       cmpSubset,
		contributes: cmpSubset,
		filesNote:   "db-mysql: idem db-postgres.",
		contribNote: "db-mysql: idem db-postgres.",
	},
	modAccessGormPostgres: {files: cmpExact, contributes: cmpExact},
	modAccessGormMySQL:    {files: cmpExact, contributes: cmpExact},
	modAddonObs:           {files: cmpExact, contributes: cmpExact},
	modAddonCI:            {files: cmpExact, contributes: cmpExact},
	modAddonDocker:        {files: cmpExact, contributes: cmpExact},
	modAddonMake: {
		files:       cmpExact,
		contributes: cmpSubset,
		contribNote: "addon-makefile: fixture mereplikasi 1 target (run); manifest nyata punya fmt/lint/docker tergated — subset terdokumentasi.",
	},
	modAddonLint: {files: cmpExact, contributes: cmpExact},
	modAddonEnv:  {files: cmpExact, contributes: cmpExact},
	// feature-strapgorm: fixture MENCERMINKAN penuh manifest nyata
	// (templates/modules/feature-strapgorm/module.yaml) — 4 file domain Product, 4
	// contributes AUTO-WIRE (main + server.go, order 30), GoMod strapgorm pin EKSAK.
	modFeatureStrapgorm: {files: cmpExact, contributes: cmpExact},
}

// goModSet memetakan path→version dari []module.ModuleDep.
func goModSet(deps []module.ModuleDep) map[string]string {
	m := make(map[string]string, len(deps))
	for _, d := range deps {
		m[d.Path] = d.Version
	}
	return m
}

// fileTargetModeSet memetakan target→mode (kosong mode dinormalkan ke "render").
func fileTargetModeSet(files []module.FileSpec) map[string]string {
	m := make(map[string]string, len(files))
	for _, f := range files {
		mode := f.Mode
		if mode == "" {
			mode = "render"
		}
		m[f.Target] = mode
	}
	return m
}

// contribKey mengidentifikasi satu kontribusi merge by (target, anchor, order).
type contribKey struct {
	target string
	anchor string
	order  int
}

func contribSet(cs []module.MergeContribution) map[contribKey]bool {
	m := make(map[contribKey]bool, len(cs))
	for _, c := range cs {
		m[contribKey{target: c.Target, anchor: c.Anchor, order: c.Order}] = true
	}
	return m
}

// TestFakeRegistryMatchesRealManifests adalah GUARD ANTI-DRIFT durable: ia memuat
// registry NYATA dari templates.FS lalu, untuk SETIAP modul yang dipakai fixture
// fake (mvpRegistry), membandingkan:
//
//   - GoMod (path+versi) — WAJIB EKSAK untuk semua modul (invarian keras).
//   - Files (target+mode) — eksak atau subset sesuai fakeModulesUnderGuard.
//   - Contributes (target+anchor+order) — eksak atau subset sesuai spec.
//
// Tujuan: mencegah fixture menyimpang dari kebenaran (manifest nyata) secara
// berulang — terutama versi dependency (sumber bug paling halus). Bila manifest
// nyata berubah (mis. bump versi), test ini GAGAL hingga fixture diselaraskan.
func TestFakeRegistryMatchesRealManifests(t *testing.T) {
	// 1. Muat registry NYATA dari embed.FS (templates/modules/**).
	real := module.NewRegistry()
	if err := real.Load(templates.FS); err != nil {
		t.Fatalf("Load(templates.FS) gagal — katalog nyata tak valid: %v", err)
	}

	// 2. Index manifest fake by Name (sumber: mvpRegistry).
	fake := mvpRegistry()
	fakeByName := make(map[string]module.Manifest)
	for _, m := range fake.All() {
		fakeByName[m.Name] = m
	}

	// 3. Pastikan SEMUA modul fake terdaftar di guard spec — modul fake baru tanpa
	//    entri spec = lubang anti-drift (fail-fast agar guard tak ketinggalan).
	for name := range fakeByName {
		if _, ok := fakeModulesUnderGuard[name]; !ok {
			t.Errorf("modul fake %q tidak terdaftar di fakeModulesUnderGuard — tambahkan agar guard anti-drift menutupnya", name)
		}
	}

	// 4. Untuk tiap modul di guard spec: bandingkan fake vs nyata.
	for name, spec := range fakeModulesUnderGuard {
		t.Run(name, func(t *testing.T) {
			fm, ok := fakeByName[name]
			if !ok {
				t.Fatalf("modul %q ada di guard spec tetapi tidak di fixture fake", name)
			}
			rm, ok := real.Get(name)
			if !ok {
				t.Fatalf("modul %q tidak ditemukan di registry NYATA (templates.FS) — nama salah?", name)
			}

			// ── GoMod: EKSAK (path+versi) — invarian keras untuk semua modul. ──
			fakeMods := goModSet(fm.GoMod)
			realMods := goModSet(rm.GoMod)
			if len(fakeMods) != len(realMods) {
				t.Errorf("GoMod jumlah beda: fake=%d (%v) real=%d (%v)", len(fakeMods), fakeMods, len(realMods), realMods)
			}
			for path, fv := range fakeMods {
				rv, ok := realMods[path]
				if !ok {
					t.Errorf("GoMod fake punya %q yang TIDAK ada di manifest nyata (drift)", path)
					continue
				}
				if fv != rv {
					t.Errorf("GoMod versi menyimpang untuk %q: fake=%q real=%q (WAJIB sama)", path, fv, rv)
				}
			}
			for path := range realMods {
				if _, ok := fakeMods[path]; !ok {
					t.Errorf("GoMod manifest nyata punya %q yang TIDAK ada di fake (fixture ketinggalan dep)", path)
				}
			}

			// ── Files: target+mode (ignore/eksak/subset). ──
			if spec.files == cmpIgnore {
				t.Logf("Files cmpIgnore (layout sintetik terdokumentasi): %s", spec.filesNote)
			} else {
				fakeFiles := fileTargetModeSet(fm.Files)
				realFiles := fileTargetModeSet(rm.Files)
				for target, fmode := range fakeFiles {
					rmode, ok := realFiles[target]
					if !ok {
						t.Errorf("Files fake punya target %q yang TIDAK ada di manifest nyata (drift/karangan)", target)
						continue
					}
					if fmode != rmode {
						t.Errorf("Files mode menyimpang untuk %q: fake=%q real=%q", target, fmode, rmode)
					}
				}
				if spec.files == cmpExact {
					if len(fakeFiles) != len(realFiles) {
						t.Errorf("Files jumlah beda (cmpExact): fake=%d real=%d\n  fake=%v\n  real=%v", len(fakeFiles), len(realFiles), fakeFiles, realFiles)
					}
					for target := range realFiles {
						if _, ok := fakeFiles[target]; !ok {
							t.Errorf("Files manifest nyata punya target %q yang TIDAK ada di fake (cmpExact)", target)
						}
					}
				} else if spec.filesNote != "" {
					t.Logf("Files cmpSubset (terdokumentasi): %s", spec.filesNote)
				}
			}

			// ── Contributes: target+anchor+order (ignore/eksak/subset). ──
			if spec.contributes == cmpIgnore {
				t.Logf("Contributes cmpIgnore (terdokumentasi): %s", spec.contribNote)
			} else {
				fakeContribs := contribSet(fm.Contributes)
				realContribs := contribSet(rm.Contributes)
				for k := range fakeContribs {
					if !realContribs[k] {
						t.Errorf("Contributes fake punya {target=%q anchor=%q order=%d} yang TIDAK ada di manifest nyata (drift/karangan)", k.target, k.anchor, k.order)
					}
				}
				if spec.contributes == cmpExact {
					if len(fakeContribs) != len(realContribs) {
						t.Errorf("Contributes jumlah beda (cmpExact): fake=%d real=%d\n  fake=%v\n  real=%v", len(fakeContribs), len(realContribs), fakeContribs, realContribs)
					}
					for k := range realContribs {
						if !fakeContribs[k] {
							t.Errorf("Contributes manifest nyata punya {target=%q anchor=%q order=%d} yang TIDAK ada di fake (cmpExact)", k.target, k.anchor, k.order)
						}
					}
				} else if spec.contribNote != "" {
					t.Logf("Contributes cmpSubset (terdokumentasi): %s", spec.contribNote)
				}
			}
		})
	}
}
