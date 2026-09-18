package account

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func newPublicUID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate public uid: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
