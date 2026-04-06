package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// startPlaywright pulls and starts the Playwright MCP sidecar container.
// It creates a dedicated Docker network, waits up to 30 seconds for the
// browser automation service to accept TCP connections on port 8931,
// and logs any startup failures to stderr. The network pointer is updated
// with the created network name.
func startPlaywright(network *string) error {
	playwrightImage := "mcr.microsoft.com/playwright/mcp"
	playwrightContainer := "playwright-mcp"
	pwd, _ := os.Getwd()
	baseName := pwd[strings.LastIndex(pwd, "/")+1:]

	if *network != "" {
		return fmt.Errorf("--playwright and --network are mutually exclusive")
	}

	user, _ := exec.Command("whoami").Output()
	*network = fmt.Sprintf("kilo-playwright-%s", strings.TrimSpace(string(user)))

	if _, err := dockerRun("network", "inspect", *network); err != nil {
		_, _ = dockerRun("network", "create", *network)
	}

	fmt.Fprintf(os.Stderr, "Pulling Playwright MCP image...\n")
	_, _ = dockerRun("pull", playwrightImage)

	_ = os.MkdirAll(".playwright-mcp", 0755)

	_, _ = dockerRun("rm", "-f", playwrightContainer)

	outputDir := fmt.Sprintf("/mnt/%s/.playwright-mcp", baseName)
	_, err := dockerRunDetached("run", "-d", "--rm", "--init",
		"--name", playwrightContainer,
		"--network", *network,
		"--entrypoint", "node",
		"-v", fmt.Sprintf("%s:/mnt/%s", pwd, baseName),
		playwrightImage,
		"cli.js", "--headless", "--browser", "chromium", "--no-sandbox",
		"--port", "8931", "--host", "0.0.0.0",
		"--output-dir", outputDir,
		"--allowed-hosts", "*",
	)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Waiting for Playwright MCP...")
	for i := 1; i <= 30; i++ {
		state := dockerState(playwrightContainer)
		if state != "running" {
			fmt.Fprintf(os.Stderr, " container stopped.\n")
			_, _ = dockerRun("logs", playwrightContainer)
			_, _ = dockerRun("rm", "-f", playwrightContainer)
			_, _ = dockerRun("network", "rm", *network)
			return fmt.Errorf("playwright MCP container exited unexpectedly")
		}

		ready, _ := dockerExec(playwrightContainer, "", "node", "-e",
			"const net=require('net');const s=net.connect(8931,'127.0.0.1',()=>{s.destroy();process.exit(0)});s.on('error',()=>process.exit(1));s.setTimeout(2000,()=>{s.destroy();process.exit(1)})")
		if ready != "" || dockerState(playwrightContainer) == "running" {
			fmt.Fprintf(os.Stderr, " ready.\n")
			break
		}

		if i == 30 {
			fmt.Fprintf(os.Stderr, " timeout.\n")
			_, _ = dockerRun("logs", playwrightContainer)
			_, _ = dockerRun("rm", "-f", playwrightContainer)
			_, _ = dockerRun("network", "rm", *network)
			return fmt.Errorf("playwright MCP did not become ready in 30s")
		}
		time.Sleep(time.Second)
		fmt.Fprintf(os.Stderr, ".")
	}

	return nil
}

// cleanupPlaywright removes the Playwright MCP container and its Docker network.
func cleanupPlaywright(network string) {
	_, _ = dockerRun("rm", "-f", "playwright-mcp")
	_, _ = dockerRun("network", "rm", network)
}
