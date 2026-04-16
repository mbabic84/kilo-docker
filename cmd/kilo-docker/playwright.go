package main

import (
	"fmt"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// startPlaywright ensures the shared Playwright MCP container is running.
// It uses the shared network (kilo-shared) and shared volume (kilo-playwright-output).
// The container is reused if already running.
func startPlaywright() error {
	playwrightImage := "mcr.microsoft.com/playwright/mcp"

	outputDir := PlaywrightMountPath

	state := dockerState(SharedPlaywrightContainerName)
	switch state {
	case "running":
		utils.Log("[playwright] Reusing existing Playwright MCP container\n")
	case "exited", "dead", "created":
		utils.Log("[playwright] Starting stopped Playwright MCP container\n")
		_, _ = dockerRun("start", SharedPlaywrightContainerName)
	default:
		utils.Log("[playwright] Pulling Playwright MCP image...\n", utils.WithOutput())
		_, _ = dockerRun("pull", playwrightImage)

		_, _ = dockerRun("rm", "-f", SharedPlaywrightContainerName)

		// Run as default 'node' user (UID 1000) - Docker handles volume ownership
		_, err := dockerRunDetached("run", "-d", "--rm", "--init",
			"--name", SharedPlaywrightContainerName,
			"--network", SharedNetworkName,
			"-v", fmt.Sprintf("%s:%s", PlaywrightVolumeName, outputDir),
			playwrightImage,
			"--headless", "--browser", "chromium", "--no-sandbox",
			"--port", "8931", "--host", "0.0.0.0",
			"--output-dir", outputDir,
			"--allowed-hosts", "*",
		)
		if err != nil {
			return fmt.Errorf("failed to start Playwright container: %w", err)
		}

		utils.Log("[playwright] Waiting for Playwright MCP...\n", utils.WithOutput())
		for i := 1; i <= 30; i++ {
			state := dockerState(SharedPlaywrightContainerName)
			if state != "running" {
				utils.LogError("[playwright] container stopped.\n")
				_, _ = dockerRun("logs", SharedPlaywrightContainerName)
				return fmt.Errorf("playwright MCP container exited unexpectedly")
			}

			ready, _ := dockerExec(SharedPlaywrightContainerName, "", "node", "-e",
				"const net=require('net');const s=net.connect(8931,'127.0.0.1',()=>{s.destroy();process.exit(0)});s.on('error',()=>process.exit(1));s.setTimeout(2000,()=>{s.destroy();process.exit(1)})")
			if ready != "" || dockerState(SharedPlaywrightContainerName) == "running" {
				utils.Log("[playwright] ready.\n", utils.WithOutput())
				break
			}

			if i == 30 {
				utils.LogError("[playwright] timeout.\n")
				_, _ = dockerRun("logs", SharedPlaywrightContainerName)
				return fmt.Errorf("playwright MCP did not become ready in 30s")
			}
			time.Sleep(time.Second)
			utils.Log(".", utils.WithOutput())
		}
	}

	return nil
}

// cleanupPlaywright removes the Playwright MCP container (optional cleanup).
// Since it's shared, we don't remove it by default - it persists for reuse.
func cleanupPlaywright() {
	// Don't remove the shared container - it persists for reuse
	utils.Log("[playwright] Container %s kept running for future sessions\n", SharedPlaywrightContainerName, utils.WithOutput())
}
