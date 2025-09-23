package oidc_client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCClaims struct {
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	GivenName         string   `json:"given_name"`
	FamilyName        string   `json:"family_name"`
	Email             string   `json:"email"`
	Status            string   `json:"status"`
	Roles             []string `json:"roles"`
	Policy            []string `json:"policy"`
	EmailVerified     bool     `json:"email_verified"`
}

type OIDCClient struct {
	Provider *oidc.Provider        // Handles OIDC protocol operations with Keycloak
	OIDC     *oidc.IDTokenVerifier // Verifies JWT tokens from Keycloak
	Oauth    oauth2.Config         // Manages OAuth2 flow (authorization codes, tokens)
}

var oidcClient *OIDCClient

func GetInstance() *OIDCClient {
	return oidcClient
}

func NewOIDCClient(host, realm, clientID, clientSecret, redirectUrl string) (*OIDCClient, error) {
	ctx := oidc.ClientContext(context.Background(), &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	})

	// Construct the provider URL using Keycloak realm
	providerURL := fmt.Sprintf("%s/realms/%s", host, realm)

	provider, err := oidc.NewProvider(ctx, providerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %v", err)
	}

	// Create ID token verifier
	verifier := provider.Verifier(&oidc.Config{
		ClientID: clientID,
	})

	// Configure an OpenID Connect aware OAuth2 client with specific scopes:
	// - oidc.ScopeOpenID: Required for OpenID Connect authentication, provides subject ID (sub)
	// - "roles": Keycloak-specific scope to get user roles in the token
	oauth2Config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectUrl,
		Endpoint:     provider.Endpoint(),
		Scopes: []string{
			oidc.ScopeOpenID, // Required for OIDC authentication
			"roles",          // Request user roles from Keycloak
		},
	}

	// Return initialized client with all required components
	oidcClient = &OIDCClient{
		// oauth2Config: Used for OAuth2 operations like:
		// - Generating login URL (AuthCodeURL)
		// - Exchanging auth code for tokens (Exchange)
		// - Managing token refresh
		Oauth: oauth2Config,

		// verifier: Used to validate tokens:
		// - Verifies JWT signature
		// - Validates token claims (exp, iss, aud)
		// - Extracts user information
		OIDC: verifier,

		// provider: Keycloak OIDC provider that:
		// - Provides endpoint URLs (auth, token)
		// - Handles OIDC protocol details
		// - Manages provider metadata
		Provider: provider,
	}

	return oidcClient, nil
}

// AuthCodeURL generates the login URL for OAuth2 authorization code flow.
// It returns a URL that the user should be redirected to for authentication.
// The state parameter is a random string that will be validated in the callback
// to prevent CSRF attacks.
func (c *OIDCClient) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return c.Oauth.AuthCodeURL(state, opts...)
}
