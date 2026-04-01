package main

// CopyConfig defines a file to copy from the container filesystem to the
// user's home directory after installation.
type CopyConfig struct {
	Src  string // Source path in container (e.g., "/etc/zellij/config.kdl")
	Dst  string // Destination path in user home (e.g., "~/.config/zellij/config.kdl")
}

// Service defines a built-in service that can be enabled via CLI flags.
// Each service specifies how it is installed inside the container, what
// environment variables it needs, what volumes to mount from the host,
// and optional host-side requirements.
type Service struct {
	Name           string            // Internal name: "docker", "zellij"
	Flag           string            // CLI flag: "--docker", "--zellij"
	Description    string            // Help text
	Install        []string          // Shell commands to run inside container at startup
	EnvVars        map[string]string // Env vars with static values
	HostEnvVars    map[string]string // Env vars to set from host (empty value = key only, no value)
	Volumes        []string          // Volumes to mount from host
	RequiresSocket string            // Host socket path if service needs one (e.g., "/var/run/docker.sock")
	GIDEnvVar      string            // Env var containing socket GID (e.g., "DOCKER_GID")
	CopyConfigs    []CopyConfig      // Files to copy from container to user home
}

// builtInServices lists all services that can be enabled via CLI flags.
var builtInServices = []Service{
	{
		Name:        "docker",
		Flag:        "--docker",
		Description: "Mount Docker socket for container management from within Kilo",
		Install: []string{
			"command -v docker >/dev/null || (curl -fsSL https://download.docker.com/linux/static/stable/x86_64/docker-28.0.4.tgz -o /tmp/docker.tgz && tar xzf /tmp/docker.tgz -C /tmp && mv /tmp/docker/docker /usr/local/bin/docker && chmod +x /usr/local/bin/docker && rm -rf /tmp/docker*)",
			"command -v docker-compose >/dev/null || (curl -fsSL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose && chmod +x /usr/local/bin/docker-compose && mkdir -p /usr/libexec/docker/cli-plugins && ln -sf /usr/local/bin/docker-compose /usr/libexec/docker/cli-plugins/docker-compose)",
		},
		EnvVars: map[string]string{
			"DOCKER_ENABLED": "1",
		},
		HostEnvVars: map[string]string{
			"DOCKER_GID": "", // Set dynamically from host socket GID
		},
		Volumes:        []string{"/var/run/docker.sock:/var/run/docker.sock"},
		RequiresSocket: "/var/run/docker.sock",
		GIDEnvVar:      "DOCKER_GID",
	},
	{
		Name:        "zellij",
		Flag:        "--zellij",
		Description: "Start with Zellij multiplexer (detach: Ctrl+P Ctrl+Q, reattach: kilo-docker sessions)",
		Install: []string{
			"command -v zellij >/dev/null || (curl -fsSL https://github.com/zellij-org/zellij/releases/latest/download/zellij-x86_64-unknown-linux-musl.tar.gz -o /tmp/zellij.tar.gz && tar xzf /tmp/zellij.tar.gz -C /usr/local/bin && rm -rf /tmp/zellij.tar.gz)",
		},
		EnvVars: map[string]string{
			"ZELLIJ_ENABLED": "1",
		},
		Volumes:        []string{},
		RequiresSocket: "",
		CopyConfigs: []CopyConfig{
			{Src: "/etc/zellij/config.kdl", Dst: "~/.config/zellij/config.kdl"},
		},
	},
	{
		Name:        "go",
		Flag:        "--go",
		Description: "Install Go 1.26.1 (latest stable) for development",
		Install: []string{
			"command -v go >/dev/null || (curl -fsSL https://go.dev/dl/go1.26.1.linux-amd64.tar.gz -o /tmp/go.tar.gz && tar -C /usr/local -xzf /tmp/go.tar.gz && rm -rf /tmp/go.tar.gz)",
			"cat > /usr/local/bin/go-wrapper << 'SCRIPT'\n#!/bin/sh\nGOPATH=${GOPATH:-$HOME/go}\nWS=\"$PWD\"\nwhile [ \"$WS\" != / ] && [ ! -f \"$WS/go.mod\" ]; do WS=$(dirname \"$WS\"); done\nif [ -f \"$WS/go.mod\" ]; then\n  export GOCACHE=\"$WS/.cache/go-build\"\n  export GOMODCACHE=\"$WS/.cache/mod\"\nelse\n  export GOCACHE=\"${GOCACHE:-$HOME/.cache/go-build}\"\n  export GOMODCACHE=\"${GOMODCACHE:-$GOPATH/pkg/mod}\"\nfi\nexport GOPATH\nexec /usr/local/go/bin/go \"$@\"\nSCRIPT",
			"chmod +x /usr/local/bin/go-wrapper",
			"ln -sf /usr/local/bin/go-wrapper /usr/local/bin/go",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
	{
		Name:        "node",
		Flag:        "--node",
		Description: "Install Node.js LTS for development",
		Install: []string{
			"command -v node >/dev/null || apk add --no-cache nodejs npm",
		},
		EnvVars: map[string]string{
			"NODE_ENABLED": "1",
		},
		Volumes:        []string{},
		RequiresSocket: "",
	},
}

// getService returns the service with the given name, or nil if not found.
func getService(name string) *Service {
	for i := range builtInServices {
		if builtInServices[i].Name == name {
			return &builtInServices[i]
		}
	}
	return nil
}

// isServiceEnabled checks if a service name is in the enabled list.
func isServiceEnabled(cfg config, name string) bool {
	for _, svc := range cfg.enabledServices {
		if svc == name {
			return true
		}
	}
	return false
}
