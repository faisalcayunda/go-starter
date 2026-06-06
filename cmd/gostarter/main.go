// Command gostarter adalah CLI generator project Go best-practice (UX gaya
// "laravel new").
//
// Entry point tipis: seluruh wiring command-tree cobra, wizard huh, resolver,
// generator, dan hooks hidup di package internal/cli (ADR-002 §1). main hanya
// memanggil cli.Execute() lalu meneruskan exit code-nya.
package main

import (
	"os"

	"github.com/faisalcayunda/gostarter/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
