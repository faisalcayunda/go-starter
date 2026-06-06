// config.go — preset "--config <file.yaml>" (SPEC §5.4 meta-flag).
//
// Memuat seluruh jawaban dari sebuah file YAML menjadi preset, lalu memetakannya
// ke struct flag mentah (createFlags) SEBELUM nilai flag eksplisit di-overlay.
//
// PRESEDENSI (SPEC §5.4 / §6.4 rule 5): default < preset(--config) < flag eksplisit.
// Implementasi presedensi ada di create.go (applyPreset): preset hanya mengisi
// field yang TIDAK di-set eksplisit di CLI (dideteksi via cobra Flags().Changed).
// Di sini config.go murni: (1) muat & parse YAML, (2) petakan ke createFlags.
//
// Key YAML = id pertanyaan SPEC §4 (selaras §5.4): name, module, arch, kind, http,
// db, access, migrate, docker, makefile, golangci/lint, env/env-example, ci, obs,
// git. Subset Fase 4a; key di luar subset diabaikan diam-diam (forward-compatible).
//
// Mode --config bersifat non-interaktif (wizard di-skip) — SPEC §5.4.

package cli

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/faisalcayunda/gostarter/internal/answers"
)

// preset adalah representasi 1:1 file YAML --config. Setiap field memakai pointer
// agar dapat dibedakan antara "tidak ditulis di YAML" (nil) dan "ditulis bernilai
// zero" (mis. docker: false). Hanya field non-nil yang dianggap diset oleh preset
// dan berhak menimpa default (tetapi tetap kalah dari flag eksplisit).
//
// Skema contoh (key = id pertanyaan SPEC §4 / §5.4; subset Fase 4a):
//
//	name:      shop
//	module:    github.com/acme/shop
//	arch:      modular-monolith      # monolith | modular-monolith
//	kind:      rest
//	http:      chi                   # net/http | chi | echo
//	db:        postgres              # none | postgres | mysql
//	access:    sqlx
//	migrate:   golang-migrate
//	docker:    true
//	makefile:  true
//	golangci:  true                  # alias: lint
//	env:       true                  # alias: env-example
//	ci:        github-actions        # github-actions | gitlab-ci | none
//	obs:       true                  # observability (otel + /metrics + health)
//	git:       false
type preset struct {
	Name   *string `yaml:"name"`
	Module *string `yaml:"module"`
	Arch   *string `yaml:"arch"`
	Kind   *string `yaml:"kind"`
	HTTP   *string `yaml:"http"`

	DB      *string `yaml:"db"`
	Access  *string `yaml:"access"`
	Migrate *string `yaml:"migrate"`

	// Add-on boolean. golangci & lint adalah alias (golangci-lint); env &
	// env-example alias (.env.example). Bila keduanya hadir, key spesifik
	// (golangci/env) menang demi konsistensi dengan kanon flag --addons.
	Docker   *bool `yaml:"docker"`
	Makefile *bool `yaml:"makefile"`
	// Taskfile DEFERRED (Fase 4b): di-parse agar preset penuh SPEC §5.4 tidak
	// menelan key 'taskfile' secara diam-diam, TETAPI belum dipetakan ke flag/
	// Answers — add-on Taskfile belum termasuk subset Fase 4a (lihat SPEC §4.8:
	// taskfile tidak pre-checked, default false). mapPresetToFlags sengaja TIDAK
	// merujuk field ini sampai --taskfile + modul addon-taskfile hadir.
	Taskfile *bool `yaml:"taskfile"`
	Golangci *bool `yaml:"golangci"`
	Lint     *bool `yaml:"lint"`
	Env      *bool `yaml:"env"`
	EnvLong  *bool `yaml:"env-example"`
	Obs      *bool `yaml:"obs"`

	// CI provider enum (github-actions | gitlab-ci | none).
	CI *string `yaml:"ci"`

	Git *bool `yaml:"git"`
}

