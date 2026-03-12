// Package codex provides OpenAI Codex OAuth authentication.
package codex

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
	"strings"
	"time"

	"github.com/charmbracelet/swarmy/internal/oauth"
)

const (
	clientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	issuer   = "https://auth.openai.com"

	deviceUserCodeURL = issuer + "/api/accounts/deviceauth/usercode"
	deviceTokenURL    = issuer + "/api/accounts/deviceauth/token"
	tokenURL          = issuer + "/oauth/token"
	deviceCallbackURI = issuer + "/deviceauth/callback"

	oauthLoginURL = issuer + "/codex/device"

	oauthPollingSafetyMarginMS = 3000

	scope = "openid profile email offline_access"
)

// DeviceCode holds the response from the device authorization endpoint.
type DeviceCode struct {
	DeviceAuthID string `json:"device_auth_id"`
	UserCode     string `json:"user_code"`
	Interval     string `json:"interval"`
	LoginURL     string
}

// RequestDeviceCode initiates the device code flow with OpenAI Codex.
func RequestDeviceCode(ctx context.Context) (*DeviceCode, error) {
	body, err := json.Marshal(map[string]string{"client_id": clientID})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceUserCodeURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

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
	if !resp.ProtoAtLeast(1, 1) || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed: %s - %s", resp.Status, string(respBody))
	}

	var dc DeviceCode
	if err := json.Unmarshal(respBody, &dc); err != nil {
		return nil, err
	}
	dc.LoginURL = oauthLoginURL
	return &dc, nil
}

// PollForToken polls OpenAI for the access token after the user authorizes.
func PollForToken(ctx context.Context, dc *DeviceCode) (*oauth.Token, error) {
	interval := 5
	if dc.Interval != "" {
		if v, err := fmt.Sscanf(dc.Interval, "%d", &interval); v != 1 || err != nil {
			interval = 5
		}
	}
	if interval < 1 {
		interval = 1
	}

	ticker := time.NewTicker(time.Duration(interval)*time.Second + oauthPollingSafetyMarginMS*time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}

		body, err := json.Marshal(map[string]string{
			"device_auth_id": dc.DeviceAuthID,
			"user_code":      dc.UserCode,
		})
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceTokenURL, strings.NewReader(string(body)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		httpClient := &http.Client{Timeout: 30 * time.Second}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("token poll failed with unexpected status: %s", resp.Status)
		}

		var data struct {
			AuthorizationCode string `json:"authorization_code"`
			CodeVerifier      string `json:"code_verifier"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		return exchangeDeviceCode(ctx, data.AuthorizationCode, data.CodeVerifier)
	}
}

// exchangeDeviceCode exchanges the device authorization code for tokens.
func exchangeDeviceCode(ctx context.Context, code, codeVerifier string) (*oauth.Token, error) {
	params := url.Values{}
	params.Set("grant_type", "authorization_code")
	params.Set("code", code)
	params.Set("redirect_uri", deviceCallbackURI)
	params.Set("client_id", clientID)
	params.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
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

// RefreshToken refreshes an expired Codex access token using the refresh token.
func RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	params := url.Values{}
	params.Set("grant_type", "refresh_token")
	params.Set("refresh_token", refreshToken)
	params.Set("client_id", clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
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

// BrowserAuthorize starts a local HTTP callback server and returns the
// authorization URL for the browser-based PKCE flow.
type BrowserAuthorize struct {
	URL         string
	RedirectURI string
	verifier    string
	state       string
	resultCh    chan *oauth.Token
	errCh       chan error
	server      *http.Server
}

// StartBrowserAuth starts the local OAuth callback server and returns an
// authorization URL for the browser-based PKCE flow.
func StartBrowserAuth(ctx context.Context) (*BrowserAuthorize, error) {
	verifier, err := generateVerifier()
	if err != nil {
		return nil, err
	}
	challenge := generateChallenge(verifier)
	state, err := generateState()
	if err != nil {
		return nil, err
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/auth/callback", oauthPort)

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", scope)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("id_token_add_organizations", "true")
	params.Set("codex_cli_simplified_flow", "true")
	params.Set("state", state)
	params.Set("originator", "opencode")

	authURL := issuer + "/oauth/authorize?" + params.Encode()

	ba := &BrowserAuthorize{
		URL:         authURL,
		RedirectURI: redirectURI,
		verifier:    verifier,
		state:       state,
		resultCh:    make(chan *oauth.Token, 1),
		errCh:       make(chan error, 1),
	}
	ba.startServer(ctx)
	return ba, nil
}

const oauthPort = 1455

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

		token, err := exchangeBrowserCode(ctx, code, ba.verifier, ba.RedirectURI)
		if err != nil {
			ba.errCh <- err
			return
		}
		ba.resultCh <- token
	})

	ba.server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", oauthPort),
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

// exchangeBrowserCode exchanges the authorization code from the browser
// callback for tokens.
func exchangeBrowserCode(ctx context.Context, code, verifier, redirectURI string) (*oauth.Token, error) {
	params := url.Values{}
	params.Set("grant_type", "authorization_code")
	params.Set("code", code)
	params.Set("redirect_uri", redirectURI)
	params.Set("client_id", clientID)
	params.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
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

// generateVerifier generates a random PKCE code verifier.
func generateVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generateChallenge derives the S256 PKCE code challenge from a verifier.
func generateChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// generateState generates a random state parameter for CSRF protection.
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
