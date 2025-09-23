package cryptoutils

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/hmac"
	crypto_rand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(crypto_rand.Reader, b)
	return b, err
}

func RandomStringURLSafe(nBytes int) (string, error) {
	b, err := RandomBytes(nBytes)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func SHA256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func SHA512Hex(b []byte) string {
	h := sha512.Sum512(b)
	return hex.EncodeToString(h[:])
}

func HMACSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	_, _ = m.Write(data)
	return m.Sum(nil)
}

func VerifyHMACSHA256(key, data, mac []byte) bool {
	expected := HMACSHA256(key, data)
	return hmac.Equal(expected, mac)
}

type Argon2Params struct {
	Time    uint32
	Memory  uint32 // KiB
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}

var DefaultArgon2 = Argon2Params{
	Time:    3,
	Memory:  64 * 1024,
	Threads: 1,
	SaltLen: 16,
	KeyLen:  32,
}

func HashPasswordArgon2id(password string, p Argon2Params) (string, error) {
	if p.SaltLen == 0 || p.KeyLen == 0 {
		return "", errors.New("invalid argon2 params")
	}
	salt, err := RandomBytes(int(p.SaltLen))
	if err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, p.KeyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", p.Memory, p.Time, p.Threads, b64Salt, b64Hash), nil
}

func VerifyPasswordArgon2id(phc, password string) (bool, error) {
	var memory, time uint32
	var threads uint8
	parts := bytes.Split([]byte(phc), []byte("$"))
	if len(parts) != 6 || string(parts[1]) != "argon2id" {
		return false, errors.New("invalid phc format")
	}
	if string(parts[2]) != "v=19" {
		return false, errors.New("unsupported argon2 version")
	}
	if _, err := fmt.Sscanf(string(parts[3]), "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, errors.New("invalid phc params")
	}
	salt, err := base64.RawStdEncoding.DecodeString(string(parts[4]))
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(string(parts[5]))
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return hmac.Equal(got, want), nil
}

func AESGCMEncrypt(key32, plaintext, aad []byte) ([]byte, error) {
	if len(key32) != 32 {
		return nil, errors.New("AESGCMEncrypt: key must be 32 bytes")
	}
	block, err := aes.NewCipher(key32)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, err := RandomBytes(aead.NonceSize())
	if err != nil {
		return nil, err
	}
	ct := aead.Seal(nil, nonce, plaintext, aad)
	return append(nonce, ct...), nil
}

func AESGCMDecrypt(key32, nonceAndCiphertext, aad []byte) ([]byte, error) {
	if len(key32) != 32 {
		return nil, errors.New("AESGCMDecrypt: key must be 32 bytes")
	}
	block, err := aes.NewCipher(key32)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := aead.NonceSize()
	if len(nonceAndCiphertext) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce := nonceAndCiphertext[:ns]
	ct := nonceAndCiphertext[ns:]
	return aead.Open(nil, nonce, ct, aad)
}

func XChaCha20PEncrypt(key32, plaintext, aad []byte) ([]byte, error) {
	if len(key32) != 32 {
		return nil, errors.New("XChaCha20PEncrypt: key must be 32 bytes")
	}
	aead, err := chacha20poly1305.NewX(key32)
	if err != nil {
		return nil, err
	}
	nonce, err := RandomBytes(chacha20poly1305.NonceSizeX)
	if err != nil {
		return nil, err
	}
	ct := aead.Seal(nil, nonce, plaintext, aad)
	return append(nonce, ct...), nil
}

func XChaCha20PDecrypt(key32, nonceAndCiphertext, aad []byte) ([]byte, error) {
	if len(key32) != 32 {
		return nil, errors.New("XChaCha20PDecrypt: key must be 32 bytes")
	}
	aead, err := chacha20poly1305.NewX(key32)
	if err != nil {
		return nil, err
	}
	ns := chacha20poly1305.NonceSizeX
	if len(nonceAndCiphertext) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce := nonceAndCiphertext[:ns]
	ct := nonceAndCiphertext[ns:]
	return aead.Open(nil, nonce, ct, aad)
}

const (
	crxMagic uint32 = 0x43525831 // "CRX1"
	aesGCMID byte   = 1
	xchID    byte   = 2
)

