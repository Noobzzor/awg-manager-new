FROM awgm-vps-candidate:20261003-194400
LABEL org.opencontainers.image.title="AWG Manager personal trial" awgm.trial.owner="66ef0989-4140-4902-beac-2b9f0a3ca7c0"
COPY --chmod=0755 awg-manager amneziawg-go awg /usr/local/bin/
COPY --chmod=0755 entrypoint.sh /usr/local/bin/entrypoint.sh
COPY --chmod=0755 trial-entrypoint.sh /usr/local/bin/trial-entrypoint.sh
COPY manifest.json SHA256SUMS /usr/local/share/awgm-trial/
COPY LICENSE-manager LICENSE-awg NOTICE /usr/local/share/awgm-trial/
RUN chown -R 0:0 /data && chmod 0750 /data /data/sing-box /data/sing-box/config.d
ENV AWG_MODE=singbox AWG_DATA_DIR=/data AWG_HTTP_ADDR=0.0.0.0:2222 \
    SINGBOX_BIN=/usr/local/bin/sing-box SINGBOX_CONFIG_DIR=/data/sing-box/config.d \
    AWG_PROXY_ADDR=127.0.0.1:1080 AWG_DNS_UPSTREAM=1.1.1.1 \
    AWG_GATEWAY_ENABLE=true AWG_GATEWAY_BACKEND=userspace-awg \
    AWG_GATEWAY_RUNTIME_BINARY=/usr/local/bin/amneziawg-go \
    AWG_GATEWAY_IP_BINARY=/usr/sbin/ip AWG_GATEWAY_TOOL=/usr/local/bin/awg \
    AWG_GATEWAY_INTERFACE=awg0 AWG_GATEWAY_WAN_INTERFACE=eth0 AWG_GATEWAY_PORT=51820 \
    AWG_GATEWAY_CLIENT_POOL=10.77.0.0/24 AWG_GATEWAY_SERVER_ADDRESS=10.77.0.1/24
ENV AWG_GATEWAY_PARAMETERS='{"jc":3,"jmin":40,"jmax":80,"s1":16,"s2":16,"s3":16,"s4":16,"h1":1001,"h2":1002,"h3":1003,"h4":1004}'
EXPOSE 2222/tcp 51820/udp
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["curl","--fail","--silent","--show-error","http://127.0.0.1:2222/readyz"]
ENTRYPOINT ["/usr/local/bin/trial-entrypoint.sh"]
CMD ["/usr/local/bin/awg-manager"]
