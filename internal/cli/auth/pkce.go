// Package auth implements OAuth 2.0 PKCE authentication for the CLI.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// GenerateCodeVerifier creates a cryptographically random code verifier
// for the PKCE flow as specified in RFC 7636.
// The verifier is 43 characters of base64url-encoded random bytes.
func GenerateCodeVerifier() (string, error) {
	// 32 random bytes → 43 base64url characters
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallenge creates a S256 code challenge from the given verifier
// as specified in RFC 7636: BASE64URL(SHA256(code_verifier)).
func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
