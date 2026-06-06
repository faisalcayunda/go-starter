// addservice.go — resolusi inkremental `gostarter add service <name>` (US-05).
//
// Berbeda dari Resolve (create), add-service TIDAK meng-emit file root atau go.mod
// dan TIDAK menjalankan EnsureEmptyDir — project microservice sudah ada. Ia hanya:
//   - merakit []plan.FileOp untuk SATU service baru dari template PER-SERVICE
//     arch-microservice (proto + services/<name>/**), memakai DataOverride yang
//     sama dengan jalur create (Service/IsFirst/Others/Downstreams/GrpcPort/
//     GatewayPort/ModulePath) — service baru SELALU IsFirst=false (tak punya HTTP
//     gateway; gateway.go di-skip via when .IsFirst), Others/Downstreams kosong
//     (service tambahan = gRPC server murni, belum memanggil service lain);
//   - menghasilkan SATU plan.Fragment compose (anchor region:services) untuk
//     service baru, agar lapis CLI menyisipkannya ke docker-compose.yml on-disk
//     lewat MergeAssembler (anchor region:services dipertahankan, idempoten).
//
// Jalur ini SATU pipeline dengan create (resolver→generator), bukan island:
// generator.Generate-lah yang menulis FileOp (containment via fsutil.JoinTarget,
// H-1) — tak ada string-concat path mentah. Port deterministik per INDEX service
// (grpcPortBase+idx) sesuai serviceData create (L-3).

package resolver

import (
	"fmt"
	"sort"

	"github.com/faisalcayunda/gostarter/internal/answers"
	"github.com/faisalcayunda/gostarter/internal/module"
	"github.com/faisalcayunda/gostarter/internal/plan"
)

// AddServiceInfo adalah input ResolveAddService: metadata project microservice
// existing (terbaca dari go.mod + scan services/ oleh lapis CLI) + nama service baru.
type AddServiceInfo struct {
	// ModulePath adalah go module path project (dari go.mod). Dipakai untuk import
	// path gen/go & libs pada template per-service (H-2: nama project/module dari
	// go.mod, BUKAN filepath.Base(projectDir) yang bisa "." bila dijalankan di cwd).
	ModulePath string
	// GoVersion adalah go directive project (dari go.mod). Diteruskan ke data render
	// agar template per-service konsisten dengan project existing.
	GoVersion string
	// ServiceName adalah nama service BARU (dari ARG CLI; sudah lolos validasi
	// regex/reserved/duplikat di lapis CLI). Bukan diturunkan dari direktori.
	ServiceName string
	// Index adalah indeks alokasi port service baru: jumlah service existing
	// (service baru = service ke-(N), port gRPC = grpcPortBase + Index). Deterministik
	// dari jumlah service existing — DIHITUNG SEKALI oleh CLI, bukan saat render (L-3).
	Index int
}

// AddServicePlan adalah keluaran ResolveAddService: file baru + kontribusi compose
// untuk satu service. Lapis CLI mengeksekusi Files via generator.Generate (tanpa
// EnsureEmptyDir / tanpa go.mod) dan menyisipkan Compose ke compose on-disk via
// MergeAssembler.
type AddServicePlan struct {
	// Files adalah FileOp per-service (render) untuk service baru, terurut
	// deterministik by TargetPath.
	Files []plan.FileOp
	// Compose adalah fragmen compose (anchor region:services) untuk service baru —
	// SATU fragment. Content = PATH template fragmen (render ditunda ke generator/
	// CLI, konsisten dgn jalur create), DataOverride = data per-service.
	Compose plan.Fragment
	// ComposeTemplatePath adalah path embed.FS template fragmen compose (untuk
	// di-render oleh CLI sebelum sisip).
	ComposeTemplatePath string
}

// ResolveAddService merakit AddServicePlan untuk satu service baru. Memakai modul
// arch-microservice dari registry sebagai SATU-SATUNYA sumber template per-service
// (jalur terpadu — tak ada embed tmpl island).
func (r *resolver) ResolveAddService(info AddServiceInfo) (AddServicePlan, error) {
	if info.ServiceName == "" {
		return AddServicePlan{}, fmt.Errorf("%w: nama service baru kosong", ErrConstraint)
	}
	if info.ModulePath == "" {
		return AddServicePlan{}, fmt.Errorf("%w: module path project kosong (go.mod tak terbaca?)", ErrConstraint)
	}

	m, ok := r.reg.Get(modArchMicro)
	if !ok {
		return AddServicePlan{}, fmt.Errorf("%w: modul %q tidak ditemukan di registry (katalog tidak lengkap)", ErrConstraint, modArchMicro)
	}

	goVer := info.GoVersion
	if goVer == "" {
		goVer = goVersionMicroservice
	}

	// Data per-service service baru. IsFirst=false (project existing sudah punya
	// service pertama), Others/Downstreams kosong (service baru hanya menyediakan
	// gRPC; inter-service call ditambahkan manual oleh user kelak). Port = base+Index.
	override := addServiceData(info)

	// Data global minimal untuk evaluasi placeholder target & render (ModulePath,
	// GoVersion, Comm grpc). Cukup untuk template per-service yang membaca .ModulePath
	// & field per-service via DataOverride.
	data := map[string]any{
		"ModulePath": info.ModulePath,
		"Module":     info.ModulePath,
		"GoVersion":  goVer,
		"Comm":       string(answers.CommGRPC),
		"Gateway":    false,
	}

	files, err := r.addServiceFiles(m, data, override)
	if err != nil {
		return AddServicePlan{}, err
	}

	// Kontribusi compose: ambil dari Contributes manifest yang menargetkan anchor
	// "services" pada docker-compose.yml (sumber kebenaran tunggal — bukan path
	// hard-coded). Render ditunda: Content = PATH fragmen (di-render CLI).
	composeFrag, composeTmpl, err := r.addServiceCompose(m, override)
	if err != nil {
		return AddServicePlan{}, err
	}

	return AddServicePlan{
		Files:               files,
		Compose:             composeFrag,
		ComposeTemplatePath: composeTmpl,
	}, nil
}

