package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildContainerArgsWithDockerService(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{"docker"},
	}

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found")

	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "-v vol:/home") {
		t.Error("expected volume mount in args")
	}
	if !strings.Contains(argsStr, "--docker") {
		t.Error("expected --docker in session args")
	}
	if !strings.Contains(argsStr, "-e DOCKER_ENABLED=1") {
		t.Error("expected DOCKER_ENABLED env var")
	}
	if !strings.Contains(argsStr, "-v /var/run/docker.sock:/var/run/docker.sock") {
		t.Error("expected docker socket volume")
	}
	if !strings.Contains(argsStr, "-e KD_SERVICES=docker") {
		t.Error("expected KD_SERVICES env var")
	}
}

func TestBuildContainerArgsNoServices(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{},
	}

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found")

	argsStr := strings.Join(args, " ")

	if strings.Contains(argsStr, "KD_SERVICES") {
		t.Error("expected no KD_SERVICES env var when no services")
	}
}

func TestBuildContainerArgsOnceMode(t *testing.T) {
	cfg := config{
		once:            true,
		enabledServices: []string{"gh"},
	}

	args := buildContainerArgs(cfg, "", "/pwd", "test-container", "not_found")

	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "--once") {
		t.Error("expected --once in session args")
	}
}

// TestBuildContainerArgsWorkspaceMount locks in the same-path
// bind-mount + workdir that the kilo wrapper relies on to resolve
// `<cwd>/tmp` and locate the worktree-local .gitignore. If either
// disappears, the workspace-scoped tmpdir policy in
// scripts/kilo-wrapper-lib.sh silently degrades to /tmp.
func TestBuildContainerArgsWorkspaceMount(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{},
	}

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found")
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "-v /pwd:/pwd") {
		t.Error("expected -v /pwd:/pwd bind mount (same-path workspace mount)")
	}
	if !strings.Contains(argsStr, "-w /pwd") {
		t.Error("expected -w /pwd workdir")
	}
}

func TestBuildContainerArgsWithPorts(t *testing.T) {
	cfg := config{
		once:  false,
		ports: []string{"8080:80", "3000:3000"},
	}

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found")

	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "-p 8080:80") {
		t.Error("expected -p 8080:80 in docker args")
	}
	if !strings.Contains(argsStr, "-p 3000:3000") {
		t.Error("expected -p 3000:3000 in docker args")
	}
	if !strings.Contains(argsStr, "--port 8080:80") {
		t.Error("expected --port 8080:80 in session args label")
	}
	if !strings.Contains(argsStr, "--port 3000:3000") {
		t.Error("expected --port 3000:3000 in session args label")
	}
}

func TestBuildContainerArgsNoPorts(t *testing.T) {
	cfg := config{
		once: false,
	}

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found")

	argsStr := strings.Join(args, " ")

	if strings.Contains(argsStr, "-p ") {
		t.Error("expected no -p flag when no ports configured")
	}
	if strings.Contains(argsStr, "--port") {
		t.Error("expected no --port in session args when no ports configured")
	}
}

