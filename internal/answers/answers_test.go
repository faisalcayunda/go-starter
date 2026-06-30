package answers

import (
	"strings"
	"testing"
)

// validMonolith mengembalikan Answers monolith REST minimal yang LOLOS Validate —
// titik awal tiap kasus negatif (ubah satu field agar gagal terisolasi).
func validMonolith() Answers {
	return Answers{
		Name:   "shop",
		Module: "github.com/acme/shop",
		Arch:   ArchMonolith,
		Kind:   KindREST,
		HTTP:   HTTPNetHTTP,
		DB:     DBNone,
	}
}

// validMicro mengembalikan Answers microservice minimal yang LOLOS Validate.
func validMicro(services ...string) Answers {
	svc := make([]Service, 0, len(services))
	for _, s := range services {
		svc = append(svc, Service{Name: s})
	}
	return Answers{
		Name:     "platform",
		Module:   "github.com/acme/platform",
		Arch:     ArchMicroservice,
		Services: svc,
	}
}

// TestValidate_HappyPaths memverifikasi kombinasi valid subset Fase 4a/4b LOLOS
// (tidak ada false-positive yang menolak input benar).
func TestValidate_HappyPaths(t *testing.T) {
	cases := []struct {
		name string
		a    Answers
	}{
		{"monolith net/http db none", validMonolith()},
		{"monolith chi postgres", func() Answers { a := validMonolith(); a.HTTP = HTTPChi; a.DB = DBPostgres; return a }()},
		{"monolith echo mysql", func() Answers { a := validMonolith(); a.HTTP = HTTPEcho; a.DB = DBMySQL; return a }()},
		{"modular-monolith", func() Answers { a := validMonolith(); a.Arch = ArchModularMonolith; return a }()},
		{"db postgres access kosong (default resolver)", func() Answers { a := validMonolith(); a.DB = DBPostgres; return a }()},
		{"db postgres sqlx + golang-migrate eksplisit", func() Answers {
			a := validMonolith()
			a.DB = DBPostgres
			a.Access = AccessSQLx
			a.Migrate = MigrateGolangMigrate
			return a
		}()},
		{"db postgres access gorm", func() Answers { a := validMonolith(); a.DB = DBPostgres; a.Access = AccessGORM; return a }()},
		{"db mysql access gorm", func() Answers { a := validMonolith(); a.DB = DBMySQL; a.Access = AccessGORM; return a }()},
		{"db postgres access database/sql", func() Answers { a := validMonolith(); a.DB = DBPostgres; a.Access = AccessDatabaseSQL; return a }()},
		{"ci github-actions", func() Answers { a := validMonolith(); a.CI = CIGitHubActions; return a }()},
		{"ci gitlab-ci", func() Answers { a := validMonolith(); a.CI = CIGitLabCI; return a }()},
		{"ci none eksplisit", func() Answers { a := validMonolith(); a.CI = CINone; return a }()},
		{"auth none eksplisit", func() Answers { a := validMonolith(); a.Auth = AuthNone; return a }()},
		{"microservice 1 service", validMicro("order")},
		{"microservice 2 service", validMicro("order", "user")},
		{"microservice comm grpc eksplisit", func() Answers { a := validMicro("order"); a.Comm = CommGRPC; return a }()},
		{"module path dengan suffix versi mayor", func() Answers { a := validMonolith(); a.Module = "github.com/acme/shop/v2"; return a }()},
		{"nama dengan dash & angka", func() Answers { a := validMonolith(); a.Name = "my-app-2"; return a }()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.a.Validate(); err != nil {
				t.Errorf("Validate() = %v, mau nil (kombinasi valid ditolak)", err)
			}
		})
	}
}

