package main

import (
	"context"
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
