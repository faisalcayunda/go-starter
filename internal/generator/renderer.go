// Package generator — renderer membungkus text/template + FuncMap helper.
//
// Renderer me-load template bernama dari embed.FS (templates.FS), mengeksekusinya
// dengan data context, lalu — untuk target .go — melewatkan hasilnya ke go/format
// agar gofmt-valid & deterministik. FuncMap menyediakan enam helper kanonik
// (ADR-001 §3, ADR-002 §3.4/§4).
package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"path"
	"strings"
	"text/template"
	"unicode"

	"github.com/faisalcayunda/gostarter/internal/modpath"
)

// Renderer me-render satu template bernama dengan data context menjadi byte.
// Output file .go dilewatkan ke go/format oleh pemanggil (generator) agar
// deterministik.
type Renderer interface {
	// Render mengeksekusi template (text/template + FuncMap) atas data.
	Render(name string, data any) ([]byte, error)
	// Raw membaca byte MENTAH file dari fsys TANPA eksekusi template engine.
	// Dipakai ModeCopy untuk aset statik (mis. .sql) yang boleh memuat "{{"
	// literal — yang akan gagal bila dilewatkan parser text/template (m-4).
	Raw(name string) ([]byte, error)
}

// fsRenderer adalah implementasi Renderer berbasis text/template di atas sebuah
// fs.FS (lazimnya templates.FS). Ia memuat tepat satu template by name, menjalankan
// FuncMap kanonik, dan — bila name menargetkan file .go — menormalkan hasil via
// go/format. Stateless: aman dipakai ulang lintas FileOp.
type fsRenderer struct {
	fsys fs.FS
}

// NewRenderer membuat Renderer yang membaca template dari fsys (mis. templates.FS).
// Pemanggil meneruskan path .tmpl relatif terhadap akar fsys ke Render.
func NewRenderer(fsys fs.FS) Renderer {
	return &fsRenderer{fsys: fsys}
}

// Render memuat template di path name dari fsys, mengeksekusinya dengan data,
// dan mengembalikan byte hasil. Bila name (setelah dilucuti suffix .tmpl)
// berekstensi .go, hasil dilewatkan go/format.Source — gagal format = error
// (tidak menulis Go yang tidak valid). Non-.go dikembalikan apa adanya.
func (r *fsRenderer) Render(name string, data any) ([]byte, error) {
	raw, err := fs.ReadFile(r.fsys, name)
	if err != nil {
		return nil, fmt.Errorf("renderer: baca template %q: %w", name, err)
	}

	// Nama template di-set ke basename agar pesan error template informatif.
	tmpl, err := template.New(path.Base(name)).Funcs(FuncMap()).Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("renderer: parse template %q: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("renderer: eksekusi template %q: %w", name, err)
	}

	// Hanya file .go yang dinormalkan gofmt; aset lain (.yml, .env, .sql, Makefile)
	// dikembalikan apa adanya. targetExt melucuti suffix .tmpl lebih dulu.
	if targetIsGo(name) {
		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("renderer: gofmt hasil render %q: %w", name, err)
		}
		return formatted, nil
	}
	return buf.Bytes(), nil
}

// Raw membaca byte mentah file di path name dari fsys, TANPA mengeksekusi
// template engine dan tanpa gofmt. Dipakai generator untuk ModeCopy (m-4): aset
// statik (mis. migrasi .sql) yang boleh memuat literal "{{" disalin verbatim —
// melewatkannya ke text/template akan gagal parse. Byte dikembalikan apa adanya.
func (r *fsRenderer) Raw(name string) ([]byte, error) {
	raw, err := fs.ReadFile(r.fsys, name)
	if err != nil {
		return nil, fmt.Errorf("renderer: baca mentah %q: %w", name, err)
	}
	return raw, nil
}

