package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html"
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

	httpClient = &http.Client{}
)

// MastodonResponse wraps the API response
type MastodonResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *string     `json:"error,omitempty"`
}

// Account represents a Mastodon user account
type Account struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// Status represents a Mastodon post
type Status struct {
	ID              string  `json:"id"`
	Content         string  `json:"content"`
	CreatedAt       string  `json:"created_at"`
	URL             string  `json:"url"`
	RepliesCount    int     `json:"replies_count"`
	ReblogsCount    int     `json:"reblogs_count"`
	FavouritesCount int     `json:"favourites_count"`
	Account         Account `json:"account"`
	Reblog          *Status `json:"reblog"`
}

// Notification represents a Mastodon notification
type Notification struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	CreatedAt string  `json:"created_at"`
	Account   Account `json:"account"`
	Status    *Status `json:"status"`
}

// SearchResult represents the response from /api/v2/search
type SearchResult struct {
	Statuses []Status `json:"statuses"`
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

	token := os.Getenv("MASTODON_TOKEN")
	if token == "" {
		outputError("MASTODON_TOKEN environment variable not set")
		os.Exit(1)
	}

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
			outputError("search command requires a query argument")
			os.Exit(1)
		}
		data, err = searchPosts(ctx, token, args[1])
	default:
		outputError(fmt.Sprintf("unknown command: %s", command))
		os.Exit(1)
	}

	if err != nil {
		outputError(err.Error())
		os.Exit(1)
	}

	if *flagJSON {
		output, err := json.Marshal(MastodonResponse{Success: true, Data: data})
		if err != nil {
			outputError(fmt.Sprintf("marshaling response: %v", err))
			os.Exit(1)
		}
		fmt.Println(string(output))
	} else {
		formatText(command, data)
	}
}

func outputError(msg string) {
	response := MastodonResponse{Success: false, Error: &msg}
	output, _ := json.Marshal(response)
	fmt.Println(string(output))
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
}

func makeRequest(ctx context.Context, token, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *flagInstanceURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
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
	body, err := makeRequest(ctx, token, fmt.Sprintf("/api/v1/timelines/home?limit=%d", *flagLimit))
	if err != nil {
		return nil, err
	}
	var statuses []Status
	if err := json.Unmarshal(body, &statuses); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return statuses, nil
}

func getUserTweets(ctx context.Context, token string) (interface{}, error) {
	body, err := makeRequest(ctx, token, "/api/v1/accounts/verify_credentials")
	if err != nil {
		return nil, err
	}
	var account Account
	if err := json.Unmarshal(body, &account); err != nil {
		return nil, fmt.Errorf("parsing account: %w", err)
	}
	if account.ID == "" {
		return nil, fmt.Errorf("account ID not found")
	}

	body, err = makeRequest(ctx, token, fmt.Sprintf("/api/v1/accounts/%s/statuses?limit=%d", account.ID, *flagLimit))
	if err != nil {
		return nil, err
	}
	var statuses []Status
	if err := json.Unmarshal(body, &statuses); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return statuses, nil
}

func getMentions(ctx context.Context, token string) (interface{}, error) {
	body, err := makeRequest(ctx, token, fmt.Sprintf("/api/v1/notifications?limit=%d&types[]=mention", *flagLimit))
	if err != nil {
		return nil, err
	}
	var notifications []Notification
	if err := json.Unmarshal(body, &notifications); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return notifications, nil
}

func searchPosts(ctx context.Context, token, query string) (interface{}, error) {
	body, err := makeRequest(ctx, token, fmt.Sprintf("/api/v2/search?q=%s&type=statuses&limit=%d",
		url.QueryEscape(query), *flagLimit))
	if err != nil {
		return nil, err
	}
	var result SearchResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return result, nil
}

func formatText(command string, data interface{}) {
	switch command {
	case "home", "user-tweets":
		statuses, ok := data.([]Status)
		if !ok {
			fmt.Println("Error: unexpected data format")
			return
		}
		formatStatuses(statuses)
	case "mentions":
		notifications, ok := data.([]Notification)
		if !ok {
			fmt.Println("Error: unexpected data format")
			return
		}
		formatMentions(notifications)
	case "search":
		result, ok := data.(SearchResult)
		if !ok {
			fmt.Println("Error: unexpected data format")
			return
		}
		formatStatuses(result.Statuses)
	}
}

// resolvePost returns the displayable post and the booster's username (if it's a boost).
func resolvePost(s Status) (post Status, boostedBy string) {
	if s.Reblog != nil {
		return *s.Reblog, s.Account.Username
	}
	return s, ""
}

func formatStatuses(statuses []Status) {
	if len(statuses) == 0 {
		fmt.Println("No posts found.")
		return
	}
	for i, s := range statuses {
		post, boostedBy := resolvePost(s)
		fmt.Printf("--- Post %d ---\n", i+1)
		if boostedBy != "" {
			fmt.Printf("🔁 @%s boosted\n", boostedBy)
		}
		fmt.Printf("@%s (%s)\n", post.Account.Username, post.Account.DisplayName)
		fmt.Printf("%s\n", post.CreatedAt)
		fmt.Printf("\n%s\n\n", stripHTML(post.Content))
		fmt.Printf("💬 %d  🔁 %d  ⭐ %d\n", post.RepliesCount, post.ReblogsCount, post.FavouritesCount)
		fmt.Printf("🔗 %s\n\n", post.URL)
	}
}

func formatMentions(notifications []Notification) {
	if len(notifications) == 0 {
		fmt.Println("No mentions found.")
		return
	}
	for i, n := range notifications {
		fmt.Printf("--- Mention %d ---\n", i+1)
		fmt.Printf("@%s (%s) mentioned you\n", n.Account.Username, n.Account.DisplayName)
		fmt.Printf("%s\n", n.CreatedAt)
		if n.Status != nil {
			fmt.Printf("\n%s\n\n", stripHTML(n.Status.Content))
		}
	}
}

// stripHTML converts block-level tags to newlines, strips all remaining tags,
// and decodes HTML entities.
func stripHTML(s string) string {
	// Convert block-level tags to newlines before stripping
	s = strings.ReplaceAll(s, "</p><p>", "\n\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")

	// Strip all remaining tags
	var b strings.Builder
	inTag := false
	for _, ch := range s {
		switch {
		case ch == '<':
			inTag = true
		case ch == '>':
			inTag = false
		case !inTag:
			b.WriteRune(ch)
		}
	}

	return html.UnescapeString(b.String())
}
