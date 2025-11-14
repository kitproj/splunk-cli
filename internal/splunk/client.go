package splunk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client represents a Splunk API client
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

// NewClient creates a new Splunk API client
func NewClient(host, token string) *Client {
	return &Client{
		BaseURL: fmt.Sprintf("https://%s:8089", host),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Token: token,
	}
}

// Search represents a Splunk search job
type Search struct {
	SID     string `json:"sid"`
	Content struct {
		IsDone        bool   `json:"isDone"`
		ResultCount   int    `json:"resultCount"`
		EventCount    int    `json:"eventCount"`
		DispatchState string `json:"dispatchState"`
	} `json:"content"`
}

// SearchResult represents a search result
type SearchResult struct {
	Results []map[string]interface{} `json:"results"`
}

// SavedSearch represents a saved search
type SavedSearch struct {
	Name        string `json:"name"`
	Search      string `json:"search"`
	Description string `json:"description"`
	CronSchedule string `json:"cron_schedule"`
}

// Alert represents a Splunk alert
type Alert struct {
	Name         string `json:"name"`
	Search       string `json:"search"`
	Description  string `json:"description"`
	CronSchedule string `json:"cron_schedule"`
	Actions      string `json:"actions"`
}

// doRequest performs an HTTP request to the Splunk API
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// RunSearch creates and runs a search job
func (c *Client) RunSearch(ctx context.Context, searchQuery string, earliestTime, latestTime string) (string, error) {
	data := url.Values{}
	data.Set("search", searchQuery)
	data.Set("output_mode", "json")
	if earliestTime != "" {
		data.Set("earliest_time", earliestTime)
	}
	if latestTime != "" {
		data.Set("latest_time", latestTime)
	}

	resp, err := c.doRequest(ctx, "POST", "/services/search/jobs", strings.NewReader(data.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		SID string `json:"sid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.SID, nil
}

// GetSearchStatus gets the status of a search job
func (c *Client) GetSearchStatus(ctx context.Context, sid string) (*Search, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/services/search/jobs/%s?output_mode=json", sid), nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var search Search
	if err := json.NewDecoder(resp.Body).Decode(&search); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &search, nil
}

// GetSearchResults gets the results of a completed search job
func (c *Client) GetSearchResults(ctx context.Context, sid string, count int) (*SearchResult, error) {
	path := fmt.Sprintf("/services/search/jobs/%s/results?output_mode=json", sid)
	if count > 0 {
		path += fmt.Sprintf("&count=%d", count)
	}

	resp, err := c.doRequest(ctx, "GET", path, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListSavedSearches lists all saved searches
func (c *Client) ListSavedSearches(ctx context.Context) ([]SavedSearch, error) {
	resp, err := c.doRequest(ctx, "GET", "/services/saved/searches?output_mode=json&count=0", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Entry []struct {
			Name    string `json:"name"`
			Content struct {
				Search       string `json:"search"`
				Description  string `json:"description"`
				CronSchedule string `json:"cron_schedule"`
			} `json:"content"`
		} `json:"entry"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	searches := make([]SavedSearch, len(result.Entry))
	for i, entry := range result.Entry {
		searches[i] = SavedSearch{
			Name:         entry.Name,
			Search:       entry.Content.Search,
			Description:  entry.Content.Description,
			CronSchedule: entry.Content.CronSchedule,
		}
	}

	return searches, nil
}

// CreateSavedSearch creates a new saved search
func (c *Client) CreateSavedSearch(ctx context.Context, name, search, description string) error {
	data := url.Values{}
	data.Set("name", name)
	data.Set("search", search)
	data.Set("output_mode", "json")
	if description != "" {
		data.Set("description", description)
	}

	resp, err := c.doRequest(ctx, "POST", "/services/saved/searches", strings.NewReader(data.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// ListAlerts lists triggered alerts
func (c *Client) ListAlerts(ctx context.Context) ([]Alert, error) {
	resp, err := c.doRequest(ctx, "GET", "/services/saved/searches?output_mode=json&count=0&search=is_scheduled%3D1", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Entry []struct {
			Name    string `json:"name"`
			Content struct {
				Search       string `json:"search"`
				Description  string `json:"description"`
				CronSchedule string `json:"cron_schedule"`
				Actions      string `json:"actions"`
			} `json:"content"`
		} `json:"entry"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	alerts := make([]Alert, len(result.Entry))
	for i, entry := range result.Entry {
		alerts[i] = Alert{
			Name:         entry.Name,
			Search:       entry.Content.Search,
			Description:  entry.Content.Description,
			CronSchedule: entry.Content.CronSchedule,
			Actions:      entry.Content.Actions,
		}
	}

	return alerts, nil
}

// GetServerInfo gets Splunk server information
func (c *Client) GetServerInfo(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.doRequest(ctx, "GET", "/services/server/info?output_mode=json", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Entry []struct {
			Content map[string]interface{} `json:"content"`
		} `json:"entry"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Entry) > 0 {
		return result.Entry[0].Content, nil
	}

	return nil, fmt.Errorf("no server info found")
}

// SendEvent sends an event to Splunk via HTTP Event Collector
func (c *Client) SendEvent(ctx context.Context, index, source, sourcetype string, event map[string]interface{}) error {
	eventData := map[string]interface{}{
		"event": event,
		"time":  time.Now().Unix(),
	}
	if index != "" {
		eventData["index"] = index
	}
	if source != "" {
		eventData["source"] = source
	}
	if sourcetype != "" {
		eventData["sourcetype"] = sourcetype
	}

	jsonData, err := json.Marshal(eventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Note: HEC typically uses port 8088, but we'll use the management port for simplicity
	resp, err := c.doRequest(ctx, "POST", "/services/receivers/simple?output_mode=json", bytes.NewReader(jsonData), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
