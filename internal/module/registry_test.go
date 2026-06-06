package module_test

import (
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/faisalcayunda/gostarter/internal/module"
	"github.com/faisalcayunda/gostarter/templates"
)

// TestLoad_RealCatalog memuat katalog template nyata dari templates.FS (embed)
// dan memverifikasi modul-modul Fase 3 ter-load, ter-index by Name, dan field
// (termasuk Vars non-string) ter-parse benar.
func TestLoad_RealCatalog(t *testing.T) {
	reg := module.NewRegistry()
	if err := reg.Load(templates.FS); err != nil {
		t.Fatalf("Load(templates.FS) gagal: %v", err)
	}

	all := reg.All()
	if len(all) == 0 {
		t.Fatal("All() kosong; tidak ada modul ter-load dari templates.FS")
	}

	// All() wajib terurut deterministik by Name (SPEC §5.2).
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i].Name < all[j].Name }) {
		t.Errorf("All() tidak terurut by Name: %v", names(all))
	}

	// Modul Fase 3 yang harus ada di katalog kanonik.
	wantModules := []string{"arch-monolith", "core", "db-postgres"}
	for _, name := range wantModules {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("Get(%q) = false; modul wajib ada di katalog", name)
		}
	}

	// Fase 4a: http-chi & http-echo DIPERKENALKAN KEMBALI sebagai modul NYATA
	// (router go-chi/chi/v5 & labstack/echo/v4 yang menukar server.go core via
	// module-level file gating, bukan stub panic Fase 3). Keduanya WAJIB ada di
	// katalog. net/http tetap default (disediakan core, tanpa modul terpisah).
	for _, want := range []string{"http-chi", "http-echo"} {
		if _, ok := reg.Get(want); !ok {
			t.Errorf("Get(%q) = false; modul http-* nyata wajib ada di katalog (Fase 4a)", want)
		}
	}

	// Modul dorman/yatim Fase 3 yang DIHAPUS tetap tak boleh muncul: http-net-http
	// (file yatim tak di-wire) & http-stdlib (alias mati). net/http = milik core.
	for _, gone := range []string{"http-net-http", "http-stdlib"} {
		if _, ok := reg.Get(gone); ok {
			t.Errorf("Get(%q) = true; modul http-* dorman/yatim harus tetap DIHAPUS (m-3)", gone)
		}
	}

	// --- core: field dasar + Vars non-string ter-parse ---
	core, ok := reg.Get("core")
	if !ok {
		t.Fatal("modul 'core' tidak ter-load")
	}
	if core.Name != "core" {
		t.Errorf("core.Name = %q, want \"core\"", core.Name)
	}
	if strings.TrimSpace(core.Description) == "" {
		t.Error("core.Description kosong; field wajib")
	}
	if len(core.Files) == 0 {
		t.Error("core.Files kosong; core membawa skeleton file shared")
	}
	// Vars non-string: AppPort harus ter-parse sebagai integer, bukan string
	// (membuktikan map[string]any menampung nilai non-string — ADR-002 §3.2).
	appPort, ok := core.Vars["AppPort"]
	if !ok {
		t.Fatal("core.Vars[\"AppPort\"] tidak ada; var default core hilang")
	}
	if _, isInt := appPort.(int); !isInt {
		t.Errorf("core.Vars[\"AppPort\"] bertipe %T (%v), want int (Vars non-string)", appPort, appPort)
	}

	// --- db-postgres: gomod + Vars non-string (DBPort) + alias fragment ---
	pg, ok := reg.Get("db-postgres")
	if !ok {
		t.Fatal("modul 'db-postgres' tidak ter-load")
	}
	// postgres (scope Fase 3) = HANYA pgx/v5; pin terverifikasi via gomod[].
	// jmoiron/sqlx SENGAJA tidak ada di Fase 3 (C-2): tak ada kode yang meng-import-
	// nya → akan di-prune go mod tidy. Lapisan akses sqlx menyusul Fase 4.
	gomodPaths := make(map[string]string, len(pg.GoMod))
	for _, d := range pg.GoMod {
		gomodPaths[d.Path] = d.Version
	}
	if v := gomodPaths["github.com/jackc/pgx/v5"]; v == "" {
		t.Errorf("db-postgres.GoMod tidak memuat pgx/v5; got %+v", pg.GoMod)
	}
	if _, ok := gomodPaths["github.com/jmoiron/sqlx"]; ok {
		t.Errorf("db-postgres.GoMod TIDAK boleh memuat jmoiron/sqlx di Fase 3 (C-2; akan di-prune go mod tidy); got %+v", pg.GoMod)
	}
	if dbPort, ok := pg.Vars["DBPort"]; !ok {
		t.Error("db-postgres.Vars[\"DBPort\"] tidak ada")
	} else if _, isInt := dbPort.(int); !isInt {
		t.Errorf("db-postgres.Vars[\"DBPort\"] bertipe %T, want int", dbPort)
	}
	// Alias YAML `fragment:` → MergeContribution.Fragment harus terisi.
	if len(pg.Contributes) == 0 {
		t.Fatal("db-postgres.Contributes kosong; modul menyumbang ke compose/env")
	}
	for i, c := range pg.Contributes {
		if c.Fragment == "" {
			t.Errorf("db-postgres.Contributes[%d].Fragment kosong (alias template/fragment gagal map)", i)
		}
	}

	// --- db-postgres: requires core (skeleton anchor pemilik) ---
	if !contains(pg.Requires, "core") {
		t.Errorf("db-postgres.Requires = %v, want mengandung \"core\"", pg.Requires)
	}
}

