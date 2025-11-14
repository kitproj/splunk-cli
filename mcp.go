package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kitproj/splunk-cli/internal/config"
	"github.com/kitproj/splunk-cli/internal/splunk"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// runMCPServer starts the MCP server that communicates over stdio using the mcp-go library
func runMCPServer(ctx context.Context) error {
	// Load host from config file
	host, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("Splunk host must be configured (use 'splunk configure <host>' or set SPLUNK_HOST env var)")
	}

	// Load token from keyring
	token, err := config.LoadToken(host)
	if err != nil {
		return fmt.Errorf("Splunk token must be set (use 'splunk configure <host>' or set SPLUNK_TOKEN env var)")
	}

	if host == "" {
		return fmt.Errorf("Splunk host must be configured (use 'splunk configure <host>')")
	}
	if token == "" {
		return fmt.Errorf("Splunk token must be set (use 'splunk configure <host>')")
	}

	api := splunk.NewClient(host, token)

	// Create a new MCP server
	s := server.NewMCPServer(
		"splunk-cli-mcp-server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Add search tool
	searchTool := mcp.NewTool("search",
		mcp.WithDescription("Run a Splunk search query and return results"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("SPL (Search Processing Language) query to execute"),
		),
		mcp.WithString("earliest_time",
			mcp.Description("Earliest time for search (e.g., '-1h', '-24h', '2024-01-01T00:00:00')"),
		),
		mcp.WithString("latest_time",
			mcp.Description("Latest time for search (e.g., 'now', '2024-01-01T23:59:59')"),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum number of results to return (default: 100)"),
		),
	)
	s.AddTool(searchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return searchHandler(ctx, api, request)
	})

	// Add list-saved-searches tool
	listSavedSearchesTool := mcp.NewTool("list_saved_searches",
		mcp.WithDescription("List all saved searches in Splunk"),
	)
	s.AddTool(listSavedSearchesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return listSavedSearchesHandler(ctx, api, request)
	})

	// Add create-saved-search tool
	createSavedSearchTool := mcp.NewTool("create_saved_search",
		mcp.WithDescription("Create a new saved search in Splunk"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the saved search"),
		),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("SPL search query"),
		),
		mcp.WithString("description",
			mcp.Description("Optional description of the saved search"),
		),
	)
	s.AddTool(createSavedSearchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return createSavedSearchHandler(ctx, api, request)
	})

	// Add list-alerts tool
	listAlertsTool := mcp.NewTool("list_alerts",
		mcp.WithDescription("List all scheduled alerts in Splunk"),
	)
	s.AddTool(listAlertsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return listAlertsHandler(ctx, api, request)
	})

	// Add server-info tool
	serverInfoTool := mcp.NewTool("server_info",
		mcp.WithDescription("Get Splunk server information including version, OS, and configuration"),
	)
	s.AddTool(serverInfoTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return serverInfoHandler(ctx, api, request)
	})

	// Add send-event tool
	sendEventTool := mcp.NewTool("send_event",
		mcp.WithDescription("Send an event to Splunk via HTTP Event Collector"),
		mcp.WithString("index",
			mcp.Required(),
			mcp.Description("Target index for the event"),
		),
		mcp.WithString("source",
			mcp.Description("Source field for the event"),
		),
		mcp.WithString("sourcetype",
			mcp.Description("Sourcetype field for the event"),
		),
		mcp.WithString("event",
			mcp.Required(),
			mcp.Description("Event data as JSON string"),
		),
	)
	s.AddTool(sendEventTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return sendEventHandler(ctx, api, request)
	})

	// Start the stdio server
	return server.ServeStdio(s)
}

