# Mastodon Scout

Read-only Mastodon CLI that outputs raw JSON from the Mastodon API.

## Features

- **Read-only**: No posting, following, or mutations
- **Raw JSON output**: Unmodified API responses
- **OAuth authentication**: Bearer token via environment variable
- **Fast**: Single binary with minimal dependencies
- **Cross-platform**: macOS and Linux binaries

## Installation

### Build from source

```bash
make build          # Build for current platform
make build-linux    # Build for Linux
make build-all      # Build for both platforms
```

## Usage

### Setup

Set your Mastodon OAuth bearer token:

```bash
export MASTODON_TOKEN="your_token_here"
```

To obtain a token:
1. Log into your Mastodon instance
2. Go to Preferences → Development
3. Create a new application with `read` scope only
4. Copy the access token

### Commands

#### Home Timeline
```bash
./dist/mastodon-scout home
```

#### Your Posts
```bash
./dist/mastodon-scout user-tweets
```

#### Mentions
```bash
./dist/mastodon-scout mentions
```

#### Search
```bash
./dist/mastodon-scout search "golang"
```

### Flags

```bash
--instance <url>    # Mastodon instance URL (default: https://mastodon.social)
--limit <int>       # Number of items to return (default: 20)
--timeout <int>     # Timeout in seconds (default: 30)
```

### Examples

```bash
# Get home timeline from different instance
./dist/mastodon-scout --instance https://fosstodon.org home

# Get more results
./dist/mastodon-scout --limit 50 home

# Search with custom timeout
./dist/mastodon-scout --timeout 60 search "rust programming"
```

## Output Format

All commands return JSON:

```json
{
  "success": true,
  "data": [ /* Raw Mastodon API response */ ]
}
```

On error:

```json
{
  "success": false,
  "error": "error message"
}
```

## Requirements

- Go 1.21 or later
- Mastodon account with OAuth token

## License

MIT
