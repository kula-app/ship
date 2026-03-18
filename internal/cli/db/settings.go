package db

import (
	"context"

	"github.com/kula-app/ship/ent"
	"github.com/kula-app/ship/ent/setting"
)

// GetSetting reads a setting value by key.
// Returns an empty string and no error if the key doesn't exist.
func GetSetting(key string) (string, error) {
	client, err := GetClient()
	if err != nil {
		return "", err
	}

	s, err := client.Setting.Query().
		Where(setting.KeyEQ(key)).
		Only(context.Background())
	if ent.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return s.Value, nil
}

// SetSetting stores a setting value by key (upsert).
func SetSetting(key, value string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Try to update existing
	n, err := client.Setting.Update().
		Where(setting.KeyEQ(key)).
		SetValue(value).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		// Doesn't exist yet, create it
		_, err = client.Setting.Create().
			SetKey(key).
			SetValue(value).
			Save(ctx)
		return err
	}

	return nil
}