// TestValidate_NameErrors memverifikasi validasi regex nama project (T5.5):
// pesan menyebut nilai yang salah & menyebut regex sebagai panduan perbaikan.
func TestValidate_NameErrors(t *testing.T) {
	cases := []struct {
		name     string
		projName string
		wantSubs []string // substring yang HARUS muncul di pesan error
	}{
		{"kosong", "", []string{"wajib"}},
		{"huruf besar di awal", "Shop", []string{"Shop", "huruf kecil"}},
		{"spasi di tengah", "shop app", []string{"shop app", "huruf kecil"}},
		{"diawali angka", "1shop", []string{"1shop", "huruf kecil"}},
		{"diawali dash", "-shop", []string{"-shop"}},
		{"karakter ilegal underscore", "shop_app", []string{"shop_app"}},
		{"karakter ilegal titik", "shop.app", []string{"shop.app"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := validMonolith()
			a.Name = tc.projName
			err := a.Validate()
			if err == nil {
				t.Fatalf("nama %q harus ditolak", tc.projName)
			}
			for _, sub := range tc.wantSubs {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("pesan error untuk nama %q harus memuat %q, dapat: %v", tc.projName, sub, err)
				}
			}
		})
	}
}

// TestValidate_ModulePathErrors memverifikasi validasi module path via
// x/mod/module.CheckPath (T5.5): pesan menyebut nilai salah + contoh perbaikan.
func TestValidate_ModulePathErrors(t *testing.T) {
	// Catatan: x/mod/module.CheckPath MENERIMA huruf besar (uppercase) pada module
	// path — yang melarang uppercase adalah import path (CheckImportPath), bukan
	// module path. Karena itu "github.com/Acme/Shop" TIDAK diuji sebagai invalid.
	cases := []struct {
		name   string
		module string
	}{
		{"kosong", ""},
		{"spasi di tengah", "github.com/acme/my project"},
		{"diawali slash", "/github.com/acme/shop"},
		{"karakter ilegal spasi trailing", "github.com/acme/shop "},
		{"trailing slash", "github.com/acme/shop/"},
		{"segmen kosong (double slash)", "github.com//shop"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := validMonolith()
			a.Module = tc.module
			err := a.Validate()
			if err == nil {
				t.Fatalf("module path %q harus ditolak", tc.module)
			}
			// Pesan kosong menyebut "wajib"; pesan non-kosong menyebut nilai + "valid".
			if tc.module == "" {
				if !strings.Contains(err.Error(), "wajib") {
					t.Errorf("module kosong harus menyebut 'wajib', dapat: %v", err)
				}
				return
			}
			if !strings.Contains(err.Error(), tc.module) {
				t.Errorf("pesan harus menyebut module path salah %q, dapat: %v", tc.module, err)
			}
			if !strings.Contains(err.Error(), "valid") {
				t.Errorf("pesan harus menyebut ketidakvalidan + panduan, dapat: %v", err)
			}
		})
	}
}

// TestValidate_UnsupportedOptions memverifikasi opsi di luar subset Fase 4a/4b
// ditolak RAMAH (T5.5): pesan menyebut nilai + "belum didukung" + petunjuk subset.
func TestValidate_UnsupportedOptions(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(a *Answers)
		wantSubs []string
	}{
		{"arch tak dikenal", func(a *Answers) { a.Arch = "serverless" }, []string{"serverless", "monolith"}},
		{"kind web belum didukung", func(a *Answers) { a.Kind = KindWeb }, []string{"web", "belum didukung"}},
		{"kind worker belum didukung", func(a *Answers) { a.Kind = KindWorker }, []string{"worker", "belum didukung"}},
		{"kind tak dikenal", func(a *Answers) { a.Kind = "daemon" }, []string{"daemon", "rest"}},
		{"http gin belum didukung", func(a *Answers) { a.HTTP = HTTPGin }, []string{"gin", "belum didukung"}},
		{"http fiber belum didukung", func(a *Answers) { a.HTTP = HTTPFiber }, []string{"fiber", "belum didukung"}},
		{"http tak dikenal", func(a *Answers) { a.HTTP = "fasthttp" }, []string{"fasthttp", "net/http"}},
		{"db sqlite belum didukung", func(a *Answers) { a.DB = DBSQLite }, []string{"sqlite", "belum didukung"}},
		{"db mongo belum didukung", func(a *Answers) { a.DB = DBMongo }, []string{"mongo", "belum didukung"}},
		{"db tak dikenal", func(a *Answers) { a.DB = "cockroach" }, []string{"cockroach", "postgres"}},
		{"access sqlc belum didukung", func(a *Answers) { a.DB = DBPostgres; a.Access = AccessSQLC }, []string{"sqlc", "belum didukung"}},
		{"access ent belum didukung", func(a *Answers) { a.DB = DBPostgres; a.Access = AccessEnt }, []string{"ent", "belum didukung"}},
		{"access tak dikenal", func(a *Answers) { a.DB = DBPostgres; a.Access = "xorm" }, []string{"xorm", "sqlx"}},
		{"migrate goose belum didukung", func(a *Answers) { a.DB = DBPostgres; a.Migrate = MigrateGoose }, []string{"goose", "belum didukung"}},
		{"migrate tak dikenal", func(a *Answers) { a.DB = DBPostgres; a.Migrate = "flyway" }, []string{"flyway", "golang-migrate"}},
		{"ci tak dikenal", func(a *Answers) { a.CI = "jenkins" }, []string{"jenkins", "github-actions"}},
		{"auth jwt belum didukung", func(a *Answers) { a.Auth = AuthJWT }, []string{"jwt", "belum didukung"}},
		// M-5: assertion bermakna — selain nilai 'oauth2' (yang trivial muncul karena
		// di-echo), verifikasi pesan memuat penanda "tidak dikenal" + panduan 'none'
		// (membedakan jalur default unknown dari jalur "belum didukung" jwt/paseto).
		{"auth tak dikenal", func(a *Answers) { a.Auth = "oauth2" }, []string{"oauth2", "tidak dikenal", "none"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := validMonolith()
			tc.mutate(&a)
			err := a.Validate()
			if err == nil {
				t.Fatalf("opsi tak didukung harus ditolak")
			}
			for _, sub := range tc.wantSubs {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("pesan harus memuat %q, dapat: %v", sub, err)
				}
			}
		})
	}
}

