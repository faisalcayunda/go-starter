#!/usr/bin/env bash
#
# microservice-e2e.sh — e2e verifikasi Fase 4b (T4.2 + T4.3).
#
# Membuktikan acceptance microservice end-to-end:
#   LANGKAH 1 — generate microservice (svc-a,svc-b) → buf generate (hook saat
#               create) → go mod tidy + vet + build kedua service → JALANKAN kedua
#               service sebagai proses lokal (port via env) → curl endpoint
#               inter-service service pertama (GET /call?to=svc-b) → verifikasi
#               respons memuat pesan dari svc-b (BUKTI A→B via gRPC). Matikan proses.
#   LANGKAH 2 — `gostarter add service svc-c` pada project yang sama: verifikasi
#               services/svc-c + proto/svc-c + gen/go/svc-c + compose ditambah +
#               build tetap hijau. Idempoten: jalankan lagi → ditolak (exit≠0).
#   LANGKAH 3 — gen DI-COMMIT: build ULANG project TANPA buf di PATH → tetap hijau.
#   LANGKAH 4 — Docker build = SKIP (tak ada daemon) — cetak SKIP, jangan gagalkan.
#   LANGKAH 5 — (Sanity) generate KETIGA arsitektur (monolith, modular-monolith,
#               microservice) → bukti acceptance Fase 4 "3 mode arsitektur".
#
# Prasyarat builder (BUKAN prasyarat project hasil generate):
#   buf + protoc-gen-go + protoc-gen-go-grpc di PATH (hook buf generate saat create).
#   PATH asdf disuntik otomatis di bawah bila ada.
#
# Catatan port (proses lokal — bukan docker DNS): svc-a gRPC :50051 + HTTP :8081;
# svc-b gRPC :50052; svc-a memanggil svc-b via SVC_B_GRPC_ADDR=127.0.0.1:50052.

set -u

# ── PATH: pastikan toolchain Go 1.26.3 + buf + plugin tersedia ────────────────
ASDF_BIN="/Users/isal/.asdf/installs/golang/1.26.3/bin"
if [ -d "$ASDF_BIN" ]; then
  export PATH="$ASDF_BIN:$PATH"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$REPO_ROOT/bin/gostarter"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/gostarter-micro-e2e-XXXXXX")"

FAIL=0
declare -a SVC_PIDS=()  # PID proses service untuk cleanup

hr()    { printf '%s\n' "================================================================"; }
step()  { printf '\n>>> %s\n' "$*"; }
okp()   { printf '    [OK]   %s\n' "$*"; }
skipp() { printf '    [SKIP] %s\n' "$*"; }
failp() { printf '    [FAIL] %s\n' "$*"; FAIL=1; }

# kill_services — matikan seluruh proses service yang masih hidup.
kill_services() {
  local pid
  for pid in "${SVC_PIDS[@]:-}"; do
    [ -n "$pid" ] && kill "$pid" 2>/dev/null
  done
  wait 2>/dev/null
  SVC_PIDS=()
}

cleanup() {
  kill_services
  rm -rf "$WORK"
}
trap cleanup EXIT INT TERM

# run <label> <dir> <cmd...> — jalankan perintah di dir; OK/FAIL + log saat gagal.
run() {
  local label="$1"; shift
  local dir="$1"; shift
  printf '    --- %s ---\n' "$label"
  ( cd "$dir" && "$@" ) >/tmp/micro-step.log 2>&1
  local rc=$?
  if [ "$rc" -eq 0 ]; then
    okp "$label"
    return 0
  fi
  failp "$label (exit $rc)"
  sed 's/^/        /' /tmp/micro-step.log | tail -30
  return 1
}

# wait_http <url> <tries> — polling sampai HTTP 2xx atau habis percobaan.
wait_http() {
  local url="$1"; local tries="${2:-40}"
  local i
  for i in $(seq 1 "$tries"); do
    if curl -fsS "$url" -o /dev/null 2>/dev/null; then
      return 0
    fi
    sleep 0.3
  done
  return 1
}

hr
printf 'microservice-e2e — Fase 4b (T4.2 + T4.3)\n'
printf 'repo:  %s\n' "$REPO_ROOT"
printf 'work:  %s\n' "$WORK"
printf 'go:    %s\n' "$(go version 2>/dev/null)"
printf 'buf:   %s\n' "$(command -v buf || echo '(tidak ada — generate akan gagal)')"
hr

