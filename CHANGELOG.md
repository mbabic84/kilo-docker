## [1.14.3](https://github.com/mbabic84/kilo-docker/compare/v1.14.2...v1.14.3) (2026-03-26)

### Bug Fixes

* persist config and state across container recreation ([b887bb1](https://github.com/mbabic84/kilo-docker/commit/b887bb1d1fdda00fcb7bffc0b686092ea4071d5b))

## [1.14.2](https://github.com/mbabic84/kilo-docker/compare/v1.14.1...v1.14.2) (2026-03-26)

### Bug Fixes

* skip password confirmation for existing volumes ([dba49df](https://github.com/mbabic84/kilo-docker/commit/dba49dfa3a696fad1dd2c11032d979388887593a))

## [1.14.1](https://github.com/mbabic84/kilo-docker/compare/v1.14.0...v1.14.1) (2026-03-26)

### Bug Fixes

* support install command when running via pipe ([73826c4](https://github.com/mbabic84/kilo-docker/commit/73826c4fac482b91ca732da7cf888038f9dd077f))

## [1.14.0](https://github.com/mbabic84/kilo-docker/compare/v1.13.1...v1.14.0) (2026-03-25)

### Features

* add install and update commands for global script management ([676ca4b](https://github.com/mbabic84/kilo-docker/commit/676ca4bda13ecc46b9a72ca2df9dcc9b1926cffc))

## [1.13.1](https://github.com/mbabic84/kilo-docker/compare/v1.13.0...v1.13.1) (2026-03-25)

### Bug Fixes

* enable MCP servers when tokens are present ([1285e49](https://github.com/mbabic84/kilo-docker/commit/1285e49ec30ea5461c5f2bc396ee06238cb84c54))

## [1.13.0](https://github.com/mbabic84/kilo-docker/compare/v1.12.1...v1.13.0) (2026-03-25)

### Features

* add --password flag for volume encryption and non-discoverable volume names ([58c32d5](https://github.com/mbabic84/kilo-docker/commit/58c32d5b607e0e4ffc8f3796d4473d2b399a4785))

## [1.12.1](https://github.com/mbabic84/kilo-docker/compare/v1.12.0...v1.12.1) (2026-03-25)

### Bug Fixes

* pass --help/-h to Kilo CLI and fix TTY for args with TUI ([284c076](https://github.com/mbabic84/kilo-docker/commit/284c076432317c6933dbf43c81e433252b91381a))

## [1.12.0](https://github.com/mbabic84/kilo-docker/compare/v1.11.2...v1.12.0) (2026-03-25)

### Features

* add --playwright flag for Playwright MCP sidecar ([72a9070](https://github.com/mbabic84/kilo-docker/commit/72a9070b628d0b076b6e9a67f5fff1fcae244459))
* bump Kilo CLI to v7.1.4 ([23e6283](https://github.com/mbabic84/kilo-docker/commit/23e6283b8e0b5efeb2c1772ddc85a469d0d3ecae))

## [1.11.2](https://github.com/mbabic84/kilo-docker/compare/v1.11.1...v1.11.2) (2026-03-24)

### Bug Fixes

* correct playwright MCP config and remove unused docker-compose.yml ([609b88a](https://github.com/mbabic84/kilo-docker/commit/609b88a92f2b5db5e396649cd31174d61c804758))

## [1.11.1](https://github.com/mbabic84/kilo-docker/compare/v1.11.0...v1.11.1) (2026-03-24)

### Bug Fixes

* use build-push-action for full image to fix shell syntax error ([3f156f6](https://github.com/mbabic84/kilo-docker/commit/3f156f6128a3f10491c0f70793921479cfa23e68))

## [1.11.0](https://github.com/mbabic84/kilo-docker/compare/v1.10.0...v1.11.0) (2026-03-24)

### Features

* add full image variant with Chromium and Playwright ([bdf3956](https://github.com/mbabic84/kilo-docker/commit/bdf39566efce1f0adbd0240ca18217eb18d14417))

### Bug Fixes

* use GHCR image tag in Dockerfile.full to avoid re-tagging ([906c951](https://github.com/mbabic84/kilo-docker/commit/906c95109185ef28fddaf928be7ae1833961bd56))

## [1.10.0](https://github.com/mbabic84/kilo-docker/compare/v1.9.0...v1.10.0) (2026-03-24)

### Features

* bump Kilo CLI to v7.1.3 ([f37ecd8](https://github.com/mbabic84/kilo-docker/commit/f37ecd8193e85e4e51f2a6c41234f1032a12848c))

## [1.9.0](https://github.com/mbabic84/kilo-docker/compare/v1.8.0...v1.9.0) (2026-03-24)

### Features

* dynamically disable MCP servers when tokens are missing ([92d5fac](https://github.com/mbabic84/kilo-docker/commit/92d5fac1a5dc7956df1cbc92e0a09f4c081799b4))

## [1.8.0](https://github.com/mbabic84/kilo-docker/compare/v1.7.0...v1.8.0) (2026-03-24)

### Features

* allow kilo user to install packages via sudo ([283eadc](https://github.com/mbabic84/kilo-docker/commit/283eadcc81a92f04046efa5eb918e397e50e0dc1))

## [1.7.0](https://github.com/mbabic84/kilo-docker/compare/v1.6.0...v1.7.0) (2026-03-24)

### Features

* add automated Kilo CLI version bump workflow ([e112792](https://github.com/mbabic84/kilo-docker/commit/e1127929ab2a684224df61beb70514614b418b15))
* bump Kilo CLI to v7.1.2 ([e22b980](https://github.com/mbabic84/kilo-docker/commit/e22b980d3d20c8fc7eb6d8532330b20e3b6b9e74))

### Bug Fixes

* use PAT token and update create-pull-request to v8 in version check workflow ([d2c7a04](https://github.com/mbabic84/kilo-docker/commit/d2c7a049bba08a828150d4c02d2a256daf0e9c2b))

## [1.6.0](https://github.com/mbabic84/kilo-docker/compare/v1.5.0...v1.6.0) (2026-03-24)

### Features

* add --network flag with interactive picker and networks subcommand ([561f72f](https://github.com/mbabic84/kilo-docker/commit/561f72f6bfd9484a1ebffde769b7b6a7559bda4a))

### Bug Fixes

* split readonly declaration to avoid masking return values ([fe20a28](https://github.com/mbabic84/kilo-docker/commit/fe20a28a9fd82a69abeb2d3304220d4660ff84c7))

## [1.5.0](https://github.com/mbabic84/kilo-docker/compare/v1.4.10...v1.5.0) (2026-03-22)

### Features

* replace Node.js image with minimal Alpine binary distribution ([0137bf5](https://github.com/mbabic84/kilo-docker/commit/0137bf58f7602952f31bd7f4a6b85c26cc0ec255))

## [1.4.10](https://github.com/mbabic84/kilo-docker/compare/v1.4.9...v1.4.10) (2026-03-22)

### Bug Fixes

* move kilo binary to /usr/local/bin to resolve permission denied ([2d2b603](https://github.com/mbabic84/kilo-docker/commit/2d2b60315ca2a9fee0cc5d58d59a382092240b4f))

## [1.4.9](https://github.com/mbabic84/kilo-docker/compare/v1.4.8...v1.4.9) (2026-03-22)

### Bug Fixes

* use official installer instead of direct binary download ([97f42c2](https://github.com/mbabic84/kilo-docker/commit/97f42c26d6e988419596e7c7f305e02e7d9d8e94))

## [1.4.8](https://github.com/mbabic84/kilo-docker/compare/v1.4.7...v1.4.8) (2026-03-22)

### Bug Fixes

* add gcompat for musl terminal handling ([c880661](https://github.com/mbabic84/kilo-docker/commit/c8806611b89fed30582155acea3da6305396a38a))

## [1.4.7](https://github.com/mbabic84/kilo-docker/compare/v1.4.6...v1.4.7) (2026-03-22)

### Bug Fixes

* set LANG env and revert forced TERM/COLORTERM defaults ([1fb0c0d](https://github.com/mbabic84/kilo-docker/commit/1fb0c0d49e3c050f09a02d9b1480079cb6ff104e)), closes [#19](https://github.com/mbabic84/kilo-docker/issues/19)

## [1.4.6](https://github.com/mbabic84/kilo-docker/compare/v1.4.5...v1.4.6) (2026-03-22)

### Bug Fixes

* pass TERM/COLORTERM to container for proper TUI formatting ([5e9bed1](https://github.com/mbabic84/kilo-docker/commit/5e9bed132dbba722ed6c891ca6d538f620099b5a))

## [1.4.5](https://github.com/mbabic84/kilo-docker/compare/v1.4.4...v1.4.5) (2026-03-22)

### Bug Fixes

* add missing CLI tool dependencies for Kilo agent ([ab2ad5b](https://github.com/mbabic84/kilo-docker/commit/ab2ad5be7a71ca9fd53b8fb5cc554c505c19585c))

## [1.4.4](https://github.com/mbabic84/kilo-docker/compare/v1.4.3...v1.4.4) (2026-03-22)

### Bug Fixes

* add Node.js setup and remove duplicate latest tag in release workflow ([c7295cc](https://github.com/mbabic84/kilo-docker/commit/c7295cc2f6d0fe6ae413503b321db0fe7ef9aaac))

## [1.4.3](https://github.com/mbabic84/kilo-docker/compare/v1.4.2...v1.4.3) (2026-03-22)

### Bug Fixes

* install project dependencies before semantic-release action ([8c0ccc0](https://github.com/mbabic84/kilo-docker/commit/8c0ccc0f60229e7196662004761daa8790f3f583))
* use semantic-release action with proper step outputs for Docker build gating ([d5c16c7](https://github.com/mbabic84/kilo-docker/commit/d5c16c7bce2d24d1a8d74ecac31856286d33a574))

## [1.4.2](https://github.com/mbabic84/kilo-docker/compare/v1.4.1...v1.4.2) (2026-03-21)

### Bug Fixes

* build Docker image only when semantic-release publishes a release ([6c61a66](https://github.com/mbabic84/kilo-docker/commit/6c61a663dc5315b9141358e8c3ce3ebc71814a7e))

## [1.4.1](https://github.com/mbabic84/kilo-docker/compare/v1.4.0...v1.4.1) (2026-03-21)

### Bug Fixes

* address Copilot review feedback ([8b5e8f8](https://github.com/mbabic84/kilo-docker/commit/8b5e8f87c003fadbf751c49110b1f1fb1db814cf))

## [1.4.0](https://github.com/mbabic84/kilo-docker/compare/v1.3.0...v1.4.0) (2026-03-21)

### Features

* add --once flag and cleanup command ([9a751cd](https://github.com/mbabic84/kilo-docker/commit/9a751cd1cb1681b105155a6999b51ce5080e1352))

## [1.3.0](https://github.com/mbabic84/kilo-docker/compare/v1.2.0...v1.3.0) (2026-03-21)

### Features

* add kilo-docker script with token persistence ([276475d](https://github.com/mbabic84/kilo-docker/commit/276475d39f94875bafa72bff0049bf63b1beba23))

## [1.2.0](https://github.com/mbabic84/kilo-docker/compare/v1.1.0...v1.2.0) (2026-03-21)

### Features

* persist database across container restarts via volume ([efc19fb](https://github.com/mbabic84/kilo-docker/commit/efc19fb758206b1a4adb44353742c593b81c108f))

## [1.1.0](https://github.com/mbabic84/kilo-docker/compare/v1.0.2...v1.1.0) (2026-03-21)

### Features

* add default MCP servers (context7, ainstruct) to Docker image ([eacc61a](https://github.com/mbabic84/kilo-docker/commit/eacc61a2b58cd2f8cfc3ed4243d69e1b5e70a9e0))

## [1.0.2](https://github.com/mbabic84/kilo-docker/compare/v1.0.1...v1.0.2) (2026-03-21)

### Bug Fixes

* resolve docker workflow annotations ([7586f08](https://github.com/mbabic84/kilo-docker/commit/7586f08a7ef223d30c7e56fe80fca7df160f76f6))

## [1.0.1](https://github.com/mbabic84/kilo-docker/compare/v1.0.0...v1.0.1) (2026-03-21)

### Bug Fixes

* support arbitrary UID and remove default --help command [#patch](https://github.com/mbabic84/kilo-docker/issues/patch) ([67b4030](https://github.com/mbabic84/kilo-docker/commit/67b4030f58749506485cb81f49d2baa0e2a44df6))

## 1.0.0 (2026-03-21)

### Features

* add kilo-docker with semantic-release and GitHub Actions workflows ([92402b8](https://github.com/mbabic84/kilo-docker/commit/92402b8e5401147fdfa5998967f489fd853b502c))

### Bug Fixes

* add package-lock.json and missing changelog dependency for release workflow ([faa82c1](https://github.com/mbabic84/kilo-docker/commit/faa82c16024bd269fd147bc842824e878acbd7e2))
* replace semantic-release action with npx and add missing conventionalcommits dep [#patch](https://github.com/mbabic84/kilo-docker/issues/patch) ([f4bbe96](https://github.com/mbabic84/kilo-docker/commit/f4bbe96f97ea517fb837060190120f65f2b2b4a6))
* use existing node user instead of creating kilo user ([e54d01b](https://github.com/mbabic84/kilo-docker/commit/e54d01bba81670a7933f6d353661b38be5c3cb54))
