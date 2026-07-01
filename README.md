# hsync

**hsync** is a tool for managing [Headscale](https://github.com/juanfont/headscale) instances. It syncs Headscale node IP addresses to DNS — either [Cloudflare](https://www.cloudflare.com/) or a BIND-format zone file — and provides direct management of nodes, users, pre-auth keys, and subnet routes via the Headscale API.

## Features

- Sync Headscale node IPv4 (`A`) and/or IPv6 (`AAAA`) addresses to DNS
- **Two output backends** — sync to Cloudflare DNS or generate a BIND-format zone file (full zone with SOA/NS, or a records-only fragment)
- Create, update, and optionally prune records — only records carrying a managed tag are ever deleted (Cloudflare mode)
- **Multiple zones** — map different Headscale users to different zones and domains via a config file
- **Tag management** — stamp a configurable managed tag on every record; add arbitrary extra tags per run or per zone (Cloudflare mode)
- **DNS record naming** — uses the Headscale-configured name (`givenName`) by default; `--use-hostname` falls back to the machine hostname
- **BIND zone file** — one-shot `zonefile` command or continuous output via `sync`/`watch`/`serve`; optional post-write reload command (e.g. `rndc reload`)
- **Four run modes** — one-shot (`sync`), zone file generator (`zonefile`), polling daemon (`watch`), or HTTP daemon (`serve`)
- **Webhook receiver** — `serve` triggers an immediate sync on `POST /webhook`, with optional bearer-token authentication
- **Prometheus metrics** — `/metrics` endpoint with sync counters, durations, node counts, and an `hsync_up` gauge
- **Dry-run mode** — preview every planned change without writing anything
- **Retry with backoff** — Cloudflare API calls are retried on network errors, HTTP 429, and HTTP 5xx (3 attempts, 1 s / 2 s backoff)
- **Config file** — JSON or YAML; all flags can be set there; CLI flags always win
- **Compiled-in defaults** — embed URLs and API keys at build time with `-ldflags` for zero-config deployments
- Node filtering by online status (`--online-only`) and Headscale user (`--user`)
- **Node management** — rename nodes, set ACL tags directly via the Headscale API
- **User management** — list, create, delete, and rename Headscale users
- **Pre-auth key management** — list, create (reusable/ephemeral, with expiration), and expire pre-auth keys
- **Route management** — list, enable, disable, and delete subnet routes

## Installation

### Build from source

```sh
git clone https://github.com/buraglio/hsync
cd hsync
go build -o hsync .
```

Requires Go 1.22 or later. The only external dependency is `gopkg.in/yaml.v3` (YAML config support).

### Build with compiled-in defaults

Embed credentials and defaults so the binary works with no flags:

```sh
go build -ldflags="\
  -X main.defaultHeadscaleURL=https://hs.example.com \
  -X main.defaultHeadscaleAPIKey=hskey-api-XXXXXXXX \
  -X main.defaultCFAPIToken=CLOUDFLARE_TOKEN \
  -X main.defaultCFZoneID=ZONE_ID \
  -X main.defaultDomain=ts.example.com" \
  -o hsync .
```

At runtime, CLI flags override compiled defaults, which in turn override environment variables.

## Quick start

```sh
# List all nodes
hsync list \
  --headscale-url https://hs.example.com \
  --headscale-key hskey-api-XXXXXXXX

# Rename a node
hsync rename \
  --headscale-url https://hs.example.com \
  --headscale-key hskey-api-XXXXXXXX \
  --node old-name --new-name new-name

# Set ACL tags on a node
hsync node-tag \
  --headscale-url https://hs.example.com \
  --headscale-key hskey-api-XXXXXXXX \
  --node gateway --tag tag:prod --tag tag:web

# List users
hsync users list \
  --headscale-url https://hs.example.com \
  --headscale-key hskey-api-XXXXXXXX

# Create a pre-auth key (reusable, expires in 24h)
hsync preauthkey create \
  --headscale-url https://hs.example.com \
  --headscale-key hskey-api-XXXXXXXX \
  --user alice --reusable --expiration 24h

# List subnet routes
hsync routes list \
  --headscale-url https://hs.example.com \
  --headscale-key hskey-api-XXXXXXXX

# One-shot sync (AAAA records only, dry run first)
hsync sync \
  --headscale-url https://hs.example.com \
  --headscale-key hskey-api-XXXXXXXX \
  --cf-token CF_TOKEN \
  --cf-zone ZONE_ID \
  --domain ts.example.com \
  --dry-run

# Run for real
hsync sync \
  --headscale-url https://hs.example.com \
  --headscale-key hskey-api-XXXXXXXX \
  --cf-token CF_TOKEN \
  --cf-zone ZONE_ID \
  --domain ts.example.com

# Sync every 5 minutes
hsync watch --config /etc/hsync/config.json --interval 5m

# HTTP daemon: webhook-triggered + periodic fallback + metrics
hsync serve --config /etc/hsync/config.json --interval 5m --listen :8080

# Generate a BIND zone file (stdout)
hsync zonefile \
  --headscale-url https://hs.example.com \
  --headscale-key hskey-api-XXXXXXXX \
  --domain ts.example.com \
  --bind-ns ns1.ts.example.com.

# Write zone file and reload BIND
hsync zonefile \
  --headscale-url https://hs.example.com \
  --headscale-key hskey-api-XXXXXXXX \
  --domain ts.example.com \
  --bind-ns ns1.ts.example.com. \
  --bind-zone-file /etc/bind/ts.example.com.zone \
  --bind-reload-cmd "rndc reload ts.example.com"
```

## Commands

### `list` — inspect Headscale nodes

```
hsync list [flags]
```

Prints a table of all Headscale nodes with their IPv4, IPv6, user, and online status.

| Flag | Default | Description |
|---|---|---|
| `-4` | true | Show IPv4 addresses |
| `-6` | true | Show IPv6 addresses |
| `--online` | false | Only show currently-online nodes |
| `--user <name>` | — | Filter to a specific Headscale user |
| `--json` | false | Output as JSON array |

---

### `rename` — rename a node

```
hsync rename --node <current-name> --new-name <name>
```

Renames a Headscale node by its current `givenName` or hostname. The node is looked up by name and renamed via the Headscale API.

| Flag | Default | Description |
|---|---|---|
| `--node <name>` | — | Current node name (givenName or hostname) |
| `--new-name <name>` | — | New name to assign |
| `--json` | false | Output updated node as JSON |

```sh
hsync rename --node laptop-alice --new-name alices-laptop
```

---

### `node-tag` — set ACL tags on a node

```
hsync node-tag --node <name> [--tag tag:value ...] [--clear]
```

Replaces the ACL tag set on a Headscale node. Tags must follow the `tag:value` format required by Headscale ACL policies. Use `--clear` to remove all tags.

| Flag | Default | Description |
|---|---|---|
| `--node <name>` | — | Node name (givenName or hostname) |
| `--tag <tag:value>` | — | ACL tag to set (repeatable; replaces all existing tags) |
| `--clear` | false | Remove all ACL tags from the node |
| `--json` | false | Output updated node as JSON |

```sh
# Set tags
hsync node-tag --node gateway --tag tag:prod --tag tag:egress

# Remove all tags
hsync node-tag --node gateway --clear
```

---

### `users` — manage Headscale users

```
hsync users <list|create|delete|rename> [flags]
```

Full CRUD for Headscale users (namespaces).

#### `users list`

Prints all users with their ID and creation time.

```sh
hsync users list
hsync users list --json
```

#### `users create`

```sh
hsync users create --name alice
```

| Flag | Description |
|---|---|
| `--name <name>` | Username to create |

#### `users delete`

```sh
hsync users delete --name alice
```

| Flag | Description |
|---|---|
| `--name <name>` | Username to delete |

#### `users rename`

```sh
hsync users rename --name alice --new-name aliceb
```

| Flag | Description |
|---|---|
| `--name <name>` | Current username |
| `--new-name <name>` | New username |

---

### `preauthkey` — manage pre-authentication keys

```
hsync preauthkey <list|create|expire> [flags]
```

Manage Headscale pre-auth keys used for node registration.

#### `preauthkey list`

```sh
hsync preauthkey list
hsync preauthkey list --user alice
hsync preauthkey list --json
```

| Flag | Description |
|---|---|
| `--user <name>` | Filter keys by username (optional) |

Output columns: ID, USER, KEY, REUSABLE, EPHEMERAL, USED, EXPIRATION

#### `preauthkey create`

```sh
# Single-use key for alice, expires in 24 hours
hsync preauthkey create --user alice --expiration 24h

# Reusable ephemeral key (nodes auto-deleted when offline)
hsync preauthkey create --user alice --reusable --ephemeral
```

| Flag | Default | Description |
|---|---|---|
| `--user <name>` | — | Headscale user to create the key for |
| `--reusable` | false | Allow the key to be used multiple times |
| `--ephemeral` | false | Nodes registered with this key are ephemeral |
| `--expiration <duration>` | 0 (server default) | Key lifetime (e.g. `24h`, `720h`) |

#### `preauthkey expire`

Immediately invalidates a key so it can no longer be used for registration.

```sh
hsync preauthkey expire --user alice --key <full-key-value>
```

| Flag | Description |
|---|---|
| `--user <name>` | Username that owns the key |
| `--key <key>` | Full key value to expire |

---

### `routes` — manage subnet routes

```
hsync routes <list|enable|disable|delete> [flags]
```

Manage advertised subnet routes from Headscale subnet routers.

#### `routes list`

```sh
hsync routes list
hsync routes list --node gateway
hsync routes list --json
```

| Flag | Description |
|---|---|
| `--node <name>` | Filter routes to a specific node (optional) |

Output columns: ID, NODE, PREFIX, ADVERTISED, ENABLED, PRIMARY

#### `routes enable`

```sh
hsync routes enable --route-id 5
```

| Flag | Description |
|---|---|
| `--route-id <id>` | Route ID to enable |

#### `routes disable`

```sh
hsync routes disable --route-id 5
```

| Flag | Description |
|---|---|
| `--route-id <id>` | Route ID to disable |

#### `routes delete`

```sh
hsync routes delete --route-id 5
```

| Flag | Description |
|---|---|
| `--route-id <id>` | Route ID to delete |

---

### `sync` — one-shot sync

```
hsync sync [flags]
```

Fetches all Headscale nodes and syncs them to Cloudflare DNS. Exits when done.

| Flag | Default | Description |
|---|---|---|
| `--domain <suffix>` | `ts.example.com` | Domain suffix for records (node `foo` → `foo.ts.example.com`) |
| `--ipv6` | true | Sync `AAAA` records |
| `--ipv4` | false | Sync `A` records |
| `--ttl <seconds>` | 60 | DNS record TTL |
| `--proxied` | false | Proxy records through Cloudflare CDN |
| `--prune` | false | Delete managed records not present in Headscale |
| `--dry-run` | false | Print planned changes; make no API calls |
| `--online-only` | false | Skip nodes where Headscale reports `online: false` |
| `--user <name>` | — | Only sync nodes owned by this Headscale user |
| `--managed-tag <tag>` | `managed:hsync` | Tag stamped on every managed record; prune filter |
| `--tag <key:value>` | — | Extra tag (repeatable: `--tag env:prod --tag region:us`) |
| `--disable-tags` | false | Omit tags from API calls (required for free/non-Enterprise Cloudflare zones) |
| `--comment <text>` | `Managed by hsync` | Comment written to each DNS record |
| `--use-hostname` | false | Use the machine hostname instead of the Headscale-configured name for DNS records |
| `--config <path>` | — | JSON or YAML config file |
| `--bind-zone-file <path>` | — | Write BIND zone file here instead of syncing to Cloudflare; stdout if omitted |
| `--bind-zone-dir <dir>` | — | Write one BIND zone file per zone to this directory (`<dir>/<domain>.zone`) |
| `--bind-ns <name>` | — | NS record for generated zone (repeatable) |
| `--bind-soa-email <email>` | — | SOA RNAME (default: `hostmaster.<domain>.`) |
| `--bind-reload-cmd <cmd>` | — | Shell command run after each zone file write |
| `--bind-fragment` | false | Write only A/AAAA records — no SOA/NS header |

---

### `zonefile` — BIND zone file generator

```
hsync zonefile [flags]
```

Fetches all Headscale nodes and writes a BIND-format zone file. Cloudflare credentials are not required. Default output is stdout, making it easy to pipe or redirect.

Accepts the same flags as `sync` (for node filtering, TTL, `--use-hostname`, etc.) plus the `--bind-*` flags above.

```sh
# Print zone to stdout
hsync zonefile --domain ts.example.com --bind-ns ns1.ts.example.com.

# Write to file and reload BIND
hsync zonefile \
  --domain ts.example.com \
  --bind-ns ns1.ts.example.com. \
  --bind-zone-file /etc/bind/ts.example.com.zone \
  --bind-reload-cmd "rndc reload ts.example.com"

# Records-only fragment (no SOA/NS), for $INCLUDE in an existing zone
hsync zonefile --domain ts.example.com --bind-fragment > /etc/bind/headscale-nodes.inc
```

**Zone file format:**

```
; Generated by hsync 0.2.0 — 2026-06-30T12:00:00Z
$ORIGIN ts.example.com.
$TTL 60

@  IN SOA  ns1.ts.example.com. hostmaster.ts.example.com. (
               1751313600 ; serial (Unix timestamp)
               3600       ; refresh
               900        ; retry
               604800     ; expire
               60         ; minimum TTL
           )

@  IN NS   ns1.ts.example.com.

; Node records
foo  IN AAAA  fd7a:115c:a1e0::1
bar  IN AAAA  fd7a:115c:a1e0::2
```

**Multi-zone:** when a `zones` config file is used, each zone writes to `<bind-zone-dir>/<domain>.zone`. The `--bind-zone-file` flag is ignored in multi-zone mode (ambiguous); use `--bind-zone-dir` instead.

---

### `watch` — polling daemon

```
hsync watch [flags]
```

Runs `sync` immediately on startup, then repeats at `--interval`. Accepts the same flags as `sync` plus:

| Flag | Default | Description |
|---|---|---|
| `--interval <duration>` | `5m` | How often to sync (e.g. `30s`, `5m`, `1h`) |

---

### `serve` — HTTP daemon

```
hsync serve [flags]
```

Starts an HTTP server and accepts all `sync` flags plus:

| Flag | Default | Description |
|---|---|---|
| `--listen <addr>` | `:8080` | Listen address |
| `--webhook-secret <s>` | — | Require `Authorization: Bearer <s>` on `/webhook` |
| `--interval <duration>` | 0 (off) | Optional periodic sync alongside webhook triggers |

**Endpoints:**

| Path | Method | Description |
|---|---|---|
| `/webhook` | `POST` | Queue an immediate sync |
| `/metrics` | `GET` | Prometheus text-format metrics |
| `/healthz` | `GET` | Liveness probe — always `200 OK` |
| `/status` | `GET` | JSON snapshot of the last sync result |

A sync triggered by multiple rapid webhooks is deduplicated — only one sync runs per batch.

**Graceful shutdown:** `SIGINT` / `SIGTERM` drains in-flight requests (10 s timeout) before exiting.

---

### `version`

```
hsync version
```

Prints the version string.

---

## Configuration

### Environment variables

All commands read these if the corresponding flag is not provided:

| Variable | Flag equivalent |
|---|---|
| `HEADSCALE_URL` | `--headscale-url` |
| `HEADSCALE_API_KEY` | `--headscale-key` |
| `CLOUDFLARE_API_TOKEN` | `--cf-token` |
| `CLOUDFLARE_ZONE_ID` | `--cf-zone` |
| `DOMAIN` | `--domain` |

### Config file

Pass `--config /path/to/config.json` (or `.yaml` / `.yml`). File values fill in any flag that was not explicitly provided on the command line.

**Precedence (highest → lowest):**
1. CLI flags
2. Config file
3. Environment variables
4. Compiled-in defaults

#### Single-zone JSON example

```json
{
  "headscale_url": "https://hs.example.com",
  "headscale_api_key": "hskey-api-XXXXXXXX",
  "cf_api_token": "CLOUDFLARE_TOKEN",
  "cf_zone_id": "ZONE_ID",
  "domain": "ts.example.com",
  "ttl": 60,
  "prune": true,
  "sync_ipv4": true,
  "managed_tag": "managed:hsync",
  "tags": ["env:prod"],
  "comment": "Managed by hsync"
}
```

#### Single-zone YAML example

```yaml
headscale_url: https://hs.example.com
headscale_api_key: hskey-api-XXXXXXXX
cf_api_token: CLOUDFLARE_TOKEN
cf_zone_id: ZONE_ID
domain: ts.example.com
ttl: 60
prune: true
sync_ipv4: true
managed_tag: managed:hsync
tags:
  - env:prod
comment: Managed by hsync
```

#### Multi-zone example

When `zones` is present, all zone-level fields (`cf_api_token`, `cf_zone_id`, `domain`) are ignored at the top level — each zone supplies its own.

```json
{
  "headscale_url": "https://hs.example.com",
  "headscale_api_key": "hskey-api-XXXXXXXX",
  "ttl": 60,
  "prune": true,
  "managed_tag": "managed:hsync",
  "tags": ["env:prod"],
  "zones": [
    {
      "cf_api_token": "CF_TOKEN_A",
      "cf_zone_id": "ZONE_ID_A",
      "domain": "ts.example.com"
    },
    {
      "cf_api_token": "CF_TOKEN_B",
      "cf_zone_id": "ZONE_ID_B",
      "domain": "ops.ts.example.com",
      "users": ["alice", "bob"],
      "tags": ["team:ops"]
    }
  ]
}
```

In this example:
- All nodes are synced to `ts.example.com` via Zone A
- Only nodes owned by `alice` or `bob` are also synced to `ops.ts.example.com` via Zone B, and those records additionally get the `team:ops` tag

---

## DNS Record Naming

By default hsync uses the **Headscale-configured name** (`givenName`) for each node — the name an admin has set via `headscale nodes rename` or `hsync rename`. If no configured name is set, it falls back to the machine hostname (`name`).

To always use the machine hostname instead:

```sh
hsync sync --use-hostname ...
```

Or in a config file:

```yaml
use_hostname: true
```

This applies to both Cloudflare sync and BIND zone file output.

---

## Tags

> **Cloudflare plan requirement:** DNS record tags are only available on paid Cloudflare plans (Business/Enterprise). Free zone users must set `--disable-tags` (or `disable_tags: true` in the config file) to avoid API error 9300. When tags are disabled, `--prune` still works for records that already carry a managed tag from a prior run, but newly created records will not be tag-tracked.

Tags are key:value strings stored on Cloudflare DNS records. hsync uses tags for two purposes:

### Managed-tag (safe pruning)

Every record hsync creates or updates is stamped with `--managed-tag` (default `managed:hsync`). When `--prune` is enabled, **only records carrying this tag are eligible for deletion**. Records in the same domain that were created manually are left untouched.

To use a custom tag (e.g. if you run multiple hsync instances):

```sh
hsync sync --managed-tag managed:hsync-prod ...
```

### User-defined tags

Add arbitrary extra tags with `--tag` (repeatable):

```sh
hsync sync --tag env:prod --tag region:us-east-1 ...
```

Or in a config file:

```yaml
tags:
  - env:prod
  - region:us-east-1
```

Per-zone tags are merged with global tags:

```json
{
  "tags": ["env:prod"],
  "zones": [
    {
      "domain": "ts.example.com",
      "tags": ["tier:public"]
    }
  ]
}
```

Records in `ts.example.com` will carry `managed:hsync`, `env:prod`, and `tier:public`.

### Tag change detection

On every sync, hsync compares the existing record's tag set against what it would write. If the tags differ (e.g. you added a new tag to your config), the record is updated in place even if the IP address has not changed.

### Removing tags

Because hsync always **replaces** the full tag set on update, removing a tag is as simple as dropping it from your config or flags. On the next sync, the record is patched with the new (smaller) tag list.

---

## Prometheus metrics

When running `hsync serve` or `hsync watch`, metrics are available at `GET /metrics` (serve mode only) or tracked internally.

```
# HELP hsync_up 1 if the last sync succeeded, 0 if it errored, -1 if never run
hsync_up 1

# HELP hsync_syncs_total Total sync operations by status
hsync_syncs_total{status="success"} 42
hsync_syncs_total{status="error"} 1

# HELP hsync_dns_operations_total Cumulative DNS record operations
hsync_dns_operations_total{op="create"} 15
hsync_dns_operations_total{op="update"} 7
hsync_dns_operations_total{op="delete"} 2
hsync_dns_operations_total{op="error"} 0

# HELP hsync_nodes_total Headscale nodes seen in the last sync
hsync_nodes_total 12

# HELP hsync_last_sync_duration_seconds Wall-clock duration of the last sync
hsync_last_sync_duration_seconds 0.412

# HELP hsync_last_sync_timestamp_seconds Unix timestamp of the last sync attempt
hsync_last_sync_timestamp_seconds 1.75112e+09
```

A Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: hsync
    static_configs:
      - targets: ["hsync-host:8080"]
```

---

## Running as a service

### systemd

```ini
# /etc/systemd/system/hsync.service
[Unit]
Description=Headscale → Cloudflare DNS sync
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/hsync serve \
    --config /etc/hsync/config.yaml \
    --listen :8080 \
    --interval 5m
Restart=on-failure
RestartSec=10s
# Keep credentials out of the unit file — use the config file or env vars
EnvironmentFile=-/etc/hsync/env

[Install]
WantedBy=multi-user.target
```

```sh
systemctl daemon-reload
systemctl enable --now hsync
```

### Docker

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY . .
RUN go build -o /hsync .

FROM alpine:3.20
COPY --from=builder /hsync /usr/local/bin/hsync
ENTRYPOINT ["hsync"]
CMD ["serve", "--config", "/etc/hsync/config.yaml", "--listen", ":8080"]
```

```sh
docker run -d \
  -v /etc/hsync:/etc/hsync:ro \
  -p 8080:8080 \
  hsync
```

### Headscale webhook integration

In your Headscale config, add:

```yaml
# headscale config.yaml
webhooks:
  - url: http://hsync-host:8080/webhook
```

Headscale will POST to `/webhook` whenever a node is registered, renamed, or deleted, triggering an immediate sync.

To require a shared secret:

```sh
# hsync serve side
hsync serve --webhook-secret my-secret-token ...

# Headscale side (if your version supports webhook headers)
# Set Authorization: Bearer my-secret-token in the webhook config
```

---

## Cloudflare API token permissions

Create a scoped token at **Cloudflare Dashboard → My Profile → API Tokens** with:

- **Zone — DNS — Edit** on the target zone(s)

The token does not need account-level permissions.

---

## Headscale API key

Generate a key in headscale:

```sh
headscale apikeys create --expiration 9999d
```

The key needs read access to nodes and write access for management commands (rename, tag, user, preauthkey, routes operations).

---

## Building with all options

```sh
# Development build
go build -o hsync .

# Production build with version and compiled-in credentials
go build \
  -ldflags="-s -w \
    -X main.defaultHeadscaleURL=https://hs.example.com \
    -X main.defaultHeadscaleAPIKey=hskey-api-XXXXXXXX \
    -X main.defaultCFAPIToken=CLOUDFLARE_TOKEN \
    -X main.defaultCFZoneID=ZONE_ID \
    -X main.defaultDomain=ts.example.com" \
  -o hsync .
```

> **Security note:** Credentials compiled in with `-ldflags` are visible in the binary via `strings`. For production deployments, prefer a config file with restricted permissions (`chmod 600 /etc/hsync/config.yaml`) or environment variables injected at runtime.

---

## Reference

### Global flags (all commands)

| Flag | Env var | Description |
|---|---|---|
| `--headscale-url` | `HEADSCALE_URL` | Headscale server base URL |
| `--headscale-key` | `HEADSCALE_API_KEY` | Headscale API key |
| `--cf-token` | `CLOUDFLARE_API_TOKEN` | Cloudflare API token |
| `--cf-zone` | `CLOUDFLARE_ZONE_ID` | Cloudflare zone ID (single-zone mode) |
| `--config` | — | Path to JSON or YAML config file |
| `-v` / `--verbose` | — | Enable `[DEBUG]` log output |
| `--json` | — | Machine-readable JSON output |

### Config file schema

| Key | Type | Description |
|---|---|---|
| `headscale_url` | string | Headscale server URL |
| `headscale_api_key` | string | Headscale API key |
| `cf_api_token` | string | Cloudflare API token (single-zone) |
| `cf_zone_id` | string | Cloudflare zone ID (single-zone) |
| `domain` | string | Domain suffix (single-zone) |
| `ttl` | int | DNS record TTL in seconds |
| `proxied` | bool | Proxy through Cloudflare CDN |
| `prune` | bool | Delete stale managed records |
| `dry_run` | bool | Preview only |
| `sync_ipv4` | bool | Sync A records |
| `sync_ipv6` | bool | Sync AAAA records (default true) |
| `online_only` | bool | Skip offline nodes |
| `managed_tag` | string | Tag identifying managed records |
| `tags` | []string | Global extra tags |
| `disable_tags` | bool | Omit tags from API calls (required for free/non-Enterprise zones) |
| `use_hostname` | bool | Use machine hostname instead of Headscale-configured name for DNS records |
| `comment` | string | Record comment text |
| `zones` | []ZoneTarget | Multi-zone config (overrides single-zone fields) |
| `bind_zone_file` | string | BIND zone output file path (single-zone; stdout if empty) |
| `bind_zone_dir` | string | Directory for per-zone BIND files (`<dir>/<domain>.zone`) |
| `bind_ns` | []string | NS records for generated zone |
| `bind_soa_email` | string | SOA RNAME (default: `hostmaster.<domain>.`) |
| `bind_reload_cmd` | string | Shell command run after each zone write |
| `bind_fragment` | bool | Write only A/AAAA records, no SOA/NS header |

**ZoneTarget fields:**

| Key | Type | Description |
|---|---|---|
| `cf_api_token` | string | Cloudflare API token for this zone |
| `cf_zone_id` | string | Cloudflare zone ID |
| `domain` | string | Domain suffix for this zone |
| `users` | []string | Headscale users to include (empty = all) |
| `tags` | []string | Zone-specific extra tags |

---

## License

MIT