// TestValidate_MicroserviceServiceRules memverifikasi aturan q_svc microservice
// (T5.5): minimal 1 service, nama valid/unik, reserved 'gateway', comm grpc-only.
func TestValidate_MicroserviceServiceRules(t *testing.T) {
	cases := []struct {
		name     string
		a        Answers
		wantSubs []string
	}{
		{
			"tanpa service",
			validMicro(),
			[]string{"microservice", "minimal 1 service"},
		},
		{
			"nama service kosong",
			func() Answers { a := validMicro(); a.Services = []Service{{Name: ""}}; return a }(),
			[]string{"tidak boleh kosong"},
		},
		{
			"nama service huruf besar",
			validMicro("OrderSvc"),
			[]string{"OrderSvc", "huruf kecil"},
		},
		{
			"nama service reserved gateway",
			validMicro("gateway"),
			[]string{"gateway", "reserved"},
		},
		{
			"nama service duplikat",
			validMicro("order", "order"),
			[]string{"order", "duplikat"},
		},
		{
			"comm rest ditolak",
			func() Answers { a := validMicro("order"); a.Comm = CommREST; return a }(),
			[]string{"rest", "gRPC"},
		},
		{
			"comm event ditolak",
			func() Answers { a := validMicro("order"); a.Comm = CommEvent; return a }(),
			[]string{"event", "gRPC"},
		},
		{
			"comm tak dikenal",
			func() Answers { a := validMicro("order"); a.Comm = "soap"; return a }(),
			[]string{"soap", "grpc"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.a.Validate()
			if err == nil {
				t.Fatalf("Answers microservice invalid harus ditolak")
			}
			for _, sub := range tc.wantSubs {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("pesan harus memuat %q, dapat: %v", sub, err)
				}
			}
		})
	}
}

// TestValidate_MicroserviceSkipsMonolithFields memverifikasi bahwa untuk
// microservice, field monolith (Kind/HTTP/DB) yang KOSONG tidak menyebabkan error
// (resolver tak men-default-kannya; Validate melewati cabang monolith). Ini
// mencegah regresi di mana microservice menuntut http=net/http / db=none isi.
func TestValidate_MicroserviceSkipsMonolithFields(t *testing.T) {
	a := validMicro("order")
	// Kind/HTTP/DB sengaja kosong (zero value) — harus tetap valid.
	if a.Kind != "" || a.HTTP != "" || a.DB != "" {
		t.Fatalf("prasyarat test: field monolith harus kosong, dapat kind=%q http=%q db=%q", a.Kind, a.HTTP, a.DB)
	}
	if err := a.Validate(); err != nil {
		t.Errorf("microservice dengan field monolith kosong harus valid, dapat: %v", err)
	}
}

