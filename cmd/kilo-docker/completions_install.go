package main

import (
	"os"
	"path/filepath"
	"strings"
)

const rcMarkerStart = "# kilo-docker completions start"
const rcMarkerEnd = "# kilo-docker completions end"

// installCompletions detects the user's shell and writes the completion
// script to the appropriate location. Returns a user-facing message about
// what was done, or "" if nothing was written.
func installCompletions() string {
	shell := detectShell()
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}

	switch {
	case strings.HasSuffix(shell, "bash"):
		return installBashCompletions(home)
	case strings.HasSuffix(shell, "zsh"):
		return installZshCompletions(home)
	case strings.HasSuffix(shell, "fish"):
		return installFishCompletions(home)
	default:
		return ""
	}
}

// detectShell returns the user's shell path from $SHELL.
func detectShell() string {
	return os.Getenv("SHELL")
}

// installBashCompletions writes the bash completion script and ensures
// ~/.bashrc sources it.
func installBashCompletions(home string) string {
	dir := filepath.Join(home, ".local", "share", "bash-completion", "completions")
	path := filepath.Join(dir, "kilo-docker")

	wrote := writeIfChanged(path, []byte(bashCompletionScript), 0o644)

	bashrc := filepath.Join(home, ".bashrc")
	sourceLine := "[ -f " + path + " ] && . " + path
	wrote = injectRcBlock(bashrc, sourceLine) || wrote

	if wrote {
		return "Shell completions installed for bash. Restart your shell or run:\n  source " + path
	}
	return ""
}

// installZshCompletions writes the zsh completion script and ensures
// ~/.zshrc has the necessary fpath configuration.
func installZshCompletions(home string) string {
	dir := filepath.Join(home, ".zsh", "completions")
	path := filepath.Join(dir, "_kilo-docker")

	wrote := writeIfChanged(path, []byte(zshCompletionScript), 0o644)

	zshrc := filepath.Join(home, ".zshrc")
	block := "fpath=(~/.zsh/completions $fpath)\nautoload -Uz compinit && compinit"
	wrote = injectRcBlock(zshrc, block) || wrote

	if wrote {
		return "Shell completions installed for zsh. Restart your shell or run:\n  source ~/.zshrc"
	}
	return ""
}

// installFishCompletions writes the fish completion script. Fish auto-loads
// completions from ~/.config/fish/completions/.
func installFishCompletions(home string) string {
	dir := filepath.Join(home, ".config", "fish", "completions")
	path := filepath.Join(dir, "kilo-docker.fish")

	if writeIfChanged(path, []byte(fishCompletionScript), 0o644) {
		return "Shell completions installed for fish."
	}
	return ""
}

// injectRcBlock ensures the given block exists in an rc file, wrapped in
// tagged markers. If the block exists, it is replaced. If markers are
// missing, the block is appended. Partial markers are left untouched.
// Returns true if the file was modified.
func injectRcBlock(rcPath, block string) bool {
	data, err := os.ReadFile(rcPath)
	if err != nil {
		return false
	}

	fullBlock := rcMarkerStart + "\n" + block + "\n" + rcMarkerEnd
	content := string(data)
	startIdx := strings.Index(content, rcMarkerStart)
	endIdx := strings.Index(content, rcMarkerEnd)

	switch {
	case startIdx != -1 && endIdx != -1 && endIdx > startIdx:
		old := content[startIdx : endIdx+len(rcMarkerEnd)]
		if old == fullBlock {
			return false
		}
		content = content[:startIdx] + fullBlock + content[endIdx+len(rcMarkerEnd):]
	case startIdx == -1 && endIdx == -1:
		content = content + "\n" + fullBlock + "\n"
	default:
		return false
	}

	return writeRcFile(rcPath, content)
}

// writeRcFile writes content to an rc file, preserving existing file permissions.
func writeRcFile(rcPath, content string) bool {
	perm := os.FileMode(0o644)
	if info, err := os.Stat(rcPath); err == nil {
		perm = info.Mode().Perm()
	}
	return os.WriteFile(rcPath, []byte(content), perm) == nil
}

// writeIfChanged writes data to path only if the content differs from what
// is already on disk. Creates parent directories as needed. Returns true
// if the file was written.
func writeIfChanged(path string, data []byte, perm os.FileMode) bool {
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == string(data) {
		return false
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}

	return os.WriteFile(path, data, perm) == nil
}
