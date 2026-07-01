package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// HeadscalePolicy holds the HuJSON ACL policy and its last update timestamp.
type HeadscalePolicy struct {
	Policy    string `json:"policy"`
	UpdatedAt string `json:"updatedAt"`
}

func hsPolicyClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

// getPolicy fetches the current ACL policy from the Headscale server.
func getPolicy(cfg *Config) (HeadscalePolicy, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.HeadscaleURL+"/api/v1/policy", nil)
	if err != nil {
		return HeadscalePolicy{}, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := hsPolicyClient().Do(req)
	if err != nil {
		return HeadscalePolicy{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return HeadscalePolicy{}, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result HeadscalePolicy
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return HeadscalePolicy{}, fmt.Errorf("decode policy response: %w", err)
	}
	return result, nil
}

// setPolicy sets the ACL policy on the Headscale server via PUT /api/v1/policy.
func setPolicy(cfg *Config, policy string) (HeadscalePolicy, error) {
	body, err := json.Marshal(map[string]string{"policy": policy})
	if err != nil {
		return HeadscalePolicy{}, err
	}

	req, err := http.NewRequest(http.MethodPut, cfg.HeadscaleURL+"/api/v1/policy", bytes.NewReader(body))
	if err != nil {
		return HeadscalePolicy{}, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := hsPolicyClient().Do(req)
	if err != nil {
		return HeadscalePolicy{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return HeadscalePolicy{}, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result HeadscalePolicy
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return HeadscalePolicy{}, fmt.Errorf("decode set-policy response: %w", err)
	}
	return result, nil
}

// ── policy command ─────────────────────────────────────────────────────────────

func runPolicy(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hsync policy <get|set> [flags]")
		os.Exit(1)
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "get":
		runPolicyGet(rest)
	case "set":
		runPolicySet(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown policy sub-command: %s\n", sub)
		fmt.Fprintln(os.Stderr, "Usage: hsync policy <get|set>")
		os.Exit(1)
	}
}

func runPolicyGet(args []string) {
	fs, cfg := newFlagSet("policy get")
	parseAndMerge(fs, cfg, args)

	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")

	p, err := getPolicy(cfg)
	must(err, "get policy")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, p)
		return
	}
	fmt.Print(p.Policy)
}

func runPolicySet(args []string) {
	fs, cfg := newFlagSet("policy set")
	file := fs.String("file", "", "Path to HuJSON policy file, or \"-\" for stdin")
	parseAndMerge(fs, cfg, args)

	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*file != "", "--file is required")

	var content []byte
	var err error
	if *file == "-" {
		content, err = io.ReadAll(os.Stdin)
	} else {
		content, err = os.ReadFile(*file)
	}
	must(err, "read policy file")

	p, err := setPolicy(cfg, string(content))
	must(err, "set policy")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, p)
		return
	}
	logInfo("Policy updated at %s", p.UpdatedAt)
}
