package resolver

import (
	"errors"
	"strings"
	"testing"

	"github.com/faisalcayunda/gostarter/internal/answers"
	"github.com/faisalcayunda/gostarter/internal/module"
	"github.com/faisalcayunda/gostarter/internal/plan"
)

// TestCheckConstraints_C1_C2_C18 menguji checkConstraints SECARA LANGSUNG (unit)
// untuk SEMUA cabang fail-fast (C1 migrate↔db, C2 access↔db, C18 obs↔server) plus
// kombinasi valid. checkConstraints dipanggil dengan Answers SUDAH ter-default
// (applyDefaults), jadi test ini meng-applyDefaults dulu seperti Resolve.
func TestCheckConstraints_C1_C2_C18(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(a *answers.Answers)
		wantErr  bool
		wantSubs []string // substring di pesan error (T5.5)
	}{
		{
			name:     "C1 migrate tanpa db",
			mutate:   func(a *answers.Answers) { a.DB = answers.DBNone; a.Migrate = answers.MigrateGolangMigrate },
			wantErr:  true,
			wantSubs: []string{"--migrate", "C1", "none"},
		},
		{
			name:     "C2 access tanpa db",
			mutate:   func(a *answers.Answers) { a.DB = answers.DBNone; a.Access = answers.AccessSQLx },
			wantErr:  true,
			wantSubs: []string{"--access", "C2", "none"},
		},
		{
			name:     "C18 obs pada kind worker tanpa server",
			mutate:   func(a *answers.Answers) { a.Obs = true; a.Kind = answers.KindWorker },
			wantErr:  true,
			wantSubs: []string{"--obs", "C18", "worker"},
		},
		{
			name:    "valid: db none tanpa migrate/access",
			mutate:  func(a *answers.Answers) { a.DB = answers.DBNone },
			wantErr: false,
		},
		{
			name:    "valid: obs pada kind rest (punya server)",
			mutate:  func(a *answers.Answers) { a.Obs = true; a.Kind = answers.KindREST },
			wantErr: false,
		},
		{
			name: "valid: migrate+access dengan db postgres",
			mutate: func(a *answers.Answers) {
				a.DB = answers.DBPostgres
				a.Migrate = answers.MigrateGolangMigrate
				a.Access = answers.AccessSQLx
			},
			wantErr: false,
		},
		{
			name: "C-strapgorm: strapgorm tanpa access gorm (sqlx)",
			mutate: func(a *answers.Answers) {
				a.DB = answers.DBPostgres
				a.Access = answers.AccessSQLx
				a.Strapgorm = true
			},
			wantErr:  true,
			wantSubs: []string{"strapgorm", "gorm", "monolith"},
		},
		{
			// Sejak strapgorm meluas ke 3 arch: modular + gorm + postgres = VALID
			// (bukan lagi ditolak). C-strapgorm hanya menuntut access=gorm + db sql.
			name: "valid: strapgorm + gorm + postgres + modular",
			mutate: func(a *answers.Answers) {
				a.Arch = answers.ArchModularMonolith
				a.DB = answers.DBPostgres
				a.Access = answers.AccessGORM
				a.Strapgorm = true
			},
			wantErr: false,
		},
		{
			// microservice + gorm + postgres juga VALID (service product mandiri).
			// Catatan: untuk microservice DB/Access tak di-skip di checkConstraints,
			// jadi C-strapgorm yang menegakkan gorm+postgres pada jalur ini.
			name: "valid: strapgorm + gorm + postgres + microservice",
			mutate: func(a *answers.Answers) {
				a.Arch = answers.ArchMicroservice
				a.DB = answers.DBPostgres
				a.Access = answers.AccessGORM
				a.Strapgorm = true
			},
			wantErr: false,
		},
		{
			name: "valid: strapgorm + gorm + postgres + monolith",
			mutate: func(a *answers.Answers) {
				a.DB = answers.DBPostgres
				a.Access = answers.AccessGORM
				a.Strapgorm = true
			},
			wantErr: false,
		},
		{
			name: "valid: strapgorm + gorm + mysql + monolith",
			mutate: func(a *answers.Answers) {
				a.DB = answers.DBMySQL
				a.Access = answers.AccessGORM
				a.Strapgorm = true
			},
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := baseAnswers()
			tc.mutate(&a)
			a = applyDefaults(a)
			err := checkConstraints(a)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("checkConstraints mau error, dapat nil")
				}
				if !errors.Is(err, ErrConstraint) {
					t.Errorf("error harus ErrConstraint, dapat: %v", err)
				}
				for _, sub := range tc.wantSubs {
					if !strings.Contains(err.Error(), sub) {
						t.Errorf("pesan harus memuat %q, dapat: %v", sub, err)
					}
				}
				return
			}
			if err != nil {
				t.Errorf("checkConstraints(valid) = %v, mau nil", err)
			}
		})
	}
}

