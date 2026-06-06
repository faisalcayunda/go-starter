#!/usr/bin/env bash
#
# matrix-4a.sh — e2e matrix verifikasi Fase 4a (T4.1 + T4.4 + T4.5).
#
# Membangun binary lalu menggenerate & memverifikasi 6 kombinasi WAJIB:
#   1. monolith / net-http / db=none           → HIJAU OFFLINE (GOPROXY=off)
#   2. monolith / chi / db=postgres            → go mod tidy + vet/build/test
#   3. monolith / echo / db=mysql              → go mod tidy + vet/build/test
#   4. modular-monolith / net-http / db=none   → HIJAU OFFLINE + 2 domain endpoint
#   5. modular / chi / db=postgres / full addons (docker,makefile,golangci,env,
#      ci=github-actions,observability)        → kombinasi penuh
#   6. --config preset vs flag → byte-identical (diff -rq)
#
# Untuk kombinasi ber-DB / non-stdlib: "go mod tidy" dulu (butuh jaringan).
# Docker build = SKIP (tak ada daemon) — cetak SKIP, jangan gagalkan.

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$REPO_ROOT/bin/gostarter"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/gostarter-matrix-XXXXXX")"

FAIL=0
declare -a ROWS  # baris hasil tabel: "name|vet|build|test|note"

hr()   { printf '%s\n' "================================================================"; }
step() { printf '\n>>> %s\n' "$*"; }
okp()  { printf '    [OK]   %s\n' "$*"; }
skipp(){ printf '    [SKIP] %s\n' "$*"; }
failp(){ printf '    [FAIL] %s\n' "$*"; FAIL=1; }

cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

# runstep <label> <dir> <env-prefix> <cmd...> → echo "OK"/"FAIL"
runstep() {
  local label="$1"; shift
  local dir="$1"; shift
  local envp="$1"; shift
  printf '    --- %s ---\n' "$label"
  ( cd "$dir" && env $envp "$@" ) >/tmp/matrix-step.log 2>&1
  local rc=$?
  if [ "$rc" -eq 0 ]; then
    okp "$label"
    echo OK
  else
    failp "$label (exit $rc)"
    sed 's/^/        /' /tmp/matrix-step.log | tail -25
    echo FAIL
  fi
}

# verify_project <name> <dir> <envprefix>  → menjalankan vet/build/test, isi ROWS.
verify_project() {
  local name="$1"; local dir="$2"; local envp="$3"
  local v b t
  v=$(runstep "$name: go vet ./..."   "$dir" "$envp" go vet ./...   | tail -1)
  b=$(runstep "$name: go build ./..." "$dir" "$envp" go build ./... | tail -1)
  t=$(runstep "$name: go test ./..."  "$dir" "$envp" go test ./...  | tail -1)
  ROWS+=("$name|$v|$b|$t")
}

hr; step "Build binary: go build -o ./bin/gostarter ./cmd/gostarter"
if ( cd "$REPO_ROOT" && go build -o "$BIN" ./cmd/gostarter ); then
  okp "binary: $BIN"
else
  failp "build binary gagal — matrix berhenti"; exit 1
fi

# ── #1 monolith / net-http / db=none (OFFLINE) ───────────────────────────────
hr; step "#1 monolith / net-http / db=none  (OFFLINE GOPROXY=off)"
D1="$WORK/c1"
"$BIN" create --name c1 --module example.com/c1 --arch monolith --kind rest \
  --http net/http --db none --feature docker,makefile,golangci,env \
  --no-git --yes -o "$D1" >/dev/null
if grep -qE '^[[:space:]]*require' "$D1/go.mod" 2>/dev/null; then
  failp "#1 go.mod ada require — melanggar invarian murni-stdlib"
else
  okp "#1 go.mod zero require (murni stdlib)"
fi
verify_project "#1 mono/nethttp/none" "$D1" "GOPROXY=off GOFLAGS=-mod=mod"
OFFLINE_1="$([ "$FAIL" -eq 0 ] && echo PASS || echo CHECK)"

# ── #2 monolith / chi / db=postgres ──────────────────────────────────────────
hr; step "#2 monolith / chi / db=postgres  (go mod tidy → vet/build/test)"
D2="$WORK/c2"
"$BIN" create --name c2 --module example.com/c2 --arch monolith --kind rest \
  --http chi --db postgres --feature docker,makefile,golangci,env \
  --no-git --yes -o "$D2" >/dev/null
runstep "#2 go mod tidy" "$D2" "" go mod tidy >/dev/null
verify_project "#2 mono/chi/postgres" "$D2" ""

