package resolver

import (
	"strings"
	"testing"

	"github.com/faisalcayunda/gostarter/internal/plan"
)

// ── Jalur terpadu microservice: GoVersion + alokasi port per-service (L-3) ─────

// TestResolve_Microservice_GoVersion125 memverifikasi arch=microservice memilih go
// directive 1.25 (grpc v1.81.1 butuh go ≥ 1.25) — plan.GoVersion DAN data["GoVersion"]
// konsisten (byte-identical). monolith tetap 1.24.
func TestResolve_Microservice_GoVersion125(t *testing.T) {
	r := New(mvpRegistry())
	p, err := r.Resolve(microAnswers("svc-a", "svc-b"))
	if err != nil {
		t.Fatalf("Resolve microservice gagal: %v", err)
	}
	if p.GoVersion != "1.25" {
		t.Errorf("microservice GoVersion = %q, mau 1.25", p.GoVersion)
	}
	// data["GoVersion"] pada FileOp root harus SAMA dengan plan.GoVersion.
	readme, ok := fileOpByTarget(p.Files, "README.md")
	if !ok {
		t.Fatalf("README.md harus ada")
	}
	if data, ok := readme.Data.(map[string]any); ok {
		if data["GoVersion"] != "1.25" {
			t.Errorf("data[GoVersion] = %v, mau 1.25 (selaras plan)", data["GoVersion"])
		}
	} else {
		t.Fatalf("README.md Data bukan map[string]any")
	}

	// monolith tetap 1.24 (tak terpengaruh perubahan microservice).
	pm, err := r.Resolve(baseAnswers())
	if err != nil {
		t.Fatalf("Resolve monolith gagal: %v", err)
	}
	if pm.GoVersion != "1.24" {
		t.Errorf("monolith GoVersion = %q, mau 1.24", pm.GoVersion)
	}
}

// TestResolve_Microservice_PortAllocation memverifikasi alokasi port DETERMINISTIK
// per INDEX service pada sortedServiceNames (L-3): gRPC service ke-i = grpcPortBase+i;
// GatewayPort = httpGatewayBase untuk semua. Downstreams memuat port CALLEE (bukan
// caller). Diperiksa via DataOverride FileOp per-service (proto/main).
func TestResolve_Microservice_PortAllocation(t *testing.T) {
	r := New(mvpRegistry())
	// svc-a (idx 0 → 50051), svc-b (idx 1 → 50052), svc-c (idx 2 → 50053).
	p, err := r.Resolve(microAnswers("svc-b", "svc-c", "svc-a")) // urutan input diacak
	if err != nil {
		t.Fatalf("Resolve gagal: %v", err)
	}

	wantPort := map[string]int{
		"services/svc-a/cmd/main.go": grpcPortBase + 0,
		"services/svc-b/cmd/main.go": grpcPortBase + 1,
		"services/svc-c/cmd/main.go": grpcPortBase + 2,
	}
	for target, want := range wantPort {
		op, ok := fileOpByTarget(p.Files, target)
		if !ok {
			t.Errorf("%q tidak ada di plan", target)
			continue
		}
		got, _ := op.DataOverride["GrpcPort"].(int)
		if got != want {
			t.Errorf("%q GrpcPort = %d, mau %d (deterministik by index sortedServiceNames)", target, got, want)
		}
		if gp, _ := op.DataOverride["GatewayPort"].(int); gp != httpGatewayBase {
			t.Errorf("%q GatewayPort = %d, mau %d", target, gp, httpGatewayBase)
		}
	}

	// Downstreams service pertama (svc-a) harus memuat svc-b:50052 & svc-c:50053
	// (port CALLEE masing-masing — bukan port svc-a).
	mainA, _ := fileOpByTarget(p.Files, "services/svc-a/cmd/main.go")
	ds, ok := mainA.DataOverride["Downstreams"].([]map[string]any)
	if !ok {
		t.Fatalf("svc-a Downstreams bukan []map[string]any: %T", mainA.DataOverride["Downstreams"])
	}
	gotPorts := map[string]int{}
	for _, d := range ds {
		name, _ := d["Name"].(string)
		port, _ := d["Port"].(int)
		gotPorts[name] = port
	}
	if gotPorts["svc-b"] != grpcPortBase+1 || gotPorts["svc-c"] != grpcPortBase+2 {
		t.Errorf("svc-a Downstreams ports salah: %v (mau svc-b=%d svc-c=%d)", gotPorts, grpcPortBase+1, grpcPortBase+2)
	}
}

