package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ipconfigURL = "https://ipconfig.io/ip"

func fetchIP() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(ipconfigURL)
	if err != nil {
		return "", fmt.Errorf("fetching IP: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ipconfig.io returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}
	return strings.TrimSpace(string(body)), nil
}
