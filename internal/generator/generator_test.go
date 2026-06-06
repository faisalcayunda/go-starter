package generator

import (
	"io/fs"
	"strings"
	"testing"
	"text/template"

	"github.com/faisalcayunda/gostarter/internal/module"
	"github.com/faisalcayunda/gostarter/internal/plan"
)

// fakeRenderer mengembalikan konten template berdasarkan map nama→byte. Ini
// menggantikan Renderer berbasis text/template agar test murni in-memory dan
// deterministik (ADR-002 §9). raw (opsional) memodelkan byte mentah Renderer.Raw
// (ModeCopy, m-4); bila kosong untuk sebuah name, jatuh ke out.
type fakeRenderer struct {
	out map[string]string
	raw map[string]string
}

func (f fakeRenderer) Render(name string, _ any) ([]byte, error) {
	return []byte(f.out[name]), nil
}

// Raw mengembalikan byte mentah (bypass template) — mensimulasikan fsRenderer.Raw.
func (f fakeRenderer) Raw(name string) ([]byte, error) {
	if f.raw != nil {
		if v, ok := f.raw[name]; ok {
			return []byte(v), nil
		}
	}
	return []byte(f.out[name]), nil
}

// fakeAssembler menggabungkan skeleton + konten fragmen terurut secara sederhana
// (append). Cukup untuk menguji bahwa generator memanggil Assemble untuk ModeMerge
// dan menulis hasilnya.
type fakeAssembler struct{}

func (fakeAssembler) Assemble(skeleton []byte, frags []plan.Fragment) ([]byte, error) {
	var b strings.Builder
	b.Write(skeleton)
	for _, fr := range frags {
		b.WriteString(fr.Content)
	}
	return []byte(b.String()), nil
}

// memWriter adalah Writer in-memory: mencatat Mkdir dan WriteFile tanpa menyentuh
// disk. Karena ia bukan *fsutil.DryRunWriter, generator memanggil
// fsutil.EnsureEmptyDir(target) lebih dulu — maka test memakai t.TempDir()
// (dijamin direktori kosong) sebagai target agar cek proteksi-overwrite lolos
// tanpa benar-benar menulis ke sana (penulisan tetap ke memWriter, in-memory).
type memWriter struct {
	dirs  []string
	files map[string][]byte
}

func newMemWriter() *memWriter { return &memWriter{files: map[string][]byte{}} }

func (m *memWriter) Mkdir(path string) error { m.dirs = append(m.dirs, path); return nil }

func (m *memWriter) WriteFile(path string, data []byte, _ fs.FileMode) error {
	m.files[path] = data
	return nil
}

// fakeRegistry adalah Registry kosong (generator tidak meng-query registry selama
// Generate; rencana sudah lengkap dari resolver).
type fakeRegistry struct{}

func (fakeRegistry) Load(fs.FS) error                   { return nil }
func (fakeRegistry) Get(string) (module.Manifest, bool) { return module.Manifest{}, false }
func (fakeRegistry) All() []module.Manifest             { return nil }

