# syntax=docker/dockerfile:1
# ── Builder: compile kilo-entrypoint ──
FROM golang:1.26-alpine AS go-builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/kilo-entrypoint ./cmd/kilo-entrypoint

# ── Builder: download Kilo binary ──
FROM alpine:latest AS builder

ARG KILO_VERSION=7.1.11

RUN apk add --no-cache curl tar \
    && curl -fsSL "https://github.com/Kilo-Org/kilocode/releases/download/v${KILO_VERSION}/kilo-linux-x64-musl.tar.gz" \
       | tar xz -C /tmp \
    && chmod +x /tmp/kilo

# ── Runtime: Alpine with tools ──
FROM alpine:latest

RUN apk add --no-cache libstdc++ git openssh-client ripgrep sudo curl tar \
    && adduser -D -u 1000 kilo-t8x3m7kp \
    && echo "kilo-t8x3m7kp ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

COPY configs/zellij.kdl /etc/zellij/config.kdl
COPY configs/opencode.json /home/kilo-t8x3m7kp/.config/kilo/opencode.json
COPY --from=builder /tmp/kilo /usr/local/bin/kilo
COPY --from=go-builder /out/kilo-entrypoint /usr/local/bin/kilo-entrypoint

ENV HOME=/home/kilo-t8x3m7kp

ENTRYPOINT ["kilo-entrypoint"]
