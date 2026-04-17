package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

func serializeArgs(cfg config, sshEnabled bool) string {
	var parts []string

	utils.Log("[serializeArgs] cfg.remember=%v, cfg.ssh=%v, cfg.once=%v, cfg.yes=%v\n", cfg.remember, cfg.ssh, cfg.once, cfg.yes)

	for _, f := range boolFlags {
		if serialized, ok := f.serialize(cfg); ok {
			parts = append(parts, serialized)
		}
	}

	for _, f := range valueFlags {
		if serialized := f.serializeArgs(cfg); len(serialized) > 0 {
			parts = append(parts, serialized...)
		}
	}

	for _, svcName := range cfg.enabledServices {
		svc := getService(svcName)
		if svc != nil && svc.Flag != "" {
			parts = append(parts, svc.Flag)
		}
	}

	if sshEnabled && !cfg.ssh {
		parts = append(parts, "--ssh")
	}

	if len(cfg.args) > 0 {
		parts = append(parts, cfg.args...)
	}

	return strings.Join(parts, " ")
}

func buildContainerArgs(cfg config, volume, workspace, containerName, containerState,
	sshAuthSock string, hostEnvVars map[string]string) []string {

	args := []string{
		"--init",
		"--ipc=host",
		"-e", "PUID=" + strconv.Itoa(os.Getuid()),
		"-e", "PGID=" + strconv.Itoa(os.Getgid()),
		"-v", workspace + ":" + workspace,
		"-w", workspace,
	}

	if !cfg.once && volume != "" {
		args = append(args, "-v", volume+":/home")
	}

	args = append(args, "--label", "kilo.workspace="+workspace)

	sessionArgs := serializeArgs(cfg, sshAuthSock != "")
	args = append(args, "--label", "kilo.args="+sessionArgs)

	if cfg.playwright {
		args = append(args, "-e", "PLAYWRIGHT_ENABLED=1")
		// Mount shared volume at /mnt/playwright-output in Kilo container
		args = append(args, "-v", fmt.Sprintf("%s:/mnt/playwright-output", PlaywrightVolumeName))
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
	for _, vol := range cfg.volumes {
		args = append(args, "-v", vol)
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

	for _, f := range valueFlags {
		if dockerArgs := f.buildDockerArgs(cfg); len(dockerArgs) > 0 {
			args = append(args, dockerArgs...)
		}
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
	args = append(args, "-e", "KILO_CONTAINER_NAME="+containerName)

	return args
}