// ── ResolveAddService (US-05, jalur terpadu) ──────────────────────────────────

// TestResolveAddService_FilesAndPort memverifikasi add-service meng-emit HANYA file
// per-service untuk service baru (TANPA file root, TANPA gateway.go karena IsFirst
// false), dengan port = grpcPortBase + Index, terurut by TargetPath, dan kontribusi
// compose ke anchor services.
func TestResolveAddService_FilesAndPort(t *testing.T) {
	r := New(mvpRegistry())
	ap, err := r.ResolveAddService(AddServiceInfo{
		ModulePath:  "github.com/acme/platform",
		GoVersion:   "1.25",
		ServiceName: "payment",
		Index:       2,
	})
	if err != nil {
		t.Fatalf("ResolveAddService gagal: %v", err)
	}
	if len(ap.Files) == 0 {
		t.Fatalf("add-service harus meng-emit file per-service")
	}

	// Semua file di bawah services/payment atau proto/payment; tak ada file root,
	// tak ada gateway.go (service baru bukan IsFirst).
	for _, op := range ap.Files {
		if strings.Contains(op.TargetPath, "gateway") {
			t.Errorf("add-service TIDAK boleh emit gateway.go (IsFirst false): %q", op.TargetPath)
		}
		if !strings.HasPrefix(op.TargetPath, "services/payment/") &&
			!strings.HasPrefix(op.TargetPath, "proto/payment/") {
			t.Errorf("file add-service di luar subtree payment: %q", op.TargetPath)
		}
		// DataOverride port = base + Index.
		if gp, _ := op.DataOverride["GrpcPort"].(int); gp != grpcPortBase+2 {
			t.Errorf("%q GrpcPort = %d, mau %d (base+Index)", op.TargetPath, gp, grpcPortBase+2)
		}
		if isFirst, _ := op.DataOverride["IsFirst"].(bool); isFirst {
			t.Errorf("%q IsFirst harus false untuk service tambahan", op.TargetPath)
		}
	}

	// Urutan deterministik by TargetPath.
	for i := 1; i < len(ap.Files); i++ {
		if ap.Files[i-1].TargetPath > ap.Files[i].TargetPath {
			t.Errorf("FileOp add-service tidak terurut: %q > %q", ap.Files[i-1].TargetPath, ap.Files[i].TargetPath)
		}
	}

	// Kontribusi compose: anchor services, DataOverride.Service = payment.
	if ap.Compose.Anchor != "services" {
		t.Errorf("compose anchor = %q, mau services", ap.Compose.Anchor)
	}
	if svc, _ := ap.Compose.DataOverride["Service"].(string); svc != "payment" {
		t.Errorf("compose DataOverride.Service = %q, mau payment", svc)
	}
	if ap.ComposeTemplatePath == "" {
		t.Errorf("ComposeTemplatePath kosong (CLI butuh path fragmen untuk render)")
	}
}

// TestResolveAddService_RejectEmptyAndMissingModule memverifikasi guard input.
func TestResolveAddService_RejectEmptyAndMissingModule(t *testing.T) {
	r := New(mvpRegistry())
	if _, err := r.ResolveAddService(AddServiceInfo{ServiceName: "", ModulePath: "x"}); err == nil {
		t.Errorf("nama service kosong harus ditolak")
	}
	if _, err := r.ResolveAddService(AddServiceInfo{ServiceName: "order", ModulePath: ""}); err == nil {
		t.Errorf("module path kosong harus ditolak")
	}
}

// TestResolveAddService_GoVersionDefault memverifikasi GoVersion kosong jatuh ke
// default microservice (1.25).
func TestResolveAddService_GoVersionDefault(t *testing.T) {
	r := New(mvpRegistry())
	ap, err := r.ResolveAddService(AddServiceInfo{
		ModulePath:  "github.com/acme/platform",
		ServiceName: "order",
		Index:       1,
	})
	if err != nil {
		t.Fatalf("ResolveAddService gagal: %v", err)
	}
	// Data global FileOp memuat GoVersion default microservice.
	var anyOp plan.FileOp
	for _, op := range ap.Files {
		anyOp = op
		break
	}
	if data, ok := anyOp.Data.(map[string]any); ok {
		if data["GoVersion"] != goVersionMicroservice {
			t.Errorf("GoVersion default = %v, mau %q", data["GoVersion"], goVersionMicroservice)
		}
	} else {
		t.Fatalf("FileOp.Data bukan map[string]any")
	}
}
