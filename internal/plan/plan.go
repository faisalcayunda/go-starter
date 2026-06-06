// Package plan mendefinisikan GeneratePlan — rencana generate deterministik yang
// dihasilkan resolver dari answers.Answers dan dikonsumsi generator.
//
// GeneratePlan adalah satu-satunya kontrak antara fase keputusan (resolver) dan
// fase render (generator): ia mendaftar operasi file, dependency go.mod, dan
// hook pasca-generate. SKELETON (Fase 2): hanya tipe data, tanpa logika.
//
// Kontrak tipe di file ini KANONIK terhadap ADR-002 §Decision 3.3
// (docs/adr/ADR-002-internal-architecture.md). Nama, tipe, dan konstanta wajib
// persis sama; perubahan struktural mensyaratkan ADR baru.
package plan

import "io/fs"

// FileOpMode membedakan cara satu FileOp diproses generator (ADR-002 §3.3/§6).
type FileOpMode int

const (
	// ModeRender me-render satu template (.tmpl) → (go/format bila .go) → tulis.
	ModeRender FileOpMode = iota
	// ModeCopy menyalin file dari embed.FS apa adanya (tanpa render).
	ModeCopy
	// ModeMkdir membuat direktori.
	ModeMkdir
	// ModeMerge merakit file shared dari Fragments terurut per anchor.
	ModeMerge
)

// Fragment adalah satu potongan terender yang disisipkan pada anchor sebuah file
// shared (dipakai hanya oleh ModeMerge). Resolver mengisi Content dari render
// fragmen modul (atau menunda render ke assembler — lihat ADR-002 §6); Order
// menentukan urutan deterministik dalam satu anchor.
type Fragment struct {
	// Anchor adalah nama anchor tujuan pada skeleton (mis. "services").
	Anchor string
	// Content adalah konten fragmen (terender) yang disisipkan pada anchor.
	Content string
	// Order menentukan urutan deterministik dalam satu anchor (tie-break: nama modul).
	Order int
	// DataOverride (OPSIONAL) adalah override data render khusus fragmen ini, di-MERGE
	// di atas data context FileOp induk saat render fragmen (key fragmen menang).
	// Dipakai bila satu skeleton merge merakit fragmen per-service dari template sama
	// dengan nilai berbeda (mis. ServiceName). nil → pakai data induk apa adanya.
	// Determinisme dijaga selama resolver mengisi nilai deterministik.
	DataOverride map[string]any
}

// FileOp adalah satu operasi file dalam GeneratePlan.
type FileOp struct {
	// Mode adalah jenis operasi (render/copy/mkdir/merge).
	Mode FileOpMode
	// TargetPath adalah path relatif tujuan di project hasil generate.
	TargetPath string
	// ModuleName adalah nama modul asal (audit/dry-run; "" untuk file shared
	// hasil merge multi-modul).
	ModuleName string
	// TemplatePath adalah path template sumber di embed.FS (untuk ModeRender/
	// ModeCopy, dan skeleton untuk ModeMerge). Kosong untuk ModeMkdir.
	TemplatePath string
	// Fragments adalah daftar fragmen terurut untuk ModeMerge (kosong untuk mode
	// lain).
	Fragments []Fragment
	// Perm adalah permission file/direktori.
	Perm fs.FileMode
	// Data adalah context render template global (proyeksi Answers + Vars modul).
	// Untuk ModeRender/ModeMerge ini menjadi data dasar render.
	Data any
	// DataOverride (OPSIONAL) adalah override data render khusus FileOp ini, di-MERGE
	// di ATAS data global (Data) saat render — key di DataOverride MENANG. Dipakai saat
	// satu template per-service di-render N kali dengan nilai berbeda (mis. ServiceName,
	// GrpcPort) tanpa menduplikasi seluruh context. nil → render hanya dengan Data.
	//
	// Kontrak determinisme: resolver WAJIB mengisi DataOverride dengan nilai
	// deterministik (urutan iterasi map TIDAK memengaruhi hasil karena hanya
	// dibaca per-key saat render) → output tetap byte-identical untuk Answers sama.
	//
	// Hanya bermakna untuk ModeRender & ModeMerge; diabaikan untuk ModeCopy/ModeMkdir.
	DataOverride map[string]any
}

// ModuleDep adalah satu dependency go.mod yang masuk ke project hasil generate.
// Identik bentuk dengan module.ModuleDep.
type ModuleDep struct {
	Path    string // mis. "github.com/go-chi/chi/v5"
	Version string // mis. "v5.3.0"
}

// HookSpec mendeskripsikan satu hook pasca-generate yang harus dijalankan
// (mis. go mod tidy, gofmt, git init) beserta urutannya.
type HookSpec struct {
	Name  string   // nama hook, cocok dengan PostGenHook.Name()
	Order int      // urutan eksekusi (menaik)
	Args  []string // argumen tambahan bila ada
}

// GeneratePlan adalah rencana lengkap hasil resolver: daftar operasi file,
// dependency go.mod terkumpul (dedup+sort di Fase 3 → modfile.Format), dan hook
// pasca-generate. Generator mengeksekusi rencana ini secara deterministik.
type GeneratePlan struct {
	// ProjectName adalah nama folder root project hasil generate.
	ProjectName string
	// ModulePath adalah go module path project hasil generate.
	ModulePath string
	// GoVersion adalah go directive untuk go.mod project hasil generate.
	GoVersion string
	// Files adalah daftar operasi file terurut (stabil).
	Files []FileOp
	// Deps adalah dependency go.mod yang masuk ke project hasil generate
	// (dedup+sort) → modfile.Format.
	Deps []ModuleDep
	// Hooks adalah hook pasca-generate terurut.
	Hooks []HookSpec
}
