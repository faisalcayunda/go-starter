// when.go mengimplementasikan evaluator ekspresi `when` (ADR-002 §5).
//
// `when` adalah mini-bahasa kondisi fragment/file-level bergaya text/template
// yang dievaluasi DI RESOLVER (bukan di dalam .tmpl) sehingga GeneratePlan final
// sebelum render — syarat dry-run akurat & golden-file stabil (ADR-003 D4).
//
// Grammar (EBNF, ADR-002 §5.1):
//
//	when      = expr ;
//	expr      = orExpr ;
//	orExpr    = andExpr , { "or" , andExpr } ;
//	andExpr   = unary  , { "and" , unary } ;
//	unary     = [ "not" ] , atom ;
//	atom      = boolField | "(" , expr , ")" | call ;
//	call      = func , SP , arg , SP , arg ;   (* fungsi biner: eq / ne *)
//	func      = "eq" | "ne" ;
//	arg       = field | string | number ;
//	field     = "." , ident , { "." , ident } ;
//
// Presedensi: not (terkuat) > and > or (terlemah); kurung override.
// Ekspresi kosong = selalu true. Field di luar daftar legal → error (fail-fast).

package resolver

import (
	"fmt"
	"strings"

	"github.com/faisalcayunda/gostarter/internal/answers"
)

// evalWhen mem-parse lalu mengevaluasi ekspresi `when` atas Answers final.
// Ekspresi kosong (atau hanya spasi) bernilai true (file/fragment aktif selama
// modulnya aktif, ADR-002 §5.1). Field di luar daftar legal (§5.2) atau sintaks
// rusak → error fail-fast (bukan silent-false).
func evalWhen(expr string, a answers.Answers) (bool, error) {
	return evalWhenService(expr, a, nil)
}

// evalWhenService mengevaluasi `when` dengan OVERLAY field PER-SERVICE (Fase 4b):
// override memuat field sintetik (mis. ".IsFirst" bool, ".Service" string) yang
// TIDAK ada di Answers — disuntikkan saat ekspansi file/fragment per-service. Field
// di override DIPERIKSA DULU (menang atas Answers); selain itu jatuh ke daftar legal
// Answers (§5.2). override nil ⇒ identik dengan evalWhen biasa. Determinisme dijaga:
// override diisi resolver dgn nilai deterministik per service.
func evalWhenService(expr string, a answers.Answers, override map[string]any) (bool, error) {
	if strings.TrimSpace(expr) == "" {
		return true, nil
	}
	toks, err := lexWhen(expr)
	if err != nil {
		return false, err
	}
	p := &whenParser{toks: toks, ans: a, override: override}
	val, err := p.parseExpr()
	if err != nil {
		return false, err
	}
	if !p.atEnd() {
		return false, fmt.Errorf("when %q: token tak terduga setelah ekspresi: %q", expr, p.peek().text)
	}
	return val, nil
}

// ── Lexer ────────────────────────────────────────────────────────────────────

type tokenKind int

const (
	tokIdent  tokenKind = iota // and, or, not, eq, ne, atau ident lain
	tokField                   // .Arch, .DB, ...
	tokString                  // "microservice", ""
	tokNumber                  // 5432
	tokLParen                  // (
	tokRParen                  // )
)

type whenToken struct {
	kind tokenKind
	text string // teks mentah (tanpa kutip untuk string)
}

// lexWhen memecah ekspresi menjadi token. Mendukung: ident kata-kunci, field
// berawalan '.', string literal ber-kutip-ganda (dengan escape \"), angka, dan
// kurung. Karakter lain → error.
//
// R-1: badan loop didelegasikan ke scanner per-jenis-token (lexOne) agar kompleksitas
// kognitif tetap rendah; grammar & perilaku TIDAK berubah (sama persis dengan versi
// switch monolitik sebelumnya). lexWhen kini hanya meng-iterasi & mengakumulasi.
func lexWhen(expr string) ([]whenToken, error) {
	var toks []whenToken
	runes := []rune(expr)
	for i := 0; i < len(runes); {
		next, tok, emit, err := lexOne(expr, runes, i)
		if err != nil {
			return nil, err
		}
		if emit {
			toks = append(toks, tok)
		}
		i = next
	}
	return toks, nil
}

