package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// tagList is a repeatable string flag (--tag key:value, multiple allowed).
type tagList []string

func (t *tagList) String() string { return strings.Join(*t, ",") }
func (t *tagList) Set(v string) error {
	*t = append(*t, v)
	return nil
}

// ZoneTarget is a single Cloudflare zone to sync nodes into.
// Users is an allowlist of Headscale usernames; empty means all users.
// Tags are applied to every record in this zone (merged with global Tags).
type ZoneTarget struct {
	CFAPIToken string   `json:"cf_api_token" yaml:"cf_api_token"`
	CFZoneID   string   `json:"cf_zone_id"   yaml:"cf_zone_id"`
	Domain     string   `json:"domain"       yaml:"domain"`
	Users      []string `json:"users"        yaml:"users"`
	Tags       []string `json:"tags"         yaml:"tags"`
}

// FileConfig is the schema for an optional JSON or YAML config file.
// Pointer fields use *bool to distinguish "unset" from explicit false.
type FileConfig struct {
	HeadscaleURL    string       `json:"headscale_url"     yaml:"headscale_url"`
	HeadscaleAPIKey string       `json:"headscale_api_key" yaml:"headscale_api_key"`
	ManagedTag      string       `json:"managed_tag"       yaml:"managed_tag"`
	Tags            []string     `json:"tags"              yaml:"tags"`
	TTL             int          `json:"ttl"               yaml:"ttl"`
	Proxied         bool         `json:"proxied"           yaml:"proxied"`
	Prune           bool         `json:"prune"             yaml:"prune"`
	Comment         string       `json:"comment"           yaml:"comment"`
	SyncIPv4        bool         `json:"sync_ipv4"         yaml:"sync_ipv4"`
	SyncIPv6        *bool        `json:"sync_ipv6"         yaml:"sync_ipv6"`
	OnlineOnly      bool         `json:"online_only"       yaml:"online_only"`
	DryRun          bool         `json:"dry_run"           yaml:"dry_run"`
	DisableTags     bool         `json:"disable_tags"      yaml:"disable_tags"`
	UseHostname     bool         `json:"use_hostname"      yaml:"use_hostname"`
	// Single-zone shorthand (for simple setups without a zones array)
	CFAPIToken string `json:"cf_api_token" yaml:"cf_api_token"`
	CFZoneID   string `json:"cf_zone_id"   yaml:"cf_zone_id"`
	Domain     string `json:"domain"       yaml:"domain"`
	// Multi-zone
	Zones []ZoneTarget `json:"zones" yaml:"zones"`
}

// Config is the merged runtime configuration used across all commands.
type Config struct {
	HeadscaleURL    string
	HeadscaleAPIKey string
	// Single-zone (used when Zones is empty)
	CFAPIToken string
	CFZoneID   string
	Domain     string
	// Multi-zone (from config file zones array)
	Zones []ZoneTarget
	// Tag management
	ManagedTag string   // stamp on every managed record; used as prune filter
	Tags       tagList  // user-defined extra tags (--tag key:value, repeatable)
	// Sync options
	TTL        int
	Proxied    bool
	Prune      bool
	DryRun     bool
	SyncIPv4   bool
	SyncIPv6   bool
	OnlineOnly bool
	User       string // single-user filter for single-zone mode
	Comment    string
	// Runtime
	DisableTags bool
	UseHostname bool
	Verbose     bool
	JSONOutput  bool
	// Watch/Serve
	WatchInterval time.Duration
	ListenAddr    string
	WebhookSecret string
	// Internal
	configFile string
}

// effectiveTargets returns the ZoneTargets to use during sync.
// If Zones is configured (from config file), use those; otherwise build one
// from the single-zone flags with the optional User filter applied.
func (cfg *Config) effectiveTargets() []ZoneTarget {
	if len(cfg.Zones) > 0 {
		return cfg.Zones
	}
	users := []string{}
	if cfg.User != "" {
		users = []string{cfg.User}
	}
	return []ZoneTarget{{
		CFAPIToken: cfg.CFAPIToken,
		CFZoneID:   cfg.CFZoneID,
		Domain:     cfg.Domain,
		Users:      users,
	}}
}

// buildTags assembles the full tag set for a record:
// [managed-tag] + global user tags + zone-specific tags (deduped).
func (cfg *Config) buildTags(zoneExtra []string) []string {
	seen := map[string]bool{}
	var tags []string
	add := func(t string) {
		if t != "" && !seen[t] {
			seen[t] = true
			tags = append(tags, t)
		}
	}
	add(cfg.ManagedTag)
	for _, t := range cfg.Tags {
		add(t)
	}
	for _, t := range zoneExtra {
		add(t)
	}
	return tags
}

// loadConfigFile reads and parses a JSON or YAML config file.
// File type is determined by extension; JSON is assumed for unknown extensions.
func loadConfigFile(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var fc FileConfig
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("parse YAML config: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("parse JSON config: %w", err)
		}
	}
	return &fc, nil
}

