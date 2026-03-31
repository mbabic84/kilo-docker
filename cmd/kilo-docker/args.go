package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// buildDockerArgs constructs the full docker run argument list from the
// parsed config, SSH agent state, environment tokens, and container state.
func buildDockerArgs(cfg config, volume, pwd, containerName, containerState,
	sshAuthSock, dockerGID, kdContext7Token, kdAinstructToken,
	ainstructSyncToken, ainstructSyncRefreshToken string, ainstructSyncTokenExpiry int64) []string {

	args := []string{
		"--init",
		"--ipc=host",
		"-e", "PUID=" + strconv.Itoa(os.Getuid()),
		"-e", "PGID=" + strconv.Itoa(os.Getgid()),
		"-v", pwd + ":" + pwd,
		"-w", pwd,
	}

	if !cfg.once && volume != "" {
		args = append(args, "-v", volume+":"+kiloHome)
	}

	if cfg.once {
		args = append(args, "--rm")
	}

	args = append(args, "--label", "kilo.workspace="+pwd)

	sessionArgs := ""
	if cfg.once {
		sessionArgs += "--once "
	}
	if cfg.zellij {
		sessionArgs += "--zellij "
	}
	if cfg.playwright {
		sessionArgs += "--playwright "
	}
	if cfg.docker {
		sessionArgs += "--docker "
	}
	if sshAuthSock != "" {
		sessionArgs += "ssh-agent "
	}
	if cfg.encrypted && !cfg.ainstruct {
		sessionArgs += "-p "
	}
	if cfg.ainstruct {
		sessionArgs += "--ainstruct "
	}
	if cfg.mcp {
		sessionArgs += "--mcp "
	}
	if cfg.network != "" {
		sessionArgs += "--network " + cfg.network + " "
	}
	if len(cfg.args) > 0 {
		sessionArgs += strings.Join(cfg.args, " ") + " "
	}
	args = append(args, "--label", "kilo.args="+strings.TrimSpace(sessionArgs))

	if cfg.mcp && kdContext7Token != "" {
		args = append(args, "-e", "KD_CONTEXT7_TOKEN="+kdContext7Token)
	}
	if cfg.mcp && kdAinstructToken != "" {
		args = append(args, "-e", "KD_AINSTRUCT_TOKEN="+kdAinstructToken)
	}
	if cfg.playwright {
		args = append(args, "-e", "PLAYWRIGHT_ENABLED=1")
	}
	if cfg.docker {
		args = append(args, "-v", "/var/run/docker.sock:/var/run/docker.sock")
		args = append(args, "-e", "DOCKER_ENABLED=1")
		args = append(args, "-e", "DOCKER_GID="+dockerGID)
	}
	if cfg.zellij {
		args = append(args, "-e", "ZELLIJ_ENABLED=1")
	}
	if sshAuthSock != "" {
		args = append(args, "-v", sshAuthSock+":/ssh-agent.sock")
		args = append(args, "-e", "SSH_AUTH_SOCK=/ssh-agent.sock")
	}

	args = append(args, "--name", containerName)

	if cfg.ainstruct {
		args = append(args, "-e", "KD_AINSTRUCT_ENABLED=1")
		args = append(args, "-e", "KD_AINSTRUCT_API_URL=https://ainstruct-dev.kralicinora.cz/api/v1")
		if ainstructSyncToken != "" {
			args = append(args, "-e", "KD_AINSTRUCT_SYNC_TOKEN="+ainstructSyncToken)
		}
		if ainstructSyncRefreshToken != "" {
			args = append(args, "-e", "KD_AINSTRUCT_SYNC_REFRESH_TOKEN="+ainstructSyncRefreshToken)
		}
		if ainstructSyncTokenExpiry > 0 {
			args = append(args, "-e", "KD_AINSTRUCT_SYNC_TOKEN_EXPIRY="+strconv.FormatInt(ainstructSyncTokenExpiry, 10))
		}
	}

	if cfg.network != "" {
		args = append(args, "--network", cfg.network)
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

	return args
}