// TestResolve_Constraint_AccessNeedsDB menguji C2 lewat Resolve penuh: --access
// di-set DAN db=none → ErrConstraint, pesan menyebut C2. (Lengkapnya: jalur
// applyDefaults TIDAK men-default access untuk db=none, jadi access tetap di-set
// user → constraint terpicu.)
func TestResolve_Constraint_AccessNeedsDB(t *testing.T) {
	r := New(mvpRegistry())
	a := baseAnswers()
	a.DB = answers.DBNone
	a.Access = answers.AccessSQLx // C2: access tanpa db → invalid.

	_, err := r.Resolve(a)
	if err == nil {
		t.Fatalf("mau error C2 (access butuh db), dapat nil")
	}
	if !errors.Is(err, ErrConstraint) {
		t.Errorf("error harus ErrConstraint, dapat: %v", err)
	}
	if !strings.Contains(err.Error(), "C2") {
		t.Errorf("pesan harus menyebut C2, dapat: %v", err)
	}
}

// TestApplyDefaults_DBTriggersDockerAndAccess memverifikasi resolusi default SPEC
// §6.2: db∈{postgres,mysql} men-default Access=sqlx, Migrate=golang-migrate, dan
// Docker=true (bila user tak eksplisit). db=none TIDAK men-default ketiganya.
func TestApplyDefaults_DBTriggersDockerAndAccess(t *testing.T) {
	t.Run("postgres men-default access/migrate/docker", func(t *testing.T) {
		a := baseAnswers()
		a.DB = answers.DBPostgres
		got := applyDefaults(a)
		if got.Access != answers.AccessSQLx {
			t.Errorf("Access default = %q, mau sqlx", got.Access)
		}
		if got.Migrate != answers.MigrateGolangMigrate {
			t.Errorf("Migrate default = %q, mau golang-migrate", got.Migrate)
		}
		if !got.Docker {
			t.Errorf("Docker harus default true untuk db=postgres (SPEC §6.2)")
		}
	})

	t.Run("db none tidak men-default access/migrate/docker", func(t *testing.T) {
		a := baseAnswers() // db none
		got := applyDefaults(a)
		if got.Access != "" || got.Migrate != "" {
			t.Errorf("db=none tak boleh men-default access/migrate, dapat access=%q migrate=%q", got.Access, got.Migrate)
		}
		if got.Docker {
			t.Errorf("db=none tak boleh men-default docker=true")
		}
	})

	t.Run("DockerSet menghormati keputusan user (docker off tetap off)", func(t *testing.T) {
		a := baseAnswers()
		a.DB = answers.DBPostgres
		a.DockerSet = true // user memutuskan eksplisit: docker tetap off.
		a.Docker = false
		got := applyDefaults(a)
		if got.Docker {
			t.Errorf("DockerSet=true + Docker=false harus dihormati (docker tetap off), dapat true")
		}
	})
}

