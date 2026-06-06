package resolver

import (
	"strings"
	"testing"

	"github.com/faisalcayunda/gostarter/internal/answers"
)

// TestLexWhen_Errors memverifikasi lexer `when` menolak token ilegal RAMAH:
// karakter tak terduga, string literal tak ditutup, field kosong setelah '.'.
func TestLexWhen_Errors(t *testing.T) {
	cases := []struct {
		name    string
		expr    string
		wantSub string
	}{
		{"karakter tak terduga", `eq .Arch @`, "karakter tak terduga"},
		{"string tak ditutup", `eq .Arch "monolith`, "tidak ditutup"},
		{"field kosong setelah titik", `eq . "x"`, "nama field kosong"},
		{"karakter persen", `100%`, "karakter tak terduga"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := lexWhen(tc.expr)
			if err == nil {
				t.Fatalf("lexWhen(%q) mau error, dapat nil", tc.expr)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("pesan harus memuat %q, dapat: %v", tc.wantSub, err)
			}
		})
	}
}

// TestLexWhen_StringEscape memverifikasi lexer menangani escape \" dan \\ di
// dalam string literal.
func TestLexWhen_StringEscape(t *testing.T) {
	toks, err := lexWhen(`eq .Name "a\"b\\c"`)
	if err != nil {
		t.Fatalf("lexWhen escape error: %v", err)
	}
	// Token terakhir = string literal dengan escape ter-unescape.
	last := toks[len(toks)-1]
	if last.kind != tokString {
		t.Fatalf("token terakhir bukan string, kind=%d", last.kind)
	}
	if last.text != `a"b\c` {
		t.Errorf("string escape = %q, mau %q", last.text, `a"b\c`)
	}
}

// TestLexWhen_Number memverifikasi lexer mengenali angka sebagai tokNumber
// (dipakai eq/ne dengan port mis. eq .Port 5432).
func TestLexWhen_Number(t *testing.T) {
	toks, err := lexWhen(`eq .X 5432`)
	if err != nil {
		t.Fatalf("lexWhen number error: %v", err)
	}
	found := false
	for _, tk := range toks {
		if tk.kind == tokNumber && tk.text == "5432" {
			found = true
		}
	}
	if !found {
		t.Errorf("angka 5432 harus jadi tokNumber, toks=%+v", toks)
	}
}

// TestEvalWhen_Precedence memverifikasi presedensi & semantik prefix and/or/not
// (text/template style): not terkuat, and/or variadic, kurung override.
func TestEvalWhen_Precedence(t *testing.T) {
	a := answers.Answers{
		Arch:   answers.ArchMonolith,
		HTTP:   answers.HTTPNetHTTP,
		DB:     answers.DBPostgres,
		Docker: true,
		Obs:    false,
		Lint:   true,
	}
	cases := []struct {
		expr string
		want bool
	}{
		// Variadic and/or (≥2 argumen).
		{`and .Docker .Lint`, true},
		{`and .Docker .Obs`, false},
		{`or .Obs .Docker`, true},
		{`or .Obs (eq .HTTP "chi")`, false},
		// and variadic 3 argumen.
		{`and .Docker .Lint (eq .DB "postgres")`, true},
		{`and .Docker .Lint .Obs`, false},
		// not membungkus grup.
		{`not (or .Obs (eq .HTTP "chi"))`, true},
		{`not .Docker`, false},
		// Nested: or(obs, and(docker, db=postgres)) → true via cabang kedua.
		{`or .Obs (and .Docker (eq .DB "postgres"))`, true},
		// Kurung mengubah evaluasi.
		{`and (or .Obs .Docker) (eq .DB "postgres")`, true},
		// eq/ne dengan string kosong.
		{`ne .DB ""`, true},
		{`eq .Obs "false"`, true}, // bool direpresentasi "false" sbg string arg eq.
	}
	for _, c := range cases {
		got, err := evalWhen(c.expr, a)
		if err != nil {
			t.Errorf("evalWhen(%q) error: %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("evalWhen(%q) = %v, mau %v", c.expr, got, c.want)
		}
	}
}

// TestEvalWhen_VariadicArityError memverifikasi and/or dengan <2 argumen → error
// (fail-fast; pesan menyebut kebutuhan minimal 2 argumen).
func TestEvalWhen_VariadicArityError(t *testing.T) {
	a := answers.Answers{Docker: true}
	for _, expr := range []string{`and .Docker`, `or .Docker`, `and`, `or`} {
		_, err := evalWhen(expr, a)
		if err == nil {
			t.Errorf("evalWhen(%q) mau error (arity), dapat nil", expr)
		}
	}
	// Pesan menyebut "2 argumen" untuk kasus 1-argumen.
	_, err := evalWhen(`and .Docker`, a)
	if err == nil || !strings.Contains(err.Error(), "2 argumen") {
		t.Errorf("pesan arity harus menyebut '2 argumen', dapat: %v", err)
	}
}

// TestEvalWhen_FieldErrors memverifikasi fail-fast field ilegal & salah-tipe:
//   - field tak dikenal → error;
//   - field string dipakai sebagai atom bool → error menyebut "string";
//   - eq/ne dengan field ilegal → error.
func TestEvalWhen_FieldErrors(t *testing.T) {
	a := answers.Answers{Arch: answers.ArchMonolith, Docker: true}
	cases := []struct {
		name    string
		expr    string
		wantSub string
	}{
		{"field tak dikenal sbg atom", ".Bogus", "tidak legal"},
		{"field string sbg atom bool", ".Arch", "string"},
		{"field ilegal di eq lhs", `eq .Bogus "x"`, "tidak legal"},
		{"field ilegal di eq rhs", `eq "x" .Bogus`, "tidak legal"},
		{"identifier tak dikenal", `xor .Docker .Obs`, "tak dikenal"},
		{"token sisa setelah ekspresi", `.Docker .Obs`, "tak terduga"},
		{"kurung tak ditutup", `(.Docker`, "tidak ditutup"},
		{"eq argumen kurang", `eq .Docker`, "argumen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := evalWhen(tc.expr, a)
			if err == nil {
				t.Fatalf("evalWhen(%q) mau error, dapat nil", tc.expr)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("pesan harus memuat %q, dapat: %v", tc.wantSub, err)
			}
		})
	}
}

