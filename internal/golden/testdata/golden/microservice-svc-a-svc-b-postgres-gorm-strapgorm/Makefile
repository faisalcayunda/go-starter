# Makefile — monorepo microservice.
#
# Target `proto` me-regenerasi stub gRPC (gen/go/**) dari kontrak proto/ memakai
# buf. gen/ DI-COMMIT, jadi `proto` hanya perlu dijalankan saat kontrak berubah —
# `go build ./...` hijau tanpa menjalankannya (riset 02 §4).
.PHONY: proto build test up down

# Regenerasi stub gRPC dari proto/. Prasyarat: buf + protoc-gen-go + protoc-gen-go-grpc.
#   go install github.com/bufbuild/buf/cmd/buf@latest
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
proto:
	buf generate

build:
	go build ./...

test:
	go test ./...

# Jalankan seluruh service + dependensi via docker compose.
up:
	docker compose up --build

down:
	docker compose down -v

# region:targets
# Anchor netral untuk target tambahan dari modul lain (mis. addon-makefile saat
# aktif). Pada microservice addon-makefile TIDAK diaktifkan resolver — anchor ini
# dipertahankan agar validasi katalog anchor↔skeleton (M-4) konsisten lintas modul.