func EncryptPassphrase(passphrase []byte, plaintext, aad []byte, useXChaCha bool) (string, error) {
	salt, err := RandomBytes(16)
	if err != nil {
		return "", err
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, passphrase, salt, []byte("cryptox-hkdf-v1")), key); err != nil {
		return "", err
	}

	var (
		aeadID byte
		out    []byte
	)
	if useXChaCha {
		aeadID = xchID
		out, err = XChaCha20PEncrypt(key, plaintext, aad)
	} else {
		aeadID = aesGCMID
		out, err = AESGCMEncrypt(key, plaintext, aad)
	}
	if err != nil {
		return "", err
	}

	header := make([]byte, 0, 4+1+16+len(out))
	magic := make([]byte, 4)
	binary.BigEndian.PutUint32(magic, crxMagic)
	header = append(header, magic...)
	header = append(header, aeadID)
	header = append(header, salt...)
	header = append(header, out...)

	return base64.RawURLEncoding.EncodeToString(header), nil
}

func DecryptPassphrase(passphrase []byte, token string, aad []byte) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	if len(raw) < 4+1+16 {
		return nil, errors.New("token too short")
	}
	if binary.BigEndian.Uint32(raw[:4]) != crxMagic {
		return nil, errors.New("invalid magic")
	}
	aeadID := raw[4]
	salt := raw[5 : 5+16]
	body := raw[5+16:]

	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, passphrase, salt, []byte("cryptox-hkdf-v1")), key); err != nil {
		return nil, err
	}

	switch aeadID {
	case aesGCMID:
		return AESGCMDecrypt(key, body, aad)
	case xchID:
		return XChaCha20PDecrypt(key, body, aad)
	default:
		return nil, errors.New("unknown aead id")
	}
}

//	func Ed25519Generate() (ed25519.PublicKey, ed25519.PrivateKey, error) {
//		return ed25519.GenerateKey(crypto_rand.Reader)
//	}
//
//	func Ed25519Sign(priv ed25519.PrivateKey, msg []byte) []byte {
//		return ed25519.Sign(priv, msg)
//	}
//
//	func Ed25519Verify(pub ed25519.PublicKey, msg, sig []byte) bool {
//		return ed25519.Verify(pub, msg, sig)
//	}

type KeyPair[P any, S any] struct {
	Public  P
	Private S
}

// GenerateKeyPair is a generic wrapper around a key generation function.
func GenerateKeyPair[P any, S any](fn func(io.Reader) (P, S, error)) (KeyPair[P, S], error) {
	pub, priv, err := fn(crypto_rand.Reader)
	return KeyPair[P, S]{Public: pub, Private: priv}, err
}

// SignFunc is a generic function type that signs a message with a private key.
type SignFunc[S any] func(S, []byte) ([]byte, error)

// VerifyFunc is a generic function type that verifies a signature with a public key.
type VerifyFunc[P any] func(P, []byte, []byte) bool

// SignMessage is a generic helper for signing messages.
func SignMessage[S any](priv S, msg []byte, signFn SignFunc[S]) ([]byte, error) {
	return signFn(priv, msg)
}

// VerifyMessage is a generic helper for verifying signatures.
func VerifyMessage[P any](pub P, msg, sig []byte, verifyFn VerifyFunc[P]) bool {
	return verifyFn(pub, msg, sig)
}

func GenerateEd25519() (KeyPair[ed25519.PublicKey, ed25519.PrivateKey], error) {
	return GenerateKeyPair(ed25519.GenerateKey)
}

func Ed25519Sign(priv ed25519.PrivateKey, msg []byte) ([]byte, error) {
	return ed25519.Sign(priv, msg), nil
}

func Ed25519Verify(pub ed25519.PublicKey, msg, sig []byte) bool {
	return ed25519.Verify(pub, msg, sig)
}

func RSASign(priv *rsa.PrivateKey, msg []byte, h crypto.Hash) ([]byte, error) {
	hasher := h.New()
	hasher.Write(msg)
	return rsa.SignPKCS1v15(crypto_rand.Reader, priv, h, hasher.Sum(nil))
}

func RSAVerify(pub *rsa.PublicKey, msg, sig []byte, h crypto.Hash) error {
	hasher := h.New()
	hasher.Write(msg)
	return rsa.VerifyPKCS1v15(pub, h, hasher.Sum(nil), sig)
}

func ECDSASign(priv *ecdsa.PrivateKey, msg []byte, h crypto.Hash) ([]byte, error) {
	hasher := h.New()
	hasher.Write(msg)
	digest := hasher.Sum(nil)
	return ecdsa.SignASN1(crypto_rand.Reader, priv, digest)
}

func ECDSAVerify(pub *ecdsa.PublicKey, msg, sig []byte, h crypto.Hash) bool {
	hasher := h.New()
	hasher.Write(msg)
	digest := hasher.Sum(nil)
	return ecdsa.VerifyASN1(pub, digest, sig)
}
