# Dockerfile multi-service — satu image, service dipilih lewat build arg SERVICE.
# docker-compose membangun image ini sekali per service dengan args.SERVICE=<nama>.
# Stub gRPC sudah DI-COMMIT (gen/), jadi build TIDAK memerlukan buf.

FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache layer dependency: salin go.mod/go.sum lebih dulu.
COPY go.mod go.sum* ./
RUN go mod download

# Salin seluruh source monorepo (single-module).
COPY . .

# Service yang dibangun ditentukan build arg (mis. "order").
ARG SERVICE
RUN test -n "$SERVICE" || (echo "build arg SERVICE wajib diisi" && exit 1)
RUN CGO_ENABLED=0 go build -o /out/service ./services/${SERVICE}/cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/service /service
USER nonroot:nonroot
ENTRYPOINT ["/service"]
