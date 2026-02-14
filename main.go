package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultInstanceURL = "https://mastodon.social"
	defaultTimeout     = 30
)

var (
	flagInstanceURL = flag.String("instance", defaultInstanceURL, "Mastodon instance URL")
	flagTimeout     = flag.Int("timeout", defaultTimeout, "Timeout in seconds")
	flagLimit       = flag.Int("limit", 20, "Number of items to return")
	flagJSON        = flag.Bool("json", false, "Output in JSON format")
)

// MastodonResponse wraps the API response
type MastodonResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *string     `json:"error,omitempty"`
}

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mastodon-scout <command> [args]")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  home              Get home timeline")
		fmt.Fprintln(os.Stderr, "  user-tweets       Get user's tweets")
		fmt.Fprintln(os.Stderr, "  mentions          Get mentions")
		fmt.Fprintln(os.Stderr, "  search <query>    Search for posts")
		os.Exit(1)
	}

	// Get bearer token from environment
	token := os.Getenv("MASTODON_TOKEN")
	if token == "" {
		errMsg := "MASTODON_TOKEN environment variable not set"
		outputError(errMsg)
		os.Exit(1)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*flagTimeout)*time.Second)
	defer cancel()

	command := args[0]
	var data interface{}
	var err error

	switch command {
	case "home":
		data, err = getHomeTimeline(ctx, token)
	case "user-tweets":
		data, err = getUserTweets(ctx, token)
	case "mentions":
		data, err = getMentions(ctx, token)
	case "search":
		if len(args) < 2 {
			errMsg := "search command requires a query argument"
			outputError(errMsg)
			os.Exit(1)
		}
		query := args[1]
		data, err = searchPosts(ctx, token, query)
	default:
		errMsg := fmt.Sprintf("unknown command: %s", command)
		outputError(errMsg)
		os.Exit(1)
	}

	if err != nil {
		outputError(err.Error())
		os.Exit(1)
	}

	// Output based on format flag
	if *flagJSON {
		// Output JSON
		response := MastodonResponse{
			Success: true,
			Data:    data,
		}

		output, err := json.Marshal(response)
		if err != nil {
			errMsg := fmt.Sprintf("Error marshaling response: %v", err)
			outputError(errMsg)
			os.Exit(1)
		}

		fmt.Println(string(output))
	} else {
		// Output human-readable text
		formatText(command, data)
	}
}

func outputError(msg string) {
	response := MastodonResponse{
		Success: false,
		Error:   &msg,
	}
	output, _ := json.Marshal(response)
	fmt.Println(string(output))
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
}

