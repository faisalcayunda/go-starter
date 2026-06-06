package hooks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestByName_KnownHooks memverifikasi keempat hook kanonik terdaftar di registry
// dengan nama yang tepat, dan nama tak dikenal mengembalikan ok=false.
func TestByName_KnownHooks(t *testing.T) {
	cases := map[string]string{
		NameBufGenerate: "buf-generate",
		NameGofmt:       "gofmt",
		NameGoModTidy:   "go-mod-tidy",
		NameGitInit:     "git-init",
	}
	for name, want := range cases {
		h, ok := ByName(name)
		if !ok {
			t.Errorf("ByName(%q) ok=false; hook mestinya terdaftar", name)
			continue
		}
		if h.Name() != want {
			t.Errorf("ByName(%q).Name()=%q; mau %q", name, h.Name(), want)
		}
	}
	if _, ok := ByName("tidak-ada"); ok {
		t.Error("ByName(nama tak dikenal) ok=true; mau false")
	}
}

// TestOrderConstants_BufGenerateFirst memverifikasi urutan kanonik order:
// BufGenerate(5) → Gofmt(10) → GoModTidy(20) → GitInit(30).
func TestOrderConstants_BufGenerateFirst(t *testing.T) {
	if OrderBufGenerate >= OrderGofmt || OrderGofmt >= OrderGoModTidy || OrderGoModTidy >= OrderGitInit {
		t.Errorf("urutan order salah: buf=%d gofmt=%d tidy=%d git=%d",
			OrderBufGenerate, OrderGofmt, OrderGoModTidy, OrderGitInit)
	}
	if OrderBufGenerate != 5 {
		t.Errorf("OrderBufGenerate=%d; mau 5", OrderBufGenerate)
	}
}

// TestBuild_OrdersAndWarnOnly memverifikasi Build: (a) mengurutkan plan by Order
// menaik sehingga BufGenerate jalan PALING DULU, (b) hanya GitInit yang warn-only —
// BufGenerate/Gofmt/GoModTidy fail-fast.
func TestBuild_OrdersAndWarnOnly(t *testing.T) {
	// Sengaja acak urutan input untuk membuktikan Build yang mengurutkan.
	specs := []Spec{
		{Name: NameGitInit, Order: OrderGitInit},
		{Name: NameGoModTidy, Order: OrderGoModTidy},
		{Name: NameBufGenerate, Order: OrderBufGenerate},
		{Name: NameGofmt, Order: OrderGofmt},
	}
	plans := Build(specs)
	if len(plans) != 4 {
		t.Fatalf("Build menghasilkan %d plan; mau 4", len(plans))
	}

	wantOrder := []string{NameBufGenerate, NameGofmt, NameGoModTidy, NameGitInit}
	for i, want := range wantOrder {
		if plans[i].Hook.Name() != want {
			t.Errorf("plan[%d].Name()=%q; mau %q (urutan by Order)", i, plans[i].Hook.Name(), want)
		}
	}

	// Hanya GitInit warn-only.
	for _, p := range plans {
		wantWarn := p.Hook.Name() == NameGitInit
		if p.WarnOnly != wantWarn {
			t.Errorf("hook %q WarnOnly=%v; mau %v", p.Hook.Name(), p.WarnOnly, wantWarn)
		}
	}
}

// TestBuild_SkipsUnknownHook memverifikasi Build mengabaikan nama hook tak dikenal
// (resolver hanya mengisi nama kanonik; pertahanan defensif).
func TestBuild_SkipsUnknownHook(t *testing.T) {
	plans := Build([]Spec{
		{Name: NameGofmt, Order: OrderGofmt},
		{Name: "hook-asing", Order: 99},
	})
	if len(plans) != 1 || plans[0].Hook.Name() != NameGofmt {
		t.Fatalf("Build tidak mengabaikan hook tak dikenal: %+v", plans)
	}
}

