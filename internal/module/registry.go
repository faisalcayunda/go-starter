// Package module — registry memuat dan mengindeks manifest modul template dari
// sebuah fs.FS (umumnya embed.FS pada package templates).
//
// Registry men-scan `modules/<name>/module.yaml`, mem-parse tiap manifest dengan
// gopkg.in/yaml.v3, mengindeks per Name, lalu MEMVALIDASI katalog (ADR-003 D6:
// field wajib, keunikan Name, referensi requires/conflicts, anchor↔skeleton,
// path template ada di FS) secara fail-fast.
//
// Catatan pembagian validasi (ADR-003 D6): Load memvalidasi KATALOG (integritas
// manifest antar-modul yang terdaftar). Validasi SELEKSI user (kombinasi opsi
// terhadap constraint matrix SPEC §6) BUKAN di sini — itu tanggung jawab
// resolver.Resolve.
package module

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// modulesRoot adalah direktori akar tempat tiap subdir berisi satu module.yaml.
// Selaras titik embed templates.FS (//go:embed modules) — ADR-002 §Decision-1.
const modulesRoot = "modules"

// manifestFile adalah nama file manifest di tiap direktori modul (ADR-003 D1).
const manifestFile = "module.yaml"

// Registry adalah indeks manifest modul template yang dapat di-query per nama.
type Registry interface {
	// Load men-scan fsys (mis. templates.FS), mem-parse setiap module.yaml,
	// MEMVALIDASI katalog (ADR-003 D6), lalu mengindeks manifest by Name.
	// Fail-fast: error pertama yang ditemukan menghentikan load.
	Load(fsys fs.FS) error
	// Get mengembalikan manifest berdasarkan nama modul; ok=false bila tak ada.
	Get(name string) (Manifest, bool)
	// All mengembalikan seluruh manifest yang termuat, urut deterministik by Name.
	All() []Manifest
}

// registry adalah implementasi Registry default: indeks manifest by Name.
type registry struct {
	// byName memetakan Name modul → manifest. Diisi saat Load sukses.
	byName map[string]Manifest
}

// NewRegistry mengembalikan implementasi Registry kosong; panggil Load untuk
// mengisinya dari sebuah fs.FS (umumnya templates.FS).
func NewRegistry() Registry {
	return &registry{byName: make(map[string]Manifest)}
}

// Load men-scan `modulesRoot/*/module.yaml` pada fsys, mem-parse tiap manifest,
// memvalidasi katalog, lalu mengindeks by Name. Bila ada error validasi, index
// TIDAK diubah (atomik terhadap state lama) dan error dikembalikan.
func (r *registry) Load(fsys fs.FS) error {
	if fsys == nil {
		return errors.New("module: Load butuh fs.FS non-nil")
	}

	// 1. Kumpulkan path module.yaml di bawah modulesRoot. Bila modulesRoot tidak
	//    ada, perlakukan sebagai katalog kosong yang valid (memudahkan fixture
	//    parsial pada test).
	dirs, err := readModuleDirs(fsys)
	if err != nil {
		return err
	}

	// 2. Parse tiap manifest. Validasi struktural per-manifest (D6 poin 1) dan
	//    keunikan Name (D6 poin 2) dilakukan saat akumulasi.
	loaded := make(map[string]Manifest, len(dirs))
	for _, dir := range dirs {
		manifestPath := path.Join(modulesRoot, dir, manifestFile)
		m, perr := parseManifest(fsys, manifestPath)
		if perr != nil {
			return perr
		}
		// Nama modul wajib = nama direktori (ADR-003 D1/D6 poin 2).
		if m.Name != dir {
			return fmt.Errorf("module %q: field name=%q tidak cocok dengan nama direktori %q (ADR-003 D1)", manifestPath, m.Name, dir)
		}
		if _, dup := loaded[m.Name]; dup {
			return fmt.Errorf("module %q: nama duplikat — name harus unik global (ADR-003 D6 poin 2)", m.Name)
		}
		loaded[m.Name] = m
	}

	// 3. Validasi lintas-manifest: referensi requires/conflicts, konsistensi
	//    requires↔conflicts, anchor↔skeleton, path template ada di FS.
	if verr := validateCatalog(fsys, loaded); verr != nil {
		return verr
	}

	// 4. Commit: ganti index hanya bila seluruh validasi lolos.
	r.byName = loaded
	return nil
}

