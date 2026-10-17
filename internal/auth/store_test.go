package auth

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	orig := Token{
		Method:       MethodOAuth,
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Scope:        []string{ScopeRead, ScopeWrite},
		Origin:       DefaultOrigin,
		UserID:       "00000000-0000-0000-0000-000000000000",
		UserName:     "akimo",
		UserBot:      true,
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	venue, err := Save(orig)
	if err != nil {
		t.Skipf("skipping: storage is unavailable: %v", err)
	}
	switch venue {
	case storeSourceFile:
		path, err := StorePath()
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("credentials file permission = %o, want 600", perm)
		}
	case storeSourceKeychain:
	default:
		t.Errorf("unexpected storage venue: %q", venue)
	}

	t.Cleanup(func() {
		if _, err := Delete(); err != nil {
			t.Errorf("failed to clean up: %v", err)
		}
	})

	loaded, source, found, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("token not found after save")
	}
	if source != venue {
		t.Errorf("source = %q, want %q", source, venue)
	}
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"method", loaded.Method, orig.Method},
		{"accessToken", loaded.AccessToken, orig.AccessToken},
		{"refreshToken", loaded.RefreshToken, orig.RefreshToken},
		{"origin", loaded.Origin, orig.Origin},
		{"userId", loaded.UserID, orig.UserID},
		{"userName", loaded.UserName, orig.UserName},
		{"userBot", loaded.UserBot, orig.UserBot},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if !reflect.DeepEqual(loaded.Scope, orig.Scope) {
		t.Errorf("scope = %v, want %v", loaded.Scope, orig.Scope)
	}
	if !loaded.ExpiresAt.Equal(orig.ExpiresAt) {
		t.Errorf("expiresAt = %v, want %v", loaded.ExpiresAt, orig.ExpiresAt)
	}

	if _, err := Save(orig); err != nil {
		t.Fatal(err)
	}
	loaded2, _, found, err := Load()
	if err != nil || !found {
		t.Fatalf("reload failed: found = %v, err = %v", found, err)
	}
	if loaded2.AccessToken != orig.AccessToken {
		t.Errorf("reloaded accessToken = %q, want %q", loaded2.AccessToken, orig.AccessToken)
	}

	if _, err := Delete(); err != nil {
		t.Fatal(err)
	}
	if _, _, found, _ := Load(); found {
		t.Error("token should be gone after delete")
	}
}
