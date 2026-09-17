package catalogsign

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

const Algorithm = "rsa-sha256-pkcs1v15"

type Signer struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	publicPEM  string
	keyID      string
}

type Signature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	Value     string `json:"value"`
}

type Envelope struct {
	Payload       json.RawMessage `json:"payload"`
	PayloadBase64 string          `json:"payload_base64"`
	Signature     Signature       `json:"signature"`
}

type PublicKeyInfo struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	PEM       string `json:"pem"`
}

func Load(privatePath, publicPath string) (*Signer, error) {
	if privatePath == "" {
		return nil, errors.New("catalog private key path is required")
	}
	privatePEM, err := os.ReadFile(privatePath)
	if err != nil {
		return nil, fmt.Errorf("read catalog private key: %w", err)
	}
	privateKey, err := parsePrivateKey(privatePEM)
	if err != nil {
		return nil, err
	}
	if privateKey.N.BitLen() < 2048 {
		return nil, errors.New("catalog RSA key must be at least 2048 bits")
	}

	publicKey := &privateKey.PublicKey
	if publicPath != "" {
		publicPEM, err := os.ReadFile(publicPath)
		if err != nil {
			return nil, fmt.Errorf("read catalog public key: %w", err)
		}
		parsed, err := parsePublicKey(publicPEM)
		if err != nil {
			return nil, err
		}
		if parsed.E != publicKey.E || parsed.N.Cmp(publicKey.N) != 0 {
			return nil, errors.New("catalog public key does not match private key")
		}
	}

	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal catalog public key: %w", err)
	}
	fingerprint := sha256.Sum256(publicDER)
	publicBlock := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	return &Signer{
		privateKey: privateKey,
		publicKey:  publicKey,
		publicPEM:  string(publicBlock),
		keyID:      "sha256:" + hex.EncodeToString(fingerprint[:]),
	}, nil
}

func (s *Signer) SignJSON(payload any) (*Envelope, error) {
	if s == nil || s.privateKey == nil {
		return nil, errors.New("catalog signer is not configured")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal catalog manifest: %w", err)
	}
	digest := sha256.Sum256(raw)
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return nil, fmt.Errorf("sign catalog manifest: %w", err)
	}
	return &Envelope{
		Payload:       json.RawMessage(raw),
		PayloadBase64: base64.StdEncoding.EncodeToString(raw),
		Signature: Signature{
			Algorithm: Algorithm,
			KeyID:     s.keyID,
			Value:     base64.StdEncoding.EncodeToString(signature),
		},
	}, nil
}

func (s *Signer) PublicKeyInfo() PublicKeyInfo {
	if s == nil {
		return PublicKeyInfo{}
	}
	return PublicKeyInfo{Algorithm: Algorithm, KeyID: s.keyID, PEM: s.publicPEM}
}

func (s *Signer) KeyID() string {
	if s == nil {
		return ""
	}
	return s.keyID
}

func parsePrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid catalog private key PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse catalog private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("catalog private key is not RSA")
	}
	return rsaKey, nil
}

func parsePublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid catalog public key PEM")
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse catalog public key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("catalog public key is not RSA")
	}
	return rsaKey, nil
}