// TestApplyDefaults_MicroserviceDefaults memverifikasi cabang microservice di
// applyDefaults: Comm→grpc, ConfigLoader→godotenv, Log→slog, CI→none, Auth→none,
// dan field monolith (Kind/HTTP/DB) TIDAK di-default (dibiarkan kosong).
func TestApplyDefaults_MicroserviceDefaults(t *testing.T) {
	a := answers.Answers{
		Name:     "platform",
		Module:   "github.com/acme/platform",
		Arch:     answers.ArchMicroservice,
		Services: []answers.Service{{Name: "order"}},
	}
	got := applyDefaults(a)
	if got.Comm != answers.CommGRPC {
		t.Errorf("Comm microservice default = %q, mau grpc", got.Comm)
	}
	if got.ConfigLoader != answers.ConfigLoaderGodotenv {
		t.Errorf("ConfigLoader default = %q, mau godotenv", got.ConfigLoader)
	}
	if got.Log != answers.LogSlog {
		t.Errorf("Log default = %q, mau slog", got.Log)
	}
	if got.CI != answers.CINone || got.Auth != answers.AuthNone {
		t.Errorf("CI/Auth microservice default salah: ci=%q auth=%q", got.CI, got.Auth)
	}
	// Field monolith TIDAK di-default untuk microservice.
	if got.Kind != "" || got.HTTP != "" || got.DB != "" {
		t.Errorf("field monolith tak boleh di-default untuk microservice, dapat kind=%q http=%q db=%q", got.Kind, got.HTTP, got.DB)
	}
}

// TestGoVersionFor memverifikasi pemilihan go directive per-arch: microservice
// → 1.25 (grpc v1.81.1 butuh go≥1.25), arch lain → 1.24. Sumber tunggal yang
// menjaga plan.GoVersion & data["GoVersion"] tak divergen.
func TestGoVersionFor(t *testing.T) {
	cases := []struct {
		arch answers.Arch
		want string
	}{
		{answers.ArchMonolith, goVersionDefault},
		{answers.ArchModularMonolith, goVersionDefault},
		{answers.ArchMicroservice, goVersionMicroservice},
	}
	for _, c := range cases {
		got := goVersionFor(answers.Answers{Arch: c.arch})
		if got != c.want {
			t.Errorf("goVersionFor(%q) = %q, mau %q", c.arch, got, c.want)
		}
	}
	// Cabang Strapgorm: menaikkan ke 1.25 untuk arch NON-microservice (monolith &
	// modular). Untuk microservice, cabang microservice menang lebih dulu — TAPI
	// juga 1.25, jadi semua jalur strapgorm = 1.25 (strapgorm butuh Go ≥ 1.25).
	strapgormCases := []struct {
		arch answers.Arch
		want string
	}{
		{answers.ArchMonolith, goVersionStrapgorm},
		{answers.ArchModularMonolith, goVersionStrapgorm},
		{answers.ArchMicroservice, goVersionMicroservice}, // micro menang; sama-sama 1.25
	}
	for _, c := range strapgormCases {
		got := goVersionFor(answers.Answers{Arch: c.arch, Strapgorm: true})
		if got != c.want {
			t.Errorf("goVersionFor(strapgorm, %q) = %q, mau %q", c.arch, got, c.want)
		}
		if got != "1.25" {
			t.Errorf("semua jalur strapgorm harus 1.25, arch=%q dapat %q", c.arch, got)
		}
	}
	// Konstanta selaras kontrak: microservice = 1.25, strapgorm = 1.25, default = 1.24.
	if goVersionMicroservice != "1.25" {
		t.Errorf("goVersionMicroservice = %q, mau 1.25 (grpc v1.81.1)", goVersionMicroservice)
	}
	if goVersionStrapgorm != "1.25" {
		t.Errorf("goVersionStrapgorm = %q, mau 1.25", goVersionStrapgorm)
	}
	if goVersionDefault != "1.24" {
		t.Errorf("goVersionDefault = %q, mau 1.24", goVersionDefault)
	}
}

// TestResolveManifests_MissingModule memverifikasi resolveManifests gagal RAMAH
// bila nama modul tak ada di registry (katalog tak lengkap) — ErrConstraint +
// pesan menyebut nama modul.
func TestResolveManifests_MissingModule(t *testing.T) {
	r := &resolver{reg: mvpRegistry()}
	_, err := r.resolveManifests([]string{modCore, "modul-hantu"})
	if err == nil {
		t.Fatalf("resolveManifests dengan modul tak ada harus error")
	}
	if !errors.Is(err, ErrConstraint) {
		t.Errorf("error harus ErrConstraint, dapat: %v", err)
	}
	if !strings.Contains(err.Error(), "modul-hantu") {
		t.Errorf("pesan harus menyebut modul yang hilang, dapat: %v", err)
	}
}

