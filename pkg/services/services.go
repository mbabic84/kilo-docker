// Package services defines built-in optional tools exposed by kilo-docker.
package services

type CopyConfig struct {
	Src  string
	Dst  string
}

// UpdateCheck describes one independently versioned binary installed by a
// service. Services that install more than one tool (e.g. --gitea installs tea
// and gitea-mcp) list each binary here so updates can be detected per binary
// while still running the service's Install commands as a single batch.
type UpdateCheck struct {
	Name    string // display name, e.g. "tea"
	Current string // shell command printing the installed version
	Latest  string // shell command printing the latest version
}

type Service struct {
	Name                 string
	DisplayName          string
	Flag                 string
	Description          string
	Install              []string
	UserInstall          []string
	VersionCheck         string
	LatestVersion        string
	InstallVersionCheck  string
	InstallLatestVersion string
	InstallUpdates       []UpdateCheck
	EnvVars              map[string]string
	Volumes              []string
	RequiresSocket       string
	CopyConfigs          []CopyConfig
}

// DisplayNameOrName returns DisplayName if set, otherwise Name.
func (s *Service) DisplayNameOrName() string {
	if s.DisplayName != "" {
		return s.DisplayName
	}
	return s.Name
}

var BuiltInServices = []Service{
	{
		Name:        "docker",
		Flag:        "--docker",
		Description: "Mount Docker socket for container management from within Kilo",
		Install: []string{
			"command -v docker >/dev/null || (DOCKER_VERSION=$(curl -fsSL https://download.docker.com/linux/static/stable/x86_64/ | grep -oE 'docker-[0-9]+\\.[0-9]+\\.[0-9]+\\.tgz' | sed 's/docker-//;s/\\.tgz//' | sort -V | tail -1) && curl -fsSL \"https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz\" -o /tmp/docker.tgz && tar xzf /tmp/docker.tgz -C /tmp && mv /tmp/docker/docker /usr/local/bin/docker && chmod +x /usr/local/bin/docker && rm -rf /tmp/docker*)",
			"command -v docker-compose >/dev/null || (curl -fsSL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose && chmod +x /usr/local/bin/docker-compose && mkdir -p /usr/libexec/docker/cli-plugins && ln -sf /usr/local/bin/docker-compose /usr/libexec/docker/cli-plugins/docker-compose)",
			"command -v docker-buildx >/dev/null || (BUILDX_VERSION=$(curl -fsSI -o /dev/null -w '%{redirect_url}' https://github.com/docker/buildx/releases/latest | sed 's#.*/tag/v\\?##') && BUILDX_ARCH=$(case $(uname -m) in x86_64) echo 'amd64' ;; aarch64|arm64) echo 'arm64' ;; esac) && curl -fsSL \"https://github.com/docker/buildx/releases/download/v${BUILDX_VERSION}/buildx-v${BUILDX_VERSION}.linux-${BUILDX_ARCH}\" -o /tmp/docker-buildx && chmod +x /tmp/docker-buildx && mkdir -p /usr/libexec/docker/cli-plugins && mv /tmp/docker-buildx /usr/libexec/docker/cli-plugins/docker-buildx)",
		},
		EnvVars: map[string]string{
			"DOCKER_ENABLED": "1",
		},
		Volumes:        []string{"/var/run/docker.sock:/var/run/docker.sock"},
		RequiresSocket: "/var/run/docker.sock",
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
		Name:        "gh",
		DisplayName: "gh-mcp extension",
		Flag:        "--gh",
		Description: "Install GitHub CLI for interacting with GitHub",
		Install: []string{
			"(GH_VERSION=$(curl -fsSI -o /dev/null -w '%{redirect_url}' https://github.com/cli/cli/releases/latest | sed 's#.*/tag/v\\?##') && GH_ARCH=$(case $(uname -m) in x86_64) echo 'amd64' ;; aarch64|arm64) echo 'arm64' ;; esac) && curl -fsSL \"https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${GH_ARCH}.tar.gz\" -o /tmp/gh.tar.gz && tar xzf /tmp/gh.tar.gz -C /tmp && mv /tmp/gh_${GH_VERSION}_linux_${GH_ARCH}/bin/gh /usr/local/bin/gh && chmod +x /usr/local/bin/gh && rm -rf /tmp/gh*)",
		},
		UserInstall: []string{
			"gh extension remove shuymn/gh-mcp 2>/dev/null; gh extension install shuymn/gh-mcp",
		},
		VersionCheck:         "gh extension list 2>/dev/null | grep 'shuymn/gh-mcp' | awk '{print $NF}' | sed 's/^v//'",
		LatestVersion:        "curl -fsSI -o /dev/null -w '%{redirect_url}' https://github.com/shuymn/gh-mcp/releases/latest | sed 's#.*/tag/v\\?##'",
		InstallVersionCheck:  "command -v gh >/dev/null && gh --version 2>/dev/null | head -1 | awk '{print $3}'",
		InstallLatestVersion: "curl -fsSI -o /dev/null -w '%{redirect_url}' https://github.com/cli/cli/releases/latest | sed 's#.*/tag/v\\?##'",
		Volumes:              []string{},
		RequiresSocket:       "",
	},
	{
		Name:        "gitea",
		DisplayName: "Gitea CLI + MCP",
		Flag:        "--gitea",
		Description: "Install the tea CLI and the gitea-mcp server for Gitea interactions",
		Install: []string{
			`TEA_VERSION=$(curl -fsSL --connect-timeout 5 --max-time 20 https://gitea.com/api/v1/repos/gitea/tea/releases/latest | jq -r '.tag_name | ltrimstr("v")') && TEA_ARCH=$(case $(uname -m) in x86_64) echo amd64 ;; aarch64|arm64) echo arm64 ;; esac) && curl -fsSL --connect-timeout 10 --max-time 120 "https://dl.gitea.com/tea/${TEA_VERSION}/tea-${TEA_VERSION}-linux-${TEA_ARCH}.xz" -o /tmp/tea.xz && xz -dc /tmp/tea.xz > /usr/local/bin/.tea.tmp && chmod +x /usr/local/bin/.tea.tmp && mv /usr/local/bin/.tea.tmp /usr/local/bin/tea && rm -f /tmp/tea.xz`,
			`MCP_TAG=$(curl -fsSL --connect-timeout 5 --max-time 20 https://gitea.com/api/v1/repos/gitea/gitea-mcp/releases/latest | jq -r '.tag_name') && MCP_ARCH=$(case $(uname -m) in x86_64) echo x86_64 ;; aarch64|arm64) echo arm64 ;; esac) && curl -fsSL --connect-timeout 10 --max-time 120 "https://gitea.com/gitea/gitea-mcp/releases/download/${MCP_TAG}/gitea-mcp_Linux_${MCP_ARCH}.tar.gz" -o /tmp/gitea-mcp.tar.gz && tar -xzf /tmp/gitea-mcp.tar.gz -C /tmp gitea-mcp && mv /tmp/gitea-mcp /usr/local/bin/gitea-mcp && chmod +x /usr/local/bin/gitea-mcp && rm -f /tmp/gitea-mcp.tar.gz`,
		},
		InstallUpdates: []UpdateCheck{
			{
				Name:    "tea",
				Current: `command -v tea >/dev/null && tea --version 2>/dev/null | head -1 | sed -E 's/\x1b\[[0-9;]*m//g; s/^Version:[[:space:]]+//' | awk '{print $1}'`,
				Latest:  `curl -fsSL --connect-timeout 5 --max-time 20 https://gitea.com/api/v1/repos/gitea/tea/releases/latest | jq -r '.tag_name | ltrimstr("v")'`,
			},
			{
				Name:    "gitea-mcp",
				Current: `command -v gitea-mcp >/dev/null && gitea-mcp -v 2>/dev/null | head -1`,
				Latest:  `curl -fsSL --connect-timeout 5 --max-time 20 https://gitea.com/api/v1/repos/gitea/gitea-mcp/releases/latest | jq -r '.tag_name | ltrimstr("v")'`,
			},
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
		LatestVersion: "curl -fsSI -o /dev/null -w '%{redirect_url}' https://github.com/astral-sh/uv/releases/latest | sed 's#.*/tag/v\\?##'",
		EnvVars: map[string]string{
			"UV_ENABLED": "1",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
	{
		Name:        "build",
		Flag:        "--build",
		Description: "Install build base dependencies (gcc, g++, make) for compiling native extensions",
		Install: []string{
		"command -v gcc >/dev/null || apt-get update && apt-get install -y --no-install-recommends build-essential",
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
		VersionCheck:  "[ -f \"$HOME/.nvm/nvm.sh\" ] && . \"$HOME/.nvm/nvm.sh\" && nvm --version 2>/dev/null || echo \"\"",
		LatestVersion: "curl -fsSI -o /dev/null -w '%{redirect_url}' https://github.com/nvm-sh/nvm/releases/latest | sed 's#.*/tag/v\\?##'",
		EnvVars: map[string]string{},
		Volumes:        []string{},
		RequiresSocket: "",
	},
	{
		Name:        "rclone",
		Flag:        "--rclone",
		Description: "Install rclone, a universal CLI for S3 and 40+ cloud storage backends",
		Install: []string{
			"command -v rclone >/dev/null || (RCLONE_VERSION=$(curl -fsSI -o /dev/null -w '%{redirect_url}' https://github.com/rclone/rclone/releases/latest | sed 's#.*/tag/v\\?##') && RCLONE_ARCH=$(case $(uname -m) in x86_64) echo 'amd64' ;; aarch64|arm64) echo 'arm64' ;; esac) && command -v unzip >/dev/null || apt-get update && apt-get install -y --no-install-recommends unzip && curl -fsSL \"https://github.com/rclone/rclone/releases/download/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-${RCLONE_ARCH}.zip\" -o /tmp/rclone.zip && unzip -o /tmp/rclone.zip -d /tmp && mv /tmp/rclone-v${RCLONE_VERSION}-linux-${RCLONE_ARCH}/rclone /usr/local/bin/rclone && chmod +x /usr/local/bin/rclone && rm -rf /tmp/rclone.zip /tmp/rclone-v${RCLONE_VERSION}-linux-${RCLONE_ARCH})",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
	{
		Name:        "gitnexus",
		Flag:        "--gitnexus",
		Description: "Install GitNexus for codebase knowledge graph indexing and MCP-based code intelligence",
		UserInstall: []string{
			"export NVM_DIR=\"$HOME/.nvm\" && if [ ! -s \"$NVM_DIR/nvm.sh\" ]; then curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/master/install.sh | bash; fi && . \"$NVM_DIR/nvm.sh\" && nvm install --lts && npm install -g gitnexus",
		},
		EnvVars: map[string]string{
			"GITNEXUS_ENABLED": "1",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
	{
		Name:        "diagnostics",
		Flag:        "--diagnostics",
		Description: "Install diagnostic tools (ps, top, htop, ss, lsof, netstat, nc, ping, tracepath, mtr, dig, iostat, sar, iotop, ncdu, tree, file, pstree, xxd) plus gitleaks for pre-commit secret detection",
		Install: []string{
			"command -v ps >/dev/null || (apt-get update && apt-get install -y --no-install-recommends procps iproute2 lsof net-tools netcat-openbsd iputils-ping iputils-tracepath psmisc dnsutils htop sysstat iotop mtr-tiny ncdu tree file xxd && rm -rf /var/lib/apt/lists/*) && command -v gitleaks >/dev/null || (GITLEAKS_VERSION=$(curl -fsSI -o /dev/null -w '%{redirect_url}' https://github.com/gitleaks/gitleaks/releases/latest | sed 's#.*/tag/v\\?##') && GITLEAKS_ARCH=$(case $(uname -m) in x86_64) echo 'x64' ;; aarch64|arm64) echo 'arm64' ;; esac) && curl -fsSL \"https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_${GITLEAKS_ARCH}.tar.gz\" | tar -xz -C /tmp gitleaks && mv /tmp/gitleaks /usr/local/bin/gitleaks && chmod +x /usr/local/bin/gitleaks)",
		},
		EnvVars: map[string]string{
			"DIAGNOSTICS_ENABLED": "1",
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
