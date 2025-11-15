package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kitproj/splunk-cli/internal/config"
	"github.com/kitproj/splunk-cli/internal/splunk"
	"golang.org/x/term"
)

var (
	host   string
	token  string
	client *splunk.Client
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprintf(w, "Usage:")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  splunk configure <host> - Configure Splunk host and token (reads token from stdin)")
		fmt.Fprintln(w, "  splunk search <query> [earliest-time] [latest-time] - Run a Splunk search query")
		fmt.Fprintln(w, "  splunk mcp-server - Start MCP server (stdio transport)")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Options:")
		flag.PrintDefaults()
	}
	flag.Parse()

	if err := run(ctx, flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: splunk <command> [args...]")
	}

	// First argument is the command
	command := args[0]

	switch command {
	case "configure":
		if len(args) < 2 {
			return fmt.Errorf("usage: splunk configure <host>")
		}
		return configure(args[1])
	case "search":
		if len(args) < 2 {
			return fmt.Errorf("usage: splunk search <query> [earliest-time] [latest-time]")
		}
		query := args[1]
		var earliestTime, latestTime string
		if len(args) >= 3 {
			earliestTime = args[2]
		}
		if len(args) >= 4 {
			latestTime = args[3]
		}
		return executeCommand(ctx, func(ctx context.Context) error {
			return runSearch(ctx, query, earliestTime, latestTime)
		})
	case "mcp-server":
		return runMCPServer(ctx)
	default:
		return fmt.Errorf("unknown sub-command: %s", command)
	}
}

func executeCommand(ctx context.Context, fn func(context.Context) error) error {
	// Load host from config file, or fall back to env var
	if host == "" {
		var err error
		host, err = config.LoadConfig()
		if err != nil {
			// Fall back to environment variable
			host = os.Getenv("SPLUNK_HOST")
		}
	}

	// Load token from keyring, or fall back to env var
	if token == "" {
		token = os.Getenv("SPLUNK_TOKEN")
	}
	if token == "" {
		var err error
		token, err = config.LoadToken(host)
		if err != nil {
			return err
		}
	}

	if host == "" {
		return fmt.Errorf("host is required")
	}
	if token == "" {
		return fmt.Errorf("token is required")
	}

	client = splunk.NewClient(host, token)
	return fn(ctx)
}

func runSearch(ctx context.Context, query string, earliestTime, latestTime string) error {
	// Ensure query starts with "search" if not already present
	if !strings.HasPrefix(strings.TrimSpace(query), "search") && !strings.HasPrefix(strings.TrimSpace(query), "|") {
		query = "search " + query
	}

	fmt.Printf("Running search: %s\n", query)

	// Create search job
	sid, err := client.RunSearch(ctx, query, earliestTime, latestTime)
	if err != nil {
		return fmt.Errorf("failed to run search: %w", err)
	}

	fmt.Printf("Search job created: %s\n", sid)

	// Poll for completion
	for {
		status, err := client.GetSearchStatus(ctx, sid)
		if err != nil {
			return fmt.Errorf("failed to get search status: %w", err)
		}

		if status.Content.IsDone {
			fmt.Printf("Search completed. Found %d results.\n\n", status.Content.ResultCount)
			break
		}

		fmt.Printf("Search in progress (%s)...\n", status.Content.DispatchState)
		time.Sleep(2 * time.Second)
	}

	// Get results
	results, err := client.GetSearchResults(ctx, sid, 100)
	if err != nil {
		return fmt.Errorf("failed to get search results: %w", err)
	}

	// Display results
	for i, result := range results.Results {
		fmt.Printf("Result %d:\n", i+1)
		for key, value := range result {
			fmt.Printf("  %s: %v\n", key, value)
		}
		fmt.Println()
	}

	return nil
}

// configure reads the token from stdin and saves it to the keyring
func configure(host string) error {
	if host == "" {
		return fmt.Errorf("host is required")
	}

	fmt.Fprintf(os.Stderr, "To create an authentication token in Splunk:\n")
	fmt.Fprintf(os.Stderr, "1. Log in to your Splunk instance at https://%s:8000\n", host)
	fmt.Fprintf(os.Stderr, "2. Go to Settings > Tokens\n")
	fmt.Fprintf(os.Stderr, "3. Click 'New Token' and generate a token\n")
	fmt.Fprintf(os.Stderr, "The token will be stored securely in your system's keyring.\n")
	fmt.Fprintf(os.Stderr, "\nEnter Splunk API token: ")

	// Read password with hidden input
	tokenBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr) // Print newline after hidden input
	if err != nil {
		return fmt.Errorf("failed to read token: %w", err)
	}

	token := string(tokenBytes)
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	// Save host to config file
	if err := config.SaveConfig(host); err != nil {
		return err
	}

	// Save token to keyring
	if err := config.SaveToken(host, token); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Configuration saved successfully for host: %s\n", host)
	return nil
}
