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
