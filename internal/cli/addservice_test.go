package cli

import (
	"slices"
	"testing"
)

// TestMergeServices_PreservesOrder mengunci invarian penting: penggabungan
// --service + --services HARUS mempertahankan urutan input (service pertama =
// pemanggil, US-04 Sk.2) — BUKAN sort alfabet seperti mergeAddons. Dedup juga.
func TestMergeServices_PreservesOrder(t *testing.T) {
	cases := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		{"repeatable only", []string{"order", "user"}, nil, []string{"order", "user"}},
		{"csv only", nil, []string{"alpha", "beta"}, []string{"alpha", "beta"}},
		{"union preserves order", []string{"zeta", "order"}, []string{"user"}, []string{"zeta", "order", "user"}},
		{"dedup keeps first", []string{"order", "user"}, []string{"order"}, []string{"order", "user"}},
		{"trims + drops empty", []string{" order ", ""}, []string{"user"}, []string{"order", "user"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeServices(tc.a, tc.b)
			if !slices.Equal(got, tc.want) {
				t.Errorf("mergeServices(%v,%v) = %v, mau %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// TestResolveGatewayFlag memverifikasi --no-gateway menang atas --gateway, default
// false (SPEC §5.1 q_gateway default no).
func TestResolveGatewayFlag(t *testing.T) {
	cases := []struct {
		name            string
		gateway, noGate bool
		want            bool
	}{
		{"default off", false, false, false},
		{"gateway on", true, false, true},
		{"no-gateway wins", true, true, false},
		{"no-gateway alone", false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveGatewayFlag(tc.gateway, tc.noGate); got != tc.want {
				t.Errorf("resolveGatewayFlag(%v,%v) = %v, mau %v", tc.gateway, tc.noGate, got, tc.want)
			}
		})
	}
}

// TestToServiceList memverifikasi pemetaan nama → []answers.Service (urutan
// dipertahankan, kosong → nil).
func TestToServiceList(t *testing.T) {
	if got := toServiceList(nil); got != nil {
		t.Errorf("toServiceList(nil) = %v, mau nil", got)
	}
	got := toServiceList([]string{"order", "user"})
	if len(got) != 2 || got[0].Name != "order" || got[1].Name != "user" {
		t.Errorf("toServiceList = %+v, mau [order user]", got)
	}
}
