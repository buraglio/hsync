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

func hsNodeClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

// getNode fetches a single node by ID via GET /api/v1/node/{id}.
func getNode(cfg *Config, nodeID string) (HeadscaleNode, error) {
	url := fmt.Sprintf("%s/api/v1/node/%s", cfg.HeadscaleURL, nodeID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return HeadscaleNode{}, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := hsNodeClient().Do(req)
	if err != nil {
		return HeadscaleNode{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return HeadscaleNode{}, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result headscaleNodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return HeadscaleNode{}, fmt.Errorf("decode get-node response: %w", err)
	}
	return result.Node, nil
}

// deleteNode removes a node by ID via DELETE /api/v1/node/{id}.
func deleteNode(cfg *Config, nodeID string) error {
	url := fmt.Sprintf("%s/api/v1/node/%s", cfg.HeadscaleURL, nodeID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)

	resp, err := hsNodeClient().Do(req)
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

// expireNode expires a node's key via POST /api/v1/node/{id}/expire.
func expireNode(cfg *Config, nodeID string) (HeadscaleNode, error) {
	url := fmt.Sprintf("%s/api/v1/node/%s/expire", cfg.HeadscaleURL, nodeID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return HeadscaleNode{}, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := hsNodeClient().Do(req)
	if err != nil {
		return HeadscaleNode{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return HeadscaleNode{}, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result headscaleNodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return HeadscaleNode{}, fmt.Errorf("decode expire-node response: %w", err)
	}
	return result.Node, nil
}

// moveNode moves a node to a different user via POST /api/v1/node/{id}/user.
func moveNode(cfg *Config, nodeID, user string) (HeadscaleNode, error) {
	body, _ := json.Marshal(map[string]string{"user": user})
	url := fmt.Sprintf("%s/api/v1/node/%s/user", cfg.HeadscaleURL, nodeID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return HeadscaleNode{}, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := hsNodeClient().Do(req)
	if err != nil {
		return HeadscaleNode{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return HeadscaleNode{}, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result headscaleNodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return HeadscaleNode{}, fmt.Errorf("decode move-node response: %w", err)
	}
	return result.Node, nil
}

// ── node command ──────────────────────────────────────────────────────────────

func runNode(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hsync node <show|delete|expire|move> [flags]")
		os.Exit(1)
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "show":
		runNodeShow(rest)
	case "delete":
		runNodeDelete(rest)
	case "expire":
		runNodeExpire(rest)
	case "move":
		runNodeMove(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown node sub-command: %s\n", sub)
		fmt.Fprintln(os.Stderr, "Usage: hsync node <show|delete|expire|move>")
		os.Exit(1)
	}
}

func runNodeShow(args []string) {
	fs, cfg := newFlagSet("node show")
	nodeName := fs.String("node", "", "Node name (givenName or hostname)")
	parseAndMerge(fs, cfg, args)

	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*nodeName != "", "--node is required")

	nodes, err := fetchHeadscaleNodes(cfg)
	must(err, "fetch nodes")

	n, err := findNodeByName(nodes, *nodeName)
	must(err, "find node")

	node, err := getNode(cfg, n.ID)
	must(err, "get node")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, node)
		return
	}

	v4, v6 := extractIPs(node.IPAddresses)
	online := "no"
	if node.Online {
		online = "yes"
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "ID\t%s\n", node.ID)
	fmt.Fprintf(w, "Name\t%s\n", node.Name)
	fmt.Fprintf(w, "GivenName\t%s\n", dash(node.GivenName))
	fmt.Fprintf(w, "User\t%s\n", node.User.Name)
	fmt.Fprintf(w, "Online\t%s\n", online)
	fmt.Fprintf(w, "LastSeen\t%s\n", dash(node.LastSeen))
	fmt.Fprintf(w, "IPv4\t%s\n", dash(v4))
	fmt.Fprintf(w, "IPv6\t%s\n", dash(v6))
}

func runNodeDelete(args []string) {
	fs, cfg := newFlagSet("node delete")
	nodeName := fs.String("node", "", "Node name (givenName or hostname)")
	parseAndMerge(fs, cfg, args)

	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*nodeName != "", "--node is required")

	nodes, err := fetchHeadscaleNodes(cfg)
	must(err, "fetch nodes")

	n, err := findNodeByName(nodes, *nodeName)
	must(err, "find node")

	must(deleteNode(cfg, n.ID), "delete node")
	logInfo("Deleted node %q (id %s)", *nodeName, n.ID)
}

func runNodeExpire(args []string) {
	fs, cfg := newFlagSet("node expire")
	nodeName := fs.String("node", "", "Node name (givenName or hostname)")
	parseAndMerge(fs, cfg, args)

	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*nodeName != "", "--node is required")

	nodes, err := fetchHeadscaleNodes(cfg)
	must(err, "fetch nodes")

	n, err := findNodeByName(nodes, *nodeName)
	must(err, "find node")

	updated, err := expireNode(cfg, n.ID)
	must(err, "expire node")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, updated)
		return
	}
	logInfo("Expired node %q (id %s)", *nodeName, n.ID)
}

func runNodeMove(args []string) {
	fs, cfg := newFlagSet("node move")
	nodeName := fs.String("node", "", "Node name (givenName or hostname)")
	userName := fs.String("user", "", "Target user to move the node to")
	parseAndMerge(fs, cfg, args)

	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*nodeName != "", "--node is required")
	require(*userName != "", "--user is required")

	nodes, err := fetchHeadscaleNodes(cfg)
	must(err, "fetch nodes")

	n, err := findNodeByName(nodes, *nodeName)
	must(err, "find node")

	updated, err := moveNode(cfg, n.ID, *userName)
	must(err, "move node")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, updated)
		return
	}
	logInfo("Moved node %q (id %s) to user %q", *nodeName, n.ID, updated.User.Name)
}
