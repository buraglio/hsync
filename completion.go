package main

import (
	"fmt"
	"os"
)

func runCompletion(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hsync completion <bash|zsh|fish>")
		os.Exit(1)
	}
	switch args[0] {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		fmt.Fprintf(os.Stderr, "unknown shell: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Usage: hsync completion <bash|zsh|fish>")
		os.Exit(1)
	}
}

const bashCompletion = `# hsync bash completion
# Add to ~/.bashrc: eval "$(hsync completion bash)"

_hsync_completion() {
    local cur prev words cword
    if declare -f _init_completion > /dev/null 2>&1; then
        _init_completion || return
    else
        COMPREPLY=()
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    fi

    local top_cmds="list sync zonefile watch serve rename node-tag users preauthkey routes node apikey policy completion version help"

    if [[ $cword -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "${top_cmds}" -- "${cur}") )
        return 0
    fi

    case "${COMP_WORDS[1]}" in
        users)
            COMPREPLY=( $(compgen -W "list create delete rename" -- "${cur}") )
            ;;
        preauthkey)
            COMPREPLY=( $(compgen -W "list create expire" -- "${cur}") )
            ;;
        routes)
            COMPREPLY=( $(compgen -W "list enable disable delete" -- "${cur}") )
            ;;
        node)
            COMPREPLY=( $(compgen -W "show delete expire move" -- "${cur}") )
            ;;
        apikey)
            COMPREPLY=( $(compgen -W "list create expire" -- "${cur}") )
            ;;
        policy)
            COMPREPLY=( $(compgen -W "get set" -- "${cur}") )
            ;;
        completion)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- "${cur}") )
            ;;
        *)
            COMPREPLY=( $(compgen -W "${top_cmds}" -- "${cur}") )
            ;;
    esac

    return 0
}

complete -F _hsync_completion hsync
`

const zshCompletion = `# hsync zsh completion
# Add to ~/.zshrc: eval "$(hsync completion zsh)"

_hsync() {
    local state

    _arguments \
        '1: :->command' \
        '*: :->args'

    case $state in
        command)
            local commands=(
                'list:List all Headscale nodes with their IP addresses'
                'sync:Sync Headscale nodes to Cloudflare DNS records'
                'zonefile:Generate a BIND-format zone file from Headscale nodes'
                'watch:Sync continuously on a repeating interval'
                'serve:HTTP daemon with webhook, metrics, and health endpoints'
                'rename:Rename a Headscale node'
                'node-tag:Set ACL tags on a Headscale node'
                'users:Manage Headscale users'
                'preauthkey:Manage Headscale pre-auth keys'
                'routes:Manage Headscale subnet routes'
                'node:Manage Headscale nodes'
                'apikey:Manage Headscale API keys'
                'policy:Manage Headscale ACL policy'
                'completion:Generate shell completion scripts'
                'version:Print version information'
                'help:Show help'
            )
            _describe 'command' commands
            ;;
        args)
            case ${words[2]} in
                users)
                    local subcmds=('list:List users' 'create:Create a user' 'delete:Delete a user' 'rename:Rename a user')
                    _describe 'subcommand' subcmds
                    ;;
                preauthkey)
                    local subcmds=('list:List pre-auth keys' 'create:Create a pre-auth key' 'expire:Expire a pre-auth key')
                    _describe 'subcommand' subcmds
                    ;;
                routes)
                    local subcmds=('list:List routes' 'enable:Enable a route' 'disable:Disable a route' 'delete:Delete a route')
                    _describe 'subcommand' subcmds
                    ;;
                node)
                    local subcmds=('show:Show node details' 'delete:Delete a node' 'expire:Expire a node' 'move:Move a node to another user')
                    _describe 'subcommand' subcmds
                    ;;
                apikey)
                    local subcmds=('list:List API keys' 'create:Create an API key' 'expire:Expire an API key')
                    _describe 'subcommand' subcmds
                    ;;
                policy)
                    local subcmds=('get:Get the ACL policy' 'set:Set the ACL policy')
                    _describe 'subcommand' subcmds
                    ;;
                completion)
                    local subcmds=('bash:Generate bash completion script' 'zsh:Generate zsh completion script' 'fish:Generate fish completion script')
                    _describe 'subcommand' subcmds
                    ;;
            esac
            ;;
    esac
}

compdef _hsync hsync
`

