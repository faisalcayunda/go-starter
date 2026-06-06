#!/usr/bin/env bash
#
# smoke.sh — smoke test otomatis untuk gostarter (Fase 3 / T3.8).
#
# Membangun binary builder lalu memverifikasi dua varian project hasil generate:
#
#   VAR A (offline-safe, db=none)  : MURNI STDLIB → "go vet/build/test ./..." HIJAU
#                                    OFFLINE, zero require di go.mod (INVARIAN keras).
#   VAR B (db=postgres)            : deps eksternal (pgx/v5) → "go mod tidy" (butuh
#                                    jaringan) + "go vet" + "go build". go test ringan
#                                    (tanpa DB live).
#
# docker build dijalankan untuk VAR A HANYA bila docker tersedia & daemon hidup;
# bila tidak → SKIP (bukan kegagalan).
#
# Exit non-zero bila ada langkah WAJIB gagal:
#   - VAR A: vet, build, test
#   - VAR B: build (vet juga diwajibkan; go test VAR B opsional/ringan)
#
# Lihat: docs/SPEC.md §5.1 (flags), §6 (constraint), ADR-002/ADR-003 (kontrak).

set -u  # variabel tak terdefinisi = error. (sengaja TANPA -e: kita kelola exit
        # per-langkah agar bisa membersihkan temp dir & melaporkan VERDICT.)

# ── Lokasi & konstanta ───────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$REPO_ROOT/bin/gostarter"

TMPBASE="${TMPDIR:-/tmp}"
DIR_A="$TMPBASE/gostarter-smoke-none"
DIR_B="$TMPBASE/gostarter-smoke-postgres"
# Fase 4a: dua varian tambahan menutupi arch modular + router chi + db mysql +
# add-on ci/observability.
DIR_C="$TMPBASE/gostarter-smoke-modular-none"        # modular + net/http + db=none (offline stdlib)
DIR_D="$TMPBASE/gostarter-smoke-modular-chi-mysql"   # modular + chi + mysql + ci + obs (ext deps)

# Set add-on yang diimplementasi di Fase 3 (lihat scope-lock).
# README.md & .gitignore BUKAN add-on — selalu dari core (C-1).
FEATURES="docker,makefile,golangci,env"
# Fase 4a: superset add-on termasuk ci + observability.
FEATURES_4A="docker,makefile,golangci,env,ci,observability"

# Status akhir; di-set ke 1 oleh fail().
FAIL=0

# ── Util tampilan ────────────────────────────────────────────────────────────
hr()   { printf '%s\n' "------------------------------------------------------------"; }
step() { printf '\n>>> %s\n' "$*"; }
ok()   { printf '    [OK]   %s\n' "$*"; }
skip() { printf '    [SKIP] %s\n' "$*"; }
fail() { printf '    [FAIL] %s\n' "$*"; FAIL=1; }

# run <label> <dir> <cmd...>
# Menjalankan perintah di <dir>, mencetak output, mengembalikan exit code asli.
run() {
  local label="$1"; shift
  local dir="$1"; shift
  printf '\n--- %s ---\n' "$label"
  ( cd "$dir" && "$@" )
  local rc=$?
  if [ "$rc" -eq 0 ]; then ok "$label"; else fail "$label (exit $rc)"; fi
  return "$rc"
}

# ── Pembersihan temp dir di akhir (selalu) ───────────────────────────────────
cleanup() {
  rm -rf "$DIR_A" "$DIR_B" "$DIR_C" "$DIR_D"
}
trap cleanup EXIT

# ── 0. Build binary builder ──────────────────────────────────────────────────
hr
step "Build binary: go build -o ./bin/gostarter ./cmd/gostarter"
if ( cd "$REPO_ROOT" && go build -o "$BIN" ./cmd/gostarter ); then
  ok "binary terbangun: $BIN"
else
  fail "gagal build binary builder — smoke berhenti"
  exit 1
fi

# Pastikan temp dir bersih sebelum generate (EnsureEmptyDir menolak dir non-kosong).
rm -rf "$DIR_A" "$DIR_B" "$DIR_C" "$DIR_D"

# ── VAR A — db=none (offline-safe, murni stdlib) ─────────────────────────────
hr
step "VAR A — generate db=none (murni stdlib) → $DIR_A"
"$BIN" create \
  --name demo-none \
  --module example.com/demo-none \
  --arch monolith \
  --kind rest \
  --http net/http \
  --db none \
  --feature "$FEATURES" \
  --no-git \
  --yes \
  -o "$DIR_A"
