package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

const baseURL = "https://discord.com/api/v10"

type Client struct {
	token        string
	clientID     string
	clientSecret string
	redirectURI  string
	serverID     string
	client       *http.Client
	cache        *cache.Cache
}

type DiscordClient interface {
	SendMessage(ctx context.Context, channelID string, message Message) error
	GetOAuth2Token(ctx context.Context, code string) (*OAuthToken, error)
	GetGuildMember(ctx context.Context, accessToken string) (*Member, error)
	SearchMembers(ctx context.Context, query string, limit int) ([]Member, error)
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
		cache:        cache.New(1*time.Minute, 5*time.Minute),
	}
}

func (c *Client) SendMessage(ctx context.Context, channelID string, message Message) error {
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
		return fmt.Errorf("request failed with status '%v' and body:\n%v", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) GetOAuth2Token(ctx context.Context, code string) (*OAuthToken, error) {
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
	cachedMember, found := c.cache.Get(accessToken)

	if found {
		return cachedMember.(*Member), nil
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

	c.cache.Set(accessToken, &member, cache.DefaultExpiration)

	return &member, nil
}

func (c *Client) SearchMembers(ctx context.Context, query string, limit int) ([]Member, error) {
	query = strings.TrimSpace(query)
	cachedMembers, found := c.cache.Get(query)

	if found {
		return cachedMembers.([]Member), nil
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

	c.cache.Set(query, members, cache.DefaultExpiration)

	return members, nil
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
