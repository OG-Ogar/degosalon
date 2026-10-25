// Package googleauth implements the Google OpenID Connect authorization-code
// flow used by the customer and barber sign-in buttons.
//
// It deliberately uses only the standard library. The application has no
// third-party Go dependencies for authentication, and adding one for this flow
// is not necessary: Google's token endpoint and userinfo endpoint are ordinary
// form/JSON HTTP APIs.
package googleauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// stateLifetime bounds how long a visitor may spend inside Google's
	// account chooser before the pending sign-in is discarded.
	stateLifetime = 10 * time.Minute

	requestTimeout   = 10 * time.Second
	maxResponseBytes = 1 << 20

	// scope asks for the account identifier, the verified email and the
	// basic profile fields used to prefill a new account.
	scope = "openid email profile"
)

// ErrEmailNotVerified is returned when Google reports an address it has not
// verified. Linking an unverified address to an existing account would let
// anyone claim it, so the flow refuses to continue.
var ErrEmailNotVerified = errors.New("google account email is not verified")

// ErrInvalidState means the signed state cookie was missing, forged, or older
// than stateLifetime.
var ErrInvalidState = errors.New("google sign-in state is invalid or expired")

// Endpoints are the three provider URLs the flow needs. They are configurable
// so tests can point at a local server without inventing transport seams.
type Endpoints struct {
	AuthURL     string
	TokenURL    string
	UserInfoURL string
}

// DefaultEndpoints returns Google's production endpoints.
func DefaultEndpoints() Endpoints {
	return Endpoints{
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		UserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
	}
}

// Config is one deployment's Google client configuration.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Endpoints    Endpoints
}

func (c Config) validate() error {
	if strings.TrimSpace(c.ClientID) == "" {
		return errors.New("google client ID is required")
	}
	if strings.TrimSpace(c.ClientSecret) == "" {
		return errors.New("google client secret is required")
	}
	if err := validateEndpoint("redirect URL", c.RedirectURL); err != nil {
		return err
	}
	for name, endpoint := range map[string]string{
		"authorization endpoint": c.Endpoints.AuthURL,
		"token endpoint":         c.Endpoints.TokenURL,
		"userinfo endpoint":      c.Endpoints.UserInfoURL,
	} {
		if err := validateEndpoint(name, endpoint); err != nil {
			return err
		}
	}
	return nil
}

func validateEndpoint(name, raw string) error {
	address, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || address.Host == "" || (address.Scheme != "https" && address.Scheme != "http") ||
		address.User != nil || address.Fragment != "" {
		return fmt.Errorf("%s must be an absolute http(s) URL without credentials or a fragment", name)
	}
	return nil
}

// ConfigFromEnvironment reads the deployment configuration.
//
// The returned boolean reports whether Google sign-in is configured at all. An
// empty GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET simply leaves the feature
// disabled; setting only one of them is a configuration error, so a half-set
// deployment cannot silently fall back to an unexpected identity provider.
func ConfigFromEnvironment() (Config, bool, error) {
	clientID := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
	if clientID == "" && clientSecret == "" {
		return Config{}, false, nil
	}
	if clientID == "" || clientSecret == "" {
		return Config{}, false, errors.New("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set together")
	}

	redirect := strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URL"))
	if redirect == "" {
		origin := strings.TrimSpace(os.Getenv("SITE_URL"))
		if origin == "" {
			origin = strings.TrimSpace(os.Getenv("RENDER_EXTERNAL_URL"))
		}
		if origin == "" {
			return Config{}, false, errors.New("GOOGLE_REDIRECT_URL or SITE_URL is required for Google sign-in")
		}
		address, err := url.Parse(origin)
		if err != nil || address.Host == "" || (address.Scheme != "https" && address.Scheme != "http") ||
			address.User != nil || address.RawQuery != "" || address.ForceQuery || address.Fragment != "" ||
			(address.Path != "" && address.Path != "/") {
			return Config{}, false, errors.New("SITE_URL must be an http(s) origin without a path, query, or credentials")
		}
		redirect = strings.TrimRight(origin, "/") + "/auth/google/callback"
	}

	config := Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirect,
		Endpoints:    DefaultEndpoints(),
	}
	if err := config.validate(); err != nil {
		return Config{}, false, err
	}
	return config, true, nil
}

// User is the subset of Google's userinfo response the application needs.
type User struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	Picture       string `json:"picture"`
}

// StatePayload is what the browser carries between the redirect and callback.
// It is signed with the client secret so a visitor cannot change the role or
// the post-login destination.
type StatePayload struct {
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Role     string `json:"r"`
	Next     string `json:"x"`
	IssuedAt int64  `json:"i"`
}

// PendingState is the result of starting a sign-in.
type PendingState struct {
	// CookieValue is stored in the HttpOnly state cookie.
	CookieValue string
	// Nonce is sent to Google as the OAuth state parameter.
	Nonce string
	// Verifier is the PKCE code verifier; only its hash goes to Google.
	Verifier string
}

// Provider performs the Google sign-in flow for one client configuration.
type Provider struct {
	config Config
	client *http.Client
	now    func() time.Time
}