// TestValidate_AccessMigrateOnlyWhenDB memverifikasi access/migrate non-default
// HANYA divalidasi bila db ∈ {postgres, mysql}. Untuk db=none, access/migrate
// kosong tak dipersoalkan Validate (constraint C1/C2 silang ditegakkan resolver,
// bukan Validate). Mencegah Validate menolak prematur sebelum resolver bicara.
func TestValidate_AccessMigrateOnlyWhenDB(t *testing.T) {
	// db=none + access/migrate kosong → valid di level field (C1/C2 silang = resolver).
	a := validMonolith() // db none, access/migrate kosong
	if err := a.Validate(); err != nil {
		t.Errorf("db=none access/migrate kosong harus lolos Validate (C1/C2 = resolver), dapat: %v", err)
	}
}

// validStrapgorm mengembalikan Answers monolith + gorm + postgres + add-on
// strapgorm yang LOLOS Validate (prasyarat keras terpenuhi).
func validStrapgorm() Answers {
	a := validMonolith()
	a.DB = DBPostgres
	a.Access = AccessGORM
	a.Strapgorm = true
	return a
}

// TestValidate_Strapgorm memverifikasi prasyarat keras add-on strapgorm
// (access=gorm + db∈{postgres,mysql} + arch=monolith) ditolak RAMAH di Validate
// bila tak terpenuhi, dan kombinasi valid lolos. Pesan tolak menyebut ketiga
// syarat (satu pesan tunggal) — selaras checkConstraints C-strapgorm di resolver.
func TestValidate_Strapgorm(t *testing.T) {
	t.Run("valid: gorm+postgres+monolith", func(t *testing.T) {
		if err := validStrapgorm().Validate(); err != nil {
			t.Errorf("strapgorm gorm+postgres+monolith harus valid, dapat: %v", err)
		}
	})
	t.Run("valid: gorm+mysql+monolith", func(t *testing.T) {
		a := validStrapgorm()
		a.DB = DBMySQL
		if err := a.Validate(); err != nil {
			t.Errorf("strapgorm gorm+mysql+monolith harus valid, dapat: %v", err)
		}
	})
	// Sejak strapgorm meluas ke 3 arch: modular-monolith & microservice = VALID
	// (asal access=gorm + db∈{postgres,mysql}).
	t.Run("valid: gorm+postgres+modular", func(t *testing.T) {
		a := validStrapgorm()
		a.Arch = ArchModularMonolith
		if err := a.Validate(); err != nil {
			t.Errorf("strapgorm gorm+postgres+modular harus valid, dapat: %v", err)
		}
	})
	t.Run("valid: gorm+postgres+microservice", func(t *testing.T) {
		a := validStrapgorm()
		a.Arch = ArchMicroservice
		a.Services = []Service{{Name: "svc-a"}}
		if err := a.Validate(); err != nil {
			t.Errorf("strapgorm gorm+postgres+microservice harus valid, dapat: %v", err)
		}
	})

	reject := []struct {
		name   string
		mutate func(a *Answers)
	}{
		{"tanpa access gorm (sqlx)", func(a *Answers) { a.Access = AccessSQLx }},
		{"tanpa access gorm (database/sql)", func(a *Answers) { a.Access = AccessDatabaseSQL }},
		{"db none", func(a *Answers) { a.DB = DBNone; a.Access = "" }},
	}
	for _, tc := range reject {
		t.Run("reject: "+tc.name, func(t *testing.T) {
			a := validStrapgorm()
			tc.mutate(&a)
			err := a.Validate()
			if err == nil {
				t.Fatalf("strapgorm %s harus ditolak Validate, dapat nil", tc.name)
			}
			// Pesan tunggal menyebut ketiga prasyarat (ramah, dapat dibaca user).
			for _, sub := range []string{"strapgorm", "gorm", "monolith"} {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("pesan tolak harus memuat %q, dapat: %v", sub, err)
				}
			}
		})
	}
}