// lexOne memindai SATU token (atau spasi) mulai dari indeks i. Mengembalikan indeks
// berikutnya, token yang dihasilkan, flag emit (false untuk whitespace yang tak
// menghasilkan token), dan error. expr disertakan hanya untuk pesan error kontekstual.
func lexOne(expr string, runes []rune, i int) (next int, tok whenToken, emit bool, err error) {
	c := runes[i]
	switch {
	case c == ' ' || c == '\t' || c == '\n' || c == '\r':
		return i + 1, whenToken{}, false, nil
	case c == '(':
		return i + 1, whenToken{tokLParen, "("}, true, nil
	case c == ')':
		return i + 1, whenToken{tokRParen, ")"}, true, nil
	case c == '"':
		return lexString(expr, runes, i)
	case c == '.':
		return lexField(expr, runes, i)
	case c >= '0' && c <= '9':
		return lexNumber(runes, i)
	case isIdentStart(c):
		return lexIdent(runes, i)
	default:
		return 0, whenToken{}, false, fmt.Errorf("when %q: karakter tak terduga %q", expr, string(c))
	}
}

// lexString memindai string literal ber-kutip-ganda mulai di runes[i]=='"', dengan
// dukungan escape \" dan \\. Mengembalikan tokString tanpa kutip.
func lexString(expr string, runes []rune, i int) (int, whenToken, bool, error) {
	i++ // lewati kutip pembuka
	var sb strings.Builder
	for i < len(runes) {
		ch := runes[i]
		if ch == '\\' && i+1 < len(runes) {
			if nxt := runes[i+1]; nxt == '"' || nxt == '\\' {
				sb.WriteRune(nxt)
				i += 2
				continue
			}
		}
		if ch == '"' {
			return i + 1, whenToken{tokString, sb.String()}, true, nil
		}
		sb.WriteRune(ch)
		i++
	}
	return 0, whenToken{}, false, fmt.Errorf("when %q: string literal tidak ditutup", expr)
}

// lexField memindai field berawalan '.' diikuti ident{.ident} mulai di runes[i]=='.'.
func lexField(expr string, runes []rune, i int) (int, whenToken, bool, error) {
	start := i
	i++ // lewati '.'
	if i >= len(runes) || !isIdentStart(runes[i]) {
		return 0, whenToken{}, false, fmt.Errorf("when %q: nama field kosong setelah '.'", expr)
	}
	for i < len(runes) && (isIdentPart(runes[i]) || runes[i] == '.') {
		i++
	}
	return i, whenToken{tokField, string(runes[start:i])}, true, nil
}

// lexNumber memindai deret digit mulai di runes[i].
func lexNumber(runes []rune, i int) (int, whenToken, bool, error) {
	start := i
	for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
		i++
	}
	return i, whenToken{tokNumber, string(runes[start:i])}, true, nil
}

// lexIdent memindai identifier (kata-kunci and/or/not/eq/ne atau lainnya) mulai di
// runes[i].
func lexIdent(runes []rune, i int) (int, whenToken, bool, error) {
	start := i
	for i < len(runes) && isIdentPart(runes[i]) {
		i++
	}
	return i, whenToken{tokIdent, string(runes[start:i])}, true, nil
}

func isIdentStart(c rune) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c rune) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

// ── Parser (recursive descent) ───────────────────────────────────────────────

type whenParser struct {
	toks []whenToken
	pos  int
	ans  answers.Answers
	// override memuat field PER-SERVICE sintetik (Fase 4b: ".IsFirst", ".Service")
	// yang tak ada di Answers; diperiksa SEBELUM Answers. nil untuk evaluasi biasa.
	override map[string]any
}

func (p *whenParser) atEnd() bool     { return p.pos >= len(p.toks) }
func (p *whenParser) peek() whenToken { return p.toks[p.pos] }
func (p *whenParser) next() whenToken { t := p.toks[p.pos]; p.pos++; return t }

