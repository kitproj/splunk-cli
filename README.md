# Splunk CLI & MCP Server

A Splunk CLI and MCP server that allows you and your coding agents to interact with Splunk. Inspired by the GitHub CLI and following the same concept as jira-cli, it aims to provide a simple and efficient way for humans and agents to interact with Splunk from the command line.

Being both a CLI and an MCP server means you get the best of both worlds. Agents can be directed to perform specific commands (e.g., `Run a search for errors in the last hour by running splunk search 'error' '-1h' 'now'`), or they can use the MCP server to interact with Splunk directly.

Like `jq`, it is a single tiny binary, without the overhead of installing a Node runtime, and without the need to put your Splunk token in plain text file (it uses the system key-ring).

## Installation

### Supported Platforms

Binaries are available for:
- **Linux**: amd64, arm64
- **macOS**: amd64 (Intel), arm64 (Apple Silicon)
- **Windows**: amd64

### Download and Install

Download the binary for your platform from the [release page](https://github.com/kitproj/splunk-cli/releases).

#### Linux

**For Linux (amd64):**
```bash
sudo curl -fsL -o /usr/local/bin/splunk https://github.com/kitproj/splunk-cli/releases/download/v0.0.1/splunk_v0.0.1_linux_amd64
sudo chmod +x /usr/local/bin/splunk
```

**For Linux (arm64):**
```bash
sudo curl -fsL -o /usr/local/bin/splunk https://github.com/kitproj/splunk-cli/releases/download/v0.0.1/splunk_v0.0.1_linux_arm64
sudo chmod +x /usr/local/bin/splunk
```

#### macOS

**For macOS (Apple Silicon/arm64):**
```bash
sudo curl -fsL -o /usr/local/bin/splunk https://github.com/kitproj/splunk-cli/releases/download/v0.0.1/splunk_v0.0.1_darwin_arm64
sudo chmod +x /usr/local/bin/splunk
```

**For macOS (Intel/amd64):**
```bash
sudo curl -fsL -o /usr/local/bin/splunk https://github.com/kitproj/splunk-cli/releases/download/v0.0.1/splunk_v0.0.1_darwin_amd64
sudo chmod +x /usr/local/bin/splunk
```

#### Verify Installation

After installing, verify the installation works:
```bash
splunk -h
```

## Usage

### Configuration

#### Getting a Splunk API Token

Before configuring, you'll need to create a Splunk authentication token:

1. Log in to your Splunk instance: `https://your-splunk-host:8000`
2. Go to Settings > Tokens
3. Click "New Token" or "Enable Token Authentication" if not already enabled
4. Generate and copy the token (you won't be able to see it again)

#### Configure the CLI

The `splunk` CLI can be configured in two ways:

1. **Using the configure command (recommended, secure)**:
   ```bash
   echo "your-api-token" | splunk configure your-splunk-host
   ```
   This stores the host in `~/.config/splunk-cli/config.json` and the token securely in your system's keyring.

2. **Using environment variables**:
   ```bash
   export SPLUNK_HOST=your-splunk-host
   export SPLUNK_TOKEN=your-api-token
   ```
   Note: The SPLUNK_TOKEN environment variable is still supported for backward compatibility, but using the keyring (via `splunk configure`) is more secure on multi-user systems.

## Usage

### Direct CLI Usage

```bash
Usage:
  splunk configure <host> - Configure Splunk host and token (reads token from stdin)
  splunk search <query> [earliest-time] [latest-time] - Run a Splunk search query
  splunk mcp-server - Start MCP server (stdio transport)
```

#### Examples

**Run a search:**
```bash
splunk search "error" "-1h" "now"
# Search for "error" in the last hour

splunk search "index=main sourcetype=access_combined | stats count by status"
# Search with SPL query
```

### MCP Server Mode

The MCP (Model Context Protocol) server allows AI assistants and other tools to interact with Splunk through a standardized JSON-RPC protocol over stdio. This enables seamless integration with AI coding assistants and other automation tools.

Learn more about MCP: https://modelcontextprotocol.io

**Setup:**

1. First, configure your Splunk host and token (stored securely in the system keyring):
   ```bash
   echo "your-api-token" | splunk configure your-splunk-host
   ```

2. Add the MCP server configuration to your MCP client (e.g., Claude Desktop, Cline):
   ```json
   {
     "mcpServers": {
       "splunk": {
         "command": "splunk",
         "args": ["mcp-server"]
       }
     }
   }
   ```

   For **Claude Desktop**, add this to:
   - macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - Windows: `%APPDATA%\Claude\claude_desktop_config.json`

The server exposes the following tool:
- `search` - Run a Splunk search query and return results

**Example usage from an AI assistant:**
> "Search Splunk for errors in the main index in the last hour and show me the top 10 results."

## Development

### Built With

This CLI uses the following Go libraries:
- **[github.com/mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)** - Model Context Protocol server library
- **[github.com/zalando/go-keyring](https://github.com/zalando/go-keyring)** - Cross-platform keyring library for secure token storage

The Splunk API client is a custom implementation using the Splunk REST API, as there is no official Go SDK for Splunk Enterprise.

### Building from Source

```bash
# Clone the repository
git clone https://github.com/kitproj/splunk-cli.git
cd splunk-cli

# Build the binary
go build -o splunk

# Run tests
go test ./...
```

### Project Structure

```
splunk-cli/
├── internal/
│   ├── config/      # Configuration management (host, token storage)
│   └── splunk/      # Splunk REST API client
├── main.go          # CLI entry point and command handlers
├── mcp.go           # MCP server implementation
├── mcp_test.go      # MCP server tests
└── README.md        # This file
```

## Troubleshooting

### Common Issues

**"Splunk host must be configured" error**
- Make sure you've run `splunk configure <host>` or set the `SPLUNK_HOST` environment variable
- Check that the config file exists: `cat ~/.config/splunk-cli/config.json`

**"Failed to execute request" or authentication errors**
- Verify your API token is still valid (tokens can expire)
- Re-run the configure command to update the token: `echo "new-token" | splunk configure your-splunk-host`
- Make sure your Splunk user has permission to access the requested resources

**Keyring issues on Linux**
- Some Linux systems may not have a keyring service installed
- Install `gnome-keyring` or `kwallet` for your desktop environment
- Alternatively, use environment variables: `export SPLUNK_TOKEN=your-token`

**MCP server not appearing in Claude Desktop**
- Restart Claude Desktop after editing the config file
- Check the config file syntax is valid JSON
- Verify the `splunk` binary is in your PATH: `which splunk`

### Getting Help

- Report issues: https://github.com/kitproj/splunk-cli/issues
- Check existing issues for solutions and workarounds

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
