package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"strings"
	"time"
)

type GoogleUser struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}
type GoogleOAuth struct {
	Config           *oauth2.Config
	Client           *http.Client
	AllowedRedirects map[string]struct{}
}

func NewGoogleOAuth() *GoogleOAuth {
	allowed := map[string]struct{}{}
	for _, value := range strings.Split(os.Getenv("GOOGLE_REDIRECT_URLS"), ",") {
		if value = strings.TrimSpace(value); value != "" {
			allowed[value] = struct{}{}
		}
	}
	if value := strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URL")); value != "" {
		allowed[value] = struct{}{}
	}
	return &GoogleOAuth{Config: &oauth2.Config{ClientID: os.Getenv("GOOGLE_CLIENT_ID"), ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"), Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token"}, RedirectURL: os.Getenv("GOOGLE_REDIRECT_URL"), Scopes: []string{"openid", "email", "profile"}}, Client: &http.Client{Timeout: 15 * time.Second}, AllowedRedirects: allowed}
}
func (g *GoogleOAuth) Exchange(ctx context.Context, code, redirect string) (GoogleUser, error) {
	cfg := *g.Config
	if redirect != "" {
		if _, ok := g.AllowedRedirects[redirect]; !ok {
			return GoogleUser{}, fmt.Errorf("redirect URI is not allowed")
		}
		cfg.RedirectURL = redirect
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, g.Client)
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return GoogleUser{}, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err := g.Client.Do(req)
	if err != nil {
		return GoogleUser{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return GoogleUser{}, fmt.Errorf("google userinfo status %d", resp.StatusCode)
	}
	var u GoogleUser
	err = json.NewDecoder(resp.Body).Decode(&u)
	return u, err
}