// targetIsGo melaporkan apakah name (path template) menargetkan file Go. Suffix
// .tmpl dilucuti dulu sehingga "router.go.tmpl" → ".go".
func targetIsGo(name string) bool {
	base := strings.TrimSuffix(name, ".tmpl")
	return strings.HasSuffix(base, ".go")
}

// FuncMap mengembalikan helper template builder (ADR-001 §3; ADR-002 §4). TEPAT
// enam fungsi kanonik, masing-masing di-unit-test sendiri (ADR-002 §9):
//
//	toCamel  — normalisasi token lalu gabung camelCase (token pertama huruf-kecil)
//	toPascal — seperti toCamel tetapi token pertama juga di-Pascal-kan (identifier exported)
//	toSnake  — normalisasi token lalu gabung snake_case (sisip _ pada batas case)
//	toScreamingSnake — seperti toSnake tetapi HURUF-BESAR (nama environment variable)
//	toKebab  — seperti toSnake tetapi pemisah - (nama service/image Docker)
//	modBase  — ambil segmen terakhir module path (strip suffix versi mayor /vN)
//	modJoin  — gabung module path + elemen path import dengan / (path.Join-style, slash-only)
//
// Normalisasi token dipusatkan di helper internal splitTokens(s) agar
// toCamel/toPascal/toSnake/toScreamingSnake/toKebab konsisten; modBase/modJoin
// murni manipulasi path (validasi module path ada di answers.Validate, bukan di sini).
func FuncMap() map[string]any {
	return map[string]any{
		"toCamel":          toCamel,
		"toPascal":         toPascal,
		"toSnake":          toSnake,
		"toScreamingSnake": toScreamingSnake,
		"toKebab":          toKebab,
		"modBase":          modBase,
		"modJoin":          modJoin,
	}
}

// splitTokens memecah s menjadi token kata atas pemisah eksplisit (`_`, `-`,
// spasi) DAN batas case (transisi lower→Upper, serta akhir run akronim seperti
// "HTTPServer" → ["HTTP","Server"]). Karakter non-alnum di luar pemisah dibuang.
// Token dikembalikan apa adanya (tanpa lower/upper) agar pemanggil yang memutuskan
// casing. Deterministik & tanpa locale (ADR-002 §4).
func splitTokens(s string) []string {
	// Tahap 1: pisah pada pemisah eksplisit & buang karakter non-alnum lain.
	// Setiap rune non-alnum berlaku sebagai pembatas token.
	var rawWords []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			rawWords = append(rawWords, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			cur.WriteRune(r)
		default:
			// `_`, `-`, spasi, dan simbol lain → pembatas; karakter dibuang.
			flush()
		}
	}
	flush()

	// Tahap 2: pecah tiap raw-word pada batas case internal.
	var tokens []string
	for _, w := range rawWords {
		tokens = append(tokens, splitCaseBoundaries(w)...)
	}
	return tokens
}

// splitCaseBoundaries memecah satu kata (sudah bebas pemisah) pada batas case:
//   - transisi lower/digit → Upper memulai token baru ("userName" → user|Name)
//   - akhir run huruf-besar yang diikuti lower memulai token baru pada huruf
//     besar terakhir ("HTTPServer" → HTTP|Server)
func splitCaseBoundaries(w string) []string {
	rs := []rune(w)
	if len(rs) == 0 {
		return nil
	}
	var out []string
	start := 0
	for i := 1; i < len(rs); i++ {
		prev, curr := rs[i-1], rs[i]
		boundary := false
		switch {
		case !unicode.IsUpper(prev) && unicode.IsUpper(curr):
			// Transisi lower/digit → Upper: "userName" → pecah sebelum N.
			boundary = true
		case unicode.IsUpper(prev) && unicode.IsUpper(curr) &&
			i+1 < len(rs) && isLowerLetter(rs[i+1]):
			// Akhir run akronim diikuti kata huruf-kecil: "HTTPServer" → pecah
			// sebelum S (prev=P, curr=S, next=e). Token akronim "HTTP" terpotong rapi.
			boundary = true
		}
		if boundary {
			out = append(out, string(rs[start:i]))
			start = i
		}
	}
	out = append(out, string(rs[start:]))
	return out
}