// TestGet_Unknown memastikan Get untuk modul tak terdaftar mengembalikan ok=false.
func TestGet_Unknown(t *testing.T) {
	reg := module.NewRegistry()
	if err := reg.Load(templates.FS); err != nil {
		t.Fatalf("Load gagal: %v", err)
	}
	if m, ok := reg.Get("tidak-ada"); ok {
		t.Errorf("Get(\"tidak-ada\") = (%+v, true), want ok=false", m)
	}
}

// TestLoad_NilFS memastikan Load menolak fs.FS nil dengan error jelas.
func TestLoad_NilFS(t *testing.T) {
	reg := module.NewRegistry()
	if err := reg.Load(nil); err == nil {
		t.Error("Load(nil) = nil, want error")
	}
}

// --- Unit test validasi katalog memakai fixture in-memory (fstest.MapFS) ---

// validCore adalah modul minimal yang valid (pemilik skeleton file shared) —
// dipakai sebagai basis fixture agar referensi requires/anchor punya pemilik.
func validCore() map[string]*fstest.MapFile {
	return map[string]*fstest.MapFile{
		"modules/core/module.yaml": {Data: []byte(`
name: core
description: "skeleton dasar"
files:
  - template: cmd/main.go.tmpl
    target:   cmd/app/main.go
    mode:     render
  - template: compose.yml.tmpl
    target:   docker-compose.yml
    mode:     render
  - template: env.example.tmpl
    target:   .env.example
    mode:     render
contributes:
  - target:   .env.example
    anchor:   app
    template: fragments/env.app.tmpl
    order:    0
`)},
		"modules/core/cmd/main.go.tmpl": {Data: []byte("package main\n")},
		"modules/core/compose.yml.tmpl": {Data: []byte("services:\n")},
		// env.example skeleton WAJIB memuat anchor "region:app" agar kontribusi
		// core (anchor app) lolos verifikasi anchor↔skeleton di Load (M-4).
		"modules/core/env.example.tmpl":       {Data: []byte("# region:app\nAPP=1\n")},
		"modules/core/fragments/env.app.tmpl": {Data: []byte("APP_PORT=8080\n")},
	}
}

// buildFS merakit fstest.MapFS dari beberapa fragmen map.
func buildFS(parts ...map[string]*fstest.MapFile) fstest.MapFS {
	out := fstest.MapFS{}
	for _, p := range parts {
		for k, v := range p {
			out[k] = v
		}
	}
	return out
}

