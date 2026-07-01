package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"
)

// HeadscaleRoute represents a Headscale subnet route.
type HeadscaleRoute struct {
	ID         string        `json:"id"`
	Node       HeadscaleNode `json:"node"`
	Prefix     string        `json:"prefix"`
	Advertised bool          `json:"advertised"`
	Enabled    bool          `json:"enabled"`
	IsPrimary  bool          `json:"isPrimary"`
	CreatedAt  string        `json:"createdAt"`
}

type headscaleRoutesResponse struct {
	Routes []HeadscaleRoute `json:"routes"`
}

func hsRouteClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

func listRoutes(cfg *Config) ([]HeadscaleRoute, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.HeadscaleURL+"/api/v1/routes", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := hsRouteClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result headscaleRoutesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode routes response: %w", err)
	}
	return result.Routes, nil
}

func enableRoute(cfg *Config, routeID string) error {
	return routeAction(cfg, routeID, "enable")
}

func disableRoute(cfg *Config, routeID string) error {
	return routeAction(cfg, routeID, "disable")
}

func routeAction(cfg *Config, routeID, action string) error {
	url := fmt.Sprintf("%s/api/v1/routes/%s/%s", cfg.HeadscaleURL, routeID, action)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := hsRouteClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	return nil
}

func deleteRoute(cfg *Config, routeID string) error {
	url := fmt.Sprintf("%s/api/v1/routes/%s", cfg.HeadscaleURL, routeID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)

	resp, err := hsRouteClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	return nil
}

// ── routes command ────────────────────────────────────────────────────────────

func runRoutes(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hsync routes <list|enable|disable|delete> [flags]")
		os.Exit(1)
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "list":
		runRoutesList(rest)
	case "enable":
		runRoutesEnable(rest)
	case "disable":
		runRoutesDisable(rest)
	case "delete":
		runRoutesDelete(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown routes sub-command: %s\n", sub)
		fmt.Fprintln(os.Stderr, "Usage: hsync routes <list|enable|disable|delete>")
		os.Exit(1)
	}
}

func runRoutesList(args []string) {
	fs, cfg := newFlagSet("routes list")
	node := fs.String("node", "", "Filter routes by node name (optional)")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")

	routes, err := listRoutes(cfg)
	must(err, "list routes")

	if *node != "" {
		filtered := routes[:0:0]
		for _, r := range routes {
			if r.Node.GivenName == *node || r.Node.Name == *node {
				filtered = append(filtered, r)
			}
		}
		routes = filtered
	}

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, routes)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "ID\tNODE\tPREFIX\tADVERTISED\tENABLED\tPRIMARY")
	for _, r := range routes {
		nodeName := r.Node.GivenName
		if nodeName == "" {
			nodeName = r.Node.Name
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%v\t%v\n",
			r.ID, nodeName, r.Prefix, r.Advertised, r.Enabled, r.IsPrimary)
	}
}

func runRoutesEnable(args []string) {
	fs, cfg := newFlagSet("routes enable")
	routeID := fs.String("route-id", "", "Route ID to enable")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*routeID != "", "--route-id is required")

	must(enableRoute(cfg, *routeID), "enable route")
	logInfo("Enabled route %s", *routeID)
}

func runRoutesDisable(args []string) {
	fs, cfg := newFlagSet("routes disable")
	routeID := fs.String("route-id", "", "Route ID to disable")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*routeID != "", "--route-id is required")

	must(disableRoute(cfg, *routeID), "disable route")
	logInfo("Disabled route %s", *routeID)
}

func runRoutesDelete(args []string) {
	fs, cfg := newFlagSet("routes delete")
	routeID := fs.String("route-id", "", "Route ID to delete")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*routeID != "", "--route-id is required")

	must(deleteRoute(cfg, *routeID), "delete route")
	logInfo("Deleted route %s", *routeID)
}