// toCamel menormalkan token lalu menggabung camelCase: token pertama huruf-kecil
// seluruhnya, token berikut di-Pascal-kan. Untuk nama variabel Go.
//
//	toCamel("user name") → "userName" ; toCamel("user_name") → "userName"
func toCamel(s string) string {
	tokens := splitTokens(s)
	if len(tokens) == 0 {
		return ""
	}
	var b strings.Builder
	for i, t := range tokens {
		if i == 0 {
			b.WriteString(strings.ToLower(t))
			continue
		}
		b.WriteString(pascalWord(t))
	}
	return b.String()
}

// toPascal seperti toCamel tetapi token pertama juga di-Pascal-kan. Untuk nama
// tipe / identifier exported.
//
//	toPascal("user name") → "UserName"
func toPascal(s string) string {
	tokens := splitTokens(s)
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString(pascalWord(t))
	}
	return b.String()
}

// toSnake menormalkan token lalu menggabung snake_case huruf-kecil. Batas case
// dari input camel/Pascal menjadi pemisah `_`.
//
//	toSnake("UserName") → "user_name"
func toSnake(s string) string {
	return joinLower(splitTokens(s), "_")
}

// toScreamingSnake menormalkan token lalu menggabung SCREAMING_SNAKE_CASE
// (huruf-besar, pemisah `_`). Untuk nama environment variable yang mengikuti
// konvensi universal POSIX (mis. override alamat downstream gRPC).
//
//	toScreamingSnake("svc-b") → "SVC_B" ; toScreamingSnake("UserName") → "USER_NAME"
func toScreamingSnake(s string) string {
	return strings.ToUpper(joinLower(splitTokens(s), "_"))
}

// toKebab seperti toSnake tetapi pemisah `-`. Untuk nama service / image Docker.
//
//	toKebab("UserName") → "user-name"
func toKebab(s string) string {
	return joinLower(splitTokens(s), "-")
}

// joinLower menggabung token (di-lower-kan) dengan separator.
func joinLower(tokens []string, sep string) string {
	lowered := make([]string, len(tokens))
	for i, t := range tokens {
		lowered[i] = strings.ToLower(t)
	}
	return strings.Join(lowered, sep)
}

// isLowerLetter melaporkan apakah r adalah huruf non-uppercase (huruf kecil).
// Dipakai untuk mendeteksi akhir run akronim pada splitCaseBoundaries.
func isLowerLetter(r rune) bool {
	return unicode.IsLetter(r) && !unicode.IsUpper(r)
}

// pascalWord meng-uppercase huruf pertama satu token & melower sisanya
// ("HTTP" → "Http", "name" → "Name"). Token sudah bebas pemisah.
func pascalWord(t string) string {
	if t == "" {
		return ""
	}
	rs := []rune(strings.ToLower(t))
	rs[0] = unicode.ToUpper(rs[0])
	return string(rs)
}

// modBase mengembalikan segmen terakhir module path (base), melucuti suffix versi
// mayor "/vN" (N ≥ 2). Delegasi ke internal/modpath (satu sumber kebenaran;
// dipakai renderer & resolver) — ASCII-only sesuai semantik module path Go.
//
//	modBase("github.com/acme/shop")    → "shop"
//	modBase("github.com/acme/shop/v2") → "shop"
func modBase(modulePath string) string {
	return modpath.Base(modulePath)
}

// modJoin menggabung module path dengan elemen path import memakai `/`
// (slash-only POSIX — SPEC §2.1). Delegasi ke internal/modpath.
//
//	modJoin("github.com/acme/fleet", "services", "user") → "github.com/acme/fleet/services/user"
func modJoin(modulePath string, elem ...string) string {
	return modpath.Join(modulePath, elem...)
}
