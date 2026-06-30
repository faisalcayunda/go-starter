# Dockerfile multi-stage — build statis Go lalu jalankan di runtime minimal.
# Stage builder: kompilasi biner statis (CGO_ENABLED=0) agar muat di distroless.
# Stage runtime: gcr.io/distroless/static (non-root, tanpa shell — permukaan kecil).

# --- builder ---
FROM golang:1.26-alpine AS builder
WORKDIR /src

# Cache layer dependency: salin go.mod/go.sum dulu, baru download.
COPY go.mod go.sum* ./
RUN go mod download

# Salin sisa source & build biner statis.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/shopdemo ./cmd/shopdemo

# --- runtime ---
FROM gcr.io/distroless/static:nonroot
WORKDIR /app

COPY --from=builder /out/shopdemo /app/shopdemo

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/shopdemo"]