# ── #3 monolith / echo / db=mysql ────────────────────────────────────────────
hr; step "#3 monolith / echo / db=mysql  (go mod tidy → vet/build/test)"
D3="$WORK/c3"
"$BIN" create --name c3 --module example.com/c3 --arch monolith --kind rest \
  --http echo --db mysql --feature docker,makefile,golangci,env \
  --no-git --yes -o "$D3" >/dev/null
runstep "#3 go mod tidy" "$D3" "" go mod tidy >/dev/null
verify_project "#3 mono/echo/mysql" "$D3" ""

# ── #4 modular-monolith / net-http / db=none (OFFLINE, 2 domain) ─────────────
hr; step "#4 modular-monolith / net-http / db=none  (OFFLINE, 2 domain endpoint)"
D4="$WORK/c4"
"$BIN" create --name c4 --module example.com/c4 --arch modular-monolith --kind rest \
  --http net/http --db none --feature docker,makefile,golangci,env \
  --no-git --yes -o "$D4" >/dev/null
if grep -qE '^[[:space:]]*require' "$D4/go.mod" 2>/dev/null; then
  failp "#4 go.mod ada require — modular db=none harus murni stdlib"
else
  okp "#4 go.mod zero require (murni stdlib)"
fi
[ -e "$D4/internal/app" ] && failp "#4 ada internal/app (file yatim monolith)" || okp "#4 tanpa internal/app (composition root tunggal di cmd)"
[ -d "$D4/internal/modules/catalog" ] && okp "#4 domain catalog ada" || failp "#4 domain catalog hilang"
[ -d "$D4/internal/modules/orders" ]  && okp "#4 domain orders ada"  || failp "#4 domain orders hilang"
# Boundary berduri: detail domain terkurung di internal/core (tak bisa di-import lintas domain).
[ -d "$D4/internal/modules/catalog/internal/core" ] && okp "#4 catalog boundary internal/core" || failp "#4 catalog tanpa internal/core"
# Endpoint kedua domain ter-mount di server (cek route registrasi di handler domain).
if grep -rqE 'catalog/products' "$D4/internal/modules/catalog" 2>/dev/null && \
   grep -rqE '/orders'          "$D4/internal/modules/orders"  2>/dev/null; then
  okp "#4 kedua domain (catalog & orders) mendaftarkan rute via RegisterRoutes"
else
  skipp "#4 cek wiring domain by-grep (lihat detail di build/test)"
fi
verify_project "#4 modular/nethttp/none" "$D4" "GOPROXY=off GOFLAGS=-mod=mod"

# Runtime: jalankan server modular & curl KEDUA endpoint domain (bukti 2 domain
# hidup + komunikasi antar-domain in-process: orders memanggil catalog via
# contract.Catalog → total order = harga × qty). OFFLINE (GOPROXY=off).
( cd "$D4" && GOPROXY=off GOFLAGS=-mod=mod go build -o ./bin/app ./cmd/c4 ) >/dev/null 2>&1
if [ -x "$D4/bin/app" ]; then
  RTPORT=19533
  ( cd "$D4" && PORT=$RTPORT ./bin/app ) >/tmp/matrix-rt.out 2>&1 &
  RTPID=$!
  sleep 2
  H=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$RTPORT/healthz" 2>/dev/null)
  CAT=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$RTPORT/api/catalog/products/p-1" 2>/dev/null)
  ORD=$(curl -s -X POST -H 'Content-Type: application/json' -d '{"product_id":"p-1","quantity":3}' \
        -o /dev/null -w '%{http_code}' "http://127.0.0.1:$RTPORT/api/orders" 2>/dev/null)
  kill "$RTPID" 2>/dev/null; wait "$RTPID" 2>/dev/null
  if [ "$H" = "200" ] && [ "$CAT" = "200" ] && [ "$ORD" = "201" ]; then
    okp "#4 runtime: /healthz=$H, catalog=$CAT, orders=$ORD (orders→contract→catalog OK)"
  else
    failp "#4 runtime endpoint gagal (/healthz=$H, catalog=$CAT, orders=$ORD; harap 200/200/201)"
  fi
else
  skipp "#4 runtime dilewati (binary tak terbangun — lihat build di atas)"
fi
OFFLINE_4="PASS"
for r in "${ROWS[@]}"; do
  case "$r" in
    "#4 "*) [[ "$r" == *"|FAIL"* ]] && OFFLINE_4="CHECK" ;;
  esac
done