// TestResolveManifests_SortStable memverifikasi resolveManifests mengembalikan
// manifest TERURUT by Name (determinisme byte-identical §5.2) meski input acak.
func TestResolveManifests_SortStable(t *testing.T) {
	r := &resolver{reg: mvpRegistry()}
	out, err := r.resolveManifests([]string{modAddonDocker, modCore, modArchMono})
	if err != nil {
		t.Fatalf("resolveManifests gagal: %v", err)
	}
	for i := 1; i < len(out); i++ {
		if out[i-1].Name > out[i].Name {
			t.Errorf("manifest tak terurut by Name: %q > %q", out[i-1].Name, out[i].Name)
		}
	}
}

// TestCollectDeps_HighestVersionWins memverifikasi dedup dependency: bila DUA
// modul aktif men-declare path yang SAMA dengan versi berbeda, versi TERTINGGI
// (semver-aware, BUKAN leksikografis) menang — kasus kritis v5.10.0 vs v5.9.0.
func TestCollectDeps_HighestVersionWins(t *testing.T) {
	active := []module.Manifest{
		{Name: "a", GoMod: []module.ModuleDep{{Path: "github.com/x/lib", Version: "v5.9.0"}}},
		{Name: "b", GoMod: []module.ModuleDep{{Path: "github.com/x/lib", Version: "v5.10.0"}}},
		{Name: "c", GoMod: []module.ModuleDep{{Path: "github.com/y/other", Version: "v1.2.0"}}},
	}
	deps := collectDeps(active)
	// Dedup: 2 path unik, sort by Path.
	if len(deps) != 2 {
		t.Fatalf("Deps = %d, mau 2 (dedup): %+v", len(deps), deps)
	}
	if deps[0].Path > deps[1].Path {
		t.Errorf("Deps tak terurut by Path: %+v", deps)
	}
	// HIGHEST WINS: v5.10.0 (BUKAN v5.9.0 yang menang secara leksikografis).
	var libVer string
	for _, d := range deps {
		if d.Path == "github.com/x/lib" {
			libVer = d.Version
		}
	}
	if libVer != "v5.10.0" {
		t.Errorf("highest-version-wins gagal: lib = %q, mau v5.10.0 (semver, bukan leksikografis)", libVer)
	}
}

// TestCollectDeps_EmptyReturnsNil memverifikasi modul tanpa GoMod → Deps nil
// (invarian db=none: GoMod kosong, murni stdlib).
func TestCollectDeps_EmptyReturnsNil(t *testing.T) {
	active := []module.Manifest{
		{Name: "core"},
		{Name: "arch-monolith"},
	}
	if deps := collectDeps(active); deps != nil {
		t.Errorf("collectDeps tanpa GoMod harus nil, dapat: %+v", deps)
	}
}

// TestBuildHooks_OrderAndGit memverifikasi buildHooks: microservice menambah
// buf-generate(5) PALING DULU; gofmt(10)+go-mod-tidy(20) selalu; git-init(30)
// hanya bila Git=true. Urutan order menjaga gen/go ada sebelum gofmt/tidy.
func TestBuildHooks_OrderAndGit(t *testing.T) {
	t.Run("monolith tanpa git", func(t *testing.T) {
		hooks := buildHooks(answers.Answers{Arch: answers.ArchMonolith})
		if hasHook(hooks, "buf-generate") {
			t.Errorf("monolith tak boleh punya buf-generate")
		}
		if !hasHook(hooks, "gofmt") || !hasHook(hooks, "go-mod-tidy") {
			t.Errorf("hooks wajib gofmt+go-mod-tidy, dapat %+v", hooks)
		}
		if hasHook(hooks, "git-init") {
			t.Errorf("git-init tak boleh ada saat Git=false")
		}
	})
	t.Run("microservice dengan git", func(t *testing.T) {
		hooks := buildHooks(answers.Answers{Arch: answers.ArchMicroservice, Git: true})
		if !hasHook(hooks, "buf-generate") {
			t.Fatalf("microservice harus punya buf-generate")
		}
		if !hasHook(hooks, "git-init") {
			t.Errorf("git-init harus ada saat Git=true")
		}
		// buf-generate(5) HARUS sebelum gofmt(10) & go-mod-tidy(20).
		orderOf := func(name string) int {
			for _, h := range hooks {
				if h.Name == name {
					return h.Order
				}
			}
			return -1
		}
		if orderOf("buf-generate") >= orderOf("gofmt") {
			t.Errorf("buf-generate(order %d) harus < gofmt(order %d)", orderOf("buf-generate"), orderOf("gofmt"))
		}
		if orderOf("gofmt") >= orderOf("go-mod-tidy") {
			t.Errorf("gofmt harus < go-mod-tidy")
		}
	})
}