// TestGenerate_DryRunAssemblesGoModAndDispatchesModes memverifikasi: (a) DryRunWriter
// melewati EnsureEmptyDir (tak ada panic dari stub fsutil), (b) keempat mode
// dieksekusi, (c) go.mod dirakit deterministik via modfile dengan require terurut.
func TestGenerate_DryRunAssemblesGoModAndDispatchesModes(t *testing.T) {
	r := fakeRenderer{out: map[string]string{
		"core/main.go.tmpl":      "package main\nfunc main(){}\n",
		"core/README.md.tmpl":    "# proj\n",
		"core/compose.skel.tmpl": "services:\n",
		// Fragment.Content menyimpan PATH fragmen; generator merendernya via
		// Renderer sebelum Assemble (ADR-002 §3.3/§6). Key = path fragmen.
		"core/fragments/compose.app.tmpl": "  app:\n",
	}}
	g := New(fakeRegistry{}, r, fakeAssembler{})

	p := plan.GeneratePlan{
		ProjectName: "proj",
		ModulePath:  "github.com/acme/proj",
		GoVersion:   "1.25",
		Files: []plan.FileOp{
			{Mode: plan.ModeMkdir, TargetPath: "cmd/proj"},
			{Mode: plan.ModeRender, TargetPath: "cmd/proj/main.go", TemplatePath: "core/main.go.tmpl"},
			{Mode: plan.ModeCopy, TargetPath: "README.md", TemplatePath: "core/README.md.tmpl"},
			{Mode: plan.ModeMerge, TargetPath: "docker-compose.yml", TemplatePath: "core/compose.skel.tmpl",
				Fragments: []plan.Fragment{{Anchor: "services", Content: "core/fragments/compose.app.tmpl", Order: 10}}},
		},
		// Sengaja tidak terurut + duplikat untuk menguji dedupSortDeps.
		Deps: []plan.ModuleDep{
			{Path: "github.com/jackc/pgx/v5", Version: "v5.10.0"},
			{Path: "github.com/go-chi/chi/v5", Version: "v5.3.0"},
			{Path: "github.com/jackc/pgx/v5", Version: "v5.10.0"},
		},
	}

	// target = t.TempDir() (direktori kosong) agar EnsureEmptyDir lolos; konten
	// sesungguhnya ditulis ke memWriter (in-memory), bukan ke disk.
	target := t.TempDir()
	mw := newMemWriter()
	if err := g.Generate(p, target, mw); err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// go.mod harus ada, dengan require terurut by Path dan dedup.
	gomod, ok := mw.files[target+"/go.mod"]
	if !ok {
		t.Fatalf("go.mod tidak ditulis; files=%v", keys(mw.files))
	}
	got := string(gomod)
	for _, want := range []string{
		"module github.com/acme/proj",
		"go 1.25",
		"github.com/go-chi/chi/v5 v5.3.0",
		"github.com/jackc/pgx/v5 v5.10.0",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("go.mod tidak memuat %q; isi:\n%s", want, got)
		}
	}
	// chi harus muncul sebelum pgx (terurut by Path) dan hanya sekali (dedup).
	if strings.Index(got, "chi/v5") > strings.Index(got, "pgx/v5") {
		t.Errorf("require tidak terurut by Path:\n%s", got)
	}
	if strings.Count(got, "pgx/v5") != 1 {
		t.Errorf("pgx tidak ter-dedup (muncul %d kali):\n%s", strings.Count(got, "pgx/v5"), got)
	}

	// ModeRender + go/format: main.go ada & gofmt-valid (sudah lewat format.Source).
	if _, ok := mw.files[target+"/cmd/proj/main.go"]; !ok {
		t.Errorf("main.go (ModeRender) tidak ditulis")
	}
	// ModeCopy: README disalin apa adanya.
	if string(mw.files[target+"/README.md"]) != "# proj\n" {
		t.Errorf("README.md (ModeCopy) salah: %q", mw.files[target+"/README.md"])
	}
	// ModeMerge: skeleton + fragmen tergabung.
	if mc := string(mw.files[target+"/docker-compose.yml"]); !strings.Contains(mc, "services:") || !strings.Contains(mc, "app:") {
		t.Errorf("compose (ModeMerge) tidak tergabung: %q", mc)
	}
	// ModeMkdir tercatat.
	if len(mw.dirs) != 1 || mw.dirs[0] != target+"/cmd/proj" {
		t.Errorf("Mkdir tidak sesuai: %v", mw.dirs)
	}
}

// TestGenerate_UnknownModeErrors memverifikasi mode tak dikenal menghasilkan error
// (bukan panic / silent skip).
func TestGenerate_UnknownModeErrors(t *testing.T) {
	g := New(fakeRegistry{}, fakeRenderer{out: map[string]string{}}, fakeAssembler{})
	p := plan.GeneratePlan{
		ModulePath: "github.com/acme/x",
		GoVersion:  "1.25",
		Files:      []plan.FileOp{{Mode: plan.FileOpMode(99), TargetPath: "weird"}},
	}
	if err := g.Generate(p, t.TempDir(), newMemWriter()); err == nil {
		t.Fatal("ekspektasi error untuk Mode tak dikenal, dapat nil")
	}
}