# ── 0. Build binary gostarter ────────────────────────────────────────────────
step "0. Build binary gostarter"
if run "go build -o bin/gostarter ./cmd/gostarter" "$REPO_ROOT" \
     go build -o "$BIN" ./cmd/gostarter; then
  :
else
  failp "build gostarter gagal — stop"
  hr; printf 'VERDICT: MERAH (binary gostarter gagal di-build)\n'; hr
  exit 1
fi

# ── 1. Generate microservice (svc-a, svc-b) via JALUR CLI TERPADU ────────────
# Memakai create --arch microservice --services svc-a,svc-b (CSV terpadu),
# jalur yang sama dipakai end-user — bukan flag internal khusus uji.
MS_DIR="$WORK/shopapp"
step "1. Generate microservice (svc-a,svc-b) via CLI terpadu → $MS_DIR"
if run "gostarter create --arch microservice --services svc-a,svc-b" "$REPO_ROOT" \
     "$BIN" create --non-interactive \
       --name shopapp --module github.com/acme/shopapp \
       --arch microservice --services svc-a,svc-b \
       -o "$MS_DIR"; then
  :
else
  failp "generate microservice gagal — stop"
  hr; printf 'VERDICT: MERAH (generate microservice gagal)\n'; hr
  exit 1
fi

# 1a. Verifikasi struktur kunci hadir (proto + gen committed + services + libs).
# CATATAN PENTING (jalur generate sebenarnya): nama service dipertahankan apa
# adanya pada path proto/gen (svc-a, BUKAN svc_a); implementasi gRPC ada di
# services/<svc>/internal/server/server.go; helper dial di libs/grpcclient/grpcclient.go.
step "1a. Verifikasi struktur & gen DI-COMMIT"
for p in \
  buf.yaml buf.gen.yaml docker-compose.yml go.mod Makefile \
  proto/svc-a/v1/svc-a.proto proto/svc-b/v1/svc-b.proto \
  gen/go/svc-a/v1/svc-a.pb.go gen/go/svc-a/v1/svc-a_grpc.pb.go \
  gen/go/svc-b/v1/svc-b.pb.go gen/go/svc-b/v1/svc-b_grpc.pb.go \
  services/svc-a/cmd/main.go services/svc-a/internal/server/server.go \
  services/svc-a/internal/gateway/gateway.go \
  services/svc-b/cmd/main.go services/svc-b/internal/server/server.go \
  libs/grpcclient/grpcclient.go libs/config/config.go libs/logger/logger.go ; do
  if [ -e "$MS_DIR/$p" ]; then okp "ada: $p"; else failp "hilang: $p"; fi
done

# 1b. Zero lock-in: tak ada import/jejak builder. Header protoc-gen-go DIKECUALIKAN
#     (konvensi protobuf yang sah). Cari "gostarter"/module builder di file .go/.mod
#     non-gen + cek tak ada "Code generated by gostarter".
step "1b. Invarian zero lock-in (output tak menyebut/import builder)"
LOCK_HITS=$(grep -rn "faisalcayunda/gostarter\|Code generated by gostarter" \
  "$MS_DIR" --include='*.go' --include='go.mod' --include='*.yaml' --include='*.yml' 2>/dev/null || true)
if [ -z "$LOCK_HITS" ]; then
  okp "tak ada import/jejak builder di output"
else
  failp "ditemukan jejak builder:"; printf '%s\n' "$LOCK_HITS" | sed 's/^/        /'
fi
# Header protoc-gen-go HARUS ada di gen (konvensi protobuf sah) — sanity informatif.
if grep -rq "Code generated by protoc-gen-go" "$MS_DIR/gen" 2>/dev/null; then
  okp "header protoc-gen-go ada di gen/ (konvensi protobuf — dikecualikan dari zero-lock-in)"
fi

# 1c. go mod tidy + vet + build kedua service.
step "1c. go mod tidy + vet + build (project hasil generate)"
run "go mod tidy"        "$MS_DIR" go mod tidy
run "go vet ./..."       "$MS_DIR" go vet ./...
run "go build ./..."     "$MS_DIR" go build ./...
# Build biner kedua service untuk dijalankan sebagai proses lokal.
run "build svc-a binary" "$MS_DIR" go build -o "$WORK/bin-svc-a" ./services/svc-a/cmd
run "build svc-b binary" "$MS_DIR" go build -o "$WORK/bin-svc-b" ./services/svc-b/cmd

