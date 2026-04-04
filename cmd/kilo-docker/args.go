package main

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

func serializeArgs(cfg config, sshEnabled bool) string {
	var sessionArgs string
	if cfg.once {
		sessionArgs += "--once "
	}
	for _, svcName := range cfg.enabledServices {
		svc := getService(svcName)
		if svc != nil && svc.Flag != "" {
			sessionArgs += svc.Flag + " "
		}
	}
	if cfg.playwright {
		sessionArgs += "--playwright "
	}
	if sshEnabled {
		sessionArgs += "--ssh "
	}
	if cfg.mcp {
		sessionArgs += "--mcp "
	}
	if cfg.network != "" {
		sessionArgs += "--network " + cfg.network + " "
	}
	for _, port := range cfg.ports {
		sessionArgs += "--port " + port + " "
	}
	if len(cfg.args) > 0 {
		sessionArgs += strings.Join(cfg.args, " ") + " "
	}
	return strings.TrimSpace(sessionArgs)
}

func buildContainerArgs(cfg config, volume, pwd, containerName, containerState,
	sshAuthSock string, hostEnvVars map[string]string) []string {

	args := []string{
		"--init",
		"--ipc=host",
		"-e", "PUID=" + strconv.Itoa(os.Getuid()),
		"-e", "PGID=" + strconv.Itoa(os.Getgid()),
		"-v", pwd + ":" + pwd,
		"-w", pwd,
	}

	if !cfg.once && volume != "" {
		args = append(args, "-v", volume+":/home")
	}

	args = append(args, "--label", "kilo.workspace="+pwd)

	sessionArgs := serializeArgs(cfg, sshAuthSock != "")
	args = append(args, "--label", "kilo.args="+sessionArgs)

	if cfg.mcp {
		args = append(args, "-e", "KD_MCP_ENABLED=1")
	}
	if cfg.playwright {
		args = append(args, "-e", "PLAYWRIGHT_ENABLED=1")
	}
	for _, svcName := range cfg.enabledServices {
		svc := getService(svcName)
		if svc == nil {
			continue
		}
		for key, value := range svc.EnvVars {
			if value != "" {
				args = append(args, "-e", key+"="+value)
			}
		}
		for key := range svc.HostEnvVars {
			if val, ok := hostEnvVars[key]; ok {
				args = append(args, "-e", key+"="+val)
			}
		}
		for _, vol := range svc.Volumes {
			args = append(args, "-v", vol)
		}
	}
	if len(cfg.enabledServices) > 0 {
		args = append(args, "-e", "KD_SERVICES="+strings.Join(cfg.enabledServices, ","))
	}
	if sshAuthSock != "" {
		args = append(args, "-v", sshAuthSock+":/ssh-agent.sock")
		args = append(args, "-e", "SSH_AUTH_SOCK=/ssh-agent.sock")
	}

	args = append(args, "--name", containerName)
	args = append(args, "--hostname", containerName)

	if cfg.network != "" {
		args = append(args, "--network", cfg.network)
	}

	for _, port := range cfg.ports {
		args = append(args, "-p", port)
	}

	for _, envVar := range []string{"TERM", "COLORTERM", "LANG", "LC_ALL"} {
		if val := os.Getenv(envVar); val != "" {
			args = append(args, "-e", envVar+"="+val)
		}
	}

	if tz := os.Getenv("TZ"); tz != "" {
		args = append(args, "-e", "TZ="+tz)
	} else if _, err := os.Stat("/etc/timezone"); err == nil {
		data, _ := os.ReadFile("/etc/timezone")
		args = append(args, "-e", "TZ="+strings.TrimSpace(string(data)))
	} else if info, _ := os.Lstat("/etc/localtime"); info != nil && info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink("/etc/localtime")
		args = append(args, "-e", "TZ="+filepath.Base(target))
	}

	u, _ := user.Current()
	hostname, _ := os.Hostname()
	username := "unknown"
	if u != nil {
		username = u.Username
	}
	args = append(args, "-e", "PAT_USERNAME="+username)
	args = append(args, "-e", "PAT_HOSTNAME="+hostname)

	return args
}