// parseExpr mengevaluasi satu ekspresi boolean.
//
// Gaya text/template (ADR-002 §5.3, §5.4): and/or/not/eq/ne adalah FUNGSI PREFIX,
// bukan operator infix. Bentuk:
//   - and arg1 arg2 ...   → konjungsi (≥2 argumen)
//   - or  arg1 arg2 ...   → disjungsi (≥2 argumen)
//   - not arg             → negasi
//   - eq  a b             → a == b
//   - ne  a b             → a != b
//   - .Field              → field bool langsung (atom)
//   - ( expr )            → grouping
//
// Argumen and/or/not adalah ekspresi boolean penuh (rekursif); argumen eq/ne
// adalah field/string/number (dibandingkan sebagai string). Presedensi natural
// dipikul oleh kurung pada bentuk prefix (and mengikat lebih kuat dari or saat
// ditulis bersarang — ADR-002 §5.1), konsisten dengan semantik text/template.
func (p *whenParser) parseExpr() (bool, error) {
	if p.atEnd() {
		return false, fmt.Errorf("when: ekspresi terpotong, mengharapkan term")
	}
	t := p.peek()
	switch t.kind {
	case tokLParen:
		p.next()
		v, err := p.parseExpr()
		if err != nil {
			return false, err
		}
		if p.atEnd() || p.peek().kind != tokRParen {
			return false, fmt.Errorf("when: kurung '(' tidak ditutup ')'")
		}
		p.next()
		return v, nil
	case tokField:
		// Atom: field bool langsung.
		p.next()
		return p.boolField(t.text)
	case tokIdent:
		switch t.text {
		case "not":
			p.next()
			v, err := p.parseExpr()
			if err != nil {
				return false, err
			}
			return !v, nil
		case "and":
			return p.parseVariadic("and")
		case "or":
			return p.parseVariadic("or")
		case "eq", "ne":
			return p.parseCompare(t.text)
		default:
			return false, fmt.Errorf("when: identifier tak dikenal %q (hanya eq/ne/and/or/not + field legal)", t.text)
		}
	default:
		return false, fmt.Errorf("when: token tak terduga %q di posisi term", t.text)
	}
}

// parseVariadic mengevaluasi fungsi prefix and/or dengan ≥2 argumen boolean.
// Argumen dibaca selama token berikutnya dapat memulai sebuah term (field, '(',
// atau ident fungsi). Semua argumen SELALU dievaluasi (tidak ada short-circuit):
// daftar term di-parse penuh lalu dilipat dengan &&/|| — menjamin validasi penuh
// & fail-fast pada field ilegal sekalipun hasil sudah terdeterminasi argumen awal.
func (p *whenParser) parseVariadic(fn string) (bool, error) {
	p.next() // konsumsi nama fungsi
	args, err := p.parseTermList()
	if err != nil {
		return false, err
	}
	if len(args) < 2 {
		return false, fmt.Errorf("when: %q butuh minimal 2 argumen, dapat %d", fn, len(args))
	}
	result := args[0]
	for _, v := range args[1:] {
		if fn == "and" {
			result = result && v
		} else {
			result = result || v
		}
	}
	return result, nil
}

// parseTermList membaca daftar term boolean berurutan untuk argumen and/or,
// berhenti saat token berikutnya menutup grup ')' atau habis.
func (p *whenParser) parseTermList() ([]bool, error) {
	var args []bool
	for !p.atEnd() && p.startsTerm() {
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	}
	return args, nil
}

// startsTerm melaporkan apakah token saat ini dapat memulai sebuah term boolean.
func (p *whenParser) startsTerm() bool {
	if p.atEnd() {
		return false
	}
	t := p.peek()
	switch t.kind {
	case tokField, tokLParen:
		return true
	case tokIdent:
		switch t.text {
		case "and", "or", "not", "eq", "ne":
			return true
		}
	}
	return false
}

// parseCompare mengevaluasi fungsi biner eq/ne atas dua argumen
// (field/string/number, dibandingkan sebagai string).
func (p *whenParser) parseCompare(fn string) (bool, error) {
	p.next() // konsumsi nama fungsi (eq/ne)
	lhs, err := p.parseArg()
	if err != nil {
		return false, err
	}
	rhs, err := p.parseArg()
	if err != nil {
		return false, err
	}
	equal := lhs == rhs
	if fn == "eq" {
		return equal, nil
	}
	return !equal, nil // ne
}

// parseArg mengembalikan representasi string sebuah argumen eq/ne (field
// diresolusi ke nilainya; literal apa adanya).
func (p *whenParser) parseArg() (string, error) {
	if p.atEnd() {
		return "", fmt.Errorf("when: argumen eq/ne kurang")
	}
	t := p.next()
	switch t.kind {
	case tokString:
		return t.text, nil
	case tokNumber:
		return t.text, nil
	case tokField:
		return p.stringField(t.text)
	default:
		return "", fmt.Errorf("when: argumen eq/ne tidak valid %q (harus field, string, atau angka)", t.text)
	}
}

// ── Resolusi field legal (ADR-002 §5.2) ──────────────────────────────────────