GEN_A_RC=$?
if [ "$GEN_A_RC" -eq 0 ]; then ok "generate VAR A"; else fail "generate VAR A (exit $GEN_A_RC)"; fi

if [ "$GEN_A_RC" -eq 0 ]; then
  # INVARIAN: db=none → zero require di go.mod (murni stdlib).
  step "VAR A — cek invarian: zero require di go.mod"
  if grep -qE '^[[:space:]]*require' "$DIR_A/go.mod" 2>/dev/null; then
    fail "go.mod VAR A mengandung 'require' — melanggar invarian murni-stdlib"
    printf '    go.mod:\n'; sed 's/^/      /' "$DIR_A/go.mod"
  else
    ok "go.mod VAR A tanpa require (murni stdlib)"
  fi

  # Verifikasi OFFLINE: paksa GOPROXY=off agar membuktikan tak ada fetch jaringan.
  # GOFLAGS=-mod=mod menghindari kegagalan akibat ketiadaan go.sum saat -mod=readonly.
  step "VAR A — go vet / go build / go test (OFFLINE: GOPROXY=off)"
  GOPROXY=off GOFLAGS=-mod=mod run "VAR A: go vet ./..."   "$DIR_A" go vet ./...
  GOPROXY=off GOFLAGS=-mod=mod run "VAR A: go build ./..." "$DIR_A" go build ./...
  GOPROXY=off GOFLAGS=-mod=mod run "VAR A: go test ./..."  "$DIR_A" go test ./...
fi

# ── VAR B — db=postgres (deps eksternal) ─────────────────────────────────────
hr
step "VAR B — generate db=postgres (deps eksternal) → $DIR_B"
"$BIN" create \
  --name demo-pg \
  --module example.com/demo-pg \
  --arch monolith \
  --kind rest \
  --http net/http \
  --db postgres \
  --feature "$FEATURES" \
  --no-git \
  --yes \
  -o "$DIR_B"
GEN_B_RC=$?
if [ "$GEN_B_RC" -eq 0 ]; then ok "generate VAR B"; else fail "generate VAR B (exit $GEN_B_RC)"; fi

if [ "$GEN_B_RC" -eq 0 ]; then
  # go mod tidy butuh jaringan (resolusi pgx + transitive). WAJIB sukses agar
  # go.sum lengkap & build dapat berjalan.
  step "VAR B — go mod tidy (butuh jaringan) / go vet / go build"
  run "VAR B: go mod tidy"     "$DIR_B" go mod tidy
  run "VAR B: go vet ./..."    "$DIR_B" go vet ./...
  run "VAR B: go build ./..."  "$DIR_B" go build ./...
  # go test VAR B = ringan: tidak butuh DB live (test contoh stdlib di handler).
  # Tidak diwajibkan untuk VERDICT, tetapi dijalankan untuk informasi.
  printf '\n--- VAR B: go test ./... (ringan, informasional) ---\n'
  if ( cd "$DIR_B" && go test ./... ); then
    ok "VAR B: go test ./... (informasional)"
  else
    skip "VAR B: go test ./... gagal — tidak diwajibkan (butuh DB live?)"
  fi
fi

# ── VAR C — modular-monolith + net/http + db=none (Fase 4a, offline stdlib) ──
hr
step "VAR C — generate modular-monolith db=none (murni stdlib) → $DIR_C"
"$BIN" create \
  --name demo-modular \
  --module example.com/demo-modular \
  --arch modular-monolith \
  --kind rest \
  --http net/http \
  --db none \
  --feature "$FEATURES" \
  --no-git \
  --yes \
  -o "$DIR_C"
GEN_C_RC=$?
if [ "$GEN_C_RC" -eq 0 ]; then ok "generate VAR C"; else fail "generate VAR C (exit $GEN_C_RC)"; fi

if [ "$GEN_C_RC" -eq 0 ]; then
  # INVARIAN: modular + db=none → tetap zero require (murni stdlib) & tanpa file
  # yatim (tak ada internal/app sisa monolith).
  step "VAR C — cek invarian: zero require + tanpa file yatim monolith"
  if grep -qE '^[[:space:]]*require' "$DIR_C/go.mod" 2>/dev/null; then
    fail "go.mod VAR C mengandung 'require' — modular db=none harus murni stdlib"
  else
    ok "go.mod VAR C tanpa require (murni stdlib)"
  fi
  if [ -e "$DIR_C/internal/app" ]; then
    fail "VAR C mengandung internal/app (file yatim monolith pada modular)"
  else
    ok "VAR C tanpa internal/app (composition root tunggal di cmd — benar)"
  fi

  step "VAR C — go vet / go build / go test (OFFLINE: GOPROXY=off)"
  GOPROXY=off GOFLAGS=-mod=mod run "VAR C: go vet ./..."   "$DIR_C" go vet ./...
  GOPROXY=off GOFLAGS=-mod=mod run "VAR C: go build ./..." "$DIR_C" go build ./...
  GOPROXY=off GOFLAGS=-mod=mod run "VAR C: go test ./..."  "$DIR_C" go test ./...
