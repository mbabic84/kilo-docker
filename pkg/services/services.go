// Package services defines built-in optional tools exposed by kilo-docker.
package services

type CopyConfig struct {
	Src  string
	Dst  string
}

type Service struct {
	Name           string
	Flag           string
	Description    string
	Install        []string
	UserInstall    []string // Install commands that run after user creation with HOME set to user home
	VersionCheck   string   // Command to check current version (returns version string or empty)
	LatestVersion  string   // Command to get latest available version
	EnvVars        map[string]string
	HostEnvVars    map[string]string
	Volumes        []string
	RequiresSocket string
	GIDEnvVar      string
	CopyConfigs    []CopyConfig
}

var BuiltInServices = []Service{
	{
		Name:        "docker",
		Flag:        "--docker",
		Description: "Mount Docker socket for container management from within Kilo",
		Install: []string{
			"command -v docker >/dev/null || (DOCKER_VERSION=$(curl -fsSL https://api.github.com/repos/docker/docker/releases/latest 2>/dev/null | grep -o 'docker-v[0-9.]*' | head -1 | sed 's/docker-v//') && curl -fsSL \"https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz\" -o /tmp/docker.tgz && tar xzf /tmp/docker.tgz -C /tmp && mv /tmp/docker/docker /usr/local/bin/docker && chmod +x /usr/local/bin/docker && rm -rf /tmp/docker*)",
			"command -v docker-compose >/dev/null || (curl -fsSL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose && chmod +x /usr/local/bin/docker-compose && mkdir -p /usr/libexec/docker/cli-plugins && ln -sf /usr/local/bin/docker-compose /usr/libexec/docker/cli-plugins/docker-compose)",
			"command -v docker-buildx >/dev/null || (BUILDX_VERSION=$(curl -fsSL https://api.github.com/repos/docker/buildx/releases/latest 2>/dev/null | grep '\"tag_name\":' | head -1 | sed 's/.*\"v*\\([^\"]*\\)\".*/\\1/') && BUILDX_ARCH=$(case $(uname -m) in x86_64) echo 'amd64' ;; aarch64|arm64) echo 'arm64' ;; esac) && curl -fsSL \"https://github.com/docker/buildx/releases/download/v${BUILDX_VERSION}/buildx-v${BUILDX_VERSION}.linux-${BUILDX_ARCH}\" -o /tmp/docker-buildx && chmod +x /tmp/docker-buildx && mkdir -p /usr/libexec/docker/cli-plugins && mv /tmp/docker-buildx /usr/libexec/docker/cli-plugins/docker-buildx)",
		},
		EnvVars: map[string]string{
			"DOCKER_ENABLED": "1",
		},
		HostEnvVars: map[string]string{
			"DOCKER_GID": "",
		},
		Volumes:        []string{"/var/run/docker.sock:/var/run/docker.sock"},
		RequiresSocket: "/var/run/docker.sock",
		GIDEnvVar:      "DOCKER_GID",
	},
	{
		Name:        "go",
		Flag:        "--go",
		Description: "Install Go (latest stable) for development",
		Install: []string{
			"GO_VERSION=$(curl -fsSL https://go.dev/VERSION?m=text | head -1) && command -v go >/dev/null || (curl -fsSL \"https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz\" -o /tmp/go.tar.gz && tar -C /usr/local -xzf /tmp/go.tar.gz && rm -rf /tmp/go.tar.gz)",
			"cat > /usr/local/bin/go-wrapper << 'SCRIPT'\n#!/bin/sh\nGOPATH=${GOPATH:-$HOME/go}\nWS=\"$PWD\"\nwhile [ \"$WS\" != / ] && [ ! -f \"$WS/go.mod\" ]; do WS=$(dirname \"$WS\"); done\nif [ -f \"$WS/go.mod\" ]; then\n  export GOCACHE=\"$WS/.cache/go-build\"\n  export GOMODCACHE=\"$WS/.cache/mod\"\nelse\n  export GOCACHE=\"${GOCACHE:-$HOME/.cache/go-build}\"\n  export GOMODCACHE=\"${GOMODCACHE:-$GOPATH/pkg/mod}\"\nfi\nexport GOPATH\nexec /usr/local/go/bin/go \"$@\"\nSCRIPT",
			"chmod +x /usr/local/bin/go-wrapper",
			"ln -sf /usr/local/bin/go-wrapper /usr/local/bin/go",
			"command -v golangci-lint >/dev/null || (curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin)",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
	{
		Name:        "node",
		Flag:        "--node",
		Description: "Install Node.js LTS for development",
		Install: []string{
			"command -v node >/dev/null || apt-get update && apt-get install -y --no-install-recommends nodejs npm",
		},
		EnvVars: map[string]string{
			"NODE_ENABLED": "1",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
	{
		Name:        "gh",
		Flag:        "--gh",
		Description: "Install GitHub CLI for interacting with GitHub",
		Install: []string{
			"command -v gh >/dev/null || apt-get update && apt-get install -y --no-install-recommends gh",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
	{
		Name:        "uv",
		Flag:        "--uv",
		Description: "Install uv for fast Python package management",
		UserInstall: []string{
			"curl -LsSf https://astral.sh/uv/install.sh | sh",
		},
		VersionCheck:  "[ -f \"$HOME/.local/bin/uv\" ] && \"$HOME/.local/bin/uv\" --version 2>/dev/null | awk '{print $2}'",
		LatestVersion: "curl -s https://api.github.com/repos/astral-sh/uv/releases/latest | grep '\"tag_name\":' | sed 's/.*\"v*\\([0-9.]*\\)\".*/\\1/'",
		EnvVars: map[string]string{
			"UV_ENABLED": "1",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
	{
		Name:        "build",
		Flag:        "--build",
		Description: "Install build base dependencies (gcc, g++, make, python3) for compiling native extensions",
		Install: []string{
"command -v gcc >/dev/null || apt-get update && apt-get install -y --no-install-recommends build-essential",
		"command -v python3 >/dev/null || apt-get update && apt-get install -y --no-install-recommends python3",
		},
		EnvVars: map[string]string{
			"BUILD_ENABLED": "1",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
	{
		Name:        "nvm",
		Flag:        "--nvm",
		Description: "Install NVM (Node Version Manager) for managing Node.js versions",
		UserInstall: []string{
			"curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/master/install.sh | bash",
		},
		VersionCheck:  "[ -d \"$HOME/.nvm\" ] && git -C \"$HOME/.nvm\" describe --tags 2>/dev/null | sed 's/v//' || echo \"\"",
		LatestVersion: "curl -s https://api.github.com/repos/nvm-sh/nvm/releases/latest | grep '\"tag_name\":' | sed 's/.*v\\([0-9.]*\\).*/\\1/'",
		EnvVars: map[string]string{
			"NVM_NODEJS_ORG_MIRROR": "https://unofficial-builds.nodejs.org/download/release",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
	{
		Name:        "python",
		Flag:        "--python",
		Description: "Install Python 3 with symlink for general purpose use",
		Install: []string{
"command -v python3 >/dev/null || apt-get update && apt-get install -y --no-install-recommends python3",
		"[ -f /usr/local/bin/python ] || ln -sf $(command -v python3) /usr/local/bin/python",
		},
		EnvVars: map[string]string{
			"PYTHON_ENABLED": "1",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
	{
		Name:        "rclone",
		Flag:        "--rclone",
		Description: "Install rclone, a universal CLI for S3 and 40+ cloud storage backends",
		Install: []string{
			"command -v rclone >/dev/null || (RCLONE_VERSION=$(curl -fsSL https://api.github.com/repos/rclone/rclone/releases/latest 2>/dev/null | grep '\"tag_name\":' | head -1 | sed 's/.*\"v*\\([^\"]*\\)\".*/\\1/') && RCLONE_ARCH=$(case $(uname -m) in x86_64) echo 'amd64' ;; aarch64|arm64) echo 'arm64' ;; esac) && command -v unzip >/dev/null || apt-get update && apt-get install -y --no-install-recommends unzip && curl -fsSL \"https://github.com/rclone/rclone/releases/download/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-${RCLONE_ARCH}.zip\" -o /tmp/rclone.zip && unzip -o /tmp/rclone.zip -d /tmp && mv /tmp/rclone-v${RCLONE_VERSION}-linux-${RCLONE_ARCH}/rclone /usr/local/bin/rclone && chmod +x /usr/local/bin/rclone && rm -rf /tmp/rclone.zip /tmp/rclone-v${RCLONE_VERSION}-linux-${RCLONE_ARCH})",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
}

func GetService(name string) *Service {
	for i := range BuiltInServices {
		if BuiltInServices[i].Name == name {
			return &BuiltInServices[i]
		}
	}
	return nil
}
