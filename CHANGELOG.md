## [3.19.0](https://github.com/mbabic84/kilo-docker/compare/v3.18.0...v3.19.0) (2026-05-14)

### Features

* add port conflict detection for sessions ([1deb716](https://github.com/mbabic84/kilo-docker/commit/1deb7168eec6ff462f0e210a940715acef8f3852))
* add sessions stop subcommand ([ace33e6](https://github.com/mbabic84/kilo-docker/commit/ace33e6ac7ea6c6e60415ef606a383ded8b1add6))

### Bug Fixes

* handle exited session re-attach via docker start ([7863f20](https://github.com/mbabic84/kilo-docker/commit/7863f208c213995cb3fd919905dd86be60dd8caa))

## [3.18.0](https://github.com/mbabic84/kilo-docker/compare/v3.17.0...v3.18.0) (2026-05-13)

### Features

* replace service groups with host supplementary groups (PGIDS); add gitnexus service ([6df1d41](https://github.com/mbabic84/kilo-docker/commit/6df1d41dbe08129b8c70d03c5ce3e112f7518905))

## [3.17.0](https://github.com/mbabic84/kilo-docker/compare/v3.16.0...v3.17.0) (2026-05-12)

### Features

* migrate Docker image from Alpine to Debian Bookworm ([00658dc](https://github.com/mbabic84/kilo-docker/commit/00658dcd47cdcc037640517828ce3984936b1d60))

## [3.16.0](https://github.com/mbabic84/kilo-docker/compare/v3.15.0...v3.16.0) (2026-05-12)

### Features

* use workspace directory name as Zellij session name ([a34b44a](https://github.com/mbabic84/kilo-docker/commit/a34b44a4b27a024132afcc530b5ee57e59e6fb9f))

## [3.15.0](https://github.com/mbabic84/kilo-docker/compare/v3.14.0...v3.15.0) (2026-05-12)

### Features

* bump Kilo CLI to v7.2.52 ([724ff79](https://github.com/mbabic84/kilo-docker/commit/724ff790e6f1a2a56aac9f9ea8f725777b921025))

## [3.14.0](https://github.com/mbabic84/kilo-docker/compare/v3.13.0...v3.14.0) (2026-05-11)

### Features

* bump Kilo CLI to v7.2.49 ([8025023](https://github.com/mbabic84/kilo-docker/commit/80250233381eb08e13859772dc7bc09d211725c2))

## [3.13.0](https://github.com/mbabic84/kilo-docker/compare/v3.12.0...v3.13.0) (2026-05-06)

### Features

* bump Kilo CLI to v7.2.40 ([440a1bc](https://github.com/mbabic84/kilo-docker/commit/440a1bc7c5faef1f1104cfd1cbf34c63ac9f13e3))

## [3.12.0](https://github.com/mbabic84/kilo-docker/compare/v3.11.0...v3.12.0) (2026-05-05)

### Features

* replace s5cmd with rclone as optional S3 service ([f93620c](https://github.com/mbabic84/kilo-docker/commit/f93620cc32affe1e069421e19f15be9dede5fd98))

## [3.11.0](https://github.com/mbabic84/kilo-docker/compare/v3.10.0...v3.11.0) (2026-05-05)

### Features

* add s5cmd as optional --s5cmd service ([01e2e7b](https://github.com/mbabic84/kilo-docker/commit/01e2e7b7bfc5b53448fc35c117b43dd61dc2e47f))

## [3.10.0](https://github.com/mbabic84/kilo-docker/compare/v3.9.1...v3.10.0) (2026-05-04)

### Features

* bump Kilo CLI to v7.2.34 ([4a66567](https://github.com/mbabic84/kilo-docker/commit/4a6656710c8088faf39715bcc70e68806a81f6c8))

## [3.9.1](https://github.com/mbabic84/kilo-docker/compare/v3.9.0...v3.9.1) (2026-04-30)

### Bug Fixes

* **entrypoint:** move init waiting into entrypoint with configurable timeout ([93b5120](https://github.com/mbabic84/kilo-docker/commit/93b5120bab903ff9bd1891337edd5625733c151d))

## [3.9.0](https://github.com/mbabic84/kilo-docker/compare/v3.8.1...v3.9.0) (2026-04-29)

### Features

* bump Kilo CLI to v7.2.31 ([1f1872c](https://github.com/mbabic84/kilo-docker/commit/1f1872c14e7c973098f61366dfd3dd852386d606))

## [3.8.1](https://github.com/mbabic84/kilo-docker/compare/v3.8.0...v3.8.1) (2026-04-28)

### Bug Fixes

* **entrypoint:** restore groups and fail on inaccessible workspace ([eaa9fce](https://github.com/mbabic84/kilo-docker/commit/eaa9fce7118bfbc5bdcb844103d5b4a8b31a06fa))

## [3.8.0](https://github.com/mbabic84/kilo-docker/compare/v3.7.1...v3.8.0) (2026-04-28)

### Features

* bump Kilo CLI to v7.2.25 ([fa954e8](https://github.com/mbabic84/kilo-docker/commit/fa954e870e11abf92f696ffa83a75e7871ecee8f))

## [3.7.1](https://github.com/mbabic84/kilo-docker/compare/v3.7.0...v3.7.1) (2026-04-26)

### Bug Fixes

* resolve golangci-lint findings ([d147021](https://github.com/mbabic84/kilo-docker/commit/d1470217d193c665ba598bc6df73155f1e8b330b))

## [3.7.0](https://github.com/mbabic84/kilo-docker/compare/v3.6.0...v3.7.0) (2026-04-26)

### Features

* add --build option for on-demand build-base installation ([405eda4](https://github.com/mbabic84/kilo-docker/commit/405eda4a7b758b0c0ba9f6456f9d0028cc8753c8))

## [3.6.0](https://github.com/mbabic84/kilo-docker/compare/v3.5.0...v3.6.0) (2026-04-25)

### Features

* bump Kilo CLI to v7.2.24 ([44e0e5d](https://github.com/mbabic84/kilo-docker/commit/44e0e5dca750eaa107681eaba50d8ecf76a8f63d))

## [3.5.0](https://github.com/mbabic84/kilo-docker/compare/v3.4.0...v3.5.0) (2026-04-24)

### Features

* bump Kilo CLI to v7.2.22 ([0a04d93](https://github.com/mbabic84/kilo-docker/commit/0a04d930d311068aa6d4860cc18f205b20f5d8f8))

## [3.4.0](https://github.com/mbabic84/kilo-docker/compare/v3.3.2...v3.4.0) (2026-04-22)

### Features

* bump Kilo CLI to v7.2.20 ([5ebacf3](https://github.com/mbabic84/kilo-docker/commit/5ebacf36cac5ee24a9c582b52ad22949bb62d595))

## [3.3.2](https://github.com/mbabic84/kilo-docker/compare/v3.3.1...v3.3.2) (2026-04-20)

### Bug Fixes

* require confirmation before bulk session cleanup ([97877bd](https://github.com/mbabic84/kilo-docker/commit/97877bdefc9c76da3602e47adc62a8c95144d2a7))

## [3.3.1](https://github.com/mbabic84/kilo-docker/compare/v3.3.0...v3.3.1) (2026-04-19)

### Bug Fixes

* make MCP tools default to ask not allow\n\nRemove broad allow rules for MCP namespaces (ainstruct_*, context7_*, playwright_*). Global "*": "ask" now covers all tools including MCP, ensuring confirmation prompts for MCP operations by default. Safer default behavior.\n\nNo other permission changes. ([f06f144](https://github.com/mbabic84/kilo-docker/commit/f06f1446fbd17061443b409cc7ac9dcaa6251f9c))

## [3.3.0](https://github.com/mbabic84/kilo-docker/compare/v3.2.0...v3.3.0) (2026-04-19)

### Features

* migrate config from opencode.json to kilo.jsonc ([77449cd](https://github.com/mbabic84/kilo-docker/commit/77449cd59e686f7d2b962a583c3552591b83a785))

### Bug Fixes

* update tests for kilo.jsonc sync paths ([11f6fdd](https://github.com/mbabic84/kilo-docker/commit/11f6fdd80ccba94da5c1b8fa2fdd9dbf70f70c11))

## [3.2.0](https://github.com/mbabic84/kilo-docker/compare/v3.1.2...v3.2.0) (2026-04-18)

### Features

* bump Kilo CLI to v7.2.14 ([35884e1](https://github.com/mbabic84/kilo-docker/commit/35884e1a7389069b0fd5e937ab09294b01201bad))

## [3.1.2](https://github.com/mbabic84/kilo-docker/compare/v3.1.1...v3.1.2) (2026-04-17)

### Bug Fixes

* hide default kilo-shared network in flag mismatch display ([8bca4ff](https://github.com/mbabic84/kilo-docker/commit/8bca4ff1f006a0c9a40e40f6b67ca249570440b6))

## [3.1.1](https://github.com/mbabic84/kilo-docker/compare/v3.1.0...v3.1.1) (2026-04-17)

### Bug Fixes

* add tzdata package to fix timezone sync with host ([9478148](https://github.com/mbabic84/kilo-docker/commit/947814827280c581a305f8bc14250aa45b7fd9dc))

## [3.1.0](https://github.com/mbabic84/kilo-docker/compare/v3.0.1...v3.1.0) (2026-04-17)

### Features

* **logs:** add instance identifier to shared log output ([2af8c2e](https://github.com/mbabic84/kilo-docker/commit/2af8c2efcdd35b039a58ef89dedf2c81da33cc8c))

## [3.0.1](https://github.com/mbabic84/kilo-docker/compare/v3.0.0...v3.0.1) (2026-04-17)

### Bug Fixes

* **ainstruct-sync:** watch parent dirs of whitelisted files for atomic replace ([e2734d5](https://github.com/mbabic84/kilo-docker/commit/e2734d58fbbe590e1c6166f0537cdb0895a0920d))

## [3.0.0](https://github.com/mbabic84/kilo-docker/compare/v2.18.0...v3.0.0) (2026-04-16)

### ⚠ BREAKING CHANGES

* kilo containers now always attach to the implicit kilo-shared network, which can change behavior for existing sessions created with previous networking assumptions.

### Features

* add shared network and persistent Playwright runtime ([33cddca](https://github.com/mbabic84/kilo-docker/commit/33cddca4d23e85fae19f0adac7a6fc0038a0a95d))

## [2.18.0](https://github.com/mbabic84/kilo-docker/compare/v2.17.1...v2.18.0) (2026-04-16)

### Features

* bump Kilo CLI to v7.2.10 ([ab0bdd6](https://github.com/mbabic84/kilo-docker/commit/ab0bdd642b61f7b2ba57e2c30174fd3408744e3f))

## [2.17.1](https://github.com/mbabic84/kilo-docker/compare/v2.17.0...v2.17.1) (2026-04-10)

### Bug Fixes

* do not persist --yes flag when user confirms recreation dialog ([75403c3](https://github.com/mbabic84/kilo-docker/commit/75403c3de60749edc2da9e667e45d49b62a592eb))

## [2.17.0](https://github.com/mbabic84/kilo-docker/compare/v2.16.0...v2.17.0) (2026-04-09)

### Features

* add --remember flag for persistent Ainstruct login ([4b15f0e](https://github.com/mbabic84/kilo-docker/commit/4b15f0ebd7b59000b942066f9e54a68fecae408d))
* refactor CLI flag parsing with declarative flag definitions ([9c815c7](https://github.com/mbabic84/kilo-docker/commit/9c815c742246c25a21838e1f66991c35fb07ae1a))

### Bug Fixes

* prevent race condition in token refresh with mutex ([7b26e46](https://github.com/mbabic84/kilo-docker/commit/7b26e466befa16f49c33902864a237d1075f6eef))

## [2.16.0](https://github.com/mbabic84/kilo-docker/compare/v2.15.0...v2.16.0) (2026-04-08)

### Features

* bump Kilo CLI to v7.2.1 ([54c5b61](https://github.com/mbabic84/kilo-docker/commit/54c5b6126b8f26311dfb19139bee873bf1fe74a6))

## [2.15.0](https://github.com/mbabic84/kilo-docker/compare/v2.14.5...v2.15.0) (2026-04-08)

### Features

* add sync ls and sync rm subcommands ([4256b39](https://github.com/mbabic84/kilo-docker/commit/4256b39296e571a688fcb1ee8b3a0198c2ba0b9c))

## [2.14.5](https://github.com/mbabic84/kilo-docker/compare/v2.14.4...v2.14.5) (2026-04-08)

### Bug Fixes

* remove MCP token prompt from initialization steps ([399f213](https://github.com/mbabic84/kilo-docker/commit/399f213d626a776d2d4aa420d140fd36687c1a60))

## [2.14.4](https://github.com/mbabic84/kilo-docker/compare/v2.14.3...v2.14.4) (2026-04-08)

### Bug Fixes

* remove --help and --version flags, use subcommands instead ([5e2662c](https://github.com/mbabic84/kilo-docker/commit/5e2662c1741d9967f11f6ab13c480c7ad7fda173))

## [2.14.3](https://github.com/mbabic84/kilo-docker/compare/v2.14.2...v2.14.3) (2026-04-08)

### Bug Fixes

* detect .profile PATH configuration in install script ([937b303](https://github.com/mbabic84/kilo-docker/commit/937b3030b1d30c41d1b87b24629cc9b052aa18aa))

## [2.14.2](https://github.com/mbabic84/kilo-docker/compare/v2.14.1...v2.14.2) (2026-04-08)

### Bug Fixes

* add PATH warning to install script ([ab8f384](https://github.com/mbabic84/kilo-docker/commit/ab8f384f3d5368caf396f12f16b431ab8bf4a9fd))

## [2.14.1](https://github.com/mbabic84/kilo-docker/compare/v2.14.0...v2.14.1) (2026-04-08)

### Bug Fixes

* correct logging prefixes and visibility ([f8438c6](https://github.com/mbabic84/kilo-docker/commit/f8438c698421a1c0a5bcf8515fd1393708579ac7))

## [2.14.0](https://github.com/mbabic84/kilo-docker/compare/v2.13.2...v2.14.0) (2026-04-08)

### Features

* bump Kilo CLI to v7.2.0 ([bfa2dbc](https://github.com/mbabic84/kilo-docker/commit/bfa2dbc57e5cafcc8506e4a5f3b7ee9c719052f7))

## [2.13.2](https://github.com/mbabic84/kilo-docker/compare/v2.13.1...v2.13.2) (2026-04-07)

### Bug Fixes

* add [kilo-docker] prefix to all user-visible logs ([0862a9b](https://github.com/mbabic84/kilo-docker/commit/0862a9bfec62bc6a1b08df40d9d5114c942bc88a))

## [2.13.1](https://github.com/mbabic84/kilo-docker/compare/v2.13.0...v2.13.1) (2026-04-07)

### Bug Fixes

* add missing context prefix to setup.go log ([6e45100](https://github.com/mbabic84/kilo-docker/commit/6e45100d30efc2ce57151be5c3184da5ea425bde))

## [2.13.0](https://github.com/mbabic84/kilo-docker/compare/v2.12.0...v2.13.0) (2026-04-07)

### Features

* add context identifiers to log messages ([9f419e2](https://github.com/mbabic84/kilo-docker/commit/9f419e2f4cfe01dfe6caad4193cbae68dff62313))

## [2.12.0](https://github.com/mbabic84/kilo-docker/compare/v2.11.0...v2.12.0) (2026-04-07)

### Features

* add file-only logging option with WithOutput() ([11a77a1](https://github.com/mbabic84/kilo-docker/commit/11a77a19729a4358124805b97ed089af804df4b2))

## [2.11.0](https://github.com/mbabic84/kilo-docker/compare/v2.10.2...v2.11.0) (2026-04-07)

### Features

* bump Kilo CLI to v7.1.23 ([291e64a](https://github.com/mbabic84/kilo-docker/commit/291e64a5341dc69c4cda7b5126df5280d80fbdbb))

## [2.10.2](https://github.com/mbabic84/kilo-docker/compare/v2.10.1...v2.10.2) (2026-04-07)

### Bug Fixes

* ainstruct sync not watching individual files and missing new files on startup ([d658f66](https://github.com/mbabic84/kilo-docker/commit/d658f66ceda775fcaba1737c215fc0ef663e13d3))

## [2.10.1](https://github.com/mbabic84/kilo-docker/compare/v2.10.0...v2.10.1) (2026-04-07)

### Bug Fixes

* skip docker pull when already on latest version ([3978793](https://github.com/mbabic84/kilo-docker/commit/397879397e296f00f8246d80c6a8efc8a512a8cf))

## [2.10.0](https://github.com/mbabic84/kilo-docker/compare/v2.9.0...v2.10.0) (2026-04-07)

### Features

* add version status to update command ([8cd0496](https://github.com/mbabic84/kilo-docker/commit/8cd04963702916eb14a2d93dcfa40de2dabb8d5d))

## [2.9.0](https://github.com/mbabic84/kilo-docker/compare/v2.8.2...v2.9.0) (2026-04-07)

### Features

* add versions step and create .versions file on release ([61993dc](https://github.com/mbabic84/kilo-docker/commit/61993dc126a82c52082a34a5381ee870671eb73d))

## [2.8.2](https://github.com/mbabic84/kilo-docker/compare/v2.8.1...v2.8.2) (2026-04-07)

### Bug Fixes

* resolve all linting errors and remove dead code ([ba019ff](https://github.com/mbabic84/kilo-docker/commit/ba019ffc4cc92e56f41d5e56f911c41efb0b8e0e))

## [2.8.1](https://github.com/mbabic84/kilo-docker/compare/v2.8.0...v2.8.1) (2026-04-07)

### Bug Fixes

* make mcp-config read tokens from encrypted storage only ([79c6473](https://github.com/mbabic84/kilo-docker/commit/79c64739be988487036fbc6e7847bde2a1349bf9))

## [2.8.0](https://github.com/mbabic84/kilo-docker/compare/v2.7.2...v2.8.0) (2026-04-07)

### Features

* enable copy_on_select in zellij config ([752a6b2](https://github.com/mbabic84/kilo-docker/commit/752a6b2495b06b10050238931f3dc110819b9d05))

## [2.7.2](https://github.com/mbabic84/kilo-docker/compare/v2.7.1...v2.7.2) (2026-04-07)

### Bug Fixes

* check remote before copying opencode.json template ([741e90b](https://github.com/mbabic84/kilo-docker/commit/741e90bc3fa3041953778d25a5d38a18b3c2a431))

## [2.7.1](https://github.com/mbabic84/kilo-docker/compare/v2.7.0...v2.7.1) (2026-04-06)

### Bug Fixes

* update test expectations for 5 go install commands ([751424a](https://github.com/mbabic84/kilo-docker/commit/751424ae22944650f838a9c53fb494edc93d4e1b))

## [2.7.0](https://github.com/mbabic84/kilo-docker/compare/v2.6.0...v2.7.0) (2026-04-06)

### Features

* add --workspace/-w flag for custom workspace path ([c78c9bd](https://github.com/mbabic84/kilo-docker/commit/c78c9bda57edba7400ff7042477a596512f3b83d))

## [2.6.0](https://github.com/mbabic84/kilo-docker/compare/v2.5.0...v2.6.0) (2026-04-06)

### Features

* bump Kilo CLI to v7.1.22 ([da04c09](https://github.com/mbabic84/kilo-docker/commit/da04c09db2178e3fc0685c6d10e33acc5374f65b))

## [2.5.0](https://github.com/mbabic84/kilo-docker/compare/v2.4.0...v2.5.0) (2026-04-06)

### Features

* add --volume flag for manual volume mounts ([8b39dc8](https://github.com/mbabic84/kilo-docker/commit/8b39dc8e74acc65fbbdc936d593ca0f958a356f2))

## [2.4.0](https://github.com/mbabic84/kilo-docker/compare/v2.3.3...v2.4.0) (2026-04-05)

### Features

* add docker-buildx plugin installation to docker service ([c8a9df8](https://github.com/mbabic84/kilo-docker/commit/c8a9df8c61a9c7402a392fe54d9a2bbfd2c40c8b))
* bump Kilo CLI to v7.1.21 ([3a8cdde](https://github.com/mbabic84/kilo-docker/commit/3a8cdde19e444a3dd44c0f533cb44726549c5c56))

### Bug Fixes

* rename config templates to avoid system config override ([9b88325](https://github.com/mbabic84/kilo-docker/commit/9b883251a7b8fbd0ce4f19014f903a1132f3e7e1))

## [2.3.3](https://github.com/mbabic84/kilo-docker/compare/v2.3.2...v2.3.3) (2026-04-05)

### Bug Fixes

* trim command output in joinServiceGroups log ([b182689](https://github.com/mbabic84/kilo-docker/commit/b1826890746e3ad325a6b80b0943d0ed078338b3))

## [2.3.2](https://github.com/mbabic84/kilo-docker/compare/v2.3.1...v2.3.2) (2026-04-05)

### Bug Fixes

* add user to correct group for Docker socket access ([e4ed393](https://github.com/mbabic84/kilo-docker/commit/e4ed3936a9fa36bbb7aa4dffe2e1b0add0f64265))

## [2.3.1](https://github.com/mbabic84/kilo-docker/compare/v2.3.0...v2.3.1) (2026-04-05)

### Bug Fixes

* refactor MCP config sync to use env vars and run once ([19c5286](https://github.com/mbabic84/kilo-docker/commit/19c52868b64a6b6561efe24a20ed6c57dfb536ab))

## [2.3.0](https://github.com/mbabic84/kilo-docker/compare/v2.2.3...v2.3.0) (2026-04-05)

### Features

* add version checking and improved logging for user-scoped services ([81d0d6d](https://github.com/mbabic84/kilo-docker/commit/81d0d6d77927e141565bf7616adda2fb661af9a5))

### Bug Fixes

* update nvm UserInstall count in tests (2 -> 1) ([8a92ce3](https://github.com/mbabic84/kilo-docker/commit/8a92ce3c4507c84570ec2c7206f76ae77d7c689b))

## [2.2.3](https://github.com/mbabic84/kilo-docker/compare/v2.2.2...v2.2.3) (2026-04-05)

### Bug Fixes

* use correct homeDir when syncing MCP config on session attach ([163b497](https://github.com/mbabic84/kilo-docker/commit/163b49751ee5f8c8fcda07af421cc35fd5b8f56d))

## [2.2.2](https://github.com/mbabic84/kilo-docker/compare/v2.2.1...v2.2.2) (2026-04-05)

### Bug Fixes

* remove hardcoded http:// prefix from ainstruct URL ([1445ca6](https://github.com/mbabic84/kilo-docker/commit/1445ca6931063ca2af0ee40da9ca7bb0d03519c6))

## [2.2.1](https://github.com/mbabic84/kilo-docker/compare/v2.2.0...v2.2.1) (2026-04-05)

### Bug Fixes

* **zellij:** disable auto copy on select ([7b94357](https://github.com/mbabic84/kilo-docker/commit/7b94357b3607202e785c39ee0209351b9c0aef48))

## [2.2.0](https://github.com/mbabic84/kilo-docker/compare/v2.1.0...v2.2.0) (2026-04-05)

### Features

* remove --mcp flag, auto-enable MCP based on token presence ([af884c7](https://github.com/mbabic84/kilo-docker/commit/af884c76700ba10a5acbe30bf9a05d64bef02f8a))

## [2.1.0](https://github.com/mbabic84/kilo-docker/compare/v2.0.4...v2.1.0) (2026-04-04)

### Features

* add help subcommand to kilo-entrypoint ([628fbb9](https://github.com/mbabic84/kilo-docker/commit/628fbb9968a696c505d3fe0e7672561b649f6e8b))
* add help subcommand to kilo-entrypoint ([9fe7e25](https://github.com/mbabic84/kilo-docker/commit/9fe7e2573f1318112cb2732086ef5330214a1e49))

## [2.0.4](https://github.com/mbabic84/kilo-docker/compare/v2.0.3...v2.0.4) (2026-04-04)

### Bug Fixes

* sessions cleanup requires -y flag, add shared logging ([418ff02](https://github.com/mbabic84/kilo-docker/commit/418ff020930ba37e087685efdaba72217ebeed40))

## [2.0.3](https://github.com/mbabic84/kilo-docker/compare/v2.0.2...v2.0.3) (2026-04-04)

### Bug Fixes

* clean up log lines and token status output ([793bf64](https://github.com/mbabic84/kilo-docker/commit/793bf64e60ccbadf57db87f9ba593b98a8d12ee1))

## [2.0.2](https://github.com/mbabic84/kilo-docker/compare/v2.0.1...v2.0.2) (2026-04-04)

### Bug Fixes

* redact sensitive IDs from logs ([24ddda5](https://github.com/mbabic84/kilo-docker/commit/24ddda5ea01cb6fe8d7e6a0a945f95d7439f7ef0))

## [2.0.1](https://github.com/mbabic84/kilo-docker/compare/v2.0.0...v2.0.1) (2026-04-04)

### Bug Fixes

* pass host user and hostname for PAT label ([c998fa4](https://github.com/mbabic84/kilo-docker/commit/c998fa4575e56c0bc0b0a77c1d66d9c0f80a563b))

## [2.0.0](https://github.com/mbabic84/kilo-docker/compare/v1.48.0...v2.0.0) (2026-04-04)

### ⚠ BREAKING CHANGES

* changed fs structure

* Merge pull request [#104](https://github.com/mbabic84/kilo-docker/issues/104) from mbabic84/feat/containerize-authentication ([06eed7c](https://github.com/mbabic84/kilo-docker/commit/06eed7c3e0d978fdd6d3123b9d3487ff5f8ca5b7))

### Features

* add container-side authentication and token management ([6d99743](https://github.com/mbabic84/kilo-docker/commit/6d9974314e68dea21a7a120f79211b12c623e1b8))

## [1.48.0](https://github.com/mbabic84/kilo-docker/compare/v1.47.0...v1.48.0) (2026-04-03)

### Features

* add flag mismatch detection and clear zellij sessions on recreate ([41a5e97](https://github.com/mbabic84/kilo-docker/commit/41a5e972ed45466ba2adba47363d294d7c7c85af))

### Bug Fixes

* update lodash and lodash-es to 4.18.1 to resolve Dependabot vulnerabilities ([5379655](https://github.com/mbabic84/kilo-docker/commit/53796551df33719b5ce9aefd2f59005592661a93))

## [1.47.0](https://github.com/mbabic84/kilo-docker/compare/v1.46.0...v1.47.0) (2026-04-03)

### Features

* make Zellij the default session instead of bash ([62b0a09](https://github.com/mbabic84/kilo-docker/commit/62b0a09dfb2838dfa26199e9f345928fe2e94c47))
* **zellij:** start in locked mode by default ([bc5bc75](https://github.com/mbabic84/kilo-docker/commit/bc5bc75df7953e806b34c77b0f9da711a13bb7ea))

### Bug Fixes

* reset terminal and show exit messages after session ends ([e5a34c2](https://github.com/mbabic84/kilo-docker/commit/e5a34c278203b87ee4bbe1da8a85adc85f0d4c0b))
* **zellij:** hide status bar shortcuts and simplify UI ([ba449d1](https://github.com/mbabic84/kilo-docker/commit/ba449d19bbc2c377b3606326ccdadf22f7de4e6d))
* **zellij:** remove Ctrl+q binding to allow container detach ([6c02d11](https://github.com/mbabic84/kilo-docker/commit/6c02d11664452c507067f82697b38a3ca4b99808))

## [1.46.0](https://github.com/mbabic84/kilo-docker/compare/v1.45.1...v1.46.0) (2026-04-03)

### Features

* bump Kilo CLI to v7.1.20 ([5f4f64b](https://github.com/mbabic84/kilo-docker/commit/5f4f64b48d6565da294c017352871f49dc1d13b5))

## [1.45.1](https://github.com/mbabic84/kilo-docker/compare/v1.45.0...v1.45.1) (2026-04-02)

### Bug Fixes

* resolve permission issues with volume directories ([3d9b6ff](https://github.com/mbabic84/kilo-docker/commit/3d9b6ff40d122134d28b31aef86c3f107d442165))

## [1.45.0](https://github.com/mbabic84/kilo-docker/compare/v1.44.1...v1.45.0) (2026-04-02)

### Features

* **entrypoint:** add resync subcommand and refactor config path handling ([810e54c](https://github.com/mbabic84/kilo-docker/commit/810e54c6659bd1e8d6d5783c1778a7daef0f01cb))

## [1.44.1](https://github.com/mbabic84/kilo-docker/compare/v1.44.0...v1.44.1) (2026-04-02)

### Bug Fixes

* **--docker:** install latest stable docker version dynamically ([55cf479](https://github.com/mbabic84/kilo-docker/commit/55cf479d8eeafd9313b193a95ce7126d9f6df1ef))

## [1.44.0](https://github.com/mbabic84/kilo-docker/compare/v1.43.0...v1.44.0) (2026-04-02)

### Features

* bump Kilo CLI to v7.1.18 ([4503f2a](https://github.com/mbabic84/kilo-docker/commit/4503f2a4ecf555345930b14ee2d51133a0dff3b5))

## [1.43.0](https://github.com/mbabic84/kilo-docker/compare/v1.42.0...v1.43.0) (2026-04-02)

### Features

* **nvm:** use unofficial builds mirror for musl Node.js binaries ([cf9fbdb](https://github.com/mbabic84/kilo-docker/commit/cf9fbdbe07c0fbe30898c47483d9719824a57f39))

## [1.42.0](https://github.com/mbabic84/kilo-docker/compare/v1.41.1...v1.42.0) (2026-04-02)

### Features

* add NVM service and make bash default shell ([665a200](https://github.com/mbabic84/kilo-docker/commit/665a200b859d26d6a56d95b928557c81766b6317))

## [1.41.1](https://github.com/mbabic84/kilo-docker/compare/v1.41.0...v1.41.1) (2026-04-02)

### Bug Fixes

* show --ssh flag instead of ssh-agent in session args ([07950d7](https://github.com/mbabic84/kilo-docker/commit/07950d736d273b079697c13dcc866eac3b3827a3))

## [1.41.0](https://github.com/mbabic84/kilo-docker/compare/v1.40.4...v1.41.0) (2026-04-02)

### Features

* add --port/-p option for Docker port mapping ([cca98b5](https://github.com/mbabic84/kilo-docker/commit/cca98b567180a7ca98b1bc3e24b437f725b785bb))

## [1.40.4](https://github.com/mbabic84/kilo-docker/compare/v1.40.3...v1.40.4) (2026-04-01)

### Bug Fixes

* resolve Dependabot security alerts (14 of 16) ([423ca8a](https://github.com/mbabic84/kilo-docker/commit/423ca8a893ad513d93ad52a946b9d30088e6bccb))

## [1.40.3](https://github.com/mbabic84/kilo-docker/compare/v1.40.2...v1.40.3) (2026-04-01)

### Bug Fixes

* use real GitHub path in Go module declaration ([be32db6](https://github.com/mbabic84/kilo-docker/commit/be32db63a6967dcb86956e2e27d32e4ba8ec02ab))

## [1.40.2](https://github.com/mbabic84/kilo-docker/compare/v1.40.1...v1.40.2) (2026-04-01)

### Bug Fixes

* handle non-zero exit code on docker session detach ([c616e9e](https://github.com/mbabic84/kilo-docker/commit/c616e9e01d6ae5faa4194ba7273270cd94e0ee80))

## [1.40.1](https://github.com/mbabic84/kilo-docker/compare/v1.40.0...v1.40.1) (2026-04-01)

### Bug Fixes

* skip service installation on subsequent container starts ([2beae36](https://github.com/mbabic84/kilo-docker/commit/2beae36be9c07c1945130978c64943abe11bf1fe))

## [1.40.0](https://github.com/mbabic84/kilo-docker/compare/v1.39.3...v1.40.0) (2026-04-01)

### Features

* bump Kilo CLI to v7.1.17 ([2f6c696](https://github.com/mbabic84/kilo-docker/commit/2f6c6961c82655a685f19388b7391b6039f1dc94))

## [1.39.3](https://github.com/mbabic84/kilo-docker/compare/v1.39.2...v1.39.3) (2026-04-01)

### Bug Fixes

* handle stale ssh-agent socket directory before container restart ([097fced](https://github.com/mbabic84/kilo-docker/commit/097fced5519d50082bbad5e74ab85924dfe357fa))

## [1.39.2](https://github.com/mbabic84/kilo-docker/compare/v1.39.1...v1.39.2) (2026-04-01)

### Bug Fixes

* read kilo version from Dockerfile instead of using kilo-docker version ([f2e5095](https://github.com/mbabic84/kilo-docker/commit/f2e5095074ed14a59117631e823ad09e694b7fcb))

## [1.39.1](https://github.com/mbabic84/kilo-docker/compare/v1.39.0...v1.39.1) (2026-04-01)

### Bug Fixes

* remove incorrect KILO_VERSION build arg from docker build ([b93f292](https://github.com/mbabic84/kilo-docker/commit/b93f292d3fef7db2e69a16ed1e481d1bf82d29ad))

## [1.39.0](https://github.com/mbabic84/kilo-docker/compare/v1.38.0...v1.39.0) (2026-04-01)

### Features

* add version command to print kilo-docker and kilo versions ([de4857f](https://github.com/mbabic84/kilo-docker/commit/de4857f270bf27d3d1e5a72310d22059918c197d))

## [1.38.0](https://github.com/mbabic84/kilo-docker/compare/v1.37.0...v1.38.0) (2026-04-01)

### Features

* bump Kilo CLI to v7.1.14 ([cdd773b](https://github.com/mbabic84/kilo-docker/commit/cdd773b8d79da7f5684bdb1b25a3236b3813cd2f))
* extract shared constants and utils packages ([b76b99d](https://github.com/mbabic84/kilo-docker/commit/b76b99dc010fe642d4cba16848a1a5075d13c28d))
* **services:** add shared pkg/services package with uv support ([a5aa384](https://github.com/mbabic84/kilo-docker/commit/a5aa3844ba86bc9bc5c8c49f9b6cbf83be66b071))

### Bug Fixes

* copy pkg/ directory in Dockerfile for shared packages ([2105101](https://github.com/mbabic84/kilo-docker/commit/2105101006162bc87e6decaacfb3a61553e7d45c))

## [1.37.0](https://github.com/mbabic84/kilo-docker/compare/v1.36.0...v1.37.0) (2026-04-01)

### Features

* add gh service for GitHub CLI ([643eae2](https://github.com/mbabic84/kilo-docker/commit/643eae2c2b9ab88eedd56ea0afa36a21842981c1))

## [1.36.0](https://github.com/mbabic84/kilo-docker/compare/v1.35.0...v1.36.0) (2026-04-01)

### Features

* bump Kilo CLI to v7.1.12 ([3c85d3b](https://github.com/mbabic84/kilo-docker/commit/3c85d3baf5c46b8a0d2cbbc0b6bd9946aeaea33d))

## [1.35.0](https://github.com/mbabic84/kilo-docker/compare/v1.34.0...v1.35.0) (2026-04-01)

### Features

* add sync paths whitelist for file watching ([10a2e4f](https://github.com/mbabic84/kilo-docker/commit/10a2e4ff8fa5d82475b5075762cc368fcecb8ce4))

### Bug Fixes

* handle stale SSH agent socket on startup ([d8f6b8d](https://github.com/mbabic84/kilo-docker/commit/d8f6b8dfd76468ea919f13b4b8c3cf5833b2406c))
* preserve workspace and env vars after sudo user switch ([222cf1a](https://github.com/mbabic84/kilo-docker/commit/222cf1ae7a0987e3db0e80a7232c3289c1d1bdb0))

## [1.34.0](https://github.com/mbabic84/kilo-docker/compare/v1.33.0...v1.34.0) (2026-03-31)

### Features

* add Go 1.26.1 as optional service via --go flag ([196409a](https://github.com/mbabic84/kilo-docker/commit/196409ac8fba9e380f41440273bff055dac1a35f))
* add Node.js LTS as optional service via --node flag ([faf1df1](https://github.com/mbabic84/kilo-docker/commit/faf1df1ea472dd81c3ee945f589182e026970787))

## [1.33.0](https://github.com/mbabic84/kilo-docker/compare/v1.32.0...v1.33.0) (2026-03-31)

### Features

* **services:** add data-driven service architecture ([d81c5d9](https://github.com/mbabic84/kilo-docker/commit/d81c5d9cb7f76904a9832f526717a145a118e361))

## [1.32.0](https://github.com/mbabic84/kilo-docker/compare/v1.31.3...v1.32.0) (2026-03-31)

### Features

* **sessions:** add recreate command and fix attach/start error handling ([01da946](https://github.com/mbabic84/kilo-docker/commit/01da9467099ec241fa5eb9a63a39758771c3dd1f))

## [1.31.3](https://github.com/mbabic84/kilo-docker/compare/v1.31.2...v1.31.3) (2026-03-31)

### Bug Fixes

* remove kilo-docker install command, simplify install.sh flow ([2f9568e](https://github.com/mbabic84/kilo-docker/commit/2f9568e13256be02c39e80cd0145422e8be89bd5))

## [1.31.2](https://github.com/mbabic84/kilo-docker/compare/v1.31.1...v1.31.2) (2026-03-31)

### Bug Fixes

* download kilo-docker binary from GitHub releases on update ([ca40087](https://github.com/mbabic84/kilo-docker/commit/ca40087474840810fa2e1c55f0621c230c0913d5))

## [1.31.1](https://github.com/mbabic84/kilo-docker/compare/v1.31.0...v1.31.1) (2026-03-31)

### Bug Fixes

* ainstruct-sync INVALID_TOKEN retry, hash errors, watcher, config safety ([b257770](https://github.com/mbabic84/kilo-docker/commit/b257770e9af86297b0d6dd1b0c0305293b4708b2))

## [1.31.0](https://github.com/mbabic84/kilo-docker/compare/v1.30.3...v1.31.0) (2026-03-31)

### Features

* bump Kilo CLI to v7.1.11 ([f604693](https://github.com/mbabic84/kilo-docker/commit/f604693bfce867037399b227410bda80dff44708))

## [1.30.3](https://github.com/mbabic84/kilo-docker/compare/v1.30.2...v1.30.3) (2026-03-31)

### Bug Fixes

* MCP token storage and loading for encrypted volumes ([5c034df](https://github.com/mbabic84/kilo-docker/commit/5c034df746ed4376503a2bad807d291d7524f99a))

## [1.30.2](https://github.com/mbabic84/kilo-docker/compare/v1.30.1...v1.30.2) (2026-03-31)

### Bug Fixes

* SSH agent key loading, logging, and socket permissions ([f5e73ef](https://github.com/mbabic84/kilo-docker/commit/f5e73efe4876fb9a46d0fbeedcebf5949e29fae8))

## [1.30.1](https://github.com/mbabic84/kilo-docker/compare/v1.30.0...v1.30.1) (2026-03-31)

### Bug Fixes

* harden install.sh and handleInstall/handleUpdate error handling ([21c9dce](https://github.com/mbabic84/kilo-docker/commit/21c9dceb404f7848e3958f76a276e0dcecb2709a))

## [1.30.0](https://github.com/mbabic84/kilo-docker/compare/v1.29.0...v1.30.0) (2026-03-31)

### Features

* add --yes/-y flag and auto-confirm for piped installs ([f63320b](https://github.com/mbabic84/kilo-docker/commit/f63320b239952bf9b8f70e1692d1af87abe7d4aa))

## [1.29.0](https://github.com/mbabic84/kilo-docker/compare/v1.28.1...v1.29.0) (2026-03-31)

### Features

* add sessions cleanup -a flag to auto-remove exited sessions ([2c0d41c](https://github.com/mbabic84/kilo-docker/commit/2c0d41c8fff5b700e2067c3b8f6621d032d18c89))
* migrate from bash to Go ([ce3dbd0](https://github.com/mbabic84/kilo-docker/commit/ce3dbd01d05b5274887caa602c8959035c0d3b57))

### Bug Fixes

* docker argument construction and container init ([d9c7b66](https://github.com/mbabic84/kilo-docker/commit/d9c7b669bb432880498296b8f6903a31646c3d54))
* pass through unknown commands to exec instead of erroring ([198d711](https://github.com/mbabic84/kilo-docker/commit/198d711f72411b4ce85d8577dd483d104de71a6d))
* prompt for MCP tokens after ainstruct login completes ([7b1898c](https://github.com/mbabic84/kilo-docker/commit/7b1898c74dba1761c7734947b51c8e78d91d2a41))
* refresh MCP token variables after prompting user ([b0be148](https://github.com/mbabic84/kilo-docker/commit/b0be148ec8bc9b520fd9c26261f4180ea6269fe5))
* skip interactive prompts in sessions cleanup -y ([e10baf0](https://github.com/mbabic84/kilo-docker/commit/e10baf080daa59342e31e91dafeb42d6c2d26baa))

## [1.28.1](https://github.com/mbabic84/kilo-docker/compare/v1.28.0...v1.28.1) (2026-03-30)

### Bug Fixes

* add --mcp flag and fix token prompting ([c9a94d9](https://github.com/mbabic84/kilo-docker/commit/c9a94d91cf38e598b459015485a127fe21433d4c))
* prompt for each missing MCP token individually ([a3ef440](https://github.com/mbabic84/kilo-docker/commit/a3ef44040abcdef342d1bc7da2b8dd6527f9dd62))

## [1.28.0](https://github.com/mbabic84/kilo-docker/compare/v1.27.0...v1.28.0) (2026-03-30)

### Features

* require --ssh flag for SSH agent forwarding ([e112317](https://github.com/mbabic84/kilo-docker/commit/e11231767a06718a0140fdefc4b912c86a4c4c65))

## [1.27.0](https://github.com/mbabic84/kilo-docker/compare/v1.26.0...v1.27.0) (2026-03-30)

### Features

* use ~/.config/kilo/ for all Kilo subdirectories ([58d0b6b](https://github.com/mbabic84/kilo-docker/commit/58d0b6bdfa61c7040b57cedc46ec8388cae06f83))

## [1.26.0](https://github.com/mbabic84/kilo-docker/compare/v1.25.1...v1.26.0) (2026-03-30)

### Features

* add automatic SSH agent forwarding ([f7ae1f1](https://github.com/mbabic84/kilo-docker/commit/f7ae1f187b654bd90079aec6a2643d653ef8f68b))

## [1.25.1](https://github.com/mbabic84/kilo-docker/compare/v1.25.0...v1.25.1) (2026-03-30)

### Bug Fixes

* resolve ainstruct-sync collection initialization failure ([1930c48](https://github.com/mbabic84/kilo-docker/commit/1930c48871ec4308615253c19e7be7587ecc661f))

## [1.25.0](https://github.com/mbabic84/kilo-docker/compare/v1.24.0...v1.25.0) (2026-03-30)

### Features

* add session management with persistent containers ([252c780](https://github.com/mbabic84/kilo-docker/commit/252c78074eb4e2587f547a01db50176590d2906a))
* add sessions cleanup subcommand with -y flag ([e3a2554](https://github.com/mbabic84/kilo-docker/commit/e3a25546207d8793bca7318483a9ee5c763527cc))

## [1.24.0](https://github.com/mbabic84/kilo-docker/compare/v1.23.0...v1.24.0) (2026-03-30)

### Features

* add ainstruct file sync with push/pull and KD_ env var prefix ([4feb139](https://github.com/mbabic84/kilo-docker/commit/4feb13980cea110c0fa5cc2b7253c3cbe5d44e1e))
* rewrite ainstruct-sync from bash to Go ([d44e31f](https://github.com/mbabic84/kilo-docker/commit/d44e31f29c6f015929311f479f313800d997419d))

### Bug Fixes

* resolve review issues from PR [#58](https://github.com/mbabic84/kilo-docker/issues/58) ([b946969](https://github.com/mbabic84/kilo-docker/commit/b9469695b8b75ec14780813eb2ab97513ae7d0d7))

## [1.23.0](https://github.com/mbabic84/kilo-docker/compare/v1.22.0...v1.23.0) (2026-03-28)

### Features

* bump Kilo CLI to v7.1.9 ([a8f1f0f](https://github.com/mbabic84/kilo-docker/commit/a8f1f0f4f7a8dbca04313a28c93efa11b3c776fe))

## [1.22.0](https://github.com/mbabic84/kilo-docker/compare/v1.21.0...v1.22.0) (2026-03-27)

### Features

* add --volume flag to restore command for specifying target volume ([730790e](https://github.com/mbabic84/kilo-docker/commit/730790efd827589831bf36cc4ab65a010a993aa1))
* add backup and restore commands for volume management ([e35912f](https://github.com/mbabic84/kilo-docker/commit/e35912f1307ff2ec8cb96b8d683e72205af8e20b))
* randomize container user home from /home/kilo to /home/kilo-t8x3m7kp ([07fd6df](https://github.com/mbabic84/kilo-docker/commit/07fd6df69cdb1529f3f0cb1f53adb24700801f87))

### Bug Fixes

* remove incorrect path translation and misleading breaking change from help ([9774aa9](https://github.com/mbabic84/kilo-docker/commit/9774aa91a5b4292d9697821fdf046d1bed7d0efc))

## [1.21.0](https://github.com/mbabic84/kilo-docker/compare/v1.20.0...v1.21.0) (2026-03-27)

### Features

* **zellij:** limit bottom menu to only Ctrl+G and Ctrl+T ([8e01f08](https://github.com/mbabic84/kilo-docker/commit/8e01f08d93b16dd98ebe21026d68b112ed22d700))

## [1.20.0](https://github.com/mbabic84/kilo-docker/compare/v1.19.1...v1.20.0) (2026-03-27)

### Features

* download latest Docker client and Compose from official sources ([4f0b756](https://github.com/mbabic84/kilo-docker/commit/4f0b756eb82838264a5e41df9fdc045ce8f6c9bc))
* use alpine:latest for base image ([fece42f](https://github.com/mbabic84/kilo-docker/commit/fece42fb2be9da27a9b72548c686e6d893efdafb))

## [1.19.1](https://github.com/mbabic84/kilo-docker/compare/v1.19.0...v1.19.1) (2026-03-27)

### Bug Fixes

* quote workdir argument and remove redundant WORKDIR from Dockerfile ([ffd513c](https://github.com/mbabic84/kilo-docker/commit/ffd513c7a47140aeaaaf2dc5d2089ed21d396e12))

## [1.19.0](https://github.com/mbabic84/kilo-docker/compare/v1.18.0...v1.19.0) (2026-03-27)

### Features

* bump Kilo CLI to v7.1.8 ([3c64e73](https://github.com/mbabic84/kilo-docker/commit/3c64e73393c52e4e1d47e3f834b9a19171d5255d))

### Bug Fixes

* mount workspace to /mnt/<project-name> for correct docker compose project detection ([fbb5c9a](https://github.com/mbabic84/kilo-docker/commit/fbb5c9a95d1d0fb248f3d9dca017b0f95c0af2f5))

## [1.18.0](https://github.com/mbabic84/kilo-docker/compare/v1.17.0...v1.18.0) (2026-03-26)

### Features

* add --zellij flag for terminal multiplexer sessions ([a278056](https://github.com/mbabic84/kilo-docker/commit/a278056ee7cf05fd0fd8d89423abcea764bb6400))

## [1.17.0](https://github.com/mbabic84/kilo-docker/compare/v1.16.0...v1.17.0) (2026-03-26)

### Features

* bump Kilo CLI to v7.1.6 ([631da62](https://github.com/mbabic84/kilo-docker/commit/631da620287a521a30f4186a186bba2cdeb23645))

## [1.16.0](https://github.com/mbabic84/kilo-docker/compare/v1.15.0...v1.16.0) (2026-03-26)

### Features

* add --docker flag for Docker socket access ([c3d51c5](https://github.com/mbabic84/kilo-docker/commit/c3d51c54f832454b2d102e5a65bced4c84e63e96))

## [1.15.0](https://github.com/mbabic84/kilo-docker/compare/v1.14.3...v1.15.0) (2026-03-26)

### Features

* reduce first-time password setup from 3 prompts to 2 ([30f8bec](https://github.com/mbabic84/kilo-docker/commit/30f8bec7db8045c4c09b2dddac0a2e038dfaa73a))

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
