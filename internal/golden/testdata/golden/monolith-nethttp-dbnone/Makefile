# Makefile — skeleton ber-anchor (dirakit ModeMerge oleh assembler).
# Modul lain (migrate, lint, dst.) menyumbang target ke anchor targets.
.PHONY: build test

build:
	go build ./...

test:
	go test ./...

# region:targets (addon-makefile) — jalankan aplikasi dari source.
.PHONY: run

run:
	go run ./cmd/demo-mono
# region:targets (addon-makefile) — format & vet seluruh kode.
.PHONY: fmt vet

fmt:
	gofmt -w .

vet:
	go vet ./...