// TestEvalWhenService_Override memverifikasi overlay field PER-SERVICE (Fase 4b):
//   - field sintetik ".IsFirst" bool diperiksa DULU (menang atas Answers);
//   - ".Service" string dipakai di eq;
//   - override bool dipakai sebagai atom; non-bool override sebagai atom → error;
//   - field Answers tetap terbaca bila tak ada di overlay.
func TestEvalWhenService_Override(t *testing.T) {
	a := answers.Answers{Arch: answers.ArchMicroservice, Docker: true}

	t.Run("IsFirst true atom", func(t *testing.T) {
		ov := map[string]any{"IsFirst": true, "Service": "svc-a"}
		got, err := evalWhenService(".IsFirst", a, ov)
		if err != nil || !got {
			t.Errorf("evalWhenService(.IsFirst) = %v, %v; mau true,nil", got, err)
		}
	})
	t.Run("IsFirst false atom", func(t *testing.T) {
		ov := map[string]any{"IsFirst": false}
		got, err := evalWhenService(".IsFirst", a, ov)
		if err != nil || got {
			t.Errorf("evalWhenService(.IsFirst=false) = %v, %v; mau false,nil", got, err)
		}
	})
	t.Run("Service string di eq", func(t *testing.T) {
		ov := map[string]any{"Service": "svc-a"}
		got, err := evalWhenService(`eq .Service "svc-a"`, a, ov)
		if err != nil || !got {
			t.Errorf("eq .Service svc-a = %v, %v; mau true,nil", got, err)
		}
	})
	t.Run("override non-bool sbg atom bool → error", func(t *testing.T) {
		ov := map[string]any{"Service": "svc-a"} // string, bukan bool
		_, err := evalWhenService(".Service", a, ov)
		if err == nil || !strings.Contains(err.Error(), "bukan bool") {
			t.Errorf("override string sbg atom bool harus error 'bukan bool', dapat: %v", err)
		}
	})
	t.Run("field Answers tetap terbaca bila tak di overlay", func(t *testing.T) {
		ov := map[string]any{"IsFirst": true} // .Docker tidak di overlay
		got, err := evalWhenService(".Docker", a, ov)
		if err != nil || !got {
			t.Errorf("evalWhenService(.Docker) fallback Answers = %v, %v; mau true,nil", got, err)
		}
	})
	t.Run("override bool sebagai string arg eq", func(t *testing.T) {
		ov := map[string]any{"IsFirst": true}
		// .IsFirst direpresentasi "true" di eq (stringField path override bool).
		got, err := evalWhenService(`eq .IsFirst "true"`, a, ov)
		if err != nil || !got {
			t.Errorf("eq .IsFirst true (override bool→string) = %v, %v; mau true,nil", got, err)
		}
	})
}