func TestLoad_EmptyCatalog(t *testing.T) {
	// modulesRoot absen → katalog kosong yang valid.
	reg := module.NewRegistry()
	if err := reg.Load(fstest.MapFS{"unrelated.txt": {Data: []byte("x")}}); err != nil {
		t.Fatalf("Load katalog kosong gagal: %v", err)
	}
	if got := reg.All(); len(got) != 0 {
		t.Errorf("All() = %d, want 0 untuk katalog kosong", len(got))
	}
}

func TestLoad_NameMismatchDir(t *testing.T) {
	fsys := fstest.MapFS{
		"modules/foo/module.yaml": {Data: []byte("name: bar\ndescription: x\n")},
	}
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "tidak cocok dengan nama direktori") {
		t.Errorf("Load = %v, want error name≠dir", err)
	}
}

func TestLoad_MissingRequiredField(t *testing.T) {
	fsys := fstest.MapFS{
		// description hilang → D6.1.
		"modules/core/module.yaml": {Data: []byte("name: core\n")},
	}
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Errorf("Load = %v, want error field description wajib", err)
	}
}

func TestLoad_DanglingRequiresIsError(t *testing.T) {
	fsys := buildFS(validCore(), map[string]*fstest.MapFile{
		"modules/http-chi/module.yaml": {Data: []byte(`
name: http-chi
description: "chi"
requires:
  - tidak-ada
`)},
	})
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "requires") {
		t.Errorf("Load = %v, want error dangling requires", err)
	}
}

func TestLoad_DanglingConflictsIsAllowed(t *testing.T) {
	// conflicts ke modul yang belum diimplementasi DIPERBOLEHKAN (katalog bertahap).
	fsys := buildFS(validCore(), map[string]*fstest.MapFile{
		"modules/http-chi/module.yaml": {Data: []byte(`
name: http-chi
description: "chi"
requires:
  - core
conflicts:
  - http-echo
  - http-gin
`)},
	})
	if err := module.NewRegistry().Load(fsys); err != nil {
		t.Errorf("Load = %v, want nil (dangling conflicts diperbolehkan)", err)
	}
}

func TestLoad_RequiresAndConflictsSame(t *testing.T) {
	fsys := buildFS(validCore(), map[string]*fstest.MapFile{
		"modules/x/module.yaml": {Data: []byte(`
name: x
description: "x"
requires:
  - core
conflicts:
  - core
`)},
	})
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "requires DAN conflicts") {
		t.Errorf("Load = %v, want error requires∩conflicts", err)
	}
}

func TestLoad_MissingTemplateFile(t *testing.T) {
	fsys := fstest.MapFS{
		"modules/core/module.yaml": {Data: []byte(`
name: core
description: "x"
files:
  - template: tidak/ada.tmpl
    target:   foo.go
    mode:     render
`)},
	}
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "tidak ada di embed.FS") {
		t.Errorf("Load = %v, want error template path hilang", err)
	}
}

func TestLoad_ContributeTargetNoSkeleton(t *testing.T) {
	// contributes ke target yang tak punya skeleton pemilik di katalog → D6.5.
	fsys := fstest.MapFS{
		"modules/core/module.yaml": {Data: []byte(`
name: core
description: "x"
contributes:
  - target:   Makefile
    anchor:   targets
    template: fragments/make.tmpl
    order:    0
`)},
		"modules/core/fragments/make.tmpl": {Data: []byte("# frag\n")},
	}
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "skeleton pemilik") {
		t.Errorf("Load = %v, want error anchor↔skeleton (target tanpa pemilik)", err)
	}
}

