FROM node:lts-alpine

RUN apk add --no-cache \
    git \
    curl \
    bash \
    ca-certificates \
    openssh-client

RUN npm install -g @kilocode/cli

RUN mkdir -p /workspace /home/user/.local && \
    chmod 777 /workspace /home/user /home/user/.local

WORKDIR /workspace

ENV HOME=/home/user

ENTRYPOINT ["kilo"]