// TestGenerate_ModeCopyRawBypassesTemplate memverifikasi ModeCopy membaca byte
// MENTAH via Renderer.Raw (m-4): file statik yang memuat literal "{{" disalin apa
// adanya — TIDAK dilewatkan template engine (yang akan gagal parse). Sekaligus
// memastikan Render (yang akan error pada "{{") TIDAK dipanggil untuk ModeCopy.
func TestGenerate_ModeCopyRawBypassesTemplate(t *testing.T) {
	// Konten mengandung "{{" tanpa penutup — akan gagal bila diparse text/template.
	const sqlBody = "-- migrasi\nINSERT INTO t (v) VALUES ('{{ literal }}');\n"
	r := fakeRenderer{
		out: map[string]string{}, // Render TIDAK boleh dipanggil untuk ModeCopy
		raw: map[string]string{"db/migrations/0001.up.sql.tmpl": sqlBody},
	}
	g := New(fakeRegistry{}, r, fakeAssembler{})

	p := plan.GeneratePlan{
		ProjectName: "proj",
		ModulePath:  "github.com/acme/proj",
		GoVersion:   "1.24",
		Files: []plan.FileOp{
			{Mode: plan.ModeCopy, TargetPath: "migrations/0001.up.sql", TemplatePath: "db/migrations/0001.up.sql.tmpl"},
		},
	}

	target := t.TempDir()
	mw := newMemWriter()
	if err := g.Generate(p, target, mw); err != nil {
		t.Fatalf("Generate ModeCopy dengan '{{' gagal: %v", err)
	}
	got := string(mw.files[target+"/migrations/0001.up.sql"])
	if got != sqlBody {
		t.Errorf("ModeCopy tidak menyalin byte mentah:\n got: %q\nwant: %q", got, sqlBody)
	}
}

// TestGenerate_RejectsPathTraversal memverifikasi B-1: FileOp dengan TargetPath
// yang keluar dari direktori target (mis. "../../etc/x") ditolak generator
// (lapis pertahanan kedua setelah registry/resolver). path.Join SAJA tidak
// menahan ini — joinTarget menambahkan cek containment via filepath.Rel.
func TestGenerate_RejectsPathTraversal(t *testing.T) {
	r := fakeRenderer{out: map[string]string{"x.tmpl": "x\n"}, raw: map[string]string{"x.tmpl": "x\n"}}
	g := New(fakeRegistry{}, r, fakeAssembler{})
	for _, evil := range []string{"../../etc/evil", "../escape", "a/../../b"} {
		p := plan.GeneratePlan{
			ModulePath: "github.com/acme/x",
			GoVersion:  "1.24",
			Files:      []plan.FileOp{{Mode: plan.ModeCopy, TargetPath: evil, TemplatePath: "x.tmpl"}},
		}
		err := g.Generate(p, t.TempDir(), newMemWriter())
		if err == nil {
			t.Errorf("Generate dengan TargetPath %q: mau error path traversal, dapat nil", evil)
		}
	}
}

// dataEchoRenderer mengeksekusi template sungguhan (text/template) atas data agar
// test dapat memverifikasi bahwa op.DataOverride / Fragment.DataOverride benar-benar
// memengaruhi hasil render. Render menutup template per-name; Raw mengembalikan byte
// mentah name. Dipakai khusus untuk test override data (per-op & per-fragmen).
type dataEchoRenderer struct {
	tmpls map[string]string
}

