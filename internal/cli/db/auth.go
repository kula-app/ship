package db

import (
	"context"
	"time"

	"github.com/kula-app/ship/ent"
)

const authRowID = 1

// GetAuthToken reads the stored access token from the database.
// Returns an empty string and no error if no token is stored.
func GetAuthToken() (string, error) {
	client, err := GetClient()
	if err != nil {
		return "", err
	}

	authEntity, err := client.Auth.Get(context.Background(), authRowID)
	if ent.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return authEntity.AccessToken, nil
}

// SetAuthToken stores the access token, refresh token, and expiry in the database.
// Uses a single-row pattern (id=1) to ensure only one set of credentials is stored.
func SetAuthToken(accessToken, refreshToken string, expiresIn int) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	expiresAt := now + int64(expiresIn)*1000

	ctx := context.Background()

	// Try to update existing row first
	n, err := client.Auth.UpdateOneID(authRowID).
		SetAccessToken(accessToken).
		SetRefreshToken(refreshToken).
		SetExpiresAt(expiresAt).
		SetIssuedAt(now).
		Save(ctx)
	if ent.IsNotFound(err) {
		// Row doesn't exist yet, create it
		_, err = client.Auth.Create().
			SetID(authRowID).
			SetAccessToken(accessToken).
			SetRefreshToken(refreshToken).
			SetExpiresAt(expiresAt).
			SetIssuedAt(now).
			Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	_ = n

	return nil
}

// ClearAuth removes all stored authentication data.
func ClearAuth() error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	// Delete the auth row (ignore not-found)
	err = client.Auth.DeleteOneID(authRowID).Exec(context.Background())
	if ent.IsNotFound(err) {
		return nil
	}
	return err
}

// IsAuthenticated checks whether a valid access token is stored.
func IsAuthenticated() bool {
	token, err := GetAuthToken()
	if err != nil {
		return false
	}
	return token != ""
}