// NewProvider validates config and returns a ready provider. A nil client uses
// a bounded client that refuses redirects, so a malicious endpoint cannot walk
// the authorization code or tokens to another host.
func NewProvider(config Config, client *http.Client) (*Provider, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{
			Timeout: requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &Provider{config: config, client: client, now: time.Now}, nil
}

// RedirectURL reports the callback URL registered with Google.
func (p *Provider) RedirectURL() string { return p.config.RedirectURL }

// NewState creates a fresh nonce, PKCE verifier and signed state cookie value.
func (p *Provider) NewState(role, next string) (PendingState, error) {
	nonce, err := RandomToken()
	if err != nil {
		return PendingState{}, err
	}
	verifier, err := RandomToken()
	if err != nil {
		return PendingState{}, err
	}
	payload := StatePayload{
		Nonce:    nonce,
		Verifier: verifier,
		Role:     role,
		Next:     next,
		IssuedAt: p.now().UTC().Unix(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return PendingState{}, fmt.Errorf("encode google sign-in state: %w", err)
	}
	body := base64.RawURLEncoding.EncodeToString(data)
	cookie := body + "." + base64.RawURLEncoding.EncodeToString(p.mac(body))
	return PendingState{CookieValue: cookie, Nonce: nonce, Verifier: verifier}, nil
}

// ReadState verifies the signed cookie value and returns its payload.
func (p *Provider) ReadState(cookieValue string) (StatePayload, error) {
	body, signature, ok := strings.Cut(cookieValue, ".")
	if !ok || body == "" || signature == "" {
		return StatePayload{}, ErrInvalidState
	}
	got, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || subtle.ConstantTimeCompare(got, p.mac(body)) != 1 {
		return StatePayload{}, ErrInvalidState
	}
	data, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return StatePayload{}, ErrInvalidState
	}
	var payload StatePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return StatePayload{}, ErrInvalidState
	}
	if payload.Nonce == "" || payload.Verifier == "" || payload.Role == "" {
		return StatePayload{}, ErrInvalidState
	}
	issued := time.Unix(payload.IssuedAt, 0)
	now := p.now()
	if issued.After(now.Add(time.Minute)) || now.Sub(issued) > stateLifetime {
		return StatePayload{}, ErrInvalidState
	}
	return payload, nil
}

func (p *Provider) mac(body string) []byte {
	mac := hmac.New(sha256.New, []byte(p.config.ClientSecret))
	mac.Write([]byte("google-sign-in-state:"))
	mac.Write([]byte(body))
	return mac.Sum(nil)
}

// AuthCodeURL builds the URL that sends the visitor to Google.
func (p *Provider) AuthCodeURL(state, verifier string) string {
	query := url.Values{}
	query.Set("client_id", p.config.ClientID)
	query.Set("redirect_uri", p.config.RedirectURL)
	query.Set("response_type", "code")
	query.Set("scope", scope)
	query.Set("state", state)
	query.Set("code_challenge", Challenge(verifier))
	query.Set("code_challenge_method", "S256")
	query.Set("access_type", "online")
	query.Set("prompt", "select_account")
	return p.config.Endpoints.AuthURL + "?" + query.Encode()
}

// Exchange redeems an authorization code and returns the Google account that
// authorized it. The access token is used against the userinfo endpoint, which
// avoids having to validate an ID token's signature and audience.
func (p *Provider) Exchange(ctx context.Context, code, verifier string) (User, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(verifier) == "" {
		return User{}, errors.New("google sign-in code is missing")
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", p.config.ClientID)
	form.Set("client_secret", p.config.ClientSecret)
	form.Set("redirect_uri", p.config.RedirectURL)
	form.Set("grant_type", "authorization_code")
	form.Set("code_verifier", verifier)

	token, err := p.exchangeCode(ctx, form)
	if err != nil {
		return User{}, err
	}
	return p.userInfo(ctx, token)
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
}

func (p *Provider) exchangeCode(ctx context.Context, form url.Values) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Endpoints.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build google token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	body, status, err := p.do(request)
	if err != nil {
		return "", fmt.Errorf("google token exchange: %w", err)
	}
	var response tokenResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("decode google token response: %w", err)
	}
	if response.Error != "" {
		return "", fmt.Errorf("google token exchange rejected: %s", response.Error)
	}
	if status != http.StatusOK || response.AccessToken == "" {
		return "", fmt.Errorf("google token exchange returned no access token (status %d)", status)
	}
	return response.AccessToken, nil
}

func (p *Provider) userInfo(ctx context.Context, accessToken string) (User, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.config.Endpoints.UserInfoURL, nil)
	if err != nil {
		return User{}, fmt.Errorf("build google userinfo request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/json")

	body, status, err := p.do(request)
	if err != nil {
		return User{}, fmt.Errorf("google userinfo: %w", err)
	}
	if status != http.StatusOK {
		return User{}, fmt.Errorf("google userinfo returned status %d", status)
	}
	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return User{}, fmt.Errorf("decode google userinfo response: %w", err)
	}
	if user.Subject == "" || user.Email == "" {
		return User{}, errors.New("google userinfo response is missing an account identifier or email")
	}
	if !validEmail(user.Email) {
		return User{}, errors.New("google userinfo returned an invalid email address")
	}
	if !user.EmailVerified {
		return User{}, ErrEmailNotVerified
	}
	return user, nil
}

func (p *Provider) do(request *http.Request) ([]byte, int, error) {
	response, err := p.client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, response.StatusCode, err
	}
	return body, response.StatusCode, nil
}

// RandomToken returns 32 random bytes in a URL-safe form. It is used for the
// OAuth state nonce and the PKCE code verifier (43 characters, inside RFC
// 7636's 43-128 character range).
func RandomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

// Challenge derives the S256 PKCE challenge for a verifier.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func validEmail(email string) bool {
	if len(email) == 0 || len(email) > 254 || strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return false
	}
	local, domain, found := strings.Cut(email, "@")
	return found && local != "" && len(local) <= 64 && strings.Contains(domain, ".")
}
