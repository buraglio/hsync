package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HeadscaleNode is the subset of fields we use from /api/v1/node.
type HeadscaleNode struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	GivenName   string   `json:"givenName"`
	IPAddresses []string `json:"ipAddresses"`
	Online      bool     `json:"online"`
	LastSeen    string   `json:"lastSeen"`
	User        struct {
		Name string `json:"name"`
	} `json:"user"`
}

type headscaleNodesResponse struct {
	Nodes []HeadscaleNode `json:"nodes"`
}

// fetchHeadscaleNodes retrieves all nodes from the Headscale API.
func fetchHeadscaleNodes(cfg *Config) ([]HeadscaleNode, error) {
	logInfo("Fetching nodes from %s", cfg.HeadscaleURL)

	req, err := http.NewRequest("GET", cfg.HeadscaleURL+"/api/v1/node", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}

	var result headscaleNodesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Headscale response: %w", err)
	}
	return result.Nodes, nil
}

// filterByTarget returns nodes matching a ZoneTarget's user allowlist,
// applying the global onlineOnly and singleUser filters too.
func filterByTarget(nodes []HeadscaleNode, target ZoneTarget, onlineOnly bool) []HeadscaleNode {
	if !onlineOnly && len(target.Users) == 0 {
		return nodes
	}
	userSet := make(map[string]bool, len(target.Users))
	for _, u := range target.Users {
		userSet[u] = true
	}
	out := nodes[:0:0]
	for _, n := range nodes {
		if onlineOnly && !n.Online {
			continue
		}
		if len(userSet) > 0 && !userSet[n.User.Name] {
			continue
		}
		out = append(out, n)
	}
	return out
}

// filterNodes applies simple onlineOnly / single-user filters (used by list).
func filterNodes(nodes []HeadscaleNode, onlineOnly bool, user string) []HeadscaleNode {
	if !onlineOnly && user == "" {
		return nodes
	}
	out := nodes[:0:0]
	for _, n := range nodes {
		if onlineOnly && !n.Online {
			continue
		}
		if user != "" && n.User.Name != user {
			continue
		}
		out = append(out, n)
	}
	return out
}

// nodeDNSName returns the name used for DNS records. By default it prefers
// the Headscale-configured GivenName; pass useHostname=true to always use
// the machine hostname (Name) instead.
func nodeDNSName(n HeadscaleNode, useHostname bool) string {
	if !useHostname && n.GivenName != "" {
		return n.GivenName
	}
	return n.Name
}

// extractIPs splits a mixed IP list into the first v4 and first v6 address.
func extractIPs(ips []string) (v4, v6 string) {
	for _, ip := range ips {
		if strings.Contains(ip, ":") {
			if v6 == "" {
				v6 = ip
			}
		} else {
			if v4 == "" {
				v4 = ip
			}
		}
	}
	return
}
