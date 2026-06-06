// Package generator — merge mengimplementasikan MergeAssembler: perakit file
// shared dari skeleton ber-anchor + fragmen terurut (ADR-002 §3.4/§6).
//
// Format anchor (kanonik, selaras skeleton template di templates/modules/**):
// satu baris komentar netral berbentuk "<prefix> region:<name>" dengan prefix
// komentar yang sah untuk sintaks file (`#` untuk YAML/Makefile/.env, `//` untuk
// Go, `--` untuk SQL). Indentasi baris marker dipertahankan saat penyisipan.
//
// Penanda anchor sengaja NETRAL (`region:`) — TIDAK menyebut nama builder —
// karena baris anchor yang tak terisi fragmen tetap muncul di output project
// (idempotensi add service). Zero lock-in (SPEC §8 N1): output tak boleh menyebut
// builder. (B-2: dulu penanda "gostarter:" bocor ke project hasil generate.)
package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/faisalcayunda/gostarter/internal/plan"
)

// anchorToken adalah penanda yang mengidentifikasi baris anchor pada skeleton.
// Baris anchor = (opsional indentasi) + prefix komentar + "region:" + nama.
// Penanda netral (bukan nama builder) — zero lock-in, B-2.
const anchorToken = "region:"

// MergeAssembler merakit satu file shared: untuk tiap anchor pada skeleton,
// menyisipkan Fragments terurut (by Order, tie-break stabil) → satu konten final
// tunggal. Dipakai hanya oleh FileOp ber-mode plan.ModeMerge. Untuk file .go,
// hasil akhir dilewatkan go/format oleh pemanggil (generator).
type MergeAssembler interface {
	Assemble(skeleton []byte, frags []plan.Fragment) ([]byte, error)
}

// mergeAssembler adalah implementasi default MergeAssembler. Stateless; aman
// dipakai ulang lintas FileOp.
type mergeAssembler struct{}

// NewMergeAssembler membuat MergeAssembler default (algoritma ADR-002 §6).
func NewMergeAssembler() MergeAssembler {
	return mergeAssembler{}
}

// Assemble menyisipkan frags ke skeleton pada baris-baris anchor bernama.
//
// Algoritma (ADR-002 §6):
//  1. Kelompokkan frags per Anchor; dalam tiap anchor sort STABIL by Order
//     (tie-break: urutan input — resolver sudah memfinalkan, assembler menegakkan).
//  2. Pindai skeleton baris demi baris; saat menemukan baris marker anchor
//     "<indent><prefix> region:<name>", ganti baris itu dengan konten fragmen
//     terurut untuk <name> (tanpa marker). Indentasi marker tidak diterapkan ulang
//     ke fragmen — fragmen membawa indentasinya sendiri (lihat catatan).
//  3. Anchor tanpa fragmen → baris marker DIPERTAHANKAN apa adanya (idempotensi
//     `add service`: marker tetap ada untuk penyisipan inkremental kelak, ADR-003 D5).
//  4. Baris non-anchor disalin apa adanya.
//
// Idempoten: skeleton + frags yang sama selalu menghasilkan byte yang sama;
// menjalankan ulang atas hasil (yang markernya sudah hilang untuk anchor terisi)
// tidak menggandakan fragmen.
//
// Catatan indentasi: fragmen pada templates/modules/** sudah membawa indentasi
// natural-nya (mis. fragmen compose service diawali dua spasi). Marker anchor
// pada skeleton berada pada indentasi yang sama, sehingga mengganti baris marker
// dengan fragmen menjaga struktur. Bila sebuah anchor punya >1 fragmen, fragmen
// digabung dengan newline tunggal sesuai urutan.
func (mergeAssembler) Assemble(skeleton []byte, frags []plan.Fragment) ([]byte, error) {
	// 1. Kelompokkan + urutkan fragmen per anchor.
	byAnchor, err := groupFragments(frags)
	if err != nil {
		return nil, err
	}

	// Lacak anchor yang benar-benar dijumpai di skeleton agar fragmen "yatim"
	// (anchor tak ada di skeleton) terdeteksi sebagai error — mencegah fragmen
	// hilang diam-diam (cermin validasi anchor↔skeleton ADR-003 D6).
	seen := make(map[string]bool, len(byAnchor))

	// 2. Pindai skeleton baris demi baris.
	//
	// Skeleton dipecah memakai pemisah "\n"; akhiran newline (bila ada)
	// direkonstruksi agar byte akhir stabil terhadap input.
	lines := strings.Split(string(skeleton), "\n")
	var out []string
	for _, line := range lines {
		name, keep, ok := parseAnchorLine(line)
		if !ok {
			out = append(out, line)
			continue
		}
		seen[name] = true
		contents, has := byAnchor[name]
		if !has || len(contents) == 0 {
			// Anchor tanpa fragmen → pertahankan marker (idempotensi add service).
			out = append(out, line)
			continue
		}
		// Sisipkan fragmen terurut. Default: marker DIHAPUS (anchor terisi). Bila
		// marker ber-flag "keep" (mis. "# region:services keep"), marker DIPERTAHANKAN
		// SETELAH fragmen — agar penyisipan inkremental berikutnya (`add service`)
		// tetap menemukan titik-sisip yang sama (idempotensi multi-add, M-2). Flag
		// eksplisit & deterministik; anchor tanpa flag (monolith) perilaku lama.
		out = append(out, contents...)
		if keep {
			out = append(out, line)
		}
	}

	// 3. Setiap fragmen harus punya anchor di skeleton; bila tidak → error.
	for anchor := range byAnchor {
		if !seen[anchor] {
			return nil, fmt.Errorf("merge: anchor %q tidak ditemukan di skeleton", anchor)
		}
	}

	return []byte(strings.Join(out, "\n")), nil
}

