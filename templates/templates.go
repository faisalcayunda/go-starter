// Package templates membundel seluruh modul template ke dalam binary builder via
// embed.FS (single-binary distribution, ADR-001 §3). Direktori modules/ berisi
// modul-modul template; tiap modul punya module.yaml + file .tmpl.
package templates

import "embed"

// FS adalah filesystem ter-embed berisi semua modul template di bawah modules/.
//
//go:embed modules
var FS embed.FS
