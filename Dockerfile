# syntax=docker/dockerfile:1
# ── Builder: download Kilo binary ──
FROM alpine:latest AS builder

ARG KILO_VERSION=7.1.8

RUN apk add --no-cache curl tar \
    && curl -fsSL "https://github.com/Kilo-Org/kilocode/releases/download/v${KILO_VERSION}/kilo-linux-x64-musl.tar.gz" \
       | tar xz -C /tmp \
    && chmod +x /tmp/kilo

# ── Runtime: Alpine with tools ──
FROM alpine:latest

RUN apk add --no-cache libstdc++ git openssh-client ripgrep su-exec sudo jq curl util-linux \
    && adduser -D -u 1000 kilo-t8x3m7kp \
    && echo "kilo-t8x3m7kp ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

COPY --from=builder /tmp/kilo /usr/local/bin/kilo
COPY configs/opencode.json /home/kilo-t8x3m7kp/.config/kilo/opencode.json
COPY configs/zellij.kdl /etc/zellij/config.kdl
COPY scripts/entrypoint.sh /usr/local/bin/docker-entrypoint.sh
COPY scripts/setup-kilo-config.sh /usr/local/bin/setup-kilo-config.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh /usr/local/bin/setup-kilo-config.sh

ENV HOME=/home/kilo-t8x3m7kp

ENTRYPOINT ["docker-entrypoint.sh"]