func (d dataEchoRenderer) Render(name string, data any) ([]byte, error) {
	t, err := template.New(name).Parse(d.tmpls[name])
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func (d dataEchoRenderer) Raw(name string) ([]byte, error) { return []byte(d.tmpls[name]), nil }

// TestGenerate_DataOverridePerOp memverifikasi T4.2 multi-service: dua FileOp
// ModeRender dari TEMPLATE YANG SAMA dengan DataOverride berbeda menghasilkan output
// berbeda sesuai override — dan override MENANG atas data global (op.Data).
func TestGenerate_DataOverridePerOp(t *testing.T) {
	r := dataEchoRenderer{tmpls: map[string]string{
		// Satu template per-service dipakai N kali. {{ .ServiceName }} berasal dari
		// DataOverride; {{ .ModulePath }} berasal dari Data global (dibagi).
		"core/service.go.tmpl": "package {{ .ServiceName }} // {{ .ModulePath }} env={{ .Env }}",
	}}
	g := New(fakeRegistry{}, r, fakeAssembler{})

	global := map[string]any{"ModulePath": "github.com/acme/proj", "Env": "base"}
	p := plan.GeneratePlan{
		ProjectName: "proj",
		ModulePath:  "github.com/acme/proj",
		GoVersion:   "1.26",
		Files: []plan.FileOp{
			{
				Mode:         plan.ModeRender,
				TargetPath:   "services/alpha/svc.go",
				TemplatePath: "core/service.go.tmpl",
				Data:         global,
				DataOverride: map[string]any{"ServiceName": "alpha", "Env": "prod"}, // Env menimpa global
			},
			{
				Mode:         plan.ModeRender,
				TargetPath:   "services/beta/svc.go",
				TemplatePath: "core/service.go.tmpl",
				Data:         global,
				DataOverride: map[string]any{"ServiceName": "beta"},
			},
		},
	}

	target := t.TempDir()
	mw := newMemWriter()
	if err := g.Generate(p, target, mw); err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	alpha := string(mw.files[target+"/services/alpha/svc.go"])
	beta := string(mw.files[target+"/services/beta/svc.go"])

	// Output dua op dari template sama HARUS berbeda sesuai override.
	if alpha == beta {
		t.Fatalf("dua FileOp dari template sama dgn DataOverride beda mestinya beda:\nalpha=%q\nbeta=%q", alpha, beta)
	}
	// ServiceName per-op masuk.
	if !strings.Contains(alpha, "package alpha") {
		t.Errorf("alpha tidak memuat ServiceName override: %q", alpha)
	}
	if !strings.Contains(beta, "package beta") {
		t.Errorf("beta tidak memuat ServiceName override: %q", beta)
	}
	// Data global (ModulePath) tetap terbaca di kedua op.
	if !strings.Contains(alpha, "github.com/acme/proj") || !strings.Contains(beta, "github.com/acme/proj") {
		t.Errorf("ModulePath global tidak ter-merge:\nalpha=%q\nbeta=%q", alpha, beta)
	}
	// Override MENANG atas global: alpha.Env=prod (override), beta.Env=base (global,
	// tak ada override → fallback global). Membuktikan presedensi & non-mutasi base.
	if !strings.Contains(alpha, "env=prod") {
		t.Errorf("DataOverride tidak menang atas Data global (alpha env): %q", alpha)
	}
	if !strings.Contains(beta, "env=base") {
		t.Errorf("Data global termutasi oleh op lain — beta env mestinya base: %q", beta)
	}
}

// TestGenerate_DataOverridePerFragment memverifikasi Fragment.DataOverride: satu
// skeleton ModeMerge merakit fragmen per-service dari template fragmen yang sama
// dengan nilai berbeda → konten merge memuat kedua service.
func TestGenerate_DataOverridePerFragment(t *testing.T) {
	r := dataEchoRenderer{tmpls: map[string]string{
		// Skeleton membawa marker anchor netral "# region:services" (zero lock-in)
		// yang assembler ganti dengan fragmen per-service terurut.
		"core/compose.skel.tmpl":  "services:\n  # region:services\n",
		"core/fragments/svc.tmpl": "  {{ .ServiceName }}: { build: ./services/{{ .ServiceName }} } # proj={{ .ModulePath }}",
	}}
	g := New(fakeRegistry{}, r, NewMergeAssembler())

	global := map[string]any{"ModulePath": "github.com/acme/proj"}
	p := plan.GeneratePlan{
		ProjectName: "proj",
		ModulePath:  "github.com/acme/proj",
		GoVersion:   "1.26",
		Files: []plan.FileOp{
			{
				Mode:         plan.ModeMerge,
				TargetPath:   "docker-compose.yml",
				TemplatePath: "core/compose.skel.tmpl",
				Data:         global,
				Fragments: []plan.Fragment{
					{Anchor: "services", Content: "core/fragments/svc.tmpl", Order: 10, DataOverride: map[string]any{"ServiceName": "alpha"}},
					{Anchor: "services", Content: "core/fragments/svc.tmpl", Order: 20, DataOverride: map[string]any{"ServiceName": "beta"}},
				},
			},
		},
	}

	target := t.TempDir()
	mw := newMemWriter()
	if err := g.Generate(p, target, mw); err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	got := string(mw.files[target+"/docker-compose.yml"])
	for _, want := range []string{"alpha:", "beta:", "github.com/acme/proj"} {
		if !strings.Contains(got, want) {
			t.Errorf("compose merge tidak memuat %q:\n%s", want, got)
		}
	}
	// Urutan fragmen deterministik (alpha sebelum beta by Order).
	if strings.Index(got, "alpha:") > strings.Index(got, "beta:") {
		t.Errorf("fragmen tidak terurut by Order:\n%s", got)
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
