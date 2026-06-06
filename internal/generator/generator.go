// Package generator mengeksekusi plan.GeneratePlan: me-render template dari
// embed.FS, menggabungkan kontribusi file shared, lalu menulis hasilnya lewat
// fsutil.Writer (real atau dry-run).
//
// File ini mengimplementasikan kontrak Generator.Generate sesuai ADR-002 §6
// (model eksekusi GeneratePlan + perakitan go.mod via x/mod/modfile). Interface
// Renderer dan MergeAssembler (renderer.go/merge.go) dikonsumsi dari sini; file
// tersebut TIDAK disentuh.
package generator

import (
	"fmt"
	"go/format"
	"io/fs"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/faisalcayunda/gostarter/internal/fsutil"
	"github.com/faisalcayunda/gostarter/internal/modpath"
	"github.com/faisalcayunda/gostarter/internal/module"
	"github.com/faisalcayunda/gostarter/internal/plan"
)

// Generator mengeksekusi sebuah GeneratePlan ke direktori target memakai Writer.
// target adalah path root project hasil generate; w menentukan apakah penulisan
// nyata (RealWriter) atau preview (DryRunWriter).
type Generator interface {
	Generate(p plan.GeneratePlan, target string, w fsutil.Writer) error
	// GenerateFiles mengeksekusi HANYA p.Files ke target — TANPA EnsureEmptyDir &
	// TANPA merakit go.mod. Dipakai jalur inkremental `add service` (US-05): project
	// existing tak boleh dikosongkan & go.mod-nya dipertahankan apa adanya. Penulisan
	// tetap melewati containment fsutil.JoinTarget (H-1) seperti Generate.
	GenerateFiles(p plan.GeneratePlan, target string, w fsutil.Writer) error
}

// generator adalah implementasi konkret Generator (ADR-002 §3.4). Ia merakit
// GeneratePlan menjadi file di disk (atau preview dry-run) secara deterministik:
// render via Renderer, merge file shared via MergeAssembler, dan rakit go.mod via
// modfile — semuanya menghasilkan byte yang stabil (SPEC §5.2).
type generator struct {
	reg module.Registry // katalog manifest modul (sumber metadata; tervalidasi saat Load)
	r   Renderer        // render template bernama → byte (text/template + FuncMap)
	m   MergeAssembler  // rakit skeleton + fragmen terurut → satu byte
}

// New mengonstruksi Generator dari registry modul, renderer, dan assembler merge.
// Signature ini KANONIK (ADR-002 §3.4 / §6) dan tidak boleh berubah: ketiga
// dependensi diisolasi di balik interface agar generator dapat diuji murni
// in-memory (golden-file, ADR-002 §9) tanpa menyentuh disk/toolchain.
func New(reg module.Registry, r Renderer, m MergeAssembler) Generator {
	return &generator{reg: reg, r: r, m: m}
}

// Generate mengeksekusi GeneratePlan ke direktori target memakai Writer.
//
// Alur (ADR-002 §6 + §8 safety flow):
//  1. Untuk RealWriter: panggil EnsureEmptyDir(target) lebih dulu (proteksi
//     overwrite, US-06 Sk.3). DryRunWriter melewati cek ini (tak ada tulis).
//  2. Untuk tiap FileOp pada urutan plan (sudah distabilkan resolver):
//     ModeMkdir  → w.Mkdir
//     ModeCopy   → Renderer.Raw (byte mentah, bypass template engine) → w.WriteFile
//     ModeRender → Renderer.Render → (go/format bila .go) → w.WriteFile
//     ModeMerge  → render skeleton → MergeAssembler.Assemble → (go/format bila
//     .go) → w.WriteFile
//  3. Rakit go.mod dari p.Deps via modfile.Format (BUKAN merge teks) → tulis.
//
// Hook pasca-generate (gofmt/go mod tidy/git init, §7) berada di luar Generate —
// orchestrator yang menjalankannya untuk RealWriter setelah Generate sukses.
func (g *generator) Generate(p plan.GeneratePlan, target string, w fsutil.Writer) error {
	// (1) Proteksi overwrite hanya untuk penulisan nyata. Deteksi DryRunWriter via
	// type-assert pointer (ADR-002 §3.5: DryRunWriter wajib pointer-receiver).
	_, isDry := w.(*fsutil.DryRunWriter)
	if !isDry {
		if err := fsutil.EnsureEmptyDir(target); err != nil {
			return fmt.Errorf("generate: target tidak siap: %w", err)
		}
	}

	// (2) Eksekusi setiap FileOp berurutan sesuai urutan dalam plan. Urutan sudah
	// final & deterministik dari resolver (sort by TargetPath; parent mkdir lebih
	// dulu) → output byte-identical antar mode (SPEC §5.2).
	for _, op := range p.Files {
		if err := g.execFileOp(op, target, w); err != nil {
			return err
		}
	}

	// (3) go.mod dirakit terpisah dari p.Deps via modfile (ADR-002 §6: BUKAN
	// ModeMerge / fragment teks). Selalu ditulis — setiap project punya go.mod.
	if err := g.writeGoMod(p, target, w); err != nil {
		return err
	}

	return nil
}

