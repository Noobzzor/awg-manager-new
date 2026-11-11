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
RUN go build -mod=vendor -trimpath -ldflags="-s -w" -tags embed_frontend -o /out/awg-manager ./cmd/awg-manager \
 && go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/awg-panel-socket ./docker/original-preview/panel-socket.go

FROM ${RUNTIME_IMAGE} AS tunnel-tools

FROM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251 AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl iproute2 iptables nftables procps util-linux wireguard-tools \
 && rm -rf /var/lib/apt/lists/*
COPY --from=tunnel-tools /usr/local/bin/sing-box /usr/local/bin/amneziawg-go /usr/local/bin/awg /usr/local/bin/
COPY --from=tunnel-tools /usr/local/share/awgm-trial/LICENSE-awg /usr/local/share/awgm-trial/LICENSE-manager /usr/local/share/awgm-trial/NOTICE /usr/local/share/awgm-trial/
COPY --from=builder --chmod=0755 /out/awg-manager /usr/local/bin/awg-manager
COPY --from=builder --chmod=0755 /out/awg-panel-socket /usr/local/bin/awg-panel-socket
ENV AWG_MODE=singbox \
    AWG_PROXY_ADDR=127.0.0.1:1080 \
    SINGBOX_BIN=/usr/local/bin/sing-box \
    AWG_GATEWAY_ENABLE=true \
    AWG_GATEWAY_BACKEND=userspace-awg \
    AWG_GATEWAY_RUNTIME_BINARY=/usr/local/bin/amneziawg-go \
    AWG_GATEWAY_IP_BINARY=/usr/sbin/ip \
    AWG_GATEWAY_TOOL=/usr/local/bin/awg \
    AWG_GATEWAY_INTERFACE=awg0 \
    AWG_GATEWAY_WAN_INTERFACE=eth0 \
    AWG_GATEWAY_PORT=51820 \
    AWG_GATEWAY_CLIENT_POOL=10.77.0.0/24 \
    AWG_GATEWAY_SERVER_ADDRESS=10.77.0.1/24 \
    AWG_GATEWAY_PARAMETERS='{"jc":3,"jmin":40,"jmax":80,"s1":16,"s2":16,"s3":16,"s4":16,"h1":1001,"h2":1002,"h3":1003,"h4":1004}' \
    AWG_ROUTER_POLICY_MODE=original \
    AWG_ORIGINAL_BROWSER_PREVIEW=1 \
    AWG_HTTP_ADDR=127.0.0.1:2222 \
    AWG_DATA_DIR=/data/original \
    SINGBOX_CONFIG_DIR=/data/original/sing-box/config.d
HEALTHCHECK --interval=15s --timeout=3s --retries=4 CMD ["curl","-fsS","http://127.0.0.1:2222/api/health"]
ENTRYPOINT ["/usr/local/bin/awg-manager"]
CMD []
