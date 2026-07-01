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

// HeadscalePreAuthKey represents a Headscale pre-authentication key.
type HeadscalePreAuthKey struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	User       string `json:"user"`
	Reusable   bool   `json:"reusable"`
	Ephemeral  bool   `json:"ephemeral"`
	Used       bool   `json:"used"`
	Expiration string `json:"expiration"`
	CreatedAt  string `json:"createdAt"`
}

type headscalePreAuthKeysResponse struct {
	PreAuthKeys []HeadscalePreAuthKey `json:"preAuthKeys"`
}

type headscalePreAuthKeyResponse struct {
	PreAuthKey HeadscalePreAuthKey `json:"preAuthKey"`
}

func hsPAKClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

func listPreAuthKeys(cfg *Config, user string) ([]HeadscalePreAuthKey, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.HeadscaleURL+"/api/v1/preauthkey", nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	if user != "" {
		q.Set("user", user)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := hsPAKClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result headscalePreAuthKeysResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode preauthkeys response: %w", err)
	}
	return result.PreAuthKeys, nil
}

// createPreAuthKey creates a new pre-auth key. expiration is an RFC3339 string;
// pass "" to use the Headscale server default.
func createPreAuthKey(cfg *Config, user string, reusable, ephemeral bool, expiration string) (HeadscalePreAuthKey, error) {
	payload := map[string]interface{}{
		"user":      user,
		"reusable":  reusable,
		"ephemeral": ephemeral,
	}
	if expiration != "" {
		payload["expiration"] = expiration
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, cfg.HeadscaleURL+"/api/v1/preauthkey", bytes.NewReader(body))
	if err != nil {
		return HeadscalePreAuthKey{}, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := hsPAKClient().Do(req)
	if err != nil {
		return HeadscalePreAuthKey{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return HeadscalePreAuthKey{}, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result headscalePreAuthKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return HeadscalePreAuthKey{}, fmt.Errorf("decode create-preauthkey response: %w", err)
	}
	return result.PreAuthKey, nil
}

// expirePreAuthKey expires (invalidates) a pre-auth key via DELETE /api/v1/preauthkey.
func expirePreAuthKey(cfg *Config, user, key string) error {
	body, _ := json.Marshal(map[string]string{"user": user, "key": key})
	req, err := http.NewRequest(http.MethodDelete, cfg.HeadscaleURL+"/api/v1/preauthkey", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := hsPAKClient().Do(req)
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

// ── preauthkey command ────────────────────────────────────────────────────────

func runPreAuthKey(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hsync preauthkey <list|create|expire> [flags]")
		os.Exit(1)
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "list":
		runPreAuthKeyList(rest)
	case "create":
		runPreAuthKeyCreate(rest)
	case "expire":
		runPreAuthKeyExpire(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown preauthkey sub-command: %s\n", sub)
		fmt.Fprintln(os.Stderr, "Usage: hsync preauthkey <list|create|expire>")
		os.Exit(1)
	}
}

func runPreAuthKeyList(args []string) {
	fs, cfg := newFlagSet("preauthkey list")
	user := fs.String("user", "", "Filter keys by Headscale username (optional)")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")

	keys, err := listPreAuthKeys(cfg, *user)
	must(err, "list preauthkeys")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, keys)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "ID\tUSER\tKEY\tREUSABLE\tEPHEMERAL\tUSED\tEXPIRATION")
	for _, k := range keys {
		fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%v\t%v\t%s\n",
			k.ID, k.User, k.Key, k.Reusable, k.Ephemeral, k.Used, dash(k.Expiration))
	}
}

func runPreAuthKeyCreate(args []string) {
	fs, cfg := newFlagSet("preauthkey create")
	user := fs.String("user", "", "Headscale user to create the key for")
	reusable := fs.Bool("reusable", false, "Allow the key to be used multiple times")
	ephemeral := fs.Bool("ephemeral", false, "Create nodes as ephemeral (auto-deleted when offline)")
	expiration := fs.Duration("expiration", 0, "Key lifetime (e.g. 24h, 30d). 0 = server default")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*user != "", "--user is required")

	var expStr string
	if *expiration > 0 {
		expStr = time.Now().Add(*expiration).UTC().Format(time.RFC3339)
	}

	k, err := createPreAuthKey(cfg, *user, *reusable, *ephemeral, expStr)
	must(err, "create preauthkey")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, k)
		return
	}
	logInfo("Created pre-auth key for user %q:", *user)
	fmt.Printf("  Key:       %s\n", k.Key)
	fmt.Printf("  Reusable:  %v\n", k.Reusable)
	fmt.Printf("  Ephemeral: %v\n", k.Ephemeral)
	fmt.Printf("  Expires:   %s\n", dash(k.Expiration))
}

func runPreAuthKeyExpire(args []string) {
	fs, cfg := newFlagSet("preauthkey expire")
	user := fs.String("user", "", "Headscale username that owns the key")
	key := fs.String("key", "", "Full key value to expire")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*user != "", "--user is required")
	require(*key != "", "--key is required")

	must(expirePreAuthKey(cfg, *user, *key), "expire preauthkey")
	logInfo("Expired pre-auth key %q for user %q", *key, *user)
}
