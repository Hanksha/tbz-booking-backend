package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/patrickmn/go-cache"
)

type Message struct {
	Content string  `json:"content"`
	Embeds  []Embed `json:"embeds"`
}

type Embed struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Author    Author       `json:"author"`
	Fields    []EmbedField `json:"fields"`
	ChannelID string       `json:"channelId"`
	Content   string       `json:"content"`
}

type EmbedImage struct {
	URL string `json:"url"`
}

type Author struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	IconURL string `json:"icon_url"`
}

type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

type Member struct {
	User  User     `json:"user"`
	Roles []string `json:"roles"`
}

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type Event struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	StartTime   string `json:"scheduled_start_time"`
	EndTime     string `json:"scheduled_end_time"`
	Status      int    `json:"status"`
}

const baseURL = "https://discord.com/api/v10"

// ErrRateLimited is returned (wrapped) when a request is refused or fails due to Discord/Cloudflare
// rate limiting, so callers can distinguish it from other failures (e.g. invalid auth).
var ErrRateLimited = errors.New("discord rate limited")

type Client struct {
	token        string
	clientID     string
	clientSecret string
	redirectURI  string
	serverID     string
	client       *http.Client
	membersCache *cache.Cache
	eventsCache  *cache.Cache
	// blockedUntil is a unix timestamp (seconds); requests are refused until this time to
	// avoid extending a Discord/Cloudflare rate-limit block by retrying too soon.
	blockedUntil atomic.Int64
	// rateLimitStore optionally persists blockedUntil so it survives process restarts.
	rateLimitStore RateLimitStore
	loadStoreOnce  sync.Once
}

type DiscordClient interface {
	SendMessage(ctx context.Context, channelID string, message Message) error
	GetOAuth2Token(ctx context.Context, code string) (*OAuthToken, error)
	GetGuildMember(ctx context.Context, accessToken string) (*Member, error)
	SearchMembers(ctx context.Context, query string, limit int) ([]Member, error)
	GetEvents(ctx context.Context) ([]Event, error)
	GetDMChannel(ctx context.Context, userID string) (string, error)
}

func NewClient(token, clientID, clientSecret, redirectURI, serverID string) *Client {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	return &Client{
		token:        token,
		client:       client,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		serverID:     serverID,
		membersCache: cache.New(10*time.Minute, 20*time.Minute),
		eventsCache:  cache.New(10*time.Minute, 20*time.Minute),
	}
}

// SetRateLimitStore wires a persistence layer for the rate-limit backoff window so it
// survives process restarts. Must be called before the client is used.
func (c *Client) SetRateLimitStore(store RateLimitStore) {
	c.rateLimitStore = store
}

func (c *Client) SendMessage(ctx context.Context, channelID string, message Message) error {
	if err := c.checkRateLimit(ctx); err != nil {
		return err
	}
	if len(strings.TrimSpace(channelID)) == 0 {
		return errors.New("channelID cannot be empty")
	}
	msgURL, err := c.getURL("channels", channelID, "messages")

	if err != nil {
		return err
	}

	body, err := json.Marshal(message)

	if err != nil {
		return fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", msgURL, bytes.NewReader(body))

	if err != nil {
		return fmt.Errorf("failed create new request: %w", err)
	}

	c.setHeaders(req)

	res, err := c.client.Do(req)

	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		bodyBytes, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			return fmt.Errorf("request failed with status %d; also failed reading body: %w", res.StatusCode, readErr)
		}
		if res.StatusCode == http.StatusTooManyRequests {
			return c.handleTooManyRequests(ctx, res, bodyBytes)
		}
		return fmt.Errorf("request failed with status '%v' and body:\n%v", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) GetDMChannel(ctx context.Context, userID string) (string, error) {
	if err := c.checkRateLimit(ctx); err != nil {
		return "", err
	}
	if len(strings.TrimSpace(userID)) == 0 {
		return "", errors.New("userID cannot be empty")
	}
	msgURL, err := c.getURL("users", "@me", "channels")

	if err != nil {
		return "", err
	}

	body, err := json.Marshal(map[string]string{
		"recipient_id": userID,
	})

	if err != nil {
		return "", fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", msgURL, bytes.NewReader(body))

	if err != nil {
		return "", fmt.Errorf("failed create new request: %w", err)
	}

	c.setHeaders(req)

	res, err := c.client.Do(req)

	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		bodyBytes, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			return "", fmt.Errorf("request failed with status %d; also failed reading body: %w", res.StatusCode, readErr)
		}
		if res.StatusCode == http.StatusTooManyRequests {
			return "", c.handleTooManyRequests(ctx, res, bodyBytes)
		}
		return "", fmt.Errorf("request failed with status '%v' and body:\n%v", res.StatusCode, string(bodyBytes))
	}

	var dmChannel struct {
		ID string `json:"id"`
	}
	err = json.NewDecoder(res.Body).Decode(&dmChannel)
	if err != nil {
		return "", fmt.Errorf("failed to decode response body: %w", err)
	}

	return dmChannel.ID, nil
}

