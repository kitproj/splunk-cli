package main

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestSearchHandlerRequiresQuery(t *testing.T) {
	// Create a mock request without a query parameter
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "search",
			Arguments: map[string]interface{}{
				// Missing "query" parameter
			},
		},
	}

	result, err := searchHandler(context.Background(), nil, request)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error result when query is missing")
	}
}