// groupFragments mengelompokkan fragmen per Anchor dan mengurutkannya STABIL by
// Order (tie-break: urutan input). Mengembalikan map anchor → daftar konten siap
// sisip (Content per fragmen, tanpa marker).
func groupFragments(frags []plan.Fragment) (map[string][]string, error) {
	// Pertahankan urutan input untuk tie-break stabil: rekam indeks asli.
	type indexed struct {
		frag plan.Fragment
		idx  int
	}
	grouped := make(map[string][]indexed)
	for i, f := range frags {
		if f.Anchor == "" {
			return nil, fmt.Errorf("merge: fragmen ke-%d tidak punya Anchor", i)
		}
		grouped[f.Anchor] = append(grouped[f.Anchor], indexed{frag: f, idx: i})
	}

	out := make(map[string][]string, len(grouped))
	for anchor, items := range grouped {
		// Sort stabil by Order; tie-break by indeks input (urutan resolver).
		sort.SliceStable(items, func(a, b int) bool {
			if items[a].frag.Order != items[b].frag.Order {
				return items[a].frag.Order < items[b].frag.Order
			}
			return items[a].idx < items[b].idx
		})
		contents := make([]string, len(items))
		for i, it := range items {
			contents[i] = it.frag.Content
		}
		out[anchor] = contents
	}
	return out, nil
}

// parseAnchorLine memeriksa apakah line adalah baris marker anchor dan, bila ya,
// mengembalikan (nama anchor, keep flag). Bentuk yang dikenali (indentasi opsional):
//
//	<indent># region:<name>[ keterangan…]
//	<indent># region:<name> keep[ keterangan…]
//	<indent>// region:<name>[ keterangan…]
//	<indent>-- region:<name>[ keterangan…]
//
// Nama anchor = token setelah "region:" sampai whitespace pertama (atau akhir
// baris). keep=true bila TOKEN BERIKUTNYA setelah nama adalah persis "keep" —
// menandai marker yang harus DIPERTAHANKAN setelah anchor terisi (idempotensi
// `add service`, M-2). Keterangan lain setelah nama (mis. "(db-postgres) — …")
// diabaikan. Baris tanpa "region:" → bukan anchor.
func parseAnchorLine(line string) (string, bool, bool) {
	trimmed := strings.TrimSpace(line)
	// Harus diawali salah satu prefix komentar yang dikenal.
	prefix := commentPrefix(trimmed)
	if prefix == "" {
		return "", false, false
	}
	rest := strings.TrimSpace(trimmed[len(prefix):])
	if !strings.HasPrefix(rest, anchorToken) {
		return "", false, false
	}
	after := rest[len(anchorToken):]
	// Nama = token sampai whitespace pertama; sisanya = flag/keterangan.
	name := after
	tail := ""
	if i := strings.IndexFunc(after, func(r rune) bool {
		return r == ' ' || r == '\t'
	}); i >= 0 {
		name = after[:i]
		tail = strings.TrimSpace(after[i:])
	}
	if name == "" {
		return "", false, false
	}
	// keep flag: token pertama pada tail persis "keep".
	keep := false
	if tail != "" {
		flag := tail
		if j := strings.IndexFunc(tail, func(r rune) bool {
			return r == ' ' || r == '\t'
		}); j >= 0 {
			flag = tail[:j]
		}
		keep = flag == "keep"
	}
	return name, keep, true
}

// commentPrefix mengembalikan prefix komentar di awal s ("//", "--", atau "#"),
// atau "" bila bukan komentar yang dikenali. Urutan cek penting: "//" sebelum
// kemungkinan lain.
func commentPrefix(s string) string {
	switch {
	case strings.HasPrefix(s, "//"):
		return "//"
	case strings.HasPrefix(s, "--"):
		return "--"
	case strings.HasPrefix(s, "#"):
		return "#"
	default:
		return ""
	}
}
