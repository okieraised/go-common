package keycloak

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/okieraised/go-common/ptrutils"
)

type JWK struct {
	Kid string   `json:"kid"`
	Kty string   `json:"kty"` // should be "RSA"
	Use string   `json:"use"` // should be "sig"
	Alg string   `json:"alg"` // usually "RS256"
	N   string   `json:"n"`   // modulus
	E   string   `json:"e"`   // exponent
	X5c []string `json:"x5c"` // optional cert chain
}

type JWKS struct {
	Keys []JWK `json:"keys"`
}

func GetKeycloakRSAPublicKey(certs []byte) (*rsa.PublicKey, error) {
	var jwks JWKS
	if err := json.Unmarshal(certs, &jwks); err != nil {
		return nil, err
	}

	var rsaKey JWK
	for _, key := range jwks.Keys {
		if key.Kty == "RSA" && key.Use == "sig" {
			rsaKey = key
		}
	}

	nb, err := base64.RawURLEncoding.DecodeString(rsaKey.N)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(rsaKey.E)
	if err != nil {
		return nil, err
	}

	e := big.NewInt(0).SetBytes(eb).Int64()
	n := big.NewInt(0).SetBytes(nb)

	return &rsa.PublicKey{
		N: n,
		E: int(e),
	}, nil
}

var (
	globalMu   sync.RWMutex
	globalKC   *KcClient
	globalOnce sync.Once
)

func SetGlobal(kc *KcClient) bool {
	set := false
	globalOnce.Do(func() {
		globalMu.Lock()
		defer globalMu.Unlock()
		globalKC = kc
		set = true
	})
	return set
}

func Global() *KcClient {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalKC
}

type KcClient struct {
	c          *gocloak.GoCloak
	realm      string
	adminRealm string
	mu         sync.RWMutex
	keys       map[string]*rsa.PublicKey
	defaultPK  *rsa.PublicKey
	ttl        time.Duration
	stopCh     chan struct{}
	stopped    chan struct{}
}

func (kc *KcClient) Realm() string      { return kc.realm }
func (kc *KcClient) AdminRealm() string { return kc.adminRealm }

// PublicKey returns the default RSA public key (first available key).
func (kc *KcClient) PublicKey() *rsa.PublicKey {
	kc.mu.RLock()
	defer kc.mu.RUnlock()
	return kc.defaultPK
}

// PublicKeyByKID returns a key by kid if present.
func (kc *KcClient) PublicKeyByKID(kid string) (*rsa.PublicKey, bool) {
	kc.mu.RLock()
	defer kc.mu.RUnlock()
	pk, ok := kc.keys[kid]
	return pk, ok
}

// RefreshJWKS refetches JWKS from Keycloak and updates the cache.
func (kc *KcClient) RefreshJWKS(ctx context.Context) error {
	keys, def, err := kc.fetchJWKS(ctx, kc.realm)
	if err != nil {
		return err
	}
	kc.mu.Lock()
	kc.keys = keys
	kc.defaultPK = def
	kc.mu.Unlock()
	return nil
}

// Stop stops the auto-refresh goroutine (if running).
func (kc *KcClient) Stop() {
	if kc.stopCh == nil {
		return
	}
	select {
	case <-kc.stopped:
		// already stopped
	default:
		close(kc.stopCh)
		<-kc.stopped
	}
}

type Option func(*opts)

type opts struct {
	adminRealmsPath string
	realmsPath      string
	tlsInsecure     bool
	timeout         time.Duration
	transport       http.RoundTripper
	ttl             time.Duration
}

func WithTLSInsecure(skipVerify bool) Option {
	return func(o *opts) {
		o.tlsInsecure = skipVerify
	}
}

func WithHTTPTimeout(d time.Duration) Option {
	return func(o *opts) {
		o.timeout = d
	}
}

func WithTransport(rt http.RoundTripper) Option {
	return func(o *opts) { o.transport = rt }
}

func WithCustomAuthPaths(adminRealmsPath, realmsPath string) Option {
	return func(o *opts) {
		if adminRealmsPath != "" {
			o.adminRealmsPath = adminRealmsPath
		}
		if realmsPath != "" {
			o.realmsPath = realmsPath
		}
	}
}

func WithJWKSRefreshTTL(d time.Duration) Option {
	return func(o *opts) {
		o.ttl = d
	}
}

// New creates a Keycloak client and eagerly fetches JWKS.
func New(ctx context.Context, host, adminRealm, realm string, options ...Option) (*KcClient, error) {
	if host == "" || realm == "" || adminRealm == "" {
		return nil, errors.New("keycloak: host, adminRealm, and realm are required")
	}
	o := &opts{
		adminRealmsPath: "admin/realms",
		realmsPath:      "realms",
		timeout:         30 * time.Second,
	}
	for _, fn := range options {
		fn(o)
	}

	cl := gocloak.NewClient(
		host,
		gocloak.SetAuthAdminRealms(o.adminRealmsPath),
		gocloak.SetAuthRealms(o.realmsPath),
	)

	resty := cl.RestyClient()
	resty.SetTimeout(o.timeout)
	if o.transport != nil {
		resty.SetTransport(o.transport)
	}
	if o.tlsInsecure {
		resty.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}) // #nosec G402 (opt-in)
	}

	kc := &KcClient{
		c:          cl,
		realm:      realm,
		adminRealm: adminRealm,
		keys:       make(map[string]*rsa.PublicKey),
		ttl:        o.ttl,
	}

	if err := kc.RefreshJWKS(ctx); err != nil {
		return nil, fmt.Errorf("keycloak: fetch JWKS: %w", err)
	}

	if kc.ttl > 0 {
		kc.stopCh = make(chan struct{})
		kc.stopped = make(chan struct{})
		go kc.runAutoRefresh()
	}

	return kc, nil
}

