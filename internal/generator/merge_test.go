package generator

import (
	"strings"
	"testing"

	"github.com/faisalcayunda/gostarter/internal/plan"
)

// TestAssembleTwoFragmentsOneAnchorOrdered adalah test inti yang diminta
// (ADR-002 §6): menyisipkan DUA fragmen pada SATU anchor, terurut by Order.
// Fragmen diberikan dengan urutan input terbalik terhadap Order untuk
// membuktikan assembler menegakkan urutan by Order, bukan urutan input.
func TestAssembleTwoFragmentsOneAnchorOrdered(t *testing.T) {
	skeleton := []byte("services:\n  # region:services\n")
	frags := []plan.Fragment{
		// Sengaja: Order 20 diberikan SEBELUM Order 10 di slice input.
		{Anchor: "services", Content: "  postgres:\n    image: postgres:17-alpine", Order: 20},
		{Anchor: "services", Content: "  app:\n    build: .", Order: 10},
	}

	got, err := NewMergeAssembler().Assemble(skeleton, frags)
	if err != nil {
		t.Fatalf("Assemble error: %v", err)
	}

	want := "services:\n" +
		"  app:\n    build: .\n" +
		"  postgres:\n    image: postgres:17-alpine\n"
	if string(got) != want {
		t.Errorf("Assemble =\n%q\nmau\n%q", string(got), want)
	}

	// Marker anchor harus hilang (tergantikan fragmen).
	if strings.Contains(string(got), "region:services") {
		t.Errorf("marker anchor masih ada di hasil:\n%s", got)
	}
}

// TestAssembleStableTieBreak memverifikasi tie-break stabil: dua fragmen ber-Order
// sama mempertahankan urutan input (resolver sudah memfinalkan).
func TestAssembleStableTieBreak(t *testing.T) {
	skeleton := []byte("# region:targets\n")
	frags := []plan.Fragment{
		{Anchor: "targets", Content: "first:", Order: 5},
		{Anchor: "targets", Content: "second:", Order: 5},
	}
	got, err := NewMergeAssembler().Assemble(skeleton, frags)
	if err != nil {
		t.Fatalf("Assemble error: %v", err)
	}
	want := "first:\nsecond:\n"
	if string(got) != want {
		t.Errorf("Assemble tie-break =\n%q\nmau\n%q", string(got), want)
	}
}

// TestAssembleMultipleAnchors memverifikasi KONTRAK assembler multi-anchor (bukan
// detail tata-letak byte yang rapuh, L-3): fragmen tersebar ke anchor yang benar,
// dan anchor yang TIDAK punya fragmen mempertahankan baris marker-nya utuh
// (idempotensi `add service`, ADR-003 D5). Skeleton meniru .env.example dengan tiga
// anchor (app/database/broker); hanya app & database yang diisi.
func TestAssembleMultipleAnchors(t *testing.T) {
	skeleton := []byte("# region:app\n# region:database\n# region:broker\n")
	frags := []plan.Fragment{
		{Anchor: "database", Content: "DB_HOST=localhost", Order: 0},
		{Anchor: "app", Content: "APP_NAME=shop", Order: 0},
	}
	got, err := NewMergeAssembler().Assemble(skeleton, frags)
	if err != nil {
		t.Fatalf("Assemble error: %v", err)
	}
	out := string(got)

	// (1) Anchor terisi: konten fragmen hadir, marker-nya hilang (tergantikan).
	for _, f := range frags {
		if !strings.Contains(out, f.Content) {
			t.Errorf("konten anchor %q (%q) hilang dari hasil:\n%s", f.Anchor, f.Content, out)
		}
		if strings.Contains(out, "region:"+f.Anchor) {
			t.Errorf("marker anchor terisi %q masih ada (mestinya tergantikan fragmen):\n%s", f.Anchor, out)
		}
	}

	// (2) KONTRAK INTI L-3: anchor tanpa fragmen (broker) → baris marker DIPERTAHANKAN
	// persis (tidak dihapus, tidak diisi). Ini eksplisit, bukan disimpulkan dari
	// pencocokan byte penuh.
	if !strings.Contains(out, "# region:broker") {
		t.Errorf("anchor tanpa fragmen (broker) harus mempertahankan baris marker, hasil:\n%s", out)
	}
}

// TestAssembleEmptyAnchorRetainsMarker memverifikasi anchor tanpa fragmen
// mempertahankan baris marker (idempotensi `add service`, ADR-003 D5).
func TestAssembleEmptyAnchorRetainsMarker(t *testing.T) {
	skeleton := []byte("services:\n  # region:services\nvolumes:\n  # region:volumes\n")
	got, err := NewMergeAssembler().Assemble(skeleton, nil)
	if err != nil {
		t.Fatalf("Assemble error: %v", err)
	}
	if string(got) != string(skeleton) {
		t.Errorf("Assemble tanpa fragmen mengubah skeleton:\n%q", string(got))
	}
}