// GenerateFiles mengeksekusi HANYA p.Files ke target (urutan plan) TANPA
// EnsureEmptyDir & TANPA go.mod (jalur inkremental `add service`, US-05). Tiap
// FileOp diproses identik dengan Generate (render/copy/mkdir/merge) — termasuk
// containment fsutil.JoinTarget (H-1) di execFileOp. Cocok untuk DryRunWriter
// (preview) maupun RealWriter (tulis nyata ke project existing).
func (g *generator) GenerateFiles(p plan.GeneratePlan, target string, w fsutil.Writer) error {
	for _, op := range p.Files {
		if err := g.execFileOp(op, target, w); err != nil {
			return err
		}
	}
	return nil
}

// execFileOp memproses satu FileOp sesuai Mode-nya (ADR-002 §6, tabel mode).
func (g *generator) execFileOp(op plan.FileOp, target string, w fsutil.Writer) error {
	dst, err := fsutil.JoinTarget(target, op.TargetPath)
	if err != nil {
		return fmt.Errorf("generate: %q: %w", op.TargetPath, err)
	}

	switch op.Mode {
	case plan.ModeMkdir:
		// Idempoten (mkdir-all di fsutil). Tidak ada konten.
		if err := w.Mkdir(dst); err != nil {
			return fmt.Errorf("generate: mkdir %q: %w", op.TargetPath, err)
		}
		return nil

	case plan.ModeCopy:
		// Salin byte MENTAH dari embed.FS, bypass total template engine (m-4).
		// Untuk aset statik (mis. migrasi .sql) yang BOLEH memuat literal "{{" —
		// melewatkannya ke text/template (seperti dulu via Render) akan gagal parse.
		// Renderer.Raw membaca verbatim dari fsys.
		data, err := g.r.Raw(op.TemplatePath)
		if err != nil {
			return fmt.Errorf("generate: copy %q (baca %q): %w", op.TargetPath, op.TemplatePath, err)
		}
		if err := w.WriteFile(dst, data, perm(op)); err != nil {
			return fmt.Errorf("generate: tulis %q: %w", op.TargetPath, err)
		}
		return nil

	case plan.ModeRender:
		data, err := g.r.Render(op.TemplatePath, mergeRenderData(op.Data, op.DataOverride))
		if err != nil {
			return fmt.Errorf("generate: render %q (template %q): %w", op.TargetPath, op.TemplatePath, err)
		}
		out, err := formatIfGo(op.TargetPath, data)
		if err != nil {
			return fmt.Errorf("generate: format %q: %w", op.TargetPath, err)
		}
		if err := w.WriteFile(dst, out, perm(op)); err != nil {
			return fmt.Errorf("generate: tulis %q: %w", op.TargetPath, err)
		}
		return nil

	case plan.ModeMerge:
		// Data render dasar = global op.Data di-merge dengan op.DataOverride (override
		// menang). Dipakai untuk skeleton & sebagai dasar tiap fragmen.
		baseData := mergeRenderData(op.Data, op.DataOverride)
		// Render skeleton ber-anchor lebih dulu, lalu rakit dengan fragmen
		// terurut (Fragments sudah final/terurut dari resolver; assembler hanya
		// menegakkan urutan).
		skeleton, err := g.r.Render(op.TemplatePath, baseData)
		if err != nil {
			return fmt.Errorf("generate: render skeleton %q (template %q): %w", op.TargetPath, op.TemplatePath, err)
		}
		// Render tiap fragmen: resolver menaruh PATH fragmen di Fragment.Content
		// (render ditunda ke sini — ADR-002 §3.3/§6). Render path → konten siap
		// sisip dengan data context yang sama (proyeksi Answers + Vars), plus
		// Fragment.DataOverride per-fragmen bila ada (mis. ServiceName per service).
		frags, err := g.renderFragments(op.Fragments, baseData)
		if err != nil {
			return fmt.Errorf("generate: render fragmen untuk %q: %w", op.TargetPath, err)
		}
		merged, err := g.m.Assemble(skeleton, frags)
		if err != nil {
			return fmt.Errorf("generate: assemble %q: %w", op.TargetPath, err)
		}
		out, err := formatIfGo(op.TargetPath, merged)
		if err != nil {
			return fmt.Errorf("generate: format %q: %w", op.TargetPath, err)
		}
		if err := w.WriteFile(dst, out, perm(op)); err != nil {
			return fmt.Errorf("generate: tulis %q: %w", op.TargetPath, err)
		}
		return nil

	default:
		return fmt.Errorf("generate: FileOp.Mode tidak dikenal (%d) untuk %q", op.Mode, op.TargetPath)
	}
}