// Get mengembalikan manifest by nama; ok=false bila tak terdaftar.
func (r *registry) Get(name string) (Manifest, bool) {
	m, ok := r.byName[name]
	return m, ok
}

// All mengembalikan seluruh manifest terurut by Name (deterministik) — penting
// untuk output dry-run & golden-file yang stabil (SPEC §5.2).
func (r *registry) All() []Manifest {
	out := make([]Manifest, 0, len(r.byName))
	for _, m := range r.byName {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// readModuleDirs mengembalikan nama subdirektori di bawah modulesRoot yang
// memuat sebuah module.yaml, terurut. Modul tanpa module.yaml dilewati (bukan
// modul template). Bila modulesRoot tidak ada → slice kosong, nil error.
func readModuleDirs(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, modulesRoot)
	if err != nil {
		// modulesRoot absen = katalog kosong (valid). Error lain diteruskan.
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("module: gagal membaca %q: %w", modulesRoot, err)
	}

	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := path.Join(modulesRoot, e.Name(), manifestFile)
		if _, statErr := fs.Stat(fsys, manifestPath); statErr != nil {
			// Subdir tanpa module.yaml bukan modul template — lewati senyap.
			continue
		}
		dirs = append(dirs, e.Name())
	}
	sort.Strings(dirs)
	return dirs, nil
}

