# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26.4@sha256:11fd8f7f63db3b6fb198797042ba4c40a4a34dc83325d3328ca3bc4bb7726786 AS go-builder
WORKDIR /src
COPY engine/go.mod engine/go.sum ./
RUN go mod download
COPY engine/ ./
ARG TARGETOS TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/engined ./cmd/engined
RUN mkdir -p /out/data

FROM --platform=$BUILDPLATFORM node:24-slim@sha256:242549cd46785b480c832479a730f4f2a20865d61ea2e404fdb2a5c3d3b73ecf AS node-builder
WORKDIR /app
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

FROM gcr.io/distroless/static-debian12
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
