# demo/demo.tape — skrip rekaman demo gostarter untuk vhs (charmbracelet/vhs).
#
# Render:   vhs demo/demo.tape      → menghasilkan assets/demo.gif
# Prasyarat: vhs (https://github.com/charmbracelet/vhs) + binary `gostarter` di PATH.
#            Build dulu bila perlu: go build -o gostarter ./cmd/gostarter
#
# Tape ini merekam alur create gostarter: cek versi, mulai wizard interaktif
# (huh) lalu navigasi beberapa langkah, dan tutup dengan jalur non-interaktif
# satu-baris yang menghasilkan project lengkap + tree-nya.

# ── Output & tema ────────────────────────────────────────────────────────────
Output assets/demo.gif

Set Shell      "bash"
Set FontSize   18
Set Width      1200
Set Height     760
Set Padding    24
Set Theme      "Dracula"
Set TypingSpeed 45ms

# ── Intro: versi ─────────────────────────────────────────────────────────────
Type "gostarter --version"
Sleep 400ms
Enter
Sleep 1500ms

# ── Wizard interaktif (huh) ──────────────────────────────────────────────────
Type "gostarter create"
Sleep 500ms
Enter
Sleep 1800ms

# Nama project?
Type "shop"
Sleep 700ms
Enter
Sleep 1000ms

# Go module path? (terima default github.com/shop)
Sleep 700ms
Enter
Sleep 1000ms

# Tipe arsitektur? — pilih modular-monolith (panah bawah sekali)
Down
Sleep 800ms
Enter
Sleep 1500ms

# Catatan modular monolith — lanjut
Sleep 900ms
Enter
Sleep 1200ms

# Keluar dari wizard untuk demo (Ctrl+C), lanjut ke jalur non-interaktif.
Ctrl+C
Sleep 1200ms

# ── Jalur non-interaktif (flag) — generate penuh ─────────────────────────────
Type "gostarter create --name shop --arch modular-monolith \"
Sleep 300ms
Enter
Type "  --http chi --db postgres --addons makefile,env,golangci \"
Sleep 300ms
Enter
Type "  --non-interactive --yes"
Sleep 600ms
Enter
Sleep 2500ms

# ── Hasil: lihat struktur project ────────────────────────────────────────────
Type "cd shop && go build ./... && echo BUILD-OK"
Sleep 500ms
Enter
Sleep 3000ms

Type "git ls-files 2>/dev/null || find . -type f -not -path './.git/*' | sort"
Sleep 500ms
Enter
Sleep 3500ms

# ── Outro ────────────────────────────────────────────────────────────────────
Type "# project Go siap pakai — zero lock-in. gostarter 🚀"
Sleep 600ms
Enter
Sleep 2500ms
