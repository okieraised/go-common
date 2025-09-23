package oidc_client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type Client struct {
	Provider *oidc.Provider
	Verifier *oidc.IDTokenVerifier
	OAuth2   oauth2.Config

	host       string
	realm      string
	logoutURL  string
	httpClient *http.Client
}

type Option func(*oidcCfg)

type oidcCfg struct {
	httpClient  *http.Client
	timeout     time.Duration
	insecureTLS bool
	scopes      []string
	redirectURL string
}

func WithHTTPClient(c *http.Client) Option {
	return func(o *oidcCfg) {
		o.httpClient = c
	}
}

func WithTimeout(d time.Duration) Option {
	return func(o *oidcCfg) {
		o.timeout = d
	}
}

func WithInsecureTLS() Option {
	return func(o *oidcCfg) {
		o.insecureTLS = true
	}
}

func WithScopes(scopes ...string) Option {
	return func(o *oidcCfg) {
		o.scopes = append([]string{}, scopes...)
	}
}
func WithRedirectURL(u string) Option { return func(o *oidcCfg) { o.redirectURL = u } }

func New(ctx context.Context, host, realm, clientID, clientSecret string, opts ...Option) (*Client, error) {
	if host == "" || realm == "" || clientID == "" {
		return nil, errors.New("oidc: host, realm, and clientID are required")
	}

	cfg := &oidcCfg{
		timeout: 30 * time.Second,
		scopes:  []string{oidc.ScopeOpenID, "roles"},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Build HTTP client
	httpClient := cfg.httpClient
	if httpClient == nil {
		tr := &http.Transport{}
		if cfg.insecureTLS {
			tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 opt-in
		}
		httpClient = &http.Client{Transport: tr, Timeout: cfg.timeout}
	}

	// Provider URL per Keycloak realm
	providerURL := strings.TrimRight(host, "/") + "/realms/" + url.PathEscape(realm)

	// Provider discovery
	pctx := oidc.ClientContext(ctx, httpClient)
	provider, err := oidc.NewProvider(pctx, providerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc: provider discovery failed: %w", err)
	}

	// Verifier (default audience = clientID)
	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

	oauthCfg := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  cfg.redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.scopes,
	}

	c := &Client{
		Provider:   provider,
		Verifier:   verifier,
		OAuth2:     oauthCfg,
		host:       strings.TrimRight(host, "/"),
		realm:      realm,
		httpClient: httpClient,
		logoutURL:  strings.TrimRight(host, "/") + "/realms/" + url.PathEscape(realm) + "/protocol/openid-connect/logout",
	}
	return c, nil
}

// AuthCodeURL builds the authorization URL. Pass a CSRF `state`.
func (c *Client) AuthCodeURL(state string, extra ...oauth2.AuthCodeOption) string {
	return c.OAuth2.AuthCodeURL(state, extra...)
}

// ExchangeCode exchanges an authorization code for tokens.
func (c *Client) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return c.OAuth2.Exchange(oidc.ClientContext(ctx, c.httpClient), code)
}

// RefreshTokenSource returns a TokenSource that uses the refresh token.
func (c *Client) RefreshTokenSource(ctx context.Context, token *oauth2.Token) oauth2.TokenSource {
	return c.OAuth2.TokenSource(oidc.ClientContext(ctx, c.httpClient), token)
}

// UserInfo fetches the OIDC userinfo document (when access token has the scope/permission).
func (c *Client) UserInfo(ctx context.Context, src oauth2.TokenSource) (*oidc.UserInfo, error) {
	return c.Provider.UserInfo(oidc.ClientContext(ctx, c.httpClient), src)
}

// EndSessionURL builds a Keycloak logout URL.
// Pass idTokenHint if you have it; postLogoutRedirectURI is optional (must be registered).
func (c *Client) EndSessionURL(idTokenHint, postLogoutRedirectURI string) (string, error) {
	u, err := url.Parse(c.logoutURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if idTokenHint != "" {
		q.Set("id_token_hint", idTokenHint)
	}
	if postLogoutRedirectURI != "" {
		q.Set("post_logout_redirect_uri", postLogoutRedirectURI)
	}
	// Keycloak also accepts client_id (useful if no id_token_hint)
	if c.OAuth2.ClientID != "" && idTokenHint == "" {
		q.Set("client_id", c.OAuth2.ClientID)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
