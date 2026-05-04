package cmd_publish

import (
	"strings"
	"testing"
)

func TestMatchAppIDBySlug(t *testing.T) {
	slug := "gritch"

	appID, err := matchAppIDBySlug([]appLookup{
		{AppID: "app-123", Slug: &slug},
	}, slug)
	if err != nil {
		t.Fatalf("matchAppIDBySlug() error = %v", err)
	}

	if appID != "app-123" {
		t.Fatalf("matchAppIDBySlug() appID = %q, want %q", appID, "app-123")
	}
}

func TestMatchAppIDBySlugNotFound(t *testing.T) {
	otherSlug := "other"

	_, err := matchAppIDBySlug([]appLookup{
		{AppID: "app-123", Slug: &otherSlug},
	}, "gritch")
	if err == nil {
		t.Fatal("matchAppIDBySlug() error = nil, want not found error")
	}

	if !strings.Contains(err.Error(), `app slug "gritch" not found`) {
		t.Fatalf("matchAppIDBySlug() error = %q, want not found message", err)
	}
}

func TestMatchAppIDBySlugDuplicate(t *testing.T) {
	slug := "gritch"

	_, err := matchAppIDBySlug([]appLookup{
		{AppID: "app-123", Slug: &slug},
		{AppID: "app-456", Slug: &slug},
	}, slug)
	if err == nil {
		t.Fatal("matchAppIDBySlug() error = nil, want duplicate error")
	}

	if !strings.Contains(err.Error(), "matched multiple apps") {
		t.Fatalf("matchAppIDBySlug() error = %q, want duplicate message", err)
	}
}