// TestLoad_AnchorTypoInSkeleton memverifikasi M-4: contributes[].anchor yang
// TIDAK ada di skeleton pemilik target → ERROR di Load (fail-fast), bukan tertunda
// sampai Assemble. validCore() menyediakan skeleton .env.example ber-anchor "app";
// kontribusi dengan anchor typo "ap" (bukan "app") harus ditolak.
func TestLoad_AnchorTypoInSkeleton(t *testing.T) {
	fsys := buildFS(validCore(), map[string]*fstest.MapFile{
		"modules/typo/module.yaml": {Data: []byte(`
name: typo
description: "kontribusi dengan anchor typo"
contributes:
  - target:   .env.example
    anchor:   ap
    template: fragments/x.tmpl
    order:    1
`)},
		"modules/typo/fragments/x.tmpl": {Data: []byte("X=1\n")},
	})
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "anchor") {
		t.Errorf("Load = %v, want error anchor typo (M-4)", err)
	}
}

// TestLoad_AnchorValidInSkeleton memverifikasi M-4 positif: contributes[].anchor
// yang ADA di skeleton pemilik lolos Load.
func TestLoad_AnchorValidInSkeleton(t *testing.T) {
	fsys := buildFS(validCore(), map[string]*fstest.MapFile{
		"modules/extra/module.yaml": {Data: []byte(`
name: extra
description: "kontribusi anchor valid"
contributes:
  - target:   .env.example
    anchor:   app
    template: fragments/x.tmpl
    order:    9
`)},
		"modules/extra/fragments/x.tmpl": {Data: []byte("X=1\n")},
	})
	if err := module.NewRegistry().Load(fsys); err != nil {
		t.Errorf("Load = %v, want nil (anchor 'app' ada di skeleton)", err)
	}
}

func TestLoad_UnknownFileMode(t *testing.T) {
	fsys := fstest.MapFS{
		"modules/core/module.yaml": {Data: []byte(`
name: core
description: "x"
files:
  - template: a.tmpl
    target:   a.txt
    mode:     bogus
`)},
		"modules/core/a.tmpl": {Data: []byte("a\n")},
	}
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "mode") {
		t.Errorf("Load = %v, want error mode tak dikenal", err)
	}
}

// TestLoad_RejectsTraversalFileTarget memverifikasi B-1: files[].target yang
// mengandung ".." ditolak di Load (path traversal).
func TestLoad_RejectsTraversalFileTarget(t *testing.T) {
	fsys := fstest.MapFS{
		"modules/core/module.yaml": {Data: []byte(`
name: core
description: "x"
files:
  - template: a.tmpl
    target:   ../../etc/evil
    mode:     render
`)},
		"modules/core/a.tmpl": {Data: []byte("a\n")},
	}
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "..") {
		t.Errorf("Load = %v, want error path traversal '..' (B-1)", err)
	}
}

// TestLoad_RejectsAbsoluteFileTarget memverifikasi B-1: files[].target absolut
// (diawali '/') ditolak di Load.
func TestLoad_RejectsAbsoluteFileTarget(t *testing.T) {
	fsys := fstest.MapFS{
		"modules/core/module.yaml": {Data: []byte(`
name: core
description: "x"
files:
  - template: a.tmpl
    target:   /etc/passwd
    mode:     render
`)},
		"modules/core/a.tmpl": {Data: []byte("a\n")},
	}
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "absolut") {
		t.Errorf("Load = %v, want error path absolut (B-1)", err)
	}
}

// TestLoad_RejectsTraversalContributeTarget memverifikasi B-1: contributes[].target
// ber-".." ditolak di Load.
func TestLoad_RejectsTraversalContributeTarget(t *testing.T) {
	fsys := buildFS(validCore(), map[string]*fstest.MapFile{
		"modules/bad/module.yaml": {Data: []byte(`
name: bad
description: "x"
contributes:
  - target:   ../escape.yml
    anchor:   app
    template: fragments/x.tmpl
    order:    1
`)},
		"modules/bad/fragments/x.tmpl": {Data: []byte("X=1\n")},
	})
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "..") {
		t.Errorf("Load = %v, want error path traversal contributes (B-1)", err)
	}
}

func TestLoad_UnknownYAMLField(t *testing.T) {
	// KnownFields(true): field asing → error parse.
	fsys := fstest.MapFS{
		"modules/core/module.yaml": {Data: []byte("name: core\ndescription: x\nbogusField: 1\n")},
	}
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "YAML tidak valid") {
		t.Errorf("Load = %v, want error field YAML asing", err)
	}
}

