# LearningService: gRPC and HTTP /health on one port, compiled to a static
# binary and run on a distroless base.

# ---- build ------------------------------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src
# Manifests first so the module download layer is cached between builds.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/server

# ---- runtime: no shell, no package manager, runs as uid 65532 ---------------
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/app /app
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
# The binary probes its own /health endpoint; distroless has no curl.
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 CMD ["/app", "healthcheck"]
ENTRYPOINT ["/app"]