// parseManifest membaca & mem-parse satu module.yaml menjadi Manifest, lalu
// memeriksa field wajib level-entri (ADR-003 D6 poin 1). KnownFields strict agar
// typo field ketahuan dini.
func parseManifest(fsys fs.FS, manifestPath string) (Manifest, error) {
	raw, err := fs.ReadFile(fsys, manifestPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("module %q: gagal baca manifest: %w", manifestPath, err)
	}

	// Alias YAML: contributes[].fragment boleh ditulis `template` atau `fragment`
	// (ADR-002 §3.2 / ADR-003 D2). Parse via tipe bayangan lalu petakan.
	var shadow manifestYAML
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if derr := dec.Decode(&shadow); derr != nil {
		return Manifest{}, fmt.Errorf("module %q: YAML tidak valid: %w", manifestPath, derr)
	}

	m := shadow.toManifest()

	if err := validateManifestFields(manifestPath, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// manifestYAML adalah bayangan Manifest untuk parsing: menangkap alias
// contributes[].template|fragment yang tidak bisa diekspresikan satu tag yaml.
type manifestYAML struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Files       []FileSpec     `yaml:"files"`
	GoMod       []ModuleDep    `yaml:"gomod"`
	Requires    []string       `yaml:"requires"`
	Conflicts   []string       `yaml:"conflicts"`
	Contributes []contribYAML  `yaml:"contributes"`
	Vars        map[string]any `yaml:"vars"`
}

// contribYAML menangkap dua alias path fragmen: `template` dan `fragment`.
type contribYAML struct {
	Target   string `yaml:"target"`
	Anchor   string `yaml:"anchor"`
	Template string `yaml:"template"`
	Fragment string `yaml:"fragment"`
	When     string `yaml:"when"`
	Order    int    `yaml:"order"`
}

// toManifest memetakan bayangan YAML → Manifest kanonik, meratakan alias
// template/fragment ke field Fragment.
func (y manifestYAML) toManifest() Manifest {
	contribs := make([]MergeContribution, 0, len(y.Contributes))
	for _, c := range y.Contributes {
		frag := c.Fragment
		if frag == "" {
			frag = c.Template // alias: `template` dipakai bila `fragment` kosong
		}
		contribs = append(contribs, MergeContribution{
			Target:   c.Target,
			Anchor:   c.Anchor,
			Fragment: frag,
			When:     c.When,
			Order:    c.Order,
		})
	}
	return Manifest{
		Name:        y.Name,
		Description: y.Description,
		Files:       y.Files,
		GoMod:       y.GoMod,
		Requires:    y.Requires,
		Conflicts:   y.Conflicts,
		Contributes: contribs,
		Vars:        y.Vars,
	}
}

// validateManifestFields menegakkan ADR-003 D6 poin 1: field wajib ada pada
// level manifest dan tiap entri files/gomod/contributes.
func validateManifestFields(manifestPath string, m Manifest) error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("module %q: field `name` wajib (ADR-003 D6.1)", manifestPath)
	}
	if strings.TrimSpace(m.Description) == "" {
		return fmt.Errorf("module %q: field `description` wajib (ADR-003 D6.1)", manifestPath)
	}
	for i, f := range m.Files {
		if strings.TrimSpace(f.Template) == "" {
			return fmt.Errorf("module %q: files[%d].template wajib (ADR-003 D6.1)", m.Name, i)
		}
		if strings.TrimSpace(f.Target) == "" {
			return fmt.Errorf("module %q: files[%d].target wajib (ADR-003 D6.1)", m.Name, i)
		}
		if err := checkSafeTarget(f.Target); err != nil {
			return fmt.Errorf("module %q: files[%d].target %v", m.Name, i, err)
		}
		if !validFileMode(f.Mode) {
			return fmt.Errorf("module %q: files[%d].mode=%q tidak dikenal (render|copy|mkdir; kosong=render) (ADR-003 D2)", m.Name, i, f.Mode)
		}
	}
	for i, d := range m.GoMod {
		if strings.TrimSpace(d.Path) == "" {
			return fmt.Errorf("module %q: gomod[%d].path wajib (ADR-003 D6.1)", m.Name, i)
		}
		if strings.TrimSpace(d.Version) == "" {
			return fmt.Errorf("module %q: gomod[%d].version wajib (ADR-003 D6.1)", m.Name, i)
		}
	}
	for i, c := range m.Contributes {
		if strings.TrimSpace(c.Target) == "" {
			return fmt.Errorf("module %q: contributes[%d].target wajib (ADR-003 D6.1)", m.Name, i)
		}
		if err := checkSafeTarget(c.Target); err != nil {
			return fmt.Errorf("module %q: contributes[%d].target %v", m.Name, i, err)
		}
		if strings.TrimSpace(c.Anchor) == "" {
			return fmt.Errorf("module %q: contributes[%d].anchor wajib (ADR-003 D6.1)", m.Name, i)
		}
		if strings.TrimSpace(c.Fragment) == "" {
			return fmt.Errorf("module %q: contributes[%d].template/fragment wajib (ADR-003 D6.1)", m.Name, i)
		}
	}
	return nil
}

// checkSafeTarget menolak path target yang dapat memecah containment project
// (B-1 path traversal). Target adalah path relatif DI DALAM project hasil
// generate; ia TIDAK boleh:
//   - diawali "/" (absolut), atau
//   - mengandung segmen ".." (naik ke atas direktori project).
//
// Pemeriksaan dilakukan atas path mentah (sebelum render placeholder) DAN pada
// bentuk ternormalisasi. Catatan: target boleh memuat placeholder template
// (mis. "cmd/{{ modBase .ModulePath }}/main.go"); placeholder tidak mengandung
// "/" atau ".." berbahaya pada FuncMap path-murni, sehingga cek mentah aman.
func checkSafeTarget(target string) error {
	t := strings.TrimSpace(target)
	if strings.HasPrefix(t, "/") {
		return fmt.Errorf("%q tidak boleh diawali '/' (path absolut; harus relatif terhadap project) (B-1)", target)
	}
	// Pisah pada slash POSIX (SPEC §2.1) dan tolak segmen "..".
	for _, seg := range strings.Split(t, "/") {
		if seg == ".." {
			return fmt.Errorf("%q mengandung segmen '..' (path traversal ditolak) (B-1)", target)
		}
	}
	return nil
}

