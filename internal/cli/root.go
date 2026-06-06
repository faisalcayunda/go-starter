// Package cli merakit command-tree cobra builder gostarter (root + subcommand
// create) dan menyediakan titik masuk Execute() yang dipanggil cmd/gostarter.
//
// Tanggung jawab package ini (ADR-002 §1, tabel cmd/gostarter):
//   - membangun root cobra ("gostarter") + subcommand,
//   - parse flag → answers.Answers (atau jalankan wizard huh bila interaktif),
//   - memanggil orchestration: Validate → Resolve → EnsureEmptyDir → Generate →
//     post-gen hooks → cetak ringkasan.
//
// Tidak ada keputusan default/constraint di sini — itu milik resolver (ADR-002 §1).
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version adalah versi builder; diisi via -ldflags saat rilis (goreleaser).
var version = "0.0.0-dev"

// newRootCmd membangun root command "gostarter".
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "gostarter",
		Short: "Generator project Go best-practice (UX gaya \"laravel new\")",
		Long: "gostarter men-generate struktur project Go best-practice dalam hitungan detik.\n" +
			"Satu perintah, sebuah wizard ringkas (atau flag lengkap), lalu project yang\n" +
			"langsung jalan — tanpa edit manual, zero lock-in (project hasil generate tidak\n" +
			"meng-import apa pun dari builder).\n\n" +
			"Fase 3 (MVP): arch=monolith, kind=rest, http=net/http, db ∈ {none, postgres}.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newCreateCmd())
	root.AddCommand(newAddCmd())
	root.AddCommand(newVersionCmd())
	return root
}

// Execute adalah titik masuk CLI: membangun command-tree dan menjalankannya.
// Mengembalikan exit code (0 sukses, non-zero gagal) — cmd/gostarter meneruskan
// ke os.Exit.
func Execute() int {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}