func TestSerializeArgsEmpty(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{},
	}
	result := serializeArgs(cfg)
	expected := "--network kilo-shared"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSerializeArgsNoneNetwork(t *testing.T) {
	cfg := config{
		networks: []string{"none"},
	}
	result := serializeArgs(cfg)
	expected := "--network none"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSerializeArgsContainerNetwork(t *testing.T) {
	cfg := config{
		networks: []string{"container:my-app"},
	}
	result := serializeArgs(cfg)
	expected := "--network container:my-app"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSerializeArgsOnce(t *testing.T) {
	cfg := config{
		once:            true,
		enabledServices: []string{},
	}
	result := serializeArgs(cfg)
	expected := "--once --network kilo-shared"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSerializeArgsServices(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{"docker", "gh"},
	}
	result := serializeArgs(cfg)
	if !strings.Contains(result, "--docker") {
		t.Errorf("expected '--docker' in result, got %q", result)
	}
	if !strings.Contains(result, "--gh") {
		t.Errorf("expected '--gh' in result, got %q", result)
	}
}

func TestSerializeArgsCombined(t *testing.T) {
	cfg := config{
		once:            true,
		enabledServices: []string{"docker"},
		playwright:      true,
	}
	result := serializeArgs(cfg)
	if !strings.Contains(result, "--once") {
		t.Errorf("expected '--once' in result, got %q", result)
	}
	if !strings.Contains(result, "--docker") {
		t.Errorf("expected '--docker' in result, got %q", result)
	}
	if !strings.Contains(result, "--playwright") {
		t.Errorf("expected '--playwright' in result, got %q", result)
	}
}

func TestSerializeArgsPorts(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{},
		ports:           []string{"8080:80", "3000:3000"},
	}
	result := serializeArgs(cfg)
	if !strings.Contains(result, "--port 8080:80") {
		t.Errorf("expected '--port 8080:80' in result, got %q", result)
	}
	if !strings.Contains(result, "--port 3000:3000") {
		t.Errorf("expected '--port 3000:3000' in result, got %q", result)
	}
}

func TestSerializeArgsNetwork(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{},
		networks:        []string{"my-network"},
	}
	result := serializeArgs(cfg)
	expected := "--network kilo-shared --network my-network"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSerializeArgsYesOnlyWhenExplicit(t *testing.T) {
	cfgWithoutYes := config{}
	resultWithoutYes := serializeArgs(cfgWithoutYes)
	if strings.Contains(resultWithoutYes, "--yes") {
		t.Errorf("expected no '--yes' when cfg.yes=false, got %q", resultWithoutYes)
	}

	cfgWithYes := config{yes: true}
	resultWithYes := serializeArgs(cfgWithYes)
	if !strings.Contains(resultWithYes, "--yes") {
		t.Errorf("expected '--yes' when cfg.yes=true, got %q", resultWithYes)
	}
}

func TestArgsMatchIdentical(t *testing.T) {
	if !argsMatch("--once", "--once") {
		t.Error("identical args should match")
	}
}

func TestArgsMatchNetworkNormalized(t *testing.T) {
	// --network kilo-shared is implicit and should be stripped before comparison
	if !argsMatch("--port 8080:80", "--network kilo-shared --port 8080:80") {
		t.Error("kilo-shared network should be normalized away")
	}
}

func TestArgsMatchDifferent(t *testing.T) {
	if argsMatch("--once", "--playwright") {
		t.Error("different args should not match")
	}
}

func TestArgsMatchPortReordering(t *testing.T) {
	if !argsMatch("--port 8080:80 --port 3000:3000", "--port 3000:3000 --port 8080:80") {
		t.Error("port reordering should match")
	}
}

func TestArgsMatchVolumeReordering(t *testing.T) {
	if !argsMatch("--volume /a:/a --volume /b:/b", "--volume /b:/b --volume /a:/a") {
		t.Error("volume reordering should match")
	}
}

func TestArgsMatchNetworkReordering(t *testing.T) {
	if !argsMatch("--network kilo-shared --network b --network a", "--network a --network b") {
		t.Error("network reordering should match after implicit network normalization")
	}
}

func TestArgsMatchServiceReordering(t *testing.T) {
	if !argsMatch("--docker --gh", "--gh --docker") {
		t.Error("service flag reordering should match")
	}
}

func TestArgsMatchDifferentRepeatableValues(t *testing.T) {
	if argsMatch("--port 8080:80 --port 3000:3000", "--port 8080:80 --port 4000:4000") {
		t.Error("different repeatable flag values should not match")
	}
}

func TestSerializeStoredArgsRoundTrip(t *testing.T) {
	stored := "--once"
	displayed := serializeStoredArgs(stored)
	if !strings.Contains(displayed, "--once") {
		t.Errorf("expected '--once' in displayed, got %q", displayed)
	}
}

func TestSerializeStoredArgsEmpty(t *testing.T) {
	if serializeStoredArgs("") != "" {
		t.Error("empty stored args should return empty")
	}
}

func TestSerializeStoredArgsNoneNetwork(t *testing.T) {
	stored := "--network none"
	displayed := serializeStoredArgs(stored)
	if !strings.Contains(displayed, "--network none") {
		t.Errorf("expected '--network none' in displayed, got %q", displayed)
	}
}

func TestSerializeStoredArgsContainerNetwork(t *testing.T) {
	stored := "--network container:my-app"
	displayed := serializeStoredArgs(stored)
	if !strings.Contains(displayed, "--network container:my-app") {
		t.Errorf("expected '--network container:my-app' in displayed, got %q", displayed)
	}
}

func TestArgsMatchNoneNetworkRoundTrip(t *testing.T) {
	if !argsMatch("--network none", "--network none") {
		t.Error("identical none network args should match")
	}
}

func TestArgsMatchContainerNetworkRoundTrip(t *testing.T) {
	if !argsMatch("--network container:app", "--network container:app") {
		t.Error("identical container network args should match")
	}
}

func TestArgsMatchSpecialVsRegular(t *testing.T) {
	if argsMatch("--network none", "--network kilo-shared") {
		t.Error("none and kilo-shared should not match")
	}
}

func TestArgsMatchHostVsNone(t *testing.T) {
	if argsMatch("--network host", "--network none") {
		t.Error("host and none should not match")
	}
}

func TestSerializeStoredArgsPorts(t *testing.T) {
	stored := "--port 8080:80 --port 3000:3000"
	displayed := serializeStoredArgs(stored)
	if !strings.Contains(displayed, "--port 8080:80") {
		t.Errorf("expected '--port 8080:80' in displayed, got %q", displayed)
	}
	if !strings.Contains(displayed, "--port 3000:3000") {
		t.Errorf("expected '--port 3000:3000' in displayed, got %q", displayed)
	}
}

func TestArgsMatchExitedContainerFlagMismatch(t *testing.T) {
	// Verify that a flag difference is detected by argsMatch (the SSH-specific
	// case was removed when --ssh was deleted; this guards the general behavior).
	current := serializeForDisplay(config{})
	stored := serializeArgs(config{once: true})
	if argsMatch(current, stored) {
		t.Error("args should not match when --once flag differs")
	}
}

func TestExtractHostPortStandard(t *testing.T) {
	if got := extractHostPort("8080:80"); got != "8080" {
		t.Errorf("extractHostPort(\"8080:80\") = %q, want %q", got, "8080")
	}
}

func TestExtractHostPortHostOnly(t *testing.T) {
	if got := extractHostPort("8080"); got != "8080" {
		t.Errorf("extractHostPort(\"8080\") = %q, want %q", got, "8080")
	}
}

func TestExtractHostPortWithProtocol(t *testing.T) {
	if got := extractHostPort("8080:80/udp"); got != "8080" {
		t.Errorf("extractHostPort(\"8080:80/udp\") = %q, want %q", got, "8080")
	}
}

func TestExtractHostPortRange(t *testing.T) {
	if got := extractHostPort("8000-8010:80-90"); got != "8000-8010" {
		t.Errorf("extractHostPort(\"8000-8010:80-90\") = %q, want %q", got, "8000-8010")
	}
}

func TestExtractHostPortEmpty(t *testing.T) {
	if got := extractHostPort(""); got != "" {
		t.Errorf("extractHostPort(\"\") = %q, want %q", got, "")
	}
}

func TestExtractHostPortWithIP(t *testing.T) {
	if got := extractHostPort("127.0.0.1:8080:80"); got != "8080" {
		t.Errorf("extractHostPort(\"127.0.0.1:8080:80\") = %q, want %q", got, "8080")
	}
}

func TestExtractHostPortWithIPNoHostPort(t *testing.T) {
	if got := extractHostPort("127.0.0.1::80"); got != "" {
		t.Errorf("extractHostPort(\"127.0.0.1::80\") = %q, want %q", got, "")
	}
}

func TestExtractHostPortWithIPRange(t *testing.T) {
	if got := extractHostPort("127.0.0.1:8000-8010:80-90"); got != "8000-8010" {
		t.Errorf("extractHostPort(\"127.0.0.1:8000-8010:80-90\") = %q, want %q", got, "8000-8010")
	}
}

func TestCheckPortConflictsEmptyPorts(t *testing.T) {
	cfg := config{}
	if err := checkPortConflicts(cfg); err != nil {
		t.Errorf("expected no error for empty ports, got %v", err)
	}
}

func TestCheckPortConflictsNoRunningSessions(t *testing.T) {
	// This tests the early return when no sessions exist (no Docker needed
	// since cfg.ports is empty after the first check).
	cfg := config{}
	if err := checkPortConflicts(cfg); err != nil {
		t.Errorf("expected no error for empty config, got %v", err)
	}
}

func TestBuildContainerArgsSkipsHostnameForNone(t *testing.T) {
	cfg := config{
		networks: []string{"none"},
	}
	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found")
	argsStr := strings.Join(args, " ")
	if strings.Contains(argsStr, "--hostname") {
		t.Error("expected no --hostname flag when using none network mode")
	}
}

func TestBuildContainerArgsSkipsHostnameForContainer(t *testing.T) {
	cfg := config{
		networks: []string{"container:my-app"},
	}
	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found")
	argsStr := strings.Join(args, " ")
	if strings.Contains(argsStr, "--hostname") {
		t.Error("expected no --hostname flag when using container network mode")
	}
}

func TestBuildContainerArgsIncludesHostnameForRegular(t *testing.T) {
	cfg := config{
		networks: []string{"my-network"},
	}
	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found")
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "--hostname test-container") {
		t.Error("expected --hostname flag for regular network")
	}
}

