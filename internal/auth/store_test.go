package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/99designs/keyring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	assert.Equal(t, wantSource, venue)

	t.Cleanup(func() {
		_, err := s.Delete()
		assert.NoError(t, err, "failed to clean up")
	})

	loaded, source, found, err := s.Load()
	require.NoError(t, err)
	require.True(t, found, "token not found after save")
	assert.Equal(t, wantSource, source)
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
		assert.Equal(t, c.want, c.got, c.name)
	}
	assert.Equal(t, orig.Scope, loaded.Scope)
	assert.True(t, loaded.ExpiresAt.Equal(orig.ExpiresAt),
		"expiresAt = %v, want %v", loaded.ExpiresAt, orig.ExpiresAt)

	_, err = s.Save(orig)
	require.NoError(t, err)
	loaded2, _, found, err := s.Load()
	require.NoError(t, err)
	require.True(t, found, "reload failed")
	assert.Equal(t, orig.AccessToken, loaded2.AccessToken)

	if verify != nil {
		verify(t)
	}

	foundAfterDelete, err := s.Delete()
	require.NoError(t, err)
	assert.True(t, foundAfterDelete, "Delete should report the item was found")
	_, _, found, _ = s.Load()
	assert.False(t, found, "token should be gone after delete")
}

func TestDefaultStoreRoundTripViaKeyring(t *testing.T) {
	kr := newFakeKeyring()
	s := testStore(t, kr, nil)
	path, err := s.filePath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), credentialsDirPerm))
	require.NoError(t, os.WriteFile(path, []byte("stale"), credentialsFilePerm))
	assertRoundTrip(t, s, storeSourceKeychain, nil)
	_, err = os.Stat(path)
	assert.ErrorIs(t, err, os.ErrNotExist,
		"stale credentials file should be removed after keyring save")
}

func TestDefaultStoreFileFallback(t *testing.T) {
	s := testStore(t, nil, keyring.ErrNoAvailImpl)
	assertRoundTrip(t, s, storeSourceFile, func(t *testing.T) {
		path, err := s.filePath()
		require.NoError(t, err)
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(credentialsFilePerm), info.Mode().Perm(),
			"credentials file permission")
		dirInfo, err := os.Stat(filepath.Dir(path))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(credentialsDirPerm), dirInfo.Mode().Perm(),
			"credentials dir permission")
	})
}

func TestDefaultStoreNotFound(t *testing.T) {
	s := testStore(t, newFakeKeyring(), nil)
	_, _, found, err := s.Load()
	require.NoError(t, err)
	assert.False(t, found, "found = true, want false when nothing is stored")
	foundAfterDelete, err := s.Delete()
	require.NoError(t, err)
	assert.False(t, foundAfterDelete, "Delete should report not found when nothing is stored")
}

func TestDefaultStoreBrokenCredentials(t *testing.T) {
	s := testStore(t, nil, keyring.ErrNoAvailImpl)
	path, err := s.filePath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), credentialsDirPerm))
	require.NoError(t, os.WriteFile(path, []byte(`{"accessToken":""}`), credentialsFilePerm))
	_, _, _, err = s.Load()
	assert.Error(t, err, "Load should fail on broken credentials")
}
