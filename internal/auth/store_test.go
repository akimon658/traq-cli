package auth

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/99designs/keyring"
)

type fakeKeyring struct {
	items map[string][]byte
}

func newFakeKeyring() *fakeKeyring {
	return &fakeKeyring{items: map[string][]byte{}}
}

func (f *fakeKeyring) Get(key string) (keyring.Item, error) {
	data, ok := f.items[key]
	if !ok {
		return keyring.Item{}, keyring.ErrKeyNotFound
	}
	return keyring.Item{Key: key, Data: data}, nil
}

func (f *fakeKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	if _, ok := f.items[key]; !ok {
		return keyring.Metadata{}, keyring.ErrKeyNotFound
	}
	return keyring.Metadata{Item: &keyring.Item{Key: key}}, nil
}

func (f *fakeKeyring) Set(item keyring.Item) error {
	f.items[item.Key] = item.Data
	return nil
}

func (f *fakeKeyring) Remove(key string) error {
	delete(f.items, key)
	return nil
}

func (f *fakeKeyring) Keys() ([]string, error) {
	keys := make([]string, 0, len(f.items))
	for k := range f.items {
		keys = append(keys, k)
	}
	return keys, nil
}

func testStore(t *testing.T, kr keyring.Keyring, openErr error) defaultStore {
	t.Helper()
	dir := t.TempDir()
	return defaultStore{
		openKeyring: func() (keyring.Keyring, error) {
			return kr, openErr
		},
		userConfigDir: func() (string, error) {
			return dir, nil
		},
	}
}

func testToken() Token {
	return Token{
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
}

func assertRoundTrip(t *testing.T, s defaultStore, wantSource string, verify func(t *testing.T)) {
	t.Helper()
	orig := testToken()
	venue, err := s.Save(orig)
	if err != nil {
		t.Fatal(err)
	}
	if venue != wantSource {
		t.Errorf("venue = %q, want %q", venue, wantSource)
	}

	t.Cleanup(func() {
		if _, err := s.Delete(); err != nil {
			t.Errorf("failed to clean up: %v", err)
		}
	})

	loaded, source, found, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("token not found after save")
	}
	if source != wantSource {
		t.Errorf("source = %q, want %q", source, wantSource)
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

	if _, err := s.Save(orig); err != nil {
		t.Fatal(err)
	}
	loaded2, _, found, err := s.Load()
	if err != nil || !found {
		t.Fatalf("reload failed: found = %v, err = %v", found, err)
	}
	if loaded2.AccessToken != orig.AccessToken {
		t.Errorf("reloaded accessToken = %q, want %q", loaded2.AccessToken, orig.AccessToken)
	}

	if verify != nil {
		verify(t)
	}

	foundAfterDelete, err := s.Delete()
	if err != nil {
		t.Fatal(err)
	}
	if !foundAfterDelete {
		t.Error("Delete should report the item was found")
	}
	if _, _, found, _ := s.Load(); found {
		t.Error("token should be gone after delete")
	}
}

func TestDefaultStoreRoundTripViaKeyring(t *testing.T) {
	kr := newFakeKeyring()
	s := testStore(t, kr, nil)
	path, err := s.filePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), credentialsDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stale"), credentialsFilePerm); err != nil {
		t.Fatal(err)
	}
	assertRoundTrip(t, s, storeSourceKeychain, nil)
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stale credentials file should be removed after keyring save, got err = %v", err)
	}
}

func TestDefaultStoreFileFallback(t *testing.T) {
	s := testStore(t, nil, keyring.ErrNoAvailImpl)
	assertRoundTrip(t, s, storeSourceFile, func(t *testing.T) {
		path, err := s.filePath()
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != credentialsFilePerm {
			t.Errorf("credentials file permission = %o, want %o", perm, credentialsFilePerm)
		}
		dirInfo, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		if perm := dirInfo.Mode().Perm(); perm != credentialsDirPerm {
			t.Errorf("credentials dir permission = %o, want %o", perm, credentialsDirPerm)
		}
	})
}

func TestDefaultStoreNotFound(t *testing.T) {
	s := testStore(t, newFakeKeyring(), nil)
	_, _, found, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("found = true, want false when nothing is stored")
	}
	foundAfterDelete, err := s.Delete()
	if err != nil {
		t.Fatal(err)
	}
	if foundAfterDelete {
		t.Error("Delete should report not found when nothing is stored")
	}
}

func TestDefaultStoreBrokenCredentials(t *testing.T) {
	s := testStore(t, nil, keyring.ErrNoAvailImpl)
	path, err := s.filePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), credentialsDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"accessToken":""}`), credentialsFilePerm); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = s.Load()
	if err == nil {
		t.Error("Load should fail on broken credentials")
	}
}