// TestBuildContainerArgsIdentityEnvVars locks in the host-identity env vars
// that agents use to disambiguate host from container. KD_HOST_USER must
// alias KD_USERNAME for backward compatibility. KD_WORKSPACE is propagated
// from the host so the entrypoint does not have to derive it.
func TestBuildContainerArgsIdentityEnvVars(t *testing.T) {
	cfg := config{}

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found")
	argsStr := strings.Join(args, " ")

	mustContain := []string{
		"-e KD_IS_KILO_DOCKER=1",
		"-e KD_WORKSPACE=/pwd",
		"-e KD_HOST_UID=",
		"-e KD_HOST_GID=",
		"-e KD_USERNAME=",
	}
	for _, want := range mustContain {
		if !strings.Contains(argsStr, want) {
			t.Errorf("expected %q in args, got:\n%s", want, argsStr)
		}
	}

	if !strings.Contains(argsStr, "-e KD_HOST_USER=") {
		t.Errorf("expected -e KD_HOST_USER=... in args, got:\n%s", argsStr)
	}

	// user.Current() may return empty HomeDir in some sandboxed test
	// environments, so we don't assert KD_HOST_HOME is always present.
	// Instead, ensure that if KD_HOST_HOME appears, it is preceded by -e.
	if strings.Contains(argsStr, "KD_HOST_HOME=") && !strings.Contains(argsStr, "-e KD_HOST_HOME=") {
		t.Errorf("KD_HOST_HOME present but not as -e flag, got:\n%s", argsStr)
	}
}

