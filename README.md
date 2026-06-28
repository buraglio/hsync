# hsync

**hsync** is a tool for managing [Headscale](https://github.com/juanfont/headscale) instances. It syncs Headscale node IP addresses to [Cloudflare](https://www.cloudflare.com/) DNS records. It keeps `A` and `AAAA` records for your Headscale nodes up to date automatically, supports multiple Cloudflare zones, and can run as a one-shot command, a polling daemon, or a webhook-driven HTTP service.

## Features

- Sync Headscale node IPv4 (`A`) and/or IPv6 (`AAAA`) addresses to Cloudflare DNS
- Create, update, and optionally prune records — only records carrying a managed tag are ever deleted
- **Multiple Cloudflare zones** — map different Headscale users to different zones and domains via a config file
- **Tag management** — stamp a configurable managed tag on every record; add arbitrary extra tags per run or per zone
- **Three run modes** — one-shot (`sync`), polling daemon (`watch`), or HTTP daemon (`serve`)
- **Webhook receiver** — `serve` triggers an immediate sync on `POST /webhook`, with optional bearer-token authentication
- **Prometheus metrics** — `/metrics` endpoint with sync counters, durations, node counts, and an `hsync_up` gauge
- **Dry-run mode** — preview every planned change without touching Cloudflare
- **Retry with backoff** — Cloudflare API calls are retried on network errors, HTTP 429, and HTTP 5xx (3 attempts, 1 s / 2 s backoff)
- **Config file** — JSON or YAML; all flags can be set there; CLI flags always win
- **Compiled-in defaults** — embed URLs and API keys at build time with `-ldflags` for zero-config deployments
- Node filtering by online status (`--online-only`) and Headscale user (`--user`)

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
| `--config <path>` | — | JSON or YAML config file |

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

The key needs read access to the node list (`GET /api/v1/node`).

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
| `comment` | string | Record comment text |
| `zones` | []ZoneTarget | Multi-zone config (overrides single-zone fields) |

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
