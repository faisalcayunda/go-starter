package modpath

import "testing"

func TestIsMajorVersionSegment(t *testing.T) {
	cases := []struct {
		seg  string
		want bool
	}{
		{"v2", true},
		{"v10", true},
		{"v0", false},
		{"v1", false},
		{"v", false},
		{"shop", false},
		{"v2a", false}, // bukan digit ASCII murni
		{"vX", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsMajorVersionSegment(c.seg); got != c.want {
			t.Errorf("IsMajorVersionSegment(%q) = %v, want %v", c.seg, got, c.want)
		}
	}
}

func TestBase(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"github.com/acme/shop", "shop"},
		{"github.com/acme/shop/v2", "shop"},
		{"github.com/acme/shop/v10", "shop"},
		{"shop", "shop"},
		{"shop/v2", "shop"},
		{"", ""},
		{"v2", "v2"}, // tunggal: tidak ada segmen sebelum → kembalikan apa adanya
	}
	for _, c := range cases {
		if got := Base(c.in); got != c.want {
			t.Errorf("Base(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJoin(t *testing.T) {
	if got := Join("github.com/acme/fleet", "services", "user"); got != "github.com/acme/fleet/services/user" {
		t.Errorf("Join = %q", got)
	}
	if got := Join("github.com/acme/fleet", "", "user"); got != "github.com/acme/fleet/user" {
		t.Errorf("Join dengan elemen kosong = %q", got)
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v5.10.0", "v5.9.0", 1},  // 10 > 9 (semver, BUKAN leksikografis)
		{"v5.9.0", "v5.10.0", -1}, // leksikografis akan salah ("v5.9" > "v5.10")
		{"v1.4.0", "v1.4.0", 0},   // setara
		{"1.4.0", "1.3.0", 1},     // tanpa prefix v → tetap dibanding semver
		{"v2.0.0", "v10.0.0", -1}, // 2 < 10
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestCompareVersions_NonSemverFallback memverifikasi jalur fallback leksikografis
// CompareVersions: bila salah satu argumen BUKAN versi semver valid (bahkan setelah
// normalisasi prefix "v"), fungsi jatuh ke pembanding leksikografis STABIL (tidak
// panik) — menjaga determinisme go.mod meski versi pseudo/aneh.
func TestCompareVersions_NonSemverFallback(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// "latest" bukan semver → fallback leksikografis ("latest" < "stable").
		{"latest", "stable", -1},
		{"stable", "latest", 1},
		{"latest", "latest", 0},
		// Satu valid, satu tidak → fallback leksikografis (semver IsValid gagal salah satu).
		{"abc", "v1.0.0", -1}, // "abc" < "v1.0.0" leksikografis
		// String kosong: normalizeSemver("") = "" (tak valid) → fallback.
		{"", "v1.0.0", -1},
		{"", "", 0},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q,%q) = %d, want %d (fallback leksikografis)", c.a, c.b, got, c.want)
		}
	}
}

// TestCompareVersions_NormalizeNoPrefix memverifikasi versi tanpa prefix "v"
// dinormalkan & dibandingkan secara semver (bukan leksikografis).
func TestCompareVersions_NormalizeNoPrefix(t *testing.T) {
	// 1.10.0 > 1.9.0 secara semver (leksikografis akan salah: "1.10" < "1.9").
	if got := CompareVersions("1.10.0", "1.9.0"); got != 1 {
		t.Errorf("CompareVersions(1.10.0, 1.9.0) = %d, want 1 (semver setelah normalisasi prefix v)", got)
	}
}

// TestHigherVersion_NonSemver memverifikasi HigherVersion stabil pada input
// non-semver (mengembalikan a saat setara/lebih tinggi leksikografis).
func TestHigherVersion_NonSemver(t *testing.T) {
	if got := HigherVersion("stable", "latest"); got != "stable" {
		t.Errorf("HigherVersion(stable, latest) = %q, want stable (leksikografis)", got)
	}
	// Setara non-semver → a.
	if got := HigherVersion("dev", "dev"); got != "dev" {
		t.Errorf("HigherVersion setara non-semver = %q, want dev", got)
	}
}

// TestBase_TrailingSlash memverifikasi Base menangani slash berlebih & path
// hanya-slash tanpa panik.
func TestBase_TrailingSlash(t *testing.T) {
	cases := []struct{ in, want string }{
		{"github.com/acme/shop/", "shop"}, // trailing slash di-trim
		{"/github.com/acme/shop", "shop"}, // leading slash di-trim
		{"///", ""},                       // hanya slash → kosong
		{"shop/", "shop"},
	}
	for _, c := range cases {
		if got := Base(c.in); got != c.want {
			t.Errorf("Base(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHigherVersion(t *testing.T) {
	// "highest version wins" — kasus kritis: v5.10.0 vs v5.9.0 (leksikografis salah).
	if got := HigherVersion("v5.9.0", "v5.10.0"); got != "v5.10.0" {
		t.Errorf("HigherVersion(v5.9.0, v5.10.0) = %q, want v5.10.0", got)
	}
	if got := HigherVersion("v5.10.0", "v5.9.0"); got != "v5.10.0" {
		t.Errorf("HigherVersion(v5.10.0, v5.9.0) = %q, want v5.10.0", got)
	}
	// Setara → kembalikan a (stabil).
	if got := HigherVersion("v1.4.0", "v1.4.0"); got != "v1.4.0" {
		t.Errorf("HigherVersion setara = %q, want v1.4.0", got)
	}
}
