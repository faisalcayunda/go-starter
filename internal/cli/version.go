// Subcommand "version" — mencetak versi builder gostarter.
//
// Versi yang ditampilkan berasal dari variabel package-level `version`
// (lihat root.go), yang di-inject saat rilis via:
//
//	-ldflags "-X github.com/faisalcayunda/gostarter/internal/cli.version={{.Version}}"
//
// Tanpa injeksi (mis. `go build` lokal), nilainya jatuh ke default "0.0.0-dev".
//
// Tersedia dua jalur ekuivalen:
//   - flag global "gostarter --version" (disediakan cobra via rootCmd.Version),
//   - subcommand "gostarter version" (didefinisikan di sini).

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVersionCmd membangun subcommand "version" yang mencetak versi builder.
// Output-nya disamakan dengan `--version` ("gostarter version <v>") agar
// kedua jalur konsisten.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Tampilkan versi gostarter",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "gostarter version %s\n", version)
			return err
		},
	}
}
