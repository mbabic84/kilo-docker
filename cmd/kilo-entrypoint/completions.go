package main

import (
	"fmt"
	"os"
)

// handleCompletions outputs shell completion scripts.
func handleCompletions(args []string) {
	if len(args) == 0 {
		printCommandHelp("completions")
		return
	}

	switch args[0] {
	case "bash":
		fmt.Print(bashCompletionScript)
	case "zsh":
		fmt.Print(zshCompletionScript)
	case "fish":
		fmt.Print(fishCompletionScript)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported shell: %s (supported: bash, zsh, fish)\n", args[0])
		os.Exit(1)
	}
}

const bashCompletionScript = `_kilo_entrypoint_completions() {
    local cur prev commands sync_subcommands custom_envs_subcommands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="update-config backup restore mcp-config mcp-tokens sync resync zellij-attach print-env custom-envs help completions"
    sync_subcommands="ls rm"
    custom_envs_subcommands="list get add edit remove"

    # Top-level command completion (kilo-entrypoint <TAB>)
    if [ "${COMP_CWORD}" -eq 1 ]; then
        COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
        return 0
    fi

    case "${COMP_WORDS[1]}" in
        sync)
            if [ "${COMP_CWORD}" -eq 2 ]; then
                COMPREPLY=( $(compgen -W "$sync_subcommands" -- "$cur") )
                return 0
            fi
            ;;
        custom-envs)
            if [ "${COMP_CWORD}" -eq 2 ]; then
                COMPREPLY=( $(compgen -W "$custom_envs_subcommands" -- "$cur") )
                return 0
            fi
            if [ "${COMP_CWORD}" -eq 3 ]; then
                case "${COMP_WORDS[2]}" in
                    get|edit|remove)
                        local custom_env_keys
                        custom_env_keys=$(kilo-entrypoint custom-envs --complete 2>/dev/null)
                        COMPREPLY=( $(compgen -W "$custom_env_keys" -- "$cur") )
                        return 0
                        ;;
                esac
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
complete -F _kilo_entrypoint_completions kilo-entrypoint
`

const zshCompletionScript = `#compdef kilo-entrypoint

_kilo_entrypoint() {
    local -a commands
    commands=(
        'update-config:Download config template, merge with existing config'
        'backup:Create tar.gz of KILO_HOME'
        'restore:Extract tar.gz into KILO_HOME'
        'mcp-config:Apply MCP enabled states from encrypted token storage'
        'mcp-tokens:Interactive token management'
        'sync:Start ainstruct file watcher + REST sync'
        'resync:Delete all remote documents and re-push local files'
        'zellij-attach:Attach to existing zellij session'
        'print-env:Print export statements for tokens and custom envs'
        'custom-envs:Manage user-defined custom environment variables'
        'help:Show help message'
        'completions:Generate shell completion scripts'
    )

    local -a sync_subcommands
    sync_subcommands=(
        'ls:List all ainstruct sync files'
        'rm:Remove a specific sync file'
    )

    local -a custom_envs_subcommands
    custom_envs_subcommands=(
        'list:List all custom envs'
        'get:Print raw value of a custom env'
        'add:Add a new custom env'
        'edit:Edit an existing custom env'
        'remove:Remove a custom env'
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
                sync)
                    if (( ${#words[@]} == 2 )); then
                        _describe 'subcommand' sync_subcommands
                    fi
                    ;;
                custom-envs)
                    if (( ${#words[@]} == 2 )); then
                        _describe 'subcommand' custom_envs_subcommands
                    elif (( ${#words[@]} == 3 )); then
                        case ${words[2]} in
                            get|edit|remove)
                                local -a custom_env_keys
                                custom_env_keys=(${(f)"$(kilo-entrypoint custom-envs --complete 2>/dev/null)"})
                                _describe 'key' custom_env_keys
                                ;;
                        esac
                    fi
                    ;;
                completions)
                    _values 'shell' bash zsh fish
                    ;;
            esac
            ;;
    esac
}

_kilo_entrypoint "$@"
`

const fishCompletionScript = `# Completions for kilo-entrypoint
complete -c kilo-entrypoint -f

# Top-level subcommands
complete -c kilo-entrypoint -n '__fish_use_subcommand' -a update-config -d 'Download config template, merge'
complete -c kilo-entrypoint -n '__fish_use_subcommand' -a backup -d 'Create tar.gz of KILO_HOME'
complete -c kilo-entrypoint -n '__fish_use_subcommand' -a restore -d 'Extract tar.gz into KILO_HOME'
complete -c kilo-entrypoint -n '__fish_use_subcommand' -a mcp-config -d 'Apply MCP enabled states'
complete -c kilo-entrypoint -n '__fish_use_subcommand' -a mcp-tokens -d 'Interactive token management'
complete -c kilo-entrypoint -n '__fish_use_subcommand' -a sync -d 'Start ainstruct file watcher + REST sync'
complete -c kilo-entrypoint -n '__fish_use_subcommand' -a resync -d 'Delete and re-push all remote documents'
complete -c kilo-entrypoint -n '__fish_use_subcommand' -a zellij-attach -d 'Attach to existing zellij session'
complete -c kilo-entrypoint -n '__fish_use_subcommand' -a print-env -d 'Print export statements for tokens'
complete -c kilo-entrypoint -n '__fish_use_subcommand' -a custom-envs -d 'Manage custom environment variables'
complete -c kilo-entrypoint -n '__fish_use_subcommand' -a help -d 'Show help'
complete -c kilo-entrypoint -n '__fish_use_subcommand' -a completions -d 'Generate completions'

# Sync sub-subcommands
complete -c kilo-entrypoint -n '__fish_seen_subcommand_from sync' -a ls -d 'List all ainstruct sync files'
complete -c kilo-entrypoint -n '__fish_seen_subcommand_from sync' -a rm -d 'Remove a specific sync file'

# Custom-envs sub-subcommands
complete -c kilo-entrypoint -n '__fish_seen_subcommand_from custom-envs' -a list -d 'List all custom envs'
complete -c kilo-entrypoint -n '__fish_seen_subcommand_from custom-envs' -a get -d 'Print raw value of a custom env'
complete -c kilo-entrypoint -n '__fish_seen_subcommand_from custom-envs' -a add -d 'Add a new custom env'
complete -c kilo-entrypoint -n '__fish_seen_subcommand_from custom-envs' -a edit -d 'Edit an existing custom env'
complete -c kilo-entrypoint -n '__fish_seen_subcommand_from custom-envs' -a remove -d 'Remove a custom env'

# Custom-envs key (dynamic completions for subcommands that take a key)
complete -c kilo-entrypoint -n '__fish_seen_subcommand_from custom-envs; and __fish_seen_subcommand_from get edit remove' -a '(kilo-entrypoint custom-envs --complete 2>/dev/null)'

# Completions subcommand
complete -c kilo-entrypoint -n '__fish_seen_subcommand_from completions' -a 'bash zsh fish'
`
