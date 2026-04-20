package auth

import (
	"encoding/base64"
	"testing"
)

func TestGenerateStateReturnsBase64URLValue(t *testing.T) {
	state, err := GenerateState()
	if err != nil {
		t.Fatalf("generate state: %v", err)
	}
	if state == "" {
		t.Fatal("expected state to be set")
	}
	if _, err := base64.RawURLEncoding.DecodeString(state); err != nil {
		t.Fatalf("state is not base64url encoded: %v", err)
	}
}
