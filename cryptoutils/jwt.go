package cryptoutils

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenTypeBearer  = "Bearer"
	TokenTypeRefresh = "Refresh"
)

type JWKSet struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	Kid string   `json:"kid"`
	Kty string   `json:"kty"`
	Alg string   `json:"alg"`
	Use string   `json:"use"`
	N   string   `json:"n,omitempty"`
	E   string   `json:"e,omitempty"`
	X5c []string `json:"x5c,omitempty"`
}

type AlgorithmConfig struct {
	Alg string // "RS256", "ES256", "EdDSA"
	Kty string // "RSA", "EC", "OKP"
	Key string // "rsa-2048"
}

type RefreshTokenClaims struct {
	Exp uint64 `json:"exp"`
	Iat uint64 `json:"iat"`
	Iss string `json:"iss"`
	Aud string `json:"aud"`
	Sub string `json:"sub"`
	Typ string `json:"typ"`
	Azp string `json:"azp"`
}

func (c *RefreshTokenClaims) ToMap() (jwt.MapClaims, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	var m jwt.MapClaims
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *RefreshTokenClaims) FromMap(m jwt.MapClaims) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, c)
}

type AccessTokenClaims struct {
	Exp               uint64   `json:"exp"`
	Iat               uint64   `json:"iat"`
	Iss               string   `json:"iss"`
	Aud               string   `json:"aud"`
	Sub               string   `json:"sub"`
	Typ               string   `json:"typ"`
	Azp               string   `json:"azp"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	GivenName         string   `json:"given_name"`
	FamilyName        string   `json:"family_name"`
	Email             string   `json:"email"`
	Status            string   `json:"status"`
	IsSuperadmin      bool     `json:"is_superadmin"`
	Roles             []string `json:"roles"`
	Policy            []string `json:"policy"`
	EmailVerified     bool     `json:"email_verified"`
}

func (c *AccessTokenClaims) ToMap() (jwt.MapClaims, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	var m jwt.MapClaims
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *AccessTokenClaims) FromMap(m jwt.MapClaims) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, c)
}

func parsePrivateKeyFromPEM(pemBytes []byte) (any, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("invalid private key PEM")
	}

	// Try PKCS8
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	// Try RSA PKCS1
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	return nil, errors.New("unsupported private key format")
}

func parsePublicKeyFromPEM(pemBytes []byte) (interface{}, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("invalid public key PEM")
	}

	switch block.Type {
	case "PUBLIC KEY":
		return x509.ParsePKIXPublicKey(block.Bytes)
	case "RSA PUBLIC KEY":
		return x509.ParsePKCS1PublicKey(block.Bytes)
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		return cert.PublicKey, nil

	default:
		return nil, fmt.Errorf("unsupported PEM type: %s", block.Type)
	}
}

func Sign(claims jwt.MapClaims, pemPrivateKey []byte) (string, error) {
	privKey, err := parsePrivateKeyFromPEM(pemPrivateKey)
	if err != nil {
		return "", err
	}

	var token *jwt.Token

	switch k := privKey.(type) {

	case *rsa.PrivateKey:
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		return token.SignedString(k)

	case *ecdsa.PrivateKey:
		curveBits := k.Params().BitSize
		var method jwt.SigningMethod

		switch curveBits {
		case 256:
			method = jwt.SigningMethodES256
		case 384:
			method = jwt.SigningMethodES384
		case 521:
			method = jwt.SigningMethodES512
		default:
			return "", fmt.Errorf("unsupported ECDSA curve size: %d", curveBits)
		}

		token = jwt.NewWithClaims(method, claims)
		return token.SignedString(k)

	case ed25519.PrivateKey:
		token = jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
		return token.SignedString(k)

	default:
		return "", errors.New("unsupported private key type for JWT signing")
	}
}

func Verify(tokenStr string, pemPublicKey []byte) (*AccessTokenClaims, error) {
	pubKey, err := parsePublicKeyFromPEM(pemPublicKey)
	if err != nil {
		return nil, err
	}

	var rawClaims jwt.MapClaims
	parser := jwt.NewParser(jwt.WithValidMethods([]string{
		jwt.SigningMethodRS256.Alg(),
		jwt.SigningMethodRS384.Alg(),
		jwt.SigningMethodRS512.Alg(),
		jwt.SigningMethodES256.Alg(),
		jwt.SigningMethodES384.Alg(),
		jwt.SigningMethodES512.Alg(),
		jwt.SigningMethodEdDSA.Alg(),
	}))

	// Validate signature and extract claims
	token, err := parser.ParseWithClaims(tokenStr, &rawClaims, func(t *jwt.Token) (interface{}, error) {
		return pubKey, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid JWT")
	}

	var out AccessTokenClaims
	b, err := json.Marshal(rawClaims)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func SignToken(claims jwt.MapClaims, privPEM []byte) (string, error) {
	return Sign(claims, privPEM)
}

func VerifyRefreshToken(tok string, pubPEM []byte) (*jwt.MapClaims, error) {
	parsedClaims := &jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tok, parsedClaims, func(t *jwt.Token) (interface{}, error) {
		return parsePublicKeyFromPEM(pubPEM)
	})
	if err != nil {
		return nil, err
	}
	return parsedClaims, nil
}
