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

// HeadscaleUser represents a Headscale user/namespace.
type HeadscaleUser struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

type headscaleUsersResponse struct {
	Users []HeadscaleUser `json:"users"`
}

type headscaleUserResponse struct {
	User HeadscaleUser `json:"user"`
}

func hsUserClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

func listUsers(cfg *Config) ([]HeadscaleUser, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.HeadscaleURL+"/api/v1/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := hsUserClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result headscaleUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode users response: %w", err)
	}
	return result.Users, nil
}

func createUser(cfg *Config, name string) (HeadscaleUser, error) {
	body, _ := json.Marshal(map[string]string{"name": name})
	req, err := http.NewRequest(http.MethodPost, cfg.HeadscaleURL+"/api/v1/user", bytes.NewReader(body))
	if err != nil {
		return HeadscaleUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := hsUserClient().Do(req)
	if err != nil {
		return HeadscaleUser{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return HeadscaleUser{}, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result headscaleUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return HeadscaleUser{}, fmt.Errorf("decode create-user response: %w", err)
	}
	return result.User, nil
}

func deleteUser(cfg *Config, name string) error {
	req, err := http.NewRequest(http.MethodDelete, cfg.HeadscaleURL+"/api/v1/user/"+name, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)

	resp, err := hsUserClient().Do(req)
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

func renameUser(cfg *Config, oldName, newName string) (HeadscaleUser, error) {
	url := fmt.Sprintf("%s/api/v1/user/%s/rename/%s", cfg.HeadscaleURL, oldName, newName)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return HeadscaleUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HeadscaleAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := hsUserClient().Do(req)
	if err != nil {
		return HeadscaleUser{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return HeadscaleUser{}, fmt.Errorf("Headscale API HTTP %d: %s", resp.StatusCode, b)
	}
	var result headscaleUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return HeadscaleUser{}, fmt.Errorf("decode rename-user response: %w", err)
	}
	return result.User, nil
}

// ── users command ─────────────────────────────────────────────────────────────

func runUsers(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hsync users <list|create|delete|rename> [flags]")
		os.Exit(1)
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "list":
		runUsersList(rest)
	case "create":
		runUsersCreate(rest)
	case "delete":
		runUsersDelete(rest)
	case "rename":
		runUsersRename(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown users sub-command: %s\n", sub)
		fmt.Fprintln(os.Stderr, "Usage: hsync users <list|create|delete|rename>")
		os.Exit(1)
	}
}

func runUsersList(args []string) {
	fs, cfg := newFlagSet("users list")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")

	users, err := listUsers(cfg)
	must(err, "list users")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, users)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "ID\tNAME\tCREATED")
	for _, u := range users {
		fmt.Fprintf(w, "%s\t%s\t%s\n", u.ID, u.Name, dash(u.CreatedAt))
	}
}

func runUsersCreate(args []string) {
	fs, cfg := newFlagSet("users create")
	name := fs.String("name", "", "Username to create")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*name != "", "--name is required")

	u, err := createUser(cfg, *name)
	must(err, "create user")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, u)
		return
	}
	logInfo("Created user %q (id %s)", u.Name, u.ID)
}

func runUsersDelete(args []string) {
	fs, cfg := newFlagSet("users delete")
	name := fs.String("name", "", "Username to delete")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*name != "", "--name is required")

	must(deleteUser(cfg, *name), "delete user")
	logInfo("Deleted user %q", *name)
}

func runUsersRename(args []string) {
	fs, cfg := newFlagSet("users rename")
	name := fs.String("name", "", "Current username")
	newName := fs.String("new-name", "", "New username")
	parseAndMerge(fs, cfg, args)
	require(cfg.HeadscaleURL != "", "headscale-url is required")
	require(cfg.HeadscaleAPIKey != "", "headscale-key is required")
	require(*name != "", "--name is required")
	require(*newName != "", "--new-name is required")

	u, err := renameUser(cfg, *name, *newName)
	must(err, "rename user")

	if cfg.JSONOutput {
		mustEncodeJSON(os.Stdout, u)
		return
	}
	logInfo("Renamed user %q -> %q (id %s)", *name, u.Name, u.ID)
}
