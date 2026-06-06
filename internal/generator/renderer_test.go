package generator

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestFuncMapRegistration memastikan FuncMap mengekspor TEPAT tujuh fungsi kanonik
// (ADR-002 §4 + toScreamingSnake untuk nama env var) — tidak kurang, tidak lebih.
func TestFuncMapRegistration(t *testing.T) {
	fm := FuncMap()
	want := []string{"toCamel", "toPascal", "toSnake", "toScreamingSnake", "toKebab", "modBase", "modJoin"}
	if len(fm) != len(want) {
		t.Fatalf("FuncMap punya %d entri, mau %d", len(fm), len(want))
	}
	for _, name := range want {
		if _, ok := fm[name]; !ok {
			t.Errorf("FuncMap kehilangan fungsi %q", name)
		}
	}
}

func TestToScreamingSnake(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"svc-b", "SVC_B"},
		{"user name", "USER_NAME"},
		{"user_name", "USER_NAME"},
		{"UserName", "USER_NAME"},
		{"HTTPServer", "HTTP_SERVER"},
		{"order", "ORDER"},
		{"", ""},
	}
	for _, c := range cases {
		if got := toScreamingSnake(c.in); got != c.want {
			t.Errorf("toScreamingSnake(%q) = %q, mau %q", c.in, got, c.want)
		}
	}
}

func TestToCamel(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"user name", "userName"}, // contoh kanonik instruksi
		{"user_name", "userName"},
		{"user-name", "userName"},
		{"UserName", "userName"},
		{"HTTPServer", "httpServer"},
		{"order", "order"},
		{"", ""},
	}
	for _, c := range cases {
		if got := toCamel(c.in); got != c.want {
			t.Errorf("toCamel(%q) = %q, mau %q", c.in, got, c.want)
		}
	}
}

func TestToPascal(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"user name", "UserName"}, // contoh kanonik instruksi
		{"user_name", "UserName"},
		{"user-name", "UserName"},
		{"userName", "UserName"},
		{"HTTPServer", "HttpServer"},
		{"order", "Order"},
		{"", ""},
	}
	for _, c := range cases {
		if got := toPascal(c.in); got != c.want {
			t.Errorf("toPascal(%q) = %q, mau %q", c.in, got, c.want)
		}
	}
}

func TestToSnake(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"user name", "user_name"}, // contoh kanonik instruksi
		{"user_name", "user_name"},
		{"user-name", "user_name"},
		{"UserName", "user_name"},
		{"HTTPServer", "http_server"},
		{"order", "order"},
		{"", ""},
	}
	for _, c := range cases {
		if got := toSnake(c.in); got != c.want {
			t.Errorf("toSnake(%q) = %q, mau %q", c.in, got, c.want)
		}
	}
}

func TestToKebab(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"user name", "user-name"}, // contoh kanonik instruksi
		{"user_name", "user-name"},
		{"user-name", "user-name"},
		{"UserName", "user-name"},
		{"HTTPServer", "http-server"},
		{"order", "order"},
		{"", ""},
	}
	for _, c := range cases {
		if got := toKebab(c.in); got != c.want {
			t.Errorf("toKebab(%q) = %q, mau %q", c.in, got, c.want)
		}
	}
}

func TestModBase(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"github.com/x/demo", "demo"}, // contoh kanonik instruksi
		{"github.com/acme/shop", "shop"},
		{"github.com/acme/shop/v2", "shop"}, // strip suffix versi mayor
		{"github.com/acme/shop/v10", "shop"},
		{"github.com/acme/shop/v1", "v1"}, // v1 BUKAN suffix versi mayor
		{"demo", "demo"},
		{"", ""},
	}
	for _, c := range cases {
		if got := modBase(c.in); got != c.want {
			t.Errorf("modBase(%q) = %q, mau %q", c.in, got, c.want)
		}
	}
}

func TestModJoin(t *testing.T) {
	cases := []struct {
		base string
		elem []string
		want string
	}{
		{ // contoh kanonik instruksi
			base: "github.com/acme/fleet",
			elem: []string{"services", "user"},
			want: "github.com/acme/fleet/services/user",
		},
		{
			base: "github.com/acme/shop",
			elem: []string{"internal", "app"},
			want: "github.com/acme/shop/internal/app",
		},
		{
			base: "github.com/acme/shop",
			elem: nil,
			want: "github.com/acme/shop",
		},
	}
	for _, c := range cases {
		if got := modJoin(c.base, c.elem...); got != c.want {
			t.Errorf("modJoin(%q, %v) = %q, mau %q", c.base, c.elem, got, c.want)
		}
	}
}