// useHostTimezoneTree redirects the host timezone sources to a temporary
// tree and clears TZ so filesystem-based detection is deterministic. It
// returns the temporary zoneinfo root.
func useHostTimezoneTree(t *testing.T) string {
	t.Helper()
	t.Setenv("TZ", "")

	dir := t.TempDir()
	root := filepath.Join(dir, "zoneinfo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create zoneinfo root: %v", err)
	}

	prevLocaltime, prevTimezone, prevRoot := hostLocaltimePath, hostTimezonePath, hostZoneinfoRoot
	hostLocaltimePath = filepath.Join(dir, "localtime")
	hostTimezonePath = filepath.Join(dir, "timezone")
	hostZoneinfoRoot = root
	t.Cleanup(func() {
		hostLocaltimePath, hostTimezonePath, hostZoneinfoRoot = prevLocaltime, prevTimezone, prevRoot
	})
	return root
}

func writeZoneFile(t *testing.T, root, zone string) string {
	t.Helper()
	path := filepath.Join(root, zone)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create zone dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("TZif"), 0o644); err != nil {
		t.Fatalf("write zone file: %v", err)
	}
	return path
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func symlinkZone(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink %s -> %s: %v", link, target, err)
	}
}

// TestResolveHostTimezonePrefersEnv locks in the explicit user override.
func TestResolveHostTimezonePrefersEnv(t *testing.T) {
	useHostTimezoneTree(t)
	t.Setenv("TZ", "Pacific/Auckland")

	if got := resolveHostTimezone(); got != "Pacific/Auckland" {
		t.Fatalf("resolveHostTimezone() = %q, want %q", got, "Pacific/Auckland")
	}
}

// TestResolveHostTimezoneRegionPreserved is the regression guard for the
// old filepath.Base behaviour, which turned "/usr/share/zoneinfo/Europe/Berlin"
// into the invalid zone "Berlin".
func TestResolveHostTimezoneRegionPreserved(t *testing.T) {
	root := useHostTimezoneTree(t)
	zoneFile := writeZoneFile(t, root, "Europe/Berlin")
	symlinkZone(t, zoneFile, hostLocaltimePath)

	if got := resolveHostTimezone(); got != "Europe/Berlin" {
		t.Fatalf("resolveHostTimezone() = %q, want %q (region must not be stripped)", got, "Europe/Berlin")
	}
}

