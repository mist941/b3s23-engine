# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26.4@sha256:d184d9be4c13614e28498d632eeaaac704d662f18ad357e1df74a44424236cea AS go-builder
WORKDIR /src
COPY engine/go.mod engine/go.sum ./
RUN go mod download
COPY engine/ ./
ARG TARGETOS TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/engined ./cmd/engined \
    && mkdir -p /out/data

FROM --platform=$BUILDPLATFORM node:26-slim@sha256:95a34da32a840bd9b3b09a5b773591c16923e350174b1c50e1200c75bf15eaa9 AS node-builder
WORKDIR /app
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

FROM gcr.io/distroless/static-debian12:nonroot@sha256:b7bb25d9f7c31d2bdd1982feb4dafcaf137703c7075dbe2febb41c24212b946f
LABEL org.opencontainers.image.source="https://github.com/mist941/b3s23-engine" \
      org.opencontainers.image.description="Conway's Game of Life (B3/S23) engine: Go simulation core, SQLite persistence, REST + WebSocket API, WebGL2 UI" \
      org.opencontainers.image.licenses="MIT"
COPY --from=go-builder /out/engined /usr/local/bin/engined
COPY --from=node-builder /app/dist /ui
COPY --from=go-builder --chown=65532:65532 /out/data /data
USER nonroot
EXPOSE 8050
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["engined", "-healthcheck", "-addr", ":8050"]

ENTRYPOINT ["engined", "-addr", ":8050", "-db", "/data/engine.db", "-static", "/ui"]