// TestModeFromString memverifikasi pemetaan mode string manifest → FileOpMode:
// kosong/"render"→Render, "copy"→Copy, "mkdir"→Mkdir.
func TestModeFromString(t *testing.T) {
	cases := map[string]plan.FileOpMode{
		"":       plan.ModeRender,
		"render": plan.ModeRender,
		"copy":   plan.ModeCopy,
		"mkdir":  plan.ModeMkdir,
	}
	for in, want := range cases {
		if got := modeFromString(in); got != want {
			t.Errorf("modeFromString(%q) = %v, mau %v", in, got, want)
		}
	}
}

// TestPermFor memverifikasi permission default per mode: mkdir=0755, lainnya=0644.
func TestPermFor(t *testing.T) {
	if got := permFor("mkdir"); got != 0o755 {
		t.Errorf("permFor(mkdir) = %o, mau 0755", got)
	}
	for _, m := range []string{"", "render", "copy"} {
		if got := permFor(m); got != 0o644 {
			t.Errorf("permFor(%q) = %o, mau 0644", m, got)
		}
	}
}

// TestRenderTargetPath_Placeholder memverifikasi renderTargetPath mengevaluasi
// placeholder modBase/modJoin & fast-path (tanpa "{{" dikembalikan apa adanya).
func TestRenderTargetPath_Placeholder(t *testing.T) {
	data := map[string]any{"ModulePath": "github.com/acme/shop"}
	cases := []struct {
		in, want string
	}{
		{"cmd/{{ modBase .ModulePath }}/main.go", "cmd/shop/main.go"},
		{"README.md", "README.md"}, // fast-path tanpa placeholder
		{"{{ modJoin .ModulePath \"internal\" \"app\" }}", "github.com/acme/shop/internal/app"},
	}
	for _, c := range cases {
		got, err := renderTargetPath(c.in, data)
		if err != nil {
			t.Errorf("renderTargetPath(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("renderTargetPath(%q) = %q, mau %q", c.in, got, c.want)
		}
	}
}

// TestRenderTargetPath_BadTemplate memverifikasi placeholder rusak (parse error)
// → error (bukan path mentah).
func TestRenderTargetPath_BadTemplate(t *testing.T) {
	if _, err := renderTargetPath("cmd/{{ modBase }/main.go", map[string]any{}); err == nil {
		t.Fatal("renderTargetPath dengan template rusak harus error")
	}
}

// TestModJoinHelper memverifikasi helper modJoinHelper (variadic) mendelegasi ke
// modpath.Join dengan benar — penting karena dipakai renderTargetPath.
func TestModJoinHelper(t *testing.T) {
	got := modJoinHelper("github.com/acme/fleet", "services", "user")
	if got != "github.com/acme/fleet/services/user" {
		t.Errorf("modJoinHelper = %q", got)
	}
	if got := modBaseHelper("github.com/acme/shop/v2"); got != "shop" {
		t.Errorf("modBaseHelper strip /v2 = %q, mau shop", got)
	}
}

// TestSortedServiceNames memverifikasi sortedServiceNames mengurutkan stabil
// (deterministik) tanpa bergantung urutan input.
func TestSortedServiceNames(t *testing.T) {
	a := answers.Answers{Services: []answers.Service{{Name: "user"}, {Name: "order"}, {Name: "auth"}}}
	got := sortedServiceNames(a)
	want := []string{"auth", "order", "user"}
	if len(got) != len(want) {
		t.Fatalf("sortedServiceNames len = %d, mau %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sortedServiceNames[%d] = %q, mau %q", i, got[i], want[i])
		}
	}
}

