FROM alpine:3.21

RUN apk add --no-cache \
    git \
    ca-certificates \
    openssh-client \
    libstdc++ \
    ripgrep \
    curl \
    coreutils \
    bash \
    && mkdir -p /workspace /home/user/.local/share/kilo /home/user/.config/kilo \
    && chmod 777 /workspace /home/user /home/user/.local /home/user/.local/share/kilo \
    && curl -fsSL https://kilo.ai/cli/install | bash -s -- --no-modify-path \
    && mv /root/.kilo/bin/kilo /usr/local/bin/kilo

COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY opencode.json /home/user/.config/kilo/opencode.json

WORKDIR /workspace

ENV HOME=/home/user

VOLUME /home/user/.local/share/kilo

ENTRYPOINT ["entrypoint.sh"]
