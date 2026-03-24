# syntax=docker/dockerfile:1
# ── Builder: download Kilo binary ──
FROM alpine:3.21 AS builder

ARG KILO_VERSION=7.1.2

RUN apk add --no-cache curl tar \
    && curl -fsSL "https://github.com/Kilo-Org/kilocode/releases/download/v${KILO_VERSION}/kilo-linux-x64-musl.tar.gz" \
       | tar xz -C /tmp \
    && chmod +x /tmp/kilo

# ── Runtime: Alpine with tools ──
FROM alpine:3.21

RUN apk add --no-cache libstdc++ git openssh-client ripgrep su-exec sudo \
    && adduser -D -u 1000 kilo \
    && echo "kilo ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

COPY --from=builder /tmp/kilo /usr/local/bin/kilo
COPY opencode.json /home/kilo/.config/kilo/opencode.json
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

ENV HOME=/home/kilo
WORKDIR /workspace

ENTRYPOINT ["docker-entrypoint.sh"]