// validFileMode menerima mode kanonik FileSpec. Kosong = render (default,
// ADR-003 D2). Merge BUKAN mode files[] (lewat contributes).
func validFileMode(mode string) bool {
	switch mode {
	case "", "render", "copy", "mkdir":
		return true
	default:
		return false
	}
}

// validateCatalog menegakkan ADR-003 D6 poin 3–6 atas himpunan manifest terdaftar.
//
// Catatan referensi dangling (D6 poin 3): `requires` yang merujuk modul tak
// terdaftar = ERROR (prasyarat mustahil terpenuhi). `conflicts` yang merujuk modul
// tak terdaftar = DIPERBOLEHKAN (modul absen tak mungkin ikut aktif, jadi tak
// berbahaya) — ini menampung katalog yang dibangun bertahap per-fase, di mana
// sebuah modul men-declare conflict ke modul yang belum diimplementasi.
func validateCatalog(fsys fs.FS, loaded map[string]Manifest) error {
	// Iterasi terurut by Name agar pesan error deterministik.
	names := make([]string, 0, len(loaded))
	for n := range loaded {
		names = append(names, n)
	}
	sort.Strings(names)

	// Set target file shared yang "dimiliki" katalog (punya skeleton via files[]).
	// Dipakai untuk validasi anchor↔skeleton level-katalog (D6 poin 5).
	// targetSkeleton memetakan target → (modul pemilik, path template skeleton)
	// agar verifikasi anchor↔skeleton (M-4) dapat membaca isi skeleton pemilik.
	ownedTargets := make(map[string]struct{})
	type skelRef struct {
		module   string
		template string // path .tmpl relatif dir modul pemilik
	}
	targetSkeleton := make(map[string]skelRef)
	for _, m := range loaded {
		for _, f := range m.Files {
			ownedTargets[f.Target] = struct{}{}
			if f.Mode != "mkdir" {
				// Pemilik pertama (deterministik: iterasi loaded tak terurut, tetapi
				// satu target hanya boleh dimiliki satu skeleton — bila ganda, owner
				// ditentukan resolver; untuk verifikasi anchor cukup salah satu yang
				// memuat anchor). Simpan bila belum ada.
				if _, exists := targetSkeleton[f.Target]; !exists {
					targetSkeleton[f.Target] = skelRef{module: m.Name, template: f.Template}
				}
			}
		}
	}

	for _, name := range names {
		m := loaded[name]

		// D6 poin 3 (requires) + poin 4 (konsistensi requires↔conflicts).
		conflictSet := make(map[string]struct{}, len(m.Conflicts))
		for _, c := range m.Conflicts {
			conflictSet[c] = struct{}{}
		}
		for _, req := range m.Requires {
			if _, ok := loaded[req]; !ok {
				return fmt.Errorf("module %q: requires %q tidak terdaftar di registry (dangling, ADR-003 D6.3)", name, req)
			}
			if _, bad := conflictSet[req]; bad {
				return fmt.Errorf("module %q: %q tercantum di requires DAN conflicts sekaligus (ADR-003 D6.4)", name, req)
			}
			if req == name {
				return fmt.Errorf("module %q: tidak boleh requires dirinya sendiri (ADR-003 D6.4)", name)
			}
		}
		for _, conf := range m.Conflicts {
			if conf == name {
				return fmt.Errorf("module %q: tidak boleh conflicts dirinya sendiri (ADR-003 D6.4)", name)
			}
			// conflicts dangling sengaja DIPERBOLEHKAN (lihat doc fungsi).
		}

		// D6 poin 6: path template files[] & contributes[] ada di subtree modul.
		modDir := path.Join(modulesRoot, name)
		for i, f := range m.Files {
			if f.Mode == "mkdir" {
				continue // mkdir tak punya template sumber
			}
			full := path.Join(modDir, f.Template)
			if _, err := fs.Stat(fsys, full); err != nil {
				return fmt.Errorf("module %q: files[%d].template %q tidak ada di embed.FS (%s) (ADR-003 D6.6)", name, i, f.Template, full)
			}
		}
		for i, c := range m.Contributes {
			full := path.Join(modDir, c.Fragment)
			if _, err := fs.Stat(fsys, full); err != nil {
				return fmt.Errorf("module %q: contributes[%d].fragment %q tidak ada di embed.FS (%s) (ADR-003 D6.6)", name, i, c.Fragment, full)
			}
			// D6 poin 5: target kontribusi harus dimiliki skeleton di katalog.
			// Modul lain (atau modul ini) wajib men-declare target itu via files[].
			if _, owned := ownedTargets[c.Target]; !owned {
				return fmt.Errorf("module %q: contributes[%d].target %q tak punya skeleton pemilik di katalog (tak ada files[] yang menghasilkan target itu) (ADR-003 D6.5)", name, i, c.Target)
			}
			// M-4: VERIFIKASI anchor↔skeleton di Load. Baca isi skeleton pemilik
			// target, lalu pastikan token anchor "region:<anchor>" ADA. Tanpa cek
			// ini, typo pada contributes[].anchor lolos Load dan baru gagal saat
			// Assemble (merge: anchor tidak ditemukan) — error tertunda jauh dari
			// sumbernya. Fail-fast di Load.
			if err := checkAnchorInSkeleton(fsys, targetSkeleton[c.Target].module, targetSkeleton[c.Target].template, c.Anchor); err != nil {
				return fmt.Errorf("module %q: contributes[%d] (target %q) %v", name, i, c.Target, err)
			}
		}
	}
	return nil
}