// TestResolveHostTimezoneNestedRegion covers multi-level zone names.
func TestResolveHostTimezoneNestedRegion(t *testing.T) {
	root := useHostTimezoneTree(t)
	zoneFile := writeZoneFile(t, root, "America/Argentina/Buenos_Aires")
	symlinkZone(t, zoneFile, hostLocaltimePath)

	const want = "America/Argentina/Buenos_Aires"
	if got := resolveHostTimezone(); got != want {
		t.Fatalf("resolveHostTimezone() = %q, want %q", got, want)
	}
}

// TestResolveHostTimezoneRelativeSymlink covers hosts whose /etc/localtime
// uses a relative symlink target.
func TestResolveHostTimezoneRelativeSymlink(t *testing.T) {
	root := useHostTimezoneTree(t)
	writeZoneFile(t, root, "Etc/UTC")
	symlinkZone(t, filepath.Join("zoneinfo", "Etc", "UTC"), hostLocaltimePath)

	if got := resolveHostTimezone(); got != "Etc/UTC" {
		t.Fatalf("resolveHostTimezone() = %q, want %q", got, "Etc/UTC")
	}
}

// TestResolveHostTimezoneLegacyEtcTimezone covers older Debian/Ubuntu hosts
// where /etc/localtime is a regular file and /etc/timezone carries the zone.
func TestResolveHostTimezoneLegacyEtcTimezone(t *testing.T) {
	root := useHostTimezoneTree(t)
	writeZoneFile(t, root, "Europe/Paris")
	writeFile(t, hostLocaltimePath, "regular file, not a symlink")
	writeFile(t, hostTimezonePath, "Europe/Paris\n")

	if got := resolveHostTimezone(); got != "Europe/Paris" {
		t.Fatalf("resolveHostTimezone() = %q, want %q", got, "Europe/Paris")
	}
}

// TestResolveHostTimezonePrefersLocaltimeOverStaleTimezone reproduces the
// Ubuntu 24.04+ case where systemd stops updating /etc/timezone and it goes
// stale, while /etc/localtime stays correct.
func TestResolveHostTimezonePrefersLocaltimeOverStaleTimezone(t *testing.T) {
	root := useHostTimezoneTree(t)
	current := writeZoneFile(t, root, "America/New_York")
	writeZoneFile(t, root, "Europe/Berlin")
	symlinkZone(t, current, hostLocaltimePath)
	writeFile(t, hostTimezonePath, "Europe/Berlin\n")

	if got := resolveHostTimezone(); got != "America/New_York" {
		t.Fatalf("resolveHostTimezone() = %q, want %q", got, "America/New_York")
	}
}

// TestResolveHostTimezoneDanglingLocaltimeFallsBack covers a broken
// /etc/localtime symlink that must yield to the legacy source.
func TestResolveHostTimezoneDanglingLocaltimeFallsBack(t *testing.T) {
	root := useHostTimezoneTree(t)
	writeZoneFile(t, root, "Asia/Tokyo")
	symlinkZone(t, filepath.Join(root, "Does", "Not", "Exist"), hostLocaltimePath)
	writeFile(t, hostTimezonePath, "Asia/Tokyo")

	if got := resolveHostTimezone(); got != "Asia/Tokyo" {
		t.Fatalf("resolveHostTimezone() = %q, want %q", got, "Asia/Tokyo")
	}
}

// TestResolveHostTimezoneNoSources verifies that missing sources produce no
// TZ override rather than an empty or bogus value.
func TestResolveHostTimezoneNoSources(t *testing.T) {
	useHostTimezoneTree(t)

	if got := resolveHostTimezone(); got != "" {
		t.Fatalf("resolveHostTimezone() = %q, want empty", got)
	}
}

// TestResolveHostTimezoneRejectsInvalidLegacyValue ensures a malformed
// /etc/timezone entry is not propagated into the container.
func TestResolveHostTimezoneRejectsInvalidLegacyValue(t *testing.T) {
	useHostTimezoneTree(t)
	writeFile(t, hostLocaltimePath, "regular file, not a symlink")
	writeFile(t, hostTimezonePath, "Not/AZone\n")

	if got := resolveHostTimezone(); got != "" {
		t.Fatalf("resolveHostTimezone() = %q, want empty", got)
	}
}

// TestZoneFromZoneinfoPathOutsideTree rejects paths that are not within a
// zoneinfo tree, such as a copied /etc/localtime file.
func TestZoneFromZoneinfoPathOutsideTree(t *testing.T) {
	useHostTimezoneTree(t)

	if got := zoneFromZoneinfoPath("/etc/localtime"); got != "" {
		t.Fatalf("zoneFromZoneinfoPath(/etc/localtime) = %q, want empty", got)
	}
}