// loadPreset membaca & mem-parse file YAML --config menjadi preset.
//
// Error ramah & menyebut path bila file tak ada atau gagal parse (SPEC §5.4:
// non-interaktif, fail-fast pada input invalid). KnownFields(true) menolak key
// asing yang salah ketik agar preset tidak diam-diam terabaikan — kecuali key
// yang memang di luar subset Fase 4a, yang ditangani pemetaan (bukan parser).
func loadPreset(path string) (*preset, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file preset --config %q tidak ditemukan", path)
		}
		return nil, fmt.Errorf("membaca preset --config %q gagal: %w", path, err)
	}

	var p preset
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	// KnownFields(false): toleran terhadap key di luar subset Fase 4a (mis.
	// service/comm/broker/auth/config-loader/log) agar preset penuh SPEC §5.4
	// tetap dimuat tanpa error walau builder belum memakai semua key. (taskfile
	// kini punya field eksplisit — di-parse tapi DEFERRED, lihat struct preset.)
	dec.KnownFields(false)
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("mem-parse preset --config %q gagal (YAML tidak valid): %w", path, err)
	}
	return &p, nil
}

// mapPresetToFlags mengisi field createFlags dari preset HANYA untuk field yang
// TIDAK di-set eksplisit oleh user di CLI. Argumen `changed(name)` mengembalikan
// true bila flag bernama `name` di-set eksplisit (cobra Flags().Changed) — flag
// eksplisit selalu menang (SPEC §6.4 rule 5).
//
// Mengembalikan daftar add-on yang diaktifkan preset (sebagai keyword kanon
// --addons) agar digabung UNION dengan --addons/--feature di buildAnswers (M-2),
// menjaga byte-identical lintas jalur input.
func mapPresetToFlags(p *preset, f *createFlags, changed func(name string) bool) {
	if p == nil {
		return
	}

	// Skalar string: hormati flag eksplisit, selain itu ambil dari preset.
	if p.Name != nil && !changed("name") {
		f.name = *p.Name
	}
	if p.Module != nil && !changed("module") {
		f.module = *p.Module
	}
	if p.Arch != nil && !changed("arch") {
		f.arch = *p.Arch
	}
	if p.Kind != nil && !changed("kind") {
		f.kind = *p.Kind
	}
	if p.HTTP != nil && !changed("http") {
		f.http = *p.HTTP
	}
	if p.DB != nil && !changed("db") {
		f.db = *p.DB
	}
	if p.Access != nil && !changed("access") {
		f.access = *p.Access
	}
	if p.Migrate != nil && !changed("migrate") {
		f.migrate = *p.Migrate
	}
	if p.CI != nil && !changed("ci") {
		f.ci = *p.CI
	}

	// git: preset boolean → flag git/no-git. Hanya berlaku bila user tak
	// menyentuh --git maupun --no-git secara eksplisit.
	if p.Git != nil && !changed("git") && !changed("no-git") {
		f.git = *p.Git
		f.noGit = !*p.Git
	}

	// Add-on boolean preset → keyword kanon, dikumpulkan ke f.presetAddons.
	// Mereka digabung UNION dengan --addons/--feature (M-2) di buildAnswers,
	// jadi tak ada konsep "no-addon" di preset add-on (ketiadaan = tidak aktif).
	var addons []string
	addBool := func(p *bool, keyword string) {
		if p != nil && *p {
			addons = append(addons, keyword)
		}
	}
	addBool(p.Docker, "docker")
	addBool(p.Makefile, "makefile")
	// golangci & lint = alias.
	if (p.Golangci != nil && *p.Golangci) || (p.Lint != nil && *p.Lint) {
		addons = append(addons, "golangci")
	}
	// env & env-example = alias.
	if (p.Env != nil && *p.Env) || (p.EnvLong != nil && *p.EnvLong) {
		addons = append(addons, "env")
	}
	addBool(p.Obs, "observability")

	// CI: di preset, provider ditulis lewat key `ci` (string enum), bukan boolean
	// add-on. Pada jalur flag, add-on 'ci' diaktifkan eksplisit lewat --feature/
	// --addons (mis. `ci`), lalu provider diambil dari --ci. Agar preset
	// byte-identical dengan jalur flag, nilai provider non-kosong & non-`none`
	// HARUS turut mengaktifkan add-on 'ci' (resolveCI lalu memetakan f.ci →
	// provider). `ci: none`/kosong = tanpa CI (add-on tidak diaktifkan).
	if p.CI != nil {
		switch *p.CI {
		case "", string(answers.CINone):
			// tanpa CI — jangan aktifkan add-on.
		default:
			addons = append(addons, "ci")
		}
	}

	f.presetAddons = addons
}