// checkAnchorInSkeleton membaca isi template skeleton pemilik target dan
// memverifikasi token anchor "region:<anchor>" muncul di dalamnya (M-4). Token
// dicari sebagai "region:<anchor>" diikuti batas (whitespace / akhir / non-ident)
// agar anchor "services" tidak salah-cocok dengan "services2". ownerModule &
// ownerTemplate berasal dari files[] pemilik target (skelRef).
func checkAnchorInSkeleton(fsys fs.FS, ownerModule, ownerTemplate, anchor string) error {
	if ownerModule == "" || ownerTemplate == "" {
		// Tak ada skeleton template (mis. owner mode mkdir) — tak bisa diverifikasi.
		// Kepemilikan target sendiri sudah dicek (D6.5); lewati cek anchor.
		return nil
	}
	full := path.Join(modulesRoot, ownerModule, ownerTemplate)
	raw, err := fs.ReadFile(fsys, full)
	if err != nil {
		return fmt.Errorf("tak dapat membaca skeleton pemilik %q: %w (M-4)", full, err)
	}
	if !skeletonHasAnchor(string(raw), anchor) {
		return fmt.Errorf("anchor %q tidak ada di skeleton pemilik %q (typo anchor?) (M-4)", anchor, full)
	}
	return nil
}

// skeletonHasAnchor melaporkan apakah konten skeleton memuat baris anchor
// "region:<anchor>" dengan <anchor> sebagai token utuh (bukan prefix nama anchor
// lain). Mengikuti semantik parseAnchorLine generator: nama = token setelah
// "region:" sampai whitespace pertama.
func skeletonHasAnchor(content, anchor string) bool {
	const token = "region:"
	search := 0
	for {
		idx := strings.Index(content[search:], token+anchor)
		if idx < 0 {
			return false
		}
		pos := search + idx + len(token) + len(anchor)
		// Karakter tepat setelah nama anchor harus batas (akhir, whitespace, atau
		// karakter non-identifier) agar "services" tak cocok "services2".
		if pos >= len(content) || isAnchorBoundary(content[pos]) {
			return true
		}
		search = search + idx + len(token)
	}
}

// isAnchorBoundary melaporkan apakah byte c adalah pembatas akhir nama anchor.
// Nama anchor terdiri dari huruf/digit/`-`/`_`; selain itu = batas.
func isAnchorBoundary(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return false
	case c == '-' || c == '_':
		return false
	default:
		return true
	}
}
