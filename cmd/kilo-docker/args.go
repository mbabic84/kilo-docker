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

func serializeArgs(cfg config) string {
	var parts []string

	utils.Log("[serializeArgs] cfg.once=%v, cfg.yes=%v\n", cfg.once, cfg.yes)

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

	if len(cfg.args) > 0 {
		parts = append(parts, cfg.args...)
	}

	return strings.Join(parts, " ")
}

func buildContainerArgs(cfg config, volume, workspace, containerName, containerState string) []string {

	args := []string{
		"--init",
		"--ipc=host",
		"-e", "PUID=" + strconv.Itoa(os.Getuid()),
		"-e", "PGID=" + strconv.Itoa(os.Getgid()),
		"-v", workspace + ":" + workspace,
		"-w", workspace,
		// Workspace path propagated so the entrypoint can export KD_WORKSPACE
		// without relying on os.Getwd() (which differs across `docker exec`
		// invocations).
		"-e", "KD_WORKSPACE=" + workspace,
	}

	// Pass supplementary group IDs so the container user can access
	// workspace files with group-level permissions.
	if groups, err := os.Getgroups(); err == nil {
		primaryGID := os.Getgid()
		var hostGroups []string
		for _, g := range groups {
			if g != primaryGID {
				hostGroups = append(hostGroups, strconv.Itoa(g))
			}
		}
		if len(hostGroups) > 0 {
			args = append(args, "-e", "PGIDS="+strings.Join(hostGroups, ","))
		}
	}

	if !cfg.once && volume != "" {
		args = append(args, "-v", volume+":/home")
	}

	args = append(args, "--label", "kilo.workspace="+workspace)

	sessionArgs := serializeArgs(cfg)
	args = append(args, "--label", "kilo.args="+sessionArgs)
	args = append(args, "--label", "kilo.version="+version)

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

	args = append(args, "--name", containerName)
	// In host/none/container network modes, Docker manages hostname resolution.
	// Setting --hostname causes "unable to resolve host" warnings. Skip it so
	// the container inherits the appropriate hostname behavior.
	if !containsSpecialNetwork(cfg.networks) {
		args = append(args, "--hostname", containerName)
	}

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

	if tz := resolveHostTimezone(); tz != "" {
		utils.Log("[args] host timezone: %s\n", tz)
		args = append(args, "-e", "TZ="+tz)
	}

	u, _ := user.Current()
	hostname, _ := os.Hostname()
	username := "unknown"
	if u != nil {
		username = u.Username
	}
	args = append(args, "-e", "KD_USERNAME="+username)
	args = append(args, "-e", "KD_HOSTNAME="+hostname)
	args = append(args, "--label", "kilo.owner="+username)
	args = append(args, "-e", "KILO_CONTAINER_NAME="+containerName)

	// Explicit host identity context for agents. These let scripts and the
	// agent distinguish host identity (KD_HOST_*) from in-container identity
	// (KD_CONTAINER_*, set by the entrypoint). KD_USERNAME above is kept as
	// a backward-compatible alias for KD_HOST_USER.
	if u != nil {
		args = append(args, "-e", "KD_HOST_USER="+u.Username)
		if u.HomeDir != "" {
			args = append(args, "-e", "KD_HOST_HOME="+u.HomeDir)
		}
		args = append(args, "-e", "KD_HOST_UID="+strconv.Itoa(os.Getuid()))
		args = append(args, "-e", "KD_HOST_GID="+strconv.Itoa(os.Getgid()))
	}
	args = append(args, "-e", "KD_IS_KILO_DOCKER=1")

	return args
}

// Host timezone sources are package variables so tests can redirect them
// to a temporary tree instead of the real filesystem.
var (
	hostLocaltimePath = "/etc/localtime"
	hostTimezonePath  = "/etc/timezone"
	hostZoneinfoRoot  = "/usr/share/zoneinfo"
)

// resolveHostTimezone returns the host's IANA timezone (for example
// "Europe/Berlin") so it can be propagated into the container via TZ.
//
// Detection order:
//  1. The TZ environment variable, when the user set it explicitly.
//  2. The /etc/localtime symlink, which systemd maintains on every
//     supported Ubuntu release (22.04 through 26.04 and newer). Ubuntu
//     25.10 removed /etc/timezone entirely, so this is the only reliable
//     source on current releases.
//  3. The legacy /etc/timezone file, still present and consistent on
//     22.04/24.04 and other Debian-derived hosts.
//
// The result is validated against the local zoneinfo tree, so a dangling
// or malformed source yields "" instead of an unusable TZ.
func resolveHostTimezone() string {
	if tz := strings.TrimSpace(os.Getenv("TZ")); tz != "" {
		return tz
	}
	if zone := zoneFromLocaltime(); zone != "" {
		return zone
	}
	return zoneFromEtcTimezone()
}

// zoneFromLocaltime derives an IANA zone from the /etc/localtime symlink.
// It returns "" when the path is missing, is a regular file copy rather
// than a symlink, or does not resolve inside a zoneinfo tree.
func zoneFromLocaltime() string {
	info, err := os.Lstat(hostLocaltimePath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return ""
	}
	target, err := os.Readlink(hostLocaltimePath)
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(hostLocaltimePath), target)
	}
	return zoneFromZoneinfoPath(filepath.Clean(target))
}

// zoneFromZoneinfoPath extracts the zone suffix from a resolved zoneinfo
// path such as "/usr/share/zoneinfo/Europe/Berlin" or
// "/usr/share/zoneinfo/America/Argentina/Buenos_Aires". Splitting on the
// final "zoneinfo/" component preserves region prefixes, unlike
// filepath.Base which returned only "Berlin"/"Buenos_Aires".
func zoneFromZoneinfoPath(path string) string {
	const marker = "zoneinfo/"
	idx := strings.LastIndex(path, marker)
	if idx < 0 {
		return ""
	}
	zone := path[idx+len(marker):]
	if !validZone(zone) {
		return ""
	}
	return zone
}

// zoneFromEtcTimezone reads the legacy /etc/timezone file, validating the
// value so a stale or malformed entry is not propagated.
func zoneFromEtcTimezone() string {
	data, err := os.ReadFile(hostTimezonePath)
	if err != nil {
		return ""
	}
	zone := strings.TrimSpace(string(data))
	if !validZone(zone) {
		return ""
	}
	return zone
}

// validZone reports whether zone names an existing entry in the local
// zoneinfo tree. It rejects empty values, absolute paths, and traversal
// so the value is always safe to embed in TZ. os.Root confines lookups to
// the zoneinfo tree even if the name contains unexpected path segments.
func validZone(zone string) bool {
	if zone == "" || strings.HasPrefix(zone, "/") || strings.Contains(zone, "..") {
		return false
	}
	root, err := os.OpenRoot(hostZoneinfoRoot)
	if err != nil {
		return false
	}
	info, statErr := root.Stat(zone)
	closeErr := root.Close()
	if statErr != nil || closeErr != nil {
		return false
	}
	return !info.IsDir()
}