// TestRenderNonGo memverifikasi render template non-.go memakai FuncMap dan
// mengembalikan output apa adanya (tanpa gofmt).
func TestRenderNonGo(t *testing.T) {
	fsys := fstest.MapFS{
		"env.example.tmpl": {Data: []byte("APP_NAME={{ modBase .ModulePath }}\n")},
	}
	r := NewRenderer(fsys)
	got, err := r.Render("env.example.tmpl", map[string]any{"ModulePath": "github.com/acme/shop"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	want := "APP_NAME=shop\n"
	if string(got) != want {
		t.Errorf("Render = %q, mau %q", string(got), want)
	}
}

// TestRenderGoGofmt memverifikasi output target .go dilewatkan go/format: input
// template yang sengaja "kurang rapi" harus keluar gofmt-valid (ter-indentasi).
func TestRenderGoGofmt(t *testing.T) {
	// Body sengaja tanpa indentasi & spasi tak rapi; gofmt harus merapikan.
	src := "package {{ modBase .ModulePath }}\nfunc  F( ){\nreturn\n}\n"
	fsys := fstest.MapFS{
		"main.go.tmpl": {Data: []byte(src)},
	}
	r := NewRenderer(fsys)
	got, err := r.Render("main.go.tmpl", map[string]any{"ModulePath": "github.com/acme/shop"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	want := "package shop\n\nfunc F() {\n\treturn\n}\n"
	if string(got) != want {
		t.Errorf("Render gofmt =\n%q\nmau\n%q", string(got), want)
	}
}

// TestRenderGoInvalid memastikan Go tidak valid hasil render → error (bukan tulis
// mentah), sesuai ADR-002 §6.
func TestRenderGoInvalid(t *testing.T) {
	fsys := fstest.MapFS{
		"bad.go.tmpl": {Data: []byte("package x\nfunc (\n")},
	}
	r := NewRenderer(fsys)
	if _, err := r.Render("bad.go.tmpl", nil); err == nil {
		t.Fatal("Render Go invalid: mau error, dapat nil")
	}
}

// TestSplitTokens menguji helper normalisasi token yang memusatkan logika
// casing keempat fungsi to*.
func TestSplitTokens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"user_name", []string{"user", "name"}},
		{"user-name", []string{"user", "name"}},
		{"user name", []string{"user", "name"}},
		{"UserName", []string{"User", "Name"}},
		{"HTTPServer", []string{"HTTP", "Server"}},
		{"order2user", []string{"order2user"}},
		{"  spaced  out ", []string{"spaced", "out"}},
		{"", nil},
	}
	for _, c := range cases {
		got := splitTokens(c.in)
		if !equalStrings(got, c.want) {
			t.Errorf("splitTokens(%q) = %v, mau %v", c.in, got, c.want)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestFuncMap_AcronymEdges memverifikasi penanganan akronim & batas case lanjutan
// di SELURUH fungsi casing — kasus tepi yang mudah pecah (HTTPServer, ID di akhir,
// run akronim di tengah). Memusatkan ekspektasi agar regresi splitCaseBoundaries
// terdeteksi lintas fungsi.
func TestFuncMap_AcronymEdges(t *testing.T) {
	cases := []struct {
		in                                     string
		camel, pascal, snake, screaming, kebab string
	}{
		// Akronim di awal diikuti kata: HTTP|Server.
		{"HTTPServer", "httpServer", "HttpServer", "http_server", "HTTP_SERVER", "http-server"},
		// Akronim tunggal: API → api/Api.
		{"API", "api", "Api", "api", "API", "api"},
		// Akronim di akhir: userID → user|ID (D di-Upper, transisi r→I).
		{"userID", "userId", "UserId", "user_id", "USER_ID", "user-id"},
		// Dua akronim berturut: XMLHTTPRequest → XML|HTTP|Request.
		{"XMLHTTPRequest", "xmlhttpRequest", "XmlhttpRequest", "xmlhttp_request", "XMLHTTP_REQUEST", "xmlhttp-request"},
	}
	for _, c := range cases {
		if got := toCamel(c.in); got != c.camel {
			t.Errorf("toCamel(%q) = %q, mau %q", c.in, got, c.camel)
		}
		if got := toPascal(c.in); got != c.pascal {
			t.Errorf("toPascal(%q) = %q, mau %q", c.in, got, c.pascal)
		}
		if got := toSnake(c.in); got != c.snake {
			t.Errorf("toSnake(%q) = %q, mau %q", c.in, got, c.snake)
		}
		if got := toScreamingSnake(c.in); got != c.screaming {
			t.Errorf("toScreamingSnake(%q) = %q, mau %q", c.in, got, c.screaming)
		}
		if got := toKebab(c.in); got != c.kebab {
			t.Errorf("toKebab(%q) = %q, mau %q", c.in, got, c.kebab)
		}
	}
}

// TestFuncMap_DigitsAndMixed memverifikasi token mengandung angka (tidak dipecah
// di batas digit→huruf bila tanpa pemisah/case eksplisit) & campuran pemisah.
func TestFuncMap_DigitsAndMixed(t *testing.T) {
	cases := []struct {
		in    string
		snake string
		kebab string
	}{
		{"order2user", "order2user", "order2user"}, // angka di tengah huruf-kecil → tetap satu token
		{"svc-b2", "svc_b2", "svc-b2"},             // dash pemisah, angka nempel
		{"v2Api", "v2_api", "v2-api"},              // transisi 2→A (digit→Upper) = batas case
		{"a_b-c d", "a_b_c_d", "a-b-c-d"},          // semua jenis pemisah dinormalkan
	}
	for _, c := range cases {
		if got := toSnake(c.in); got != c.snake {
			t.Errorf("toSnake(%q) = %q, mau %q", c.in, got, c.snake)
		}
		if got := toKebab(c.in); got != c.kebab {
			t.Errorf("toKebab(%q) = %q, mau %q", c.in, got, c.kebab)
		}
	}
}

// TestFuncMap_UnicodeAndSymbols memverifikasi karakter non-alnum (selain pemisah)
// dibuang & unicode letter/digit dipertahankan sebagai token. splitTokens membuang
// simbol; huruf unicode (mis. é) diperlakukan sebagai huruf.
func TestFuncMap_UnicodeAndSymbols(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"user@name", []string{"user", "name"}}, // @ = pembatas, dibuang
		{"a.b.c", []string{"a", "b", "c"}},      // titik = pembatas
		{"foo!!!bar", []string{"foo", "bar"}},   // simbol beruntun = satu+ pembatas
		{"café_bar", []string{"café", "bar"}},   // huruf unicode é dipertahankan
		{"   ", nil},                            // hanya whitespace → kosong
		{"___", nil},                            // hanya pemisah → kosong
	}
	for _, c := range cases {
		got := splitTokens(c.in)
		if !equalStrings(got, c.want) {
			t.Errorf("splitTokens(%q) = %v, mau %v", c.in, got, c.want)
		}
	}
	// toSnake unicode: café_bar → café_bar (lower-kan token, pemisah _).
	if got := toSnake("Café_Bar"); got != "café_bar" {
		t.Errorf("toSnake(Café_Bar) = %q, mau café_bar", got)
	}
}

// TestRaw_BypassesTemplate memverifikasi Renderer.Raw membaca byte MENTAH tanpa
// mengeksekusi template engine — aset statik (mis. .sql) yang memuat "{{" literal
// disalin verbatim (m-4: ModeCopy tak boleh gagal parse pada "{{").
func TestRaw_BypassesTemplate(t *testing.T) {
	// Konten memuat "{{" yang AKAN gagal bila dilewatkan text/template.
	raw := "INSERT INTO t (j) VALUES ('{{ not_a_template }}');\n"
	fsys := fstest.MapFS{
		"seed.sql.tmpl": {Data: []byte(raw)},
	}
	r := NewRenderer(fsys)
	got, err := r.Raw("seed.sql.tmpl")
	if err != nil {
		t.Fatalf("Raw error: %v", err)
	}
	if string(got) != raw {
		t.Errorf("Raw = %q, mau verbatim %q", string(got), raw)
	}
	// Render (engine) atas konten yang sama HARUS gagal — membuktikan Raw memang perlu.
	if _, err := r.Render("seed.sql.tmpl", nil); err == nil {
		t.Errorf("Render atas '{{ ... }}' non-template harus gagal (justifikasi Raw)")
	}
}

// TestRaw_MissingFile memverifikasi Raw mengembalikan error bila file tak ada.
func TestRaw_MissingFile(t *testing.T) {
	r := NewRenderer(fstest.MapFS{})
	if _, err := r.Raw("tidak/ada.tmpl"); err == nil {
		t.Fatal("Raw file tak ada: mau error, dapat nil")
	}
}

// TestRender_MissingTemplate memverifikasi Render mengembalikan error ramah bila
// path template tak ada di fsys.
func TestRender_MissingTemplate(t *testing.T) {
	r := NewRenderer(fstest.MapFS{})
	_, err := r.Render("tidak/ada.tmpl", nil)
	if err == nil || !strings.Contains(err.Error(), "baca template") {
		t.Errorf("Render template hilang = %v, mau error 'baca template'", err)
	}
}

// TestRender_ParseError memverifikasi template dengan sintaks rusak → error parse.
func TestRender_ParseError(t *testing.T) {
	fsys := fstest.MapFS{
		"bad.tmpl": {Data: []byte("{{ .Unclosed ")},
	}
	r := NewRenderer(fsys)
	_, err := r.Render("bad.tmpl", nil)
	if err == nil || !strings.Contains(err.Error(), "parse template") {
		t.Errorf("Render parse rusak = %v, mau error 'parse template'", err)
	}
}

// TestRender_ExecuteError memverifikasi error saat eksekusi → 'eksekusi template'.
// Mengakses field ".Field" pada data bertipe int memicu execute error yang andal
// ("can't evaluate field Field in type int") — TIDAK seperti nil data yang justru
// menghasilkan "<no value>" tanpa error.
func TestRender_ExecuteError(t *testing.T) {
	fsys := fstest.MapFS{
		"x.tmpl": {Data: []byte("{{ .Field }}")},
	}
	r := NewRenderer(fsys)
	_, err := r.Render("x.tmpl", 5) // data int → akses .Field gagal saat eksekusi
	if err == nil || !strings.Contains(err.Error(), "eksekusi template") {
		t.Errorf("Render execute error = %v, mau 'eksekusi template'", err)
	}
}
