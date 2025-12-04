package cryptoutils

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/pkg/errors"
)

func parseCACert(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("failed to decode CA cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse x509 cert")
	}
	if !cert.IsCA {
		return nil, fmt.Errorf("provided certificate is not a CA certificate")
	}
	return cert, nil
}

func parseCAKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode CA key PEM")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		// PKCS#8 (generic) – still expect RSA inside for this helper
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse PKCS8")
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("CA key is not RSA (got %T)", key)
		}
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unsupported CA key type %q", block.Type)
	}
}

// GenerateClientCert auto-detects the CA key type (RSA or Ed25519) and produces a matching client certificate and private key.
//
//   - caCertPath: path to CA certificate PEM
//   - caKeyPath:  path to CA private key PEM (unencrypted, RSA in this example)
//   - commonName: CN for the client cert (e.g. "robot-123", "agent-01")
//   - dnsNames:   optional DNS names (used if you do hostname-based auth)
func GenerateClientCert(caCertPath, caKeyPath string, commonName string, dnsNames []string) ([]byte, []byte, error) {
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to read ca cert")
	}
	certBlock, _ := pem.Decode(caCertPEM)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("invalid CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse CA cert")
	}

	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to read ca key")
	}
	keyBlock, _ := pem.Decode(caKeyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("invalid CA key PEM")
	}

	var caPrivKey any
	switch keyBlock.Type {
	case "RSA PRIVATE KEY":
		caPrivKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to parse RSA key")
		}
	case "PRIVATE KEY":
		caPrivKey, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to parse PKCS#8 key")
		}
	default:
		return nil, nil, fmt.Errorf("unsupported CA key type %q", keyBlock.Type)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"Client"},
		},
		NotBefore: time.Now().Add(-5 * time.Minute),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              dnsNames,
	}

	switch ca := caPrivKey.(type) {
	case *rsa.PrivateKey:
		clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to generate RSA client key: %w")
		}

		derBytes, err := x509.CreateCertificate(rand.Reader, template, caCert, &clientKey.PublicKey, ca)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to sign RSA client cert")
		}

		clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
		clientKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
		})
		return clientCertPEM, clientKeyPEM, nil

	case ed25519.PrivateKey:
		clientPub, clientPriv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to generate Ed25519 key")
		}

		derBytes, err := x509.CreateCertificate(rand.Reader, template, caCert, clientPub, ca)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to sign Ed25519 client cert")
		}

		clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

		keyBytes, err := x509.MarshalPKCS8PrivateKey(clientPriv)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to marshal Ed25519 key")
		}

		clientKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: keyBytes,
		})

		return clientCertPEM, clientKeyPEM, nil

	default:
		return nil, nil, fmt.Errorf("unsupported CA key type: %T", caPrivKey)
	}
}