fi

# ── VAR D — modular + chi + mysql + ci + observability (Fase 4a, ext deps) ────
hr
step "VAR D — generate modular + chi + mysql + ci(gitlab) + obs → $DIR_D"
"$BIN" create \
  --name demo-modchi \
  --module example.com/demo-modchi \
  --arch modular-monolith \
  --kind rest \
  --http chi \
  --db mysql \
  --feature "$FEATURES_4A" \
  --ci gitlab-ci \
  --no-git \
  --yes \
  -o "$DIR_D"
GEN_D_RC=$?
if [ "$GEN_D_RC" -eq 0 ]; then ok "generate VAR D"; else fail "generate VAR D (exit $GEN_D_RC)"; fi

if [ "$GEN_D_RC" -eq 0 ]; then
  # CI provider: gitlab-ci → .gitlab-ci.yml ADA, .github TIDAK ada (gating `when`).
  step "VAR D — cek CI provider gating (.gitlab-ci.yml ada, .github tidak)"
  if [ -f "$DIR_D/.gitlab-ci.yml" ] && [ ! -d "$DIR_D/.github" ]; then
    ok "CI gating benar (.gitlab-ci.yml saja)"
  else
    fail "CI gating salah (harap .gitlab-ci.yml tanpa .github)"
  fi
  # go mod tidy butuh jaringan (chi + mysql + otel + prometheus).
  step "VAR D — go mod tidy / go vet / go build / go test"
  run "VAR D: go mod tidy"     "$DIR_D" go mod tidy
  run "VAR D: go vet ./..."    "$DIR_D" go vet ./...
  run "VAR D: go build ./..."  "$DIR_D" go build ./...
  run "VAR D: go test ./..."   "$DIR_D" go test ./...
fi

# ── Docker build (VAR A) — opsional ──────────────────────────────────────────
hr
step "Docker build (VAR A) — opsional"
DOCKER_STATUS="skip"
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  DOCKER_STATUS="ran"
  if [ "$GEN_A_RC" -eq 0 ] && [ -f "$DIR_A/Dockerfile" ]; then
    run "docker build VAR A" "$DIR_A" docker build -t gostarter-smoke-none:latest .
    # Bersihkan image agar tak menumpuk (best-effort).
    docker image rm gostarter-smoke-none:latest >/dev/null 2>&1 || true
  else
    skip "docker tersedia tetapi VAR A tidak ter-generate / tanpa Dockerfile"
  fi
else
  skip "docker tidak tersedia (binary/daemon) — bukan kegagalan smoke"
fi

# ── VERDICT ──────────────────────────────────────────────────────────────────
hr
step "VERDICT"
printf '    VAR A (db=none, offline)          : vet/build/test %s\n' \
  "$([ "$GEN_A_RC" -eq 0 ] && echo 'dijalankan' || echo 'TIDAK ter-generate')"
printf '    VAR B (db=postgres)               : build %s\n' \
  "$([ "$GEN_B_RC" -eq 0 ] && echo 'dijalankan' || echo 'TIDAK ter-generate')"
printf '    VAR C (modular, db=none, offline) : vet/build/test %s\n' \
  "$([ "$GEN_C_RC" -eq 0 ] && echo 'dijalankan' || echo 'TIDAK ter-generate')"
printf '    VAR D (modular+chi+mysql+ci+obs)  : tidy/vet/build/test %s\n' \
  "$([ "$GEN_D_RC" -eq 0 ] && echo 'dijalankan' || echo 'TIDAK ter-generate')"
printf '    Docker                            : %s\n' "$DOCKER_STATUS"
hr
if [ "$FAIL" -eq 0 ]; then
  printf '\nVERDICT: HIJAU — semua langkah wajib lolos.\n'
  exit 0
else
  printf '\nVERDICT: MERAH — ada langkah wajib yang gagal (lihat [FAIL] di atas).\n'
  exit 1
fi
