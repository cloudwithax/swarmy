// Package gitlab provides GitLab OAuth authentication.
package gitlab

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cloudwithax/swarmy/internal/oauth"
)

// IMPORTANT: The bundled client ID below is from gitlab-vscode-extension and is
// registered with redirect URI: vscode://gitlab.gitlab-workflow/authentication
// This will NOT work with a local HTTP callback server.
// To fix: Set GITLAB_OAUTH_CLIENT_ID environment variable with your own client
// ID. See https://docs.gitlab.com/ee/integration/oauth_provider.html for
// instructions on registering a new OAuth application.
const (
	bundledClientID = "1d89f9fdb23ee96d4e603201f6861dab6e143c5c3c00469a018a2d94bdc03d4e"
	defaultInstance = "https://gitlab.com"
	scopes          = "api"
	callbackPort    = 8080
)

// ClientID returns the OAuth client ID, preferring the GITLAB_OAUTH_CLIENT_ID
// environment variable over the bundled default.
func ClientID() string {
	if id := os.Getenv("GITLAB_OAUTH_CLIENT_ID"); id != "" {
		return id
	}
	return bundledClientID
}

// BrowserAuthorize holds state for the browser-based OAuth flow.
type BrowserAuthorize struct {
	URL         string
	RedirectURI string
	InstanceURL string
	verifier    string
	state       string
	resultCh    chan *oauth.Token
	errCh       chan error
	server      *http.Server
}

// StartBrowserAuth starts a local OAuth callback server and returns the
// authorization URL for the browser-based PKCE flow.
func StartBrowserAuth(ctx context.Context, instanceURL string) (*BrowserAuthorize, error) {
	if instanceURL == "" {
		instanceURL = os.Getenv("GITLAB_INSTANCE_URL")
	}
	if instanceURL == "" {
		instanceURL = defaultInstance
	}

	// Normalize instance URL (strip trailing slash).
	parsed, err := url.Parse(instanceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid GitLab instance URL: %w", err)
	}
	instanceURL = fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	verifier, err := generateVerifier(43)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE verifier: %w", err)
	}
	challenge := generateChallenge(verifier)
	state, err := generateVerifier(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/auth/callback", callbackPort)

	params := url.Values{}
	params.Set("client_id", ClientID())
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("state", state)
	params.Set("scope", scopes)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")

	authURL := strings.TrimRight(instanceURL, "/") + "/oauth/authorize?" + params.Encode()

	ba := &BrowserAuthorize{
		URL:         authURL,
		RedirectURI: redirectURI,
		InstanceURL: instanceURL,
		verifier:    verifier,
		state:       state,
		resultCh:    make(chan *oauth.Token, 1),
		errCh:       make(chan error, 1),
	}
	ba.startServer(ctx)
	return ba, nil
}

func (ba *BrowserAuthorize) startServer(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errMsg := q.Get("error"); errMsg != "" {
			desc := q.Get("error_description")
			if desc == "" {
				desc = errMsg
			}
			http.Error(w, desc, http.StatusBadRequest)
			ba.errCh <- fmt.Errorf("oauth error: %s", desc)
			return
		}

		code := q.Get("code")
		gotState := q.Get("state")
		if code == "" {
			http.Error(w, "missing authorization code", http.StatusBadRequest)
			ba.errCh <- fmt.Errorf("missing authorization code")
			return
		}
		if gotState != ba.state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			ba.errCh <- fmt.Errorf("state mismatch: potential CSRF attack")
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html><head><title>Authorization Successful</title></head><body><h1>Authorization Successful</h1><p>You can close this window and return to Swarmy.</p><script>setTimeout(()=>window.close(),2000)</script></body></html>`)

		token, err := ExchangeCode(ctx, ba.InstanceURL, code, ba.verifier, ba.RedirectURI)
		if err != nil {
			ba.errCh <- err
			return
		}
		ba.resultCh <- token
	})

	ba.server = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", callbackPort),
		Handler: mux,
	}
	go ba.server.ListenAndServe() //nolint:errcheck
}

// Wait waits for the OAuth callback result.
func (ba *BrowserAuthorize) Wait(ctx context.Context) (*oauth.Token, error) {
	defer ba.server.Close()
	select {
	case token := <-ba.resultCh:
		return token, nil
	case err := <-ba.errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ExchangeCode exchanges an authorization code for tokens.
func ExchangeCode(ctx context.Context, instanceURL, code, codeVerifier, redirectURI string) (*oauth.Token, error) {
	tokenURL := strings.TrimRight(instanceURL, "/") + "/oauth/token"

	params := url.Values{}
	params.Set("client_id", ClientID())
	params.Set("code", code)
	params.Set("grant_type", "authorization_code")
	params.Set("redirect_uri", redirectURI)
	params.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %s - %s", resp.Status, string(respBody))
	}

	return parseTokenResponse(respBody)
}

// RefreshToken refreshes an expired GitLab access token.
func RefreshToken(ctx context.Context, instanceURL, refreshToken string) (*oauth.Token, error) {
	if instanceURL == "" {
		instanceURL = defaultInstance
	}
	tokenURL := strings.TrimRight(instanceURL, "/") + "/oauth/token"

	params := url.Values{}
	params.Set("client_id", ClientID())
	params.Set("refresh_token", refreshToken)
	params.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed: %s - %s", resp.Status, string(respBody))
	}

	return parseTokenResponse(respBody)
}

func parseTokenResponse(body []byte) (*oauth.Token, error) {
	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	token := &oauth.Token{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
	}
	token.SetExpiresAt()
	return token, nil
}

// generateVerifier generates a random PKCE code verifier of the given length.
func generateVerifier(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Use only URL-safe characters to match the TS implementation's charset.
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
	result := make([]byte, length)
	for i, v := range b {
		result[i] = chars[int(v)%len(chars)]
	}
	return string(result), nil
}

// generateChallenge derives the S256 PKCE code challenge from a verifier.
func generateChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