const fishCompletion = `# hsync fish completion
# Add to ~/.config/fish/completions/hsync.fish: hsync completion fish > ~/.config/fish/completions/hsync.fish

set -l top_cmds list sync zonefile watch serve rename node-tag users preauthkey routes node apikey policy completion version help

# Disable file completions for hsync
complete -c hsync -f

# Top-level commands (only when no subcommand has been given yet)
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a list       -d "List all Headscale nodes with their IP addresses"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a sync       -d "Sync Headscale nodes to Cloudflare DNS records"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a zonefile   -d "Generate a BIND-format zone file from Headscale nodes"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a watch      -d "Sync continuously on a repeating interval"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a serve      -d "HTTP daemon with webhook, metrics, and health endpoints"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a rename     -d "Rename a Headscale node"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a node-tag   -d "Set ACL tags on a Headscale node"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a users      -d "Manage Headscale users"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a preauthkey -d "Manage Headscale pre-auth keys"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a routes     -d "Manage Headscale subnet routes"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a node       -d "Manage Headscale nodes"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a apikey     -d "Manage Headscale API keys"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a policy     -d "Manage Headscale ACL policy"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a completion -d "Generate shell completion scripts"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a version    -d "Print version information"
complete -c hsync -f -n "not __fish_seen_subcommand_from $top_cmds" -a help       -d "Show help"

# Sub-commands
complete -c hsync -f -n "__fish_seen_subcommand_from users"      -a list    -d "List users"
complete -c hsync -f -n "__fish_seen_subcommand_from users"      -a create  -d "Create a user"
complete -c hsync -f -n "__fish_seen_subcommand_from users"      -a delete  -d "Delete a user"
complete -c hsync -f -n "__fish_seen_subcommand_from users"      -a rename  -d "Rename a user"

complete -c hsync -f -n "__fish_seen_subcommand_from preauthkey" -a list    -d "List pre-auth keys"
complete -c hsync -f -n "__fish_seen_subcommand_from preauthkey" -a create  -d "Create a pre-auth key"
complete -c hsync -f -n "__fish_seen_subcommand_from preauthkey" -a expire  -d "Expire a pre-auth key"

complete -c hsync -f -n "__fish_seen_subcommand_from routes"     -a list    -d "List routes"
complete -c hsync -f -n "__fish_seen_subcommand_from routes"     -a enable  -d "Enable a route"
complete -c hsync -f -n "__fish_seen_subcommand_from routes"     -a disable -d "Disable a route"
complete -c hsync -f -n "__fish_seen_subcommand_from routes"     -a delete  -d "Delete a route"

complete -c hsync -f -n "__fish_seen_subcommand_from node"       -a show    -d "Show node details"
complete -c hsync -f -n "__fish_seen_subcommand_from node"       -a delete  -d "Delete a node"
complete -c hsync -f -n "__fish_seen_subcommand_from node"       -a expire  -d "Expire a node"
complete -c hsync -f -n "__fish_seen_subcommand_from node"       -a move    -d "Move a node to another user"

complete -c hsync -f -n "__fish_seen_subcommand_from apikey"     -a list    -d "List API keys"
complete -c hsync -f -n "__fish_seen_subcommand_from apikey"     -a create  -d "Create an API key"
complete -c hsync -f -n "__fish_seen_subcommand_from apikey"     -a expire  -d "Expire an API key"

complete -c hsync -f -n "__fish_seen_subcommand_from policy"     -a get     -d "Get the ACL policy"
complete -c hsync -f -n "__fish_seen_subcommand_from policy"     -a set     -d "Set the ACL policy"

complete -c hsync -f -n "__fish_seen_subcommand_from completion" -a bash    -d "Generate bash completion script"
complete -c hsync -f -n "__fish_seen_subcommand_from completion" -a zsh     -d "Generate zsh completion script"
complete -c hsync -f -n "__fish_seen_subcommand_from completion" -a fish    -d "Generate fish completion script"
`