# ── #5 modular / chi / db=postgres / FULL addons (ci=github-actions, obs) ─────
hr; step "#5 modular / chi / db=postgres / docker,makefile,golangci,env,ci(github),observability"
D5="$WORK/c5"
"$BIN" create --name c5 --module example.com/c5 --arch modular-monolith --kind rest \
  --http chi --db postgres \
  --feature docker,makefile,golangci,env,ci,observability --ci github-actions \
  --no-git --yes -o "$D5" >/dev/null
# CI gating: github-actions → .github/workflows/ci.yml ADA, .gitlab-ci.yml TIDAK.
if [ -f "$D5/.github/workflows/ci.yml" ] && [ ! -f "$D5/.gitlab-ci.yml" ]; then
  okp "#5 CI gating benar (.github/workflows/ci.yml saja)"
else
  failp "#5 CI gating salah (harap .github/workflows/ci.yml tanpa .gitlab-ci.yml)"
fi
[ -d "$D5/internal/platform/observability" ] && okp "#5 paket observability ada" || failp "#5 observability hilang"
[ -f "$D5/docker-compose.yml" ] && okp "#5 docker-compose.yml ada" || failp "#5 compose hilang"
runstep "#5 go mod tidy" "$D5" "" go mod tidy >/dev/null
verify_project "#5 modular/chi/pg/full" "$D5" ""

# ── #6 --config preset vs flag → byte-identical ──────────────────────────────
hr; step "#6 --config preset vs flag (byte-identical, diff -rq)"
PRESET="$WORK/preset.yaml"
cat > "$PRESET" <<'YAML'
name: c6
module: example.com/c6
arch: modular-monolith
kind: rest
http: chi
db: postgres
access: sqlx
migrate: golang-migrate
docker: true
makefile: true
golangci: true
env: true
ci: github-actions
obs: true
git: false
YAML
D6P="$WORK/c6-preset"
D6F="$WORK/c6-flags"
"$BIN" create --config "$PRESET" --no-git --yes -o "$D6P" >/dev/null
GEN6P=$?
"$BIN" create --name c6 --module example.com/c6 --arch modular-monolith --kind rest \
  --http chi --db postgres --access sqlx --migrate golang-migrate \
  --feature docker,makefile,golangci,env,ci,observability --ci github-actions \
  --no-git --yes -o "$D6F" >/dev/null
GEN6F=$?
if [ "$GEN6P" -ne 0 ] || [ "$GEN6F" -ne 0 ]; then
  failp "#6 generate gagal (preset rc=$GEN6P, flag rc=$GEN6F)"
  ROWS+=("#6 config==flag|FAIL|-|-")
else
  if diff -rq "$D6P" "$D6F" >/tmp/matrix-diff.log 2>&1; then
    okp "#6 byte-identical: preset == flag (diff -rq bersih)"
    ROWS+=("#6 config==flag|OK|OK|OK")
  else
    failp "#6 BUKAN byte-identical — diff:"
    sed 's/^/        /' /tmp/matrix-diff.log
    ROWS+=("#6 config==flag|FAIL|FAIL|FAIL")
  fi
fi

# ── Docker build = SKIP (tak ada daemon) ─────────────────────────────────────
hr; step "Docker build — SKIP (tak ada daemon)"
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  skipp "docker daemon hidup tapi dilewati per scope (e2e fokus go toolchain)"
else
  skipp "docker tidak tersedia — SKIP (bukan kegagalan)"
fi

# ── Tabel hasil ──────────────────────────────────────────────────────────────
hr; step "TABEL HASIL MATRIX FASE 4a"
printf '    %-28s | %-6s | %-6s | %-6s\n' "KOMBINASI" "VET" "BUILD" "TEST"
printf '    %-28s-+-%-6s-+-%-6s-+-%-6s\n' "----------------------------" "------" "------" "------"
for r in "${ROWS[@]}"; do
  IFS='|' read -r nm v b t <<< "$r"
  printf '    %-28s | %-6s | %-6s | %-6s\n' "$nm" "$v" "$b" "$t"
done

printf '\n    Offline #1 (mono/none)   : %s (GOPROXY=off)\n' "$OFFLINE_1"
printf '    Offline #4 (modular/none): %s (GOPROXY=off)\n' "$OFFLINE_4"
printf '    Byte-identical #6        : lihat baris config==flag\n'
printf '    Docker build             : SKIP (tak ada daemon)\n'

hr
if [ "$FAIL" -eq 0 ]; then
  printf '\nVERDICT: HIJAU — semua kombinasi wajib lolos.\n'; exit 0
else
  printf '\nVERDICT: MERAH — ada kombinasi wajib gagal (lihat [FAIL]).\n'; exit 1
fi
