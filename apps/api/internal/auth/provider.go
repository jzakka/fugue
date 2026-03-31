package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// UserProfile is the normalized profile extracted from an OAuth provider.
type UserProfile struct {
	ProviderID    string
	Email         string
	EmailVerified bool
	Nickname      string
	AvatarURL     string
	Bio           string
	RawProfile    json.RawMessage
}

// Provider defines the interface for an OAuth provider.
type Provider interface {
	Name() string
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
	FetchProfile(ctx context.Context, token *oauth2.Token) (*UserProfile, error)
}

// --- Google ---

type GoogleProvider struct {
	config *oauth2.Config
}

func NewGoogleProvider(clientID, clientSecret, callbackURL string) *GoogleProvider {
	return &GoogleProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  callbackURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

func (p *GoogleProvider) Name() string { return "google" }

func (p *GoogleProvider) AuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (p *GoogleProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.config.Exchange(ctx, code)
}

func (p *GoogleProvider) FetchProfile(ctx context.Context, token *oauth2.Token) (*UserProfile, error) {
	client := p.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("google userinfo request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read google response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo returned %d: %s", resp.StatusCode, body)
	}

	var info struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parse google profile: %w", err)
	}

	return &UserProfile{
		ProviderID:    info.ID,
		Email:         info.Email,
		EmailVerified: info.VerifiedEmail,
		Nickname:      info.Name,
		AvatarURL:     info.Picture,
		RawProfile:    body,
	}, nil
}

// --- Discord ---

var discordEndpoint = oauth2.Endpoint{
	AuthURL:  "https://discord.com/api/oauth2/authorize",
	TokenURL: "https://discord.com/api/oauth2/token",
}

type DiscordProvider struct {
	config *oauth2.Config
}

func NewDiscordProvider(clientID, clientSecret, callbackURL string) *DiscordProvider {
	return &DiscordProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  callbackURL,
			Scopes:       []string{"identify", "email"},
			Endpoint:     discordEndpoint,
		},
	}
}

func (p *DiscordProvider) Name() string { return "discord" }

func (p *DiscordProvider) AuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *DiscordProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.config.Exchange(ctx, code)
}

func (p *DiscordProvider) FetchProfile(ctx context.Context, token *oauth2.Token) (*UserProfile, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://discord.com/api/v10/users/@me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discord user request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read discord response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord user returned %d: %s", resp.StatusCode, body)
	}

	var info struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
		Email    string `json:"email"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parse discord profile: %w", err)
	}

	avatarURL := ""
	if info.Avatar != "" {
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", info.ID, info.Avatar)
	}

	return &UserProfile{
		ProviderID:    info.ID,
		Email:         info.Email,
		EmailVerified: info.Verified,
		Nickname:      info.Username,
		AvatarURL:     avatarURL,
		RawProfile:    body,
	}, nil
}