// TestAssembleIdempotent memverifikasi menjalankan Assemble dua kali atas konten
// yang sama menghasilkan byte identik (determinisme, ADR-002 §6).
func TestAssembleIdempotent(t *testing.T) {
	skeleton := []byte("services:\n  # region:services\n")
	frags := []plan.Fragment{
		{Anchor: "services", Content: "  app:\n    build: .", Order: 10},
	}
	a := NewMergeAssembler()
	first, err := a.Assemble(skeleton, frags)
	if err != nil {
		t.Fatalf("Assemble pertama error: %v", err)
	}
	second, err := a.Assemble(skeleton, frags)
	if err != nil {
		t.Fatalf("Assemble kedua error: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("Assemble tidak idempoten:\n%q\nvs\n%q", string(first), string(second))
	}
}

// TestAssembleGoComment memverifikasi marker gaya Go ("// region:wiring")
// dikenali sama seperti gaya "#".
func TestAssembleGoComment(t *testing.T) {
	skeleton := []byte("func main() {\n\t// region:wiring\n}\n")
	frags := []plan.Fragment{
		{Anchor: "wiring", Content: "\trouter := NewRouter()", Order: 0},
	}
	got, err := NewMergeAssembler().Assemble(skeleton, frags)
	if err != nil {
		t.Fatalf("Assemble error: %v", err)
	}
	want := "func main() {\n\trouter := NewRouter()\n}\n"
	if string(got) != want {
		t.Errorf("Assemble Go-comment =\n%q\nmau\n%q", string(got), want)
	}
}

// TestAssembleOrphanFragmentErrors memverifikasi fragmen dengan anchor yang tidak
// ada di skeleton → error (mencegah fragmen hilang diam-diam).
func TestAssembleOrphanFragmentErrors(t *testing.T) {
	skeleton := []byte("# region:services\n")
	frags := []plan.Fragment{
		{Anchor: "tidak-ada", Content: "x", Order: 0},
	}
	if _, err := NewMergeAssembler().Assemble(skeleton, frags); err == nil {
		t.Fatal("Assemble fragmen yatim: mau error, dapat nil")
	}
}

// TestAssembleEmptyAnchorNameErrors memverifikasi fragmen tanpa Anchor → error.
func TestAssembleEmptyAnchorNameErrors(t *testing.T) {
	skeleton := []byte("# region:services\n")
	frags := []plan.Fragment{{Anchor: "", Content: "x", Order: 0}}
	if _, err := NewMergeAssembler().Assemble(skeleton, frags); err == nil {
		t.Fatal("Assemble fragmen tanpa Anchor: mau error, dapat nil")
	}
}

// TestAssembleSQLComment memverifikasi marker gaya SQL ("-- region:seed") dikenali
// sama seperti gaya "#" / "//" (prefix komentar SQL untuk file migrasi).
func TestAssembleSQLComment(t *testing.T) {
	skeleton := []byte("-- migrasi awal\n-- region:seed\n")
	frags := []plan.Fragment{
		{Anchor: "seed", Content: "INSERT INTO t VALUES (1);", Order: 0},
	}
	got, err := NewMergeAssembler().Assemble(skeleton, frags)
	if err != nil {
		t.Fatalf("Assemble SQL error: %v", err)
	}
	want := "-- migrasi awal\nINSERT INTO t VALUES (1);\n"
	if string(got) != want {
		t.Errorf("Assemble SQL-comment =\n%q\nmau\n%q", string(got), want)
	}
}

// TestAssembleIndentedMarkerPreservesContent memverifikasi marker ber-INDENTASI
// (mis. "  # region:services" di YAML) dikenali, dan fragmen membawa indentasinya
// sendiri (marker diganti utuh oleh konten fragmen).
func TestAssembleIndentedMarkerPreservesContent(t *testing.T) {
	skeleton := []byte("services:\n    # region:services\n")
	frags := []plan.Fragment{
		{Anchor: "services", Content: "    app:\n      build: .", Order: 0},
	}
	got, err := NewMergeAssembler().Assemble(skeleton, frags)
	if err != nil {
		t.Fatalf("Assemble indented error: %v", err)
	}
	want := "services:\n    app:\n      build: .\n"
	if string(got) != want {
		t.Errorf("Assemble indented marker =\n%q\nmau\n%q", string(got), want)
	}
}

// TestAssembleNonAnchorCommentUntouched memverifikasi baris komentar yang BUKAN
// anchor ("# region:" wajib; "# komentar biasa" bukan) disalin apa adanya.
func TestAssembleNonAnchorCommentUntouched(t *testing.T) {
	skeleton := []byte("# komentar biasa\n# region:app\n# bukan-region: x\n")
	frags := []plan.Fragment{{Anchor: "app", Content: "APP=1", Order: 0}}
	got, err := NewMergeAssembler().Assemble(skeleton, frags)
	if err != nil {
		t.Fatalf("Assemble error: %v", err)
	}
	want := "# komentar biasa\nAPP=1\n# bukan-region: x\n"
	if string(got) != want {
		t.Errorf("Assemble non-anchor komentar =\n%q\nmau\n%q", string(got), want)
	}
}

// TestAssembleAnchorWithDescriptionIgnoresTail memverifikasi keterangan setelah
// nama anchor (mis. "# region:services (db-postgres) — daftar service") diabaikan;
// nama anchor = token pertama setelah "region:". keep HANYA bila token PERSIS "keep".
func TestAssembleAnchorWithDescriptionIgnoresTail(t *testing.T) {
	skeleton := []byte("# region:services daftar service compose\n")
	frags := []plan.Fragment{{Anchor: "services", Content: "  app: {}", Order: 0}}
	got, err := NewMergeAssembler().Assemble(skeleton, frags)
	if err != nil {
		t.Fatalf("Assemble error: %v", err)
	}
	// Keterangan bukan "keep" → marker DIKONSUMSI (hilang).
	if strings.Contains(string(got), "region:services") {
		t.Errorf("keterangan non-keep harus tetap mengonsumsi marker:\n%s", got)
	}
	if !strings.Contains(string(got), "app: {}") {
		t.Errorf("fragmen harus tersisip:\n%s", got)
	}
}

// TestAssemblePreservesTrailingNewlineState memverifikasi rekonstruksi akhiran:
// skeleton TANPA newline akhir → output juga tanpa newline akhir (byte-stabil).
func TestAssemblePreservesTrailingNewlineState(t *testing.T) {
	// Skeleton tanpa newline akhir; anchor di baris terakhir tanpa \n.
	skeleton := []byte("a\n# region:x")
	frags := []plan.Fragment{{Anchor: "x", Content: "B", Order: 0}}
	got, err := NewMergeAssembler().Assemble(skeleton, frags)
	if err != nil {
		t.Fatalf("Assemble error: %v", err)
	}
	want := "a\nB"
	if string(got) != want {
		t.Errorf("Assemble tanpa newline akhir = %q, mau %q", string(got), want)
	}
}

// TestAssembleKeepFlagPreservesMarker memverifikasi flag "keep" pada baris anchor
// (mis. "# region:services keep") MEMPERTAHANKAN marker SETELAH fragmen disisipkan —
// agar penyisipan inkremental berikutnya (`add service`) tetap menemukan titik-sisip
// yang sama (idempotensi multi-add, M-2). Anchor tanpa flag tetap mengonsumsi marker.
func TestAssembleKeepFlagPreservesMarker(t *testing.T) {
	skeleton := []byte("services:\n  # region:services keep\n")
	frags := []plan.Fragment{
		{Anchor: "services", Content: "  order:\n    build: .", Order: 10},
	}
	got, err := NewMergeAssembler().Assemble(skeleton, frags)
	if err != nil {
		t.Fatalf("Assemble error: %v", err)
	}
	gotStr := string(got)
	if !strings.Contains(gotStr, "order:") {
		t.Errorf("fragmen harus tersisip:\n%s", gotStr)
	}
	// Marker DIPERTAHANKAN (keep) — fragmen DULU, lalu baris marker.
	if !strings.Contains(gotStr, "region:services") {
		t.Errorf("flag keep harus MEMPERTAHANKAN marker setelah fragmen:\n%s", gotStr)
	}
	want := "services:\n  order:\n    build: .\n  # region:services keep\n"
	if gotStr != want {
		t.Errorf("Assemble(keep) =\n%q\nmau\n%q", gotStr, want)
	}

	// Re-merge atas hasil (idempotensi add-service): marker masih ada → fragmen
	// kedua dapat disisip; tak menggandakan fragmen pertama.
	frags2 := []plan.Fragment{{Anchor: "services", Content: "  user:\n    build: .", Order: 10}}
	got2, err := NewMergeAssembler().Assemble(got, frags2)
	if err != nil {
		t.Fatalf("Assemble kedua error: %v", err)
	}
	g2 := string(got2)
	if strings.Count(g2, "order:") != 1 {
		t.Errorf("order tak boleh tergandakan pada merge kedua:\n%s", g2)
	}
	if !strings.Contains(g2, "user:") || !strings.Contains(g2, "region:services") {
		t.Errorf("merge kedua harus menambah user & mempertahankan marker:\n%s", g2)
	}
}