# 1d. Jalankan kedua service sebagai proses lokal + curl inter-service call.
step "1d. Jalankan svc-a + svc-b lokal; curl GET /call?to=svc-b (bukti A→B gRPC)"
INTERSERVICE_OK=0
INTERSERVICE_RESP=""
if [ -x "$WORK/bin-svc-a" ] && [ -x "$WORK/bin-svc-b" ]; then
  # svc-b: gRPC :50052
  GRPC_ADDR=":50052" "$WORK/bin-svc-b" >"$WORK/svc-b.log" 2>&1 &
  SVC_PIDS+=("$!")
  # svc-a: gRPC :50051, HTTP :8081, callee → 127.0.0.1:50052 (proses lokal, bukan DNS docker)
  GRPC_ADDR=":50051" HTTP_ADDR=":8081" SVC_B_GRPC_ADDR="127.0.0.1:50052" \
    "$WORK/bin-svc-a" >"$WORK/svc-a.log" 2>&1 &
  SVC_PIDS+=("$!")

  if wait_http "http://127.0.0.1:8081/call?to=svc-b" 40; then
    INTERSERVICE_RESP="$(curl -fsS "http://127.0.0.1:8081/call?to=svc-b" 2>/dev/null)"
    printf '    respons /call?to=svc-b : %s\n' "$INTERSERVICE_RESP"
    # Bukti A→B: respons HARUS memuat pesan dari svc-b ("dari service svc-b").
    if printf '%s' "$INTERSERVICE_RESP" | grep -q "dari service svc-b"; then
      okp "inter-service call A→B BERHASIL (svc-a memanggil svc-b via gRPC)"
      INTERSERVICE_OK=1
    else
      failp "respons tidak memuat pesan svc-b: $INTERSERVICE_RESP"
    fi
  else
    failp "HTTP svc-a tak merespons di :8081 — log:"
    sed 's/^/        a> /' "$WORK/svc-a.log" | tail -15
    sed 's/^/        b> /' "$WORK/svc-b.log" | tail -15
  fi
else
  failp "biner service tak terbangun — lewati run"
fi
kill_services
okp "proses service dimatikan & dibersihkan"

# ── 2. add service svc-c (T4.3) ──────────────────────────────────────────────
step "2. gostarter add service svc-c (project microservice existing)"
ADDSVC_OK=0
if run "gostarter add service svc-c -o $MS_DIR" "$REPO_ROOT" \
     "$BIN" add service svc-c -o "$MS_DIR"; then
  # 2a. Verifikasi artefak svc-c terbentuk (nama service literal: svc-c).
  ADD_OK=1
  for p in \
    services/svc-c/cmd/main.go services/svc-c/internal/server/server.go \
    proto/svc-c/v1/svc-c.proto \
    gen/go/svc-c/v1/svc-c.pb.go gen/go/svc-c/v1/svc-c_grpc.pb.go ; do
    if [ -e "$MS_DIR/$p" ]; then okp "ada: $p"; else failp "hilang: $p"; ADD_OK=0; fi
  done
  # 2b. compose ditambah svc-c.
  if grep -q "svc-c:" "$MS_DIR/docker-compose.yml"; then
    okp "docker-compose.yml memuat blok svc-c"
  else
    failp "docker-compose.yml TIDAK memuat svc-c"; ADD_OK=0
  fi
  # 2c. Marker anchor masih hidup (idempotensi).
  if grep -q "region:services" "$MS_DIR/docker-compose.yml"; then
    okp "anchor region:services dipertahankan (idempotensi terjaga)"
  else
    failp "anchor region:services HILANG — add-service berikutnya tak punya titik-sisip"; ADD_OK=0
  fi
  # 2d. Build tetap hijau setelah add.
  if run "go build ./... (setelah add svc-c)" "$MS_DIR" go build ./...; then
    :
  else
    ADD_OK=0
  fi
  # 2e. Idempoten: add svc-c lagi → HARUS ditolak (exit≠0).
  printf '    --- idempotensi: add service svc-c (ulang) HARUS ditolak ---\n'
  if ( cd "$REPO_ROOT" && "$BIN" add service svc-c -o "$MS_DIR" ) >/tmp/micro-reject.log 2>&1; then
    failp "add service svc-c ulang TIDAK ditolak (seharusnya gagal)"; ADD_OK=0
  else
    okp "add svc-c ulang ditolak dengan benar (exit≠0)"
    sed 's/^/        /' /tmp/micro-reject.log | tail -3
  fi
  # 2f. Tolak pada project NON-microservice.
  printf '    --- penolakan: add service pada project NON-microservice ---\n'
  NONMS="$WORK/plainmono"
  ( cd "$REPO_ROOT" && "$BIN" create --non-interactive --name plainmono \
      --module github.com/acme/plainmono -o "$NONMS" ) >/dev/null 2>&1
  if ( cd "$REPO_ROOT" && "$BIN" add service foo -o "$NONMS" ) >/tmp/micro-reject2.log 2>&1; then
    failp "add service pada non-microservice TIDAK ditolak"; ADD_OK=0
  else
    okp "add service pada non-microservice ditolak dengan benar (exit≠0)"
    sed 's/^/        /' /tmp/micro-reject2.log | tail -2
  fi
  ADDSVC_OK=$ADD_OK