// TestEvalWhen_TermListAndGrouping memverifikasi parseTermList/startsTerm: argumen
// and/or dapat berupa term ber-kurung, ber-not, atau call eq/ne bersarang — daftar
// term di-baca hingga ')' atau habis. Menutup cabang startsTerm untuk tiap jenis term.
func TestEvalWhen_TermListAndGrouping(t *testing.T) {
	a := answers.Answers{
		Arch: answers.ArchMonolith, DB: answers.DBPostgres,
		Docker: true, Lint: true, Obs: false,
	}
	cases := []struct {
		expr string
		want bool
	}{
		// and dengan argumen call (eq) + grup + not.
		{`and (eq .Arch "monolith") (not .Obs)`, true},
		// or dengan 3 argumen campuran: field, grup, call.
		{`or .Obs (and .Docker .Lint) (eq .DB "mysql")`, true},
		// and bersarang dalam or, term pertama call.
		{`or (eq .DB "mysql") (and (eq .Arch "monolith") .Docker)`, true},
		// not membungkus and variadic.
		{`not (and .Docker .Obs)`, true},
		// term list berhenti di ')': and di dalam grup, lalu field di luar.
		{`and (or .Obs .Docker) .Lint`, true},
	}
	for _, c := range cases {
		got, err := evalWhen(c.expr, a)
		if err != nil {
			t.Errorf("evalWhen(%q) error: %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("evalWhen(%q) = %v, mau %v", c.expr, got, c.want)
		}
	}
}

// TestEvalWhen_ArgErrors memverifikasi parseArg menolak argumen eq/ne yang BUKAN
// field/string/number (mis. operator 'and' sebagai argumen eq) & argumen kurang.
func TestEvalWhen_ArgErrors(t *testing.T) {
	a := answers.Answers{Docker: true}
	cases := []struct {
		name string
		expr string
	}{
		{"eq dengan operator sbg arg", `eq and .Docker`},
		{"ne dengan kurung sbg arg", `ne ( .Docker`},
		{"eq lhs ada rhs hilang", `eq .Docker`},
		{"eq tanpa argumen", `eq`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := evalWhen(tc.expr, a); err == nil {
				t.Errorf("evalWhen(%q) mau error (arg invalid), dapat nil", tc.expr)
			}
		})
	}
}

// TestEvalWhen_EmptyAlwaysTrue memverifikasi ekspresi kosong / whitespace = true
// (file/fragment aktif selama modul aktif, ADR-002 §5.1).
func TestEvalWhen_EmptyAlwaysTrue(t *testing.T) {
	for _, expr := range []string{"", "   ", "\t\n"} {
		got, err := evalWhen(expr, answers.Answers{})
		if err != nil || !got {
			t.Errorf("evalWhen(%q) = %v, %v; mau true,nil", expr, got, err)
		}
	}
}

// TestEvalWhen_AllLegalBoolFields memverifikasi SEMUA field bool legal (§5.2)
// dapat dievaluasi sebagai atom tanpa error — mencegah daftar accessor drift.
func TestEvalWhen_AllLegalBoolFields(t *testing.T) {
	a := answers.Answers{
		Docker: true, Makefile: true, Taskfile: true, Lint: true, Obs: true,
		EnvExample: true, ValidateInput: true, Gateway: true, Git: true,
		Mock: true, Integration: true,
	}
	boolFields := []string{
		".Docker", ".Makefile", ".Taskfile", ".Lint", ".Obs", ".EnvExample",
		".ValidateInput", ".Gateway", ".Git", ".Mock", ".Integration",
	}
	for _, f := range boolFields {
		got, err := evalWhen(f, a)
		if err != nil {
			t.Errorf("evalWhen(%q) bool field legal harus tanpa error, dapat: %v", f, err)
			continue
		}
		if !got {
			t.Errorf("evalWhen(%q) = false, mau true (semua di-set)", f)
		}
	}
}

// TestEvalWhen_AllLegalStringFields memverifikasi field string legal (§5.2)
// dapat dipakai di eq tanpa error.
func TestEvalWhen_AllLegalStringFields(t *testing.T) {
	a := answers.Answers{
		Arch: answers.ArchMonolith, Kind: answers.KindREST, Comm: answers.CommGRPC,
		Broker: answers.BrokerNATS, HTTP: answers.HTTPChi, DB: answers.DBPostgres,
		Access: answers.AccessSQLx, Migrate: answers.MigrateGolangMigrate,
		CI: answers.CIGitHubActions, Auth: answers.AuthJWT,
		ConfigLoader: answers.ConfigLoaderKoanf, Log: answers.LogZap,
	}
	stringExprs := map[string]bool{
		`eq .Arch "monolith"`:          true,
		`eq .Kind "rest"`:              true,
		`eq .Comm "grpc"`:              true,
		`eq .Broker "nats"`:            true,
		`eq .HTTP "chi"`:               true,
		`eq .DB "postgres"`:            true,
		`eq .Access "sqlx"`:            true,
		`eq .Migrate "golang-migrate"`: true,
		`eq .CI "github-actions"`:      true,
		`eq .Auth "jwt"`:               true,
		`eq .ConfigLoader "koanf"`:     true,
		`eq .Log "zap"`:                true,
	}
	for expr, want := range stringExprs {
		got, err := evalWhen(expr, a)
		if err != nil {
			t.Errorf("evalWhen(%q) string field legal harus tanpa error, dapat: %v", expr, err)
			continue
		}
		if got != want {
			t.Errorf("evalWhen(%q) = %v, mau %v", expr, got, want)
		}
	}
}
