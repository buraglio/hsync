package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const cfAPIBase = "https://api.cloudflare.com/client/v4"

// CloudflareRecord represents a Cloudflare DNS record.
type CloudflareRecord struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Content string   `json:"content"`
	TTL     int      `json:"ttl"`
	Proxied bool     `json:"proxied"`
	Comment string   `json:"comment,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type cloudflareResponse struct {
	Success    bool               `json:"success"`
	Result     []CloudflareRecord `json:"result"`
	Errors     []cloudflareError  `json:"errors"`
	ResultInfo struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

type cloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// cfDo executes a Cloudflare API call with up to 3 attempts.
// Retries on network errors, HTTP 429, and HTTP 5xx with exponential backoff.
func cfDo(cfg *Config, token, method, path string, payload interface{}) ([]byte, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		data, retryable, err := cfDoOnce(token, method, path, payload)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !retryable || attempt == maxAttempts {
			return nil, err
		}
		wait := time.Duration(1<<(attempt-1)) * time.Second // 1s, 2s
		logWarn("CF attempt %d/%d failed (%v), retrying in %s", attempt, maxAttempts, err, wait)
		time.Sleep(wait)
	}
	return nil, lastErr
}

func cfDoOnce(token, method, path string, payload interface{}) (data []byte, retryable bool, err error) {
	var body io.Reader
	if payload != nil {
		b, e := json.Marshal(payload)
		if e != nil {
			return nil, false, e
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, cfAPIBase+path, body)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, true, err // network error — retryable
	}
	defer resp.Body.Close()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, true, fmt.Errorf("rate limited (HTTP 429)")
	case resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("server error HTTP %d", resp.StatusCode)
	case resp.StatusCode >= 400:
		return nil, false, fmt.Errorf("client error HTTP %d: %s", resp.StatusCode, data)
	}

	var check struct {
		Success bool              `json:"success"`
		Errors  []cloudflareError `json:"errors"`
	}
	if err := json.Unmarshal(data, &check); err != nil {
		return nil, false, fmt.Errorf("decode CF response: %w", err)
	}
	if !check.Success {
		msgs := make([]string, 0, len(check.Errors))
		for _, e := range check.Errors {
			msgs = append(msgs, e.Message)
		}
		return nil, false, fmt.Errorf("Cloudflare: %s", strings.Join(msgs, "; "))
	}
	return data, false, nil
}

// fetchCFRecords returns all DNS records of recordType in the given zone,
// paginating through all result pages.
func fetchCFRecords(cfg *Config, token, zoneID, recordType string) ([]CloudflareRecord, error) {
	var all []CloudflareRecord
	for page := 1; ; page++ {
		path := fmt.Sprintf("/zones/%s/dns_records?type=%s&per_page=100&page=%d", zoneID, recordType, page)
		data, err := cfDo(cfg, token, "GET", path, nil)
		if err != nil {
			return nil, err
		}
		var r cloudflareResponse
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("decode records: %w", err)
		}
		all = append(all, r.Result...)
		if page >= r.ResultInfo.TotalPages || r.ResultInfo.TotalPages == 0 {
			break
		}
	}
	return all, nil
}

func createCFRecord(cfg *Config, token, zoneID, name, ip, recType string, tags []string) error {
	_, err := cfDo(cfg, token, "POST",
		fmt.Sprintf("/zones/%s/dns_records", zoneID),
		map[string]interface{}{
			"type":    recType,
			"name":    name,
			"content": ip,
			"ttl":     cfg.TTL,
			"proxied": cfg.Proxied,
			"comment": cfg.Comment,
			"tags":    tags,
		})
	return err
}

func updateCFRecord(cfg *Config, token, zoneID, id, name, ip, recType string, tags []string) error {
	_, err := cfDo(cfg, token, "PATCH",
		fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, id),
		map[string]interface{}{
			"type":    recType,
			"name":    name,
			"content": ip,
			"ttl":     cfg.TTL,
			"proxied": cfg.Proxied,
			"comment": cfg.Comment,
			"tags":    tags,
		})
	return err
}

func deleteCFRecord(cfg *Config, token, zoneID, id string) error {
	_, err := cfDo(cfg, token, "DELETE",
		fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, id),
		nil)
	return err
}

// hasManagedTag reports whether rec carries the given managed tag.
func hasManagedTag(rec CloudflareRecord, tag string) bool {
	for _, t := range rec.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// tagsMatch reports whether two tag slices are equal (order-independent).
func tagsMatch(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := append([]string(nil), a...)
	sb := append([]string(nil), b...)
	sort.Strings(sa)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}