// renderFragments merender tiap fragmen merge: resolver menaruh PATH template
// fragmen di Fragment.Content (render ditunda ke generator — ADR-002 §3.3/§6).
// Fungsi ini membaca tiap path dari embed.FS via Renderer, mengeksekusinya dengan
// data context yang sama (proyeksi Answers + Vars), lalu mengembalikan salinan
// Fragment dengan Content = konten terender (Anchor & Order dipertahankan).
//
// Trailing newline tunggal dilucuti agar assembler — yang menyatukan fragmen &
// baris skeleton dengan "\n" — tidak menyisipkan baris kosong ekstra.
func (g *generator) renderFragments(frags []plan.Fragment, data any) ([]plan.Fragment, error) {
	if len(frags) == 0 {
		return frags, nil
	}
	out := make([]plan.Fragment, len(frags))
	for i, fr := range frags {
		// Fragment.DataOverride (bila ada) di-merge di atas data induk → mendukung
		// fragmen per-service dari template sama dengan nilai berbeda.
		fragData := mergeRenderData(data, fr.DataOverride)
		rendered, err := g.r.Render(fr.Content, fragData)
		if err != nil {
			return nil, fmt.Errorf("fragmen %q (anchor %q): %w", fr.Content, fr.Anchor, err)
		}
		out[i] = plan.Fragment{
			Anchor:  fr.Anchor,
			Content: strings.TrimRight(string(rendered), "\n"),
			Order:   fr.Order,
		}
	}
	return out, nil
}