func makeRequest(ctx context.Context, token, endpoint string) ([]byte, error) {
	reqURL := *flagInstanceURL + endpoint

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func getHomeTimeline(ctx context.Context, token string) (interface{}, error) {
	endpoint := fmt.Sprintf("/api/v1/timelines/home?limit=%d", *flagLimit)
	body, err := makeRequest(ctx, token, endpoint)
	if err != nil {
		return nil, err
	}

	var timeline []map[string]interface{}
	if err := json.Unmarshal(body, &timeline); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return timeline, nil
}

func getUserTweets(ctx context.Context, token string) (interface{}, error) {
	// First get the authenticated user's ID
	body, err := makeRequest(ctx, token, "/api/v1/accounts/verify_credentials")
	if err != nil {
		return nil, err
	}

	var account map[string]interface{}
	if err := json.Unmarshal(body, &account); err != nil {
		return nil, fmt.Errorf("parsing account: %w", err)
	}

	accountID, ok := account["id"].(string)
	if !ok {
		return nil, fmt.Errorf("account ID not found")
	}

	// Get the user's statuses
	endpoint := fmt.Sprintf("/api/v1/accounts/%s/statuses?limit=%d", accountID, *flagLimit)
	body, err = makeRequest(ctx, token, endpoint)
	if err != nil {
		return nil, err
	}

	var statuses []map[string]interface{}
	if err := json.Unmarshal(body, &statuses); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return statuses, nil
}

func getMentions(ctx context.Context, token string) (interface{}, error) {
	endpoint := fmt.Sprintf("/api/v1/notifications?limit=%d&types[]=mention", *flagLimit)
	body, err := makeRequest(ctx, token, endpoint)
	if err != nil {
		return nil, err
	}

	var mentions []map[string]interface{}
	if err := json.Unmarshal(body, &mentions); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return mentions, nil
}

func searchPosts(ctx context.Context, token, query string) (interface{}, error) {
	endpoint := fmt.Sprintf("/api/v2/search?q=%s&type=statuses&limit=%d",
		url.QueryEscape(query), *flagLimit)
	body, err := makeRequest(ctx, token, endpoint)
	if err != nil {
		return nil, err
	}

	var searchResult map[string]interface{}
	if err := json.Unmarshal(body, &searchResult); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return searchResult, nil
}

// formatText outputs human-readable text for the given command and data
func formatText(command string, data interface{}) {
	switch command {
	case "home", "user-tweets":
		formatStatuses(data)
	case "mentions":
		formatMentions(data)
	case "search":
		formatSearchResults(data)
	default:
		fmt.Println("Unknown command format")
	}
}

// formatStatuses formats timeline/status data
func formatStatuses(data interface{}) {
	statuses, ok := data.([]map[string]interface{})
	if !ok {
		fmt.Println("Error: unexpected data format")
		return
	}

	if len(statuses) == 0 {
		fmt.Println("No posts found.")
		return
	}

	for i, status := range statuses {
		// Check if this is a boost (reblog)
		reblog, isReblog := status["reblog"].(map[string]interface{})
		var boostedBy string
		if isReblog {
			// Get the booster's info
			account, _ := status["account"].(map[string]interface{})
			boostedBy = getStringField(account, "username")
		}

		// Extract post info - use reblog content if boosted
		var content string
		var createdAt string
		var reblogsCount, favoritesCount, repliesCount float64
		var postURL string
		var postAccount map[string]interface{}

		if isReblog {
			content = getStringField(reblog, "content")
			createdAt = getStringField(reblog, "created_at")
			reblogsCount = getFloatField(reblog, "reblogs_count")
			favoritesCount = getFloatField(reblog, "favourites_count")
			repliesCount = getFloatField(reblog, "replies_count")
			postURL = getStringField(reblog, "url")
			postAccount, _ = reblog["account"].(map[string]interface{})
		} else {
			content = getStringField(status, "content")
			createdAt = getStringField(status, "created_at")
			reblogsCount = getFloatField(status, "reblogs_count")
			favoritesCount = getFloatField(status, "favourites_count")
			repliesCount = getFloatField(status, "replies_count")
			postURL = getStringField(status, "url")
			postAccount, _ = status["account"].(map[string]interface{})
		}

		// Extract account info (from original post for boosts)
		username := getStringField(postAccount, "username")
		displayName := getStringField(postAccount, "display_name")

		// Strip HTML tags from content
		content = stripHTML(content)

		// Print formatted post
		fmt.Printf("--- Post %d ---\n", i+1)
		if isReblog {
			fmt.Printf("🔁 @%s boosted\n", boostedBy)
		}
		fmt.Printf("@%s (%s)\n", username, displayName)
		fmt.Printf("%s\n", createdAt)
		fmt.Printf("\n%s\n\n", content)
		fmt.Printf("💬 %d  🔁 %d  ⭐ %d\n", int(repliesCount), int(reblogsCount), int(favoritesCount))
		fmt.Printf("🔗 %s\n\n", postURL)
	}
}

// formatMentions formats mentions/notifications data
func formatMentions(data interface{}) {
	mentions, ok := data.([]map[string]interface{})
	if !ok {
		fmt.Println("Error: unexpected data format")
		return
	}

	if len(mentions) == 0 {
		fmt.Println("No mentions found.")
		return
	}

	for i, mention := range mentions {
		// Extract account info
		account, _ := mention["account"].(map[string]interface{})
		username := getStringField(account, "username")
		displayName := getStringField(account, "display_name")

		// Extract status info if present
		status, _ := mention["status"].(map[string]interface{})
		content := getStringField(status, "content")
		createdAt := getStringField(mention, "created_at")

		// Strip HTML tags from content
		content = stripHTML(content)

		// Print formatted mention
		fmt.Printf("--- Mention %d ---\n", i+1)
		fmt.Printf("@%s (%s) mentioned you\n", username, displayName)
		fmt.Printf("%s\n", createdAt)
		fmt.Printf("\n%s\n\n", content)
	}
}

// formatSearchResults formats search results
func formatSearchResults(data interface{}) {
	searchResult, ok := data.(map[string]interface{})
	if !ok {
		fmt.Println("Error: unexpected data format")
		return
	}

	statuses, _ := searchResult["statuses"].([]interface{})
	if len(statuses) == 0 {
		fmt.Println("No posts found.")
		return
	}

	// Convert to the format expected by formatStatuses
	statusMaps := make([]map[string]interface{}, len(statuses))
	for i, s := range statuses {
		if sm, ok := s.(map[string]interface{}); ok {
			statusMaps[i] = sm
		}
	}

	formatStatuses(statusMaps)
}

// Helper functions
func getStringField(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getFloatField(m map[string]interface{}, key string) float64 {
	if m == nil {
		return 0
	}
	if val, ok := m[key].(float64); ok {
		return val
	}
	return 0
}

// stripHTML removes HTML tags from a string (simple regex-free approach)
func stripHTML(s string) string {
	var result string
	inTag := false
	for _, char := range s {
		if char == '<' {
			inTag = true
		} else if char == '>' {
			inTag = false
		} else if !inTag {
			result += string(char)
		}
	}
	// Replace HTML entities
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&quot;", "\"")
	result = strings.ReplaceAll(result, "&#39;", "'")
	result = strings.ReplaceAll(result, "<br>", "\n")
	result = strings.ReplaceAll(result, "<br/>", "\n")
	result = strings.ReplaceAll(result, "<br />", "\n")
	result = strings.ReplaceAll(result, "</p><p>", "\n\n")
	return result
}
