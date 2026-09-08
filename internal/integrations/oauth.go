package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"net/http"
	"os"
)

type GoogleUser struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}
type GoogleOAuth struct {
	Config *oauth2.Config
	Client *http.Client
}

func NewGoogleOAuth() *GoogleOAuth {
	return &GoogleOAuth{Config: &oauth2.Config{ClientID: os.Getenv("GOOGLE_CLIENT_ID"), ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"), Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token"}, RedirectURL: os.Getenv("GOOGLE_REDIRECT_URL"), Scopes: []string{"openid", "email", "profile"}}, Client: http.DefaultClient}
}
func (g *GoogleOAuth) Exchange(ctx context.Context, code, redirect string) (GoogleUser, error) {
	cfg := *g.Config
	if redirect != "" {
		cfg.RedirectURL = redirect
	}
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
