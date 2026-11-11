ARG RUNTIME_IMAGE=awgm-personal-trial:20261021-dns1

FROM node:22-bookworm-slim@sha256:48e4b67d85f87bd551df43704e24d252f56cc5f8e9718841aace50f19948f0f9 AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
COPY internal/presets/defaults.json /src/internal/presets/defaults.json
RUN npm run build

FROM golang:1.26-bookworm@sha256:a688600ca24f8a4d3ca77f95b0dd40704a9fc787c826660eb7ba0b641b8b175d AS builder
WORKDIR /src
COPY . .
COPY --from=frontend /src/frontend/build ./frontend/build
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local
RUN go build -mod=vendor -trimpath -tags embed_frontend -o /out/awg-manager ./cmd/awg-manager \
 && go build -mod=vendor -trimpath -o /out/awg-panel-socket ./docker/original-preview/panel-socket.go

FROM ${RUNTIME_IMAGE}
COPY --from=builder --chmod=0755 /out/awg-manager /usr/local/bin/awg-manager
COPY --from=builder --chmod=0755 /out/awg-panel-socket /usr/local/bin/awg-panel-socket
ENV AWG_ROUTER_POLICY_MODE=original \
    AWG_ORIGINAL_BROWSER_PREVIEW=1 \
    AWG_HTTP_ADDR=127.0.0.1:2222 \
    AWG_DATA_DIR=/data/original \
    SINGBOX_CONFIG_DIR=/data/original/sing-box/config.d
HEALTHCHECK --interval=15s --timeout=3s --retries=4 CMD ["curl","-fsS","http://127.0.0.1:2222/api/health"]
ENTRYPOINT ["/usr/local/bin/awg-manager"]
CMD []