// stringFieldAccessors & boolFieldAccessors memetakan nama field `when` → fungsi
// pengambil nilai dari Answers. Di-hoist ke level paket (dibangun SEKALI saat
// init) agar evalWhen TIDAK mengalokasikan map baru per evaluasi — daftar field
// legal bersifat statik; hanya nilainya yang bergantung pada Answers (ADR-002
// §5.2). Daftar key identik dengan implementasi map-literal sebelumnya.
var (
	stringFieldAccessors = map[string]func(answers.Answers) string{
		".Arch":         func(a answers.Answers) string { return string(a.Arch) },
		".Kind":         func(a answers.Answers) string { return string(a.Kind) },
		".Comm":         func(a answers.Answers) string { return string(a.Comm) },
		".Broker":       func(a answers.Answers) string { return string(a.Broker) },
		".HTTP":         func(a answers.Answers) string { return string(a.HTTP) },
		".DB":           func(a answers.Answers) string { return string(a.DB) },
		".Access":       func(a answers.Answers) string { return string(a.Access) },
		".Migrate":      func(a answers.Answers) string { return string(a.Migrate) },
		".CI":           func(a answers.Answers) string { return string(a.CI) },
		".Auth":         func(a answers.Answers) string { return string(a.Auth) },
		".ConfigLoader": func(a answers.Answers) string { return string(a.ConfigLoader) },
		".Log":          func(a answers.Answers) string { return string(a.Log) },
	}

	boolFieldAccessors = map[string]func(answers.Answers) bool{
		".Docker":        func(a answers.Answers) bool { return a.Docker },
		".Makefile":      func(a answers.Answers) bool { return a.Makefile },
		".Taskfile":      func(a answers.Answers) bool { return a.Taskfile },
		".Lint":          func(a answers.Answers) bool { return a.Lint },
		".Obs":           func(a answers.Answers) bool { return a.Obs },
		".EnvExample":    func(a answers.Answers) bool { return a.EnvExample },
		".ValidateInput": func(a answers.Answers) bool { return a.ValidateInput },
		".Gateway":       func(a answers.Answers) bool { return a.Gateway },
		".Git":           func(a answers.Answers) bool { return a.Git },
		".Mock":          func(a answers.Answers) bool { return a.Mock },
		".Integration":   func(a answers.Answers) bool { return a.Integration },
	}
)

// boolField mengembalikan nilai bool sebuah field; error bila field bukan bool
// legal (fail-fast — termasuk bila field sebenarnya string atau tak dikenal).
// Field PER-SERVICE di override (mis. ".IsFirst") diperiksa DULU.
func (p *whenParser) boolField(name string) (bool, error) {
	if v, ok := p.overrideValue(name); ok {
		b, isBool := v.(bool)
		if !isBool {
			return false, fmt.Errorf("when: field per-service %q bukan bool (tipe %T); tak boleh dipakai sbg atom boolean", name, v)
		}
		return b, nil
	}
	if get, ok := boolFieldAccessors[name]; ok {
		return get(p.ans), nil
	}
	if _, ok := stringFieldAccessors[name]; ok {
		return false, fmt.Errorf("when: field %q bertipe string, tidak boleh dipakai sebagai atom boolean (gunakan eq/ne)", name)
	}
	return false, fmt.Errorf("when: field %q tidak legal (lihat ADR-002 §5.2 daftar field)", name)
}

// stringField mengembalikan representasi string sebuah field untuk argumen
// eq/ne; error bila field tak legal. Field bool direpresentasikan "true"/"false".
// Field PER-SERVICE di override (mis. ".Service") diperiksa DULU.
func (p *whenParser) stringField(name string) (string, error) {
	if v, ok := p.overrideValue(name); ok {
		switch t := v.(type) {
		case string:
			return t, nil
		case bool:
			if t {
				return "true", nil
			}
			return "false", nil
		default:
			return fmt.Sprintf("%v", t), nil
		}
	}
	if get, ok := stringFieldAccessors[name]; ok {
		return get(p.ans), nil
	}
	if get, ok := boolFieldAccessors[name]; ok {
		if get(p.ans) {
			return "true", nil
		}
		return "false", nil
	}
	return "", fmt.Errorf("when: field %q tidak legal (lihat ADR-002 §5.2 daftar field)", name)
}

// overrideValue mencari nilai field PER-SERVICE di overlay (kunci tanpa titik
// awal, mis. name ".IsFirst" → kunci "IsFirst"). Mengembalikan (nil,false) bila
// tak ada overlay atau field tak ada di overlay (jatuh ke resolusi Answers).
func (p *whenParser) overrideValue(name string) (any, bool) {
	if p.override == nil {
		return nil, false
	}
	v, ok := p.override[strings.TrimPrefix(name, ".")]
	return v, ok
}
