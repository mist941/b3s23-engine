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

FROM --platform=$BUILDPLATFORM node:26-slim@sha256:191ef878ecb351d68b78219593de18bd8942afd59af59f29960dc4b24805a3f1 AS node-builder
WORKDIR /app
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
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
