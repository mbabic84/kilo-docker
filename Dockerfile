FROM node:lts-alpine

RUN apk add --no-cache \
    git \
    curl \
    bash \
    ca-certificates \
    openssh-client

RUN apk add --no-cache \
    git \
    curl \
    bash \
    ca-certificates \
    openssh-client

RUN npm install -g @kilocode/cli

RUN mkdir /workspace && chown node:node /workspace

WORKDIR /workspace

USER node

ENTRYPOINT ["kilo"]
CMD ["--help"]