func (kc *KcClient) runAutoRefresh() {
	defer close(kc.stopped)
	ticker := time.NewTicker(kc.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-kc.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := kc.RefreshJWKS(ctx)
			cancel()
			if err != nil {
				continue
			}
		}
	}
}

func (kc *KcClient) fetchJWKS(ctx context.Context, realm string) (map[string]*rsa.PublicKey, *rsa.PublicKey, error) {
	cr, err := kc.c.GetCerts(ctx, realm)
	if err != nil {
		return nil, nil, fmt.Errorf("get certs: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey)
	var first *rsa.PublicKey

	for _, k := range ptrutils.Deref(cr.Keys) {
		if k == (gocloak.CertResponseKey{}) {
			continue
		}
		var pk *rsa.PublicKey

		if k.N != nil && k.E != nil {
			if v, err := rsaFromModExp(*k.N, *k.E); err == nil {
				pk = v
			}
		}

		if pk == nil && k.X5c != nil && len(*k.X5c) > 0 && ptrutils.Deref(k.X5c)[0] != "" {
			if v, err := rsaFromX5C(ptrutils.Deref(k.X5c)[0]); err == nil {
				pk = v
			}
		}

		if pk == nil {
			continue
		}
		if first == nil {
			first = pk
		}
		if k.Kid != nil && *k.Kid != "" {
			keys[ptrutils.Deref(k.Kid)] = pk
		}
	}

	if len(keys) == 0 {
		return nil, nil, errors.New("keycloak: no RSA keys in JWKS")
	}
	return keys, first, nil
}

func rsaFromModExp(nBase64URL, eBase64URL string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(nBase64URL)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(eBase64URL)
	if err != nil {
		return nil, err
	}
	n := new(big.Int).SetBytes(nb)
	e := 0
	for _, b := range eb {
		e = e<<8 | int(b)
	}
	if e <= 0 {
		return nil, errors.New("invalid RSA exponent")
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}

func rsaFromX5C(firstCertBase64 string) (*rsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(firstCertBase64)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	pk, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("x5c is not RSA")
	}
	return pk, nil
}

func (kc *KcClient) LoginAdmin(ctx context.Context, username, password string) (*gocloak.JWT, error) {
	return kc.c.LoginAdmin(ctx, username, password, kc.adminRealm)
}

func (kc *KcClient) Login(ctx context.Context, clientID, clientSecret, username, password string) (*gocloak.JWT, error) {
	return kc.c.Login(ctx, clientID, clientSecret, kc.realm, username, password)
}

func (kc *KcClient) LoginClient(ctx context.Context, clientID, clientSecret string, scopes ...string) (*gocloak.JWT, error) {
	return kc.c.LoginClient(ctx, clientID, clientSecret, kc.realm, scopes...)
}

func (kc *KcClient) Logout(ctx context.Context, clientID, clientSecret, refreshToken string) error {
	return kc.c.Logout(ctx, clientID, clientSecret, kc.realm, refreshToken)
}

func (kc *KcClient) Revoke(ctx context.Context, clientID, clientSecret, refreshToken string) error {
	return kc.c.RevokeToken(ctx, kc.realm, clientID, clientSecret, refreshToken)
}

func (kc *KcClient) LogoutAllSessions(ctx context.Context, accessToken, userID string) error {
	return kc.c.LogoutAllSessions(ctx, accessToken, kc.realm, userID)
}

func (kc *KcClient) RefreshToken(ctx context.Context, refreshToken, clientID, clientSecret string) (*gocloak.JWT, error) {
	return kc.c.RefreshToken(ctx, refreshToken, clientID, clientSecret, kc.realm)
}

func (kc *KcClient) CreateClient(ctx context.Context, accessToken string, newClient gocloak.Client) (string, error) {
	return kc.c.CreateClient(ctx, accessToken, kc.realm, newClient)
}

func (kc *KcClient) UpdateClient(ctx context.Context, token string, updatedClient gocloak.Client) error {
	return kc.c.UpdateClient(ctx, token, kc.realm, updatedClient)
}

func (kc *KcClient) DeleteClient(ctx context.Context, token, idOfClient string) error {
	return kc.c.DeleteClient(ctx, token, kc.realm, idOfClient)
}

func (kc *KcClient) CreateClientScope(ctx context.Context, token string, scope gocloak.ClientScope) (string, error) {
	return kc.c.CreateClientScope(ctx, token, kc.realm, scope)
}

func (kc *KcClient) CreateRealm(ctx context.Context, token string, realm gocloak.RealmRepresentation) (string, error) {
	return kc.c.CreateRealm(ctx, token, realm)
}

func (kc *KcClient) UpdateRealm(ctx context.Context, token string, realm gocloak.RealmRepresentation) error {
	return kc.c.UpdateRealm(ctx, token, realm)
}

func (kc *KcClient) CreateRealmRole(ctx context.Context, token string, role gocloak.Role) (string, error) {
	return kc.c.CreateRealmRole(ctx, token, kc.realm, role)
}

func (kc *KcClient) DeleteClientRole(ctx context.Context, token, idOfClient, roleName string) error {
	return kc.c.DeleteClientRole(ctx, token, kc.realm, idOfClient, roleName)
}

func (kc *KcClient) CreateGroup(ctx context.Context, token string, group gocloak.Group) (string, error) {
	return kc.c.CreateGroup(ctx, token, kc.realm, group)
}

func (kc *KcClient) DeleteGroup(ctx context.Context, token, groupID string) error {
	return kc.c.DeleteGroup(ctx, token, kc.realm, groupID)
}

func (kc *KcClient) GetClient(ctx context.Context, token, idOfClient string) (*gocloak.Client, error) {
	return kc.c.GetClient(ctx, token, kc.realm, idOfClient)
}

func (kc *KcClient) GetClients(ctx context.Context, token string, params gocloak.GetClientsParams) ([]*gocloak.Client, error) {
	return kc.c.GetClients(ctx, token, kc.realm, params)
}

func (kc *KcClient) CreateClientRole(ctx context.Context, token, idOfClient string, role gocloak.Role) (string, error) {
	return kc.c.CreateClientRole(ctx, token, kc.realm, idOfClient, role)
}

func (kc *KcClient) GetClientRole(ctx context.Context, token, idOfClient, roleName string) (*gocloak.Role, error) {
	return kc.c.GetClientRole(ctx, token, kc.realm, idOfClient, roleName)
}

func (kc *KcClient) GetClientRoles(ctx context.Context, token, idOfClient string, params gocloak.GetRoleParams) ([]*gocloak.Role, error) {
	return kc.c.GetClientRoles(ctx, token, kc.realm, idOfClient, params)
}

func (kc *KcClient) GetClientRoleByID(ctx context.Context, token, roleID string) (*gocloak.Role, error) {
	return kc.c.GetClientRoleByID(ctx, token, kc.realm, roleID)
}

func (kc *KcClient) GetClientRolesByGroupID(ctx context.Context, token, idOfClient, groupID string) ([]*gocloak.Role, error) {
	return kc.c.GetClientRolesByGroupID(ctx, token, kc.realm, idOfClient, groupID)
}

func (kc *KcClient) CreateUser(ctx context.Context, token string, user gocloak.User) (string, error) {
	return kc.c.CreateUser(ctx, token, kc.realm, user)
}

func (kc *KcClient) UpdateUser(ctx context.Context, token string, user gocloak.User) error {
	return kc.c.UpdateUser(ctx, token, kc.realm, user)
}

func (kc *KcClient) DeleteUser(ctx context.Context, token, userID string) error {
	return kc.c.DeleteUser(ctx, token, kc.realm, userID)
}

func (kc *KcClient) AddUserToGroup(ctx context.Context, token, userID, groupID string) error {
	return kc.c.AddUserToGroup(ctx, token, kc.realm, userID, groupID)
}

func (kc *KcClient) DeleteUserFromGroup(ctx context.Context, token, userID, groupID string) error {
	return kc.c.DeleteUserFromGroup(ctx, token, kc.realm, userID, groupID)
}

func (kc *KcClient) AddClientRolesToUser(ctx context.Context, token, idOfClient, userID string, roles []gocloak.Role) error {
	return kc.c.AddClientRolesToUser(ctx, token, kc.realm, idOfClient, userID, roles)
}

func (kc *KcClient) DeleteClientRolesFromUser(ctx context.Context, token, idOfClient, userID string, roles []gocloak.Role) error {
	return kc.c.DeleteClientRolesFromUser(ctx, token, kc.realm, idOfClient, userID, roles)
}

func (kc *KcClient) AddRealmRoleToUser(ctx context.Context, token, userID string, roles []gocloak.Role) error {
	return kc.c.AddRealmRoleToUser(ctx, token, kc.realm, userID, roles)
}

func (kc *KcClient) DeleteRealmRoleFromUser(ctx context.Context, token, userID string, roles []gocloak.Role) error {
	return kc.c.DeleteRealmRoleFromUser(ctx, token, kc.realm, userID, roles)
}

func (kc *KcClient) GetClientRolesByUserID(ctx context.Context, token, idOfClient, userID string) ([]*gocloak.Role, error) {
	return kc.c.GetClientRolesByUserID(ctx, token, kc.realm, idOfClient, userID)
}

func (kc *KcClient) GetUserByID(ctx context.Context, accessToken, userID string) (*gocloak.User, error) {
	return kc.c.GetUserByID(ctx, accessToken, kc.realm, userID)
}
