package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// SyncStats tracks what happened during a single sync run.
type SyncStats struct {
	Created   int `json:"created"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	Deleted   int `json:"deleted"`
	Errors    int `json:"errors"`
}

// ── sync command ──────────────────────────────────────────────────────────────

func runSync(args []string) {
	fs, cfg := newFlagSet("sync")
	addSyncFlags(fs, cfg)
	parseAndMerge(fs, cfg, args)
	requireSyncConfig(cfg)

	stats, err := doSync(cfg)
	must(err, "sync")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, stats)
		return
	}
	if cfg.DryRun {
		logInfo("DRY RUN complete — no changes made")
	} else {
		logInfo("Sync complete: %d created, %d updated, %d unchanged, %d deleted, %d errors",
			stats.Created, stats.Updated, stats.Unchanged, stats.Deleted, stats.Errors)
	}
}

// ── watch command ─────────────────────────────────────────────────────────────

func runWatch(args []string) {
	fs, cfg := newFlagSet("watch")
	addSyncFlags(fs, cfg)
	fs.DurationVar(&cfg.WatchInterval, "interval", 5*time.Minute, "Sync interval (e.g. 30s, 5m, 1h)")
	parseAndMerge(fs, cfg, args)
	requireSyncConfig(cfg)

	logInfo("Watch mode: syncing every %s", cfg.WatchInterval)
	runOnce(cfg)
	ticker := time.NewTicker(cfg.WatchInterval)
	defer ticker.Stop()
	for range ticker.C {
		runOnce(cfg)
	}
}

func runOnce(cfg *Config) {
	start := time.Now()
	stats, err := doSync(cfg)
	dur := time.Since(start)
	globalMetrics.recordSync(dur, stats, err)
	if err != nil {
		logWarn("Sync error: %v", err)
		return
	}
	logInfo("Sync done in %s: %d created, %d updated, %d unchanged, %d deleted, %d errors",
		dur.Round(time.Millisecond),
		stats.Created, stats.Updated, stats.Unchanged, stats.Deleted, stats.Errors)
}

// ── core sync logic ───────────────────────────────────────────────────────────

// doSync fetches Headscale nodes and syncs them to every configured zone.
func doSync(cfg *Config) (*SyncStats, error) {
	if cfg.DryRun {
		logInfo("DRY RUN — no changes will be made")
	}

	nodes, err := fetchHeadscaleNodes(cfg)
	if err != nil {
		return nil, fmt.Errorf("fetch nodes: %w", err)
	}
	logInfo("Found %d Headscale nodes", len(nodes))
	globalMetrics.setNodeCount(len(nodes))

	stats := &SyncStats{}

	for _, target := range cfg.effectiveTargets() {
		filtered := filterByTarget(nodes, target, cfg.OnlineOnly)
		logInfo("Zone %s: %d nodes after filters", target.Domain, len(filtered))

		if cfg.SyncIPv6 {
			if err := syncRecordType(cfg, filtered, "AAAA", target, stats); err != nil {
				return stats, fmt.Errorf("zone %s AAAA: %w", target.Domain, err)
			}
		}
		if cfg.SyncIPv4 {
			if err := syncRecordType(cfg, filtered, "A", target, stats); err != nil {
				return stats, fmt.Errorf("zone %s A: %w", target.Domain, err)
			}
		}
	}
	return stats, nil
}

func syncRecordType(cfg *Config, nodes []HeadscaleNode, recType string, target ZoneTarget, stats *SyncStats) error {
	existing, err := fetchCFRecords(cfg, target.CFAPIToken, target.CFZoneID, recType)
	if err != nil {
		return fmt.Errorf("fetch %s records: %w", recType, err)
	}
	logInfo("  %s: %d existing records in Cloudflare", recType, len(existing))

	// Index existing records by FQDN for O(1) lookup
	byName := make(map[string]CloudflareRecord, len(existing))
	for _, r := range existing {
		byName[r.Name] = r
	}

	wantedTags := cfg.buildTags(target.Tags)
	headscaleFQDNs := make(map[string]bool)

	for _, node := range nodes {
		v4, v6 := extractIPs(node.IPAddresses)
		var ip string
		switch recType {
		case "AAAA":
			ip = v6
		case "A":
			ip = v4
		}
		if ip == "" {
			logDebug(cfg, "SKIP %s: no %s address", node.Name, recType)
			continue
		}

		fqdn := node.Name + "." + target.Domain
		headscaleFQDNs[fqdn] = true

		rec, exists := byName[fqdn]
		if !exists {
			logInfo("  CREATE %s %s %s", recType, fqdn, ip)
			if !cfg.DryRun {
				if err := createCFRecord(cfg, target.CFAPIToken, target.CFZoneID, fqdn, ip, recType, wantedTags); err != nil {
					logWarn("    create failed: %v", err)
					stats.Errors++
				} else {
					logInfo("    -> created")
					stats.Created++
				}
			}
			continue
		}

		ipChanged := rec.Content != ip
		tagsChanged := !cfg.DisableTags && !tagsMatch(rec.Tags, wantedTags)
		if ipChanged || tagsChanged {
			if ipChanged {
				logInfo("  UPDATE %s %s  %s -> %s", recType, fqdn, rec.Content, ip)
			} else {
				logInfo("  UPDATE %s %s (tags changed)", recType, fqdn)
			}
			if !cfg.DryRun {
				if err := updateCFRecord(cfg, target.CFAPIToken, target.CFZoneID, rec.ID, fqdn, ip, recType, wantedTags); err != nil {
					logWarn("    update failed: %v", err)
					stats.Errors++
				} else {
					logInfo("    -> updated")
					stats.Updated++
				}
			}
		} else {
			logDebug(cfg, "UNCHANGED %s %s (%s)", recType, fqdn, ip)
			stats.Unchanged++
		}
	}

	if cfg.Prune {
		suffix := "." + target.Domain
		for name, rec := range byName {
			if !strings.HasSuffix(name, suffix) {
				continue
			}
			// Only prune records carrying our managed tag — never touch hand-managed records.
			if !hasManagedTag(rec, cfg.ManagedTag) {
				logDebug(cfg, "SKIP prune %s: no managed tag %q", name, cfg.ManagedTag)
				continue
			}
			if !headscaleFQDNs[name] {
				logInfo("  DELETE %s %s (%s) — not in Headscale", recType, name, rec.Content)
				if !cfg.DryRun {
					if err := deleteCFRecord(cfg, target.CFAPIToken, target.CFZoneID, rec.ID); err != nil {
						logWarn("    delete failed: %v", err)
						stats.Errors++
					} else {
						logInfo("    -> deleted")
						stats.Deleted++
					}
				}
			}
		}
	}
	return nil
}

// requireSyncConfig validates that the minimum fields needed for a sync are set.
func requireSyncConfig(cfg *Config) {
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(cfg.ManagedTag != "", "managed-tag must not be empty")

	// For single-zone mode we need the three CF fields; multi-zone comes from
	// config file (each ZoneTarget is validated lazily during sync).
	if len(cfg.Zones) == 0 {
		require(cfg.CFAPIToken != "", "cf-token is required (or set zones in config file)")
		require(cfg.CFZoneID != "", "cf-zone is required (or set zones in config file)")
		require(cfg.Domain != "", "domain is required (or set zones in config file)")
	}
}
