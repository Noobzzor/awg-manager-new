# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM node:22-bookworm-slim@sha256:48e4b67d85f87bd551df43704e24d252f56cc5f8e9718841aace50f19948f0f9 AS frontend-assets
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
COPY internal/presets/defaults.json /src/internal/presets/defaults.json
RUN npm run build

FROM frontend-assets AS frontend
RUN npm run check && npm test

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm@sha256:a688600ca24f8a4d3ca77f95b0dd40704a9fc787c826660eb7ba0b641b8b175d AS singbox-builder
ARG TARGETARCH
ARG AWG_VERSION=2.19.0
ARG AMNEZIA_BOX_VERSION=1.14.0-awgm.16
ARG AMNEZIA_BOX_COMMIT=d231098f6f1a7bd90c690ecda1cf5e9176b32c2a
ARG CRONET_GO_COMMIT=45832ab074849607406baa3e3a2c4660274602ed
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl file git gnupg python3 python3-requests unzip xz-utils zip \
    && rm -rf /var/lib/apt/lists/*

# The pinned release metadata contains no verified linux/amd64 asset. Build the
# fork from the exact release commit, following its release-entware workflow.
RUN case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 64 ;; esac
WORKDIR /src/amnezia-box
RUN mkdir -p /out \
    && git init \
    && git remote add origin https://github.com/hoaxisr/amnezia-box.git \
    && git fetch --depth=1 origin "$AMNEZIA_BOX_COMMIT" \
    && git checkout --detach FETCH_HEAD \
    && test "$(git rev-parse HEAD)" = "$AMNEZIA_BOX_COMMIT"
RUN git clone --no-checkout https://github.com/sagernet/cronet-go.git /root/cronet-go \
    && git -C /root/cronet-go fetch --depth=1 origin "$CRONET_GO_COMMIT" \
    && git -C /root/cronet-go checkout --detach FETCH_HEAD \
    && test "$(git -C /root/cronet-go rev-parse HEAD)" = "$CRONET_GO_COMMIT" \
    && git -C /root/cronet-go submodule update --init --recursive --depth=1 \
    && rm -f /root/cronet-go/naiveproxy/src/build/linux/sysroot_scripts/keyring.gpg \
    && (cd /root/cronet-go && GPG_TTY=/dev/null ./naiveproxy/src/build/linux/sysroot_scripts/generate_keyring.sh)
RUN cd /root/cronet-go \
    && go run ./cmd/build-naive --target="linux/$TARGETARCH" --libc=musl download-toolchain
RUN eval "$(cd /root/cronet-go && go run ./cmd/build-naive --target="linux/$TARGETARCH" --libc=musl env --export)" \
    && TAGS="$(cat release/DEFAULT_BUILD_TAGS),with_musl,with_awg,with_low_memory" \
    && CGO_ENABLED=1 GOOS=linux GOARCH="$TARGETARCH" go build -trimpath \
      -tags "$TAGS" \
      -ldflags "-X 'github.com/sagernet/sing-box/constant.Version=$AMNEZIA_BOX_VERSION' $(cat release/LDFLAGS) -s -w -buildid=" \
      -o /out/sing-box ./cmd/sing-box

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm@sha256:a688600ca24f8a4d3ca77f95b0dd40704a9fc787c826660eb7ba0b641b8b175d AS builder
ARG TARGETARCH
ARG AWG_VERSION=2.19.0
WORKDIR /src/awg-manager
COPY . .
COPY --from=frontend /src/frontend/build ./frontend/build
RUN CGO_ENABLED=0 GOOS=linux GOARCH="$TARGETARCH" \
    go build -mod=vendor -trimpath -tags embed_frontend \
      -ldflags="-s -w -buildid= -X main.version=$AWG_VERSION" \
      -o /out/awg-manager ./cmd/awg-manager

FROM --platform=$TARGETPLATFORM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251 AS singbox-verify
ARG AMNEZIA_BOX_VERSION=1.14.0-awgm.16
COPY --from=singbox-builder /out/sing-box /usr/local/bin/sing-box
RUN /usr/local/bin/sing-box version | grep -F "sing-box version $AMNEZIA_BOX_VERSION" \
    && /usr/local/bin/sing-box version | grep -F 'with_awg'

FROM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251 AS runtime-base
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl iproute2 iptables nftables procps util-linux wireguard-tools \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 awg \
    && useradd --uid 10001 --gid awg --home-dir /data --no-create-home --shell /usr/sbin/nologin awg \
    && install -d -o awg -g awg -m 0750 /data /data/sing-box /data/sing-box/config.d

FROM golang:1.26-bookworm@sha256:a688600ca24f8a4d3ca77f95b0dd40704a9fc787c826660eb7ba0b641b8b175d AS amneziawg-tools

ADD --checksum=sha256:29fb0fe215fc021337498a64acee835bb87afc8ba6449587050f603f888962c3 https://codeload.github.com/hoaxisr/amneziawg-go/tar.gz/ad2b1d906df4e4e0c405c5490addc7c984bc878d /tmp/runtime.tar.gz
ADD --checksum=sha256:22438f231d39ea27e4bdc69707ec3103357f3cecc9ad3eecba18a61e5b39c9b9 https://codeload.github.com/amnezia-vpn/amneziawg-tools/tar.gz/ee0f0a9aa34ff0a0da4b3433b9512781cfe02843 /tmp/tools.tar.gz
COPY docker/original-preview/amneziawg-runtime.patch /tmp/runtime.patch

WORKDIR /src/runtime
RUN mkdir -p /out /src/tools \
 && tar -xzf /tmp/runtime.tar.gz --strip-components=1 \
 && tar -xzf /tmp/tools.tar.gz -C /src/tools --strip-components=1 \
 && git apply /tmp/runtime.patch \
 && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local \
    go build -mod=readonly -trimpath -buildvcs=false -ldflags="-s -w" -o /out/amneziawg-go . \
 && make -C /src/tools/src -j2 LDFLAGS=-static wg \
 && cp /src/tools/src/wg /out/awg \
 && cp LICENSE /out/LICENSE-amneziawg-go \
 && cp /src/tools/COPYING /out/LICENSE-amneziawg-tools

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm@sha256:a688600ca24f8a4d3ca77f95b0dd40704a9fc787c826660eb7ba0b641b8b175d AS preview-builder
WORKDIR /src
COPY . .
COPY --from=frontend-assets /src/frontend/build ./frontend/build
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local
RUN go build -mod=vendor -trimpath -ldflags="-s -w" -tags embed_frontend -o /out/awg-manager ./cmd/awg-manager \
 && go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/awg-panel-socket ./docker/original-preview/panel-socket.go

FROM runtime-base AS original-preview
RUN chown root:root /data && chmod 0700 /data
COPY --from=singbox-verify /usr/local/bin/sing-box /usr/local/bin/sing-box
COPY --from=singbox-builder /src/amnezia-box/LICENSE /usr/local/share/licenses/amnezia-box/LICENSE
COPY --from=amneziawg-tools /out/amneziawg-go /out/awg /usr/local/bin/
COPY --from=amneziawg-tools /out/LICENSE-amneziawg-go /out/LICENSE-amneziawg-tools /usr/local/share/licenses/amneziawg/
COPY LICENSE /usr/local/share/licenses/awg-manager/LICENSE
COPY --from=preview-builder /out/awg-manager /out/awg-panel-socket /usr/local/bin/
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

FROM runtime-base AS runtime
COPY --from=builder /out/awg-manager /usr/local/bin/awg-manager
COPY --from=singbox-verify /usr/local/bin/sing-box /usr/local/bin/sing-box
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 0755 /usr/local/bin/awg-manager /usr/local/bin/sing-box /usr/local/bin/entrypoint.sh
ENV AWG_MODE=singbox \
    AWG_DATA_DIR=/data \
    AWG_HTTP_ADDR=0.0.0.0:2222 \
    SINGBOX_BIN=/usr/local/bin/sing-box \
    SINGBOX_CONFIG_DIR=/data/sing-box/config.d \
    AWG_DNS_UPSTREAM=1.1.1.1 \
    AWG_PROXY_ADDR=127.0.0.1:1080
VOLUME ["/data"]
EXPOSE 2222 1080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["/usr/local/bin/awg-manager"]
