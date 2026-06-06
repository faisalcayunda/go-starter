// Package logger menyediakan logger slog terstruktur bersama untuk seluruh
// service. Wrapper tipis di atas log/slog (stdlib) agar tiap service membangun
// logger dengan format & atribut service yang konsisten.
package logger

import (
	"log/slog"
	"os"
)

// New membangun *slog.Logger menulis ke stdout dengan format teks, ditambah
// atribut "service" agar log lintas-service mudah dikorelasikan. Logger yang
// dikembalikan juga dipasang sebagai default proses (slog.SetDefault).
func New(service string) *slog.Logger {
	l := slog.New(slog.NewTextHandler(os.Stdout, nil)).With(slog.String("service", service))
	slog.SetDefault(l)
	return l
}
