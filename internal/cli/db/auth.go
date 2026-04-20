package db

import (
	"context"
	"time"

	"github.com/kula-app/ship/ent"
)

const authRowID = 1

const authExpirySkew = time.Minute

// AuthCredentials contains the locally stored OAuth credentials.
type AuthCredentials struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
	IssuedAt     int64
}

// IsExpired reports whether the access token should be refreshed before use.
func (c AuthCredentials) IsExpired(now time.Time) bool {
	if c.ExpiresAt == 0 {
		return true
	}
	return now.Add(authExpirySkew).UnixMilli() >= c.ExpiresAt
}

// GetAuthCredentials reads the stored OAuth credentials from the database.
// Returns nil and no error if no credentials are stored.
func GetAuthCredentials() (*AuthCredentials, error) {
	client, err := GetClient()
	if err != nil {
		return nil, err
	}

	authEntity, err := client.Auth.Get(context.Background(), authRowID)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	credentials := &AuthCredentials{
		AccessToken:  authEntity.AccessToken,
		RefreshToken: authEntity.RefreshToken,
	}
	if authEntity.ExpiresAt != nil {
		credentials.ExpiresAt = *authEntity.ExpiresAt
	}
	if authEntity.IssuedAt != nil {
		credentials.IssuedAt = *authEntity.IssuedAt
	}

	return credentials, nil
}

// GetAuthToken reads the stored access token from the database.
// Returns an empty string and no error if no token is stored.
func GetAuthToken() (string, error) {
	credentials, err := GetAuthCredentials()
	if err != nil {
		return "", err
	}
	if credentials == nil {
		return "", nil
	}
	return credentials.AccessToken, nil
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

// IsAuthenticated checks whether local credentials are stored.
func IsAuthenticated() bool {
	token, err := GetAuthToken()
	if err != nil {
		return false
	}
	return token != ""
}
