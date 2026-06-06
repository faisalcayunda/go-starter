# Makefile — repo BUILDER (gostarter CLI generator).
#
# Ini Makefile untuk membangun & menguji builder itu sendiri, BUKAN Makefile
# yang dihasilkan ke project hasil generate (itu ada di
# templates/modules/core/Makefile.tmpl dan dirakit terpisah saat generate).
#
# Tooling (buf, golangci-lint, protoc-gen-go) di-resolve dari PATH. Di mesin
# pengembang asdf, pastikan: export PATH="$$HOME/.asdf/installs/golang/<ver>/bin:$$PATH".

# Nama binary & lokasi entrypoint.
BINARY      := gostarter
CMD_PKG     := ./cmd/gostarter
GOLDEN_PKG  := ./internal/golden/...

# Script e2e (self-contained; mengatur PATH-nya sendiri).
E2E_SCRIPTS := scripts/matrix-4a.sh scripts/microservice-e2e.sh scripts/smoke.sh

.PHONY: all help build test vet fmt lint e2e golden-update golden-verify install ci tidy clean install-hooks

# Target default: jalankan gerbang lokal cepat (fmt-check via vet + lint + test).
all: lint test ## Alias: lint + test

help: ## Tampilkan daftar target beserta deskripsinya.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## Kompilasi seluruh paket builder.
	go build ./...

test: ## Jalankan unit test builder (KECUALI internal/golden — lihat golden-verify).
	# H-3: golden DIKECUALIKAN di sini (go list | grep -v internal/golden) — selaras
	# job 'test' di CI. Golden hanya dijalankan via 'golden-verify' (satu-satunya
	# pemeriksa golden, sama seperti CI job 'golden') agar tak ada double-run lokal.
	go test $(shell go list ./... | grep -v /internal/golden) -count=1

vet: ## Jalankan go vet pada seluruh paket.
	go vet ./...

fmt: ## Format seluruh sumber Go (menulis perubahan in-place).
	# M-2: KECUALIKAN internal/golden/testdata/** — pohon hasil render (snapshot
	# golden byte-identical), bukan kode builder; mem-format-nya merusak invarian
	# byte-identical. Format hanya kode builder asli (cmd/ + internal/ non-testdata).
	gofmt -w cmd
	@find internal -name '*.go' -not -path 'internal/golden/testdata/*' -exec gofmt -w {} +

lint: ## Jalankan golangci-lint (config .golangci.yml).
	golangci-lint run

# Golden-file (T5.2): regenerasi snapshot testdata yang DI-COMMIT.
# Harness golden membaca env UPDATE_GOLDEN untuk menulis ulang testdata/golden/.
golden-update: ## Regenerasi golden testdata (UPDATE_GOLDEN=1).
	UPDATE_GOLDEN=1 go test $(GOLDEN_PKG)

# Golden-verify (L-6): mode BANDING (bukan update) — gerbang yang dipakai 'ci'.
# Membandingkan render aktual vs snapshot yang di-commit byte-per-byte; GAGAL bila
# ada drift. -count=1 mematikan cache agar tak ada hasil basi (M-3).
golden-verify: ## Verifikasi golden testdata vs snapshot (mode banding, tanpa update).
	go test $(GOLDEN_PKG) -count=1

# e2e: ketiga script verifikasi end-to-end (matrix 4a + microservice + smoke).
# Script men-setup PATH tool-nya sendiri; jaringan AKTIF dibutuhkan untuk
# kombinasi db!=none (go mod tidy) dan microservice (buf generate).
e2e: ## Jalankan seluruh script e2e (matrix-4a, microservice-e2e, smoke).
	@for s in $(E2E_SCRIPTS); do \
		echo ">>> $$s"; \
		bash "$$s" || exit $$?; \
	done

install: ## Pasang binary gostarter ke GOBIN/GOPATH.
	go install $(CMD_PKG)

tidy: ## Rapikan go.mod / go.sum.
	go mod tidy

clean: ## Hapus artefak build lokal.
	rm -rf bin dist $(BINARY)

# Gerbang CI lokal: cerminkan job .github/workflows/ci.yml
# (lint -> test -> golden-verify -> e2e). H-2: golden-verify disertakan eksplisit
# agar drift snapshot tertangkap lokal sama seperti job 'golden' di CI. Konsisten
# dengan H-3: 'test' mengecualikan golden (go list | grep -v /internal/golden),
# 'golden-verify' adalah satu-satunya pemeriksa golden — jadi 'make ci' menjalankan
# golden tepat sekali (via golden-verify), sama seperti CI.
ci: lint test golden-verify e2e ## Gerbang penuh: lint + test + golden-verify + e2e.

# Pasang git hook pre-commit (symlink ke scripts/pre-commit).
install-hooks: ## Pasang git pre-commit hook (symlink ke scripts/pre-commit).
	@mkdir -p .git/hooks
	@ln -sf ../../scripts/pre-commit .git/hooks/pre-commit
	@chmod +x scripts/pre-commit
	@echo "pre-commit hook terpasang -> .git/hooks/pre-commit"