// addServiceData membangun DataOverride per-service untuk service BARU. Konsisten
// dgn serviceData (create) tetapi IsFirst selalu false & Others/Downstreams kosong.
func addServiceData(info AddServiceInfo) map[string]any {
	return map[string]any{
		"Service":     info.ServiceName,
		"IsFirst":     false,
		"Others":      []string{},
		"Downstreams": []map[string]any{},
		"ModulePath":  info.ModulePath,
		"GrpcPort":    grpcPortBase + info.Index,
		"GatewayPort": httpGatewayBase,
	}
}

// addServiceFiles meng-emit FileOp per-service (target ber-{{ .Service }}) untuk
// service baru, melewati `when` per-service (gateway.go di-skip karena .IsFirst
// false). File ROOT (tanpa placeholder service) DIABAIKAN — add-service tak menulis
// ulang file root. Urutan akhir distabilkan by TargetPath (determinisme).
func (r *resolver) addServiceFiles(m module.Manifest, data, override map[string]any) ([]plan.FileOp, error) {
	var ops []plan.FileOp
	for _, f := range m.Files {
		// Hanya file PER-SERVICE (placeholder {{ .Service }}) yang relevan untuk
		// add-service; file root di-skip.
		if !isPerServiceTarget(f.Target) {
			continue
		}

		// `when` per-service dgn overlay (mendukung .IsFirst) — gateway.go (.IsFirst)
		// otomatis di-skip untuk service baru.
		ok, werr := evalWhenService(f.When, answers.Answers{Arch: answers.ArchMicroservice}, override)
		if werr != nil {
			return nil, fmt.Errorf("modul %q file %q (add-service %q): %w", m.Name, f.Target, override["Service"], werr)
		}
		if !ok {
			continue
		}

		// Render placeholder target dgn data global + override (target butuh .Service).
		merged := make(map[string]any, len(data)+len(override))
		for k, v := range data {
			merged[k] = v
		}
		for k, v := range override {
			merged[k] = v
		}
		target, terr := renderTargetPath(f.Target, merged)
		if terr != nil {
			return nil, fmt.Errorf("modul %q file %q (add-service): %w", m.Name, f.Target, terr)
		}
		if serr := checkSafeTargetPath(target); serr != nil {
			return nil, fmt.Errorf("%w: modul %q file %q (add-service): %v", ErrConstraint, m.Name, f.Target, serr)
		}

		ops = append(ops, plan.FileOp{
			Mode:         modeFromString(f.Mode),
			TargetPath:   target,
			ModuleName:   m.Name,
			TemplatePath: joinModulePath(m.Name, f.Template),
			Perm:         permFor(f.Mode),
			Data:         data,
			DataOverride: override,
		})
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("%w: modul %q tak punya file per-service untuk add-service", ErrConstraint, m.Name)
	}
	// Urutan deterministik by TargetPath (selaras buildFiles, idempotensi ADR-002 §6).
	sort.SliceStable(ops, func(i, j int) bool { return ops[i].TargetPath < ops[j].TargetPath })
	return ops, nil
}

// addServiceCompose mengambil kontribusi compose (anchor "services") dari manifest
// arch-microservice dan membungkusnya menjadi SATU plan.Fragment untuk service
// baru. Content diisi PATH fragmen (render ditunda ke CLI); DataOverride = data
// per-service. Mengembalikan (fragment, templatePath, error).
func (r *resolver) addServiceCompose(m module.Manifest, override map[string]any) (plan.Fragment, string, error) {
	for _, c := range m.Contributes {
		if c.Target != "docker-compose.yml" || c.Anchor != "services" {
			continue
		}
		tmplPath := joinModulePath(m.Name, c.Fragment)
		return plan.Fragment{
			Anchor:       c.Anchor,
			Content:      tmplPath,
			Order:        c.Order,
			DataOverride: override,
		}, tmplPath, nil
	}
	return plan.Fragment{}, "", fmt.Errorf("%w: modul %q tak punya kontribusi compose region:services", ErrConstraint, m.Name)
}