else
  failp "add service svc-c gagal"
fi

# ── 3. gen DI-COMMIT → build TANPA buf di PATH ───────────────────────────────
step "3. Build ULANG project TANPA buf di PATH (bukti gen di-commit)"
GEN_NOBUF_OK=0
# Bangun PATH minimal yang BERISI toolchain Go (go, gofmt) TAPI TIDAK berisi
# buf/protoc-gen-*. `which go` adalah shim asdf yang butuh `asdf` di PATH; dan
# direktori install asdf ($ASDF_BIN) memuat `buf`. Untuk uji "tanpa buf" yang
# valid kita pakai biner Go ASLI di $GOROOT/bin (direktori TERPISAH dari $ASDF_BIN,
# TIDAK memuat buf) sebagai satu-satunya sumber toolchain, ditambah path sistem
# dasar. Dengan begitu `command -v buf` benar-benar gagal → membuktikan project
# ter-build tanpa buf (gen/ di-commit).
GOROOT_DIR="$(go env GOROOT 2>/dev/null)"
GO_BIN_REAL="$GOROOT_DIR/bin"
if [ ! -x "$GO_BIN_REAL/go" ]; then
  # Fallback: resolusi simbolik shim (jarang diperlukan).
  GO_BIN_REAL="$(dirname "$(readlink -f "$(command -v go)" 2>/dev/null || command -v go)")"
fi
# PATH tanpa buf: GOROOT/bin (go,gofmt asli) + path sistem dasar (TANPA $ASDF_BIN).
NOBUF_PATH="$GO_BIN_REAL:/usr/bin:/bin:/usr/sbin:/sbin"
if PATH="$NOBUF_PATH" command -v buf >/dev/null 2>&1; then
  skipp "buf masih terlihat di PATH minimal — uji 'tanpa buf' kurang ketat, tapi build tetap dijalankan"
else
  okp "buf TIDAK ada di PATH minimal (uji 'tanpa buf' valid)"
fi
if ( cd "$MS_DIR" && PATH="$NOBUF_PATH" go build ./... ) >/tmp/micro-nobuf.log 2>&1; then
  okp "go build ./... HIJAU tanpa buf di PATH (gen/ di-commit terbukti)"
  GEN_NOBUF_OK=1
else
  failp "build gagal tanpa buf — gen/ mungkin tidak di-commit:"
  sed 's/^/        /' /tmp/micro-nobuf.log | tail -20
fi

# ── 4. Docker build = SKIP (no daemon) ───────────────────────────────────────
step "4. Docker build"
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  skipp "docker daemon terdeteksi, tetapi sesuai scope T4.2/T4.3 docker build DILEWATI"
else
  skipp "docker daemon TIDAK tersedia — docker build dilewati (sesuai instruksi)"
fi

# ── 5. Sanity: generate 3 arsitektur ─────────────────────────────────────────
step "5. Sanity: generate 3 mode arsitektur (acceptance Fase 4)"
ARCH3_OK=1

# 5a. monolith (db=none) → murni stdlib, build OFFLINE.
MONO="$WORK/mono"
if run "create monolith (db=none)" "$REPO_ROOT" \
     "$BIN" create --non-interactive --name monoapp --module github.com/acme/monoapp \
       --arch monolith --http net/http --db none -o "$MONO"; then
  if run "monolith build OFFLINE (GOFLAGS=-mod=mod GOPROXY=off)" "$MONO" \
       env GOPROXY=off go build ./...; then :; else ARCH3_OK=0; fi
else
  ARCH3_OK=0
fi

