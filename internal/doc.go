// Package internal adalah akar package non-publik builder gostarter.
//
// Seluruh kode builder (cmd-wiring, prompt, resolver, generator, hooks, dst.)
// hidup di bawah github.com/faisalcayunda/gostarter/internal/... sehingga
// boundary "berduri" Go dipaksakan compiler: tidak ada konsumen di luar module
// builder yang bisa meng-import package internal ini. Ini sekaligus menjaga
// invarian zero lock-in — project hasil generate tidak pernah meng-import apa
// pun dari builder.
//
// Alur data builder (ADR-002):
//
//	cmd (cobra) -> prompt (huh) -> answers.Answers -> resolver -> plan.GeneratePlan
//	  -> generator (render template embed.FS) -> hooks (go mod tidy, gofmt, git init)
package internal