// applyFileConfig merges file values into cfg, skipping any flag that was
// explicitly set on the command line (tracked in explicit).
func applyFileConfig(cfg *Config, fc *FileConfig, explicit map[string]bool) {
	str := func(flagName string, dst *string, src string) {
		if !explicit[flagName] && src != "" {
			*dst = src
		}
	}
	bl := func(flagName string, dst *bool, src bool) {
		if !explicit[flagName] && src {
			*dst = src
		}
	}
	str("headscale-url", &cfg.HeadscaleURL, fc.HeadscaleURL)
	str("headscale-key", &cfg.HeadscaleAPIKey, fc.HeadscaleAPIKey)
	str("cf-token", &cfg.CFAPIToken, fc.CFAPIToken)
	str("cf-zone", &cfg.CFZoneID, fc.CFZoneID)
	str("domain", &cfg.Domain, fc.Domain)
	str("managed-tag", &cfg.ManagedTag, fc.ManagedTag)
	str("comment", &cfg.Comment, fc.Comment)
	if !explicit["ttl"] && fc.TTL != 0 {
		cfg.TTL = fc.TTL
	}
	bl("proxied", &cfg.Proxied, fc.Proxied)
	bl("prune", &cfg.Prune, fc.Prune)
	bl("dry-run", &cfg.DryRun, fc.DryRun)
	bl("ipv4", &cfg.SyncIPv4, fc.SyncIPv4)
	bl("online-only", &cfg.OnlineOnly, fc.OnlineOnly)
	bl("disable-tags", &cfg.DisableTags, fc.DisableTags)
	bl("use-hostname", &cfg.UseHostname, fc.UseHostname)
	if !explicit["ipv6"] && fc.SyncIPv6 != nil {
		cfg.SyncIPv6 = *fc.SyncIPv6
	}
	// File tags are appended to (not replace) flag tags
	if len(fc.Tags) > 0 && !explicit["tag"] {
		cfg.Tags = append(cfg.Tags, fc.Tags...)
	}
	if len(fc.Zones) > 0 {
		cfg.Zones = fc.Zones
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// addCommonFlags registers flags present on every subcommand.
func addCommonFlags(fs *flag.FlagSet, cfg *Config) {
	fs.StringVar(&cfg.HeadscaleURL, "headscale-url", env("HEADSCALE_URL", defaultHeadscaleURL), "Headscale server URL")
	fs.StringVar(&cfg.HeadscaleAPIKey, "headscale-key", env("HEADSCALE_API_KEY", defaultHeadscaleAPIKey), "Headscale API key")
	fs.StringVar(&cfg.CFAPIToken, "cf-token", env("CLOUDFLARE_API_TOKEN", defaultCFAPIToken), "Cloudflare API token")
	fs.StringVar(&cfg.CFZoneID, "cf-zone", env("CLOUDFLARE_ZONE_ID", defaultCFZoneID), "Cloudflare zone ID")
	fs.StringVar(&cfg.configFile, "config", "", "Path to JSON or YAML config file")
	fs.BoolVar(&cfg.Verbose, "v", false, "Verbose/debug output")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "Verbose/debug output")
	fs.BoolVar(&cfg.JSONOutput, "json", false, "Output results as JSON")
}

// addSyncFlags registers flags for sync/watch/serve commands.
func addSyncFlags(fs *flag.FlagSet, cfg *Config) {
	fs.StringVar(&cfg.Domain, "domain", env("DOMAIN", defaultDomain), "Domain suffix (e.g. ts.example.com)")
	fs.IntVar(&cfg.TTL, "ttl", 60, "DNS record TTL in seconds")
	fs.BoolVar(&cfg.Proxied, "proxied", false, "Proxy records through Cloudflare CDN")
	fs.BoolVar(&cfg.Prune, "prune", false, "Delete Cloudflare records absent from Headscale (managed-tag only)")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Preview changes without applying")
	fs.BoolVar(&cfg.SyncIPv4, "ipv4", false, "Sync A (IPv4) records")
	fs.BoolVar(&cfg.SyncIPv6, "ipv6", true, "Sync AAAA (IPv6) records")
	fs.BoolVar(&cfg.OnlineOnly, "online-only", false, "Skip nodes that are not currently online")
	fs.StringVar(&cfg.User, "user", "", "Only sync nodes belonging to this Headscale user")
	fs.StringVar(&cfg.Comment, "comment", "Managed by hsync", "Comment written to every DNS record")
	fs.StringVar(&cfg.ManagedTag, "managed-tag", "managed:hsync", "Tag stamped on every managed record; prune only removes records with this tag")
	fs.Var(&cfg.Tags, "tag", "Extra tag to apply to every record (key:value, repeatable)")
	fs.BoolVar(&cfg.DisableTags, "disable-tags", false, "Omit tags from Cloudflare API calls (required for free/non-Enterprise zones)")
	fs.BoolVar(&cfg.UseHostname, "use-hostname", false, "Use the machine hostname instead of the Headscale-configured name for DNS records")
}

// parseAndMerge parses args, then loads and merges any config file.
func parseAndMerge(fs *flag.FlagSet, cfg *Config, args []string) {
	fs.Parse(args)

	if cfg.configFile == "" {
		return
	}
	fc, err := loadConfigFile(cfg.configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	applyFileConfig(cfg, fc, explicit)
}