# 5b. modular-monolith (db=none) → build OFFLINE.
MODULAR="$WORK/modular"
if run "create modular-monolith (db=none)" "$REPO_ROOT" \
     "$BIN" create --non-interactive --name modapp --module github.com/acme/modapp \
       --arch modular-monolith --http net/http --db none -o "$MODULAR"; then
  if run "modular build OFFLINE (GOPROXY=off)" "$MODULAR" \
       env GOPROXY=off go build ./...; then :; else ARCH3_OK=0; fi
else
  ARCH3_OK=0
fi

# 5c. microservice → sudah dibuat & dibangun di Langkah 1 (build hijau).
if [ -d "$MS_DIR" ] && [ -f "$MS_DIR/buf.yaml" ]; then
  okp "microservice tergenerate (dipakai dari Langkah 1)"
else
  failp "microservice tidak tergenerate"; ARCH3_OK=0
fi
if [ "$ARCH3_OK" -eq 1 ]; then
  okp "3 mode arsitektur (monolith, modular-monolith, microservice) TER-GENERATE"
else
  failp "satu/lebih arsitektur gagal generate/build"
fi

# ── 6. Gateway: create --gateway → ter-generate + build hijau ────────────────
# --gateway memproyeksikan .Gateway ke data manifest (edge HTTP→gRPC). Gateway
# in-proses (HTTP /call → gRPC) per-service tetap hadir untuk service pertama.
# Assertion: flag diterima, gateway file ada, project build HIJAU.
step "6. create --gateway (microservice) → ter-generate + build hijau"
GATEWAY_OK=0
GW_DIR="$WORK/gwapp"
if run "create --arch microservice --services svc-a,svc-b --gateway" "$REPO_ROOT" \
     "$BIN" create --non-interactive --name gwapp --module github.com/acme/gwapp \
       --arch microservice --services svc-a,svc-b --gateway -o "$GW_DIR"; then
  GW_OK=1
  # Gateway HTTP→gRPC milik service pertama (svc-a) harus ter-generate.
  if [ -e "$GW_DIR/services/svc-a/internal/gateway/gateway.go" ]; then
    okp "ada: services/svc-a/internal/gateway/gateway.go (gateway HTTP→gRPC)"
  else
    failp "hilang: services/svc-a/internal/gateway/gateway.go"; GW_OK=0
  fi
  if run "go build ./... (project --gateway)" "$GW_DIR" go build ./...; then
    :
  else
    GW_OK=0
  fi
  GATEWAY_OK=$GW_OK
else
  failp "create --gateway gagal"
fi

# ── VERDICT ──────────────────────────────────────────────────────────────────
hr
printf 'RINGKASAN\n'
printf '  inter-service call A→B (svc-a → svc-b gRPC) : %s\n' \
  "$([ "$INTERSERVICE_OK" -eq 1 ] && echo HIJAU || echo MERAH)"
printf '    respons konkret                           : %s\n' "${INTERSERVICE_RESP:-(kosong)}"
printf '  add service svc-c (+idempoten +tolak non-ms): %s\n' \
  "$([ "$ADDSVC_OK" -eq 1 ] && echo HIJAU || echo MERAH)"
printf '  gen di-commit (build tanpa buf)             : %s\n' \
  "$([ "$GEN_NOBUF_OK" -eq 1 ] && echo HIJAU || echo MERAH)"
printf '  3 arsitektur ter-generate                   : %s\n' \
  "$([ "$ARCH3_OK" -eq 1 ] && echo HIJAU || echo MERAH)"
printf '  gateway (--gateway) ter-generate + build    : %s\n' \
  "$([ "$GATEWAY_OK" -eq 1 ] && echo HIJAU || echo MERAH)"
printf '  docker build                                : SKIP (no daemon)\n'
hr
if [ "$FAIL" -eq 0 ] \
   && [ "$INTERSERVICE_OK" -eq 1 ] \
   && [ "$ADDSVC_OK" -eq 1 ] \
   && [ "$GEN_NOBUF_OK" -eq 1 ] \
   && [ "$ARCH3_OK" -eq 1 ] \
   && [ "$GATEWAY_OK" -eq 1 ]; then
  printf 'VERDICT: HIJAU — 2 service build+call jalan, add-service jalan, 3 arch generate, gateway hijau.\n'
  hr
  exit 0
fi
printf 'VERDICT: MERAH — lihat baris [FAIL] di atas.\n'
hr
exit 1
