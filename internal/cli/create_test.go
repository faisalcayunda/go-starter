package cli

import (
	"slices"
	"testing"

	"github.com/faisalcayunda/gostarter/internal/answers"
)

// TestResolveCI_Gating memverifikasi BLK-1: modul CI HANYA aktif bila add-on 'ci'
// dipilih. `--ci <provider>` SENDIRIAN (tanpa --addons ci) TIDAK boleh menyetel CI
// ke provider apa pun — ia harus tetap CINone agar resolver.activeModules tidak
// mengaktifkan addon-ci (yang di-gate pada CI ∈ {github-actions, gitlab-ci}).
//
// Urutan cek di resolveCI: ciAddonActive di-gate dulu (return CINone bila mati),
// baru menghormati ciFlag, baru default provider.
func TestResolveCI_Gating(t *testing.T) {
	cases := []struct {
		name          string
		ciAddonActive bool
		ciFlag        string
		want          answers.CI
	}{
		// Add-on ci TIDAK aktif → selalu CINone, apa pun --ci. Inilah inti BLK-1:
		// --ci tanpa --addons ci tidak mengaktifkan modul CI.
		{"addon off, ci flag kosong", false, "", answers.CINone},
		{"addon off, ci=github-actions", false, "github-actions", answers.CINone},
		{"addon off, ci=gitlab-ci", false, "gitlab-ci", answers.CINone},
		{"addon off, ci=none", false, "none", answers.CINone},

		// Add-on ci aktif → provider berasal dari --ci; kosong → default github.
		{"addon on, ci flag kosong → default github", true, "", answers.CIGitHubActions},
		{"addon on, ci=github-actions", true, "github-actions", answers.CIGitHubActions},
		{"addon on, ci=gitlab-ci", true, "gitlab-ci", answers.CIGitLabCI},
		// Nilai apa adanya diteruskan (validasi enum = answers.Validate).
		{"addon on, ci=none (eksplisit)", true, "none", answers.CINone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCI(tc.ciAddonActive, tc.ciFlag)
			if got != tc.want {
				t.Errorf("resolveCI(%v, %q) = %q, mau %q", tc.ciAddonActive, tc.ciFlag, got, tc.want)
			}
		})
	}
}

// TestMapPresetToFlags_CIAddon mengunci regresi byte-identical (SPEC §5.2):
// preset `ci: <provider>` HARUS turut mengaktifkan add-on 'ci' di presetAddons,
// persis seperti jalur flag yang menyebut `ci` di --feature/--addons. Tanpa ini,
// `gostarter create --config preset.yaml` (ci: github-actions) TIDAK meng-emit
// .github/workflows/ci.yml, sementara jalur flag setara meng-emit — melanggar
// invarian byte-identical (matrix-4a #6). `ci: none`/absen = add-on TIDAK aktif.
func TestMapPresetToFlags_CIAddon(t *testing.T) {
	ptr := func(s string) *string { return &s }
	never := func(string) bool { return false } // tak ada flag eksplisit di CLI

	cases := []struct {
		name        string
		ci          *string
		wantCIAddon bool
	}{
		{"ci=github-actions → add-on ci aktif", ptr("github-actions"), true},
		{"ci=gitlab-ci → add-on ci aktif", ptr("gitlab-ci"), true},
		{"ci=none → add-on ci TIDAK aktif", ptr("none"), false},
		{"ci kosong → add-on ci TIDAK aktif", ptr(""), false},
		{"ci absen (nil) → add-on ci TIDAK aktif", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &preset{CI: tc.ci}
			var f createFlags
			mapPresetToFlags(p, &f, never)

			gotCIAddon := slices.Contains(f.presetAddons, "ci")
			if gotCIAddon != tc.wantCIAddon {
				t.Errorf("presetAddons=%v, add-on 'ci' present=%v, mau %v",
					f.presetAddons, gotCIAddon, tc.wantCIAddon)
			}
			// Provider enum tetap dialirkan ke f.ci apa adanya (validasi di Validate).
			if tc.ci != nil && f.ci != *tc.ci {
				t.Errorf("f.ci=%q, mau %q (provider harus mengalir dari preset)", f.ci, *tc.ci)
			}
		})
	}
}
