package main

import (
	"fmt"
	"os"
)

// handleCompletions outputs shell completion scripts or installs them.
func handleCompletions(cfg config) {
	if len(cfg.args) > 0 && cfg.args[0] == "--install" {
		msg := installCompletions()
		if msg != "" {
			fmt.Println(msg)
		}
		return
	}

	if cfg.help || len(cfg.args) == 0 {
		printCommandHelp("completions")
		return
	}

	switch cfg.args[0] {
	case "bash":
		fmt.Print(bashCompletionScript)
	case "zsh":
		fmt.Print(zshCompletionScript)
	case "fish":
		fmt.Print(fishCompletionScript)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported shell: %s (supported: bash, zsh, fish)\n", cfg.args[0])
		os.Exit(1)
	}
}

const bashCompletionScript = `_kilo_docker_completions() {
    local cur prev commands subcommands session_targets
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="sessions networks playwright profile backup restore init cleanup update install-dev version help completions"
    subcommands="cleanup recreate stop"
    session_targets=$(kilo-docker sessions --complete 2>/dev/null)

    # Top-level command completion (kilo-docker <TAB>)
    if [ "${COMP_CWORD}" -eq 1 ]; then
        COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
        return 0
    fi

    case "${COMP_WORDS[1]}" in
        sessions)
            # kilo-docker sessions <TAB> — show both subcommands and session targets
            if [ "${COMP_CWORD}" -eq 2 ]; then
                COMPREPLY=( $(compgen -W "$subcommands $session_targets" -- "$cur") )
                return 0
            fi
            # kilo-docker sessions <subcommand> <TAB> — show session targets
            if [ "${COMP_CWORD}" -eq 3 ]; then
                if [[ " $subcommands " == *" ${COMP_WORDS[2]} "* ]]; then
                    COMPREPLY=( $(compgen -W "$session_targets" -- "$cur") )
                    return 0
                fi
            fi
            ;;
        completions)
            if [ "${COMP_CWORD}" -eq 2 ]; then
                COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
                return 0
            fi
            ;;
    esac
    return 0
}
complete -F _kilo_docker_completions kilo-docker
`

const zshCompletionScript = `#compdef kilo-docker

_kilo_docker() {
    local -a commands
    commands=(
        'sessions:Manage sessions (list, attach, stop, cleanup, recreate)'
        'networks:List available Docker networks'
        'playwright:Recreate Playwright MCP container'
        'profile:Manage flag profiles'
        'backup:Create backup of volume'
        'restore:Restore volume from backup'
        'init:Reset configuration'
        'cleanup:Remove all artifacts'
        'update:Update binary and/or config'
        'install-dev:Install development binary'
        'version:Show versions'
        'help:Show help message'
        'completions:Generate shell completion scripts'
    )

    local -a session_subcommands
    session_subcommands=(
        'cleanup:Remove sessions'
        'recreate:Recreate session'
        'stop:Stop sessions'
    )

    _arguments -C \
        '1:command:->command' \
        '*::arg:->args'

    case $state in
        command)
            _describe 'command' commands
            ;;
        args)
            case ${words[1]} in
                sessions)
                    if (( ${#words[@]} == 2 )); then
                        local -a session_targets
                        session_targets=(${(f)"$(kilo-docker sessions --complete 2>/dev/null)"})
                        _describe 'subcommand' session_subcommands
                        _describe 'session' session_targets
                    else
                        local -a completions
                        completions=(${(f)"$(kilo-docker sessions --complete 2>/dev/null)"})
                        _describe 'session' completions
                    fi
                    ;;
                completions)
                    _values 'shell' bash zsh fish
                    ;;
            esac
            ;;
    esac
}

_kilo_docker "$@"
`

const fishCompletionScript = `# Completions for kilo-docker
complete -c kilo-docker -f

# Top-level subcommands
complete -c kilo-docker -n '__fish_use_subcommand' -a sessions -d 'Manage sessions'
complete -c kilo-docker -n '__fish_use_subcommand' -a networks -d 'List Docker networks'
complete -c kilo-docker -n '__fish_use_subcommand' -a playwright -d 'Recreate Playwright container'
complete -c kilo-docker -n '__fish_use_subcommand' -a profile -d 'Manage flag profiles'
complete -c kilo-docker -n '__fish_use_subcommand' -a backup -d 'Create volume backup'
complete -c kilo-docker -n '__fish_use_subcommand' -a restore -d 'Restore from backup'
complete -c kilo-docker -n '__fish_use_subcommand' -a init -d 'Reset configuration'
complete -c kilo-docker -n '__fish_use_subcommand' -a cleanup -d 'Remove all artifacts'
complete -c kilo-docker -n '__fish_use_subcommand' -a update -d 'Update binary/config'
complete -c kilo-docker -n '__fish_use_subcommand' -a install-dev -d 'Install dev binary'
complete -c kilo-docker -n '__fish_use_subcommand' -a version -d 'Show versions'
complete -c kilo-docker -n '__fish_use_subcommand' -a help -d 'Show help'
complete -c kilo-docker -n '__fish_use_subcommand' -a completions -d 'Generate completions'

# Session sub-subcommands
complete -c kilo-docker -n '__fish_seen_subcommand_from sessions' -a cleanup -d 'Remove sessions'
complete -c kilo-docker -n '__fish_seen_subcommand_from sessions' -a recreate -d 'Recreate session'
complete -c kilo-docker -n '__fish_seen_subcommand_from sessions' -a stop -d 'Stop sessions'

# Session target (dynamic completions)
complete -c kilo-docker -n '__fish_seen_subcommand_from sessions' -a '(kilo-docker sessions --complete 2>/dev/null)'

# Completions subcommand
complete -c kilo-docker -n '__fish_seen_subcommand_from completions' -a 'bash zsh fish'
`
