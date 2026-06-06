// Package modpath memusatkan helper manipulasi module path & pembandingan versi
// yang sebelumnya terduplikasi di renderer (generator) dan resolver.
//
// Tujuannya menegakkan SATU sumber kebenaran untuk dua keputusan yang
// memengaruhi invarian byte-identical (SPEC §5.2):
//
//   - Kebijakan dedup dependency go.mod: HIGHEST VERSION WINS bila path ganda
//     (ADR-002 §6 "Perakitan go.mod"). Sebelumnya resolver memakai pembanding
//     leksikografis dan generator memakai "last wins" — dua kebijakan berbeda
//     berisiko menghasilkan go.mod tak konsisten. HigherVersion menyatukannya.
//   - Strip suffix versi mayor "/vN" pada module path (modBase). Sebelumnya
//     renderer memakai unicode.IsDigit dan resolver memakai cek ASCII manual —
//     dua semantik berbeda. IsMajorVersionSegment menyatukannya (ASCII-only,
//     sesuai semantik module path Go).
//
// Semua fungsi MURNI & deterministik (tanpa side-effect/locale).
package modpath

import (
	"strings"

	"golang.org/x/mod/semver"
)

// IsMajorVersionSegment melaporkan apakah seg berbentuk "vN" dengan N integer
// ASCII ≥ 2 (konvensi semantic import versioning Go module ≥2). "v0"/"v1" bukan
// suffix versi mayor yang disuffix-kan pada path. Hanya digit ASCII yang diterima
// (module path Go ASCII-only, SPEC §4.2).
func IsMajorVersionSegment(seg string) bool {
	if len(seg) < 2 || seg[0] != 'v' {
		return false
	}
	for i := 1; i < len(seg); i++ {
		if seg[i] < '0' || seg[i] > '9' {
			return false
		}
	}
	return seg != "v0" && seg != "v1"
}

// Base mengembalikan segmen terakhir module path (base), melucuti suffix versi
// mayor "/vN" (N ≥ 2) bila ada segmen sebelumnya. Untuk nama biner / package
// root. Murni manipulasi path; tidak memvalidasi module path.
//
//	Base("github.com/acme/shop")    → "shop"
//	Base("github.com/acme/shop/v2") → "shop"
func Base(modulePath string) string {
	p := strings.Trim(modulePath, "/")
	if p == "" {
		return ""
	}
	segs := strings.Split(p, "/")
	last := segs[len(segs)-1]
	if len(segs) > 1 && IsMajorVersionSegment(last) {
		last = segs[len(segs)-2]
	}
	return last
}

// Join menggabung module path + elemen path import dengan "/" (slash-only POSIX,
// SPEC §2.1). Untuk menyusun import path internal. Memakai path.Join-style
// cleaning tetapi tanpa import path standar agar pemanggil tidak bergantung
// semantik OS-aware.
func Join(modulePath string, elem ...string) string {
	parts := make([]string, 0, 1+len(elem))
	if modulePath != "" {
		parts = append(parts, strings.Trim(modulePath, "/"))
	}
	for _, e := range elem {
		if e == "" {
			continue
		}
		parts = append(parts, strings.Trim(e, "/"))
	}
	return strings.Join(parts, "/")
}

// CompareVersions membandingkan dua versi modul go.mod dengan semantik semver
// (golang.org/x/mod/semver). Mengembalikan -1, 0, atau +1 (a<b, a==b, a>b).
//
// Versi tanpa prefix "v" (mis. "1.4.0") dinormalkan ke "v1.4.0" agar tetap
// terbandingkan secara semver. Bila salah satu tetap bukan versi semver yang
// valid setelah normalisasi, fungsi jatuh ke pembanding leksikografis stabil
// agar deterministik (tidak panik).
func CompareVersions(a, b string) int {
	na, nb := normalizeSemver(a), normalizeSemver(b)
	if semver.IsValid(na) && semver.IsValid(nb) {
		return semver.Compare(na, nb)
	}
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// HigherVersion mengembalikan versi yang lebih tinggi di antara a dan b
// (kebijakan kanonik "highest version wins", ADR-002 §6). Bila setara,
// mengembalikan a (stabil).
func HigherVersion(a, b string) string {
	if CompareVersions(a, b) >= 0 {
		return a
	}
	return b
}

// normalizeSemver menambahkan prefix "v" bila versi tidak diawali "v" agar
// golang.org/x/mod/semver dapat membandingkannya. Versi yang sudah valid (mis.
// "v5.10.0") dikembalikan apa adanya.
func normalizeSemver(v string) string {
	if v == "" {
		return v
	}
	if v[0] == 'v' {
		return v
	}
	return "v" + v
}
