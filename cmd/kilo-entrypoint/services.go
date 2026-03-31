package main

// CopyConfig defines a file to copy from the container filesystem to the
// user's home directory after installation.
type CopyConfig struct {
	Src string // Source path in container
	Dst string // Destination path in user home
}

// Service defines a built-in service that can be installed in the container.
type Service struct {
	Name           string            // Internal name: "docker", "zellij"
	Flag           string            // CLI flag (unused in entrypoint, kept for struct parity)
	Description    string            // Help text (unused in entrypoint)
	Install        []string          // Shell commands to run inside container at startup
	EnvVars        map[string]string // Env vars (unused in entrypoint)
	Volumes        []string          // Volumes (unused in entrypoint)
	RequiresSocket string            // Host socket path if service needs one
	GIDEnvVar      string            // Env var containing socket GID (e.g., "DOCKER_GID")
	CopyConfigs    []CopyConfig      // Files to copy from container to user home
}

// builtInServices lists all services that can be installed.
// Must stay in sync with cmd/kilo-docker/services.go.
var builtInServices = []Service{
	{
		Name: "docker",
		Install: []string{
			"command -v docker >/dev/null || (curl -fsSL https://download.docker.com/linux/static/stable/x86_64/docker-28.0.4.tgz -o /tmp/docker.tgz && tar xzf /tmp/docker.tgz -C /tmp && mv /tmp/docker/docker /usr/local/bin/docker && chmod +x /usr/local/bin/docker && rm -rf /tmp/docker*)",
			"command -v docker-compose >/dev/null || (curl -fsSL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose && chmod +x /usr/local/bin/docker-compose && mkdir -p /usr/libexec/docker/cli-plugins && ln -sf /usr/local/bin/docker-compose /usr/libexec/docker/cli-plugins/docker-compose)",
		},
		RequiresSocket: "/var/run/docker.sock",
		GIDEnvVar:      "DOCKER_GID",
	},
	{
		Name: "zellij",
		Install: []string{
			"command -v zellij >/dev/null || (curl -fsSL https://github.com/zellij-org/zellij/releases/latest/download/zellij-x86_64-unknown-linux-musl.tar.gz -o /tmp/zellij.tar.gz && tar xzf /tmp/zellij.tar.gz -C /usr/local/bin && rm -rf /tmp/zellij.tar.gz)",
		},
		CopyConfigs: []CopyConfig{
			{Src: "/etc/zellij/config.kdl", Dst: "~/.config/zellij/config.kdl"},
		},
	},
	{
		Name: "go",
		Install: []string{
			"command -v go >/dev/null || (curl -fsSL https://go.dev/dl/go1.26.1.linux-amd64.tar.gz -o /tmp/go.tar.gz && tar -C /usr/local -xzf /tmp/go.tar.gz && rm -rf /tmp/go.tar.gz)",
			"echo 'export PATH=/usr/local/go/bin:$PATH' > /etc/profile.d/go.sh",
		},
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
