package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
)

// Build-time defaults. Override at compile time:
//
//	go build -ldflags="\
//	  -X main.defaultHeadscaleURL=https://hs.example.com \
//	  -X main.defaultHeadscaleAPIKey=hskey-api-... \
//	  -X main.defaultCFAPIToken=... \
//	  -X main.defaultCFZoneID=... \
//	  -X main.defaultDomain=ts.example.com" .
var (
	defaultHeadscaleURL    = ""
	defaultHeadscaleAPIKey = ""
	defaultCFAPIToken      = ""
	defaultCFZoneID        = ""
	defaultDomain          = "ts.example.com"
)

const version = "0.2.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "list":
		runList(os.Args[2:])
	case "sync":
		runSync(os.Args[2:])
	case "watch":
		runWatch(os.Args[2:])
	case "serve":
		runServe(os.Args[2:])
	case "version", "--version", "-version":
		fmt.Printf("hsync %s\n", version)
	case "help", "--help", "-help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `hsync — Headscale → Cloudflare DNS sync tool

Usage:
  hsync <command> [flags]

Commands:
  list      List all Headscale nodes with their IP addresses
  sync      Sync Headscale nodes to Cloudflare DNS records (one-shot)
  zonefile  Generate a BIND-format zone file from Headscale nodes (one-shot)
  watch     Sync continuously on a repeating interval (no HTTP server)
  serve     HTTP daemon: POST /webhook triggers sync, GET /metrics /healthz /status
  version   Print version information

Run 'hsync <command> -help' for command-specific flags.

Environment variables (all commands):
  HEADSCALE_URL          Headscale server base URL
  HEADSCALE_API_KEY      Headscale API key
  CLOUDFLARE_API_TOKEN   Cloudflare API token
  CLOUDFLARE_ZONE_ID     Cloudflare zone ID
  DOMAIN                 Domain suffix for DNS records

Compiled-in defaults (set at build time):
  go build -ldflags="-X main.defaultHeadscaleURL=https://hs.example.com \
                      -X main.defaultHeadscaleAPIKey=hskey-api-... \
                      -X main.defaultCFAPIToken=... \
                      -X main.defaultCFZoneID=... \
                      -X main.defaultDomain=ts.example.com" .

Config file (JSON or YAML, pass with -config):
  {
    "headscale_url": "https://hs.example.com",
    "headscale_api_key": "hskey-api-...",
    "managed_tag": "managed:hsync",
    "tags": ["env:prod"],
    "zones": [
      {
        "cf_api_token": "...", "cf_zone_id": "...",
        "domain": "ts.example.com"
      },
      {
        "cf_api_token": "...", "cf_zone_id": "...",
        "domain": "ops.ts.example.com",
        "users": ["alice"],
        "tags": ["team:ops"]
      }
    ]
  }
`)
}

// ── list command ──────────────────────────────────────────────────────────────

func runList(args []string) {
	fs, cfg := newFlagSet("list")
	showV4 := fs.Bool("4", true, "Show IPv4 addresses")
	showV6 := fs.Bool("6", true, "Show IPv6 addresses")
	onlineOnly := fs.Bool("online", false, "Only show online nodes")
	user := fs.String("user", "", "Filter by Headscale user/namespace")
	parseAndMerge(fs, cfg, args)

	require(cfg.HeadscaleURL != "", "headscale-url is required (flag, env HEADSCALE_URL, or build-time default)")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required (flag, env HEADSCALE_API_KEY, or build-time default)")

	nodes, err := fetchHeadscaleNodes(cfg)
	must(err, "fetch nodes")

	nodes = filterNodes(nodes, *onlineOnly, *user)

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, nodes)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	hdr := "NAME\tUSER\tONLINE"
	if *showV4 {
		hdr += "\tIPV4"
	}
	if *showV6 {
		hdr += "\tIPV6"
	}
	fmt.Fprintln(w, hdr)

	for _, n := range nodes {
		v4, v6 := extractIPs(n.IPAddresses)
		online := "no"
		if n.Online {
			online = "yes"
		}
		name := n.Name
		if n.GivenName != "" && n.GivenName != n.Name {
			name = n.GivenName + " (" + n.Name + ")"
		}
		line := fmt.Sprintf("%s\t%s\t%s", name, n.User.Name, online)
		if *showV4 {
			line += "\t" + dash(v4)
		}
		if *showV6 {
			line += "\t" + dash(v6)
		}
		fmt.Fprintln(w, line)
	}
}

// ── shared helpers ────────────────────────────────────────────────────────────

func logInfo(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[INFO]  "+format+"\n", args...)
}

func logWarn(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[WARN]  "+format+"\n", args...)
}

func logDebug(cfg *Config, format string, args ...interface{}) {
	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

func require(cond bool, msg string) {
	if !cond {
		fmt.Fprintln(os.Stderr, "[ERROR] "+msg)
		os.Exit(1)
	}
}

func must(err error, context string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %s: %v\n", context, err)
		os.Exit(1)
	}
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func mustEncodeJSON(w io.Writer, v interface{}) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		must(err, "encode JSON")
	}
}

// newFlagSet creates a FlagSet with the common flags pre-registered.
func newFlagSet(name string) (*flag.FlagSet, *Config) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	cfg := &Config{}
	addCommonFlags(fs, cfg)
	return fs, cfg
}