// TestServiceData_DownstreamsAndPorts memverifikasi serviceData (Fase 4b):
//   - IsFirst hanya untuk service pertama (urutan sort);
//   - Others = semua service LAIN;
//   - Downstreams memasangkan tiap service lain dengan port gRPC-NYA SENDIRI
//     (grpcPortBase + indeks-nya, BUKAN port caller — perbaikan bug template lama);
//   - GrpcPort service ini = base + indeks-nya.
func TestServiceData_DownstreamsAndPorts(t *testing.T) {
	a := answers.Answers{Services: []answers.Service{{Name: "svc-a"}, {Name: "svc-b"}, {Name: "svc-c"}}}
	// Urutan sort: svc-a(0), svc-b(1), svc-c(2).

	dataA := serviceData(a, "svc-a")
	if dataA["IsFirst"] != true {
		t.Errorf("svc-a (pertama) harus IsFirst=true")
	}
	if dataA["GrpcPort"] != grpcPortBase+0 {
		t.Errorf("svc-a GrpcPort = %v, mau %d", dataA["GrpcPort"], grpcPortBase)
	}
	if dataA["GatewayPort"] != httpGatewayBase {
		t.Errorf("GatewayPort = %v, mau %d", dataA["GatewayPort"], httpGatewayBase)
	}
	others, _ := dataA["Others"].([]string)
	if len(others) != 2 || others[0] != "svc-b" || others[1] != "svc-c" {
		t.Errorf("svc-a Others = %v, mau [svc-b svc-c]", others)
	}
	// Downstreams svc-a: svc-b@base+1, svc-c@base+2 (port MEREKA, bukan port svc-a).
	downs, _ := dataA["Downstreams"].([]map[string]any)
	if len(downs) != 2 {
		t.Fatalf("svc-a Downstreams len = %d, mau 2", len(downs))
	}
	portByName := map[string]any{}
	for _, d := range downs {
		portByName[d["Name"].(string)] = d["Port"]
	}
	if portByName["svc-b"] != grpcPortBase+1 {
		t.Errorf("downstream svc-b port = %v, mau %d (port svc-b sendiri, bukan caller)", portByName["svc-b"], grpcPortBase+1)
	}
	if portByName["svc-c"] != grpcPortBase+2 {
		t.Errorf("downstream svc-c port = %v, mau %d", portByName["svc-c"], grpcPortBase+2)
	}

	// svc-b BUKAN pertama → IsFirst=false; GrpcPort = base+1.
	dataB := serviceData(a, "svc-b")
	if dataB["IsFirst"] != false {
		t.Errorf("svc-b bukan pertama → IsFirst=false")
	}
	if dataB["GrpcPort"] != grpcPortBase+1 {
		t.Errorf("svc-b GrpcPort = %v, mau %d", dataB["GrpcPort"], grpcPortBase+1)
	}
}

// TestIsPerServiceTarget memverifikasi deteksi placeholder {{ .Service }}
// (whitespace-toleran) — penanda emit per-service.
func TestIsPerServiceTarget(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"proto/{{ .Service }}/v1/x.proto", true},
		{"services/{{.Service}}/cmd/main.go", true}, // tanpa spasi
		{"{{ .Service.Foo }}", true},                // dukung field bersarang
		{"README.md", false},
		{"cmd/{{ modBase .ModulePath }}/main.go", false}, // placeholder LAIN, bukan .Service
		{"docker-compose.yml", false},
	}
	for _, c := range cases {
		if got := isPerServiceTarget(c.in); got != c.want {
			t.Errorf("isPerServiceTarget(%q) = %v, mau %v", c.in, got, c.want)
		}
	}
}