func (c *Client) GetOAuth2Token(ctx context.Context, code string) (*OAuthToken, error) {
	if err := c.checkRateLimit(ctx); err != nil {
		return nil, err
	}
	tokenURL, err := c.getURL("oauth2", "token")

	if err != nil {
		return nil, err
	}

	formValues := url.Values{
		"code":          {code},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"redirect_uri":  {c.redirectURI},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(formValues.Encode()))

	if err != nil {
		return nil, fmt.Errorf("failed create new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := c.client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer res.Body.Close()

	bodyBytes, readErr := io.ReadAll(res.Body)

	if res.StatusCode == http.StatusTooManyRequests {
		return nil, c.handleTooManyRequests(ctx, res, bodyBytes)
	}

	if res.StatusCode != http.StatusOK {
		if readErr != nil {
			return nil, fmt.Errorf("request failed with status %d; also failed reading body: %w", res.StatusCode, readErr)
		}
		return nil, fmt.Errorf("request failed with status '%v' and body:\n%v", res.StatusCode, string(bodyBytes))
	}

	if readErr != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var oauthToken = OAuthToken{}
	err = json.Unmarshal(bodyBytes, &oauthToken)

	if err != nil {
		return nil, fmt.Errorf("failed reading body: %w", err)
	}

	return &oauthToken, nil
}

func (c *Client) GetGuildMember(ctx context.Context, accessToken string) (*Member, error) {
	cachedMember, found := c.membersCache.Get(accessToken)

	if found {
		return cachedMember.(*Member), nil
	}

	if err := c.checkRateLimit(ctx); err != nil {
		return nil, err
	}

	memberURL, err := c.getURL("users", "@me", "guilds", c.serverID, "member")

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", memberURL, http.NoBody)

	if err != nil {
		return nil, fmt.Errorf("failed create new request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	res, err := c.client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer res.Body.Close()

	bodyBytes, readErr := io.ReadAll(res.Body)

	if res.StatusCode == http.StatusTooManyRequests {
		return nil, c.handleTooManyRequests(ctx, res, bodyBytes)
	}

	if res.StatusCode != http.StatusOK {
		if readErr != nil {
			return nil, fmt.Errorf("request failed with status %d; also failed reading body: %w", res.StatusCode, readErr)
		}
		return nil, fmt.Errorf("request failed with status '%v' and body:\n%v", res.StatusCode, string(bodyBytes))
	}

	if readErr != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var member = Member{}
	err = json.Unmarshal(bodyBytes, &member)

	if err != nil {
		return nil, fmt.Errorf("failed reading body: %w", err)
	}

	c.membersCache.Set(accessToken, &member, cache.DefaultExpiration)

	return &member, nil
}

func (c *Client) SearchMembers(ctx context.Context, query string, limit int) ([]Member, error) {
	query = strings.TrimSpace(query)
	cachedMembers, found := c.membersCache.Get(query)

	if found {
		return cachedMembers.([]Member), nil
	}

	if err := c.checkRateLimit(ctx); err != nil {
		return nil, err
	}

	searchURL, err := c.getURL("guilds", c.serverID, "members", "search")

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, http.NoBody)

	if err != nil {
		return nil, fmt.Errorf("failed create new request: %w", err)
	}

	q := req.URL.Query()
	q.Add("query", query)
	q.Add("limit", strconv.Itoa(limit))
	req.URL.RawQuery = q.Encode()

	c.setHeaders(req)

	res, err := c.client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer res.Body.Close()

	bodyBytes, readErr := io.ReadAll(res.Body)

	if res.StatusCode == http.StatusTooManyRequests {
		return nil, c.handleTooManyRequests(ctx, res, bodyBytes)
	}

	if res.StatusCode != http.StatusOK {
		if readErr != nil {
			return nil, fmt.Errorf("request failed with status %d; also failed reading body: %w", res.StatusCode, readErr)
		}
		return nil, fmt.Errorf("request failed with status '%v' and body:\n%v", res.StatusCode, string(bodyBytes))
	}

	if readErr != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var members = []Member{}
	err = json.Unmarshal(bodyBytes, &members)

	if err != nil {
		return nil, fmt.Errorf("failed reading body: %w", err)
	}

	c.membersCache.Set(query, members, cache.DefaultExpiration)

	return members, nil
}

func (c *Client) GetEvents(ctx context.Context) ([]Event, error) {
	cachedEvents, found := c.eventsCache.Get("events")

	if found {
		return cachedEvents.([]Event), nil
	}

	if err := c.checkRateLimit(ctx); err != nil {
		return nil, err
	}

	url, err := c.getURL("guilds", c.serverID, "scheduled-events")

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)

	if err != nil {
		return nil, fmt.Errorf("failed create new request: %w", err)
	}

	c.setHeaders(req)

	res, err := c.client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer res.Body.Close()

	bodyBytes, readErr := io.ReadAll(res.Body)

	if res.StatusCode == http.StatusTooManyRequests {
		return nil, c.handleTooManyRequests(ctx, res, bodyBytes)
	}

	if res.StatusCode != http.StatusOK {
		if readErr != nil {
			return nil, fmt.Errorf("request failed with status %d; also failed reading body: %w", res.StatusCode, readErr)
		}
		return nil, fmt.Errorf("request failed with status '%v' and body:\n%v", res.StatusCode, string(bodyBytes))
	}

	if readErr != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var events = []Event{}
	err = json.Unmarshal(bodyBytes, &events)

	if err != nil {
		return nil, fmt.Errorf("failed reading body: %w", err)
	}

	c.eventsCache.Set("events", events, cache.DefaultExpiration)

	return events, nil
}

// checkRateLimit returns an error without making a request if a previous 429 response set a
// backoff window that hasn't elapsed yet, to avoid extending the block by retrying too soon.
// On first use it lazily loads any persisted deadline so the guard survives process restarts.
func (c *Client) checkRateLimit(ctx context.Context) error {
	if c.rateLimitStore != nil {
		c.loadStoreOnce.Do(func() {
			until, err := c.rateLimitStore.GetBlockedUntil(ctx)
			if err != nil {
				slog.Error("failed to load discord rate limit state", "err", err)
				return
			}
			if !until.IsZero() {
				c.blockedUntil.Store(until.Unix())
			}
		})
	}

	if until := time.Unix(c.blockedUntil.Load(), 0); time.Now().Before(until) {
		return fmt.Errorf("%w: refusing request until %s", ErrRateLimited, until.Format(time.RFC3339))
	}
	return nil
}

// handleTooManyRequests records a backoff window from the response's retry-after info and logs
// diagnostics, returning an error describing the block. The deadline is persisted via
// rateLimitStore, if configured, so it survives process restarts.
func (c *Client) handleTooManyRequests(ctx context.Context, res *http.Response, body []byte) error {
	retryAfter := parseRetryAfter(res, body)
	until := time.Now().Add(retryAfter)
	c.blockedUntil.Store(until.Unix())
	if c.rateLimitStore != nil {
		if err := c.rateLimitStore.SetBlockedUntil(ctx, until); err != nil {
			slog.Error("failed to persist discord rate limit state", "err", err)
		}
	}
	logRateLimitHeaders(res, body)
	return fmt.Errorf("%w: backing off until %s", ErrRateLimited, until.Format(time.RFC3339))
}

// parseRetryAfter extracts how long to wait from the Retry-After header (seconds or HTTP-date)
// or, failing that, a "retry_after" field in the JSON body. Defaults to 60s if neither is present.
func parseRetryAfter(res *http.Response, body []byte) time.Duration {
	const defaultRetryAfter = 60 * time.Second

	if h := res.Header.Get("Retry-After"); h != "" {
		if secs, err := strconv.ParseFloat(h, 64); err == nil && secs > 0 {
			return time.Duration(secs * float64(time.Second))
		}
		if t, err := time.Parse(http.TimeFormat, h); err == nil {
			if d := time.Until(t); d > 0 {
				return d
			}
		}
	}

	var payload struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.RetryAfter > 0 {
		return time.Duration(payload.RetryAfter * float64(time.Second))
	}

	return defaultRetryAfter
}

// logRateLimitHeaders logs Discord's rate limit headers and body to identify global vs
// per-route/shared limits. Empty X-RateLimit-* fields with only Retry-After set usually
// indicate the 429 came from Cloudflare's edge (IP-level) rather than Discord's API itself.
func logRateLimitHeaders(res *http.Response, body []byte) {
	slog.Warn("discord rate limit hit",
		"limit", res.Header.Get("X-RateLimit-Limit"),
		"remaining", res.Header.Get("X-RateLimit-Remaining"),
		"reset", res.Header.Get("X-RateLimit-Reset"),
		"resetAfter", res.Header.Get("X-RateLimit-Reset-After"),
		"bucket", res.Header.Get("X-RateLimit-Bucket"),
		"global", res.Header.Get("X-RateLimit-Global"),
		"scope", res.Header.Get("X-RateLimit-Scope"),
		"retryAfter", res.Header.Get("Retry-After"),
		"server", res.Header.Get("Server"),
		"cfRay", res.Header.Get("Cf-Ray"),
		"allHeaders", res.Header,
		"body", string(body),
	)
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bot "+c.token)
}

func (c *Client) getURL(elem ...string) (string, error) {
	clientURL, err := url.JoinPath(baseURL, elem...)
	if err != nil {
		return "", fmt.Errorf("failed to create URL: %w", err)
	}

	return clientURL, nil
}