// TestLoad_MissingFileTemplate memverifikasi files[].template kosong → error D6.1.
func TestLoad_MissingFileTemplate(t *testing.T) {
	fsys := fstest.MapFS{
		"modules/core/module.yaml": {Data: []byte(`
name: core
description: "x"
files:
  - target: foo.go
    mode:   render
`)},
	}
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "template wajib") {
		t.Errorf("Load = %v, want error files[].template wajib (D6.1)", err)
	}
}

// TestLoad_MissingFileTarget memverifikasi files[].target kosong → error D6.1.
func TestLoad_MissingFileTarget(t *testing.T) {
	fsys := fstest.MapFS{
		"modules/core/module.yaml": {Data: []byte(`
name: core
description: "x"
files:
  - template: a.tmpl
    mode:     render
`)},
		"modules/core/a.tmpl": {Data: []byte("a\n")},
	}
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "target wajib") {
		t.Errorf("Load = %v, want error files[].target wajib (D6.1)", err)
	}
}

// TestLoad_MissingGoModFields memverifikasi gomod[].path & gomod[].version kosong
// → error D6.1 (versi dependency wajib lengkap agar go.mod deterministik).
func TestLoad_MissingGoModFields(t *testing.T) {
	t.Run("path kosong", func(t *testing.T) {
		fsys := fstest.MapFS{
			"modules/core/module.yaml": {Data: []byte("name: core\ndescription: x\ngomod:\n  - version: v1.0.0\n")},
		}
		err := module.NewRegistry().Load(fsys)
		if err == nil || !strings.Contains(err.Error(), "path wajib") {
			t.Errorf("Load = %v, want error gomod[].path wajib", err)
		}
	})
	t.Run("version kosong", func(t *testing.T) {
		fsys := fstest.MapFS{
			"modules/core/module.yaml": {Data: []byte("name: core\ndescription: x\ngomod:\n  - path: example.com/x\n")},
		}
		err := module.NewRegistry().Load(fsys)
		if err == nil || !strings.Contains(err.Error(), "version wajib") {
			t.Errorf("Load = %v, want error gomod[].version wajib", err)
		}
	})
}

// TestLoad_MissingContributeFields memverifikasi contributes[] tanpa anchor atau
// fragment/template → error D6.1.
func TestLoad_MissingContributeFields(t *testing.T) {
	t.Run("anchor kosong", func(t *testing.T) {
		fsys := buildFS(validCore(), map[string]*fstest.MapFile{
			"modules/x/module.yaml": {Data: []byte(`
name: x
description: "x"
contributes:
  - target:   .env.example
    template: fragments/f.tmpl
    order:    1
`)},
			"modules/x/fragments/f.tmpl": {Data: []byte("Y=1\n")},
		})
		err := module.NewRegistry().Load(fsys)
		if err == nil || !strings.Contains(err.Error(), "anchor wajib") {
			t.Errorf("Load = %v, want error contributes[].anchor wajib", err)
		}
	})
	t.Run("fragment/template kosong", func(t *testing.T) {
		fsys := buildFS(validCore(), map[string]*fstest.MapFile{
			"modules/x/module.yaml": {Data: []byte(`
name: x
description: "x"
contributes:
  - target: .env.example
    anchor: app
    order:  1
`)},
		})
		err := module.NewRegistry().Load(fsys)
		if err == nil || !strings.Contains(err.Error(), "template/fragment wajib") {
			t.Errorf("Load = %v, want error contributes[].template/fragment wajib", err)
		}
	})
}

