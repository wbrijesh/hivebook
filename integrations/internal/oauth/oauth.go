// Package oauth holds the connector-authorization primitives: AES-GCM encryption
// for tokens at rest and a signed, expiring state for the OAuth redirect, which
// ties the session-less callback back to a tenant (ADR-0027, design-doc 0009).
package oauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// --- token encryption ----------------------------------------------------

// Cipher encrypts/decrypts secrets at rest with AES-256-GCM. The key is a 32-byte
// secret from config; swapping it for a customer-managed key is the BYOK path
// (ADR-0010).
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher builds a Cipher from a 32-byte key.
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("token key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// EncryptString seals a string as nonce||ciphertext. Empty input encrypts to nil
// so an absent token round-trips to absent.
func (c *Cipher) EncryptString(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, []byte(s), nil), nil
}

// DecryptString opens a nonce||ciphertext blob. nil/empty decrypts to "".
func (c *Cipher) DecryptString(b []byte) (string, error) {
	if len(b) == 0 {
		return "", nil
	}
	ns := c.aead.NonceSize()
	if len(b) < ns {
		return "", errors.New("ciphertext too short")
	}
	plain, err := c.aead.Open(nil, b[:ns], b[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// --- signed OAuth state --------------------------------------------------

// State is the payload carried through the OAuth redirect: which tenant, region,
// and connector started the flow, a nonce, and an expiry.
type State struct {
	Tenant    string `json:"t"`
	Region    string `json:"r"`
	Connector string `json:"c"`
	Nonce     string `json:"n"`
	Exp       int64  `json:"e"` // unix seconds
}

// Signer signs and verifies State with HMAC-SHA256.
type Signer struct {
	key []byte
}

// NewSigner builds a Signer from any non-empty key.
func NewSigner(key []byte) (*Signer, error) {
	if len(key) == 0 {
		return nil, errors.New("state key must not be empty")
	}
	return &Signer{key: key}, nil
}

var b64 = base64.RawURLEncoding

// Sign mints a signed state for (tenant, region, connector) valid for ttl. The
// token is "<payload>.<mac>", both base64url.
func (s *Signer) Sign(tenant, region, connector string, ttl time.Duration) (string, error) {
	nonce := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	payload, err := json.Marshal(State{
		Tenant:    tenant,
		Region:    region,
		Connector: connector,
		Nonce:     b64.EncodeToString(nonce),
		Exp:       time.Now().Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	enc := b64.EncodeToString(payload)
	return enc + "." + b64.EncodeToString(s.mac([]byte(enc))), nil
}

// Verify checks the signature and expiry and returns the State.
func (s *Signer) Verify(token string) (State, error) {
	enc, macPart, ok := strings.Cut(token, ".")
	if !ok {
		return State{}, errors.New("malformed state")
	}
	gotMAC, err := b64.DecodeString(macPart)
	if err != nil {
		return State{}, errors.New("malformed state mac")
	}
	if subtle.ConstantTimeCompare(gotMAC, s.mac([]byte(enc))) != 1 {
		return State{}, errors.New("bad state signature")
	}
	payload, err := b64.DecodeString(enc)
	if err != nil {
		return State{}, errors.New("malformed state payload")
	}
	var st State
	if err := json.Unmarshal(payload, &st); err != nil {
		return State{}, errors.New("malformed state payload")
	}
	if time.Now().Unix() > st.Exp {
		return State{}, errors.New("state expired")
	}
	return st, nil
}

func (s *Signer) mac(msg []byte) []byte {
	h := hmac.New(sha256.New, s.key)
	h.Write(msg)
	return h.Sum(nil)
}