// mergeRenderData menggabungkan data render dasar dengan override per-op/per-fragmen.
//
// Aturan:
//   - override nil/kosong → kembalikan base apa adanya (zero-alloc; jalur lama,
//     menjaga byte-identical untuk FileOp tanpa override).
//   - base map[string]any → salinan dangkal base lalu timpa dengan override
//     (override MENANG per-key). Base TIDAK dimutasi (data global dibagi lintas
//     FileOp; mutasi akan membocorkan nilai antar-service → memecah determinisme).
//   - base bertipe lain (non-map) namun override ada → kembalikan override saja
//     (jalur defensif; resolver selalu memberi map[string]any sebagai Data).
//
// Determinisme: hasil hanya bergantung pada isi key, bukan urutan iterasi map —
// output byte-identical untuk Answers + override yang sama (SPEC §5.2).
func mergeRenderData(base any, override map[string]any) any {
	if len(override) == 0 {
		return base
	}
	bm, ok := base.(map[string]any)
	if !ok {
		// Base bukan map (mis. nil atau struct): override berdiri sendiri.
		return override
	}
	merged := make(map[string]any, len(bm)+len(override))
	for k, v := range bm {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}

// writeGoMod merakit go.mod project hasil generate dari plan via x/mod/modfile,
// BUKAN sebagai fragment teks (ADR-002 §6 "Perakitan go.mod"). p.Deps diasumsikan
// sudah dedup+sort oleh resolver; SortBlocks() + Cleanup() menormalisasi ulang
// agar output byte-stabil tanpa bergantung urutan input (SPEC §5.2 poin 3).
func (g *generator) writeGoMod(p plan.GeneratePlan, target string, w fsutil.Writer) error {
	f := new(modfile.File)
	if err := f.AddModuleStmt(p.ModulePath); err != nil {
		return fmt.Errorf("generate: go.mod module %q: %w", p.ModulePath, err)
	}
	if err := f.AddGoStmt(p.GoVersion); err != nil {
		return fmt.Errorf("generate: go.mod go directive %q: %w", p.GoVersion, err)
	}

	// Defensif: jaga determinisme walau resolver lalai meng-sort/dedup. Salin lalu
	// urutkan stabil by Path; ambil versi terakhir bila path ganda.
	deps := dedupSortDeps(p.Deps)
	for _, d := range deps {
		if err := f.AddRequire(d.Path, d.Version); err != nil {
			return fmt.Errorf("generate: go.mod require %s %s: %w", d.Path, d.Version, err)
		}
	}

	f.SortBlocks()
	f.Cleanup()

	out, err := f.Format()
	if err != nil {
		return fmt.Errorf("generate: format go.mod: %w", err)
	}
	dst, err := fsutil.JoinTarget(target, "go.mod")
	if err != nil {
		return fmt.Errorf("generate: go.mod: %w", err)
	}
	if err := w.WriteFile(dst, out, 0o644); err != nil {
		return fmt.Errorf("generate: tulis go.mod: %w", err)
	}
	return nil
}

// dedupSortDeps mengembalikan salinan deps yang ter-dedup by Path (HIGHEST
// VERSION WINS bila path ganda — kebijakan kanonik ADR-002 §6) dan terurut stabil
// by Path. Memakai modpath.HigherVersion agar IDENTIK dengan kebijakan dedup di
// resolver.collectDeps (satu sumber kebenaran → tidak ada divergensi yang
// memecah byte-identical, SPEC §5.2).
func dedupSortDeps(in []plan.ModuleDep) []plan.ModuleDep {
	if len(in) == 0 {
		return nil
	}
	byPath := make(map[string]string, len(in))
	order := make([]string, 0, len(in))
	for _, d := range in {
		cur, seen := byPath[d.Path]
		if !seen {
			order = append(order, d.Path)
			byPath[d.Path] = d.Version
			continue
		}
		byPath[d.Path] = modpath.HigherVersion(cur, d.Version)
	}
	sort.Strings(order)
	out := make([]plan.ModuleDep, 0, len(order))
	for _, p := range order {
		out = append(out, plan.ModuleDep{Path: p, Version: byPath[p]})
	}
	return out
}

// formatIfGo melewatkan data ke go/format bila target berekstensi .go (ADR-002 §6:
// gagal format = error, bukan tulis mentah). File non-.go diteruskan apa adanya.
func formatIfGo(targetPath string, data []byte) ([]byte, error) {
	if !strings.HasSuffix(targetPath, ".go") {
		return data, nil
	}
	out, err := format.Source(data)
	if err != nil {
		return nil, fmt.Errorf("go/format: %w", err)
	}
	return out, nil
}

// perm mengembalikan permission FileOp; default 0o644 untuk file bila tak diset
// (FileMode 0). Direktori memakai jalur Mkdir terpisah, jadi helper ini hanya
// untuk WriteFile.
func perm(op plan.FileOp) fs.FileMode {
	if op.Perm == 0 {
		return 0o644
	}
	return op.Perm
}
