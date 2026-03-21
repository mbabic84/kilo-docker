FROM alpine:3.21 AS builder

RUN apk add --no-cache curl \
    && curl -fsSL https://github.com/Kilo-Org/kilocode/releases/download/v7.1.0/kilo-linux-x64-musl.tar.gz \
       | tar xzf - kilo

FROM alpine:3.21

RUN apk add --no-cache git ca-certificates openssh-client libstdc++ \
    && mkdir -p /workspace /home/user/.local /home/user/.config/kilo \
    && chmod 777 /workspace /home/user /home/user/.local

COPY --from=builder /kilo /usr/local/bin/kilo
COPY opencode.json /home/user/.config/kilo/opencode.json

WORKDIR /workspace

ENV HOME=/home/user

VOLUME /home/user/.local/share/kilo

ENTRYPOINT ["kilo"]