// TestLoad_MissingFragmentFile memverifikasi contributes[].fragment yang path-nya
// tak ada di embed.FS → error D6.6.
func TestLoad_MissingFragmentFile(t *testing.T) {
	fsys := buildFS(validCore(), map[string]*fstest.MapFile{
		"modules/x/module.yaml": {Data: []byte(`
name: x
description: "x"
contributes:
  - target:   .env.example
    anchor:   app
    template: fragments/hilang.tmpl
    order:    1
`)},
		// fragments/hilang.tmpl SENGAJA tidak disediakan.
	})
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "tidak ada di embed.FS") {
		t.Errorf("Load = %v, want error fragment path hilang (D6.6)", err)
	}
}

// TestLoad_SelfRequiresAndSelfConflicts memverifikasi modul tak boleh requires/
// conflicts dirinya sendiri (D6.4).
func TestLoad_SelfRequiresAndSelfConflicts(t *testing.T) {
	t.Run("self-requires", func(t *testing.T) {
		fsys := fstest.MapFS{
			"modules/core/module.yaml": {Data: []byte("name: core\ndescription: x\nrequires:\n  - core\n")},
		}
		err := module.NewRegistry().Load(fsys)
		if err == nil || !strings.Contains(err.Error(), "requires dirinya sendiri") {
			t.Errorf("Load = %v, want error self-requires (D6.4)", err)
		}
	})
	t.Run("self-conflicts", func(t *testing.T) {
		fsys := fstest.MapFS{
			"modules/core/module.yaml": {Data: []byte("name: core\ndescription: x\nconflicts:\n  - core\n")},
		}
		err := module.NewRegistry().Load(fsys)
		if err == nil || !strings.Contains(err.Error(), "conflicts dirinya sendiri") {
			t.Errorf("Load = %v, want error self-conflicts (D6.4)", err)
		}
	})
}

// TestLoad_DuplicateModuleName memverifikasi nama modul harus unik global — tetapi
// karena nama = nama direktori (dijamin unik oleh FS), duplikat hanya bisa terjadi
// bila name field ≠ dir; itu sudah dicegah TestLoad_NameMismatchDir. Di sini kita
// pastikan name kosong (TrimSpace) ditolak D6.1.
func TestLoad_EmptyNameField(t *testing.T) {
	fsys := fstest.MapFS{
		"modules/core/module.yaml": {Data: []byte("name: \"   \"\ndescription: x\n")},
	}
	err := module.NewRegistry().Load(fsys)
	// name "   " (whitespace) ≠ dir "core" → ditolak (name mismatch ATAU name wajib).
	if err == nil {
		t.Errorf("Load = nil, want error untuk name kosong/whitespace")
	}
}

// TestLoad_AnchorBoundaryNotPrefixMatch memverifikasi skeletonHasAnchor TIDAK
// salah-cocok prefix: skeleton ber-anchor "services2" TIDAK memenuhi kontribusi
// ke anchor "services" (typo terdeteksi). Mencegah anchor "services" cocok dengan
// "services2" / "services-extra".
func TestLoad_AnchorBoundaryNotPrefixMatch(t *testing.T) {
	fsys := buildFS(map[string]*fstest.MapFile{
		// core menyediakan skeleton compose ber-anchor "services2" SAJA (bukan "services").
		"modules/core/module.yaml": {Data: []byte(`
name: core
description: "x"
files:
  - template: compose.yml.tmpl
    target:   docker-compose.yml
    mode:     render
`)},
		"modules/core/compose.yml.tmpl": {Data: []byte("services:\n  # region:services2\n")},
	}, map[string]*fstest.MapFile{
		// modul lain berkontribusi ke anchor "services" (TIDAK ada di skeleton — hanya "services2").
		"modules/x/module.yaml": {Data: []byte(`
name: x
description: "x"
contributes:
  - target:   docker-compose.yml
    anchor:   services
    template: fragments/svc.tmpl
    order:    1
`)},
		"modules/x/fragments/svc.tmpl": {Data: []byte("  app: {}\n")},
	})
	err := module.NewRegistry().Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "anchor") {
		t.Errorf("Load = %v, want error anchor 'services' tak cocok dengan 'services2' (boundary, M-4)", err)
	}
}

// names ekstrak nama modul untuk pesan error.
func names(ms []module.Manifest) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Name
	}
	return out
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