func searchHandler(ctx context.Context, client *splunk.Client, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid 'query' argument: %v", err)), nil
	}

	earliestTime := request.GetString("earliest_time", "")
	latestTime := request.GetString("latest_time", "")
	maxResults := request.GetInt("max_results", 100)

	// Ensure query starts with "search" if not already present
	if !strings.HasPrefix(strings.TrimSpace(query), "search") && !strings.HasPrefix(strings.TrimSpace(query), "|") {
		query = "search " + query
	}

	// Create search job
	sid, err := client.RunSearch(ctx, query, earliestTime, latestTime)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to run search: %v", err)), nil
	}

	// Poll for completion (with timeout)
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return mcp.NewToolResultError("Search timed out after 60 seconds"), nil
		case <-ticker.C:
			status, err := client.GetSearchStatus(ctx, sid)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to get search status: %v", err)), nil
			}

			if status.Content.IsDone {
				// Get results
				results, err := client.GetSearchResults(ctx, sid, maxResults)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to get search results: %v", err)), nil
				}

				// Format results as text
				var output strings.Builder
				output.WriteString(fmt.Sprintf("Search completed. Found %d result(s).\n\n", status.Content.ResultCount))

				for i, result := range results.Results {
					output.WriteString(fmt.Sprintf("Result %d:\n", i+1))
					for key, value := range result {
						output.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
					}
					output.WriteString("\n")
				}

				return mcp.NewToolResultText(output.String()), nil
			}
		}
	}
}

func listSavedSearchesHandler(ctx context.Context, client *splunk.Client, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	searches, err := client.ListSavedSearches(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list saved searches: %v", err)), nil
	}

	if len(searches) == 0 {
		return mcp.NewToolResultText("No saved searches found"), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d saved search(es):\n\n", len(searches)))

	for _, search := range searches {
		output.WriteString(fmt.Sprintf("Name: %s\n", search.Name))
		output.WriteString(fmt.Sprintf("Search: %s\n", search.Search))
		if search.Description != "" {
			output.WriteString(fmt.Sprintf("Description: %s\n", search.Description))
		}
		if search.CronSchedule != "" {
			output.WriteString(fmt.Sprintf("Schedule: %s\n", search.CronSchedule))
		}
		output.WriteString("---\n")
	}

	return mcp.NewToolResultText(output.String()), nil
}

func createSavedSearchHandler(ctx context.Context, client *splunk.Client, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid 'name' argument: %v", err)), nil
	}

	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid 'query' argument: %v", err)), nil
	}

	description := request.GetString("description", "")

	err = client.CreateSavedSearch(ctx, name, query, description)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create saved search: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully created saved search: %s", name)), nil
}

func listAlertsHandler(ctx context.Context, client *splunk.Client, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	alerts, err := client.ListAlerts(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list alerts: %v", err)), nil
	}

	if len(alerts) == 0 {
		return mcp.NewToolResultText("No scheduled alerts found"), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d alert(s):\n\n", len(alerts)))

	for _, alert := range alerts {
		output.WriteString(fmt.Sprintf("Name: %s\n", alert.Name))
		output.WriteString(fmt.Sprintf("Search: %s\n", alert.Search))
		if alert.Description != "" {
			output.WriteString(fmt.Sprintf("Description: %s\n", alert.Description))
		}
		if alert.CronSchedule != "" {
			output.WriteString(fmt.Sprintf("Schedule: %s\n", alert.CronSchedule))
		}
		if alert.Actions != "" {
			output.WriteString(fmt.Sprintf("Actions: %s\n", alert.Actions))
		}
		output.WriteString("---\n")
	}

	return mcp.NewToolResultText(output.String()), nil
}

func serverInfoHandler(ctx context.Context, client *splunk.Client, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	info, err := client.GetServerInfo(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get server info: %v", err)), nil
	}

	var output strings.Builder
	output.WriteString("Splunk Server Information:\n")
	for key, value := range info {
		output.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
	}

	return mcp.NewToolResultText(output.String()), nil
}

func sendEventHandler(ctx context.Context, client *splunk.Client, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	index, err := request.RequireString("index")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid 'index' argument: %v", err)), nil
	}

	eventJSON, err := request.RequireString("event")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid 'event' argument: %v", err)), nil
	}

	source := request.GetString("source", "")
	sourcetype := request.GetString("sourcetype", "")

	var event map[string]interface{}
	if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse event JSON: %v", err)), nil
	}

	err = client.SendEvent(ctx, index, source, sourcetype, event)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to send event: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully sent event to index: %s", index)), nil
}
