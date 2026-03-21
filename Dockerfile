FROM node:lts-alpine

RUN apk add --no-cache \
    git \
    curl \
    bash \
    ca-certificates \
    openssh-client

RUN addgroup -g 1000 kilo && \
    adduser -D -u 1000 -G kilo -s /bin/bash kilo

RUN npm install -g @kilocode/cli

RUN mkdir /workspace && chown kilo:kilo /workspace

WORKDIR /workspace

USER kilo

ENTRYPOINT ["kilo"]
CMD ["--help"]
