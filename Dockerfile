# syntax=docker/dockerfile:1
# ── Builder: compile kilo-entrypoint ──
FROM golang:1.26-alpine AS go-builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY pkg/ pkg/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/kilo-entrypoint ./cmd/kilo-entrypoint

# ── Builder: download Kilo binary ──
FROM alpine:latest AS builder

ARG KILO_VERSION=7.2.52

RUN apk add --no-cache curl tar \
    && curl -fsSL "https://github.com/Kilo-Org/kilocode/releases/download/v${KILO_VERSION}/kilo-linux-x64-musl.tar.gz" \
       | tar xz -C /tmp \
    && chmod +x /tmp/kilo

# ── Runtime: Alpine with tools ──
FROM alpine:latest

ARG AINSTRUCT_BASE_URL=https://ainstruct-dev.kralicinora.cz

RUN apk add --no-cache bash coreutils grep sed gawk libstdc++ git openssh-client ripgrep curl tar xz sudo jq tzdata \
    && curl -fsSL https://github.com/zellij-org/zellij/releases/latest/download/zellij-x86_64-unknown-linux-musl.tar.gz -o /tmp/zellij.tar.gz \
    && tar xzf /tmp/zellij.tar.gz -C /usr/local/bin && rm -rf /tmp/zellij.tar.gz && chmod +x /usr/local/bin/zellij \
    && echo "ALL ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers.d/nopasswd \
    && chmod 0440 /etc/sudoers.d/nopasswd

# Templates for initial user configs - use 'template-' prefix to avoid
# being read as system configs (which would override user settings).
COPY configs/zellij.kdl /etc/zellij/template-config.kdl
COPY configs/kilo.jsonc /etc/kilo/template-kilo.jsonc
COPY --from=builder /tmp/kilo /usr/local/bin/kilo-real
COPY --from=go-builder /out/kilo-entrypoint /usr/local/bin/kilo-entrypoint
COPY scripts/kilo-wrapper.sh /usr/local/bin/kilo

RUN chmod +x /usr/local/bin/kilo /usr/local/bin/kilo-real

ENV SHELL=/bin/bash
ENV KD_AINSTRUCT_BASE_URL=${AINSTRUCT_BASE_URL}

ENTRYPOINT ["kilo-entrypoint"]