// TestBufGenerate_MissingBufClearError memverifikasi: bila "buf" tidak ada di PATH,
// BufGenerate.Run mengembalikan error JELAS yang menyertakan perintah instalasi.
//
// L-5: PATH di-arahkan ke direktori temporer KOSONG (bukan dikosongkan total) lewat
// t.Setenv. t.Setenv:
//   - mengembalikan nilai PATH semula otomatis saat test selesai (tidak membocorkan
//     state ke test lain), dan
//   - memanggil t.Parallel() pada test ini akan panic, sehingga mutasi env proses
//     ini dijamin tidak overlap dengan test concurrent lain.
//
// Mengarahkan ke dir kosong (bukan "") lebih deterministik: LookPath("buf") menelusuri
// entri PATH yang ada namun tak memuat binary apa pun → pasti gagal, tanpa bergantung
// pada perilaku tepi "PATH kosong".
func TestBufGenerate_MissingBufClearError(t *testing.T) {
	emptyDir := t.TempDir()    // dir nyata namun dijamin tak berisi binary "buf"
	t.Setenv("PATH", emptyDir) // restore otomatis; t.Parallel() terlarang di test ini
	err := BufGenerate{}.Run(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("BufGenerate.Run tanpa buf di PATH: mau error, dapat nil")
	}
	msg := err.Error()
	for _, want := range []string{
		"microservice butuh buf",
		"go install github.com/bufbuild/buf/cmd/buf@latest",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("pesan error tidak memuat %q; isi: %q", want, msg)
		}
	}
}

// failHook adalah PostGenHook palsu yang selalu gagal — untuk menguji fail-fast
// vs warn-only di Run.
type failHook struct{ name string }

func (h failHook) Name() string                    { return h.name }
func (failHook) Run(context.Context, string) error { return errors.New("boom") }

// TestRun_FailFastNonWarnOnly memverifikasi Run berhenti & mengembalikan error pada
// hook non-warn-only yang gagal (mensimulasikan BufGenerate/Gofmt/GoModTidy gagal),
// dan hook setelahnya TIDAK dijalankan.
func TestRun_FailFastNonWarnOnly(t *testing.T) {
	var ran []string
	track := func(name string) PostGenHook { return trackHook{name: name, ran: &ran} }

	plans := []Plan{
		{Hook: failHook{name: NameBufGenerate}, Order: OrderBufGenerate, WarnOnly: false},
		{Hook: track("gofmt"), Order: OrderGofmt, WarnOnly: false},
	}
	err := Run(context.Background(), t.TempDir(), plans, nil)
	if err == nil {
		t.Fatal("Run dengan hook fail-fast gagal: mau error, dapat nil")
	}
	if !strings.Contains(err.Error(), NameBufGenerate) {
		t.Errorf("error tidak menyebut hook yang gagal: %v", err)
	}
	if len(ran) != 0 {
		t.Errorf("hook setelah kegagalan fail-fast tetap jalan: %v", ran)
	}
}

// TestRun_WarnOnlyContinues memverifikasi kegagalan hook warn-only (GitInit) TIDAK
// membatalkan Run; warnf dipanggil dan hook berikutnya tetap jalan. L-4: pesan
// warning DIFORMAT (fmt.Sprintf) lalu diverifikasi memuat nama hook yang gagal
// (NameGitInit) — operator harus tahu hook MANA yang di-skip, bukan sekadar "ada
// warning".
func TestRun_WarnOnlyContinues(t *testing.T) {
	var ran []string
	var warnMsg string
	warned := false
	warnf := func(format string, args ...any) {
		warned = true
		warnMsg = fmt.Sprintf(format, args...)
	}

	plans := []Plan{
		{Hook: failHook{name: NameGitInit}, Order: OrderGitInit, WarnOnly: true},
		{Hook: trackHook{name: "after", ran: &ran}, Order: 99, WarnOnly: false},
	}
	if err := Run(context.Background(), t.TempDir(), plans, warnf); err != nil {
		t.Fatalf("Run dengan warn-only gagal tidak boleh error: %v", err)
	}
	if !warned {
		t.Error("warnf tidak dipanggil untuk hook warn-only yang gagal")
	}
	if !strings.Contains(warnMsg, NameGitInit) {
		t.Errorf("pesan warning harus memuat nama hook %q yang di-skip, dapat: %q", NameGitInit, warnMsg)
	}
	if len(ran) != 1 || ran[0] != "after" {
		t.Errorf("hook setelah warn-only tidak jalan: %v", ran)
	}
}

// trackHook adalah PostGenHook palsu yang mencatat eksekusinya & selalu sukses.
type trackHook struct {
	name string
	ran  *[]string
}

func (h trackHook) Name() string { return h.name }
func (h trackHook) Run(context.Context, string) error {
	*h.ran = append(*h.ran, h.name)
	return nil
}
