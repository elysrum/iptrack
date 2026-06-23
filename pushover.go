package main

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var pushoverURL = "https://api.pushover.net/1/messages.json"

func notify(token, user, title, message string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(pushoverURL, url.Values{
		"token":   {token},
		"user":    {user},
		"title":   {title},
		"message": {message},
	})
	if err != nil {
		return fmt.Errorf("sending notification: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pushover returned status %d", resp.StatusCode)
	}
	return nil
}
