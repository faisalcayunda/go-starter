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
	go run ./cmd/demo-chi
# region:targets (addon-makefile) — format & vet seluruh kode.
.PHONY: fmt vet

fmt:
	gofmt -w .

vet:
	go vet ./...
# region:targets (addon-makefile) — build image Docker dari Dockerfile.
.PHONY: docker-build

docker-build:
	docker build -t demo-chi:latest .
# region:targets (db-postgres) — target migrasi DB via golang-migrate CLI.
# Memerlukan tool `migrate` terpasang: https://github.com/golang-migrate/migrate
# (CLI, BUKAN import Go — go build tidak menariknya). DSN dari env DATABASE_URL.
.PHONY: migrate-up migrate-down

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1
