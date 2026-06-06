// Package module memodelkan sistem template modular: tiap modul template punya
// manifest (module.yaml) yang mendeskripsikan file yang dibawa, dependency
// go.mod, requires/conflicts, dan kontribusi merge ke file shared.
//
// SKELETON (Fase 2): hanya tipe data yang mencerminkan skema module.yaml.
// Parsing YAML (loader) diimplementasikan pada Fase 3 (lihat registry.go).
//
// Kontrak tipe di file ini KANONIK terhadap ADR-002 §Decision 3.2
// (docs/adr/ADR-002-internal-architecture.md). Nama, tipe, dan tag YAML wajib
// persis sama; perubahan struktural mensyaratkan ADR baru.
package module

// FileSpec adalah satu file yang dibawa modul (cermin module.yaml `files[]`,
// ADR-003 D2). Mode menentukan cara file diproses generator.
type FileSpec struct {
	// Template adalah path .tmpl relatif terhadap dir modul di embed.FS.
	Template string `yaml:"template"`
	// Target adalah path relatif project hasil generate. Boleh mengandung
	// placeholder template (mis. "internal/{{.Feature}}/handler.go") yang
	// dievaluasi resolver saat merakit FileOp.
	Target string `yaml:"target"`
	// Mode: "render" (default), "copy", atau "mkdir". Merge TIDAK di sini
	// (lihat MergeContribution / contributes). Dipetakan ke plan.FileOpMode
	// saat resolver merakit FileOp.
	Mode string `yaml:"mode"`
	// When adalah ekspresi kondisi opsional (ADR-002 §5). Kosong = aktif selama
	// modulnya aktif.
	When string `yaml:"when"`
}

// MergeContribution adalah kontribusi satu modul ke sebuah file SHARED yang
// dirakit dari banyak modul (mis. docker-compose.yml, Makefile, .env.example,
// atau blok wiring di main) — selalu via assembler, menghasilkan FileOp ber-mode
// plan.ModeMerge. Cermin module.yaml `contributes[]` (ADR-003 D2/D5).
type MergeContribution struct {
	// Target adalah file shared tujuan, relatif root project hasil generate
	// (mis. "docker-compose.yml").
	Target string `yaml:"target"`
	// Anchor adalah nama section/anchor pada skeleton file shared (mis.
	// "services", "volumes", "app", "database", "broker"). Generator memakai
	// anchor untuk menempatkan fragment.
	Anchor string `yaml:"anchor"`
	// Fragment adalah path fragmen .tmpl (lazim di fragments/) yang di-render
	// lalu disisipkan pada anchor. Di module.yaml field ini boleh ditulis sebagai
	// `template` atau `fragment` (alias); keduanya memetakan ke field ini.
	Fragment string `yaml:"fragment"`
	// When adalah kondisi opsional fragment-level (ADR-002 §5). Kosong = aktif.
	When string `yaml:"when"`
	// Order menentukan urutan deterministik dalam satu (Target, Anchor),
	// menjaga output stabil/byte-identical (SPEC §5.2). Tie-break: nama modul.
	Order int `yaml:"order"`
}

// ModuleDep mencerminkan satu entri dependency pada module.yaml `gomod[]`
// (path + version). Identik bentuk dengan plan.ModuleDep.
type ModuleDep struct {
	Path    string `yaml:"path"`
	Version string `yaml:"version"`
}

// Manifest adalah representasi in-memory dari satu module.yaml (skema ADR-003 D2).
type Manifest struct {
	// Name adalah identitas unik modul template (mis. "core", "http-chi").
	Name string `yaml:"name"`
	// Description adalah keterangan singkat peran modul.
	Description string `yaml:"description"`
	// Files adalah daftar file yang dibawa modul ini (render/copy/mkdir) ke
	// project hasil generate.
	Files []FileSpec `yaml:"files"`
	// GoMod adalah dependency go.mod yang dibawa modul ini ke project hasil
	// generate (dialirkan ke plan.Deps lalu dirakit via modfile.Format, BUKAN
	// sebagai fragment teks — ADR-002 §6).
	GoMod []ModuleDep `yaml:"gomod"`
	// Requires adalah nama-nama modul lain yang wajib aktif bila modul ini aktif.
	Requires []string `yaml:"requires"`
	// Conflicts adalah nama-nama modul yang tidak boleh aktif bersamaan dengan modul ini.
	Conflicts []string `yaml:"conflicts"`
	// Contributes adalah kontribusi merge modul ini ke file shared.
	Contributes []MergeContribution `yaml:"contributes"`
	// Vars adalah default var modul yang digabung ke template context. Bertipe
	// map[string]any agar nilai non-string muat (mis. DBPort: 5432).
	Vars map[string]any `yaml:"vars"`
}
