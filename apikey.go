package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"
)

// HeadscaleAPIKey represents a Headscale API key.
type HeadscaleAPIKey struct {
	ID         string `json:"id"`
	Prefix     string `json:"prefix"`
	Expiration string `json:"expiration"`
	CreatedAt  string `json:"createdAt"`
}

type headscaleAPIKeysResponse struct {
	APIKeys []HeadscaleAPIKey `json:"apiKeys"`
}

func hsAPIKeyClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

// listAPIKeys returns all API keys from the Headscale server.
func listAPIKeys(cfg *Config) ([]HeadscaleAPIKey, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.HeadscaleURL+"/api/v1/apikey", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := hsAPIKeyClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result headscaleAPIKeysResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode apikeys response: %w", err)
	}
	return result.APIKeys, nil
}

// createAPIKey creates a new API key. expiration is an RFC3339 string;
// pass "" to use the Headscale server default.
func createAPIKey(cfg *Config, expiration string) (string, error) {
	payload := map[string]interface{}{}
	if expiration != "" {
		payload["expiration"] = expiration
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, cfg.HeadscaleURL+"/api/v1/apikey", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := hsAPIKeyClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode create-apikey response: %w", err)
	}
	return result["apiKey"], nil
}

// expireAPIKey expires (invalidates) an API key via DELETE /api/v1/apikey/{prefix}.
func expireAPIKey(cfg *Config, prefix string) error {
	req, err := http.NewRequest(http.MethodDelete, cfg.HeadscaleURL+"/api/v1/apikey/"+prefix, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := hsAPIKeyClient().Do(req)
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

// ── apikey command ────────────────────────────────────────────────────────────

func runAPIKey(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hsync apikey <list|create|expire> [flags]")
		os.Exit(1)
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "list":
		runAPIKeyList(rest)
	case "create":
		runAPIKeyCreate(rest)
	case "expire":
		runAPIKeyExpire(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown apikey sub-command: %s\n", sub)
		fmt.Fprintln(os.Stderr, "Usage: hsync apikey <list|create|expire>")
		os.Exit(1)
	}
}

func runAPIKeyList(args []string) {
	fs, cfg := newFlagSet("apikey list")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")

	keys, err := listAPIKeys(cfg)
	must(err, "list apikeys")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, keys)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "ID\tPREFIX\tEXPIRATION\tCREATED")
	for _, k := range keys {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			k.ID, k.Prefix, dash(k.Expiration), dash(k.CreatedAt))
	}
}

func runAPIKeyCreate(args []string) {
	fs, cfg := newFlagSet("apikey create")
	expiration := fs.Duration("expiration", 0, "Key lifetime (e.g. 24h, 720h). 0 = server default")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")

	var expStr string
	if *expiration > 0 {
		expStr = time.Now().Add(*expiration).UTC().Format(time.RFC3339)
	}

	key, err := createAPIKey(cfg, expStr)
	must(err, "create apikey")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, map[string]string{"apiKey": key})
		return
	}
	logInfo("Created API key (shown once — store it securely:")
	fmt.Println(key)
}

func runAPIKeyExpire(args []string) {
	fs, cfg := newFlagSet("apikey expire")
	prefix := fs.String("prefix", "", "API key prefix to expire")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*prefix != "", "--prefix is required")

	must(expireAPIKey(cfg, *prefix), "expire apikey")
	logInfo("Expired API key with prefix %q", *prefix)
}
